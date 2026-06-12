# Agent Platform 快速测试脚本 (Windows PowerShell)
# 在本地或服务器上运行此脚本验证部署

param(
    [string]$BaseUrl = "http://192.168.10.100:9000"
)

$TenantId = "test-tenant-" + (Get-Date -Format "yyyyMMddHHmmss")

Write-Host "========================================" -ForegroundColor Green
Write-Host "Agent Platform 测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "API 地址: $BaseUrl"
Write-Host "租户 ID: $TenantId"
Write-Host ""

function Test-Api {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Endpoint,
        [string]$Data
    )

    Write-Host "测试: $Name" -ForegroundColor Yellow
    Write-Host "请求: $Method $Endpoint"

    try {
        if ($Data) {
            $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -ContentType "application/json" -Body $Data -Headers @{"X-Tenant-ID" = $TenantId} -TimeoutSec 30
        } else {
            $response = Invoke-RestMethod -Uri "$BaseUrl$Endpoint" -Method $Method -Headers @{"X-Tenant-ID" = $TenantId} -TimeoutSec 30
        }

        Write-Host "✓ 成功" -ForegroundColor Green
        $response | ConvertTo-Json -Depth 3 | Out-String | Write-Host
    } catch {
        Write-Host "✗ 失败: $($_.Exception.Message)" -ForegroundColor Red
    }

    Write-Host "----------------------------------------"
}

# 1. 基础服务测试
Write-Host "========================================" -ForegroundColor Green
Write-Host "1. 基础服务测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Test-Api -Name "MCP 工具列表" -Method "GET" -Endpoint "/api/v2/mcp/tools"
Test-Api -Name "Agent 列表" -Method "GET" -Endpoint "/api/v2/agents"

# 2. 工具调用测试
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "2. 工具调用测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Test-Api -Name "计算器工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{"name":"calculator","arguments":"{\"expression\":\"(10+5)*2\"}"}'
Test-Api -Name "时间工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{"name":"time","arguments":"{}"}'
Test-Api -Name "数据分析工具" -Method "POST" -Endpoint "/api/v2/mcp/call" -Data '{"name":"data_analysis","arguments":"{\"data\":[1,2,3,4,5,6,7,8,9,10]}"}'

# 3. 对话功能测试
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "3. 对话功能测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Test-Api -Name "基础对话" -Method "POST" -Endpoint "/api/v2/chat" -Data '{"message":"你好，请用一句话介绍你自己","tenant_id":"' + $TenantId + '"}'

# 4. Agent + 工具测试
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "4. Agent + 工具组合测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Test-Api -Name "Agent 调用计算器" -Method "POST" -Endpoint "/api/v2/chat" -Data '{"message":"请帮我计算 123 + 456 的结果","tenant_id":"' + $TenantId + '"}'

# 5. 记忆功能测试
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "5. 记忆功能测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Write-Host "测试: 存储用户信息" -ForegroundColor Yellow
$chatData = @{
    message = "我叫张三，我喜欢编程和看电影"
    tenant_id = $TenantId
    user_id = "user-001"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$BaseUrl/api/v2/chat" -Method POST -ContentType "application/json" -Body $chatData -TimeoutSec 60
    $sessionId = $response.data.session_id
    Write-Host "✓ 对话成功, Session ID: $sessionId" -ForegroundColor Green

    # 短期记忆测试
    Write-Host ""
    Write-Host "测试: 短期记忆（同一 Session）" -ForegroundColor Yellow
    $followUpData = @{
        session_id = $sessionId
        message = "我叫什么？我喜欢什么？"
        tenant_id = $TenantId
    } | ConvertTo-Json

    $followUpResponse = Invoke-RestMethod -Uri "$BaseUrl/api/v2/chat" -Method POST -ContentType "application/json" -Body $followUpData -TimeoutSec 60
    Write-Host "✓ 短期记忆测试成功" -ForegroundColor Green
    Write-Host "响应: $($followUpResponse.data.content.Substring(0, [Math]::Min(200, $followUpResponse.data.content.Length)))..."

    # 长期记忆测试
    Write-Host ""
    Write-Host "测试: 长期记忆（新 Session，同一用户）" -ForegroundColor Yellow
    $newSessionData = @{
        message = "你还记得我的名字和爱好吗？"
        tenant_id = $TenantId
        user_id = "user-001"
    } | ConvertTo-Json

    Start-Sleep -Seconds 2  # 等待记忆保存
    $newSessionResponse = Invoke-RestMethod -Uri "$BaseUrl/api/v2/chat" -Method POST -ContentType "application/json" -Body $newSessionData -TimeoutSec 60
    Write-Host "✓ 长期记忆测试完成" -ForegroundColor Green
    Write-Host "响应: $($newSessionResponse.data.content.Substring(0, [Math]::Min(200, $newSessionResponse.data.content.Length)))..."

} catch {
    Write-Host "✗ 记忆测试失败: $($_.Exception.Message)" -ForegroundColor Red
}

# 总结
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "测试完成" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "如果所有测试都显示绿色 ✓，说明部署成功！" -ForegroundColor Green
Write-Host ""
Write-Host "查看日志命令:" -ForegroundColor Yellow
Write-Host "  docker-compose logs -f chat-service"
Write-Host "  docker-compose logs -f agent-service"
Write-Host "  docker-compose logs -f mcp-service"
Write-Host "  docker-compose logs -f memory-service"
