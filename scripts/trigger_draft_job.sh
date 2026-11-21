#!/bin/bash
# 服务器一键触发次日软文草稿（upload_draft）+ 回读任务状态——无需 Go/源码，走运行中服务的 admin API。
#
# 用法（在服务器上）：
#   1) 浏览器登录公众号后台对应的服务管理台 → DevTools → Application → localStorage 复制 token
#   2) ADMIN_TOKEN=eyJxxxx ./scripts/trigger_draft_job.sh [BASE_URL]
#      BASE_URL 缺省 http://127.0.0.1:9090
#
# 输出即诊断：触发响应 + starfuyu.upload_draft 的 lastEnd/lastErr/runCount。
# lastErr 非空 → 把原文贴给开发者；为空且草稿入箱 → 全链路通。
set -euo pipefail

BASE="${1:-${BASE_URL:-http://127.0.0.1:9090}}"
: "${ADMIN_TOKEN:?请设置 ADMIN_TOKEN（管理台浏览器 localStorage 的 token）}"
NAME="starfuyu.upload_draft"

echo "== 1) 触发 $NAME（同步执行，约 40-60 秒：抓取+封面+上传） =="
code=$(curl -sS --max-time 150 -o /tmp/job_trigger_resp.json -w '%{http_code}' \
  -X POST "$BASE/api/admin/jobs/$NAME/trigger" \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" || true)
echo "HTTP $code: $(cat /tmp/job_trigger_resp.json 2>/dev/null)"

echo
echo "== 2) 任务状态回读 =="
curl -sS --max-time 20 "$BASE/api/admin/jobs" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception as e:
    print("解析失败:", e); sys.exit(0)
items = d.get("data") or d.get("list") or []
if isinstance(items, dict):
    items = items.get("list") or items.get("items") or []
hit = [j for j in items if j.get("name") == "starfuyu.upload_draft"]
if not hit:
    print("⚠️ 列表里没有 starfuyu.upload_draft——部署的二进制没有此任务（需重新构建部署）")
else:
    j = hit[0]
    print(f"name={j[\"name\"]} enabled={j.get(\"enabled\")} spec={j.get(\"spec\")}")
    print(f"lastEnd={j.get(\"lastEnd\")} runCount={j.get(\"runCount\")}")
    print(f"lastErr={j.get(\"lastErr\") or \"（空=上次执行成功）\"}")
    print(f"nextRun={j.get(\"nextRun\")}")
'
echo
echo "== 3) 提示 =="
echo "lastErr 为空 → 去公众号后台草稿箱确认明日日期的稿件；非空 → 错误词典见文档/贴给开发者。"
