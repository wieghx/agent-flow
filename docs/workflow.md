# Workflow 编排

Agent Flow 通过 **Workflow CRD** 编排多步骤流水线，覆盖大纲、三阶段扩写、团队润色质检、历史调研、导入拆书与选章重写。

## Workflow 模板

| 模板 | 说明 | 典型场景 |
|------|------|----------|
| `novel-team-chapters` | **默认**团队流水线：设定圣经 → 三阶段扩写 → 润色 → L0/L1/L2 质检 → 合并 | 原创长篇 |
| `novel-team-historical` | 团队流水线 + MCP 历史背景调研 | 古装 / 朝代题材 |
| `novel-outline-chapters` | 单作者流水线（`teamMode=false` 时回退） | 轻量测试 |
| `novel-import-deconstruct` | 导入全文 → AI 拆书 → RAG 索引 → 可选续写 | 拆书、仿写、续写 |
| `novel-chapter-rewrite` | 按指令重写单章剧情或正文 | 阅读器「选章重写」 |

Planner 对话或 API 未指定模板时，默认按 `teamMode=true` 选用 `novel-team-chapters`；检测到历史/朝代关键词时自动切换 `novel-team-historical`。

### 团队流水线（`novel-team-chapters`）

```
历史调研(MCP，可选) → 大纲 → 设定圣经 → 逐章[梗概 → 剧情 → 执笔者 → 润色 → 质检] → 合并书稿
```

三阶段扩写（`threeStage=true`，默认开启）将每章拆为：

1. **梗概**（`chapters/chapter-NNN.outline`）
2. **剧情脚本**（`chapters/chapter-NNN.plot`）
3. **正文**（`chapters/chapter-NNN.md`）

Web UI 的 `pipeline_stage` 字段反映当前阶段：`outline` → `plots` → `chapters` → `merge` → `done`。

### 导入拆书（`novel-import-deconstruct`）

```
写入 import/source.md → 拆书提取 outline.json → 构建 RAG 索引 →（可选）续写新章
```

通过 `POST /novels/import` 创建；`continue_writing=false` 时仅拆书与建索引。

### 选章重写（`novel-chapter-rewrite`）

由 `POST /novels/{ns}/{name}/chapters/{n}/regenerate` 触发，在父 Workflow 工作区内重写指定章节的 `plot` 或 `chapter` 层。

## 常用参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `teamMode` | `true` | 启用团队模式（润色 + 团队质检） |
| `threeStage` | `true` | 梗概 → 剧情 → 正文三阶段扩写 |
| `historicalResearch` | 自动 | 历史题材自动开启 MCP 调研 |
| `historicalEra` | — | 时代锚点，如 `明朝万历年间` |
| `chapterCount` | `10` | 章节数 |
| `wordsPerChapter` | `2500` | 每章目标字数 |
| `qualityThreshold` | `78` | 质检通过分数 |
| `contextChapters` | `8` | 跨章上下文窗口 |
| `ragEnabled` | `true` | 写作时注入 RAG 参考片段 |
| `ragSearchMode` | `hybrid` | `keyword` / `vector` / `hybrid` |
| `ragTopK` | `5` | 检索返回片段数 |
| `chapterSegmentMode` | `true` | 长章分段撰写 |
| `chapterSegments` | `5` | 每章分段数 |
| `segmentWords` | `500` | 每段目标字数 |
| `taskMaxRetries` | `6` | Task 内重试上限 |
| `maxParallel` | `8` | 最大并行 Task 数 |
| `chapterPipeline` | `8` | 章节流水线宽度 |

完整默认值见 `internal/workflow/params.go` 的 `DefaultNovelParams`。

### 执行策略 `spec.execution`

| 字段 | 说明 | 推荐值 |
|------|------|--------|
| `mode` | `parallel` / `sequential` | `parallel` |
| `maxParallel` | 最大并行 Task 数 | `8` |
| `chapterMode` | `pipeline` 滑动窗口并行章节 | `pipeline` |
| `chapterPipeline` | 章节流水线宽度 | `8` |
| `stepMaxRetries` | Workflow 层重派上限 | `3` |
| `stepRetryBaseDelaySec` | 步骤重试指数退避基数（秒） | `30` |
| `stepRetryMaxDelaySec` | 步骤重试退避上限（秒） | `300` |
| `pauseOnStepFail` | 阻塞步骤失败时是否暂停 | `false` |

## 创建 Workflow

### Web 对话（推荐）

```
写一部万历年间朝堂斗争的历史小说，10章，主角是翰林编修沈敬言
```

或显式标记：

```
[CREATE_WORKFLOW:novel-team-chapters:{"chapterCount":"20","wordsPerChapter":"2500"}]
[CREATE_WORKFLOW:novel-team-historical:{"chapterCount":"10","historicalEra":"明朝万历年间"}]
```

### API

```bash
kubectl port-forward -n agent-flow-system svc/agentflow-manager 18082:8082 &

curl -s -X POST http://127.0.0.1:18082/workflows/create \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "novel-demo",
    "prompt": "写一部荒岛生存小说",
    "template": "novel-team-chapters",
    "params": {"chapterCount": "10", "wordsPerChapter": "2500"}
  }'
```

### kubectl 样本

```bash
kubectl apply -f config/samples/novel_parallel_demo.yaml
kubectl apply -f config/samples/novel_workflow_100_local.yaml
```

## 监控进度

```bash
kubectl get workflow -n default -w
kubectl get tasks -l agentflow.io/workflow=<workflow-name>
./run-web.sh   # http://localhost:3000
```

## 产出路径

| 类型 | 路径 |
|------|------|
| 工作区根 | `/data/outputs/workflows/<namespace>/<workflow-name>/` |
| 历史调研 | `research_notes.md` |
| 大纲 | `outline.json`、`volumes/volume-*.json` |
| 设定圣经 | `style_bible.json` |
| 章节梗概 | `chapters/chapter-NNN.outline` |
| 章节剧情 | `chapters/chapter-NNN.plot` |
| 章节正文 | `chapters/chapter-NNN.md` |
| 章节摘要 | `chapters/chapter-NNN.summary` |
| 故事弧 | `arcs/arc-*.md` |
| 合并书稿 | `book.md` |
| RAG 索引 | `rag/index.json`、`rag/vectors.json` |
| SQLite 元数据 | `/data/outputs/agentflow/novels.db` |

## 运维脚本

### 恢复 Failed/Paused Workflow

```bash
./scripts/resume-workflow.sh novel-parallel-demo default
# 或 API: POST /novels/{namespace}/{name}/resume
```

### 清理已完成 Task 残留

```bash
./scripts/cleanup-workflow-tasks.sh novel-parallel-demo default
```

### 本地 E2E（2 章小说）

```bash
./scripts/e2e-novel-local.sh
```

需已部署集群且配置可用 AI；默认 `AGENTFLOW_SKIP_SANDBOX=true`。

## 重试策略（三层）

```
分段失败 → 只重试该段（最多 3 次，2s 起指数退避）
    ↓
Task 失败 → 分类反馈 + 指数退避（章节最多 6 次）
    ↓
Workflow 失败 → 等待退避后重派新 Task（最多 3 次）
```

实现：`internal/retry/`。全局退避参数见 `config/ai_config.yaml` 的 `retry` 段。