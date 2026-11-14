#!/usr/bin/env bash
# menu_tools.sh 静态检查（不依赖数据库）
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

if ! bash -n menu_tools.sh; then
  echo "FAIL: bash -n 语法错误"; fail=1
fi

for kw in scorecard fishgame "status=0" verify-hidden verify-restored; do
  if ! grep -q "$kw" menu_tools.sh; then
    echo "FAIL: menu_tools.sh 缺少关键字 $kw"; fail=1
  fi
done

if ! grep -q "menu_backup.tsv" ../.gitignore 2>/dev/null && ! grep -q "menu_backup.tsv" .gitignore 2>/dev/null; then
  echo "FAIL: .gitignore 未覆盖 menu_backup.tsv"; fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "PASS: menu_tools.sh static checks"
fi
exit "$fail"
