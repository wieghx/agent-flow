# 架构设计

## 系统概览

Agent Flow 是一个基于 Kubernetes 的 AI Agent 编排系统，采用 Planner-Worker-Monitor 三层架构，通过 eino 框架实现任务流程编排。

## 架构图

```
用户 (Web UI / API)
        │
        ▼
┌─────────────────────────────────────────────┐
│              ChatRouter (对话路由)            │
│  接收用户消息 → 调用 Planner AI → 生成任务方案  │
└──────────────────────┬──────────────────────┘
                       │ 批准任务
                       ▼
┌─────────────────────────────────────────────┐
│          TaskPlannerEino (K8s Controller)    │
│  Watch Task CRD → 创建 Sandbox → 管理生命周期  │
└──────────────────────┬──────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │ Sandbox  │ │ Worker   │ │ Monitor  │
    │ (Pod)    │→│ (AI执行) │→│ (AI评估) │
    └──────────┘ └──────────┘ └──────────┘
                       │            │
                       │  不通过(重试) │
                       └────────────┘
                              │
                              ▼ 通过
                        Task Succeeded
```

## 核心组件

### 1. Planner（大脑）

- **职责**: 理解用户需求，规划任务，创建 Sandbox
- **实现**: `internal/architecture/planner_eino.go` 中的 `TaskPlannerEino`
- **AI 角色**: 通过 `ChatRouter` 调用 Planner AI，分析对话并生成任务方案
- **K8s 控制器**: Watch Task CRD，协调整个任务生命周期

### 2. Worker（执行者）

- **职责**: 执行具体任务，生成产出物
- **实现**: `internal/flow/agent_flow.go` 中的 `WorkerNode`
- **执行方式**: 调用 Worker AI 生成文本产出，或从 Sandbox PVC 读取已有产出
- **重试机制**: Monitor 评估不通过时，根据反馈自动重试

### 3. Monitor（监工）

- **职责**: 评估产出质量，决定是否通过
- **实现**: `internal/flow/agent_flow.go` 中的 `MonitorNode`
- **评估方式**: 调用 Monitor AI 进行质量评分（满分 100 分）
- **阈值**: 默认 70 分通过，低于阈值自动重试

### 4. ChatRouter（对话路由）

- **职责**: 管理用户对话，协调任务创建流程
- **实现**: `internal/flow/chat_interface.go`
- **功能**: 对话历史管理、任务方案生成、待批准任务队列

### 5. HTTP API

- **职责**: 提供 RESTful API 接口
- **实现**: `internal/api/http_api.go`
- **端口**: 8082

### 6. Web UI

- **职责**: 对话、任务/Workflow 管理、SSE 进度、章节浏览
- **实现**: `web/src/`（Vite + React + TypeScript）+ `web/server.js`（静态资源与 API 代理）
- **访问**: `./run-web.sh` → `http://localhost:3000`

### 7. Workflow Controller

- **职责**: 解析 Workflow 模板、并行派发章节 Task、回补缺失章节、步骤级重试
- **实现**: `internal/architecture/workflow_controller.go` + `internal/workflow/`
- **模板**: `novel-outline-chapters`（大纲 → 分卷 → 章节 → 故事弧）

### 8. 混合存储

- **PVC**: 章节正文、大纲 JSON、`chapters/*.md`
- **SQLite**: `internal/store/`，章节状态、小说元数据（`novels.db`）

### 9. 重试与日志

- **重试**: `internal/retry/` — 失败分类、指数退避、分段/Task/Workflow 三层策略
- **日志**: `internal/log/` — `log/slog` 结构化输出，桥接 controller-runtime

## 数据流

### 任务创建流程

```
1. 用户发送消息 → ChatRouter 接收
2. ChatRouter 调用 Planner AI 生成回复和任务方案
3. AI 在回复中标记 [CREATE_TASK:描述] 触发任务创建
4. 任务进入待批准队列
5. 用户批准 → 创建 Task CRD
6. TaskPlannerEino 监听到 Task → 创建 Sandbox
7. Sandbox Pod 执行任务脚本
8. Sandbox 完成 → 触发 Worker-Monitor 循环
9. Worker AI 生成产出物
10. Monitor AI 评估质量
11. 通过 → Task Succeeded
12. 不通过 → 带反馈重试（指数退避，章节最多 6 次）
13. Workflow 层可在 Task 失败后按退避重派（最多 3 次）
```

### eino 流程编排

```go
// Worker -> Monitor 链式流程
flow := flow.NewAgentFlow()
flow.AddNode(&flow.WorkerNode{Name: "worker"})
flow.AddNode(&flow.MonitorNode{Name: "monitor"})
flow.Compile()
flow.Execute(ctx, state)
```

eino 框架提供 `Chain` 和 `Lambda` 抽象，将节点串联为可执行的 `Runnable`。

## 状态机

### Task 状态

```
Pending → Running → Succeeded
              │
              └→ Failed (终态)
```

| 状态 | 说明 |
|------|------|
| Pending | 等待创建 Sandbox |
| Running | Sandbox 正在执行 |
| Succeeded | 任务成功完成 |
| Failed | 执行失败或质量检查未通过 |

### 质量检查循环

```
Worker 执行 → Monitor 评估
                │
                ├→ 评分 ≥ 70 → Succeeded
                │
                └→ 评分 < 阈值 → 指数退避后重试
                                  │
                                  ├→ Worker 执行（带 Monitor 反馈）
                                  └→ 超过次数 → Failed → Workflow 可重派
```

## 技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| 后端语言 | Go 1.26 | 主控制器 |
| 流程编排 | eino | AI 流程编排框架 |
| 容器编排 | Kubernetes | 容器调度 |
| 沙箱执行 | agent-sandbox | 官方沙箱 Pod |
| AI 模型 | Qwen3.5-35B | 对话/执行/评估 |
| Web UI | React + Node.js | 前端 + API 代理 |
| 重试 | internal/retry | 指数退避 + 失败分类 |
| 元数据 | SQLite | 章节状态索引 |
| 配置管理 | Kustomize | K8s 资源配置 |
| CRD 管理 | controller-runtime | Kubernetes Operator |

## MCP Sidecar 模式（工具执行）

### 概述

MCP (Model Context Protocol) Sidecar 模式允许在 Sandbox Pod 内执行工具调用，实现安全隔离的工具执行环境。

### 架构

```
Sandbox Pod
├── mcp-sidecar (容器 1)
│   ├── /tools/list   — 列出可用工具
│   ├── /tools/call   — 调用工具
│   ├── /ai/chat      — AI 代理（调用外部 LLM）
│   └── 内置工具: shell_exec, file_read, file_write, http_request, list_dir
│
└── worker-agent (容器 2)
    ├── ReAct Agent 循环
    ├── 调用 mcp-sidecar 执行工具
    └── 输出结果到 PVC
```

### 启用方式

在 Task CRD 中添加 annotation：

```yaml
apiVersion: agentflow.io/v1alpha1
kind: Task
metadata:
  name: my-task
  annotations:
    agentflow.io/mcp-mode: "true"
```

### 执行流程

```
1. Controller 检测到 mcp-mode annotation
2. 创建 Sandbox Pod（包含 mcp-sidecar + worker-agent）
3. mcp-sidecar 启动，暴露工具 API（:9090）
4. worker-agent 启动 ReAct Agent 循环：
   a. 调用 /ai/chat 获取下一步行动
   b. 解析 Action + ActionInput
   c. 调用 /tools/call 执行工具
   d. 将 Observation 反馈给 AI
   e. 重复直到 FinalAnswer
5. 输出写入 PVC
6. Controller 读取产出，运行 Monitor 质量检查
```

### 内置工具

| 工具 | 说明 | 输入 |
|------|------|------|
| `shell_exec` | 执行 shell 命令 | `{"command": "ls", "workdir": "/tmp"}` |
| `file_read` | 读取文件 | `{"path": "/data/file.txt"}` |
| `file_write` | 写入文件 | `{"path": "/data/out.txt", "content": "..."}` |
| `http_request` | HTTP 请求 | `{"method": "GET", "url": "..."}` |
| `list_dir` | 列出目录 | `{"path": "/data"}` |

### Agent Loop (ReAct)

Worker Agent 使用 ReAct (Reasoning + Acting) 模式：

```
Thought: 分析任务，决定使用哪个工具
Action: shell_exec
ActionInput: {"command": "echo 'hello world'"}
Observation: hello world
Thought: 已获得结果，生成最终答案
FinalAnswer: 任务完成，输出为 "hello world"
```

### 安全特性

- 工具在 Sandbox Pod 内执行，与控制器隔离
- PVC 挂载限制在 `/data/outputs`
- 资源限制（CPU/Memory）
- 无特权容器
- gVisor/Kata 运行时支持

### 配置

控制器通过环境变量获取 MCP 镜像和 AI 配置：

```yaml
env:
  - name: MCP_IMAGE
    value: "minagflow/mcp-sidecar:latest"
  - name: WORKER_AGENT_IMAGE
    value: "minagflow/worker-agent:latest"
  - name: AI_BASE_URL
    value: "http://your-llm-server:9101"
  - name: AI_API_KEY
    valueFrom:
      secretKeyRef:
        name: agentflow-ai-secrets
        key: AI_API_KEY
```

### 扩展工具

在 `internal/mcp/tools.go` 中实现 `Tool` 接口：

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input map[string]interface{}) (string, error)
}
```

注册到 MCP Server：

```go
server := mcp.NewMCPServer(aiConfig)
server.RegisterTool(&MyCustomTool{})
```
