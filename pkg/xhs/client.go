package xhs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Browser is the page-control surface xhs.Client needs. The mcp-service tool
// layer adapts a session-managed *browseragent.Browser to this interface; it
// keeps this package free of any browseragent dependency.
type Browser interface {
	// Navigate opens url in the shared page. On the stealth (Obscura) path this
	// injects preset cookies and waits for render.
	Navigate(ctx context.Context, url string) error
	// Eval runs JavaScript in the page and returns its value as a string.
	// Promises are awaited, so async JS (fetch) is supported.
	Eval(ctx context.Context, js string) (string, error)
	// EvalAsync runs JS that returns a Promise, awaits it, and returns the
	// resolved value as a string. Required for in-page fetch on Obscura, where
	// plain Eval leaves the page event loop frozen so async callbacks never fire.
	EvalAsync(ctx context.Context, js string) (string, error)
	// PressEnter dispatches a real (isTrusted) Enter key to the focused element.
	// Needed because XHS ignores synthetic KeyboardEvents (isTrusted guard).
	PressEnter(ctx context.Context) error
	// InjectInitScript injects JS that runs before each page's own scripts, so
	// fetch/XHR patching can capture requests the SPA fires during page load.
	InjectInitScript(ctx context.Context, source string) (string, error)
}

// Client reads XHS notes and searches the XHS note API.
type Client struct{}

// NewClient returns an XHS client.
func NewClient() *Client { return &Client{} }

// noteWait is how long ReadNote will keep retrying for the SSR note state.
const noteWait = 8 * time.Second

// homeWait lets the home page boot the SPA and acquire server-set session
// cookies (a1, webId, ...) before navigating to the search page. Without this
// warm-up a fresh browser gets 302'd from /search_result back to home.
const homeWait = 4 * time.Second

// initialWait is how long Search waits for the URL-load search to populate
// before forcing a client-side search via Enter.
const initialWait = 5 * time.Second

// searchWait bounds how long Search polls after forcing the search.
const searchWait = 12 * time.Second

// feedCard mirrors one entry returned by searchFeedsJS.
type feedCard struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	XSecToken string `json:"xsec_token"`
	Type      string `json:"type"`
	Author    string `json:"author"`
	Liked     string `json:"liked"`
}

// searchFeedsResult mirrors the object returned by searchFeedsJS.
type searchFeedsResult struct {
	Count         int         `json:"count"`
	Notes         []feedCard  `json:"notes"`
	DomCards      int         `json:"domCards"`
	DomNotes      []feedCard  `json:"domNotes"`
	Href          string      `json:"href"`
	HasState      bool        `json:"hasState"`
	Error         string      `json:"error"`
	SearchMissing bool        `json:"searchMissing"`
	SearchKeys    []string    `json:"searchKeys"`
	StateKeys     []string    `json:"stateKeys"`
	FeedsType     string      `json:"feedsType"`
	SearchValue   string      `json:"searchValue"`
	HasMore       interface{} `json:"hasMore"`
	FirstEnter    interface{} `json:"firstEnter"`
	PageTextHead  string      `json:"pageTextHead"`
}

func parseSearchFeeds(raw string) searchFeedsResult {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return searchFeedsResult{}
	}
	var r searchFeedsResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return searchFeedsResult{Error: err.Error()}
	}
	return r
}

// ReadNote opens a note (by id, URL, or share link) and returns its structured
// content. Note data is server-rendered into __INITIAL_STATE__, so this path is
// reliable and does not depend on the search anti-bot.
func (c *Client) ReadNote(ctx context.Context, br Browser, ref string) (*Note, error) {
	noteID, xsecToken := ParseShareLink(ref)
	if noteID == "" {
		return nil, fmt.Errorf("无法从 %q 解析笔记ID；请提供笔记链接或ID", ref)
	}
	noteURL := BuildNoteURL(noteID, xsecToken)
	// Inject conditional anti-detection patches before navigation (no-op on
	// Obscura; needed for headed-Chrome). Persists across navigations.
	_, _ = br.InjectInitScript(ctx, stealthPatchJS())
	if err := br.Navigate(ctx, noteURL); err != nil {
		return nil, fmt.Errorf("导航笔记页失败: %w", err)
	}

	// The stealth navigate already sleeps for render; retry the extract for a
	// few seconds in case the SSR state is slow to populate.
	deadline := time.Now().Add(noteWait)
	var last string
	for {
		raw, err := br.Eval(ctx, noteExtractJS())
		if err != nil {
			return nil, fmt.Errorf("读取笔记状态失败: %w", err)
		}
		last = raw
		if n, ok := tryParseNote(raw, noteURL); ok {
			return n, nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("笔记状态未就绪（可能需要登录或链接已失效）: %s", head(last, 300))
}

func tryParseNote(raw, noteURL string) (*Note, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, false
	}
	var probe struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err == nil && probe.Error != "" {
		// no_initial_state / no_note_in_map etc. -> retry
		return nil, false
	}
	var n Note
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return nil, false
	}
	if n.Title == "" && n.Desc == "" && n.ID == "" {
		return nil, false
	}
	if n.ID == "" {
		// fall back to the id parsed from the link
		id, _ := ParseShareLink(noteURL)
		n.ID = id
	}
	n.URL = noteURL
	if n.Tags == nil {
		n.Tags = []string{}
	}
	return &n, true
}

// FormatNote renders a Note as readable text for the LLM.
func FormatNote(n *Note) string {
	var b strings.Builder
	fmt.Fprintf(&b, "【小红书笔记】%s\n", n.Title)
	fmt.Fprintf(&b, "作者: %s  类型: %s\n", n.Author, n.Type)
	fmt.Fprintf(&b, "点赞: %s  评论: %s\n", n.Liked, n.Comment)
	if len(n.Tags) > 0 {
		fmt.Fprintf(&b, "标签: %s\n", strings.Join(n.Tags, " / "))
	}
	if n.Desc != "" {
		fmt.Fprintf(&b, "\n正文:\n%s\n", n.Desc)
	}
	fmt.Fprintf(&b, "\n链接: %s\n", n.URL)
	return b.String()
}

// Search looks up keyword on XHS and returns matching note cards.
//
// Flow:
//  1. Navigate to home first. This boots the SPA and, critically, makes XHS's
//     server set the session cookies (a1, webId, ...) that a fresh browser
//     lacks. Without this warm-up, navigating straight to /search_result
//     302-redirects back to home.
//  2. Navigate to /search_result?keyword=... which now lands on the real
//     search page (title "<kw> - 小红书搜索", __INITIAL_STATE__ present).
//  3. Read __INITIAL_STATE__.search.feeds AND scrape DOM note-card links.
//  4. If both are empty, force a client-side search by dispatching Enter on the
//     search input (the page's own signed search XHR then fires and completes
//     during the polling loop), and re-poll.
//
// Obscura runs the page's OWN async (the signed search XHR) but not async
// scheduled via CDP Evaluate, so we never call fetch ourselves - we let the
// page do it and read the result synchronously.
func (c *Client) Search(ctx context.Context, br Browser, keyword string, page int, sort string) (*SearchResult, error) {
	if keyword = strings.TrimSpace(keyword); keyword == "" {
		return nil, fmt.Errorf("keyword 不能为空")
	}
	if page < 1 {
		page = 1
	}
	if !IsValidSort(sort) {
		return nil, fmt.Errorf("不支持的排序: %q", sort)
	}

	// Inject conditional anti-detection patches before navigation (no-op on
	// Obscura; needed for headed-Chrome). Persists across navigations.
	_, _ = br.InjectInitScript(ctx, stealthPatchJS())

	// 1. Warm up on home so the session cookies are established.
	if err := br.Navigate(ctx, HomeURL()); err != nil {
		return nil, fmt.Errorf("导航首页失败: %w", err)
	}
	sleep(ctx, homeWait)

	// 2. Navigate to the search_result page; the page's own JS fires the signed
	// search XHR on load.
	if err := br.Navigate(ctx, BuildSearchPageURL(keyword)); err != nil {
		return nil, fmt.Errorf("导航搜索页失败: %w", err)
	}

	// 3. Navigate to the search_result page so _webmsxyw and the signing
	// context are loaded.
	if err := br.Navigate(ctx, BuildSearchPageURL(keyword)); err != nil {
		return nil, fmt.Errorf("导航搜索页失败: %w", err)
	}
	sleep(ctx, 3*time.Second)

	// 4. Signed in-page fetch to the search API. The SPA's own search never
	// fires (Obscura freezes the post-hydration event loop), so this is the
	// only path to the API.
	fetchCards, fetchDiag := c.signedFetchSearch(ctx, br, keyword)
	cards := fetchCards

	diag := SearchDiagnostics{
		Attempt: "signed_fetch",
		Note:    fetchDiag,
	}
	if len(cards) == 0 {
		return &SearchResult{
			Keyword: keyword, Page: page, Count: 0, Notes: cards,
			Diagnostic: diag.String(),
		}, fmt.Errorf("搜索无结果（可能被反爬挡住或关键词无匹配）: %s", diag.String())
	}
	return &SearchResult{
		Keyword:    keyword,
		Page:       page,
		Count:      len(cards),
		Notes:      cards,
		Diagnostic: diag.String(),
	}, nil
}

// pollFeeds reads searchFeedsJS repeatedly until notes/domNotes appear or the
// deadline passes, scrolling between attempts to trigger lazy loads.
func pollFeeds(ctx context.Context, br Browser, wait time.Duration) searchFeedsResult {
	deadline := time.Now().Add(wait)
	var feeds searchFeedsResult
	for {
		raw, err := br.Eval(ctx, searchFeedsJS())
		if err == nil {
			feeds = parseSearchFeeds(raw)
		} else {
			feeds = searchFeedsResult{Error: err.Error()}
		}
		if len(feeds.Notes) > 0 || len(feeds.DomNotes) > 0 {
			return feeds
		}
		if time.Now().After(deadline) {
			return feeds
		}
		_, _ = br.Eval(ctx, scrollPageJS())
		sleep(ctx, 2500*time.Millisecond)
	}
}

// cardsFromFeeds builds NoteCards from the state feeds (preferred) or the DOM
// note-card links (fallback).
func cardsFromFeeds(feeds searchFeedsResult) []NoteCard {
	src := feeds.Notes
	if len(src) == 0 {
		src = feeds.DomNotes
	}
	return cardsFromFeedCards(src)
}

// cardsFromFeedCards converts raw feed cards (from either the page-state poll
// or the signed-fetch parse) into NoteCards.
func cardsFromFeedCards(src []feedCard) []NoteCard {
	cards := make([]NoteCard, 0, len(src))
	for _, n := range src {
		cards = append(cards, NoteCard{
			ID: n.ID, Title: n.Title, XSecToken: n.XSecToken,
			Type: n.Type, Author: n.Author, Liked: n.Liked,
			URL: BuildNoteURL(n.ID, n.XSecToken),
		})
	}
	return cards
}

// sleep blocks for d, respecting context cancellation.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// fetchResult mirrors the JSON returned by searchFireJS.
type fetchResult struct {
	Status   string            `json:"status"`
	Code     int               `json:"code"`
	Body     string            `json:"body"`
	SignKeys []string          `json:"signKeys"`
	Sign     map[string]string `json:"sign"`
	Msg      string            `json:"msg"`
}

func parseFetchResult(raw string) fetchResult {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return fetchResult{Status: "none"}
	}
	var r fetchResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return fetchResult{Status: "parse_error", Msg: err.Error()}
	}
	return r
}

// captureResult mirrors the JSON returned by readCaptureJS (the injected
// fetch/XHR patch's captured requests + response).
type captureResult struct {
	Resp    string          `json:"resp"`    // captured search-API response body
	Code    int             `json:"code"`    // captured response HTTP status
	Reqs    json.RawMessage `json:"reqs"`    // captured request descriptors
	RespUrl string          `json:"respUrl"` // URL of the captured response
	Href    string          `json:"href"`    // current page URL
}

func parseCapture(raw string) captureResult {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return captureResult{}
	}
	var r captureResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return captureResult{}
	}
	return r
}

// searchWaitResult mirrors the JSON returned by searchWaitJS (the EvalAsync
// fetch-interception capture of the SPA's own search request).
type searchWaitResult struct {
	Status  string          `json:"status"` // done | timeout
	Code    int             `json:"code"`   // captured response HTTP status
	Body    string          `json:"body"`   // captured search-API response body
	Reqs    json.RawMessage `json:"reqs"`   // captured request descriptors (diagnostic)
	TrigErr string          `json:"trigErr"`
	Href    string          `json:"href"`
	HasCap  bool            `json:"hasCap"`
}

func parseSearchWait(raw string) searchWaitResult {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return searchWaitResult{Status: "none"}
	}
	var r searchWaitResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return searchWaitResult{Status: "parse_error"}
	}
	return r
}

// Cookie is a name/value cookie pair for ServerSignedSearch.
type Cookie struct {
	Name, Value string
}

// HTTPDoer is the subset of *http.Client ServerSignedSearch needs.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ServerSignedSearch signs the search request in the browser (via _webmsxyw)
// then sends it server-to-server to the XHS API. This bypasses the browser
// CORS preflight that blocks an in-page fetch to edith.xiaohongshu.com, and
// bypasses the SPA (whose search XHR never fires under Obscura's frozen
// post-hydration event loop). Returns parsed cards plus a one-line diagnostic.
func (c *Client) ServerSignedSearch(ctx context.Context, br Browser, keyword string, cookies []Cookie, doer HTTPDoer) ([]NoteCard, string) {
	// Inject stealth patches FIRST (before any page JS runs) so anti-bot checks
	// (webdriver, plugins, chrome object, ...) see a real headed browser, then
	// the idempotent x-s-common capture patch. Both persist across navigations.
	if _, err := br.InjectInitScript(ctx, stealthPatchJS()); err != nil {
		return nil, "stealth_inject_error:" + err.Error()
	}
	if _, err := br.InjectInitScript(ctx, captureAllPatchJS()); err != nil {
		return nil, "inject_error:" + err.Error()
	}
	// (Re)navigate home so the SPA's load-time XHRs fire with the patch active.
	// We're looking for x-s-common, which _webmsxyw does NOT return but the SPA
	// sets on its own requests.
	if err := br.Navigate(ctx, HomeURL()); err != nil {
		return nil, "nav_error:" + err.Error()
	}
	// Also patch post-load (backup in case the init-script was skipped) and pump
	// so any later XHRs are caught too.
	_, _ = br.Eval(ctx, captureAllPatchJS())
	for i := 0; i < 3; i++ {
		_, _ = br.EvalAsync(ctx, pumpJS())
	}
	capRaw, _ := br.Eval(ctx, readAllCaptureJS())
	var caps []struct {
		URL string `json:"url"`
		Xsc string `json:"xsc"`
	}
	_ = json.Unmarshal([]byte(strings.TrimSpace(capRaw)), &caps)
	// Pick the longest captured x-s-common (most complete device fingerprint).
	capturedXsc := ""
	for _, c := range caps {
		if len(c.Xsc) > len(capturedXsc) {
			capturedXsc = c.Xsc
		}
	}

	// Sign the search request (sync - pumping already done above).
	raw, err := br.Eval(ctx, signSyncJS(keyword))
	if err != nil {
		return nil, "sign_eval_error:" + err.Error()
	}
	var s struct {
		Xs, Xt, Xsc, Body, Path string
		SignKeys                []string
		V18                     string
		Error, Msg              string
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, "sign_parse_error:" + err.Error() + " raw=" + head(raw, 200)
	}
	if s.Error != "" {
		return nil, "sign_error:" + s.Error + ":" + s.Msg
	}
	if s.Xs == "" {
		return nil, "no_xs signKeys=" + fmt.Sprint(s.SignKeys)
	}
	// Prefer the SPA's captured x-s-common over the (empty) signed one.
	xsc := s.Xsc
	if xsc == "" {
		xsc = capturedXsc
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://edith.xiaohongshu.com"+s.Path, strings.NewReader(s.Body))
	if err != nil {
		return nil, "req_error:" + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-s", s.Xs)
	req.Header.Set("x-t", s.Xt)
	if xsc != "" {
		req.Header.Set("x-s-common", xsc)
	}
	req.Header.Set("Origin", "https://www.xiaohongshu.com")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	for _, ck := range cookies {
		req.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
	}

	resp, err := doer.Do(req)
	if err != nil {
		return nil, "http_error:" + err.Error()
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyBytes)
	capSummary := "[]"
	if len(caps) > 0 {
		capSummary = fmt.Sprintf("[%d reqs, xscLen=%d]", len(caps), len(capturedXsc))
	}
	diag := fmt.Sprintf("http=%d signKeys=%v xsPrefix=%v xsc=%v(signed=%v,captured=%v) v18=%s capReqs=%s bodyHead=%q",
		resp.StatusCode, s.SignKeys, head(s.Xs, 4), xsc != "", s.Xsc != "", capturedXsc != "", head(s.V18, 60), capSummary, head(bodyStr, 200))
	if resp.StatusCode != 200 {
		return nil, diag
	}
	cards, _, perr := ParseSearchResponse(bodyStr)
	if perr != nil {
		return nil, diag + " parse_err:" + perr.Error()
	}
	return cards, diag
}

// inside the page (via EvalAsync so the event loop pumps and the fetc
// inside the page (via EvalAsync so the event loop pumps and the fetch
// completes) and parses the response into note cards. Returns cards (if any)
// plus a one-line diagnostic.
func (c *Client) signedFetchSearch(ctx context.Context, br Browser, keyword string) ([]NoteCard, string) {
	diagRaw, _ := br.Eval(ctx, signDiagJS())
	raw, err := br.EvalAsync(ctx, searchFireJS(keyword))
	if err != nil {
		return nil, "eval_error:" + err.Error() + " diag=" + head(diagRaw, 300)
	}
	fr := parseFetchResult(raw)
	diag := fmt.Sprintf("status=%s code=%d sign=%v bodyHead=%q msg=%q diag=%s",
		fr.Status, fr.Code, fr.Sign, head(fr.Body, 200), fr.Msg, head(diagRaw, 800))
	if fr.Body == "" {
		return nil, diag
	}
	cards, _, perr := ParseSearchResponse(fr.Body)
	if perr != nil {
		return nil, diag + " parse_err:" + perr.Error()
	}
	return cards, diag
}

// FormatSearch renders a SearchResult as readable text for the LLM.
func FormatSearch(r *SearchResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "【小红书搜索】%s （第%d页，共%d条）\n", r.Keyword, r.Page, r.Count)
	if r.Diagnostic != "" {
		fmt.Fprintf(&b, "诊断: %s\n", r.Diagnostic)
	}
	if r.Count == 0 {
		b.WriteString("\n无结果（可能被反爬挡住或关键词无匹配）。\n")
		return b.String()
	}
	b.WriteString("\n")
	for i, n := range r.Notes {
		fmt.Fprintf(&b, "%d. %s\n", i+1, n.Title)
		fmt.Fprintf(&b, "   作者: %s  点赞: %s  类型: %s\n", n.Author, n.Liked, n.Type)
		fmt.Fprintf(&b, "   链接: %s\n", n.URL)
	}
	b.WriteString("\n提示: 用 xhs_read_note 读取任一条目的完整内容（传上面的链接）。\n")
	return b.String()
}
