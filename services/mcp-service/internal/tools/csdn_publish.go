// Package tools provides CSDN article publishing via API
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/pkg/browseragent"
)

// CSDNPublishTool publishes articles to CSDN via API
type CSDNPublishTool struct {
	cookieLoader *CookieLoader
}

// NewCSDNPublishTool creates a new CSDN publish tool
func NewCSDNPublishTool() *CSDNPublishTool {
	return &CSDNPublishTool{
		cookieLoader: NewCookieLoader("", "default", "default"),
	}
}

// GetInfo returns tool information for LLM
func (t *CSDNPublishTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "csdn_publish",
		Description: "通过 API 在 CSDN 发布文章。绕过浏览器自动化检测，直接使用 Cookie 调用 CSDN 发布接口。适用于：发表博客、发布文章等任务。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "文章标题",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "文章内容（Markdown 格式）",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "文章标签（可选）",
					"items":       map[string]interface{}{"type": "string"},
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "文章分类（可选，如：后端、前端、人工智能）",
				},
			},
			"required": []string{"title", "content"},
		},
	}
}

// Execute executes the CSDN publish tool
func (t *CSDNPublishTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes with config
func (t *CSDNPublishTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)

	if title == "" || content == "" {
		return "", fmt.Errorf("title and content are required")
	}

	fmt.Printf("CSDNPublish: 开始发布文章，标题: %s\n", title)

	// 加载 CSDN Cookie
	cookies, err := t.cookieLoader.LoadCookiesForURL(ctx, ".csdn.net")
	if err != nil || len(cookies) == 0 {
		return "", fmt.Errorf("未找到 CSDN Cookie，请先登录 CSDN")
	}

	fmt.Printf("CSDNPublish: 已加载 %d 个 Cookie\n", len(cookies))

	// 构建 Cookie 字符串
	cookieStr := buildCookieString(cookies)
	fmt.Printf("CSDNPublish: Cookie 字符串长度: %d\n", len(cookieStr))

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 方法1: 尝试通过编辑器 API 发布
	fmt.Printf("CSDNPublish: 尝试方法1 - 编辑器 API...\n")
	result, err := t.publishViaEditorAPI(ctx, client, cookieStr, title, content, args)
	if err == nil {
		return fmt.Sprintf("文章发布成功！\n\n标题: %s\n%s", title, result), nil
	}
	fmt.Printf("CSDNPublish: 方法1失败: %v\n", err)

	// 方法2: 尝试旧版 API
	fmt.Printf("CSDNPublish: 尝试方法2 - 旧版 API...\n")
	result2, err2 := t.publishViaOldAPI(ctx, client, cookieStr, title, content, args)
	if err2 == nil {
		return fmt.Sprintf("文章发布成功（旧版API）！\n\n标题: %s\n%s", title, result2), nil
	}
	fmt.Printf("CSDNPublish: 方法2失败: %v\n", err2)

	// 方法3: 使用浏览器自动化的提示
	return fmt.Sprintf("API 发布方式暂时不可用。请使用浏览器方式发布文章:\n\n1. 打开 https://mp.csdn.net/mp_blog/creation/editor\n2. 输入标题: %s\n3. 输入内容\n4. 点击发布\n\n或者请确保 Cookie 包含完整的登录信息（包括 sessionId 和 csrf token）。\n\nAPI 错误信息:\n- 编辑器API: %v\n- 旧版API: %v", title, err, err2), nil
}

// publishViaEditorAPI 尝试通过编辑器页面 API 发布
func (t *CSDNPublishTool) publishViaEditorAPI(ctx context.Context, client *http.Client, cookieStr string, title, content string, args map[string]interface{}) (string, error) {
	// CSDN 新版编辑器 API
	apiURL := "https://mp.csdn.net/mp_blog/creation/save"

	// 构建请求体
	payload := map[string]interface{}{
		"title":           title,
		"markdowncontent": content,
		"content":         content,
		"type":            "original",
		"status":          0,
		"readType":        "public",
		"level":           1,
		"editType":        1,
		"coverImg":        "",
		"isNewImg":        false,
		"articleId":       "",
		"draftId":         "",
		"private":         false,
	}

	// 标签
	tags := []string{"技术"}
	if tagArr, ok := args["tags"].([]interface{}); ok && len(tagArr) > 0 {
		tagStrs := []string{}
		for _, tag := range tagArr {
			if t, ok := tag.(string); ok && t != "" {
				tagStrs = append(tagStrs, t)
			}
		}
		if len(tagStrs) > 0 {
			tags = tagStrs
		}
	}
	payload["tags"] = tags

	// 分类
	category := "后端"
	if c, ok := args["category"].(string); ok && c != "" {
		category = c
	}
	payload["categories"] = []string{category}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://mp.csdn.net/mp_blog/creation/editor")
	req.Header.Set("Origin", "https://mp.csdn.net")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	fmt.Printf("CSDNPublish: EditorAPI Response status: %d, body: %s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode == 405 || resp.StatusCode == 403 {
		return "", fmt.Errorf("API denied (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ArticleID string `json:"articleId"`
			URL       string `json:"url"`
			DraftID   string `json:"draftId"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
	}

	if result.Code != 200 {
		return "", fmt.Errorf("API error: %s (code: %d)", result.Message, result.Code)
	}

	articleURL := result.Data.URL
	if articleURL == "" && result.Data.ArticleID != "" {
		articleURL = fmt.Sprintf("https://blog.csdn.net/article/details/%s", result.Data.ArticleID)
	}

	return fmt.Sprintf("文章ID: %s\n链接: %s", result.Data.ArticleID, articleURL), nil
}

// publishViaOldAPI 尝试旧版 API
func (t *CSDNPublishTool) publishViaOldAPI(ctx context.Context, client *http.Client, cookieStr string, title, content string, args map[string]interface{}) (string, error) {
	// CSDN 旧版发布 API
	apiURL := "https://mp.csdn.net/mp_blog/creation/saveArticle"

	// 构建表单数据
	formData := url.Values{}
	formData.Set("title", title)
	formData.Set("markdowncontent", content)
	formData.Set("content", content)
	formData.Set("type", "original")
	formData.Set("status", "0")
	formData.Set("readType", "public")
	formData.Set("level", "1")
	formData.Set("editType", "1")
	formData.Set("isNewImg", "false")
	formData.Set("coverImg", "")
	formData.Set("articleId", "")

	// 标签
	tags := "技术"
	if tagArr, ok := args["tags"].([]interface{}); ok && len(tagArr) > 0 {
		tagStrs := []string{}
		for _, tag := range tagArr {
			if t, ok := tag.(string); ok && t != "" {
				tagStrs = append(tagStrs, t)
			}
		}
		if len(tagStrs) > 0 {
			tags = strings.Join(tagStrs, ",")
		}
	}
	formData.Set("tags", tags)

	// 分类
	category := "后端"
	if c, ok := args["category"].(string); ok && c != "" {
		category = c
	}
	formData.Set("categories", category)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://mp.csdn.net/mp_blog/creation/editor")
	req.Header.Set("Origin", "https://mp.csdn.net")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	fmt.Printf("CSDNPublish: OldAPI Response status: %d, body: %s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode == 405 || resp.StatusCode == 403 {
		return "", fmt.Errorf("API denied (status %d)", resp.StatusCode)
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ArticleID string `json:"articleId"`
			URL       string `json:"url"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
	}

	if result.Code != 200 {
		return "", fmt.Errorf("API error: %s (code: %d)", result.Message, result.Code)
	}

	articleURL := result.Data.URL
	if articleURL == "" && result.Data.ArticleID != "" {
		articleURL = fmt.Sprintf("https://blog.csdn.net/article/details/%s", result.Data.ArticleID)
	}

	return fmt.Sprintf("文章ID: %s\n链接: %s", result.Data.ArticleID, articleURL), nil
}

// buildCookieString builds cookie string from cookie list
func buildCookieString(cookies []browseragent.Cookie) string {
	var sb strings.Builder
	for i, c := range cookies {
		if c.Name != "" && c.Value != "" {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(fmt.Sprintf("%s=%s", c.Name, c.Value))
		}
	}
	return sb.String()
}
