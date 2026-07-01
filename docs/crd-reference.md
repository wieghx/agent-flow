# CRD 参考

## Task

Task 是 Agent Flow 的核心自定义资源，代表一个 AI 执行任务。

### API 版本

```
agentflow.io/v1alpha1
```

### 定义

```yaml
apiVersion: agentflow.io/v1alpha1
kind: Task
metadata:
  name: my-task
  namespace: default
spec:
  command:
    - /bin/sh
    - -c
  args:
    - "echo 'Executing: 任务描述'"
  image: docker.io/library/alpine:latest
  env:
    - name: TASK_NAME
      value: my-task
  resources:
    limits:
      cpu: "500m"
      memory: "256Mi"
  retryPolicy:
    maxRetries: 3
    retryDelaySeconds: 10
    retryOn:
      - OnFailure
  timeoutSeconds: 60
  runtimeClassName: gvisor
  podSecurityContext:
    runAsNonRoot: true
status:
  phase: Succeeded
  message: "任务执行成功，质量评分: 85"
  startTime: "2026-06-29T10:00:00Z"
  completionTime: "2026-06-29T10:01:30Z"
  retries: 0
  output:
    content: "产出物内容..."
    format: text
    generatedAt: "2026-06-29T10:01:00Z"
  qualityCheck:
    score: 85
    passed: true
    feedback: "诗歌格律正确，意境优美"
    evaluatedAt: "2026-06-29T10:01:20Z"
  workerName: my-task-sandbox
```

### Spec 字段

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `command` | `[]string` | 是 | 容器执行命令 |
| `args` | `[]string` | 否 | 命令参数 |
| `image` | `string` | 否 | 容器镜像（默认 alpine） |
| `env` | `[]EnvVar` | 否 | 环境变量 |
| `resources` | `ResourceRequirements` | 否 | 资源限制 |
| `retryPolicy` | `RetryPolicy` | 否 | 重试策略 |
| `timeoutSeconds` | `*int32` | 否 | 超时时间（秒） |
| `runtimeClassName` | `*string` | 否 | 运行时类（如 gvisor） |
| `podSecurityContext` | `*PodSecurityContext` | 否 | Pod 安全上下文 |

### RetryPolicy

| 字段 | 类型 | 说明 |
|------|------|------|
| `maxRetries` | `int32` | 最大重试次数 |
| `retryDelaySeconds` | `int32` | 重试间隔（秒） |
| `retryOn` | `[]TaskCondition` | 触发重试的条件 |

### TaskCondition

| 值 | 说明 |
|-----|------|
| `OnFailure` | 失败时重试 |
| `OnTimeout` | 超时时重试 |
| `OnAnyError` | 任何错误时重试 |

### Status 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `phase` | `TaskPhase` | 当前状态 |
| `message` | `string` | 状态消息 |
| `startTime` | `*Time` | 开始时间 |
| `completionTime` | `*Time` | 完成时间 |
| `retries` | `int32` | 已重试次数 |
| `output` | `*TaskOutput` | 产出物 |
| `qualityCheck` | `*QualityCheck` | 质量检查结果 |
| `workerName` | `string` | 执行的 Sandbox 名称 |

### TaskPhase

| 值 | 说明 |
|-----|------|
| `Pending` | 等待创建 Sandbox |
| `Running` | Sandbox 正在执行 |
| `Succeeded` | 任务成功完成 |
| `Failed` | 执行失败 |
| `Cancelled` | 已取消 |

### TaskOutput

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | `string` | 文本产出内容 |
| `format` | `string` | 产出格式（text, code, json） |
| `generatedAt` | `*Time` | 生成时间 |

### QualityCheck

| 字段 | 类型 | 说明 |
|------|------|------|
| `score` | `int32` | 质量评分（0-100） |
| `passed` | `bool` | 是否通过 |
| `feedback` | `string` | 评估反馈 |
| `evaluatedAt` | `*Time` | 评估时间 |

---

## Monitor

Monitor 用于监控任务执行状态，触发告警和自动化操作。

### API 版本

```
agentflow.io/v1alpha1
```

### 定义

```yaml
apiVersion: agentflow.io/v1alpha1
kind: Monitor
metadata:
  name: task-monitor
  namespace: default
spec:
  namespace: default
  labelSelector:
    agentflow.io/needs-quality-check: "true"
  checkIntervalSeconds: 30
  failedTaskThreshold: 3
  staleTaskThreshold: 300
  actions:
    - condition: OnFailure
      actionType: Retry
      severity: warning
    - condition: OnTimeout
      actionType: Alert
      severity: critical
status:
  phase: Running
  lastCheckTime: "2026-06-29T10:00:00Z"
  monitoredCount: 5
  phaseDistribution:
    pending: 1
    running: 2
    succeeded: 1
    failed: 1
  alerts: []
```

### Spec 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `namespace` | `string` | 监控的命名空间（空=全部） |
| `labelSelector` | `map[string]string` | 标签选择器 |
| `checkIntervalSeconds` | `int32` | 检查间隔（秒） |
| `failedTaskThreshold` | `int32` | 失败告警阈值 |
| `staleTaskThreshold` | `int32` | 僵尸任务阈值（秒） |
| `actions` | `[]MonitorAction` | 告警动作 |

### MonitorAction

| 字段 | 类型 | 说明 |
|------|------|------|
| `condition` | `MonitorCondition` | 触发条件 |
| `actionType` | `ActionType` | 动作类型 |
| `severity` | `string` | 告警级别 |

### MonitorCondition

| 值 | 说明 |
|-----|------|
| `OnFailure` | 任务失败时 |
| `OnTimeout` | 任务超时时 |
| `OnStale` | 僵尸任务时 |
| `OnRetry` | 重试时 |

### ActionType

| 值 | 说明 |
|-----|------|
| `Alert` | 发送告警 |
| `Retry` | 自动重试 |
| `Cancel` | 取消任务 |
| `Notify` | 通知 |
| `Escalate` | 升级处理 |

---

## Sandbox (agents.x-k8s.io)

Sandbox 是 agent-sandbox 官方 CRD，用于创建隔离的 Pod 执行环境。

### API 版本

```
agents.x-k8s.io/v1alpha1
```

### 由 Agent Flow 自动创建

Agent Flow 控制器会自动创建 Sandbox 资源，用户通常不需要直接操作。

```yaml
apiVersion: agents.x-k8s.io/v1alpha1
kind: Sandbox
metadata:
  name: my-task-sandbox
  namespace: default
  labels:
    agents.x-k8s.io/sandbox: my-task-sandbox
    agentflow.io/task: my-task
spec:
  podTemplate:
    spec:
      restartPolicy: Never
      containers:
        - name: my-task
          image: docker.io/library/alpine:latest
          command: ["/bin/sh", "-c"]
          args: ["echo 'Executing: 任务描述'"]
  lifecycle:
    shutdownPolicy: null
```

### 状态

Sandbox 的 `status.conditions` 中包含 `Finished` 条件：

- `Reason: PodSucceeded` — 执行成功
- `Reason: PodFailed` — 执行失败
