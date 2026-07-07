#!/usr/bin/env bash
# Raise inotify limits required by kind + controller-runtime (agent-sandbox, kube-proxy).
set -euo pipefail

need_instances=512
need_watches=524288

used=$(find /proc/*/fd -lname 'anon_inode:inotify' 2>/dev/null | wc -l | tr -d ' ')
max_inst=$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo 128)
max_watch=$(sysctl -n fs.inotify.max_user_watches 2>/dev/null || echo 0)

echo "inotify instances: ${used}/${max_inst} (recommended >= ${need_instances})"
echo "inotify watches:   max=${max_watch} (recommended >= ${need_watches})"

if [ "$max_inst" -ge "$need_instances" ] && [ "$max_watch" -ge "$need_watches" ]; then
  echo "Limits already sufficient."
  exit 0
fi

echo ""
echo "需要 root 权限提升 inotify 上限（agent-sandbox-controller CrashLoop 常见原因: too many open files）"
echo "请执行:"
echo "  sudo sysctl -w fs.inotify.max_user_instances=${need_instances}"
echo "  sudo sysctl -w fs.inotify.max_user_watches=${need_watches}"
echo ""
echo "持久化（可选）:"
echo "  sudo tee /etc/sysctl.d/99-agent-flow-inotify.conf >/dev/null <<EOF"
echo "fs.inotify.max_user_instances=${need_instances}"
echo "fs.inotify.max_user_watches=${need_watches}"
echo "EOF"
echo "  sudo sysctl --system"
echo ""
echo "然后重启 sandbox controller:"
echo "  kubectl delete pod -n agent-sandbox-system -l app=agent-sandbox-controller"

if [ "$(id -u)" -eq 0 ]; then
  sysctl -w fs.inotify.max_user_instances="${need_instances}"
  sysctl -w fs.inotify.max_user_watches="${need_watches}"
  echo "Done."
fi