#!/usr/bin/env bash
# 打开集群 Web UI（kubectl port-forward）
set -euo pipefail

NAMESPACE="${NAMESPACE:-agent-flow-system}"
SVC="${WEB_SVC:-agentflow-web}"
LOCAL_PORT="${LOCAL_PORT:-3000}"

if ! kubectl get svc "$SVC" -n "$NAMESPACE" &>/dev/null; then
  echo "错误: 未找到 Service $NAMESPACE/$SVC"
  echo "请先部署: make deploy-web 或 ./deploy.sh"
  exit 1
fi

if ! kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=web -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q Running; then
  echo "错误: web Pod 未在运行"
  kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=web
  exit 1
fi

echo "Agent Flow Web UI"
echo "  集群 Service: $NAMESPACE/$SVC"
echo "  本地地址:     http://localhost:${LOCAL_PORT}"
echo ""
echo "说明: kind 集群 LoadBalancer 无外部 IP，必须用 port-forward。"
echo "按 Ctrl+C 停止转发。"
echo ""

# 勿用 pkill port-forward：会误杀本脚本启动的进程
exec kubectl port-forward -n "$NAMESPACE" "svc/${SVC}" "${LOCAL_PORT}:80"