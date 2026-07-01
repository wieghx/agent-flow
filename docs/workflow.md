# Workflow 编排

Agent Flow 支持多步骤 **Workflow** CRD，用于大纲生成、分卷并行、章节流水线等长任务。

## 小说模板 `novel-outline-chapters`

样本配置：`config/samples/novel_parallel_demo.yaml`

| 参数 | 说明 | 示例 |
|------|------|------|
| `chapterCount` | 章节总数 | `100` |
| `wordsPerChapter` | 每章目标字数 | `2500` |
| `chapterSegmentMode` | 分段撰写 | `true` |
| `chapterSegments` | 每章分段数 | `5` |
| `segmentWords` | 每段目标字数 | `500` |
| `taskMaxRetries` | Task 内重试上限（章节） | `6` |
| `qualityThreshold` | Monitor 通过分数 | `72` |

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

## 创建与监控

```bash
# 创建 100 章 Workflow
kubectl apply -f config/samples/novel_parallel_demo.yaml

# 查看进度
kubectl get workflow novel-parallel-demo -n default -w

# 查看关联 Task
kubectl get tasks -n default -l agentflow.io/workflow=novel-parallel-demo

# 查看章节产出（PVC 内）
kubectl exec -n agent-flow-system deploy/agentflow-manager -- \
  ls /data/outputs/workflows/default/novel-parallel-demo/chapters/
```

## 产出路径

| 类型 | 路径 |
|------|------|
| 工作区根 | `/data/outputs/workflows/<namespace>/<workflow-name>/` |
| 大纲 | `outline.json`、`volumes/volume-*.json` |
| 章节正文 | `chapters/chapter-NNN.md` |
| 章节摘要 | `chapters/chapter-NNN.summary` |
| 故事弧 | `arcs/arc-*.md` |
| SQLite 元数据 | `/data/outputs/agentflow/novels.db` |

## 运维脚本

### 恢复 Failed/Paused Workflow

```bash
./scripts/resume-workflow.sh novel-parallel-demo default
```

删除失败 Task、将 Workflow 状态重置为 `Running`。

### 清理已完成 Task 残留

```bash
./scripts/cleanup-workflow-tasks.sh novel-parallel-demo default
```

删除已完成步骤的 Task CRD（章节文件保留在 PVC）。清理后 manager 日志可能出现短暂的 `task not found`（队列排空），属正常现象。

### 集群部署 Planner

```bash
./scripts/deploy-planner-cluster.sh --local
# 或
make deploy-planner-cluster
```

构建镜像、部署 `agentflow-manager`，默认 `AGENTFLOW_SKIP_SANDBOX=true` 时 Planner 直连 AI。

## 重试策略（三层）

```
分段失败 → 只重试该段（最多 3 次，2s 起指数退避）
    ↓
Task 失败 → 分类反馈 + 指数退避（章节最多 6 次）
    ↓
Workflow 失败 → 等待退避后重派新 Task（最多 3 次）
```

实现：`internal/retry/`。全局退避参数见 `config/ai_config.yaml` 的 `retry` 段。

## Web UI 监控

```bash
./run-web.sh
# 浏览器打开 http://localhost:3000
```

支持 Workflow 列表、SSE 任务进度、章节浏览（kind 集群需 port-forward，见 `run-web.sh`）。