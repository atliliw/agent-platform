package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-platform/pkg/browseragent"
)

// ============================================================
// Session continuity for fine-grained browser tools
//
// The six browser primitives (navigate/click/type/extract/scroll/wait) are
// designed to be chained within one chat session: navigate opens a page, click
// acts on it, extract reads it back. For that to work, every call in the chain
// must drive the *same* browser page so cookies and login state survive.
//
// BrowserSessionManager holds one pooled browser per session_id, reused across
// calls and reaped after a TTL of idleness. Callers without a session_id get a
// one-shot browser (acquired + released per call, no continuity but still
// functional) so the tools degrade gracefully on code paths that never set a
// session (e.g. chat-service local loop).
// ============================================================

// browserOps is the subset of *browseragent.Browser the executors depend on.
// Declaring it as an interface lets tests inject a fake instead of launching
// Chrome.
type browserOps interface {
	Start(ctx context.Context) error
	Close()
	GetState(ctx context.Context) (*browseragent.PageState, error)
	ExecuteAction(ctx context.Context, action *browseragent.Action) (string, error)
	SetCookies([]browseragent.Cookie)
	// Eval runs JS in the page and returns its (awaited) value as a string.
	// Used by the XHS tools to sign + fetch XHS APIs from inside the page.
	Eval(ctx context.Context, js string) (string, error)
	// EvalAsync runs JS that returns a Promise, awaits it, and returns the
	// resolved value as a string. Pumps the page event loop so async fetch
	// callbacks actually complete (plain Eval leaves the loop frozen on Obscura).
	EvalAsync(ctx context.Context, js string) (string, error)
	// PressEnter dispatches a real (isTrusted) Enter key to the focused element
	// via the CDP Input domain. Bypasses isTrusted guards on synthetic events.
	PressEnter(ctx context.Context) error
	// InjectInitScript injects JS that runs before each page's own scripts
	// (Page.addScriptToEvaluateOnNewDocument), so fetch/XHR patching can capture
	// requests fired during load. Returns the script id.
	InjectInitScript(ctx context.Context, source string) (string, error)
}

// browserProvider abstracts how an executor obtains a browser so the executor
// logic is testable without the session manager or the real pool.
type browserProvider interface {
	// Managed returns a browser bound to sessionID, reused across calls. The
	// caller must NOT Close it - the session manager reaps it on idle TTL.
	Managed(sessionID string) (browserOps, error)
	// OneShot returns a throwaway browser the caller MUST Close after use.
	OneShot() (browserOps, error)
}

// sessionEntry pairs a browser with its last-used timestamp for TTL reaping.
type sessionEntry struct {
	browser  browserOps
	lastUsed time.Time
}

// BrowserSessionManager keeps one browser per session_id for the lifetime of a
// chat session, so a navigate -> click -> extract chain shares one page.
type BrowserSessionManager struct {
	mu         sync.Mutex
	sessions   map[string]*sessionEntry
	ctx        context.Context
	cancel     context.CancelFunc
	ttl        time.Duration
	newBrowser func(ctx context.Context) (browserOps, error)
}

var (
	sessionMgrOnce sync.Once
	sessionMgr     *BrowserSessionManager
)

// GetBrowserSessionManager returns the process-wide session manager singleton.
// The browser factory defaults to the pooled browser (NewBrowserFromPool) and
// the TTL defaults to 15 minutes.
func GetBrowserSessionManager() *BrowserSessionManager {
	sessionMgrOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		sessionMgr = &BrowserSessionManager{
			sessions:   make(map[string]*sessionEntry),
			ctx:        ctx,
			cancel:     cancel,
			ttl:        15 * time.Minute,
			newBrowser: newPooledBrowser,
		}
		go sessionMgr.ttlLoop()
	})
	return sessionMgr
}

// newPooledBrowser acquires a browser from the global pool and starts it.
// Start is a no-op for pooled browsers but keeps the interface honest for any
// future non-pooled factory.
func newPooledBrowser(ctx context.Context) (browserOps, error) {
	b, err := browseragent.NewBrowserFromPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire from pool: %w", err)
	}
	if err := b.Start(ctx); err != nil {
		b.Close()
		return nil, fmt.Errorf("start browser: %w", err)
	}
	return b, nil
}

// GetOrCreate returns the browser for sessionID, creating one on first use.
// Subsequent calls with the same sessionID reuse the same browser instance.
func (m *BrowserSessionManager) GetOrCreate(sessionID string) (browserOps, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.sessions[sessionID]; ok {
		entry.lastUsed = time.Now()
		return entry.browser, nil
	}
	b, err := m.newBrowser(m.ctx)
	if err != nil {
		return nil, err
	}
	m.sessions[sessionID] = &sessionEntry{browser: b, lastUsed: time.Now()}
	return b, nil
}

// Release closes the browser for sessionID and drops the entry. Useful for
// explicit teardown; the TTL loop also handles this automatically on idle.
func (m *BrowserSessionManager) Release(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.sessions[sessionID]; ok {
		entry.browser.Close()
		delete(m.sessions, sessionID)
	}
}

// Close shuts down every session browser and stops the TTL loop. Intended for
// process shutdown.
func (m *BrowserSessionManager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, entry := range m.sessions {
		entry.browser.Close()
		delete(m.sessions, id)
	}
}

// ttlLoop periodically reaps browsers idle for longer than ttl.
func (m *BrowserSessionManager) ttlLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.reapIdle()
		}
	}
}

// reapIdle closes and removes sessions whose lastUsed exceeds the TTL.
func (m *BrowserSessionManager) reapIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, entry := range m.sessions {
		if now.Sub(entry.lastUsed) > m.ttl {
			entry.browser.Close()
			delete(m.sessions, id)
		}
	}
}

// realProvider is the production browserProvider: managed calls go through the
// session singleton, one-shot calls acquire directly from the pool.
type realProvider struct{}

func (realProvider) Managed(sessionID string) (browserOps, error) {
	return GetBrowserSessionManager().GetOrCreate(sessionID)
}

func (realProvider) OneShot() (browserOps, error) {
	return newPooledBrowser(context.Background())
}

// readSessionID extracts the session id the engine injected into Extra.
func readSessionID(config map[string]interface{}) string {
	if config == nil {
		return ""
	}
	extra, ok := config["extra"].(map[string]interface{})
	if !ok {
		return ""
	}
	if s, ok := extra["session_id"].(string); ok {
		return s
	}
	return ""
}

// acquireBrowser returns a browser plus a flag telling the caller whether it
// must Close the browser itself. Managed (session-bound) browsers are released
// by the TTL loop, so shouldRelease is false; one-shot browsers must be closed
// by the caller.
func acquireBrowser(p browserProvider, config map[string]interface{}) (browserOps, bool, error) {
	if sid := readSessionID(config); sid != "" {
		b, err := p.Managed(sid)
		return b, false, err
	}
	b, err := p.OneShot()
	return b, true, err
}

// cookieLoader is the part of *CookieLoader navigate uses to auto-inject
// stored cookies for the target domain (same behavior as browser_execute).
type cookieLoader interface {
	LoadCookiesForURL(ctx context.Context, url string) ([]browseragent.Cookie, error)
}

// ============================================================
// browser_navigate
// ============================================================

// BrowserNavigateTool opens a URL. On first use in a session it auto-loads
// stored cookies for the URL's domain, matching browser_execute behavior.
type BrowserNavigateTool struct {
	provider browserProvider
	cookies  cookieLoader
}

// NewBrowserNavigateTool creates the production navigate tool.
func NewBrowserNavigateTool() *BrowserNavigateTool {
	return &BrowserNavigateTool{provider: realProvider{}, cookies: NewCookieLoader("", "default", "default")}
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserNavigateTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	if t.cookies != nil {
		if cks, _ := t.cookies.LoadCookiesForURL(ctx, url); len(cks) > 0 {
			b.SetCookies(cks)
		}
	}
	return b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionNavigate, URL: url})
}

// ============================================================
// browser_click
// ============================================================

// BrowserClickTool clicks the interactive element at the given index, where the
// index comes from the elements list returned by browser_extract.
type BrowserClickTool struct{ provider browserProvider }

func NewBrowserClickTool() *BrowserClickTool { return &BrowserClickTool{provider: realProvider{}} }

func (t *BrowserClickTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserClickTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	idx, ok := intArg(args, "element")
	if !ok {
		return "", fmt.Errorf("element (index) is required")
	}
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	return b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionClick, Element: idx})
}

// ============================================================
// browser_type
// ============================================================

// BrowserTypeTool types text into the interactive element at the given index.
type BrowserTypeTool struct{ provider browserProvider }

func NewBrowserTypeTool() *BrowserTypeTool { return &BrowserTypeTool{provider: realProvider{}} }

func (t *BrowserTypeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserTypeTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	idx, ok := intArg(args, "element")
	if !ok {
		return "", fmt.Errorf("element (index) is required")
	}
	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	return b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionInput, Element: idx, Text: text})
}

// ============================================================
// browser_scroll
// ============================================================

// BrowserScrollTool scrolls the page up or down by one viewport.
type BrowserScrollTool struct{ provider browserProvider }

func NewBrowserScrollTool() *BrowserScrollTool { return &BrowserScrollTool{provider: realProvider{}} }

func (t *BrowserScrollTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserScrollTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	direction, _ := args["direction"].(string)
	if direction == "" {
		direction = "down"
	}
	if direction != "up" && direction != "down" {
		return "", fmt.Errorf("direction must be 'up' or 'down', got %q", direction)
	}
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	return b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionScroll, Direction: direction})
}

// ============================================================
// browser_wait
// ============================================================

// BrowserWaitTool pauses for the given number of seconds (default 1).
type BrowserWaitTool struct{ provider browserProvider }

func NewBrowserWaitTool() *BrowserWaitTool { return &BrowserWaitTool{provider: realProvider{}} }

func (t *BrowserWaitTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserWaitTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	seconds := 1
	if s, ok := intArg(args, "seconds"); ok {
		seconds = s
	}
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	return b.ExecuteAction(ctx, &browseragent.Action{Type: browseragent.ActionWait, Seconds: seconds})
}

// ============================================================
// browser_extract
// ============================================================

// BrowserExtractTool reads the current page state: URL, title, the indexed list
// of interactive elements (so the LLM can pick click/type targets), and a
// snippet of page text.
type BrowserExtractTool struct{ provider browserProvider }

func NewBrowserExtractTool() *BrowserExtractTool {
	return &BrowserExtractTool{provider: realProvider{}}
}

func (t *BrowserExtractTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

func (t *BrowserExtractTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	b, release, err := acquireBrowser(t.provider, config)
	if err != nil {
		return "", fmt.Errorf("acquire browser: %w", err)
	}
	if release {
		defer b.Close()
	}
	state, err := b.GetState(ctx)
	if err != nil {
		return "", fmt.Errorf("get state: %w", err)
	}
	return formatPageState(state), nil
}

// formatPageState renders a PageState as the text returned to the LLM. Element
// indices are preserved so the LLM can pass them back to browser_click /
// browser_type. All output is sanitized to valid UTF-8 because gRPC requires
// valid UTF-8 string fields and stealth-engine page text occasionally contains
// invalid byte sequences.
func formatPageState(s *browseragent.PageState) string {
	vu := func(str string) string { return strings.ToValidUTF8(str, "�") }
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("URL: %s\n", vu(s.URL)))
	sb.WriteString(fmt.Sprintf("Title: %s\n", vu(s.Title)))
	sb.WriteString(fmt.Sprintf("\nInteractive elements (%d):\n", len(s.Elements)))
	for _, el := range s.Elements {
		text := el.Text
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", el.Index, vu(el.Tag), vu(text)))
	}
	text := strings.TrimSpace(s.Text)
	if len(text) > 2000 {
		text = text[:2000] + "\n...(truncated)"
	}
	sb.WriteString("\nPage text:\n")
	sb.WriteString(vu(text))
	return sb.String()
}

// intArg reads an integer argument that may arrive as float64 (the default for
// JSON numbers decoded into map[string]interface{}).
func intArg(args map[string]interface{}, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
