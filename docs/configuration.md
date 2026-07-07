# 配置指南

## AI 配置

主配置文件：`config/ai_config.yaml`（提交到 git，仅含占位符，**勿写入真实密钥**）。

本地覆盖：`config/ai_config.local.yaml`（`.gitignore`，运行时自动 merge 覆盖同名段）。

```bash
cp config/ai_config.example.yaml config/ai_config.local.yaml
# 编辑 base_url、api_key、model
```

### 支持的提供商

所有角色通过 **OpenAI 兼容** Chat Completions API 调用（实现见 `internal/ai/remote_client.go`：`POST {base_url}/v1/chat/completions`）。

| 提供商 | `base_url` | 常用模型 | 说明 |
|--------|------------|----------|------|
| [DeepSeek](https://api.deepseek.com) | `https://api.deepseek.com` | `deepseek-chat` | **默认**，Planner / Worker / Monitor 通用 |
| [xAI Grok](https://console.x.ai) | `https://api.x.ai` | `grok-4.3`、`grok-3` | 适合 Worker 长文创作 |
| 自托管 vLLM / Ollama 网关 | `http://<host>:<port>` | 自定义 | 任意 OpenAI 兼容端点 |
| [OpenAI](https://platform.openai.com) | `https://api.openai.com` | `gpt-4o` 等 | 标准兼容 |

> **Grok CLI ≠ xAI API**：Cursor / Grok CLI 是对话式编程助手，**没有**可供 agent-flow Worker 调用的 HTTP API。要在流水线里使用 Grok 模型，请申请 xAI API Key 并配置 `base_url: https://api.x.ai`。

### 配置结构

```yaml
global:
  default_model: deepseek-chat
  system_prompt: "系统提示词..."

planner:        # Planner AI — 对话、任务/Workflow 编排
  mode: remote  # remote 或 local
  remote:
    enabled: true
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: deepseek-chat
    temperature: 0.7
    max_tokens: 8192
    timeout_seconds: 300
  local:
    enabled: false
    base_url: http://localhost:11434
    model: qwen2.5:7b

worker:         # Worker AI — 大纲、章节正文
  mode: remote
  remote:
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: deepseek-chat
  local:
    enabled: true   # 本地开发可切 Ollama
    base_url: http://localhost:11434
    model: qwen2.5:7b

monitor:        # Monitor AI — 质量评分
  mode: remote
  remote:
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: deepseek-chat
    temperature: 0.2

logging:
  level: info
  ai_requests: true
  ai_responses: true

retry:
  max_retries: 3
  base_delay_seconds: 5
  max_delay_seconds: 60

quality:
  threshold: 70
  max_retries: 3
```

### 角色模式

#### Remote 模式（推荐）

通过 HTTP 调用远程 LLM。请求/响应格式可在 yaml 中自定义 `request.body_template` 与 `response.extract_field`；默认按 OpenAI Chat Completions 解析。

```yaml
planner:
  mode: remote
  remote:
    enabled: true
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: deepseek-chat
    request:
      headers:
        Content-Type: application/json
        Authorization: Bearer ${AI_API_KEY}
    response:
      extract_field: choices[0].message.content
      error_field: error.message
```

#### Local 模式（Ollama）

```yaml
worker:
  mode: local
  local:
    enabled: true
    base_url: http://localhost:11434
    model: qwen2.5:7b
    temperature: 0.7
    top_p: 0.9
    max_tokens: 4096
    endpoints:
      generate: /api/generate
      chat: /api/chat
```

### 分角色使用不同模型

`ai_config.local.yaml` 只需覆盖要改的段，其余继承 `ai_config.yaml`。

**示例：Planner / Monitor 用 DeepSeek，Worker 用 xAI Grok**

```yaml
# config/ai_config.local.yaml
planner:
  remote:
    base_url: https://api.deepseek.com
    api_key: sk-...
    model: deepseek-chat

worker:
  mode: remote
  remote:
    base_url: https://api.x.ai
    api_key: xai-...
    model: grok-4.3
  local:
    enabled: false

monitor:
  remote:
    base_url: https://api.deepseek.com
    api_key: sk-...
    model: deepseek-chat
```

### 环境变量

yaml 支持 `${VAR}` 占位符，启动时从进程环境替换：

| 变量 | 说明 |
|------|------|
| `AI_API_KEY` | 默认 API Key（Planner / Worker / Monitor 共用） |
| `AI_BASE_URL` | 默认 API 根地址（不含 `/v1` 路径） |
| `WORKER_AI_API_KEY` | 可选，K8s Secret 注入 Worker 专用 Key |
| `WORKER_AI_BASE_URL` | 可选，K8s Secret 注入 Worker 专用地址 |

```bash
export AI_API_KEY=sk-...
export AI_BASE_URL=https://api.deepseek.com

# 从 local 配置一键导出（deploy 脚本会 source）
source ./scripts/read-ai-secrets.sh
```

**Kubernetes 注意**：`deploy.sh` 会将 `WORKER_AI_*` 写入 Secret，但 `ai_config.yaml` 中 Worker 段默认引用 `${AI_BASE_URL}` / `${AI_API_KEY}`。要在集群内分角色部署，请任选其一：

1. 在容器可访问路径提供 `ai_config.local.yaml` 覆盖 Worker 段（推荐）
2. 将 Worker 段的占位符改为 `${WORKER_AI_BASE_URL}` / `${WORKER_AI_API_KEY}` 并重新部署 ConfigMap

访问 `api.x.ai` 若需代理，请确认 Sandbox 网络与 `NO_PROXY` 白名单（DeepSeek 域名通常在白名单内，xAI 可能需走代理 Sidecar）。

### Token 统计

启用 Remote / Local AI 后，系统自动从 API 响应解析 `usage` 字段并累计：

- **Task CRD**：`status.tokenUsage`
- **SQLite**：`novels` / `chapters` 表的 `prompt_tokens`、`completion_tokens`、`total_tokens`
- **Web**：小说库、阅读器、[/tokens 报表页](api-reference.md#get-novelstokensreport)

历史任务在开启统计前已完成，不会回填用量。

### 质量检查配置

```yaml
quality:
  threshold: 70        # 质量评分阈值（0-100）
  max_retries: 3       # 最大重试次数
```

### 重试配置

Task 层与 Worker-Monitor 循环使用 `ai_config.yaml` 中的退避参数（指数退避 + 抖动）：

```yaml
retry:
  max_retries: 3
  base_delay_seconds: 5
  max_delay_seconds: 60
```

Workflow 层在 CRD `spec.execution` 中单独配置：

```yaml
execution:
  stepMaxRetries: 3
  stepRetryBaseDelaySec: 30
  stepRetryMaxDelaySec: 300
```

章节 Task 重试上限通过 Workflow `params.taskMaxRetries` 设置（默认 5，样本为 6）。

实现细节见 [workflow.md](workflow.md)。

### RAG Embedding（可选）

```bash
export RAG_EMBEDDING_BASE_URL="${AI_BASE_URL}"
export RAG_EMBEDDING_API_KEY="${AI_API_KEY}"
export RAG_EMBEDDING_MODEL="text-embedding-3-small"
```

### 结构化日志

控制器与二进制使用 `log/slog`，通过环境变量配置：

| 变量 | 说明 | 默认 |
|------|------|------|
| `AGENTFLOW_LOG_FORMAT` | `json` 或 `text` | `text` |
| `AGENTFLOW_LOG_LEVEL` | `debug` / `info` / `warn` / `error` | `info` |

代码入口：`internal/log/`。controller-runtime 通过 slog 桥接统一输出。

### 小说元数据存储

| 变量 | 说明 | 默认 |
|------|------|------|
| `AGENTFLOW_NOVEL_DB` | SQLite 路径 | `/data/outputs/agentflow/novels.db` |

章节正文仍在 PVC：`/data/outputs/workflows/<ns>/<name>/chapters/`。

### Planner 集群模式

| 变量 | 说明 |
|------|------|
| `AGENTFLOW_SKIP_SANDBOX` | `true` 时 Planner 直连 AI，不创建 Sandbox |
| `AGENTFLOW_OUTPUT_DIR` | 产出根目录，默认 `/data/outputs` |
| `AGENTFLOW_MAX_CONCURRENT_TASKS` | Task 控制器并发 reconcile 数 |
| `REDIS_URL` | 可选，对话/checkpoint 热状态 |

## Kubernetes 配置

### 命名空间

默认命名空间: `agent-flow-system`

```bash
kubectl create namespace agent-flow-system
```

### RBAC

控制器需要以下权限：

- `tasks`: get, list, watch, create, update, patch, delete
- `tasks/status`: get, update, patch
- `tasks/finalizers`: update
- `sandboxes`: create, update, patch, delete
- `sandboxes/status`: get, update, patch

### PVC

产出物存储需要 PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: task-outputs
  namespace: agent-flow-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

### 资源限制

控制器默认资源限制（可在 `config/manager/deployment.yaml` 中调整）：

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## 命令行参数

```bash
./planner [flags]

--metrics-bind-address=:8080    # Prometheus 指标端口
--health-probe-bind-address=:8081  # 健康检查端口
--api-port=8082                 # Chat API 端口
--leader-elect                  # 启用 Leader Election
--ai-config=config/ai_config.yaml  # AI 配置文件路径
```