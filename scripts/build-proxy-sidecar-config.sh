#!/usr/bin/env bash
# Build minimal sing-box sidecar config from local v2rayN (never committed).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
V2RAYN_CONFIG="${V2RAYN_CONFIG:-$HOME/.local/share/v2rayN/binConfigs/config.json}"
OUT="${ROOT}/config/proxy-sidecar.local.json"

if [ ! -f "$V2RAYN_CONFIG" ]; then
  echo "错误: 未找到 v2rayN 配置: $V2RAYN_CONFIG" >&2
  exit 1
fi

export V2RAYN_CONFIG OUT
python3 <<'PY'
import copy, json, os, sys

src = os.environ["V2RAYN_CONFIG"]
out = os.environ["OUT"]

with open(src, encoding="utf-8") as f:
    full = json.load(f)

outbounds = full.get("outbounds", [])
proxy_ob = next((o for o in outbounds if o.get("tag") == "proxy"), None)
if proxy_ob is None:
    proxy_ob = next((o for o in outbounds if o.get("type") != "direct"), None)
if proxy_ob is None:
    sys.exit("v2rayN 配置中未找到 proxy outbound")

cfg = {
    "log": {"level": "warn", "timestamp": True},
    "inbounds": [
        {
            "type": "mixed",
            "tag": "mixed-in",
            "listen": "0.0.0.0",
            "listen_port": 10810,
        }
    ],
    "outbounds": [copy.deepcopy(proxy_ob), {"type": "direct", "tag": "direct"}],
    "route": {
        "rules": [
            {"domain_suffix": ["deepseek.com"], "outbound": "direct"},
            {"ip_is_private": True, "outbound": "direct"},
        ],
        "final": "proxy",
    },
}

os.makedirs(os.path.dirname(out), exist_ok=True)
with open(out, "w", encoding="utf-8") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
print(f"已生成 sidecar 配置: {out}")
PY