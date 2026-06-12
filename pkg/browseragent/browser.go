// Package browseragent provides browser control using chromedp
package browseragent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/network"
)

// Browser manages a browser instance using chromedp
type Browser struct {
	ctx       context.Context
	cancel    context.CancelFunc
	allocator context.Context
	started   bool
	cookies   []Cookie // 预注入的 Cookie

	// Pool mode flags
	fromPool  bool        // 是否从池中获取
	pool      *BrowserPool // 所属的池（如果是从池中获取）
	pooled    *PooledBrowser // 池化的浏览器实例
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
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.WindowSize(1920, 1080),
		// 反检测选项
		chromedp.Flag("disable-blink-features", "AutomationControlled"), // 隐藏自动化特征
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"), // 真实浏览器 UA
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
	var url, title, html string

	err := chromedp.Run(b.ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, fmt.Errorf("get page state: %w", err)
	}

	elements := b.parseElements(html)

	return &PageState{
		URL:      url,
		Title:    title,
		Elements: elements,
		Text:     b.extractText(html),
	}, nil
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
	default:
		return "", fmt.Errorf("unknown action: %s", action.Type)
	}
}

func (b *Browser) navigate(url string) (string, error) {
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	// 解析域名
	domain := extractDomain(url)

	// 如果有预设置的 Cookie，先导航到目标域名再注入
	if len(b.cookies) > 0 {
		// 先导航到目标页面的域名（这样才能正确设置 Cookie）
		if err := chromedp.Run(b.ctx, chromedp.Navigate(url)); err != nil {
			return "", fmt.Errorf("navigate: %w", err)
		}

		// 等待页面加载
		if err := chromedp.Run(b.ctx, chromedp.WaitReady("body")); err != nil {
			return "", fmt.Errorf("wait ready: %w", err)
		}

		// 注入所有 Cookie（在目标域名上下文中）
		for _, c := range b.cookies {
			if err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				setCookie := network.SetCookie(c.Name, c.Value)
				// 使用 Cookie 的域名或当前页面域名
				cookieDomain := c.Domain
				if cookieDomain == "" {
					cookieDomain = domain
				}
				setCookie = setCookie.WithDomain(cookieDomain)
				if c.Path != "" {
					setCookie = setCookie.WithPath(c.Path)
				}
				if c.HTTPOnly {
					setCookie = setCookie.WithHTTPOnly(true)
				}
				if c.Secure {
					setCookie = setCookie.WithSecure(true)
				}
				return setCookie.Do(ctx)
			})); err != nil {
				fmt.Printf("Warning: failed to set cookie %s: %v\n", c.Name, err)
			}
		}

		// 刷新页面使 Cookie 生效
		if err := chromedp.Run(b.ctx, chromedp.Reload()); err != nil {
			return "", fmt.Errorf("reload: %w", err)
		}

		if err := chromedp.Run(b.ctx, chromedp.WaitReady("body")); err != nil {
			return "", fmt.Errorf("wait ready after reload: %w", err)
		}
	} else {
		// 没有 Cookie，直接导航
		err := chromedp.Run(b.ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body"),
		)
		if err != nil {
			return "", fmt.Errorf("navigate: %w", err)
		}
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

	err = chromedp.Run(b.ctx,
		chromedp.Click(element.Selector),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return "", fmt.Errorf("click: %w", err)
	}

	return fmt.Sprintf("已点击元素[%d]", elementIndex), nil
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

	err = chromedp.Run(b.ctx,
		chromedp.Clear(element.Selector),
		chromedp.SendKeys(element.Selector, text),
	)
	if err != nil {
		return "", fmt.Errorf("type: %w", err)
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

func (b *Browser) parseElements(html string) []Element {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var elements []Element
	idx := 0

	doc.Find("a, button, input, select, textarea, [onclick]").Each(func(i int, s *goquery.Selection) {
		el := Element{Index: idx}

		if len(s.Nodes) > 0 {
			el.Tag = s.Nodes[0].Data
		}

		el.Text = strings.TrimSpace(s.Text())
		if len(el.Text) > 100 {
			el.Text = el.Text[:100] + "..."
		}

		el.Selector = b.buildSelector(s)

		// Skip hidden elements
		if typ, exists := s.Attr("type"); exists && typ == "hidden" {
			return
		}

		elements = append(elements, el)
		idx++
	})

	return elements
}

func (b *Browser) buildSelector(s *goquery.Selection) string {
	if id, exists := s.Attr("id"); exists && id != "" {
		return "#" + id
	}

	if name, exists := s.Attr("name"); exists && name != "" {
		return fmt.Sprintf("[name='%s']", name)
	}

	tag := "div"
	if len(s.Nodes) > 0 {
		tag = s.Nodes[0].Data
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
	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2 * time.Second),
		chromedp.OuterHTML("html", &html),
	}

	if err := chromedp.Run(b.ctx, actions...); err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
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