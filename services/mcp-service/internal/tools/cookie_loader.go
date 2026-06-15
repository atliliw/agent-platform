// Package tools provides tool implementations
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-platform/pkg/browseragent"
)

// CookieLoader loads cookies from Gateway API
type CookieLoader struct {
	gatewayURL string
	userID     string
	tenantID   string
}

// NewCookieLoader creates a new cookie loader
func NewCookieLoader(gatewayURL, userID, tenantID string) *CookieLoader {
	if gatewayURL == "" {
		gatewayURL = "http://gateway:9000"
	}
	if userID == "" {
		userID = "default"
	}
	if tenantID == "" {
		tenantID = "default"
	}
	return &CookieLoader{
		gatewayURL: gatewayURL,
		userID:     userID,
		tenantID:   tenantID,
	}
}

// LoadCookies loads cookies for a domain from Gateway API
func (l *CookieLoader) LoadCookies(ctx context.Context, domain string) ([]browseragent.Cookie, error) {
	fmt.Printf("CookieLoader: 尝试加载Cookie, domain=%s, gateway=%s\n", domain, l.gatewayURL)

	// Extract domain from URL if needed
	if strings.HasPrefix(domain, "http") {
		// Parse URL to get domain
		parts := strings.Split(domain, "/")
		if len(parts) >= 3 {
			host := parts[2]
			// Convert to cookie domain format (e.g., .csdn.net)
			if strings.Contains(host, ".") {
				// Get the last two parts for cookie domain
				hostParts := strings.Split(host, ".")
				if len(hostParts) >= 2 {
					domain = "." + strings.Join(hostParts[len(hostParts)-2:], ".")
				} else {
					domain = host
				}
			} else {
				domain = host
			}
		}
	}

	fmt.Printf("CookieLoader: 处理后的域名: %s\n", domain)

	// Call Gateway API to get cookies
	url := fmt.Sprintf("%s/api/v2/cookies?domain=%s&user_id=%s&tenant_id=%s",
		l.gatewayURL, domain, l.userID, l.tenantID)

	fmt.Printf("CookieLoader: API URL: %s\n", url)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Tenant-ID", l.tenantID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch cookies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Parse response
	var apiResp struct {
		Code int `json:"code"`
		Data struct {
			Cookies []browseragent.Cookie `json:"cookies"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("api error: code %d", apiResp.Code)
	}

	return apiResp.Data.Cookies, nil
}

// LoadCookiesForURL loads cookies that match the given URL
func (l *CookieLoader) LoadCookiesForURL(ctx context.Context, url string) ([]browseragent.Cookie, error) {
	// Extract domain from URL
	domain := extractDomain(url)
	if domain == "" {
		return nil, nil
	}

	fmt.Printf("CookieLoader: 提取的原始域名: %s\n", domain)

	var cookies []browseragent.Cookie

	// Try multiple domain patterns to find matching cookies
	// For example, for mp.csdn.net, try: mp.csdn.net, .mp.csdn.net, csdn.net, .csdn.net
	domainPatterns := []string{
		domain,                        // mp.csdn.net
		"." + domain,                  // .mp.csdn.net
		getParentDomain(domain),       // csdn.net
		"." + getParentDomain(domain), // .csdn.net
	}

	// Also try the input URL if it's already a domain pattern (like .csdn.net)
	if strings.HasPrefix(url, ".") || !strings.Contains(url, "/") {
		domainPatterns = append(domainPatterns, url)
		if !strings.HasPrefix(url, ".") {
			domainPatterns = append(domainPatterns, "."+url)
		}
	}

	seen := make(map[string]bool) // Avoid duplicate cookies
	for _, pattern := range domainPatterns {
		if pattern == "" || pattern == "." || seen[pattern] {
			continue
		}
		seen[pattern] = true
		cks, err := l.LoadCookies(ctx, pattern)
		if err != nil {
			fmt.Printf("CookieLoader: 加载 %s 失败: %v\n", pattern, err)
		} else if len(cks) > 0 {
			fmt.Printf("CookieLoader: 从 %s 加载了 %d 个 Cookie\n", pattern, len(cks))
			cookies = append(cookies, cks...)
		}
	}

	return cookies, nil
}

// getParentDomain extracts the parent domain (e.g., csdn.net from blog.csdn.net)
func getParentDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return domain
}

// extractDomain extracts domain from URL
func extractDomain(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	// Remove path
	idx := strings.Index(url, "/")
	if idx > 0 {
		url = url[:idx]
	}

	// Remove port
	idx = strings.Index(url, ":")
	if idx > 0 {
		url = url[:idx]
	}

	return url
}