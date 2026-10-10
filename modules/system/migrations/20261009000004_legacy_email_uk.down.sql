-- 回滚 = 重建遗留唯一索引，但设**双重门控**（R8 裁定），任一不满足即 DO 0：
-- ① 遗留签名索引 `idx_system_users_username` 必须存在——fresh 库经 20261008000001 RENAME
--    保留的是 `idx_username`，凭此区分谱系，绝不给 fresh 的建表迁移结构补遗留索引；
-- ② 表内 email 无重复（GROUP BY email HAVING COUNT(*)>1 为空）——唯一索引建在有重复值的表上必失败
--    （含 CreateUser 恒写 '' 累积出的多空串），此时宁可不还原也不让 down 报 1062 污染版本簿记。
-- fresh 库：① 不满足 → DO 0；遗留库若已有重复：② 不满足 → DO 0。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'system_users'
      AND index_name = 'idx_system_users_username')
  AND NOT EXISTS(
    SELECT 1 FROM (
      SELECT `email` FROM `system_users` GROUP BY `email` HAVING COUNT(*) > 1
    ) `dup`),
  'ALTER TABLE `system_users` ADD UNIQUE INDEX `idx_system_users_email` (`email`)',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
