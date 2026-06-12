# Agent Platform API 测试脚本 (Windows PowerShell)

$BaseUrl = "http://localhost:9000"

Write-Host "========================================" -ForegroundColor Green
Write-Host "Agent Platform API 测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

function Test-Api {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Endpoint,
        [string]$Data
    )

    Write-Host ""
    Write-Host "测试: $Name" -ForegroundColor Cyan
    Write-Host "请求: $Method $Endpoint"

    try {
        if ($Data) {
            $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -ContentType "application/json" -Body $Data -TimeoutSec 30
        } else {
            $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -TimeoutSec 30
        }

        Write-Host "[SUCCESS] 成功" -ForegroundColor Green
        $response | ConvertTo-Json -Depth 5 | Out-String | Write-Host
        return $true
    } catch {
        Write-Host "[ERROR] 失败: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
}

# 1. 测试 MCP 工具列表
Test-Api -Name "MCP 工具列表" -Method "GET" -Endpoint "/api/v2/mcp/tools"

# 2. 测试对话
Test-Api -Name "对话接口" -Method "POST" -Endpoint "/api/v2/chat" -Data '{
    "message": "你好，请介绍一下你自己",
    "tenant_id": "test-tenant",
    "user_id": "test-user"
}'

# 3. 测试会话列表
Test-Api -Name "会话列表" -Method "POST" -Endpoint "/api/v2/sessions" -Data '{
    "tenant_id": "test-tenant",
    "user_id": "test-user"
}'

# 4. 测试知识库搜索
Test-Api -Name "知识库搜索" -Method "POST" -Endpoint "/api/v2/knowledge/search" -Data '{
    "query": "测试搜索",
    "top_k": 5,
    "tenant_id": "test-tenant"
}'

# 5. 测试 Agent 列表
Test-Api -Name "Agent 列表" -Method "GET" -Endpoint "/api/v2/agents"

# 6. 测试计算器工具
Test-Api -Name "计算器工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{
    "name": "calculator",
    "arguments": "{\"expression\": \"2 + 3 * 4\"}"
}'

# 7. 测试时间工具
Test-Api -Name "时间工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{
    "name": "time",
    "arguments": "{}"
}'

# 8. 测试数据分析工具
Test-Api -Name "数据分析工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{
    "name": "data_analysis",
    "arguments": "{\"data\": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]}"
}'

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "测试完成" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
