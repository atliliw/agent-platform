package main

import (
	"context"
	"fmt"
	"time"

	"agent-platform/pkg/browseragent"
	"agent-platform/pkg/mongodb"
)

func main() {
	// ============================================================
	// 方式 1: 手动设置 Cookie（原有方式）
	// ============================================================
	fmt.Println("=== 方式 1: 手动设置 Cookie ===")

	cookies := []browseragent.Cookie{
		{Name: "UserName", Value: "m0_54140879", Domain: ".csdn.net"},
		{Name: "UserInfo", Value: "818a4e24e3e94ee686ebc30893c5cd37", Domain: ".csdn.net"},
		{Name: "UserToken", Value: "818a4e24e3e94ee686ebc30893c5cd37", Domain: ".csdn.net"},
		{Name: "UN", Value: "m0_54140879", Domain: ".csdn.net"},
		{Name: "AU", Value: "81C", Domain: ".csdn.net"},
	}

	apiKey := "sk-xxx"
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"
	model := "qwen-plus"

	llmClient := browseragent.NewOpenAIClient(apiKey, baseURL, model)
	browser := browseragent.NewBrowser()

	agent := browseragent.New(llmClient, browser,
		browseragent.WithMaxSteps(10),
		browseragent.WithDebug(true),
		browseragent.WithCookies(cookies), // 手动传入 Cookie
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := agent.Run(ctx, "打开 https://blog.csdn.net/m0_54140879 查看文章列表")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result.Answer)
	}

	// ============================================================
	// 方式 2: 使用 MongoDB 存储 Cookie（新方式）
	// ============================================================
	fmt.Println("\n=== 方式 2: MongoDB Cookie 存储 ===")

	// 连接 MongoDB
	mongoClient, err := mongodb.NewClient("mongodb://localhost:27017")
	if err != nil {
		fmt.Printf("MongoDB connection failed: %v\n", err)
		fmt.Println("跳过 MongoDB 测试...")
		return
	}
	defer mongoClient.Disconnect(context.Background())

	db := mongoClient.Database("agent_platform")
	cookieStorage := browseragent.NewMongoCookieStorage(db)

	// 创建新的 Agent，配置 Cookie 存储
	agent2 := browseragent.New(llmClient, browseragent.NewBrowser(),
		browseragent.WithMaxSteps(10),
		browseragent.WithDebug(true),
		browseragent.WithCookieStorage(cookieStorage),  // 配置存储
		browseragent.WithUserContext("user-123", "tenant-default"), // 用户上下文
		browseragent.WithAutoSaveCookie(true),  // 自动保存登录后的 Cookie
	)

	// 首次运行 - 登录并保存 Cookie
	fmt.Println("\n--- 首次运行（登录并保存 Cookie）---")
	loginCtx, loginCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer loginCancel()

	loginResult, err := agent2.RunWithDomain(loginCtx, "打开 https://example.com 登录系统", ".example.com")
	if err != nil {
		fmt.Printf("Login Error: %v\n", err)
	} else {
		fmt.Printf("Login Result: %s\n", loginResult.Answer)
	}

	// 第二次运行 - 自动加载已保存的 Cookie
	fmt.Println("\n--- 第二次运行（自动加载 Cookie）---")
	agent3 := browseragent.New(llmClient, browseragent.NewBrowser(),
		browseragent.WithMaxSteps(10),
		browseragent.WithDebug(true),
		browseragent.WithCookieStorage(cookieStorage),
		browseragent.WithUserContext("user-123", "tenant-default"),
	)

	// 不需要手动设置 Cookie，会自动从 MongoDB 加载
	visitCtx, visitCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer visitCancel()

	visitResult, err := agent3.RunWithDomain(visitCtx, "打开 https://example.com/dashboard 查看数据", ".example.com")
	if err != nil {
		fmt.Printf("Visit Error: %v\n", err)
	} else {
		fmt.Printf("Visit Result: %s\n", visitResult.Answer)
	}

	// ============================================================
	// 管理 Cookie
	// ============================================================
	fmt.Println("\n=== Cookie 管理 ===")

	// 查看所有已存储的 Cookie
	allCookies, err := cookieStorage.GetAll(context.Background(), "user-123", "tenant-default")
	if err != nil {
		fmt.Printf("GetAll Error: %v\n", err)
	} else {
		fmt.Printf("已存储 %d 个域名\n", len(allCookies))
		for _, c := range allCookies {
			fmt.Printf("  - %s: %s (Domain: %s)\n", c.Name, c.Value[:20]+"...", c.Domain)
		}
	}

	// 删除某个域名的 Cookie
	err = cookieStorage.Delete(context.Background(), "user-123", "tenant-default", ".example.com")
	if err != nil {
		fmt.Printf("Delete Error: %v\n", err)
	} else {
		fmt.Println("已删除 .example.com 的所有 Cookie")
	}
}