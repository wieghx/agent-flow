#!/usr/bin/env bash
# 将 Planner 部署到 Kubernetes 集群（替代本机 nohup ./bin/planner）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

LOCAL="${LOCAL:-true}"
NO_BUILD="${NO_BUILD:-false}"
SKIP_SANDBOX="${SKIP_SANDBOX:-false}"
NAMESPACE="${NAMESPACE:-agent-flow-system}"
SCALE_DOWN_LOCAL="${SCALE_DOWN_LOCAL:-true}"

while [ $# -gt 0 ]; do
  case "$1" in
    --local) LOCAL=true ;;
    --no-build) NO_BUILD=true ;;
    --no-sandbox) SKIP_SANDBOX=true ;;
    --keep-local-planner) SCALE_DOWN_LOCAL=false ;;
    -h|--help)
      echo "用法: $0 [--local] [--no-build] [--no-sandbox] [--keep-local-planner]"
      echo "  构建镜像、部署 agentflow-manager 到集群，并停止本机 planner 避免双控制器冲突。"
      exit 0
      ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
  shift
done

if [ "$LOCAL" = true ]; then
  PLANNER_IMAGE="kind-local:5000/minagflow/agent-flow-planner:latest"
else
  PLANNER_IMAGE="${PLANNER_IMAGE:-ghcr.io/minagflow/agent-flow-planner:latest}"
fi

echo "==> [1/6] 构建 planner 二进制与镜像"
if [ "$NO_BUILD" = false ]; then
  CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/planner cmd/planner/main.go
  docker build -t "$PLANNER_IMAGE" -f Dockerfile .
  if kind get clusters 2>/dev/null | grep -q .; then
    KIND_CLUSTER=$(kind get clusters 2>/dev/null | head -1)
    echo "    加载镜像到 kind: $KIND_CLUSTER"
    kind load docker-image "$PLANNER_IMAGE" --name "$KIND_CLUSTER" 2>/dev/null || true
  fi
else
  echo "    跳过构建 (--no-build)"
fi

echo "==> [2/6] 安装 CRD"
make install

echo "==> [3/6] 同步 AI ConfigMap"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap ai-config \
  --namespace "$NAMESPACE" \
  --from-file=ai_config.yaml=config/ai_config.yaml \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> [4/6] 同步 AI Secret（可选）"
# shellcheck source=scripts/read-ai-secrets.sh
source "$(dirname "$0")/read-ai-secrets.sh"
if [ -n "${AI_API_KEY:-}" ] && [ -n "${AI_BASE_URL:-}" ]; then
  kubectl create secret generic ai-secrets \
    --namespace "$NAMESPACE" \
    --from-literal=AI_API_KEY="$AI_API_KEY" \
    --from-literal=AI_BASE_URL="$AI_BASE_URL" \
    --dry-run=client -o yaml | kubectl apply -f -
elif [ -n "${AI_API_KEY:-}" ]; then
  kubectl create secret generic ai-secrets \
    --namespace "$NAMESPACE" \
    --from-literal=AI_API_KEY="$AI_API_KEY" \
    --dry-run=client -o yaml | kubectl apply -f -
else
  echo "    未设置 AI_API_KEY（env 或 config/ai_config.local.yaml），跳过 Secret"
fi

echo "==> [5/6] 更新 deployment 镜像并部署"
sed -i "s|image: .*minagflow/agent-flow-planner:.*|image: $PLANNER_IMAGE|g" config/manager/deployment.yaml
make deploy

echo "==> [6/6] 等待 agentflow-manager 就绪"
kubectl -n "$NAMESPACE" rollout status deployment/agentflow-manager --timeout=180s
kubectl -n "$NAMESPACE" get pods -l control-plane=controller-manager -o wide

if [ "$SCALE_DOWN_LOCAL" = true ]; then
  echo "==> 停止本机 planner（避免双控制器）"
  if pgrep -x planner >/dev/null 2>&1; then
    pkill -x planner || true
    sleep 2
    echo "    本机 planner 已停止"
  else
    echo "    本机 planner 未运行"
  fi
else
  echo "==> 保留本机 planner (--keep-local-planner)"
fi

echo ""
echo "Planner 已在集群运行:"
echo "  kubectl get pods -n $NAMESPACE -l control-plane=controller-manager"
echo "  kubectl logs -n $NAMESPACE deploy/agentflow-manager -f"
echo ""
echo "创建集群版 100 章 Workflow（使用 PVC 存储）:"
echo "  kubectl apply -f config/samples/novel_parallel_demo.yaml"
echo ""
echo "查看 workflow 产出（在 PVC 内）:"
echo "  kubectl exec -n $NAMESPACE deploy/agentflow-manager -- ls /data/outputs/workflows/default/"