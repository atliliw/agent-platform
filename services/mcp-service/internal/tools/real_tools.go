// Package tools provides tool implementations
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"agent-platform/pkg/browseragent"
	"github.com/PuerkitoBio/goquery"
)

// HTTP client with timeout
var defaultHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

func getDefaultHTTPClient() *http.Client {
	return defaultHTTPClient
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ============================================================
// Browser Tool - 直接调用 browseragent (不需要远程服务)
// ============================================================

// BrowserTool executes browser automation tasks using browseragent
type BrowserTool struct {
	apiKey  string
	baseURL string
	model   string
}

// NewBrowserTool creates a new browser tool (reads from env for backward compatibility)
func NewBrowserTool() *BrowserTool {
	return &BrowserTool{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: os.Getenv("OPENAI_BASE_URL"),
		model:   os.Getenv("LLM_MODEL"),
	}
}

// NewBrowserToolWithConfig creates a new browser tool with config
func NewBrowserToolWithConfig(apiKey, baseURL, model string) *BrowserTool {
	return &BrowserTool{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}

// GetInfo returns tool information for LLM
func (t *BrowserTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "browser_execute",
		Description: "执行浏览器自动化任务。接收自然语言描述，自动操控浏览器完成网页操作、数据采集、表单填写等任务。支持注入 Cookie 用于访问需要登录的网站。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "任务描述，如：打开百度搜索Python，打开GitHub查看某个项目",
				},
				"max_steps": map[string]interface{}{
					"type":        "integer",
					"description": "最大执行步数 (default: 20)",
				},
				"cookies": map[string]interface{}{
					"type":        "array",
					"description": "预注入的 Cookie 列表，用于访问需要登录的网站",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":     map[string]interface{}{"type": "string", "description": "Cookie 名称"},
							"value":    map[string]interface{}{"type": "string", "description": "Cookie 值"},
							"domain":   map[string]interface{}{"type": "string", "description": "Cookie 作用域名，如 .taobao.com"},
							"path":     map[string]interface{}{"type": "string", "description": "Cookie 路径 (default: /)"},
							"expires":  map[string]interface{}{"type": "integer", "description": "过期时间 (Unix timestamp)"},
							"httpOnly": map[string]interface{}{"type": "boolean", "description": "是否 HTTP Only"},
							"secure":   map[string]interface{}{"type": "boolean", "description": "是否 Secure"},
						},
						"required": []string{"name", "value"},
					},
				},
			},
			"required": []string{"task"},
		},
	}
}

// Execute executes the browser automation task
func (t *BrowserTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the browser automation task with tool config
func (t *BrowserTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	fmt.Println("BrowserTool: 开始执行")

	// Browser Agent 需要较长时间，创建新的 context
	// 避免继承父 context 的超时限制
	execCtx, cancel := context.WithTimeout(context.Background(), 240*time.Second) // 4 分钟
	defer cancel()

	task, _ := args["task"].(string)
	fmt.Printf("BrowserTool: 任务: %s\n", task)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	maxSteps := 50
	if ms, ok := args["max_steps"].(float64); ok {
		maxSteps = int(ms)
	}

	// 解析 Cookie 参数（手动传入）
	var cookies []browseragent.Cookie
	if cookieArr, ok := args["cookies"].([]interface{}); ok {
		for _, c := range cookieArr {
			if cm, ok := c.(map[string]interface{}); ok {
				cookie := browseragent.Cookie{}
				if name, ok := cm["name"].(string); ok {
					cookie.Name = name
				}
				if value, ok := cm["value"].(string); ok {
					cookie.Value = value
				}
				if domain, ok := cm["domain"].(string); ok {
					cookie.Domain = domain
				}
				if path, ok := cm["path"].(string); ok {
					cookie.Path = path
				}
				if expires, ok := cm["expires"].(float64); ok {
					cookie.Expires = int64(expires)
				}
				if httpOnly, ok := cm["httpOnly"].(bool); ok {
					cookie.HTTPOnly = httpOnly
				}
				if secure, ok := cm["secure"].(bool); ok {
					cookie.Secure = secure
				}
				cookies = append(cookies, cookie)
			}
		}
	}

	// ★ 如果没有手动传入 Cookie，尝试自动加载预存的 Cookie
	if len(cookies) == 0 {
		// 从任务描述中提取可能的 URL
		urlPattern := extractURLFromTask(task)
		fmt.Printf("BrowserTool: 提取的URL模式: %s\n", urlPattern)
		if urlPattern != "" {
			loader := NewCookieLoader("", "default", "default")
			autoCookies, err := loader.LoadCookiesForURL(ctx, urlPattern)
			if err != nil {
				fmt.Printf("BrowserTool: 加载Cookie失败: %v\n", err)
			} else if len(autoCookies) > 0 {
				cookies = autoCookies
				fmt.Printf("BrowserTool: 自动加载了 %d 个预存的 Cookie (域名: %s)\n", len(cookies), urlPattern)
			} else {
				fmt.Printf("BrowserTool: 未找到预存的 Cookie\n")
			}
		}
	}

	// Get config from tool config or fall back to struct fields
	apiKey := t.apiKey
	baseURL := t.baseURL
	model := t.model

	if config != nil {
		if ak, ok := config["api_key"].(string); ok && ak != "" {
			apiKey = ak
		}
		if bu, ok := config["base_url"].(string); ok && bu != "" {
			baseURL = bu
		}
		if m, ok := config["model"].(string); ok && m != "" {
			model = m
		}
	}

	// 检查配置
	if apiKey == "" {
		return "", fmt.Errorf("LLM API Key not configured for browser tool")
	}

	// 创建 LLM 客户端
	llmClient := browseragent.NewOpenAIClient(apiKey, baseURL, model)

	// 创建浏览器
	browser := browseragent.NewBrowser()

	// 创建 Agent
	agent := browseragent.New(llmClient, browser,
		browseragent.WithMaxSteps(maxSteps),
		browseragent.WithMaxRetries(3),
	)

	// 设置 Cookie
	if len(cookies) > 0 {
		agent.SetCookies(cookies)
	}

	// 执行任务
	result, err := agent.Run(execCtx, task)
	if err != nil {
		return fmt.Sprintf("任务执行失败: %v", err), nil
	}

	// 构建响应
	response := fmt.Sprintf("任务: %s\n\n结果: %s\n\n执行步数: %d\n耗时: %v",
		task, strings.ToValidUTF8(result.Answer, ""), result.Steps, result.Duration)

	if len(cookies) > 0 {
		response = fmt.Sprintf("已注入 %d 个 Cookie\n\n%s", len(cookies), response)
	}

	if len(result.StepHistory) > 0 {
		response += "\n\n执行历史:"
		for _, step := range result.StepHistory {
			response += fmt.Sprintf("\n  步骤%d: %s -> %s", step.Step, step.Action.Type, strings.ToValidUTF8(step.Result, ""))
		}
	}

	return response, nil
}

// ============================================================
// Quick Fetch Tool - 快速抓取页面内容（不需要 LLM 决策）
// ============================================================

// QuickFetchTool 快速抓取工具
type QuickFetchTool struct{}

// NewQuickFetchTool creates a new quick fetch tool
func NewQuickFetchTool() *QuickFetchTool {
	return &QuickFetchTool{}
}

// GetInfo returns tool information for LLM
func (t *QuickFetchTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "quick_fetch",
		Description: "快速抓取网页内容。适用于需要登录的网站，通过注入 Cookie 获取页面内容。不需要 LLM 决策，速度快。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "要抓取的网页 URL",
				},
				"selector": map[string]interface{}{
					"type":        "string",
					"description": "CSS 选择器，用于提取特定元素（可选）",
				},
				"cookies": map[string]interface{}{
					"type":        "array",
					"description": "Cookie 列表，用于访问需要登录的网站",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":   map[string]interface{}{"type": "string", "description": "Cookie 名称"},
							"value":  map[string]interface{}{"type": "string", "description": "Cookie 值"},
							"domain": map[string]interface{}{"type": "string", "description": "Cookie 作用域名"},
						},
					},
				},
			},
			"required": []string{"url"},
		},
	}
}

// Execute executes the quick fetch
func (t *QuickFetchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the quick fetch with config
func (t *QuickFetchTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	// 创建新的 context 避免超时
	fetchCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	selector, _ := args["selector"].(string)

	// 解析 Cookie（手动传入）
	var cookies []browseragent.Cookie
	if cookieArr, ok := args["cookies"].([]interface{}); ok {
		for _, c := range cookieArr {
			if cm, ok := c.(map[string]interface{}); ok {
				cookie := browseragent.Cookie{}
				if name, ok := cm["name"].(string); ok {
					cookie.Name = name
				}
				if value, ok := cm["value"].(string); ok {
					cookie.Value = value
				}
				if domain, ok := cm["domain"].(string); ok {
					cookie.Domain = domain
				}
				cookies = append(cookies, cookie)
			}
		}
	}

	// ★ 如果没有手动传入 Cookie，尝试自动加载预存的 Cookie
	if len(cookies) == 0 {
		loader := NewCookieLoader("", "default", "default")
		autoCookies, err := loader.LoadCookiesForURL(fetchCtx, url)
		if err != nil {
			fmt.Printf("QuickFetch: 加载Cookie失败: %v\n", err)
		} else if len(autoCookies) > 0 {
			cookies = autoCookies
			fmt.Printf("QuickFetch: 自动加载了 %d 个预存的 Cookie\n", len(cookies))
		}
	}

	// 创建浏览器
	browser := browseragent.NewBrowser()

	if selector != "" {
		// 使用选择器提取特定元素
		results, err := browser.QuickFetchWithSelector(fetchCtx, url, cookies, selector)
		if err != nil {
			return "", fmt.Errorf("fetch failed: %w", err)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("URL: %s\n", url))
		sb.WriteString(fmt.Sprintf("Selector: %s\n\n", selector))
		sb.WriteString(fmt.Sprintf("找到 %d 个元素:\n", len(results)))
		for i, r := range results {
			if len(r) > 200 {
				r = r[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
		}
		return sb.String(), nil
	}

	// 获取整个页面
	html, err := browser.QuickFetch(fetchCtx, url, cookies)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}

	// 清理 HTML 中的无效 UTF-8 字符
	html = strings.ToValidUTF8(html, "")

	// 提取文本
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	// 移除不需要的元素
	doc.Find("script, style, noscript, nav, footer, header").Remove()

	// 获取标题
	title := doc.Find("title").Text()
	title = strings.ToValidUTF8(title, "?")

	// 获取主要内容
	text := doc.Find("body").Text()
	text = strings.TrimSpace(text)
	text = strings.ToValidUTF8(text, "?") // 清理无效 UTF-8，用 ? 替换
	text = strings.ReplaceAll(text, "\n\n\n", "\n")
	text = strings.ReplaceAll(text, "\n\n", "\n")

	// 移除控制字符
	text = removeControlChars(text)
	title = removeControlChars(title)

	// 限制长度
	if len(text) > 3000 {
		text = text[:3000] + "\n... (内容已截断)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("URL: %s\n", url))
	sb.WriteString(fmt.Sprintf("标题: %s\n", title))
	if len(cookies) > 0 {
		sb.WriteString(fmt.Sprintf("已注入 %d 个 Cookie\n", len(cookies)))
	}
	sb.WriteString("\n--- 页面内容 ---\n")
	sb.WriteString(text)

	return sb.String(), nil
}

// removeControlChars 移除控制字符
func removeControlChars(s string) string {
	var result strings.Builder
	for _, r := range s {
		// 保留换行、制表符和可打印字符
		if r == '\n' || r == '\t' || r >= 32 && r < 127 || r >= 128 && r < 256 {
			result.WriteRune(r)
		} else if r > 256 {
			result.WriteRune(r) // 保留中文等 Unicode 字符
		}
	}
	return result.String()
}

// ============================================================
// Web Search Tool - Real Implementation
// ============================================================

// WebSearchTool implements web search via external APIs
type WebSearchTool struct {
	APIKey   string
	Provider string // "serpapi", "bing", "google"
}

// NewWebSearchToolWithConfig creates a new web search tool with config
func NewWebSearchToolWithConfig(apiKey, provider string) *WebSearchTool {
	return &WebSearchTool{
		APIKey:   apiKey,
		Provider: provider,
	}
}

// NewWebSearchTool creates a new web search tool (reads from env for backward compatibility)
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		APIKey:   os.Getenv("WEB_SEARCH_API_KEY"),
		Provider: os.Getenv("WEB_SEARCH_PROVIDER"),
	}
}

// GetInfo returns tool information for LLM
func (t *WebSearchTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "web_search",
		Description: "Search the web for information. Use this to find current information, news, articles, and general knowledge from the internet.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
				"num_results": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results to return (default: 5)",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute executes the web search tool
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the web search tool with config
func (t *WebSearchTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	numResults := 5
	if nr, ok := args["num_results"].(float64); ok {
		numResults = int(nr)
	}

	// If no API key configured, return helpful message
	if t.APIKey == "" {
		return fmt.Sprintf(`Web Search: %q

Note: Web search API not configured. To enable real web search:
1. Set WEB_SEARCH_API_KEY environment variable
2. Set WEB_SEARCH_PROVIDER (supported: serpapi, bing)
3. Restart the MCP service

For now, this is a placeholder response.`, query), nil
	}

	// Use configured provider
	switch t.Provider {
	case "serpapi":
		return t.searchSerpAPI(ctx, query, numResults)
	case "bing":
		return t.searchBing(ctx, query, numResults)
	default:
		return t.searchSerpAPI(ctx, query, numResults)
	}
}

// searchSerpAPI searches using SerpAPI
func (t *WebSearchTool) searchSerpAPI(ctx context.Context, query string, numResults int) (string, error) {
	url := fmt.Sprintf("https://serpapi.com/search.json?q=%s&api_key=%s&num=%d",
		urlEncode(query), t.APIKey, numResults)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := getDefaultHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("API error: %s", result.Error)
	}

	if len(result.OrganicResults) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}

	output := fmt.Sprintf("Web search results for %q:\n\n", query)
	for i, r := range result.OrganicResults {
		output += fmt.Sprintf("%d. %s\n", i+1, r.Title)
		output += fmt.Sprintf("   %s\n", r.Snippet)
		output += fmt.Sprintf("   URL: %s\n\n", r.Link)
	}

	return output, nil
}

// searchBing searches using Bing Web Search API
func (t *WebSearchTool) searchBing(ctx context.Context, query string, numResults int) (string, error) {
	url := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s&count=%d",
		urlEncode(query), numResults)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", t.APIKey)

	resp, err := getDefaultHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(result.WebPages.Value) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}

	output := fmt.Sprintf("Web search results for %q:\n\n", query)
	for i, r := range result.WebPages.Value {
		output += fmt.Sprintf("%d. %s\n", i+1, r.Name)
		output += fmt.Sprintf("   %s\n", r.Snippet)
		output += fmt.Sprintf("   URL: %s\n\n", r.URL)
	}

	return output, nil
}

// ============================================================
// Weather Tool - Real Implementation
// ============================================================

// WeatherTool implements weather queries via external APIs
type WeatherTool struct {
	APIKey   string
	Provider string // "openweathermap", "qweather"
}

// NewWeatherToolWithConfig creates a new weather tool with config
func NewWeatherToolWithConfig(apiKey, provider string) *WeatherTool {
	return &WeatherTool{
		APIKey:   apiKey,
		Provider: provider,
	}
}

// NewWeatherTool creates a new weather tool (reads from env for backward compatibility)
func NewWeatherTool() *WeatherTool {
	return &WeatherTool{
		APIKey:   os.Getenv("WEATHER_API_KEY"),
		Provider: os.Getenv("WEATHER_PROVIDER"),
	}
}

// GetInfo returns tool information for LLM
func (t *WeatherTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "weather",
		Description: "Get current weather information for a location.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name or location (e.g., 'Beijing', 'New York')",
				},
				"units": map[string]interface{}{
					"type":        "string",
					"description": "Temperature units: 'celsius' or 'fahrenheit' (default: celsius)",
				},
			},
			"required": []string{"location"},
		},
	}
}

// Execute executes the weather tool
func (t *WeatherTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the weather tool with config
func (t *WeatherTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	location, _ := args["location"].(string)
	if location == "" {
		return "", fmt.Errorf("location is required")
	}

	units := "metric" // celsius
	if u, ok := args["units"].(string); ok && u == "fahrenheit" {
		units = "imperial"
	}

	// If no API key configured, return helpful message
	if t.APIKey == "" {
		return fmt.Sprintf(`Weather for %s

Note: Weather API not configured. To enable real weather data:
1. Set WEATHER_API_KEY environment variable
2. Set WEATHER_PROVIDER (supported: openweathermap, qweather)
3. Restart the MCP service

For now, this is a placeholder response.`, location), nil
	}

	switch t.Provider {
	case "qweather":
		return t.getQWeather(ctx, location)
	default:
		return t.getOpenWeatherMap(ctx, location, units)
	}
}

// getOpenWeatherMap fetches weather from OpenWeatherMap
func (t *WeatherTool) getOpenWeatherMap(ctx context.Context, location, units string) (string, error) {
	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=%s",
		urlEncode(location), t.APIKey, units)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := getDefaultHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("weather request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Sprintf("Location not found: %s", location), nil
	}

	var result struct {
		Name string `json:"name"`
		Main struct {
			Temp     float64 `json:"temp"`
			Humidity int     `json:"humidity"`
			Pressure int     `json:"pressure"`
		} `json:"main"`
		Weather []struct {
			Main        string `json:"main"`
			Description string `json:"description"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
		Sys struct {
			Country string `json:"country"`
		} `json:"sys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(result.Weather) == 0 {
		return "", fmt.Errorf("no weather data available")
	}

	tempUnit := "°C"
	if units == "imperial" {
		tempUnit = "°F"
	}

	output := fmt.Sprintf(`Weather for %s, %s:

Current Conditions:
- Temperature: %.1f%s
- Humidity: %d%%
- Pressure: %d hPa
- Wind Speed: %.1f m/s
- Conditions: %s (%s)`,
		result.Name, result.Sys.Country,
		result.Main.Temp, tempUnit,
		result.Main.Humidity,
		result.Main.Pressure,
		result.Wind.Speed,
		result.Weather[0].Main,
		result.Weather[0].Description)

	return output, nil
}

// getQWeather fetches weather from QWeather (和风天气)
func (t *WeatherTool) getQWeather(ctx context.Context, location string) (string, error) {
	// First, get location ID
	geoURL := fmt.Sprintf("https://geoapi.qweather.com/v2/city/lookup?location=%s&key=%s",
		urlEncode(location), t.APIKey)

	geoReq, err := http.NewRequestWithContext(ctx, "GET", geoURL, nil)
	if err != nil {
		return "", fmt.Errorf("create geo request: %w", err)
	}

	geoResp, err := getDefaultHTTPClient().Do(geoReq)
	if err != nil {
		return "", fmt.Errorf("geo request failed: %w", err)
	}
	defer geoResp.Body.Close()

	var geoResult struct {
		Code string `json:"code"`
		Location []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Adm1 string `json:"adm1"`
		} `json:"location"`
	}

	if err := json.NewDecoder(geoResp.Body).Decode(&geoResult); err != nil {
		return "", fmt.Errorf("parse geo response: %w", err)
	}

	if geoResult.Code != "200" || len(geoResult.Location) == 0 {
		return fmt.Sprintf("Location not found: %s", location), nil
	}

	loc := geoResult.Location[0]

	// Get weather
	weatherURL := fmt.Sprintf("https://devapi.qweather.com/v7/weather/now?location=%s&key=%s",
		loc.ID, t.APIKey)

	weatherReq, err := http.NewRequestWithContext(ctx, "GET", weatherURL, nil)
	if err != nil {
		return "", fmt.Errorf("create weather request: %w", err)
	}

	weatherResp, err := getDefaultHTTPClient().Do(weatherReq)
	if err != nil {
		return "", fmt.Errorf("weather request failed: %w", err)
	}
	defer weatherResp.Body.Close()

	var weatherResult struct {
		Code string `json:"code"`
		Now  struct {
			Temp     string `json:"temp"`
			Humidity string `json:"humidity"`
			Pressure string `json:"pressure"`
			WindDir  string `json:"windDir"`
			WindSpeed string `json:"windSpeed"`
			Text     string `json:"text"`
		} `json:"now"`
	}

	if err := json.NewDecoder(weatherResp.Body).Decode(&weatherResult); err != nil {
		return "", fmt.Errorf("parse weather response: %w", err)
	}

	if weatherResult.Code != "200" {
		return fmt.Sprintf("Weather data unavailable for: %s", location), nil
	}

	output := fmt.Sprintf(`Weather for %s, %s:

Current Conditions:
- Temperature: %s°C
- Humidity: %s%%
- Pressure: %s hPa
- Wind: %s, %s km/h
- Conditions: %s`,
		loc.Name, loc.Adm1,
		weatherResult.Now.Temp,
		weatherResult.Now.Humidity,
		weatherResult.Now.Pressure,
		weatherResult.Now.WindDir,
		weatherResult.Now.WindSpeed,
		weatherResult.Now.Text)

	return output, nil
}

// ============================================================
// Helper Functions
// ============================================================

// urlEncode encodes a string for use in a URL
func urlEncode(s string) string {
	// Simple URL encoding for common characters
	s = replaceAll(s, " ", "%20")
	s = replaceAll(s, "&", "%26")
	s = replaceAll(s, "=", "%3D")
	return s
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i <= len(s)-len(old) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

// extractURLFromTask extracts URL from task description for cookie matching
func extractURLFromTask(task string) string {
	// Common URL patterns
	patterns := []string{
		"https://",
		"http://",
	}

	for _, pattern := range patterns {
		idx := strings.Index(task, pattern)
		if idx != -1 {
			// Extract URL from task
			url := task[idx:]
			// Find end of URL
			endChars := []string{" ", "\n", "\t", "。", "，", "！", "？", ")", "]", "}"}
			endIdx := len(url)
			for _, endChar := range endChars {
				if i := strings.Index(url, endChar); i != -1 && i < endIdx {
					endIdx = i
				}
			}
			return url[:endIdx]
		}
	}

	// Try to extract domain name from task text
	domainKeywords := []string{
		"csdn", "csdn.net",
		"github", "github.com",
		"zhihu", "zhihu.com",
		"baidu", "baidu.com",
		"weibo", "weibo.com",
		"juejin", "juejin.cn",
	}

	taskLower := strings.ToLower(task)
	for _, domain := range domainKeywords {
		if strings.Contains(taskLower, domain) {
			return domain
		}
	}

	return ""
}
