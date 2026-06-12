#!/bin/bash

# ========================================
# Agent Platform 测试脚本
# ========================================

BASE_URL="http://localhost:9000"

echo "========================================"
echo "Agent Platform API 测试"
echo "========================================"

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 测试函数
test_api() {
    local name=$1
    local method=$2
    local endpoint=$3
    local data=$4

    echo -e "\n测试: $name"
    echo "请求: $method $endpoint"

    if [ -z "$data" ]; then
        response=$(curl -s -X $method "$BASE_URL$endpoint" 2>/dev/null)
    else
        response=$(curl -s -X $method "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null)
    fi

    if [ $? -eq 0 ] && [ -n "$response" ]; then
        echo -e "${GREEN}✓ 成功${NC}"
        echo "响应: $response" | head -c 200
        echo ""
    else
        echo -e "${RED}✗ 失败${NC}"
    fi
}

# 1. 测试 MCP 工具列表
test_api "MCP 工具列表" "GET" "/api/v2/mcp/tools"

# 2. 测试对话
test_api "对话接口" "POST" "/api/v2/chat" '{
    "message": "你好，请介绍一下你自己",
    "tenant_id": "test-tenant",
    "user_id": "test-user"
}'

# 3. 测试会话列表
test_api "会话列表" "POST" "/api/v2/sessions" '{
    "tenant_id": "test-tenant",
    "user_id": "test-user"
}'

# 4. 测试知识库搜索
test_api "知识库搜索" "POST" "/api/v2/knowledge/search" '{
    "query": "测试搜索",
    "top_k": 5,
    "tenant_id": "test-tenant"
}'

# 5. 测试 Agent 列表
test_api "Agent 列表" "GET" "/api/v2/agents"

# 6. 测试 MCP 工具调用 - 计算器
test_api "计算器工具" "POST" "/api/v2/mcp/call" '{
    "name": "calculator",
    "arguments": "{\"expression\": \"2 + 3 * 4\"}"
}'

# 7. 测试 MCP 工具调用 - 时间
test_api "时间工具" "POST" "/api/v2/mcp/call" '{
    "name": "time",
    "arguments": "{}"
}'

# 8. 测试 MCP 工具调用 - 数据分析
test_api "数据分析工具" "POST" "/api/v2/mcp/call" '{
    "name": "data_analysis",
    "arguments": "{\"data\": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]}"
}'

echo ""
echo "========================================"
echo "测试完成"
echo "========================================"
