-- 回滚 = 把四列恢复到加默认值之前的定义（NOT NULL、无 DEFAULT），条件门控与 up 对称（列存在才 MODIFY）。
-- 安全性：MODIFY 仅改列定义、不重写行数据；列恒为 NOT NULL（不存在 NULL 行）、类型/字符集不变
-- （varchar(N) 未显式指定 charset/collation → 沿用表默认 utf8mb4_unicode_ci，与现列一致），
-- 存量行（含 up 之后经默认值写入的 '' 行）均非 NULL，不会触发 1264/1265。
-- 注意：回滚即还原 R3 之前的故障态——省略这些列的 INSERT 会再次报 Error 1364（这正是"回滚到原状"的语义）。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'password'),
  'ALTER TABLE `system_users` MODIFY COLUMN `password` varchar(128) NOT NULL',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'salt'),
  'ALTER TABLE `system_users` MODIFY COLUMN `salt` varchar(32) NOT NULL',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'type'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `type` varchar(32) NOT NULL',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'title'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `title` varchar(128) NOT NULL',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
