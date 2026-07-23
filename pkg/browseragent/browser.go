// Package browseragent provides browser control using chromedp
package browseragent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// resolveBrowserWSURL resolves the WebSocket debugger URL for a remote browser.
// A ws:// / wss:// URL is returned verbatim (Obscura accepts the bare
// /devtools/browser path). An http(s):// URL means a plain headed-Chrome
// container: plain Chrome requires the full ws://host:port/devtools/browser/<id>
// path and does NOT accept the bare path, so the browser id is discovered via
// /json/version. The advertised host (127.0.0.1) is rewritten to the host in
// rawURL so the URL is reachable cross-container.
func resolveBrowserWSURL(ctx context.Context, rawURL string) (string, error) {
	if strings.HasPrefix(rawURL, "ws://") || strings.HasPrefix(rawURL, "wss://") {
		return rawURL, nil
	}
	base := strings.TrimRight(rawURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/version", nil)
	if err != nil {
		return "", fmt.Errorf("build /json/version request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("query %s/json/version: %w", base, err)
	}
	defer resp.Body.Close()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", fmt.Errorf("decode /json/version: %w", err)
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("no webSocketDebuggerUrl at %s/json/version", base)
	}
	// Replace the ws URL's host:port (Chrome advertises 127.0.0.1:<port>,
	// unreachable cross-container) with the host:port from rawURL. Using
	// url.Parse handles the host:port as a unit so the port is not doubled.
	if u, e := url.Parse(rawURL); e == nil && u.Host != "" {
		if wu, e2 := url.Parse(v.WebSocketDebuggerURL); e2 == nil {
			wu.Host = u.Host
			v.WebSocketDebuggerURL = wu.String()
		}
	}
	return v.WebSocketDebuggerURL, nil
}

// Browser manages a browser instance using chromedp
type Browser struct {
	ctx       context.Context
	cancel    context.CancelFunc
	allocator context.Context
	started   bool
	cookies   []Cookie // 预注入的 Cookie
	stealth   bool     // true when connected to the Obscura stealth CDP server

	// Pool mode flags
	fromPool bool           // 是否从池中获取
	pool     *BrowserPool   // 所属的池（如果是从池中获取）
	pooled   *PooledBrowser // 池化的浏览器实例
}

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	return &Browser{}
}

// NewBrowserFromPool creates a browser instance from the global pool
// This is much faster than NewBrowser() as it reuses existing browser processes
func NewBrowserFromPool(ctx context.Context) (*Browser, error) {
	pool := GetBrowserPool()
	pb, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire from pool: %w", err)
	}

	return &Browser{
		ctx:       pb.ctx,
		cancel:    pb.cancel,
		allocator: pb.allocator,
		started:   true,
		stealth:   pb.stealth, // propagate Obscura stealth flag from the pooled browser
		fromPool:  true,
		pool:      pool,
		pooled:    pb,
	}, nil
}

// SetCookies sets cookies to be injected before navigation
func (b *Browser) SetCookies(cookies []Cookie) {
	b.cookies = cookies
}

// Start starts the browser
func (b *Browser) Start(ctx context.Context) error {
	// 如果是从池中获取的浏览器，已经启动，直接返回
	if b.started {
		return nil
	}

	// Check if using a remote CDP browser (Obscura stealth engine, or a plain
	// headed-Chrome container for sites whose anti-bot needs a non-headless
	// browser - e.g. Xiaohongshu search, which can only generate its x-s-common
	// signature when the page's event loop runs normally).
	obscuraURL := os.Getenv("OBSCURA_CDP_URL")
	if obscuraURL != "" {
		// Resolve the ws URL: ws:// is used directly (Obscura accepts the bare
		// path); http:// triggers /json/version discovery for plain Chrome.
		wsURL, err := resolveBrowserWSURL(ctx, obscuraURL)
		if err != nil {
			return fmt.Errorf("resolve browser WS URL from %s: %w", obscuraURL, err)
		}
		allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, wsURL, chromedp.NoModifyURL)
		b.allocator = allocCtx
		b.cancel = allocCancel

		browserCtx, _ := chromedp.NewContext(allocCtx)
		// Obscura does not fire Page.frameStoppedLoading, so chromedp's Navigate
		// (which blocks on that event) hangs. Prime the target with a trivial
		// evaluate instead of Navigate+WaitReady. Plain headed-Chrome fires the
		// event, but the prime works for both.
		var ua string
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(`navigator.userAgent`, &ua)); err != nil {
			b.cancel()
			b.started = false
			return fmt.Errorf("connect to CDP server at %s: %w", obscuraURL, err)
		}
		b.ctx = browserCtx
		b.started = true
		b.stealth = true
		return nil
	}

	// Fallback: use local Chrome/Chromium (original behavior)
	// Get Chrome executable path from environment or auto-detect
	chromePath := os.Getenv("CHROME_BIN")
	if chromePath == "" {
		// Auto-detect Chrome/Chromium path
		chromePath = detectChromePath()
	}

	// Check if we should run in headless mode (default: true)
	headless := os.Getenv("CHROME_HEADLESS") != "false"

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("disable-extensions", false), // 改为 false，有些网站检测这个
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.WindowSize(1920, 1080),
		// === 反检测选项 ===
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		// 添加更多反检测 flags
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-web-security", true), // 禁用同源策略检查（仅用于测试）
		chromedp.Flag("allow-running-insecure-content", true),
		// 语言和平台设置
		chromedp.Flag("lang", "zh-CN"),
		chromedp.Flag("accept-lang", "zh-CN,zh;q=0.9,en;q=0.8"),
	)

	// Create allocator with timeout
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	b.allocator = allocCtx
	b.cancel = allocCancel

	// Create browser context
	browserCtx, _ := chromedp.NewContext(allocCtx)

	// Initialize browser by navigating to a blank page
	// This ensures the browser is ready before we try to get state
	err := chromedp.Run(browserCtx,
		chromedp.Navigate("about:blank"),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		b.cancel()
		b.started = false
		return fmt.Errorf("initialize browser: %w", err)
	}

	// === 在空白页面设置 Navigator 属性来隐藏自动化特征 ===
	// 这些属性会在后续页面中保留
	_ = chromedp.Run(browserCtx,
		chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', {
				get: () => undefined
			});
			Object.defineProperty(navigator, 'plugins', {
				get: () => [1, 2, 3, 4, 5]
			});
			Object.defineProperty(navigator, 'languages', {
				get: () => ['zh-CN', 'zh', 'en']
			});
			window.chrome = {
				runtime: {}
			};
		`, nil),
	)

	b.ctx = browserCtx
	b.started = true
	return nil
}

// Close closes the browser
func (b *Browser) Close() {
	// 如果是从池中获取的，归还到池中而不是关闭
	if b.fromPool && b.pool != nil && b.pooled != nil {
		b.pool.Release(b.pooled)
		b.pooled = nil
		return
	}

	if b.cancel != nil {
		b.cancel()
	}
}

// GetState gets current page state
func (b *Browser) GetState(ctx context.Context) (*PageState, error) {
	if b.stealth {
		return b.getStateStealth()
	}
	// Local Chromium path (DOM domain via chromedp.OuterHTML works fine here).
	var url, title, html string
	err := chromedp.Run(b.ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, fmt.Errorf("get page state: %w", err)
	}
	return &PageState{
		URL:      url,
		Title:    title,
		Elements: b.parseElements(html),
		Text:     b.extractText(html),
	}, nil
}

// getStateStealth reads page state via a single Runtime.evaluate. On Obscura:
//   - the DOM domain (chromedp.OuterHTML) hangs, and
//   - returning the full outerHTML (~1MB on a real page) via returnByValue also
//     hangs,
//
// so url/title/text/elements are extracted directly in JS. This is the generic
// page-state reader; site-specific extraction (XHS notes/search) lives in the
// dedicated pkg/xhs tools, not here.
func (b *Browser) getStateStealth() (*PageState, error) {
	const js = `(function(){
		function cssPath(el){
			if (el.id) return '#'+el.id;
			var parts=[]; var cur=el;
			while (cur && cur.nodeType===1 && parts.length<5){
				var part=cur.tagName.toLowerCase();
				if (cur.className && typeof cur.className==='string'){
					var cls=cur.className.trim().split(/\s+/);
					for (var i=0;i<cls.length;i++){var c=cls[i]; if(c.length>2 && c[0]!=='_'){part+='.'+c; break;}}
				}
				parts.unshift(part); cur=cur.parentElement;
			}
			return parts.join('>') || el.tagName.toLowerCase();
		}
		var sels='a, button, input, select, textarea, [onclick], [role="button"], [role="link"], [role="textbox"], [contenteditable="true"], [data-role], .btn, .button';
		var nodes=Array.from(document.querySelectorAll(sels)).slice(0,150);
		var elements=[];
		for (var i=0;i<nodes.length;i++){
			var el=nodes[i];
			if (el.getAttribute && el.getAttribute('type')==='hidden') continue;
			var t=(el.innerText||el.placeholder||el.title||el.getAttribute('aria-label')||el.value||'').toString().trim();
			if (t.length>100) t=t.substring(0,100);
			elements.push({index:elements.length, tag:el.tagName.toLowerCase(), text:t, selector:cssPath(el)});
		}
		var text=document.body?document.body.innerText:'';
		return JSON.stringify({url:location.href, title:document.title, text:text, elements:elements});
	})()`
	var raw string
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(js, &raw)); err != nil {
		return nil, fmt.Errorf("get page state: %w", err)
	}
	var s struct {
		URL      string
		Title    string
		Text     string
		Elements []Element
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("parse page state: %w", err)
	}
	return &PageState{URL: s.URL, Title: s.Title, Elements: s.Elements, Text: s.Text}, nil
}

// ExecuteAction executes a browser action
func (b *Browser) ExecuteAction(ctx context.Context, action *Action) (string, error) {
	switch action.Type {
	case ActionNavigate:
		return b.navigate(action.URL)
	case ActionClick:
		return b.click(action.Element)
	case ActionInput:
		return b.typeText(action.Element, action.Text)
	case ActionScroll:
		return b.scroll(action.Direction)
	case ActionWait:
		return b.wait(action.Seconds)
	case ActionEditorInput:
		return b.setEditorContent(action.Text)
	case ActionEditorTitle:
		return b.setEditorTitle(action.Text)
	case ActionPublish:
		return b.clickPublishButton()
	case ActionExecuteJS:
		return b.executeJavaScript(action.JavaScript)
	default:
		return "", fmt.Errorf("unknown action: %s", action.Type)
	}
}

// stealthNavigate sends Page.navigate without waiting for the load event.
// Obscura's CDP server does not fire Page.frameStoppedLoading, so chromedp's
// Navigate action (which blocks on that event) hangs forever. Fire-and-forget
// plus a caller-supplied sleep lets the page render. Navigation-level failures
// (e.g. DNS errors) are surfaced via errText.
func (b *Browser) stealthNavigate(url string) error {
	return chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, errText, e := page.Navigate(url).Do(ctx)
		if e != nil {
			return fmt.Errorf("page.Navigate %s: %w", url, e)
		}
		if errText != "" {
			return fmt.Errorf("page.Navigate %s: %s", url, errText)
		}
		return nil
	}))
}

// setPresetCookies injects b.cookies into the browser cookie jar for the given
// domain. This is cookie-level (network.SetCookie), so it works from any
// current page - no need to navigate to the domain root first.
func (b *Browser) setPresetCookies(domain string) {
	for _, c := range b.cookies {
		if err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			setCookie := network.SetCookie(c.Name, c.Value)
			cookieDomain := c.Domain
			if cookieDomain == "" {
				cookieDomain = domain
			}
			setCookie = setCookie.WithDomain(cookieDomain)
			if c.Path != "" {
				setCookie = setCookie.WithPath(c.Path)
			} else {
				setCookie = setCookie.WithPath("/")
			}
			if c.HTTPOnly {
				setCookie = setCookie.WithHTTPOnly(true)
			}
			if c.Secure {
				setCookie = setCookie.WithSecure(true)
			}
			return setCookie.Do(ctx)
		})); err != nil {
			fmt.Printf("Warning: failed to set cookie %s (domain=%s): %v\n", c.Name, c.Domain, err)
		}
	}
}

func (b *Browser) navigate(url string) (string, error) {
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	// Parse domain
	domain := extractDomain(url)

	// Obscura (stealth) path: fire-and-forget navigation. Obscura doesn't fire
	// load events, so chromedp.Navigate would hang; stealthNavigate sends
	// Page.navigate and returns immediately, then we sleep to let the page render.
	if b.stealth {
		if len(b.cookies) > 0 {
			b.setPresetCookies(domain)
			if err := b.stealthNavigate(url); err != nil {
				return "", fmt.Errorf("navigate: %w", err)
			}
			_ = chromedp.Run(b.ctx, chromedp.Sleep(4*time.Second))
		} else {
			if err := b.stealthNavigate(url); err != nil {
				return "", fmt.Errorf("navigate: %w", err)
			}
			_ = chromedp.Run(b.ctx, chromedp.Sleep(3*time.Second))
		}
		// Defensive anti-detection (Obscura is already stealth; harmless no-op).
		_ = chromedp.Run(b.ctx, chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
		`, nil))
		cookieInfo := ""
		if len(b.cookies) > 0 {
			cookieInfo = fmt.Sprintf(" (已注入 %d 个 Cookie)", len(b.cookies))
		}
		return fmt.Sprintf("已导航到 %s%s", url, cookieInfo), nil
	}

	// Local Chromium path (original behavior).
	// If we have preset cookies, first navigate to root domain, set cookies, then navigate to target
	if len(b.cookies) > 0 {
		// First navigate to root URL of domain (for proper cross-domain cookie setting)
		rootURL := "https://" + domain + "/"
		if err := chromedp.Run(b.ctx, chromedp.Navigate(rootURL)); err != nil {
			// If root fails, navigate directly to target
			if err := chromedp.Run(b.ctx, chromedp.Navigate(url)); err != nil {
				return "", fmt.Errorf("navigate: %w", err)
			}
		}

		// Wait for page load
		if err := chromedp.Run(b.ctx, chromedp.WaitReady("body")); err != nil {
			fmt.Printf("Warning: wait ready failed: %v\n", err)
		}

		// Wait extra time for page to stabilize
		_ = chromedp.Run(b.ctx, chromedp.Sleep(1*time.Second))

		// Inject all cookies
		b.setPresetCookies(domain)

		// Now navigate to target URL (cookies are already set)
		if err := chromedp.Run(b.ctx, chromedp.Navigate(url)); err != nil {
			return "", fmt.Errorf("navigate to target: %w", err)
		}

		// Wait for page load (increased timeout)
		if err := chromedp.Run(b.ctx, chromedp.WaitReady("body")); err != nil {
			fmt.Printf("Warning: wait ready after navigation: %v\n", err)
		}

		// Wait extra time for dynamic content
		_ = chromedp.Run(b.ctx, chromedp.Sleep(3*time.Second))

		// Inject anti-detection script after navigation
		_ = chromedp.Run(b.ctx,
			chromedp.Evaluate(`
				Object.defineProperty(navigator, 'webdriver', {
					get: () => undefined
				});
				window.chrome = { runtime: {} };
			`, nil),
		)
	} else {
		// No cookies, navigate directly
		err := chromedp.Run(b.ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body"),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			return "", fmt.Errorf("navigate: %w", err)
		}

		// Inject anti-detection script
		_ = chromedp.Run(b.ctx,
			chromedp.Evaluate(`
				Object.defineProperty(navigator, 'webdriver', {
					get: () => undefined
				});
			`, nil),
		)
	}

	cookieInfo := ""
	if len(b.cookies) > 0 {
		cookieInfo = fmt.Sprintf(" (已注入 %d 个 Cookie)", len(b.cookies))
	}

	return fmt.Sprintf("已导航到 %s%s", url, cookieInfo), nil
}

// extractDomain 从 URL 中提取域名
func extractDomain(url string) string {
	// 移除协议
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// 移除路径
	if idx := strings.Index(url, "/"); idx > 0 {
		url = url[:idx]
	}

	// 移除端口
	if idx := strings.Index(url, ":"); idx > 0 {
		url = url[:idx]
	}

	return url
}

func (b *Browser) click(elementIndex int) (string, error) {
	state, err := b.GetState(context.Background())
	if err != nil {
		return "", err
	}

	if elementIndex < 0 || elementIndex >= len(state.Elements) {
		return "", fmt.Errorf("invalid element index: %d", elementIndex)
	}

	element := state.Elements[elementIndex]

	// chromedp.Click is a QueryAction (DOM domain) which hangs on Obscura; use a
	// JS-based click via Runtime.evaluate instead.
	var clickRes string
	err = chromedp.Run(b.ctx,
		chromedp.Evaluate(fmt.Sprintf(`(function(){
			var el = document.querySelector(%q);
			if (!el) return 'element not found';
			el.click();
			return 'clicked';
		})()`, element.Selector), &clickRes),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return "", fmt.Errorf("click: %w", err)
	}

	return fmt.Sprintf("已点击元素[%d]: %s", elementIndex, clickRes), nil
}

func (b *Browser) typeText(elementIndex int, text string) (string, error) {
	state, err := b.GetState(context.Background())
	if err != nil {
		return "", err
	}

	if elementIndex < 0 || elementIndex >= len(state.Elements) {
		return "", fmt.Errorf("invalid element index: %d", elementIndex)
	}

	element := state.Elements[elementIndex]

	// 尝试使用 JavaScript 直接设置值（更可靠）
	var setValueScript string
	if element.Tag == "textarea" || element.Tag == "input" {
		// 对于 input 和 textarea，使用 value 设置
		setValueScript = fmt.Sprintf(`
			(function() {
				var el = document.querySelector('%s');
				if (!el) return 'element not found';
				el.value = %q;
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return 'success';
			})()
		`, element.Selector, text)
	} else {
		// 对于其他元素（如 contenteditable），使用 textContent 或 innerText
		setValueScript = fmt.Sprintf(`
			(function() {
				var el = document.querySelector('%s');
				if (!el) return 'element not found';
				if (el.isContentEditable || el.contentEditable === 'true') {
					el.innerText = %q;
				} else {
					el.value = %q;
				}
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return 'success';
			})()
		`, element.Selector, text, text)
	}

	var result string
	err = chromedp.Run(b.ctx,
		chromedp.Evaluate(setValueScript, &result),
	)

	if err != nil {
		if b.stealth {
			// chromedp.Click/SendKeys 是 QueryAction（DOM 域），在 Obscura 上会挂起。
			return "", fmt.Errorf("type (js): %w", err)
		}
		// 本地 Chromium：如果 JavaScript 方法失败，回退到传统方法
		err = chromedp.Run(b.ctx,
			chromedp.Click(element.Selector),
			chromedp.Sleep(100*time.Millisecond),
			chromedp.SendKeys(element.Selector, text),
		)
		if err != nil {
			return "", fmt.Errorf("type: %w", err)
		}
	}

	return fmt.Sprintf("已输入: %s", text), nil
}

func (b *Browser) scroll(direction string) (string, error) {
	scrollY := 500
	if direction == "up" {
		scrollY = -500
	}

	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(fmt.Sprintf("window.scrollBy(0, %d)", scrollY), nil),
	)
	if err != nil {
		return "", fmt.Errorf("scroll: %w", err)
	}

	return fmt.Sprintf("已滚动 %s", direction), nil
}

func (b *Browser) wait(seconds int) (string, error) {
	if seconds <= 0 {
		seconds = 1
	}

	err := chromedp.Run(b.ctx,
		chromedp.Sleep(time.Duration(seconds)*time.Second),
	)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("已等待 %d 秒", seconds), nil
}

// setEditorContent sets content in CSDN Markdown editor using JavaScript
func (b *Browser) setEditorContent(content string) (string, error) {
	// CSDN 编辑器可能有多种形式，尝试多种方式设置内容
	script := fmt.Sprintf(`
		(function() {
			var content = %q;

			// 方式1: 尝试 CKEditor (Markdown 编辑器)
			var ckeditor = document.querySelector('.cke_editor .cke_wysiwyg_frame');
			if (ckeditor) {
				var innerDoc = ckeditor.contentDocument || ckeditor.contentWindow.document;
				var body = innerDoc.body;
				if (body) {
					body.innerHTML = content.replace(/\n/g, '<br>');
					return 'CKEditor content set';
				}
			}

			// 方式2: 尝试 CodeMirror 编辑器
			var cmEditor = document.querySelector('.CodeMirror');
			if (cmEditor && cmEditor.CodeMirror) {
				cmEditor.CodeMirror.setValue(content);
				return 'CodeMirror content set';
			}

			// 方式3: 尝试简单的 textarea
			var textarea = document.querySelector('textarea.editor, textarea.markdown, #editor, .editor textarea');
			if (textarea) {
				textarea.value = content;
				textarea.dispatchEvent(new Event('input', { bubbles: true }));
				textarea.dispatchEvent(new Event('change', { bubbles: true }));
				return 'Textarea content set';
			}

			// 方式4: 尝试 contenteditable div
			var editableDiv = document.querySelector('[contenteditable="true"].editor, .editor-content, .markdown-editor');
			if (editableDiv) {
				editableDiv.innerText = content;
				editableDiv.dispatchEvent(new Event('input', { bubbles: true }));
				return 'ContentEditable content set';
			}

			// 方式5: 查找任何可能的编辑器 iframe
			var iframes = document.querySelectorAll('iframe');
			for (var i = 0; i < iframes.length; i++) {
				try {
					var iframeDoc = iframes[i].contentDocument || iframes[i].contentWindow.document;
					var iframeBody = iframeDoc.body;
					if (iframeBody && iframeBody.isContentEditable) {
						iframeBody.innerHTML = content.replace(/\n/g, '<br>');
						return 'iFrame editor content set';
					}
				} catch (e) {
					// Cross-origin iframe, skip
				}
			}

			return 'No editor found';
		})()
	`, content)

	var result string
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(script, &result),
	)
	if err != nil {
		return "", fmt.Errorf("set editor content: %w", err)
	}

	fmt.Printf("DEBUG setEditorContent: %s\n", result)
	return fmt.Sprintf("已设置编辑器内容: %s (长度: %d)", result, len(content)), nil
}

// setEditorTitle sets the article title
func (b *Browser) setEditorTitle(title string) (string, error) {
	// 使用 JavaScript 直接设置标题
	script := fmt.Sprintf(`
		(function() {
			var title = %q;

			// 尝试多种标题输入框选择器
			var selectors = ['#txtTitle', 'input[placeholder*="标题"]', 'input[placeholder*="title"]', '.article-title input', 'input.title'];

			for (var i = 0; i < selectors.length; i++) {
				var el = document.querySelector(selectors[i]);
				if (el) {
					el.value = title;
					el.dispatchEvent(new Event('input', { bubbles: true }));
					el.dispatchEvent(new Event('change', { bubbles: true }));
					return 'Title set via ' + selectors[i];
				}
			}

			return 'Title input not found';
		})()
	`, title)

	var result string
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(script, &result),
	)
	if err != nil {
		return "", fmt.Errorf("set editor title: %w", err)
	}

	return fmt.Sprintf("已设置标题: %s", title), nil
}

// clickPublishButton finds and clicks the publish button
func (b *Browser) clickPublishButton() (string, error) {
	// 使用 JavaScript 查找并点击发布按钮（改进版）
	script := `
		(function() {
			// 方法1: 首先尝试通过文字查找按钮（最可靠）
			var allButtons = document.querySelectorAll('button, a.btn, .btn, input[type="button"], input[type="submit"], [role="button"], .el-button, .btn-primary');
			for (var i = 0; i < allButtons.length; i++) {
				var el = allButtons[i];
				var text = (el.innerText || el.textContent || el.value || '').trim();
				var className = el.className || '';
				// 检查是否包含"发布"关键词
				if (text.indexOf('发布') >= 0 || text.indexOf('Publish') >= 0 ||
					className.indexOf('publish') >= 0 || className.indexOf('btn-publish') >= 0) {
					el.click();
					return 'Clicked button: "' + text + '" (class: ' + className + ')';
				}
			}

			// 方法2: 查找特定的发布按钮 ID
			var specificIds = ['#btnPublish', '#publishBtn', '#submitBtn', '#saveBtn'];
			for (var j = 0; j < specificIds.length; j++) {
				var el = document.querySelector(specificIds[j]);
				if (el) {
					el.click();
					return 'Clicked via ID: ' + specificIds[j];
				}
			}

			// 方法3: 查找 CSDN 特定的发布按钮（右上角）
			var csdnBtn = document.querySelector('.editor-toolbar .publish, .bar-right .btn, .right-side button');
			if (csdnBtn) {
				csdnBtn.click();
				return 'Clicked CSDN toolbar button';
			}

			// 方法4: 滚动到底部再查找
			window.scrollTo(0, document.body.scrollHeight);

			// 等待一下后再次查找
			setTimeout(function() {
				var bottomButtons = document.querySelectorAll('button, .btn');
				for (var k = 0; k < bottomButtons.length; k++) {
					var bText = (bottomButtons[k].innerText || '').trim();
					if (bText.indexOf('发布') >= 0) {
						bottomButtons[k].click();
						return 'Clicked bottom publish button';
					}
				}
			}, 500);

			// 返回页面上的所有按钮信息（用于调试）
			var buttonInfo = [];
			for (var m = 0; m < allButtons.length && m < 20; m++) {
				var btnText = (allButtons[m].innerText || allButtons[m].value || '').trim();
				if (btnText) buttonInfo.push(btnText);
			}
			return 'No publish button found. Available buttons: ' + buttonInfo.join(', ');
		})()
	`

	var result string
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(script, &result),
	)
	if err != nil {
		return "", fmt.Errorf("click publish button: %w", err)
	}

	// 等待弹窗出现
	_ = chromedp.Run(b.ctx, chromedp.Sleep(1*time.Second))

	return fmt.Sprintf("已点击发布按钮: %s", result), nil
}

func (b *Browser) parseElements(html string) []Element {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Printf("DEBUG parseElements: goquery error: %v\n", err)
		return nil
	}

	var elements []Element
	idx := 0

	// 扩展选择器，支持更多交互元素
	selector := "a, button, input, select, textarea, [onclick], [role='button'], [role='link'], [role='textbox'], [contenteditable='true'], [data-role], .btn, .button"

	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		el := Element{Index: idx}

		if len(s.Nodes) > 0 {
			el.Tag = s.Nodes[0].Data
		}

		// 获取元素的文本或占位符
		el.Text = strings.TrimSpace(s.Text())
		if el.Text == "" {
			// 尝试获取 placeholder 或 title 或 aria-label
			if placeholder, exists := s.Attr("placeholder"); exists {
				el.Text = placeholder
			} else if title, exists := s.Attr("title"); exists {
				el.Text = title
			} else if ariaLabel, exists := s.Attr("aria-label"); exists {
				el.Text = ariaLabel
			} else if value, exists := s.Attr("value"); exists && value != "" {
				el.Text = value
			}
		}

		if len(el.Text) > 100 {
			el.Text = el.Text[:100] + "..."
		}

		el.Selector = b.buildSelector(s)

		// Skip hidden elements
		if typ, exists := s.Attr("type"); exists && typ == "hidden" {
			return
		}

		// Skip elements without useful selector
		if el.Selector == "" {
			return
		}

		elements = append(elements, el)
		idx++
	})

	fmt.Printf("DEBUG parseElements: Found %d elements with selector: %s\n", len(elements), selector)
	return elements
}

func (b *Browser) buildSelector(s *goquery.Selection) string {
	// 优先使用 id
	if id, exists := s.Attr("id"); exists && id != "" {
		return "#" + id
	}

	// 然后尝试 name
	if name, exists := s.Attr("name"); exists && name != "" {
		return fmt.Sprintf("[name='%s']", name)
	}

	// 尝试 data-testid 或 data-id
	if testID, exists := s.Attr("data-testid"); exists && testID != "" {
		return fmt.Sprintf("[data-testid='%s']", testID)
	}
	if dataID, exists := s.Attr("data-id"); exists && dataID != "" {
		return fmt.Sprintf("[data-id='%s']", dataID)
	}

	// 尝试 class（选择第一个有意义的 class）
	if class, exists := s.Attr("class"); exists && class != "" {
		classes := strings.Fields(class)
		for _, c := range classes {
			// 选择看起来有意义的 class（不是纯数字或太短的）
			if len(c) > 2 && !strings.HasPrefix(c, "_") {
				return fmt.Sprintf(".%s", c)
			}
		}
	}

	// 尝试 role 属性
	if role, exists := s.Attr("role"); exists && role != "" {
		tag := "div"
		if len(s.Nodes) > 0 {
			tag = s.Nodes[0].Data
		}
		return fmt.Sprintf("%s[role='%s']", tag, role)
	}

	// 尝试 placeholder
	if placeholder, exists := s.Attr("placeholder"); exists && placeholder != "" {
		tag := "input"
		if len(s.Nodes) > 0 {
			tag = s.Nodes[0].Data
		}
		return fmt.Sprintf("%s[placeholder='%s']", tag, placeholder)
	}

	// 尝试 type 属性（对于 input）
	if inputType, exists := s.Attr("type"); exists && inputType != "" {
		tag := "input"
		if len(s.Nodes) > 0 {
			tag = s.Nodes[0].Data
		}
		return fmt.Sprintf("%s[type='%s']", tag, inputType)
	}

	// 最后回退到标签名 + 索引位置
	tag := "div"
	if len(s.Nodes) > 0 {
		tag = s.Nodes[0].Data
	}

	// 尝试找到父元素的 id 来构建更具体的选择器
	parent := s.Parent()
	if parent.Length() > 0 {
		if parentID, exists := parent.Attr("id"); exists && parentID != "" {
			return fmt.Sprintf("#%s > %s", parentID, tag)
		}
	}

	return tag
}

func (b *Browser) extractText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	doc.Find("script, style, noscript").Remove()

	text := doc.Text()
	text = strings.TrimSpace(text)

	if len(text) > 5000 {
		text = text[:5000] + "..."
	}

	return text
}

// QuickFetch 快速抓取页面内容（不需要 LLM 决策）
func (b *Browser) QuickFetch(ctx context.Context, url string, cookies []Cookie) (string, error) {
	// 启动浏览器
	if err := b.Start(ctx); err != nil {
		return "", err
	}
	defer b.Close()

	// 设置 Cookie
	if len(cookies) > 0 {
		b.cookies = cookies
	}

	// 导航到目标页面
	var html string
	if b.stealth {
		// Obscura: fire-and-forget nav; can't fetch full outerHTML (too large,
		// returnByValue hangs on Obscura), so return title + body innerText.
		if len(b.cookies) > 0 {
			b.setPresetCookies(extractDomain(url))
		}
		if err := b.stealthNavigate(url); err != nil {
			return "", fmt.Errorf("fetch page: %w", err)
		}
		_ = chromedp.Run(b.ctx, chromedp.Sleep(4*time.Second))
		var title, body string
		if err := chromedp.Run(b.ctx,
			chromedp.Title(&title),
			chromedp.Evaluate(`document.body?document.body.innerText:''`, &body),
		); err != nil {
			return "", fmt.Errorf("fetch page: %w", err)
		}
		return fmt.Sprintf("Title: %s\n\n%s", title, body), nil
	}

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2 * time.Second), // 等待动态内容加载
		chromedp.OuterHTML("html", &html),
	}

	if err := chromedp.Run(b.ctx, actions...); err != nil {
		return "", fmt.Errorf("fetch page: %w", err)
	}

	return html, nil
}

// QuickFetchWithSelector 快速抓取页面特定元素
func (b *Browser) QuickFetchWithSelector(ctx context.Context, url string, cookies []Cookie, selector string) ([]string, error) {
	// 启动浏览器
	if err := b.Start(ctx); err != nil {
		return nil, err
	}
	defer b.Close()

	// 设置 Cookie
	if len(cookies) > 0 {
		b.cookies = cookies
	}

	// 导航并获取内容
	var html string
	if b.stealth {
		// Obscura: can't fetch full HTML (too large for returnByValue); extract
		// matching elements directly via JS querySelectorAll.
		if len(b.cookies) > 0 {
			b.setPresetCookies(extractDomain(url))
		}
		if err := b.stealthNavigate(url); err != nil {
			return nil, fmt.Errorf("fetch page: %w", err)
		}
		_ = chromedp.Run(b.ctx, chromedp.Sleep(4*time.Second))
		var raw string
		if err := chromedp.Run(b.ctx, chromedp.Evaluate(fmt.Sprintf(`JSON.stringify(Array.from(document.querySelectorAll(%q)).map(function(el){return el.innerText;}))`, selector), &raw)); err != nil {
			return nil, fmt.Errorf("fetch page: %w", err)
		}
		var results []string
		_ = json.Unmarshal([]byte(raw), &results)
		return results, nil
	} else {
		actions := []chromedp.Action{
			chromedp.Navigate(url),
			chromedp.WaitReady("body"),
			chromedp.Sleep(2 * time.Second),
			chromedp.OuterHTML("html", &html),
		}
		if err := chromedp.Run(b.ctx, actions...); err != nil {
			return nil, fmt.Errorf("fetch page: %w", err)
		}
	}

	// 解析 HTML 提取指定元素
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var results []string
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			results = append(results, text)
		}
	})

	return results, nil
}

// ============================================================
// Cookie Extraction
// ============================================================

// ExtractCookies extracts all cookies from current page
func (b *Browser) ExtractCookies(ctx context.Context) ([]Cookie, error) {
	var cookies []*network.Cookie

	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Get all cookies
			cookiesResult, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			cookies = cookiesResult
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	result := make([]Cookie, len(cookies))
	for i, c := range cookies {
		result[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}

	return result, nil
}

// ExtractCookiesByDomain extracts cookies for a specific domain
func (b *Browser) ExtractCookiesByDomain(ctx context.Context, domain string) ([]Cookie, error) {
	allCookies, err := b.ExtractCookies(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []Cookie
	for _, c := range allCookies {
		// Match domain (exact or subdomain)
		if c.Domain == domain || strings.HasSuffix(c.Domain, domain) {
			filtered = append(filtered, c)
		}
	}

	return filtered, nil
}

// GetCurrentURL gets the current page URL
func (b *Browser) GetCurrentURL(ctx context.Context) (string, error) {
	var url string
	err := chromedp.Run(b.ctx, chromedp.Location(&url))
	if err != nil {
		return "", fmt.Errorf("get url: %w", err)
	}
	return url, nil
}

// detectChromePath auto-detects Chrome/Chromium executable path
func detectChromePath() string {
	// Common Chrome/Chromium paths (in order of preference)
	paths := []string{
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome-beta",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", // macOS
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",   // Windows
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fallback to chromium-browser (will fail with clear error if not found)
	return "/usr/bin/chromium-browser"
}

// executeJavaScript executes arbitrary JavaScript code in the browse
// executeJavaScript executes arbitrary JavaScript code in the browser
func (b *Browser) executeJavaScript(jsCode string) (string, error) {
	if jsCode == "" {
		return "", fmt.Errorf("javascript code is empty")
	}

	var result string
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(jsCode, &result),
	)
	if err != nil {
		return "", fmt.Errorf("execute javascript: %w", err)
	}

	return fmt.Sprintf("JavaScript 执行结果: %s", result), nil
}

// Eval runs synchronous JavaScript in the current page and returns the result
// as a string. Uses the plain chromedp.Evaluate (Runtime.evaluate with
// returnByValue) - the same path getStateStealth uses, confirmed reliable on
// the Obscura stealth CDP server. The JS must be synchronous (no top-level
// await): use XMLHttpRequest (sync) rather than fetch for any in-page HTTP.
//
// Keep the returned value small (well under ~500KB) - very large returnByValue
// payloads hang Obscura's CDP server.
func (b *Browser) Eval(ctx context.Context, js string) (string, error) {
	if js == "" {
		return "", fmt.Errorf("javascript is empty")
	}
	var raw string
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(js, &raw)); err != nil {
		return "", fmt.Errorf("eval: %w", err)
	}
	return raw, nil
}

// EvalAsync runs JS that evaluates to a Promise and awaits it, returning the
// resolved value as a string. Unlike Eval (synchronous returnByValue, which
// leaves the page's event loop frozen on Obscura so async callbacks never
// fire), a single awaitPromise:true Runtime.evaluate must pump the page event
// loop until the promise settles - so async fetch/Promise chains actually
// complete. Use this for in-page network calls (e.g. signed XHS API fetches).
//
// Keep the resolved value small (well under ~500KB).
func (b *Browser) EvalAsync(ctx context.Context, js string) (string, error) {
	if js == "" {
		return "", fmt.Errorf("javascript is empty")
	}
	var obj *cdpruntime.RemoteObject
	var exc *cdpruntime.ExceptionDetails
	err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(c context.Context) error {
		var e error
		obj, exc, e = cdpruntime.Evaluate(js).WithAwaitPromise(true).WithReturnByValue(true).Do(c)
		return e
	}))
	if err != nil {
		return "", fmt.Errorf("eval_async: %w", err)
	}
	if exc != nil {
		return "", fmt.Errorf("eval_async exception: %s", exc.Text)
	}
	if obj == nil || obj.Value == nil {
		return "", nil
	}
	// returnByValue wraps a string result as a JSON string literal.
	var s string
	if err := json.Unmarshal(obj.Value, &s); err == nil {
		return s, nil
	}
	return string(obj.Value), nil
}

// PressEnter dispatches a REAL Enter keydown/keyup (isTrusted=true) via the CDP
// Input domain to whatever element currently has focus. Unlike a synthetic
// KeyboardEvent dispatched via Eval, a real key event bypasses isTrusted guards
// that sites (e.g. XHS) use to ignore programmatic input, so it actually
// triggers onKeyDown handlers. The caller must focus the target element first.
func (b *Browser) PressEnter(ctx context.Context) error {
	return chromedp.Run(b.ctx,
		input.DispatchKeyEvent(input.KeyDown).
			WithKey("Enter").WithCode("Enter").
			WithWindowsVirtualKeyCode(13).WithNativeVirtualKeyCode(13),
		input.DispatchKeyEvent(input.KeyUp).
			WithKey("Enter").WithCode("Enter").
			WithWindowsVirtualKeyCode(13).WithNativeVirtualKeyCode(13),
	)
}

// InjectInitScript injects JS that runs on every new page load BEFORE the
// page's own scripts (via Page.addScriptToEvaluateOnNewDocument). Used to patch
// fetch/XHR ahead of hydration so requests the SPA fires during load are
// captured. Returns the script identifier.
func (b *Browser) InjectInitScript(ctx context.Context, source string) (string, error) {
	var scriptID string
	err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(c context.Context) error {
		id, err := page.AddScriptToEvaluateOnNewDocument(source).Do(c)
		if err != nil {
			return err
		}
		scriptID = string(id)
		return nil
	}))
	if err != nil {
		return "", fmt.Errorf("inject init script: %w", err)
	}
	return scriptID, nil
}
