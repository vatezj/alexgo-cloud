#!/usr/bin/env bash
# 重新生成 docs/sql/alexgo-cloud-full.sql —— 全模块 .up.sql 按执行顺序合并。
# 顺序：system → system/infra → oauth2 → member → order（与 pkg/migrate 各 source 独立执行兼容；
#       system 在最前是因为其内部存在链式依赖，如 20260408000004 ALTER users → 20261008000001 RENAME）。
set -euo pipefail
cd "$(dirname "$0")/.."

OUT=docs/sql/alexgo-cloud-full.sql
mkdir -p docs/sql

{
  echo "-- ============================================================"
  echo "-- alexgo-cloud 全量建库 SQL（schema + 迁移内种子数据）"
  echo "-- 本文件由 scripts/gen-full-sql.sh 生成，请勿手工编辑。"
  echo "-- 用途: 全新环境一键建库。已有环境请勿重复执行（ALTER 非幂等），"
  echo "--       走应用内 migrate（pkg/migrate，按模块 schema_migrations_* 记录版本）。"
  echo "-- 注意: 默认管理员/角色/系统管理菜单基线由 modules/system/service/seed.go"
  echo "--       应用启动时补种，不在本文件内。"
  echo "-- ============================================================"
  echo
  for f in $(find modules/system/migrations -name '*.up.sql' | sort) \
           $(find modules/system/infra/migrations -name '*.up.sql' | sort) \
           $(find modules/system/oauth2/migrations -name '*.up.sql' | sort) \
           $(find modules/member/migrations -name '*.up.sql' | sort) \
           $(find modules/order/migrations -name '*.up.sql' | sort); do
    echo "-- >>> [$(basename "$(dirname "$(dirname "$f")")")/$(basename "$(dirname "$f")")] $(basename "$f")"
    echo
    cat "$f"
    echo
  done
} > "$OUT"

echo "generated: $OUT ($(grep -c -- '>>>' "$OUT") migrations)"
