# 快速开始

## 前置条件

- Kubernetes 集群 (v1.25+)
- kubectl 已配置并连接集群
- Docker 或 Podman 已安装
- Go 1.26+（开发构建用）

## 配置 AI

部署前配置 LLM 凭证（OpenAI 兼容 API，默认 DeepSeek）：

```bash
cp config/ai_config.example.yaml config/ai_config.local.yaml
# 编辑 api_key、base_url；勿提交 ai_config.local.yaml

export AI_BASE_URL=https://api.deepseek.com
export AI_API_KEY=sk-...

# 可选：仅 Worker 使用 xAI Grok — 在 local 文件中覆盖 worker.remote 段
# base_url: https://api.x.ai  model: grok-4.3
```

详见 [configuration.md](configuration.md)。Grok CLI 不能作为 API 接入；需使用 xAI 官方端点。

## 一键部署

```bash
# 克隆项目
git clone <repo-url>
cd agent-flow

# 本地 kind：先检查 inotify（agent-sandbox CrashLoop 时）
./scripts/fix-inotify.sh

# 一键部署（构建镜像 + 安装 CRD + 部署控制器 + Web）
./deploy.sh
```

部署脚本会自动完成：
1. 构建控制器和 Web 镜像
2. 安装 agent-sandbox controller（K8s 沙箱执行环境）
3. 安装 CRD（Task、Monitor）
4. 部署控制器和 Web 到 `agent-flow-system` 命名空间
5. 等待 Pod 就绪

## 部署选项

```bash
# 使用本地 kind 集群镜像仓库
./deploy.sh --local

# 跳过镜像构建（使用已有镜像）
./deploy.sh --no-build

# 跳过 agent-sandbox 安装
./deploy.sh --no-sandbox
```

## 访问 Web UI

Web UI 为 Vite + React 应用，由 `agentflow-web` 服务静态资源并代理 API。

```bash
# 推荐：一键 port-forward（kind 集群 LoadBalancer 无外部 IP）
./run-web.sh

# 或手动转发
kubectl port-forward -n agent-flow-system svc/agentflow-web 3000:80

# 浏览器打开 http://localhost:3000
```

功能：对话、任务列表、Workflow 监控、SSE 进度、章节浏览、**Token 报表**（`/tokens`）。

## 使用流程

### 1. 对话创建任务

在 Web UI 的对话框中输入你的需求：

```
用户: 写一首关于春天的七言绝句
```

AI Planner 会分析需求并生成任务方案，回复中会包含 `[CREATE_TASK:任务描述]` 标记。

### 2. 批准任务

AI 生成任务后，输入批准指令：

```
用户: 批准
```

或者在 Web UI 中点击"批准"按钮。

### 3. 等待执行

任务创建后，系统会：
1. 创建 Sandbox Pod 执行任务脚本
2. Worker AI 生成产出物
3. Monitor AI 评估质量
4. 不通过则自动重试（最多 3 次）

### 4. 查看结果

任务完成后，在 Web UI 中可以看到：
- 任务状态（Succeeded/Failed）
- 质量评分
- 产出物内容

## Workflow：100 章小说

```bash
# 部署集群 Planner（跳过 Sandbox，直连 AI）
make deploy-planner-cluster

# 创建 Workflow
kubectl apply -f config/samples/novel_parallel_demo.yaml

# 监控
kubectl get workflow novel-parallel-demo -n default -w
kubectl get tasks -n default -l agentflow.io/workflow=novel-parallel-demo

# 查看章节文件
kubectl exec -n agent-flow-system deploy/agentflow-manager -- \
  ls /data/outputs/workflows/default/novel-parallel-demo/chapters/
```

运维：

```bash
./scripts/resume-workflow.sh novel-parallel-demo default   # 恢复失败 Workflow
./scripts/cleanup-workflow-tasks.sh novel-parallel-demo default  # 清理 Task 残留
```

详见 [workflow.md](workflow.md)。

## 开发环境

```bash
# 构建
make build

# 运行测试
go test ./...

# 本地运行（需要 K8s 集群连接）
make run

# 构建镜像
make docker-build
make docker-build-web

# 生成 CRD 和 RBAC
make manifests
make generate
```

## CI

推送至 GitHub 后，`.github/workflows/ci.yml` 自动运行：

| Job | 内容 |
|-----|------|
| Go Test & Vet | `go test ./...`、`go vet ./...` |
| Go Lint | `golangci-lint`（配置见 `.golangci.yml`） |
| Build Go Binaries | `planner`、`worker-agent`、`mcp-server` |
| Build Web | `web/` 下 `npm ci && npm run build` |

本地可手动执行：

```bash
go test ./...
go vet ./...
make lint          # 需先安装 golangci-lint
cd web && npm ci && npm run build
```

## 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n agent-flow-system

# 检查 CRD
kubectl get crd | grep agentflow

# 检查 Task 列表
kubectl get tasks

# 查看控制器日志
kubectl logs -n agent-flow-system -l control-plane=controller-manager
```

## 卸载

```bash
# 卸载控制器和 CRD
make undeploy
make uninstall

# 卸载 agent-sandbox
make uninstall-agent-sandbox

# 或一键卸载
./deploy.sh --help  # 查看更多选项
```
