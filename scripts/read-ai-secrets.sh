#!/usr/bin/env bash
# Reads AI credentials from env or config/ai_config.local.yaml (never committed).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_CFG="${ROOT}/config/ai_config.local.yaml"

read_from_local() {
  local field="$1"
  [ -f "$LOCAL_CFG" ] || return 0
  python3 - "$field" "$LOCAL_CFG" <<'PY'
import sys, yaml
field = sys.argv[1]
path = sys.argv[2]
with open(path) as f:
    data = yaml.safe_load(f) or {}

def pick(d, *keys):
    cur = d
    for k in keys:
        if not isinstance(cur, dict):
            return ""
        cur = cur.get(k) or {}
    return cur if isinstance(cur, str) else ""

roles = {
    "api_key": "planner",
    "base_url": "planner",
    "worker_api_key": "worker",
    "worker_base_url": "worker",
}
role = roles.get(field)
if role:
    v = pick(data, role, "remote", field.removeprefix("worker_"))
    if v:
        print(v)
PY
}

export AI_API_KEY="${AI_API_KEY:-$(read_from_local api_key)}"
export AI_BASE_URL="${AI_BASE_URL:-$(read_from_local base_url)}"
export WORKER_AI_API_KEY="${WORKER_AI_API_KEY:-$(read_from_local worker_api_key)}"
export WORKER_AI_BASE_URL="${WORKER_AI_BASE_URL:-$(read_from_local worker_base_url)}"