#!/bin/bash

# CSDN Cookie
COOKIES='[
  {"name": "UserName", "value": "m0_54140879", "domain": ".csdn.net"},
  {"name": "UserInfo", "value": "818a4e24e3e94ee686ebc30893c5cd37", "domain": ".csdn.net"},
  {"name": "UserToken", "value": "818a4e24e3e94ee686ebc30893c5cd37", "domain": ".csdn.net"},
  {"name": "UN", "value": "m0_54140879", "domain": ".csdn.net"},
  {"name": "AU", "value": "81C", "domain": ".csdn.net"}
]'

# 转义 JSON
COOKIES_ESCAPED=$(echo "$COOKIES" | sed 's/"/\\"/g')

curl -s -X POST http://localhost:9000/api/v2/mcp/call \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"browser_execute\",
    \"arguments\": {
      \"task\": \"打开 https://blog.csdn.net/m0_54140879 查看我的博客文章列表，告诉我文章标题\",
      \"max_steps\": 10,
      \"cookies\": $COOKIES
    }
  }" | python3 -m json.tool 2>/dev/null || cat
