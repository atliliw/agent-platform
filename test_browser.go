package main

import (
	"context"
	"fmt"
	"time"

	"agent-platform/pkg/browseragent"
)

func main() {
	// Cookie 列表
	cookies := []browseragent.Cookie{
		{Name: "UserName", Value: "m0_54140879", Domain: ".csdn.net"},
		{Name: "UserInfo", Value: "818a4e24e3e94ee686ebc30893c5cd37", Domain: ".csdn.net"},
		{Name: "UserToken", Value: "818a4e24e3e94ee686ebc30893c5cd37", Domain: ".csdn.net"},
		{Name: "UN", Value: "m0_54140879", Domain: ".csdn.net"},
		{Name: "AU", Value: "81C", Domain: ".csdn.net"},
	}

	// 创建 LLM 客户端 (使用环境变量)
	apiKey := "sk-xxx" // 替换为你的 API Key
	baseURL := "https://dashscope.aliyuncs.com/compatible-mode/v1"
	model := "qwen-plus"

	llmClient := browseragent.NewOpenAIClient(apiKey, baseURL, model)

	// 创建浏览器
	browser := browseragent.NewBrowser()

	// 创建 Agent
	agent := browseragent.New(llmClient, browser,
		browseragent.WithMaxSteps(10),
		browseragent.WithDebug(true),
	)

	// 设置 Cookie
	agent.SetCookies(cookies)

	// 执行任务
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := agent.Run(ctx, "打开 https://blog.csdn.net/m0_54140879 查看文章列表，告诉我有哪些文章")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %s\n", result.Answer)
	fmt.Printf("Steps: %d\n", result.Steps)
}