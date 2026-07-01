# Agent Flow Operator

基于 Kubernetes 和 eino 框架的 AI Agent 编排系统，支持单任务（Task）与多步骤工作流（Workflow），面向长篇小说生产级流水线。

## 项目概述

| 组件 | 职责 |
|------|------|
| **Planner** | 对话交互、任务/Workflow 编排、Sandbox / MCP 生命周期 |
| **Worker** | AI 执笔者：撰写大纲、章节正文 |
| **Polish** | 团队模式润色编辑：统一文风，不改剧情 |
| **Monitor** | 分层质检（L0 规则 + L1/L2 AI），未通过则带反馈重试 |
| **Workflow Controller** | 调研→大纲→设定圣经→逐章→合并，并行调度 |
| **MCP Sidecar** | 联网调研工具（维基检索、网页抓取、历史背景包） |

## 架构

```
用户 (Web UI / API / 对话)
        │
        ▼
  ChatRouter → Task CRD / Workflow CRD
        │
        ▼
  TaskPlannerEino + WorkflowController
        │
        ├─ [团队章节] Worker → Polish → Monitor（质量循环）
        ├─ [MCP 调研]  LocalAgent / Sidecar ReAct + 联网工具
        └─ 产出写入 PVC（正文）+ SQLite（元数据）
```

## 核心特性

- **AI 驱动对话** — Planner 理解需求并生成任务/Workflow 方案
- **团队流水线（默认）** — `novel-team-chapters`：大纲 → 设定圣经 → 执笔者 → 润色 → 多层质检 → 合并书稿
- **历史小说模式** — `novel-team-historical`：MCP 自动联网调研时代背景、真实人物、民俗制度，写入 `research_notes.md` 并注入后续写作
- **Workflow 模板** — 支持 10～100+ 章：大纲、分卷、并行章节、故事弧摘要
- **分段撰写** — 长章节拆为多段 AI 调用，单段失败可局部重试
- **三层重试** — 分段 / Task / Workflow，指数退避 + 针对性反馈
- **MCP 工具** — `historical_research`、`web_search`、`wikipedia_search`、`web_fetch` 及文件/HTTP 工具集
- **混合存储** — PVC 存正文，SQLite 存章节状态与元数据
- **三阶段扩写** — 默认 `threeStage=true`：梗概 → 剧情脚本 → 正文，降低长篇跑题
- **导入拆书** — `POST /novels/import`：导入全文 → AI 拆书 → RAG 索引 → 可选续写
- **RAG 剧情库** — 关键词检索工作区梗概/剧情/正文，写作时自动注入参考片段
- **Web UI** — Vite + React，小说库导入、三阶段进度、RAG 检索、SSE 进度、章节浏览
- **CI** — GitHub Actions：`go test` + 多二进制构建

## 快速开始

```bash
# 前置：Kubernetes 集群、kubectl、Docker

# 一键部署（控制器 + Web + agent-sandbox）
./deploy.sh --local

# 访问 Web UI（kind 集群无外部 IP，必须 port-forward）
./run-web.sh
# 浏览器打开 http://localhost:3000

# 保持 run-web.sh 终端不关闭；关闭后网页会打不开，需重新执行
```

### 创建历史小说（团队 + 联网调研）

**Web 对话**（推荐）：

```
写一部万历年间朝堂斗争的历史小说，10章，主角是翰林编修沈敬言
```

Planner 会自动选用 `novel-team-historical` 模板。

**API**：

```bash
kubectl port-forward -n agent-flow-system svc/agentflow-manager 18082:8082 &

curl -s -X POST http://127.0.0.1:18082/workflows/create \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "novel-wanli-court",
    "prompt": "写一部万历年间朝堂斗争的历史小说，主角沈敬言",
    "template": "novel-team-historical",
    "params": {
      "chapterCount": "10",
      "wordsPerChapter": "2500",
      "historicalEra": "明朝万历年间",
      "teamMode": "true"
    }
  }'
```

**聊天标记**：

```
[CREATE_WORKFLOW:novel-team-historical:{"chapterCount":"10","historicalEra":"明朝万历年间"}]
```

### 创建普通团队模式小说

```
[CREATE_WORKFLOW:novel-team-chapters:{"chapterCount":"20","wordsPerChapter":"2500"}]
```

### 监控进度

```bash
kubectl get workflow -n default -w
kubectl get tasks -l agentflow.io/workflow=<workflow-name>
```

工作区路径：`/data/outputs/workflows/<namespace>/<workflow-name>/`（含 `research_notes.md`、`outline.json`、`style_bible.json`、`chapters/`、`book.md`）

## Workflow 模板

| 模板 | 说明 |
|------|------|
| `novel-team-chapters` | **默认**团队流水线：设定圣经 + 润色 + 多层质检 |
| `novel-team-historical` | 团队流水线 + MCP 历史背景调研（自动识别古装/朝代题材） |
| `novel-outline-chapters` | 旧版单作者流水线（`teamMode=false` 时回退） |
| `novel-import-deconstruct` | 导入拆书 + RAG 索引 + 可选续写 |

### 团队流水线步骤（默认三阶段）

```
历史调研(MCP，可选) → 大纲 → 大纲精修 → 设定圣经 → 剧情(plots) → 正文(chapters) → 合并书稿
```

关闭三阶段时回退为：`大纲精修 → 逐章正文`（设 `threeStage=false`）。

### 导入已有小说（拆书 + 续写）

**Web 小说库** →「导入拆书」，粘贴 TXT 全文。

**API**：

```bash
curl -s -X POST http://127.0.0.1:18082/novels/import \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "我的旧稿",
    "text": "第1章 风起\n正文……",
    "continue_writing": true,
    "prompt": "拆书后续写方向说明（可选）"
  }'
```

流程：`import-deconstruct`（AI 拆书）→ `import-rag-index`（构建索引）→ 可选续写三阶段流水线。

### RAG 剧情检索

```bash
curl -s "http://127.0.0.1:18082/novels/default/<workflow-name>/rag/search?q=朝堂+宰相"
```

写作时 `ragEnabled=true`（默认）会自动将相关片段注入 plot / 正文 prompt。

### 常用参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `teamMode` | `true` | 启用团队模式（润色 + 团队质检） |
| `threeStage` | `true` | 梗概 → 剧情 → 正文三阶段扩写 |
| `ragEnabled` | `true` | 启用 RAG 检索注入 |
| `ragTopK` | `5` | 每次检索返回片段数 |
| `plotWords` | `1000` | 剧情脚本目标字数 |
| `historicalResearch` | 自动 | 历史/朝代题材自动开启 MCP 调研 |
| `historicalEra` | — | 时代锚点，如 `明朝万历年间` |
| `chapterCount` | `10` | 章节数 |
| `wordsPerChapter` | `2500` | 每章目标字数 |
| `qualityThreshold` | `78` | 质检通过分数 |
| `contextChapters` | `8` | 跨章上下文窗口 |

## 访问 Web UI 故障排查

kind 本地集群 **没有公网 IP**，直接访问集群 Service 会失败。

| 现象 | 处理 |
|------|------|
| 网页打不开 / 连接拒绝 | 执行 `./run-web.sh`，访问 http://localhost:3000 |
| 页面能开但无数据 | 确认 `agentflow-manager`、`agentflow-web` Pod 均为 Running |
| API 调试 | `kubectl port-forward -n agent-flow-system svc/agentflow-manager 18082:8082` |

```bash
kubectl get pods -n agent-flow-system
kubectl rollout status deployment/agentflow-manager -n agent-flow-system
kubectl rollout status deployment/agentflow-web -n agent-flow-system
```

## 项目结构

```
agent-flow/
├── cmd/
│   ├── planner/           # 主控制器（Task + Workflow）
│   ├── worker-agent/      # MCP ReAct Agent
│   └── mcp-server/        # MCP Sidecar（联网工具）
├── api/v1alpha1/          # Task、Workflow、Monitor CRD
├── internal/
│   ├── architecture/      # K8s 控制器 + MCP 本地执行
│   ├── flow/              # eino Worker-Polish-Monitor 流程
│   ├── workflow/          # Workflow 引擎、团队/历史模板
│   ├── mcp/               # 工具集（web_search、historical_research 等）
│   ├── retry/             # 重试分类与指数退避
│   ├── store/             # SQLite 小说元数据
│   └── api/               # HTTP API
├── web/                   # React Web UI
├── config/                # Kustomize、样本 Workflow、AI 配置
├── scripts/               # 部署与运维脚本
└── .github/workflows/     # CI
```

## 运维脚本

| 脚本 | 用途 |
|------|------|
| `./deploy.sh` | 全量一键部署 |
| `./scripts/deploy-planner-cluster.sh` | 构建并部署 agentflow-manager |
| `./run-web.sh` | port-forward Web UI → http://localhost:3000 |
| `./scripts/resume-workflow.sh` | 恢复失败的 Workflow |
| `./scripts/cleanup-workflow-tasks.sh` | 清理已完成 Task CRD 残留 |

## 开发与 CI

```bash
make build          # 构建 planner
go test ./...       # 运行测试
make deploy-planner-cluster

# 环境变量
export AGENTFLOW_LOG_FORMAT=text    # 或 json
export AGENTFLOW_LOG_LEVEL=info
export AGENTFLOW_SKIP_SANDBOX=true # 本地跳过 Sandbox，Planner 内直接跑 AI / MCP
```

推送至 GitHub 后，`.github/workflows/ci.yml` 自动执行测试与编译。

## 文档

| 文档 | 说明 |
|------|------|
| [docs/README.md](docs/README.md) | 文档索引 |
| [docs/getting-started.md](docs/getting-started.md) | 快速开始与 Web 访问 |
| [docs/architecture.md](docs/architecture.md) | 架构设计 |
| [docs/workflow.md](docs/workflow.md) | Workflow 与小说任务 |
| [docs/configuration.md](docs/configuration.md) | AI、日志、重试配置 |
| [docs/deployment.md](docs/deployment.md) | 部署与故障排查 |

## 技术栈

Go · eino · Kubernetes · controller-runtime · agent-sandbox · MCP · slog · React · SQLite · Kustomize

## 许可证

MIT License