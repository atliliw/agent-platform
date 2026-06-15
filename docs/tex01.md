# 多 Agent 架构使用指南

## 概述

Agent Platform 现已支持多 Agent 协作架构，基于 OpenAI Swarm 的 Handoff 模式实现。

## 架构

```
用户请求 → Main Agent → (Handoff) → Researcher/Coder/Analyst Agent
                 ↑_________________________|
                      (完成或继续交接)
```
