#!/bin/bash
# Agent Platform 快速测试脚本
# 在服务器上运行此脚本验证部署

BASE_URL="${1:-http://localhost:9000}"
TENANT_ID="test-tenant-$(date +%s)"

echo "========================================"
echo "Agent Platform 测试"
echo "========================================"
echo "API 地址: $BASE_URL"
echo "租户 ID: $TENANT_ID"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试函数
test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"

    echo -e "${YELLOW}测试: $name${NC}"
    echo "请求: $method $endpoint"

    if [ -n "$data" ]; then
        response=$(curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "X-Tenant-ID: $TENANT_ID" \
            -d "$data" \
            -w "\nHTTP_CODE:%{http_code}")
    else
        response=$(curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "X-Tenant-ID: $TENANT_ID" \
            -w "\nHTTP_CODE:%{http_code}")
    fi

    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    body=$(echo "$response" | grep -v "HTTP_CODE:")

    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo -e "${GREEN}✓ 成功 (HTTP $http_code)${NC}"
        # 显示响应摘要
        if command -v jq &> /dev/null; then
            echo "$body" | jq -C '.' 2>/dev/null | head -20
        else
            echo "$body" | head -c 500
        fi
    else
        echo -e "${RED}✗ 失败 (HTTP $http_code)${NC}"
        echo "$body" | head -c 500
    fi
    echo ""
    echo "----------------------------------------"
}

# 1. 测试 Gateway 健康检查
echo "========================================"
echo "1. 基础服务测试"
echo "========================================"

test_api "MCP 工具列表" "GET" "/api/v2/mcp/tools"

test_api "Agent 列表" "GET" "/api/v2/agents"

# 2. 测试工具调用
echo ""
echo "========================================"
echo "2. 工具调用测试"
echo "========================================"

test_api "计算器工具" "POST" "/api/v2/mcp/call" \
    '{"name":"calculator","arguments":"{\"expression\":\"(10+5)*2\"}"}'

test_api "时间工具" "POST" "/api/v2/mcp/call" \
    '{"name":"time","arguments":"{}"}'

test_api "数据分析工具" "POST" "/api/v2/mcp/call" \
    '{"name":"data_analysis","arguments":"{\"data\":[1,2,3,4,5,6,7,8,9,10]}"}'

# 3. 测试对话
echo ""
echo "========================================"
echo "3. 对话功能测试"
echo "========================================"

test_api "基础对话" "POST" "/api/v2/chat" \
    '{"message":"你好，请用一句话介绍你自己","tenant_id":"'"$TENANT_ID"'"}'

# 4. 测试 Agent + 工具
echo ""
echo "========================================"
echo "4. Agent + 工具组合测试"
echo "========================================"

test_api "Agent 调用计算器" "POST" "/api/v2/chat" \
    '{"message":"请帮我计算 123 + 456 的结果","tenant_id":"'"$TENANT_ID"'"}'

# 5. 测试记忆功能
echo ""
echo "========================================"
echo "5. 记忆功能测试"
echo "========================================"

# 第一轮：存储信息
echo -e "${YELLOW}测试: 存储用户信息${NC}"
response=$(curl -s -X POST "$BASE_URL/api/v2/chat" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -d '{"message":"我叫张三，我喜欢编程和看电影","tenant_id":"'"$TENANT_ID"'","user_id":"user-001"}')

session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
echo "Session ID: $session_id"

if [ -n "$session_id" ]; then
    echo -e "${GREEN}✓ 对话成功，Session ID 已获取${NC}"

    # 第二轮：短期记忆测试
    echo ""
    echo -e "${YELLOW}测试: 短期记忆（同一 Session）${NC}"
    curl -s -X POST "$BASE_URL/api/v2/chat" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d '{"session_id":"'"$session_id"'","message":"我叫什么？我喜欢什么？","tenant_id":"'"$TENANT_ID"'"}' \
        | (command -v jq &> /dev/null && jq -C '.data.content' || cat)

    # 第三轮：长期记忆测试（新 Session）
    echo ""
    echo -e "${YELLOW}测试: 长期记忆（新 Session，同一用户）${NC}"
    curl -s -X POST "$BASE_URL/api/v2/chat" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d '{"message":"你还记得我的名字和爱好吗？","tenant_id":"'"$TENANT_ID"'","user_id":"user-001"}' \
        | (command -v jq &> /dev/null && jq -C '.data.content' || cat)
else
    echo -e "${RED}✗ 获取 Session ID 失败${NC}"
fi

# 6. 会话管理测试
echo ""
echo "========================================"
echo "6. 会话管理测试"
echo "========================================"

test_api "会话列表" "POST" "/api/v2/sessions" \
    '{"tenant_id":"'"$TENANT_ID"'","user_id":"user-001"}'

# 总结
echo ""
echo "========================================"
echo "测试完成"
echo "========================================"
echo ""
echo "如果所有测试都显示绿色 ✓，说明部署成功！"
echo "如果有红色 ✗，请查看对应的错误信息。"
echo ""
echo "查看日志命令:"
echo "  docker-compose logs -f chat-service"
echo "  docker-compose logs -f agent-service"
echo "  docker-compose logs -f mcp-service"
echo "  docker-compose logs -f memory-service"
echo ""