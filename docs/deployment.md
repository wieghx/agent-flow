# 部署指南

## 部署架构

```
┌─────────────────────────────────────────────┐
│            agent-flow-system                 │
│                                             │
│  ┌─────────────────┐  ┌─────────────────┐  │
│  │ agentflow-manager│  │  agentflow-web  │  │
│  │ (Go 控制器)      │  │  (Node.js 代理) │  │
│  │ :8080 (metrics)  │  │  :3000 (Web UI) │  │
│  │ :8081 (health)   │  │                 │  │
│  │ :8082 (chat API) │  │                 │  │
│  └─────────────────┘  └─────────────────┘  │
│           │                      │          │
│           │ proxy                │          │
│           └──────────────────────┘          │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│         agent-sandbox-system                 │
│                                             │
│  ┌─────────────────────────────────────┐    │
│  │   agent-sandbox controller          │    │
│  │   (官方沙箱执行环境)                  │    │
│  └─────────────────────────────────────┘    │
│                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Sandbox  │ │ Sandbox  │ │ Sandbox  │   │
│  │ (Pod)    │ │ (Pod)    │ │ (Pod)    │   │
│  └──────────┘ └──────────┘ └──────────┘   │
└─────────────────────────────────────────────┘
```

## 一键部署

```bash
./deploy.sh
```

部署脚本自动执行以下步骤：

1. 构建 Go 控制器二进制和 Docker 镜像
2. 安装 agent-sandbox controller
3. 安装 CRD（Task、Monitor）
4. 部署控制器和 Web 服务
5. 等待所有 Pod 就绪

## 手动部署

### 1. 构建镜像

```bash
# 构建控制器
make build
make docker-build

# 构建 Web
make docker-build-web
```

### 2. 安装 agent-sandbox

```bash
make install-agent-sandbox
```

### 3. 安装 CRD

```bash
make install
```

### 4. 部署控制器

```bash
# 更新镜像地址
sed -i 's|image: .*|image: your-registry/agent-flow-planner:latest|g' config/manager/deployment.yaml

# 部署
make deploy
```

### 5. 部署 Web

```bash
# 更新镜像地址
sed -i 's|image: .*|image: your-registry/agent-flow-web:latest|g' config/web/deployment.yaml

# 部署
make deploy-web
```

## 镜像配置

### 默认镜像

| 镜像 | 用途 |
|------|------|
| `minagflow/agent-flow-planner:latest` | 控制器 |
| `minagflow/agent-flow-web:latest` | Web UI |

### 自定义镜像

```bash
# 构建时指定
IMAGE_REGISTRY=your-registry make docker-build
IMAGE_REGISTRY=your-registry make docker-build-web

# 或通过环境变量
PLANNER_IMAGE=your-registry/planner:v1.0 ./deploy.sh
WEB_IMAGE=your-registry/web:v1.0 ./deploy.sh
```

### 本地 Kind 集群

```bash
# 使用本地镜像仓库
./deploy.sh --local

# 镜像会推送到 kind-local:5000
```

## CI/CD

GitHub Actions 工作流：`.github/workflows/ci.yml`

| Job | 触发条件 | 说明 |
|-----|----------|------|
| Go Test & Vet | push / PR → `main` | 单元测试 + `go vet` |
| Go Lint | push / PR → `main` | `golangci-lint`（`.golangci.yml`） |
| Build Go Binaries | 测试与 lint 通过后 | 编译三个 Go 二进制 |
| Build Web | 测试通过后 | TypeScript 检查 + Vite 生产构建 |

本地提交前建议：

```bash
go test ./... -count=1
go vet ./...
cd web && npm run build
```

E2E 脚本 `scripts/e2e-novel-local.sh` 需 K8s 集群与可用 AI，暂未纳入 CI。

## 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n agent-flow-system

# 检查 CRD
kubectl get crd | grep -E 'agentflow|agents.x-k8s'

# 检查控制器日志
kubectl logs -n agent-flow-system -l control-plane=controller-manager -f

# 测试 Chat API
curl -X POST http://localhost:8082/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'
```

## 访问 Web UI

```bash
./run-web.sh
# 或 kubectl port-forward -n agent-flow-system svc/agentflow-web 3000:80
# http://localhost:3000
```

kind 集群 Service 类型为 ClusterIP，必须使用 port-forward。

## 集群 Planner 部署

开发环境推荐将 Planner 跑在集群内（共享 PVC、避免本机双控制器）：

```bash
./scripts/deploy-planner-cluster.sh --local
# 等价于 make deploy-planner-cluster
```

脚本会：构建镜像 → 加载 kind → 安装 CRD → 部署 manager → 停止本机 planner 进程。

常用环境变量（Deployment 中配置）：

```bash
AGENTFLOW_SKIP_SANDBOX=true
AGENTFLOW_OUTPUT_DIR=/data/outputs
```

## Workflow 运维

```bash
kubectl apply -f config/samples/novel_parallel_demo.yaml
./scripts/resume-workflow.sh novel-parallel-demo default
./scripts/cleanup-workflow-tasks.sh novel-parallel-demo default
```

## 故障排查

### agent-sandbox-controller CrashLoopBackOff

本地 kind + controller-runtime 会大量占用 inotify。宿主机上限过低时日志出现 `too many open files`，sandbox 控制器反复重启。

```bash
./scripts/fix-inotify.sh

# 若提示不足，以 root 执行：
sudo sysctl -w fs.inotify.max_user_instances=512
sudo sysctl -w fs.inotify.max_user_watches=524288

# 持久化（可选）
sudo tee /etc/sysctl.d/99-agent-flow-inotify.conf >/dev/null <<EOF
fs.inotify.max_user_instances=512
fs.inotify.max_user_watches=524288
EOF
sudo sysctl --system

kubectl delete pod -n agent-sandbox-system -l app=agent-sandbox-controller
```

### 控制器无法启动

```bash
# 检查日志
kubectl logs -n agent-flow-system -l control-plane=controller-manager --tail=100

# 常见问题:
# - AI 配置文件错误
# - K8s 集群连接失败
# - CRD 未安装
```

### Sandbox 创建失败

```bash
# 检查 agent-sandbox controller
kubectl get pods -n agent-sandbox-system

# 检查 Sandbox 状态
kubectl get sandboxes

# 检查 RBAC
kubectl auth can-i create sandboxes -n default
```

### 任务执行超时

```bash
# 检查 Task 状态
kubectl get tasks -o wide

# 检查 Sandbox Pod 日志
kubectl logs <sandbox-pod-name>

# 调整超时配置
# config/ai_config.yaml → timeout_seconds
```

### Workflow 卡住或 Failed

```bash
kubectl get workflow novel-parallel-demo -n default -o yaml
kubectl get tasks -n default -l agentflow.io/workflow=novel-parallel-demo

# 恢复
./scripts/resume-workflow.sh novel-parallel-demo default

# 清理已完成 Task CRD（章节文件保留）
./scripts/cleanup-workflow-tasks.sh novel-parallel-demo default
```

### 日志中出现 `task not found`

清理 Task CRD 后，控制器队列中可能仍有陈旧 reconcile 事件，会短暂出现该日志，排空后消失。已将此类日志降为 debug 级别。

### Workflow status 冲突

`the object has been modified` 为多控制器并发写 status 的乐观锁冲突，会自动重试，一般可忽略。

## 卸载

```bash
# 卸载控制器
make undeploy

# 卸载 CRD
make uninstall

# 卸载 agent-sandbox
make uninstall-agent-sandbox

# 删除命名空间（可选）
kubectl delete namespace agent-flow-system
```
