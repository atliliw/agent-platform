# Agent Platform Deploy Script

Write-Host "========================================" -ForegroundColor Green
Write-Host "Agent Platform Deployment" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

# Check Docker
Write-Host "[INFO] Checking Docker..." -ForegroundColor Cyan
try {
    $dockerVersion = docker --version
    Write-Host "Docker version: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Docker not installed" -ForegroundColor Red
    exit 1
}

# Check Docker Compose
$useComposeV2 = $false
try {
    docker compose version | Out-Null
    $useComposeV2 = $true
    Write-Host "Using Docker Compose V2" -ForegroundColor Green
} catch {
    try {
        docker-compose --version | Out-Null
        Write-Host "Using Docker Compose V1" -ForegroundColor Green
    } catch {
        Write-Host "[ERROR] Docker Compose not installed" -ForegroundColor Red
        exit 1
    }
}

# Stop old containers
Write-Host "[INFO] Stopping old containers..." -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose down --remove-orphans 2>$null
} else {
    docker-compose down --remove-orphans 2>$null
}

# Build images
Write-Host "[INFO] Building Docker images..." -ForegroundColor Cyan
Write-Host "Please wait..." -ForegroundColor Yellow
if ($useComposeV2) {
    docker compose build
} else {
    docker-compose build
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Build failed" -ForegroundColor Red
    exit 1
}

# Start services
Write-Host "[INFO] Starting services..." -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose up -d
} else {
    docker-compose up -d
}

# Wait for services
Write-Host "[INFO] Waiting for services..." -ForegroundColor Cyan
Start-Sleep -Seconds 15

# Show status
Write-Host ""
Write-Host "[INFO] Service status:" -ForegroundColor Cyan
if ($useComposeV2) {
    docker compose ps
} else {
    docker-compose ps
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Deployment Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Service URLs:" -ForegroundColor Yellow
Write-Host "  - Gateway API:    http://localhost:9000"
Write-Host "  - Qdrant:         http://localhost:6333"
Write-Host "  - MongoDB:        localhost:27017"
Write-Host "  - Redis:          localhost:6379"
Write-Host ""

# Health check
Write-Host "[INFO] Health check..." -ForegroundColor Cyan
Start-Sleep -Seconds 5

try {
    $response = Invoke-WebRequest -Uri "http://localhost:9000/health" -Method GET -TimeoutSec 10 -UseBasicParsing
    if ($response.StatusCode -eq 200) {
        Write-Host "[SUCCESS] Gateway is running!" -ForegroundColor Green
    }
} catch {
    Write-Host "[WARN] Gateway may still be starting, please check manually" -ForegroundColor Yellow
}
