#!/usr/bin/env bash
# Reads AI_API_KEY and AI_BASE_URL from env or config/ai_config.local.yaml (never committed).
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

if field == "api_key":
    for role in ("planner", "worker", "monitor"):
        v = pick(data, role, "remote", "api_key")
        if v:
            print(v)
            break
elif field == "base_url":
    for role in ("planner", "worker", "monitor"):
        v = pick(data, role, "remote", "base_url")
        if v:
            print(v)
            break
PY
}

export AI_API_KEY="${AI_API_KEY:-$(read_from_local api_key)}"
export AI_BASE_URL="${AI_BASE_URL:-$(read_from_local base_url)}"