package tools

import (
	"context"
	"fmt"

	"agent-platform/pkg/browseragent"
	"agent-platform/pkg/xhs"
)

// ============================================================
// Xiaohongshu (小红书) dedicated tools
//
// These wrap pkg/xhs.Client, which owns all XHS-specific logic. The generic
// browser_extract no longer carries any XHS special-casing. Both tools are
// session-aware: they reuse the same stealth browser page as the browser_*
// primitives when tool_config.extra.session_id is set, so XHS cookies/login
// state stay warm across a search -> read-note chain.
// ============================================================

// xhsBrowserAdapter adapts the session-managed browserOps to xhs.Browser.
type xhsBrowserAdapter struct{ b browserOps }

func (a xhsBrowserAdapter) Navigate(ctx context.Context, url string) error {
	_, err := a.b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionNavigate, URL: url})
	return err
}

func (a xhsBrowserAdapter) Eval(ctx context.Context, js string) (string, error) {
	return a.b.Eval(ctx, js)
}

// injectXHSCookies loads stored XHS cookies for the URL's domain and injects
// them so the page renders logged-in. Mirrors BrowserNavigateTool's behavior.
func injectXHSCookies(ctx context.Context, b browserOps, loader cookieLoader, url string) {
	if loader == nil {
		return
	}
	cks, _ := loader.LoadCookiesForURL(ctx, url)
	if len(cks) > 0 {
		b.SetCookies(cks)
	}
}

// resolveXHSURL turns a user-supplied ref (link or id) into a full note URL so
// cookies load for the right domain even when only an id was given.
func resolveXHSURL(ref string) string {
	if id, token := xhs.ParseShareLink(ref); id != "" {
		return xhs.BuildNoteURL(id, token)
	}
	return ref
}

// ------------------------------------------------------------
// xhs_read_note
// ------------------------------------------------------------

// XHSReadNoteTool reads a Xiaohongshu note's full structured content.
type XHSReadNoteTool struct {
	provider browserProvider
	cookies  cookieLoader
	client   *xhs.Client
}

// NewXHSReadNoteTool creates the production read-note tool.
func NewXHSReadNoteTool() *XHSReadNoteTool {
	return &XHSReadNoteTool{
		provider: realProvider{},
		cookies:  NewCookieLoader("", "default", "default"),
		client:   xhs.NewClient(),
	}
}

func (t *XHSReadNoteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *XHSReadNoteTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	ref := firstNonEmpty(
		asString(args["url"]),
		asString(args["link"]),
		asString(args["note_id"]),
	)
	if ref == "" {
		return "", fmt.Errorf("url/link/note_id 是必需的（传入小红书笔记链接或笔记ID）")
	}

	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("获取浏览器失败: %w", err)
	}
	if release {
		defer b.Close()
	}

	injectXHSCookies(ctx, b, t.cookies, resolveXHSURL(ref))

	note, err := t.client.ReadNote(ctx, xhsBrowserAdapter{b}, ref)
	if err != nil {
		return "", err
	}
	return xhs.FormatNote(note), nil
}

// ------------------------------------------------------------
// xhs_search
// ------------------------------------------------------------

// XHSSearchTool searches Xiaohongshu notes by keyword.
type XHSSearchTool struct {
	provider browserProvider
	cookies  cookieLoader
	client   *xhs.Client
}

// NewXHSSearchTool creates the production search tool.
func NewXHSSearchTool() *XHSSearchTool {
	return &XHSSearchTool{
		provider: realProvider{},
		cookies:  NewCookieLoader("", "default", "default"),
		client:   xhs.NewClient(),
	}
}

func (t *XHSSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *XHSSearchTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	keyword := asString(args["keyword"])
	if keyword == "" {
		return "", fmt.Errorf("keyword 是必需的")
	}
	page := 1
	if v, ok := args["page"].(float64); ok && v > 0 {
		page = int(v)
	}
	sort := asString(args["sort"])

	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("获取浏览器失败: %w", err)
	}
	if release {
		defer b.Close()
	}

	injectXHSCookies(ctx, b, t.cookies, xhs.HomeURL())

	res, err := t.client.Search(ctx, xhsBrowserAdapter{b}, keyword, page, sort)
	// Hard failure (no result at all) -> propagate as error.
	if err != nil && res == nil {
		return "", err
	}
	// Success or blocked-with-diagnostics -> return the formatted result so the
	// LLM sees the diagnostic (attempt / http status / xhs error code) instead
	// of a bare error string.
	return xhs.FormatSearch(res), nil
}

// ------------------------------------------------------------
// small arg helpers
// ------------------------------------------------------------

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
