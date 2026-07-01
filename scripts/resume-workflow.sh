#!/usr/bin/env bash
# 恢复 Failed/Paused 的 Workflow：重置状态、清理失败 Task、触发重试
set -euo pipefail

WORKFLOW="${1:-novel-parallel-demo}"
NAMESPACE="${2:-default}"

echo "==> 恢复 Workflow: $WORKFLOW (ns=$NAMESPACE)"

phase=$(kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
if [ -z "$phase" ]; then
  echo "Workflow 不存在: $WORKFLOW"
  exit 1
fi
echo "    当前 phase=$phase"

echo "==> 删除失败/卡住的 Task"
kubectl delete tasks -n "$NAMESPACE" -l "agentflow.io/workflow=$WORKFLOW" \
  --field-selector=status.phase=Failed 2>/dev/null || true

echo "==> 重置 Workflow 状态为 Running"
kubectl patch workflow "$WORKFLOW" -n "$NAMESPACE" --type=merge -p '{
  "status": {
    "phase": "Running",
    "message": "resumed manually",
    "completionTime": null
  }
}' 2>/dev/null || {
  # status subresource may need json patch
  kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o json | python3 -c "
import json,sys
w=json.load(sys.stdin)
w['status']['phase']='Running'
w['status']['message']='resumed manually'
w['status'].pop('completionTime', None)
print(json.dumps({'status': w['status']}))
" | kubectl patch workflow "$WORKFLOW" -n "$NAMESPACE" --type=merge --subresource=status -p @-
}

echo "==> 当前状态"
kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o wide
kubectl get tasks -n "$NAMESPACE" -l "agentflow.io/workflow=$WORKFLOW" 2>/dev/null | head -20