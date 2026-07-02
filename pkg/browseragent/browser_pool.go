// Package browseragent provides browser control using chromedp
package browseragent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserPool manages a pool of reusable browser instances
type BrowserPool struct {
	mu       sync.Mutex
	browsers chan *PooledBrowser
	maxSize  int
	opts     []chromedp.ContextOption

	// Background health check
	stopHealthCheck chan struct{}
	wg              sync.WaitGroup
}

// PooledBrowser wraps a Browser with additional metadata
type PooledBrowser struct {
	*Browser
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

// globalPool is the default browser pool instance
var globalPool *BrowserPool
var poolOnce sync.Once

// GetBrowserPool returns the global browser pool instance
func GetBrowserPool() *BrowserPool {
	poolOnce.Do(func() {
		maxSize := 5 // default pool size
		if envSize := os.Getenv("BROWSER_POOL_SIZE"); envSize != "" {
			var size int
			if _, err := fmt.Sscanf(envSize, "%d", &size); err == nil && size > 0 {
				maxSize = size
			}
		}
		globalPool = NewBrowserPool(maxSize)
	})
	return globalPool
}

// NewBrowserPool creates a new browser pool
func NewBrowserPool(maxSize int) *BrowserPool {
	pool := &BrowserPool{
		browsers:        make(chan *PooledBrowser, maxSize),
		maxSize:         maxSize,
		stopHealthCheck: make(chan struct{}),
	}

	// Start background health check
	pool.wg.Add(1)
	go pool.healthCheck()

	return pool
}

// Acquire gets a browser from the pool or creates a new one
func (p *BrowserPool) Acquire(ctx context.Context) (*PooledBrowser, error) {
	select {
	case pb := <-p.browsers:
		// Check if browser is still valid
		if pb.isValid() {
			// ★ Don't reset browser state - keep cookies intact
			// Just mark as in use and return
			pb.lastUsed = time.Now()
			pb.inUse = true
			return pb, nil
		}
		// Invalid browser, close and create new one
		pb.Close()

	default:
		// Pool is empty, create new browser
	}

	return p.createNewPooledBrowser(ctx)
}

// createNewPooledBrowser creates a new pooled browser instance
func (p *BrowserPool) createNewPooledBrowser(ctx context.Context) (*PooledBrowser, error) {
	browser, err := p.createNewBrowser(ctx)
	if err != nil {
		return nil, err
	}

	return &PooledBrowser{
		Browser:   browser,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		inUse:     true,
	}, nil
}

// Release returns a browser to the pool
func (p *BrowserPool) Release(pb *PooledBrowser) {
	if pb == nil {
		return
	}

	pb.inUse = false
	pb.lastUsed = time.Now()

	select {
	case p.browsers <- pb:
		// Returned to pool
	default:
		// Pool is full, close the browser
		pb.Close()
	}
}

// createNewBrowser creates a new browser instance
func (p *BrowserPool) createNewBrowser(ctx context.Context) (*Browser, error) {
	// Check if using Obscura CDP server (remote browser engine)
	obscuraURL := os.Getenv("OBSCURA_CDP_URL")
	if obscuraURL != "" {
		// Connect to Obscura CDP server — stealth mode, low memory, fast startup
		allocCtx, _ := chromedp.NewRemoteAllocator(ctx, obscuraURL)
		browserCtx, cancel := chromedp.NewContext(allocCtx)

		err := chromedp.Run(browserCtx,
			chromedp.Navigate("about:blank"),
			chromedp.WaitReady("body"),
		)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("connect to Obscura CDP server at %s: %w", obscuraURL, err)
		}

		return &Browser{
			ctx:       browserCtx,
			cancel:    cancel,
			allocator: allocCtx,
			started:   true,
		}, nil
	}

	// Fallback: use local Chrome/Chromium (original behavior)
	// Get Chrome executable path from environment or auto-detect
	chromePath := os.Getenv("CHROME_BIN")
	if chromePath == "" {
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
		// Anti-detection options
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	// Create allocator context
	allocCtx, _ := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, cancel := chromedp.NewContext(allocCtx)

	// Initialize browser by navigating to a blank page
	err := chromedp.Run(browserCtx,
		chromedp.Navigate("about:blank"),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("initialize browser: %w", err)
	}

	return &Browser{
		ctx:       browserCtx,
		cancel:    cancel,
		allocator: allocCtx,
		started:   true,
	}, nil
}

// isValid checks if the browser is still functional
func (pb *PooledBrowser) isValid() bool {
	if pb == nil || pb.Browser == nil || pb.ctx == nil {
		return false
	}

	// Check if context is done
	select {
	case <-pb.ctx.Done():
		return false
	default:
	}

	// Try a simple operation to verify browser is alive
	var url string

	err := chromedp.Run(pb.ctx,
		chromedp.Location(&url),
	)
	return err == nil
}

// healthCheck periodically cleans up idle browsers
func (p *BrowserPool) healthCheck() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdleBrowsers()
		case <-p.stopHealthCheck:
			return
		}
	}
}

// cleanupIdleBrowsers removes browsers that have been idle for too long
func (p *BrowserPool) cleanupIdleBrowsers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to check browsers in the pool
	for {
		select {
		case pb := <-p.browsers:
			// Check if browser has been idle for more than 5 minutes
			if time.Since(pb.lastUsed) > 5*time.Minute || !pb.isValid() {
				pb.Close()
			} else {
				// Put it back
				select {
				case p.browsers <- pb:
				default:
					pb.Close()
				}
			}
		default:
			return
		}
	}
}

// Close closes all browsers in the pool
func (p *BrowserPool) Close() {
	close(p.stopHealthCheck)
	p.wg.Wait()

	// Close all browsers in the pool
	for {
		select {
		case pb := <-p.browsers:
			pb.Close()
		default:
			return
		}
	}
}

// Stats returns pool statistics
func (p *BrowserPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	return map[string]interface{}{
		"pool_size":    len(p.browsers),
		"max_size":     p.maxSize,
		"available":    len(p.browsers),
	}
}

// CreateTab creates a new tab in an existing browser context
// This is more efficient than creating a new browser instance
func (pb *PooledBrowser) CreateTab() (*BrowserTab, error) {
	if pb == nil || pb.Browser == nil {
		return nil, fmt.Errorf("invalid browser")
	}

	// Create new context for tab (shares same browser process)
	tabCtx, cancel := chromedp.NewContext(pb.allocator)

	// Initialize tab
	err := chromedp.Run(tabCtx,
		chromedp.Navigate("about:blank"),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("initialize tab: %w", err)
	}

	return &BrowserTab{
		ctx:    tabCtx,
		cancel: cancel,
	}, nil
}

// BrowserTab represents a tab in a pooled browser
type BrowserTab struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Close closes the tab
func (t *BrowserTab) Close() {
	if t.cancel != nil {
		t.cancel()
	}
}
