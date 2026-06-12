# 同步修改到服务器并部署
# 在本地 Windows PowerShell 执行

$SERVER = "root@192.168.10.100"
$SSH_KEY = "$env:USERPROFILE\.ssh\demo_deploy_key"
$REMOTE_PATH = "/opt/agent-platform"

Write-Host "========================================" -ForegroundColor Green
Write-Host "同步修改到服务器" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

# 检查 SSH Key
if (-not (Test-Path $SSH_KEY)) {
    Write-Host "[ERROR] SSH Key 不存在: $SSH_KEY" -ForegroundColor Red
    Write-Host "请确认 Key 文件路径正确" -ForegroundColor Yellow
    exit 1
}

Write-Host "服务器: $SERVER" -ForegroundColor Cyan
Write-Host "远程路径: $REMOTE_PATH" -ForegroundColor Cyan
Write-Host ""

# 使用 scp 同步修改的文件
$filesToSync = @(
    "services/chat-service/internal/service/chat_service.go",
    "services/chat-service/config.yaml",
    "services/agent-service/config.yaml",
    "services/harness-service/config.yaml",
    "services/gateway/config.yaml"
)

foreach ($file in $filesToSync) {
    $localPath = $file
    $remotePath = "$REMOTE_PATH/$file"

    Write-Host "同步: $file" -ForegroundColor Yellow

    # 创建远程目录（如果不存在）
    $remoteDir = $remotePath.Substring(0, $remotePath.LastIndexOf('/'))
    ssh -i $SSH_KEY $SERVER "mkdir -p $remoteDir"

    # 同步文件
    scp -i $SSH_KEY $localPath "${SERVER}:${remotePath}"

    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ 成功" -ForegroundColor Green
    } else {
        Write-Host "  ✗ 失败" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "在服务器上重新部署" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

# 执行部署命令
$deployCommands = @"
cd $REMOTE_PATH/docker &&
docker compose down &&
docker compose build --no-cache chat-service agent-service gateway &&
docker compose up -d &&
docker compose ps
"@

Write-Host "执行部署命令..." -ForegroundColor Yellow
ssh -i $SSH_KEY $SERVER $deployCommands

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "部署完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "测试命令:" -ForegroundColor Yellow
Write-Host "  curl http://192.168.10.100:9000/api/v2/mcp/tools"
Write-Host "  curl -X POST http://192.168.10.100:9000/api/v2/chat -H 'Content-Type: application/json' -d '{`"message`":`"你好`",`"tenant_id`":`"test`"}'"
Write-Host ""
