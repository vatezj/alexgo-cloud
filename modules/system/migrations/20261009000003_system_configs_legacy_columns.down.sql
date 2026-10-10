-- 回滚 = 把 up 加的两列从遗留表上删掉（对称条件 DROP）；**仅针对遗留结构，fresh 库无操作**。
-- 门控用遗留谱系标记列 `is_system`（fresh 库 20260408000006 建表无此列）：
--   * 遗留表（含 is_system）→ 列在则 DROP，还原 R5 之前的故障态（与 20261009000002 down 的语义一致）；
--   * fresh 表（无 is_system，列由建表迁移拥有）→ DO 0，绝不误删建表迁移定义的列。
-- remark/tenant_id 在两谱系上定义完全相同（varchar(255) NOT NULL DEFAULT '' / bigint unsigned NOT NULL DEFAULT 0），
-- 无法按列特征区分归属，故以表谱系标记列作门控。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'is_system')
  AND EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'remark'),
  'ALTER TABLE `system_configs` DROP COLUMN `remark`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'is_system')
  AND EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'tenant_id'),
  'ALTER TABLE `system_configs` DROP COLUMN `tenant_id`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
