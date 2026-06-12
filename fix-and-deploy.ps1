# Agent Platform 一键修复和部署脚本 (Windows PowerShell)
# 修复端口配置问题并重新部署

Write-Host "========================================" -ForegroundColor Green
Write-Host "Agent Platform 一键修复和部署" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""

# 1. 显示当前修复内容
Write-Host "[1/6] 修复说明:" -ForegroundColor Cyan
Write-Host "  - agent-service 端口: 50007 -> 50006" -ForegroundColor Yellow
Write-Host "  - harness-service 端口: 50006 -> 50007" -ForegroundColor Yellow
Write-Host "  - gateway/chat-service 配置同步更新" -ForegroundColor Yellow
Write-Host ""

# 2. 验证配置文件
Write-Host "[2/6] 验证配置文件..." -ForegroundColor Cyan

$agentConfig = Get-Content "services\agent-service\config.yaml" -Raw
if ($agentConfig -match "grpc_port: 50006") {
    Write-Host "  ✓ agent-service 端口配置正确 (50006)" -ForegroundColor Green
} else {
    Write-Host "  ! agent-service 端口需要修复" -ForegroundColor Red
}

$harnessConfig = Get-Content "services\harness-service\config.yaml" -Raw
if ($harnessConfig -match "grpc_port: 50007") {
    Write-Host "  ✓ harness-service 端口配置正确 (50007)" -ForegroundColor Green
} else {
    Write-Host "  ! harness-service 端口需要修复" -ForegroundColor Red
}

# 3. 检查 Docker
Write-Host ""
Write-Host "[3/6] 检查 Docker..." -ForegroundColor Cyan
try {
    $dockerVersion = docker --version
    Write-Host "  Docker 版本: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "  [ERROR] Docker 未安装或未启动" -ForegroundColor Red
    Write-Host "  请先安装并启动 Docker Desktop" -ForegroundColor Yellow
    exit 1
}

# 4. 停止旧服务
Write-Host ""
Write-Host "[4/6] 停止旧服务..." -ForegroundColor Cyan
try {
    docker compose down --remove-orphans 2>$null
    Write-Host "  ✓ 旧服务已停止" -ForegroundColor Green
} catch {
    Write-Host "  ! 没有运行中的服务" -ForegroundColor Yellow
}

# 5. 构建镜像
Write-Host ""
Write-Host "[5/6] 构建 Docker 镜像..." -ForegroundColor Cyan
Write-Host "  这可能需要几分钟，请耐心等待..." -ForegroundColor Yellow
docker compose build --no-cache

if ($LASTEXITCODE -ne 0) {
    Write-Host "  [ERROR] 镜像构建失败" -ForegroundColor Red
    Write-Host "  请检查网络连接或配置 Docker 镜像加速器" -ForegroundColor Yellow
    exit 1
}
Write-Host "  ✓ 镜像构建完成" -ForegroundColor Green

# 6. 启动服务
Write-Host ""
Write-Host "[6/6] 启动服务..." -ForegroundColor Cyan
docker compose up -d

Start-Sleep -Seconds 10

# 显示服务状态
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "服务状态" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
docker compose ps

# 健康检查
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "健康检查" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

Start-Sleep -Seconds 5

# 检查 Gateway
try {
    $response = Invoke-WebRequest -Uri "http://localhost:9000/api/v2/mcp/tools" -Method GET -TimeoutSec 10 -UseBasicParsing
    Write-Host "  ✓ Gateway 正常 (端口 9000)" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Gateway 未响应" -ForegroundColor Red
}

# 检查 Chat Service
try {
    $response = Invoke-WebRequest -Uri "http://localhost:50001/health" -Method GET -TimeoutSec 5 -UseBasicParsing -ErrorAction SilentlyContinue
    Write-Host "  ✓ Chat Service 正常 (端口 50001)" -ForegroundColor Green
} catch {
    Write-Host "  ? Chat Service 可能还在启动中" -ForegroundColor Yellow
}

# 检查 Agent Service
try {
    $response = Invoke-WebRequest -Uri "http://localhost:50006/health" -Method GET -TimeoutSec 5 -UseBasicParsing -ErrorAction SilentlyContinue
    Write-Host "  ✓ Agent Service 正常 (端口 50006)" -ForegroundColor Green
} catch {
    Write-Host "  ? Agent Service 可能还在启动中" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "部署完成!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "访问地址:" -ForegroundColor Yellow
Write-Host "  - Gateway API:    http://localhost:9000" -ForegroundColor White
Write-Host "  - 服务器访问:     http://192.168.10.100:9000" -ForegroundColor White
Write-Host ""
Write-Host "测试命令:" -ForegroundColor Yellow
Write-Host "  curl http://localhost:9000/api/v2/mcp/tools" -ForegroundColor White
Write-Host "  curl -X POST http://localhost:9000/api/v2/chat -H 'Content-Type: application/json' -d '{\"message\":\"你好\"}'" -ForegroundColor White
Write-Host ""
Write-Host "查看日志:" -ForegroundColor Yellow
Write-Host "  docker compose logs -f [service-name]" -ForegroundColor White
Write-Host ""
Write-Host "停止服务:" -ForegroundColor Yellow
Write-Host "  docker compose down" -ForegroundColor White
Write-Host ""
