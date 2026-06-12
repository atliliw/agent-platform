# Agent Platform 部署脚本 (Windows PowerShell)

Write-Host "========================================" -ForegroundColor Green
Write-Host "Agent Platform 部署脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

# 检查 Docker
Write-Host "[INFO] 检查 Docker..." -ForegroundColor Cyan
try {
    $dockerVersion = docker --version
    Write-Host "Docker 版本: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Docker 未安装，请先安装 Docker Desktop" -ForegroundColor Red
    exit 1
}

# 检查 Docker Compose
try {
    docker compose version | Out-Null
    $useComposeV2 = $true
} catch {
    try {
        docker-compose --version | Out-Null
        $useComposeV2 = $false
    } catch {
        Write-Host "[ERROR] Docker Compose 未安装" -ForegroundColor Red
        exit 1
    }
}

# 检查配置文件
Write-Host "[INFO] 检查配置文件..." -ForegroundColor Cyan
$configs = @(
    "services\gateway\config.yaml",
    "services\chat-service\config.yaml",
    "services\knowledge-service\config.yaml",
    "services\memory-service\config.yaml",
    "services\mcp-service\config.yaml",
    "services\agent-service\config.yaml"
)

foreach ($config in $configs) {
    if (-not (Test-Path $config)) {
        Write-Host "[ERROR] 配置文件不存在: $config" -ForegroundColor Red
        exit 1
    }
}
Write-Host "[INFO] 所有配置文件检查通过" -ForegroundColor Green

# 停止旧容器
Write-Host "[INFO] 停止旧容器..." -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose down --remove-orphans 2>$null
} else {
    docker-compose down --remove-orphans 2>$null
}

# 构建镜像
Write-Host "[INFO] 构建 Docker 镜像 (这可能需要几分钟)..." -ForegroundColor Cyan
Write-Host "请耐心等待..." -ForegroundColor Yellow
if ($useComposeV2) {
    docker compose build --no-cache
} else {
    docker-compose build --no-cache
}

# 启动服务
Write-Host "[INFO] 启动服务..." -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose up -d
} else {
    docker-compose up -d
}

# 等待服务就绪
Write-Host "[INFO] 等待服务启动..." -ForegroundColor Cyan
Start-Sleep -Seconds 15

# 显示服务状态
Write-Host ""
Write-Host "[INFO] 服务状态:" -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose ps
} else {
    docker-compose ps
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "部署完成!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "服务地址:" -ForegroundColor Yellow
Write-Host "  - Gateway API:    http://localhost:9000"
Write-Host "  - Qdrant:         http://localhost:6333"
Write-Host "  - MongoDB:        localhost:27017"
Write-Host "  - Redis:          localhost:6379"
Write-Host ""
Write-Host "测试 API:" -ForegroundColor Yellow
Write-Host "  curl -X POST http://localhost:9000/api/v2/chat -H 'Content-Type: application/json' -d '{\"message\":\"你好\",\"tenant_id\":\"test\"}'"
Write-Host ""
Write-Host "查看日志:" -ForegroundColor Yellow
Write-Host "  docker compose logs -f [service-name]"
Write-Host ""
Write-Host "停止服务:" -ForegroundColor Yellow
Write-Host "  docker compose down"
Write-Host ""

# 健康检查
Write-Host "[INFO] 执行健康检查..." -ForegroundColor Cyan
Start-Sleep -Seconds 5

try {
    $response = Invoke-WebRequest -Uri "http://localhost:9000/api/v2/mcp/tools" -Method GET -TimeoutSec 10 -UseBasicParsing
    if ($response.StatusCode -eq 200) {
        Write-Host "[SUCCESS] Gateway 服务正常响应!" -ForegroundColor Green
    }
} catch {
    Write-Host "[WARN] Gateway 可能还在启动中，请稍后手动检查" -ForegroundColor Yellow
}