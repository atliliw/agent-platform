package xhs

import (
	"context"
	"encoding/json"
	"fmt"
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
	Count         int        `json:"count"`
	Notes         []feedCard `json:"notes"`
	DomCards      int        `json:"domCards"`
	DomNotes      []feedCard `json:"domNotes"`
	Href          string     `json:"href"`
	HasState      bool       `json:"hasState"`
	Error         string     `json:"error"`
	SearchMissing bool       `json:"searchMissing"`
	SearchKeys    []string   `json:"searchKeys"`
	StateKeys     []string   `json:"stateKeys"`
	FeedsType     string     `json:"feedsType"`
	SearchValue   string     `json:"searchValue"`
	HasMore       string     `json:"hasMore"`
	FirstEnter    string     `json:"firstEnter"`
	PageTextHead  string     `json:"pageTextHead"`
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

	// 3. Initial poll for the URL-load search.
	feeds := pollFeeds(ctx, br, initialWait)
	enterResult := ""

	// 4. If empty, force a client-side search via Enter and re-poll.
	if len(feeds.Notes) == 0 && len(feeds.DomNotes) == 0 {
		if raw, err := br.Eval(ctx, searchSubmitJS(keyword)); err == nil {
			enterResult = strings.TrimSpace(raw)
		} else {
			enterResult = "eval_error:" + err.Error()
		}
		feeds = pollFeeds(ctx, br, searchWait)
	}

	cards := cardsFromFeeds(feeds)

	diag := SearchDiagnostics{
		Attempt: "page_capture",
		Note: fmt.Sprintf("feeds=%d domCards=%d hasState=%v searchValue=%q hasMore=%v firstEnter=%v searchKeys=%v feedsType=%s enter=%s pageTextHead=%q err=%q",
			feeds.Count, feeds.DomCards, feeds.HasState, feeds.SearchValue, feeds.HasMore, feeds.FirstEnter,
			feeds.SearchKeys, feeds.FeedsType, head(enterResult, 120), head(feeds.PageTextHead, 100), feeds.Error),
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
