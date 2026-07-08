package agent

import (
	"context"
	"time"
)

// DefaultAgents returns the default agent configurations
// These are inserted into MongoDB on first startup if no agents exist
func DefaultAgents() []*Agent {
	now := time.Now()

	return []*Agent{
		// Main Agent - 主调度 Agent
		{
			ID:                "main-agent",
			Name:              "Main Agent",
			Description:       "主调度 Agent，负责理解用户意图并分配任务",
			Instructions:      "你是主调度助手，根据用户请求分配给合适的 Agent。", // fallback when harness is down
			PromptTemplateKey: "agent-main-dispatch",
			Tools:             []string{},
			Handoffs:          []string{"researcher-agent", "coder-agent", "analyst-agent", "browser-agent"},
			Model:             "",
			MaxTokens:         4096,
			Temperature:       0.7,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// Researcher Agent - 研究 Agent
		{
			ID:                "researcher-agent",
			Name:              "Researcher Agent",
			Description:       "研究 Agent，负责信息搜索和知识检索",
			Instructions:      "你是研究助手，负责搜索信息和检索知识。", // fallback
			PromptTemplateKey: "agent-researcher",
			Tools:             []string{"web_search", "knowledge_search", "weather"},
			Handoffs:          []string{"main-agent", "coder-agent"},
			Model:             "",
			MaxTokens:         4096,
			Temperature:       0.7,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// Coder Agent - 编程 Agent
		{
			ID:                "coder-agent",
			Name:              "Coder Agent",
			Description:       "编程 Agent，负责代码编写和执行",
			Instructions:      "你是编程助手，负责编写、调试和优化代码。", // fallback
			PromptTemplateKey: "agent-coder",
			Tools:             []string{"code_execute", "file_read", "file_write", "calculator"},
			Handoffs:          []string{"main-agent", "analyst-agent"},
			Model:             "",
			MaxTokens:         4096,
			Temperature:       0.7,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// Analyst Agent - 分析 Agent
		{
			ID:                "analyst-agent",
			Name:              "Analyst Agent",
			Description:       "分析 Agent，负责数据分析和可视化",
			Instructions:      "你是数据分析助手，负责数据分析和生成洞察报告。", // fallback
			PromptTemplateKey: "agent-analyst",
			Tools:             []string{"data_analysis", "visualization", "code_execute"},
			Handoffs:          []string{"main-agent"},
			Model:             "",
			MaxTokens:         4096,
			Temperature:       0.7,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// Browser Agent - 浏览器自动化 Agent
		{
			ID:                "browser-agent",
			Name:              "Browser Agent",
			Description:       "浏览器自动化 Agent，负责网页操作和数据采集",
			Instructions:      "你是浏览器自动化助手，负责网页操作和数据采集。", // fallback
			PromptTemplateKey: "agent-browser",
			Tools:             []string{"browser_navigate", "browser_click", "browser_type", "browser_extract", "browser_scroll", "browser_wait"},
			Handoffs:          []string{"main-agent", "researcher-agent"},
			Model:             "",
			MaxTokens:         4096,
			Temperature:       0.7,
			ToolConfig: map[string]ToolSpecificConfig{
				"browser_navigate": {
					APIKey:  "",  // 从环境变量读取
					BaseURL: "",
					Model:   "",
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// InitializeDefaultAgents inserts default agents into MongoDB if no agents exist
func InitializeDefaultAgents(ctx context.Context, store AgentStore) (int, error) {
	// Check if agents already exist
	count, err := store.Count(ctx)
	if err != nil {
		return 0, err
	}

	if count > 0 {
		return 0, nil // Already have agents, skip initialization
	}

	// Insert default agents
	defaults := DefaultAgents()
	inserted := 0

	for _, agent := range defaults {
		if err := store.Save(ctx, agent); err != nil {
			continue // Skip if error
		}
		inserted++
	}

	return inserted, nil
}