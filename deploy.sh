#!/bin/bash
# Agent Flow 一键部署脚本
# 用法: ./deploy.sh [--local] [--no-build] [--no-sandbox]

set -e

echo "========================================="
echo "  Agent Flow - K8s 一键部署"
echo "========================================="

# ---- 参数解析 ----
LOCAL=false
NO_BUILD=false
NO_SANDBOX=false
for arg in "$@"; do
  case $arg in
    --local) LOCAL=true ;;
    --no-build) NO_BUILD=true ;;
    --no-sandbox) NO_SANDBOX=true ;;
    --help|-h)
      echo "用法: ./deploy.sh [选项]"
      echo "  --local       使用本地 kind-local:5000 镜像仓库（默认）"
      echo "  --no-build    跳过镜像构建，使用已有镜像"
      echo "  --no-sandbox  跳过 agent-sandbox 安装"
      exit 0 ;;
  esac
done

# ---- 前置检查 ----
if ! command -v kubectl &> /dev/null; then
    echo "错误: kubectl 未安装"
    exit 1
fi

echo "检查集群连接..."
kubectl cluster-info &> /dev/null || {
    echo "错误: 无法连接到 Kubernetes 雜群"
    exit 1
}
echo "集群连接成功!"

# kind + rootless podman：inotify 实例耗尽会导致 kube-proxy CrashLoop
if [ "$LOCAL" = true ]; then
    INOTIFY_USED=$(find /proc/*/fd -lname 'anon_inode:inotify' 2>/dev/null | wc -l | tr -d ' ')
    INOTIFY_MAX=$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo 128)
    if [ -n "$INOTIFY_USED" ] && [ -n "$INOTIFY_MAX" ] && [ "$INOTIFY_USED" -ge "$((INOTIFY_MAX - 2))" ]; then
        echo "警告: inotify ${INOTIFY_USED}/${INOTIFY_MAX}，kube-proxy 可能无法启动。"
        echo "  建议: sudo sysctl -w fs.inotify.max_user_instances=512"
        echo "  或先运行: RECREATE=true ./scripts/setup-kind-cluster.sh"
    fi
fi

cd "$(dirname "$0")"

# ---- 镜像配置 ----
NAMESPACE="agent-flow-system"
if [ "$LOCAL" = true ]; then
    PLANNER_IMAGE="kind-local:5000/minagflow/agent-flow-planner:latest"
    WEB_IMAGE="kind-local:5000/minagflow/agent-flow-web:latest"
    MCP_IMAGE="kind-local:5000/minagflow/mcp-sidecar:latest"
    WORKER_IMAGE="kind-local:5000/minagflow/worker-agent:latest"
    PROXY_IMAGE="kind-local:5000/minagflow/proxy-sidecar:latest"
else
    PLANNER_IMAGE="${PLANNER_IMAGE:-minagflow/agent-flow-planner:latest}"
    WEB_IMAGE="${WEB_IMAGE:-minagflow/agent-flow-web:latest}"
    MCP_IMAGE="${MCP_IMAGE:-minagflow/mcp-sidecar:latest}"
    WORKER_IMAGE="${WORKER_IMAGE:-minagflow/worker-agent:latest}"
    PROXY_IMAGE="${PROXY_IMAGE:-minagflow/proxy-sidecar:latest}"
fi

# ---- Step 1: 构建镜像 ----
if [ "$NO_BUILD" = false ]; then
    echo ""
    echo "[1/6] 构建镜像..."

    # 构建控制器镜像
    echo "  构建控制器镜像: $PLANNER_IMAGE"
    CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/planner cmd/planner/main.go
    docker build -t "$PLANNER_IMAGE" -f Dockerfile .

    # 构建 MCP sidecar 镜像
    echo "  构建 MCP sidecar 镜像: $MCP_IMAGE"
    docker build -t "$MCP_IMAGE" -f Dockerfile.mcp .

    # 构建 Worker Agent 镜像
    echo "  构建 Worker Agent 镜像: $WORKER_IMAGE"
    docker build -t "$WORKER_IMAGE" -f Dockerfile.worker .

    # 构建 Web 镜像
    echo "  构建 Web 镜像: $WEB_IMAGE"
    docker build -t "$WEB_IMAGE" -f Dockerfile.web .

    # 构建 proxy sidecar 镜像（复用 v2rayN 自带 sing-box）
    echo "  构建 proxy sidecar 镜像: $PROXY_IMAGE"
    SING_BOX_SRC="${SING_BOX_SRC:-$HOME/.local/share/v2rayN/bin/sing_box/sing-box}"
    if [ ! -x "$SING_BOX_SRC" ]; then
        echo "错误: 未找到 sing-box 二进制: $SING_BOX_SRC" >&2
        exit 1
    fi
    cp "$SING_BOX_SRC" bin/sing-box
    docker build -t "$PROXY_IMAGE" -f Dockerfile.proxy-sidecar .

    # 推送到本地 kind 集群（如果可用）
    if docker info 2>/dev/null | grep -q "kind-local:5000"; then
        echo "  推送到 kind-local:5000..."
        docker push "$PLANNER_IMAGE" 2>/dev/null || true
        docker push "$MCP_IMAGE" 2>/dev/null || true
        docker push "$WORKER_IMAGE" 2>/dev/null || true
        docker push "$WEB_IMAGE" 2>/dev/null || true
        docker push "$PROXY_IMAGE" 2>/dev/null || true
    fi
    # 加载镜像到 kind 集群（避免节点无法拉取外网镜像）
    if kind get clusters 2>/dev/null | grep -q .; then
        KIND_CLUSTER=$(kind get clusters 2>/dev/null | head -1)
        echo "  加载镜像到 kind 集群 ($KIND_CLUSTER)..."
        for img in "$PLANNER_IMAGE" "$MCP_IMAGE" "$WORKER_IMAGE" "$WEB_IMAGE" "$PROXY_IMAGE" \
            registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.5.0 \
            redis:7-alpine; do
            kind load docker-image "$img" --name "$KIND_CLUSTER" 2>/dev/null || true
        done
    fi
    echo "  镜像构建完成!"
else
    echo "[1/6] 跳过镜像构建 (--no-build)"
fi

# ---- Step 2: 安装 agent-sandbox ----
if [ "$NO_SANDBOX" = false ]; then
    echo ""
    echo "[2/6] 安装 agent-sandbox controller..."
    make install-agent-sandbox

    echo "  等待 agent-sandbox controller 就绪..."
    kubectl wait --namespace agent-sandbox-system \
        --for=condition=ready pod \
        --selector=app=agent-sandbox-controller \
        --timeout=180s 2>/dev/null || echo "  agent-sandbox 可能尚未就绪，继续..."
else
    echo "[2/6] 跳过 agent-sandbox (--no-sandbox)"
fi

# ---- Step 3: 安装 CRD ----
echo ""
echo "[3/6] 安装 CRD..."
make install

# ---- Step 4: 更新镜像配置 ----
echo ""
echo "[4/6] 更新镜像配置..."
sed -i "s|image: .*minagflow/agent-flow-planner:.*|image: $PLANNER_IMAGE|g" config/manager/deployment.yaml
sed -i "s|image: .*minagflow/proxy-sidecar:.*|image: $PROXY_IMAGE|g" config/manager/deployment.yaml
sed -i "s|image: .*minagflow/agent-flow-web:.*|image: $WEB_IMAGE|g" config/web/deployment.yaml

# ---- Step 5: 部署 ----
echo ""
echo "[5/6] 部署控制器 + Web..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap ai-config \
  --namespace "$NAMESPACE" \
  --from-file=ai_config.yaml=config/ai_config.yaml \
  --dry-run=client -o yaml | kubectl apply -f -
echo "  同步 proxy sidecar 配置（从 v2rayN 提取节点）..."
chmod +x "$(dirname "$0")/scripts/build-proxy-sidecar-config.sh"
"$(dirname "$0")/scripts/build-proxy-sidecar-config.sh"
kubectl create secret generic proxy-sidecar-config \
  --namespace "$NAMESPACE" \
  --from-file=config.json=config/proxy-sidecar.local.json \
  --dry-run=client -o yaml | kubectl apply -f -

# shellcheck source=scripts/read-ai-secrets.sh
source "$(dirname "$0")/scripts/read-ai-secrets.sh"
SECRET_ARGS=()
[ -n "${AI_API_KEY:-}" ] && SECRET_ARGS+=(--from-literal=AI_API_KEY="$AI_API_KEY")
[ -n "${AI_BASE_URL:-}" ] && SECRET_ARGS+=(--from-literal=AI_BASE_URL="$AI_BASE_URL")
[ -n "${WORKER_AI_API_KEY:-}" ] && SECRET_ARGS+=(--from-literal=WORKER_AI_API_KEY="$WORKER_AI_API_KEY")
[ -n "${WORKER_AI_BASE_URL:-}" ] && SECRET_ARGS+=(--from-literal=WORKER_AI_BASE_URL="$WORKER_AI_BASE_URL")
if [ "${#SECRET_ARGS[@]}" -gt 0 ]; then
  kubectl create secret generic ai-secrets \
    --namespace "$NAMESPACE" \
    "${SECRET_ARGS[@]}" \
    --dry-run=client -o yaml | kubectl apply -f -
fi
make deploy

# ---- Step 6: 等待就绪 ----
echo ""
echo "[6/6] 等待 Pod 就绪..."
kubectl -n "$NAMESPACE" rollout status deployment/agentflow-manager --timeout=120s 2>/dev/null || true
kubectl -n "$NAMESPACE" rollout status deployment/agentflow-web --timeout=120s 2>/dev/null || true

# ---- 输出结果 ----
echo ""
echo "========================================="
echo "  部署完成!"
echo "========================================="
echo ""
echo "命名空间: $NAMESPACE"
echo ""
echo "Pod 状态:"
kubectl get pods -n "$NAMESPACE" -o wide
echo ""
echo "Service 状态:"
kubectl get svc -n "$NAMESPACE"
echo ""
echo "CRD 列表:"
kubectl get crd | grep -E 'agentflow|agents.x-k8s' || true
echo ""
echo "Web 访问方式:"
WEB_SVC=$(kubectl get svc -n "$NAMESPACE" web -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
WEB_NODEPORT=$(kubectl get svc -n "$NAMESPACE" web -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
if [ -n "$WEB_SVC" ] && [ "$WEB_SVC" != "" ]; then
    echo "  http://$WEB_SVC"
elif [ -n "$WEB_NODEPORT" ]; then
    echo "  http://<任意节点IP>:$WEB_NODEPORT"
else
    echo "  ./run-web.sh"
    echo "  然后访问 http://localhost:3000"
fi
