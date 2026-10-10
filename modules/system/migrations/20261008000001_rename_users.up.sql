-- 条件重命名：目标表 system_users 已存在（从旧结构库导入的混合历史库）时跳过，
-- 否则 RENAME 因目标已存在报错，迁移被标记 dirty，卡死后续所有迁移。
-- 新库（system_users 不存在、users 存在）时行为与原 RENAME 完全一致。
-- [2026-10-09 回写] 本文件曾于 2026-10-09 由原无条件形态改写为条件 PREPARE（双谱系兼容，见提交 6d91959）；版本号未变，已跑环境不重放。
SET @rename_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'system_users') = 0
  AND
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'users') = 1,
  'RENAME TABLE `users` TO `system_users`',
  'DO 0'
);
PREPARE rename_stmt FROM @rename_sql;
EXECUTE rename_stmt;
DEALLOCATE PREPARE rename_stmt;
