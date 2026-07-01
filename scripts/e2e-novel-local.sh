#!/bin/bash
# 本地 2 章小说 E2E：跳过 Sandbox，Planner 直连远程 AI
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

WORKFLOW_NAME="e2e-novel-2ch"
WORKSPACE="$ROOT/data/outputs/workflows/default/$WORKFLOW_NAME"
TIMEOUT="${E2E_TIMEOUT:-900}"

echo "==> 构建 planner"
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/planner cmd/planner/main.go

echo "==> 清理旧资源"
kubectl delete workflow "$WORKFLOW_NAME" -n default --ignore-not-found
kubectl delete tasks -n default -l "agentflow.io/workflow=$WORKFLOW_NAME" --ignore-not-found
kubectl delete sandboxes.agents.x-k8s.io -n default -l "agentflow.io/workflow=$WORKFLOW_NAME" --ignore-not-found 2>/dev/null || true
rm -rf "$WORKSPACE"

echo "==> 启动 planner（AGENTFLOW_SKIP_SANDBOX=true）"
pkill -f 'bin/planner' 2>/dev/null || true
sleep 1
AGENTFLOW_SKIP_SANDBOX=true \
AGENTFLOW_OUTPUT_DIR="$ROOT/data/outputs" \
MCP_IMAGE="kind-local:5000/minagflow/mcp-sidecar:latest" \
WORKER_AGENT_IMAGE="kind-local:5000/minagflow/worker-agent:latest" \
nohup ./bin/planner --leader-elect=false --ai-config=config/ai_config.yaml --health-probe-bind-address=:8083 \
  > /tmp/agentflow-planner-e2e.log 2>&1 &
PLANNER_PID=$!
sleep 3
if ! kill -0 "$PLANNER_PID" 2>/dev/null; then
  echo "planner 启动失败，日志："
  tail -30 /tmp/agentflow-planner-e2e.log
  exit 1
fi
echo "planner PID: $PLANNER_PID"

echo "==> 创建 2 章 Workflow"
sed -e "s/deploy-test-novel/$WORKFLOW_NAME/g" \
  -e "s|__WORKSPACE_ROOT__|$ROOT/data/outputs/workflows|g" \
  config/samples/deploy_test_workflow.yaml | kubectl apply -f -

echo "==> 等待 Workflow 完成（超时 ${TIMEOUT}s）"
deadline=$((SECONDS + TIMEOUT))
last_phase=""
while [ $SECONDS -lt $deadline ]; do
  phase=$(kubectl get workflow "$WORKFLOW_NAME" -n default -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
  step=$(kubectl get workflow "$WORKFLOW_NAME" -n default -o jsonpath='{.status.currentStep}' 2>/dev/null || echo "")
  progress=$(kubectl get workflow "$WORKFLOW_NAME" -n default -o jsonpath='{.status.progress.percent}' 2>/dev/null || echo "0")
  if [ "$phase" != "$last_phase" ] || [ -n "$step" ]; then
    echo "  phase=$phase step=$step progress=${progress}%"
    last_phase="$phase"
  fi
  if [ "$phase" = "Succeeded" ]; then
    break
  fi
  if [ "$phase" = "Failed" ] || [ "$phase" = "Paused" ]; then
    echo "Workflow 异常结束: phase=$phase"
    kubectl get workflow "$WORKFLOW_NAME" -n default -o yaml | tail -30
    kubectl get tasks -n default -l "agentflow.io/workflow=$WORKFLOW_NAME" -o wide 2>/dev/null || true
    exit 1
  fi
  sleep 10
done

phase=$(kubectl get workflow "$WORKFLOW_NAME" -n default -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
if [ "$phase" != "Succeeded" ]; then
  echo "超时：Workflow 未达到 Succeeded"
  kubectl get workflow,tasks -n default 2>/dev/null || true
  tail -40 /tmp/agentflow-planner-e2e.log
  exit 1
fi

echo "==> 验证产出物"
missing=0
for f in outline.json chapters/chapter-01.md chapters/chapter-02.md book.md; do
  if [ -f "$WORKSPACE/$f" ]; then
    echo "  OK $f ($(wc -c < "$WORKSPACE/$f") bytes)"
  else
    echo "  MISSING $f"
    missing=1
  fi
done

if [ "$missing" -ne 0 ]; then
  ls -laR "$WORKSPACE" 2>/dev/null || true
  exit 1
fi

echo ""
echo "========================================="
echo "  E2E 通过：2 章小说已生成"
echo "  书稿: $WORKSPACE/book.md"
echo "========================================="
head -40 "$WORKSPACE/book.md"