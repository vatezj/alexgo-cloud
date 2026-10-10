-- 混合历史库遗留 system_configs 缺列补齐（R5 裁定，T24 全页巡检撞 Error 1054 Unknown column 'tenant_id'）：
-- * 遗留结构（Ruoyi 谱系，含 type/title/sort_order 等导入列）无 tenant_id/remark：
--   repository configRepo.List 的 WHERE tenant_id 与 gorm INSERT 的 remark 均报 Unknown column
--   → GET/POST /api/admin/system/configs 500。
-- * fresh 库 20260408000006_basic_admin 建表已含两列 → NOT EXISTS 门控全程 no-op（仅针对遗留结构）。
-- 惯例照 20261009000002_legacy_notnull_defaults 的条件 PREPARE 写法，此处门控为 NOT EXISTS（缺则加）。
SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'remark'),
  'ALTER TABLE `system_configs` ADD COLUMN `remark` varchar(255) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'tenant_id'),
  'ALTER TABLE `system_configs` ADD COLUMN `tenant_id` bigint unsigned NOT NULL DEFAULT 0',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
