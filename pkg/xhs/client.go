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

// searchJSResult mirrors the object returned by searchJS.
type searchJSResult struct {
	Attempt    string                 `json:"attempt"`
	Status     int                    `json:"status"`
	Body       string                 `json:"body"`
	SignFn     string                 `json:"signFn"`
	SignedKeys []string               `json:"signedKeys"`
	Plain      map[string]interface{} `json:"plain"`
	Candidates []string               `json:"candidates"`
	Detail     string                 `json:"detail"`
}

// Search looks up keyword on XHS and returns matching note cards. It drives the
// search from inside a real XHS page so XHS's own JS signs the request; see
// sign.go for the two-tier (plain fetch -> explicit sign) strategy.
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

	// Warm the signing context (load XHS SPA JS + cookies).
	if err := br.Navigate(ctx, HomeURL()); err != nil {
		return nil, fmt.Errorf("导航小红书首页失败: %w", err)
	}

	raw, err := br.Eval(ctx, searchJS(keyword, page, sort))
	if err != nil {
		return nil, fmt.Errorf("执行搜索失败: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, fmt.Errorf("搜索无返回（浏览器页面可能未就绪）")
	}

	var jsr searchJSResult
	if err := json.Unmarshal([]byte(raw), &jsr); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w (head: %s)", err, head(raw, 300))
	}

	diag := SearchDiagnostics{
		Attempt:    jsr.Attempt,
		HTTPStatus: jsr.Status,
		SignFn:     jsr.SignFn,
		SignedKeys: jsr.SignedKeys,
	}

	// No usable body -> surface why.
	if jsr.Body == "" || jsr.Status != 200 {
		note := jsr.Detail
		if note == "" {
			note = fmt.Sprintf("attempt=%s status=%d", jsr.Attempt, jsr.Status)
		}
		if len(jsr.Candidates) > 0 {
			note += fmt.Sprintf("；未找到签名函数，候选: %s", strings.Join(jsr.Candidates, ","))
		}
		diag.Note = note
		return &SearchResult{
			Keyword: keyword, Page: page, Count: 0, Notes: []NoteCard{},
			Diagnostic: diag.String(),
		}, fmt.Errorf("搜索被反爬挡住: %s", diag.String())
	}

	cards, parseDiag, perr := ParseSearchResponse(jsr.Body)
	// Carry the parse diagnostics (xhs_code etc.) into our diag.
	diag.XHSCode = parseDiag.XHSCode
	if diag.Note == "" {
		diag.Note = parseDiag.Note
	}
	if perr != nil {
		return &SearchResult{
			Keyword: keyword, Page: page, Count: 0, Notes: []NoteCard{},
			Diagnostic: diag.String(),
		}, fmt.Errorf("解析搜索响应失败: %w", perr)
	}

	return &SearchResult{
		Keyword:    keyword,
		Page:       page,
		Count:      len(cards),
		Notes:      cards,
		Diagnostic: diag.String(),
	}, nil
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
