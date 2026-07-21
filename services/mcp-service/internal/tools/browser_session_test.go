package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-platform/pkg/browseragent"
)

// --- fakes ---

type fakeBrowserOps struct {
	started    bool
	closed     bool
	actions    []*browseragent.Action
	setCookies []browseragent.Cookie
	state      *browseragent.PageState
	stateErr   error
	actionErr  error
	startErr   error
	evalResult string
	evalErr    error
}

func (f *fakeBrowserOps) Start(context.Context) error { f.started = true; return f.startErr }
func (f *fakeBrowserOps) Close()                      { f.closed = true }
func (f *fakeBrowserOps) GetState(context.Context) (*browseragent.PageState, error) {
	return f.state, f.stateErr
}
func (f *fakeBrowserOps) ExecuteAction(_ context.Context, a *browseragent.Action) (string, error) {
	cp := *a
	f.actions = append(f.actions, &cp)
	return "ok", f.actionErr
}
func (f *fakeBrowserOps) SetCookies(c []browseragent.Cookie) { f.setCookies = c }
func (f *fakeBrowserOps) Eval(_ context.Context, _ string) (string, error) {
	return f.evalResult, f.evalErr
}
func (f *fakeBrowserOps) EvalAsync(_ context.Context, _ string) (string, error) {
	return f.evalResult, f.evalErr
}
func (f *fakeBrowserOps) PressEnter(_ context.Context) error { return nil }
func (f *fakeBrowserOps) InjectInitScript(_ context.Context, _ string) (string, error) {
	return "fake-script-id", nil
}

// fakeProvider returns a fresh fake browser per call and records how many of
// each kind it handed out. Used to test executor argument parsing and the
// one-shot vs managed release contract without the session manager.
type fakeProvider struct {
	managedBrowsers []*fakeBrowserOps
	oneShotBrowsers []*fakeBrowserOps
	managedErr      error
	oneShotErr      error
}

func (p *fakeProvider) Managed(string) (browserOps, error) {
	if p.managedErr != nil {
		return nil, p.managedErr
	}
	b := &fakeBrowserOps{}
	p.managedBrowsers = append(p.managedBrowsers, b)
	return b, nil
}

func (p *fakeProvider) OneShot() (browserOps, error) {
	if p.oneShotErr != nil {
		return nil, p.oneShotErr
	}
	b := &fakeBrowserOps{}
	p.oneShotBrowsers = append(p.oneShotBrowsers, b)
	return b, nil
}

type fakeCookieLoader struct {
	cookies []browseragent.Cookie
	called  bool
	url     string
}

func (f *fakeCookieLoader) LoadCookiesForURL(_ context.Context, url string) ([]browseragent.Cookie, error) {
	f.called = true
	f.url = url
	return f.cookies, nil
}

func withSession(config map[string]interface{}, sid string) map[string]interface{} {
	if config == nil {
		config = map[string]interface{}{}
	}
	config["extra"] = map[string]interface{}{"session_id": sid}
	return config
}

// --- BrowserSessionManager ---

func newTestManager(factory func(ctx context.Context) (browserOps, error)) *BrowserSessionManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BrowserSessionManager{
		sessions:   make(map[string]*sessionEntry),
		ctx:        ctx,
		cancel:     cancel,
		ttl:        15 * time.Minute,
		newBrowser: factory,
	}
}

func TestManager_GetOrCreate_ReusesAcrossCalls(t *testing.T) {
	calls := 0
	factory := func(context.Context) (browserOps, error) {
		calls++
		return &fakeBrowserOps{}, nil
	}
	m := newTestManager(factory)

	first, err := m.GetOrCreate("sess-1")
	if err != nil {
		t.Fatalf("first GetOrCreate: %v", err)
	}
	second, err := m.GetOrCreate("sess-1")
	if err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	if first != second {
		t.Error("same sessionID should return the same browser instance")
	}
	if calls != 1 {
		t.Errorf("factory should be called once for reuse, got %d", calls)
	}
}

func TestManager_DifferentSessions_DifferentBrowsers(t *testing.T) {
	m := newTestManager(func(context.Context) (browserOps, error) {
		return &fakeBrowserOps{}, nil
	})
	a, _ := m.GetOrCreate("sess-a")
	b, _ := m.GetOrCreate("sess-b")
	if a == b {
		t.Error("different sessionIDs must yield different browsers")
	}
}

func TestManager_EmptySessionID(t *testing.T) {
	m := newTestManager(func(context.Context) (browserOps, error) {
		t.Fatal("factory must not be called for empty sessionID")
		return nil, nil
	})
	if _, err := m.GetOrCreate(""); err == nil {
		t.Error("expected error for empty sessionID")
	}
}

func TestManager_TTLRelease(t *testing.T) {
	fake := &fakeBrowserOps{}
	m := newTestManager(func(context.Context) (browserOps, error) { return fake, nil })
	m.ttl = 1 * time.Millisecond

	if _, err := m.GetOrCreate("sess-1"); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	// Backdate lastUsed so the entry is older than ttl.
	m.mu.Lock()
	m.sessions["sess-1"].lastUsed = time.Now().Add(-1 * time.Hour)
	m.mu.Unlock()

	m.reapIdle()

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions["sess-1"]; ok {
		t.Error("idle session should have been reaped")
	}
	if !fake.closed {
		t.Error("reaped browser should have been closed")
	}
}

// --- executors: argument parsing + primitive dispatch ---

func TestBrowserNavigateTool(t *testing.T) {
	p := &fakeProvider{}
	cl := &fakeCookieLoader{cookies: []browseragent.Cookie{{Name: "k", Value: "v"}}}
	tool := &BrowserNavigateTool{provider: p, cookies: cl}

	out, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"url": "https://example.com"}, withSession(nil, "s1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Errorf("output = %q, want ok", out)
	}
	if len(p.managedBrowsers) != 1 || p.managedBrowsers[0].closed {
		t.Error("managed browser should be acquired and NOT closed")
	}
	got := p.managedBrowsers[0].actions
	if len(got) != 1 || got[0].Type != browseragent.ActionNavigate || got[0].URL != "https://example.com" {
		t.Errorf("navigate action mismatch: %+v", got)
	}
	if len(p.managedBrowsers[0].setCookies) != 1 {
		t.Error("cookies from loader should be injected before navigate")
	}
	if !cl.called || cl.url != "https://example.com" {
		t.Error("cookie loader not invoked with the target url")
	}
}

func TestBrowserNavigateTool_MissingURL(t *testing.T) {
	tool := &BrowserNavigateTool{provider: &fakeProvider{}, cookies: &fakeCookieLoader{}}
	if _, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, nil); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestBrowserClickTool(t *testing.T) {
	p := &fakeProvider{}
	tool := &BrowserClickTool{provider: p}
	if _, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"element": float64(5)}, withSession(nil, "s1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.managedBrowsers[0].actions
	if len(got) != 1 || got[0].Type != browseragent.ActionClick || got[0].Element != 5 {
		t.Errorf("click action mismatch: %+v", got)
	}
}

func TestBrowserClickTool_MissingElement(t *testing.T) {
	tool := &BrowserClickTool{provider: &fakeProvider{}}
	if _, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, nil); err == nil {
		t.Error("expected error for missing element")
	}
}

func TestBrowserTypeTool(t *testing.T) {
	p := &fakeProvider{}
	tool := &BrowserTypeTool{provider: p}
	if _, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"element": float64(2), "text": "hello"}, withSession(nil, "s1")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.managedBrowsers[0].actions
	if len(got) != 1 || got[0].Type != browseragent.ActionInput || got[0].Element != 2 || got[0].Text != "hello" {
		t.Errorf("type action mismatch: %+v", got)
	}
}

func TestBrowserScrollTool(t *testing.T) {
	t.Run("explicit up", func(t *testing.T) {
		p := &fakeProvider{}
		tool := &BrowserScrollTool{provider: p}
		if _, err := tool.ExecuteWithConfig(context.Background(),
			map[string]interface{}{"direction": "up"}, withSession(nil, "s1")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := p.managedBrowsers[0].actions
		if got[0].Type != browseragent.ActionScroll || got[0].Direction != "up" {
			t.Errorf("scroll action mismatch: %+v", got)
		}
	})
	t.Run("defaults to down", func(t *testing.T) {
		p := &fakeProvider{}
		tool := &BrowserScrollTool{provider: p}
		if _, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, withSession(nil, "s1")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := p.managedBrowsers[0].actions[0]; got.Direction != "down" {
			t.Errorf("default direction = %q, want down", got.Direction)
		}
	})
	t.Run("rejects invalid", func(t *testing.T) {
		tool := &BrowserScrollTool{provider: &fakeProvider{}}
		if _, err := tool.ExecuteWithConfig(context.Background(),
			map[string]interface{}{"direction": "sideways"}, nil); err == nil {
			t.Error("expected error for invalid direction")
		}
	})
}

func TestBrowserWaitTool(t *testing.T) {
	t.Run("explicit seconds", func(t *testing.T) {
		p := &fakeProvider{}
		tool := &BrowserWaitTool{provider: p}
		if _, err := tool.ExecuteWithConfig(context.Background(),
			map[string]interface{}{"seconds": float64(3)}, withSession(nil, "s1")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := p.managedBrowsers[0].actions[0]; got.Type != browseragent.ActionWait || got.Seconds != 3 {
			t.Errorf("wait action mismatch: %+v", got)
		}
	})
	t.Run("defaults to 1", func(t *testing.T) {
		p := &fakeProvider{}
		tool := &BrowserWaitTool{provider: p}
		if _, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, withSession(nil, "s1")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := p.managedBrowsers[0].actions[0]; got.Seconds != 1 {
			t.Errorf("default seconds = %d, want 1", got.Seconds)
		}
	})
}

func TestBrowserExtractTool_ReturnsElementsWithIndices(t *testing.T) {
	state := &browseragent.PageState{
		URL:   "https://example.com",
		Title: "Example",
		Elements: []browseragent.Element{
			{Index: 0, Tag: "a", Text: "Login"},
			{Index: 1, Tag: "input", Text: "Search"},
		},
		Text: "welcome to the page",
	}
	tool := &BrowserExtractTool{provider: &stateProvider{state: state}}

	out, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, withSession(nil, "s1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"URL: https://example.com", "Title: Example", "[0] a: Login", "[1] input: Search", "welcome to the page"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

// stateProvider returns a single fake browser preloaded with a page state.
type stateProvider struct {
	state   *browseragent.PageState
	browser *fakeBrowserOps
}

func (p *stateProvider) Managed(string) (browserOps, error) {
	if p.browser == nil {
		p.browser = &fakeBrowserOps{state: p.state}
	}
	return p.browser, nil
}
func (p *stateProvider) OneShot() (browserOps, error) {
	return &fakeBrowserOps{state: p.state}, nil
}

// --- one-shot vs managed release contract ---

func TestOneShotFallback_NoSessionID(t *testing.T) {
	p := &fakeProvider{}
	tool := &BrowserClickTool{provider: p}
	if _, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"element": float64(0)}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.managedBrowsers) != 0 {
		t.Error("no session_id must not use the managed path")
	}
	if len(p.oneShotBrowsers) != 1 {
		t.Fatalf("expected one one-shot browser, got %d", len(p.oneShotBrowsers))
	}
	if !p.oneShotBrowsers[0].closed {
		t.Error("one-shot browser must be closed after execution")
	}
}

func TestManagedBrowser_NotReleased(t *testing.T) {
	p := &fakeProvider{}
	tool := &BrowserClickTool{provider: p}
	if _, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"element": float64(0)}, withSession(nil, "sess-9")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.oneShotBrowsers) != 0 {
		t.Error("session_id present must not use the one-shot path")
	}
	if len(p.managedBrowsers) != 1 {
		t.Fatalf("expected one managed browser, got %d", len(p.managedBrowsers))
	}
	if p.managedBrowsers[0].closed {
		t.Error("managed browser must NOT be closed (TTL reaps it)")
	}
}

func TestReadSessionID(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no extra", map[string]interface{}{"api_key": "k"}, ""},
		{"extra without session_id", map[string]interface{}{"extra": map[string]interface{}{"foo": "bar"}}, ""},
		{"extra with session_id", map[string]interface{}{"extra": map[string]interface{}{"session_id": "s1"}}, "s1"},
	}
	for _, c := range cases {
		if got := readSessionID(c.cfg); got != c.want {
			t.Errorf("%s: readSessionID = %q, want %q", c.name, got, c.want)
		}
	}
}
