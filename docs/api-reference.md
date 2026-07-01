# API 参考

## HTTP API

Base URL: `http://<host>:8082`

所有响应格式：

```json
{
  "success": true,
  "message": "操作描述",
  "data": {},
  "error": "错误信息"
}
```

### 聊天接口

#### POST /chat

发送聊天消息，AI Planner 会分析需求并可能生成任务方案。

**请求:**

```json
{
  "message": "写一首关于春天的七言绝句",
  "role": "user"
}
```

**响应:**

```json
{
  "success": true,
  "data": {
    "assistant_reply": "好的，我来为您创作一首七言绝句...",
    "task_suggested": true,
    "task": {
      "ID": "task-1",
      "Description": "创作七言绝句：春天",
      "Rationale": "根据用户对话需求自动生成",
      "Priority": 5,
      "Resources": {
        "CPU": "500m",
        "Memory": "256Mi",
        "Duration": 60
      },
      "NeedsQualityCheck": true,
      "Approved": false
    }
  }
}
```

#### GET /conversation

获取当前对话历史。

**响应:**

```json
{
  "success": true,
  "data": {
    "ID": "default",
    "Name": "默认对话",
    "Rules": "你是一个任务编排助手...",
    "Messages": [
      {
        "Role": "user",
        "Content": "写一首诗",
        "Timestamp": "2026-06-29T10:00:00Z"
      }
    ],
    "CreatedAt": "2026-06-29T10:00:00Z",
    "UpdatedAt": "2026-06-29T10:00:00Z"
  }
}
```

### 任务管理接口

#### GET /tasks

获取所有任务列表。

**响应:**

```json
{
  "success": true,
  "data": {
    "count": 2,
    "tasks": [
      {
        "name": "chat-task-1",
        "namespace": "default",
        "phase": "Succeeded",
        "message": "任务执行成功，质量评分: 85",
        "output": "春风又绿江南岸...",
        "score": 85,
        "passed": true,
        "retries": 0,
        "created_at": "2026-06-29 10:00:00",
        "completion_at": "2026-06-29 10:01:30"
      }
    ]
  }
}
```

#### GET /tasks/pending

获取待批准的任务。

**响应:**

```json
{
  "success": true,
  "data": {
    "count": 1,
    "tasks": [
      {
        "ID": "task-1",
        "Description": "创作七言绝句：春天",
        "Rationale": "根据用户对话需求自动生成",
        "Priority": 5,
        "Resources": {
          "CPU": "500m",
          "Memory": "256Mi",
          "Duration": 60
        },
        "NeedsQualityCheck": true,
        "Approved": false,
        "CreatedAt": "2026-06-29T10:00:00Z"
      }
    ]
  }
}
```

#### POST /tasks/approve

批准任务并创建 Task CRD。

**请求:**

```json
{
  "task_id": "task-1",
  "approver": "user"
}
```

**响应:**

```json
{
  "success": true,
  "message": "Task approved",
  "data": {
    "ID": "task-1",
    "Description": "创作七言绝句：春天 (已创建 Task CRD: chat-task-1)",
    "Approved": true,
    "ApprovedAt": "2026-06-29T10:00:05Z",
    "ApprovedBy": "user"
  }
}
```

#### POST /tasks/reject

拒绝任务。

**请求:**

```json
{
  "task_id": "task-1",
  "reason": "不需要这个任务"
}
```

**响应:**

```json
{
  "success": true,
  "message": "Task rejected"
}
```

#### POST /tasks/create

直接创建新任务（跳过对话流程）。

**请求:**

```json
{
  "name": "my-task",
  "namespace": "default",
  "image": "docker.io/library/alpine:latest",
  "command": ["/bin/sh", "-c"],
  "args": ["echo 'Executing: 任务描述'"],
  "env": [{"name": "ENV_VAR", "value": "value"}],
  "resources": {
    "limits": {
      "cpu": "500m",
      "memory": "256Mi"
    }
  },
  "retryPolicy": {
    "maxRetries": 3
  }
}
```

**响应:**

```json
{
  "success": true,
  "message": "Task my-task created",
  "data": { ... }
}
```

#### DELETE /tasks/{name}

删除任务。

**参数:**

- `name`: 任务名称（路径参数）
- `namespace`: 命名空间（查询参数，默认 "default"）

**响应:**

```json
{
  "success": true,
  "message": "Task default/my-task deleted"
}
```

### 产出物接口

#### GET /outputs/{namespace}/{task-name}.txt

下载任务产出物文件。

**响应:** 文本文件内容

## WebSocket API

暂不支持。使用 HTTP 轮询获取最新状态。
