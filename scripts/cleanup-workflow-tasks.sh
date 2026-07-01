#!/usr/bin/env bash
# 清理 Workflow 关联的残留 Task：已完成步骤、失败、卡住的 Sandbox
set -euo pipefail

WORKFLOW="${1:-novel-parallel-demo}"
NAMESPACE="${2:-default}"

echo "==> 清理 Workflow 任务残留: $WORKFLOW (ns=$NAMESPACE)"

completed=$(kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o jsonpath='{.status.completedSteps}' 2>/dev/null | python3 -c "
import json,sys
raw=sys.stdin.read().strip()
if not raw:
    print('')
    sys.exit(0)
steps=json.loads(raw)
print(' '.join(s for s in steps if s.startswith('chapter-') or s.startswith('arc-')))
" 2>/dev/null || echo "")

deleted=0
for step in $completed; do
  task="wf-${WORKFLOW}-${step}"
  if kubectl get task "$task" -n "$NAMESPACE" &>/dev/null; then
    kubectl delete task "$task" -n "$NAMESPACE" --wait=false
    echo "    删除已完成残留: $task"
    deleted=$((deleted+1))
  fi
done

echo "==> 删除 Failed Task"
kubectl delete tasks -n "$NAMESPACE" -l "agentflow.io/workflow=$WORKFLOW" \
  --field-selector=status.phase=Failed --ignore-not-found 2>/dev/null || true

echo "==> 删除关联 Sandbox（如有）"
kubectl delete sandboxes.agents.x-k8s.io -n "$NAMESPACE" -l "agentflow.io/workflow=$WORKFLOW" \
  --ignore-not-found 2>/dev/null || true

echo "==> 确保 Workflow 处于 Running"
status_file=$(mktemp)
kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o json | python3 -c "
import json,sys
w=json.load(sys.stdin)
w['status']['phase']='Running'
w['status']['message']='cleaned and resumed'
w['status'].pop('completionTime', None)
json.dump({'status': w['status']}, open('$status_file','w'))
"
kubectl patch workflow "$WORKFLOW" -n "$NAMESPACE" --subresource=status --type=merge --patch-file="$status_file"
rm -f "$status_file"

echo "==> 触发 Workflow 同步（修正步骤状态、避免陈旧 Task 事件）"
kubectl annotate workflow "$WORKFLOW" -n "$NAMESPACE" \
  "agentflow.io/last-cleanup=$(date -u +%Y-%m-%dT%H:%M:%SZ)" --overwrite >/dev/null

echo "==> 清理完成，删除已完成残留 $deleted 个"
echo "    说明: manager 日志中短暂的「task not found」是删除 Task 后队列排空，属正常现象。"
kubectl get workflow "$WORKFLOW" -n "$NAMESPACE" -o wide
kubectl get tasks -n "$NAMESPACE" -l "agentflow.io/workflow=$WORKFLOW" 2>/dev/null | head -20