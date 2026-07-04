# API 参考

## 访问方式

| 方式 | Base URL | 说明 |
|------|----------|------|
| 直连 Manager | `http://<host>:8082` | Planner 内置 HTTP API |
| Web 代理 | `http://localhost:3000` | `web/server.js` 反向代理 `/chat`、`/tasks`、`/workflows`、`/novels` 等 |

本地 kind 集群推荐 `./run-web.sh` 访问 Web 代理；调试 API 可单独 port-forward manager。

## 响应格式

```json
{
  "success": true,
  "message": "操作描述",
  "data": {},
  "error": "错误信息"
}
```

---

## 聊天接口

### POST /chat

发送聊天消息，Planner AI 分析需求并可能生成任务/Workflow 方案。

**请求:**

```json
{
  "message": "写一部10章的历史小说，万历年间朝堂斗争",
  "role": "user"
}
```

**响应:** 含 `assistant_reply`；若 AI 标记 `[CREATE_WORKFLOW:...]` 或 `[CREATE_TASK:...]` 则附带待批准方案。

### GET /conversation

获取当前对话历史。

---

## 任务管理

### GET /tasks

列出所有 Task CRD 及状态摘要。

### GET /tasks/pending

获取待批准任务（来自对话流程）。

### POST /tasks/approve

批准任务并创建 Task CRD。

```json
{"task_id": "task-1", "approver": "user"}
```

### POST /tasks/reject

```json
{"task_id": "task-1", "reason": "不需要"}
```

### POST /tasks/create

直接创建 Task CRD（跳过对话）。

### DELETE /tasks/{name}?namespace=default

删除任务。

### GET /tasks/events?namespace=default&name={task-name}

**SSE 流式进度**。事件类型包括 `connected`、`ping`、以及各执行步骤名。

```
event: connected
data: {"task":"chapter-01","namespace":"default"}

event: worker
data: {"step":"worker","message":"..."}
```

### GET /tasks/evals?namespace=default&name={task-name}

获取 Monitor 评估历史（来自 Redis/内存状态存储）。

---

## Workflow 管理

### GET /workflows

列出所有 Workflow。

### GET /workflows/{namespace}/{name}

获取单个 Workflow 详情。

### GET /workflows/pending

获取待批准 Workflow（来自对话）。

### POST /workflows/approve

```json
{"workflow_id": "wf-1", "approver": "user"}
```

### POST /workflows/reject

```json
{"workflow_id": "wf-1", "reason": "暂不需要"}
```

### POST /workflows/create

直接创建 Workflow CRD。

```json
{
  "name": "novel-demo",
  "namespace": "default",
  "prompt": "写一部荒岛生存小说",
  "template": "novel-team-chapters",
  "params": {
    "chapterCount": "10",
    "wordsPerChapter": "2500",
    "teamMode": "true"
  }
}
```

`template` 可选值见 [Workflow 编排](workflow.md)。

---

## 小说库接口

小说库 API 将 Workflow CRD 与 SQLite 元数据合并为 `NovelSummary`。

### GET /novels

列出所有小说（Workflow + 元数据）。

**响应字段（节选）:**

| 字段 | 说明 |
|------|------|
| `phase` | Workflow 阶段 |
| `progress` | 完成百分比 |
| `pipeline_stage` | `outline` / `plots` / `chapters` / `merge` / `done` 等 |
| `chapters_done` | 已完成正文章数 |
| `plots_done` | 已完成剧情章数 |
| `workspace_path` | PVC 工作区路径 |
| `book_url` | 合并书稿访问路径 |

### POST /novels/create

从小说库 UI 创建原创 Workflow。

```json
{
  "name": "novel-island",
  "title": "荒岛求生",
  "prompt": "第三人称，荒岛生存题材……",
  "chapter_count": 20,
  "words_per_chapter": 2500,
  "quality_threshold": 78,
  "params": {"historicalEra": ""}
}
```

### POST /novels/import

导入已有全文并启动拆书 Workflow。

```json
{
  "title": "导入小说",
  "text": "第一章\n\n正文……\n\n第二章\n\n……",
  "continue_writing": true,
  "params": {"chapterCount": "50"}
}
```

- `text`：完整小说正文（按章切分）
- `continue_writing`：`false` 时仅拆书 + RAG 索引，不续写

### GET /novels/{namespace}/{name}

获取单本小说详情。

### DELETE /novels/{namespace}/{name}

删除 Workflow 及关联 Task。

### POST /novels/{namespace}/{name}/resume

恢复 Failed/Paused Workflow（等同 `scripts/resume-workflow.sh`）。

### GET /novels/{namespace}/{name}/rag/search?q={query}

在工作区内检索 RAG 剧情片段。

```json
{
  "success": true,
  "data": {
    "query": "主角第一次登岛",
    "count": 3,
    "chunks": [
      {"chapter": 1, "layer": "plot", "text": "...", "score": 0.92}
    ]
  }
}
```

检索模式由 Workflow `params.ragSearchMode` 控制（`keyword` / `vector` / `hybrid`）。

### POST /novels/{namespace}/{name}/chapters/{n}/regenerate

触发选章重写 Workflow。

```json
{
  "instruction": "加强悬念，结尾落在发现神秘脚印",
  "layer": "chapter"
}
```

- `layer`：`plot`（剧情层）或 `chapter`（正文层，默认）
- 响应含 `rewrite_workflow` 子 Workflow 名称

---

## 产出物接口

### GET /outputs/{path}

从 Manager PVC 读取产出文件。路径相对于 `/data/outputs`。

示例：`GET /outputs/workflows/default/novel-demo/book.md`

---

## WebSocket

暂不支持。任务进度请使用 `GET /tasks/events`（SSE）或轮询 `/tasks`、`/novels`。