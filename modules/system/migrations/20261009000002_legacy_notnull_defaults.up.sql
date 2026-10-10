-- 混合历史库遗留 NOT NULL 无默认列补默认值（R3 裁定，T14 断言② / T20 创建流同根因）：
-- * system_users.password / salt：旧结构导入列，Go 模型只有 password_hash（无 Password/Salt 字段），
--   gorm INSERT 不写这两列 → MySQL 严格模式 Error 1364（"Field 'password' doesn't have a default value"）。
-- * system_configs.type / title：同为导入库遗留列，model.SystemConfig 无 type/title 字段（fresh 迁移
--   20260408000006 定义里也没有这两列），createConfig 会撞同一面墙。
-- 列存在才 MODIFY（加 DEFAULT ''，不改存量行数据）；fresh 链路无这些列 → DO 0 全程 no-op。
-- 惯例照 20261008000002_system_users_columns.up.sql 的条件 PREPARE 写法，此处门控从 NOT EXISTS 翻为 EXISTS。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'password'),
  'ALTER TABLE `system_users` MODIFY COLUMN `password` varchar(128) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'salt'),
  'ALTER TABLE `system_users` MODIFY COLUMN `salt` varchar(32) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'type'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `type` varchar(32) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'title'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `title` varchar(128) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
