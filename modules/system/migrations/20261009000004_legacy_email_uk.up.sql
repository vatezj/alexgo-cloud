-- 混合历史库遗留 system_users.email 唯一索引下线（R8 裁定，T26 总验收 用户页新建 撞 Error 1062）：
-- * 遗留导入库自带 UNIQUE `idx_system_users_email`（全仓迁移均未创建——20261008000002 只补 email 列不建索引），
--   而 baseline CreateUser 接口不收 email、INSERT 恒写 '' → 第二条空邮箱用户即
--   `Error 1062 (23000): Duplicate entry '' for key 'idx_system_users_email'`
--   → POST /api/admin/system/users 500（vben 用户页/旧 admin-web 同一 API，同样中招）。
-- * fresh 库 20250408000001_init_system 建表只带 `idx_username`，无此索引 → EXISTS 门控全程 no-op。
-- 惯例照 20261009000002/000003 的条件 PREPARE 写法，此处门控为「唯一签名的该索引存在才 DROP」。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'system_users'
      AND index_name = 'idx_system_users_email' AND non_unique = 0),
  'ALTER TABLE `system_users` DROP INDEX `idx_system_users_email`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
