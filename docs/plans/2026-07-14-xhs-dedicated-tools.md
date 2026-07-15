# 小红书专用工具：拆分 + 搜索硬攻反爬

日期：2026-07-14
状态：待审批

## 背景与问题

当前小红书逻辑全部塞在通用浏览器工具里，"太乱了"：

- `pkg/browseragent/browser.go` 的 `getStateStealth()`（即 `browser_extract` 的代码路径）里写死了小红书分支：检测 `window.__INITIAL_STATE__`，吐出 `[XHS笔记内容]` / `[XHS搜索结果]` 前缀。
- 通用工具里混入站点专属逻辑，违反高内聚低耦合，后续加别的站会越堆越乱。
- 搜索是"被动读页面已加载内容"：`__INITIAL_STATE__.search.feeds` 实测为空（搜索结果靠异步 API 拉，接口被反爬挡），extract 再挖也是空。没有主动调搜索接口的能力。

## 目标

1. **重构**：把小红书逻辑从通用 `browser_extract` 里彻底拆出，做成独立工具。`browser_extract` 回归纯通用页面状态读取。
2. **读笔记**：`xhs_read_note` —— 给笔记链接 + xsec_token，读出结构化内容。已有 SSR 路径，稳。
3. **搜索硬攻**：`xhs_search` —— 主动调小红书搜索接口拿结果。用户已选"再额外硬攻反爬"，按三层递进实测推进，直到打通或确诊无法打通。

## 架构

```
pkg/xhs/                      ← 新包，所有小红书逻辑
  client.go     XHSClient: Search / ReadNote
  sign.go       页面内签名 + fetch 的 JS 构造
  parse.go      搜索响应 / 笔记 JSON 解析（纯逻辑，可单测）
  types.go      Note / SearchResult / NoteCard
  url.go        笔记 URL / 搜索 URL 构造（纯逻辑，可单测）
  client_test.go / parse_test.go / url_test.go

services/mcp-service/internal/tools/
  xhs_tool.go   ← 新文件：XHSSearchTool / XHSReadNoteTool（实现 tools.Executor）

services/mcp-service/internal/service/mcp_service.go
  registerBuiltInTools()  ← 注册 xhs_search / xhs_read_note

pkg/browseragent/browser.go
  getStateStealth()  ← 删除 [XHS笔记内容]/[XHS搜索结果] 两段，回归通用
```

依赖方向：`pkg/xhs` → `pkg/browseragent`（复用 Obscura 隐身浏览器 + `chromedp.Evaluate`）。`pkg/xhs` 不依赖 mcp-service。

## 工具设计

### xhs_read_note（稳）

- 参数：`note_id`（或完整 `url`）、可选 `xsec_token`
- 流程：构造 `https://www.xiaohongshu.com/explore/{id}?xsec_token={token}&xsec_source=pc_search` → stealthNavigate → wait → Evaluate 抽 `__INITIAL_STATE__.note.noteDetailMap[id].note` → 结构化 Note（title/desc/author/type/liked/comment/tags/images/video）
- 输出：可读文本 + JSON
- 这条路径已实测可读，几乎零风险

### xhs_search（硬攻反爬）

- 参数：`keyword`、可选 `page`(默认1)、`sort`(general/hot_descending/time_descending)
- 目标接口：`https://edith.xiaohongshu.com/api/sns/web/v1/search/notes?keyword=...&page=...&page_size=20&search_id=...&sort=...&note_type=0`
- **核心策略：不自己逆向 x-s 算法，借 Obscura 里跑着的小红书自己的 JS 签名**（成功率最高、最不易过期）

三层递进，每层实测后据结果决定走多远：

**第 1 层（主）：页面内显式签名 + fetch**（只依赖 `Runtime.evaluate`，已在 Obscura 验证可用）
1. stealthNavigate 到 `https://www.xiaohongshu.com/`（加载小红书 JS + 建立 cookie 登录态，preset cookie 已注入）
2. Evaluate 探测签名函数：`Object.keys(window).filter(k=>/_webms|sign|xyw|mns/.test(k))`
3. 调签名函数对搜索接口 path+params 签名，拿到 `{x-s, x-t, x-s-common}`
4. 页面内 `fetch(edith搜索接口, {headers:{x-s,x-t,x-s-common}, credentials:'include'}).then(r=>r.text())`
5. 解析 JSON → NoteCard 列表（id/title/xsec_token/type/author/liked）
6. 返回列表（带 xsec_token，供下一步 `xhs_read_note` 读详情）

**第 2 层（备）：CDP 网络抓包**（若第 1 层签名函数找不到 / fetch 被拦）
- navigate 到 `search_result?keyword=...`，`chromedp.ListenTarget` 捕获 `edith.../search/notes` 的 `EventResponseReceived` → `network.GetResponseBody`
- 复用小红书页面自己的请求机制（签名/search_id 全由它自己处理），我们只抓响应
- 风险：Obscura 的 Network domain 是否可用未验证（DOM domain 挂过）。挂了就回退第 1 层或第 3 层

**第 3 层：代理换 IP**（若 1、2 签名都对但仍被挡 = IP/账号风控）
- config 加 `xhs.proxy_url`，给 Obscura 或 fetch 走代理
- 若到这层仍不通，确诊为风控，如实告知用户：当前 IP/账号被风控，搜索无法稳定可用

**每层都先诊断再决定下一步**：打印接口 HTTP 状态码 + 小红书 error_code（如 300011 验证码 / 300015 风控 / 461 token），据此判断是签名问题、验证码、还是 IP 风控。不盲目堆层数。

## 清理

`pkg/browseragent/browser.go` `getStateStealth()`：
- 删除 `var xhs=''` 整段（笔记抽取，~20 行）
- 删除 `var search=''` 整段（搜索抽取，~15 行）
- 删除 `prefix` 拼接逻辑
- `browser_extract` 回归：url/title/innerText/elements，纯通用

`pkg/browseragent/browser.go.bak`：删除（陈旧备份，已在 git 历史）。

## 测试

纯逻辑部分单测（AAA 模式，不依赖浏览器）：
- `url_test.go`：笔记 URL 带 token 构造、搜索 URL 构造、从分享链接提取 note_id+xsec_token
- `parse_test.go`：用录制好的真实响应 fixture 解析搜索 JSON / 笔记 JSON
- `sign_test.go`：签名 JS 字符串构造（含 keyword/page/search_id 插值正确）

集成验证（手动，复用既有 curl 方式）：
- 部署后 `POST /api/v2/mcp/call` 直调 `xhs_read_note`（已知笔记，应成）
- 直调 `xhs_search`（关键词，真实检验）
- E2E：agent 自然语言"在小红书搜 羽毛球 并读第一条"→ agent 自行调 xhs_search → xhs_read_note

## 部署

- 优先 `docker compose build mcp-service`（若 daocloud 镜像源恢复）
- 否则复用已验证的 workaround：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` → `docker cp` 二进制 → `docker restart`
- 健康检查 7 服务全 healthy
- 注意：当前线上 mcp-service 镜像仍是旧的（cd04855），改动靠 docker cp 维持；本次顺手重建镜像做持久化（若镜像源允许）

## 风险与诚实声明

- **搜索不保证成**：三层都走完仍可能被风控挡。但每层会诊断确切原因（状态码/error_code），给用户明确结论而非"试不了"。
- **CDP Network domain on Obscura**：未验证，可能像 DOM domain 一样挂。第 1 层只用 Runtime.evaluate（已验证可用），作为主路径规避此风险。
- **签名函数命名**：`window._webmsxyw` 等可能改版，需实测探测；探测逻辑内建兜底。
- **读笔记稳定可用**，是本次确定的交付物。
- 不动 cookie 安全策略（用户已表态"别介意"）；web_session 等 cookie 仍走既有 gateway cookie 存储 + preset 注入。

## 实施顺序

1. 建 `pkg/xhs/` 骨架 + url/parse 纯逻辑 + 单测（先绿）
2. `xhs_read_note` 端到端（稳路径，先拿到确定交付物）
3. 清理 `getStateStealth` 小红书分支 + 删 .bak
4. `xhs_search` 第 1 层（页面内签名 fetch）→ 部署 → 实测
5. 据结果：第 2 层 / 第 3 层 / 收尾
6. 注册工具 + mcp-service 部署 + E2E 验证
7. 更新 memory（xhs-obscura-stealth-browser.md 增补专用工具 + 搜索结论）
