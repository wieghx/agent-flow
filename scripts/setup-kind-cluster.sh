#!/usr/bin/env bash
# 创建/修复 kind 集群并满足 Agent Flow 部署前置条件。
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER_NAME:-agent-flow}"
PROVIDER="${KIND_EXPERIMENTAL_PROVIDER:-podman}"
RECREATE="${RECREATE:-false}"

need_inotify=3
inotify_used=$(find /proc/*/fd -lname 'anon_inode:inotify' 2>/dev/null | wc -l | tr -d ' ')
inotify_max=$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo 128)

echo "inotify: ${inotify_used}/${inotify_max} instances"
if [ "$inotify_used" -ge "$((inotify_max - need_inotify))" ]; then
  echo "警告: inotify 实例接近上限，kube-proxy 可能无法启动。"
  echo "请执行（需 root）:"
  echo "  sudo sysctl -w fs.inotify.max_user_instances=512"
  echo "  sudo sysctl -w fs.inotify.max_user_watches=524288"
  echo "或关闭部分文件监视较多的桌面应用后重试。"
fi

# kind 会把主机代理环境变量注入系统 Pod，导致 kube-proxy 异常。
clean_env() {
  env -u HTTP_PROXY -u HTTPS_PROXY -u FTP_PROXY -u ALL_PROXY \
      -u http_proxy -u https_proxy -u ftp_proxy -u all_proxy \
      -u NO_PROXY -u no_proxy "$@"
}

if [ "$RECREATE" = true ] && kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "删除已有集群 $CLUSTER_NAME ..."
  clean_env kind delete cluster --name "$CLUSTER_NAME"
fi

if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "创建 kind 集群 $CLUSTER_NAME (provider=$PROVIDER) ..."
  clean_env env "KIND_EXPERIMENTAL_PROVIDER=$PROVIDER" kind create cluster --name "$CLUSTER_NAME"
fi

echo "等待 kube-system 就绪..."
kubectl wait --namespace kube-system \
  --for=condition=ready pod \
  --selector=k8s-app=kube-proxy \
  --timeout=120s

# 移除 kube-proxy 上的代理环境变量（若 kind 已注入）
if kubectl get ds -n kube-system kube-proxy &>/dev/null; then
  kubectl get ds -n kube-system kube-proxy -o json | python3 - <<'PY' | kubectl replace -f - >/dev/null
import json, sys
proxy = {"HTTP_PROXY","HTTPS_PROXY","FTP_PROXY","NO_PROXY","http_proxy","https_proxy","ftp_proxy","no_proxy"}
d = json.load(sys.stdin)
c = d["spec"]["template"]["spec"]["containers"][0]
c["env"] = [e for e in c.get("env", []) if e.get("name") not in proxy]
json.dump(d, sys.stdout)
PY
  kubectl delete pod -n kube-system -l k8s-app=kube-proxy --ignore-not-found
  kubectl wait --namespace kube-system \
    --for=condition=ready pod \
    --selector=k8s-app=kube-proxy \
    --timeout=120s
fi

echo "加载镜像到 kind 节点..."
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export KIND_EXPERIMENTAL_PROVIDER="$PROVIDER"
for img in \
  kind-local:5000/minagflow/agent-flow-planner:latest \
  kind-local:5000/minagflow/agent-flow-web:latest \
  kind-local:5000/minagflow/mcp-sidecar:latest \
  kind-local:5000/minagflow/worker-agent:latest \
  registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.5.0 \
  redis:7-alpine; do
  echo "  $img"
  kind load docker-image "$img" --name "$CLUSTER_NAME" 2>/dev/null || true
done

echo "kind 集群 $CLUSTER_NAME 已就绪。"