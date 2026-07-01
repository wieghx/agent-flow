# 配置指南

## AI 配置

配置文件: `config/ai_config.yaml`

### 配置结构

```yaml
global:
  default_model: Qwen3.5-35B-A3B
  system_prompt: "系统提示词..."

planner:        # Planner AI 配置
  description: 任务规划大脑
  mode: remote  # remote 或 local
  remote:
    enabled: true
    base_url: http://<host>:<port>
    api_key: ${AI_API_KEY}
    model: Qwen3.5-35B-A3B
    temperature: 0.7
    max_tokens: 65536
    timeout_seconds: 300
  local:
    enabled: false
    base_url: http://localhost:11434
    model: qwen2.5:7b

worker:         # Worker AI 配置
  mode: remote
  ...

monitor:        # Monitor AI 配置
  mode: remote
  ...

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

### 角色配置

每个 AI 角色（Planner/Worker/Monitor）支持两种模式：

#### Remote 模式

通过 HTTP API 调用远程 LLM 服务（如 vLLM、OpenAI 兼容 API）。

```yaml
planner:
  mode: remote
  remote:
    enabled: true
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: Qwen3.5-35B-A3B
    temperature: 0.7
    max_tokens: 65536
    timeout_seconds: 300
    request:
      headers:
        Content-Type: application/json
        Authorization: Bearer ${AI_API_KEY}
    response:
      extract_field: choices[0].message.content
```

#### Local 模式

通过 Ollama 调用本地 LLM 服务。

```yaml
worker:
  mode: local
  local:
    enabled: true
    base_url: http://localhost:11434
    model: qwen2.5:7b
    temperature: 0.7
    top_p: 0.9
    max_tokens: 2048
    endpoints:
      generate: /api/generate
      chat: /api/chat
```

### 环境变量

配置文件中支持环境变量替换：

```yaml
api_key: ${AI_API_KEY}
base_url: ${AI_BASE_URL}
```

在环境或 `.env` 文件中设置：

```bash
export AI_API_KEY=your-api-key
export AI_BASE_URL=http://your-llm-server:9101
```

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
