#!/usr/bin/env bash
# 业务菜单（scorecard/fishgame）停用/恢复 —— 截图前后用。
# 子命令: backup | hide | restore | verify-hidden | verify-restored
# 状态备份到 .menu_backup.tsv（不入库）；已有备份时 hide 不覆盖。
set -euo pipefail

cd "$(dirname "$0")"
BACKUP=".menu_backup.tsv"
MYSQL_CMD=${MYSQL_CMD:-"mysql -uroot ${MYSQL_PWD:+-p$MYSQL_PWD} alexgo"}
WHERE="deleted=0 AND (path LIKE '/scorecard%' OR path LIKE '/fishgame%')"

run_sql() { $MYSQL_CMD -N -B -e "$1"; }

do_backup() {
  if [ -f "$BACKUP" ]; then
    echo "backup exists, keep: $BACKUP"
    return 0
  fi
  run_sql "SELECT id, path, status FROM menus WHERE $WHERE ORDER BY id;" > "$BACKUP"
  echo "backed up $(wc -l < "$BACKUP") rows -> $BACKUP"
}

do_hide() {
  do_backup
  run_sql "UPDATE menus SET status=0 WHERE $WHERE;"
  echo "hidden"
}

do_restore() {
  if [ ! -f "$BACKUP" ]; then
    echo "ERROR: $BACKUP not found (run backup/hide first)" >&2
    exit 1
  fi
  while IFS=$'\t' read -r id path status; do
    run_sql "UPDATE menus SET status=$status WHERE id=$id;"
  done < "$BACKUP"
  # TSV 保留给 verify-restored 比对；校验通过后由其删除
  echo "restored (run verify-restored next)"
}

do_verify_hidden() {
  local n
  n=$(run_sql "SELECT COUNT(*) FROM menus WHERE $WHERE AND status<>0;")
  if [ "$n" != "0" ]; then
    echo "FAIL: $n rows not hidden" >&2
    run_sql "SELECT id,path,status FROM menus WHERE $WHERE AND status<>0;" >&2
    exit 1
  fi
  echo "verify-hidden OK"
}

do_verify_restored() {
  if [ ! -f "$BACKUP" ]; then
    echo "ERROR: $BACKUP not found (verify before restore deletes it)" >&2
    exit 1
  fi
  local fail=0
  while IFS=$'\t' read -r id path status; do
    local now
    now=$(run_sql "SELECT status FROM menus WHERE id=$id;")
    if [ "$now" != "$status" ]; then
      echo "FAIL: id=$id ($path) status=$now want=$status" >&2
      fail=1
    fi
  done < "$BACKUP"
  if [ "$fail" -ne 0 ]; then exit 1; fi
  rm -f "$BACKUP"
  echo "verify-restored OK"
}

case "${1:-}" in
  backup)          do_backup ;;
  hide)            do_hide ;;
  restore)         do_restore ;;
  verify-hidden)   do_verify_hidden ;;
  verify-restored) do_verify_restored ;;
  *) echo "usage: $0 backup|hide|restore|verify-hidden|verify-restored" >&2; exit 2 ;;
esac
