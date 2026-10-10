-- 先加常规列（全部 AFTER 引用本迁移内已存在的列或表自带列），
-- 条件列随后补：fresh 链路 post_ids/sex 已由下方 ALTER 建好，AFTER 引用才成立。
-- [2026-10-09 回写] 本文件曾于 2026-10-09 由原无条件形态改写为条件 PREPARE（双谱系兼容，见提交 6d91959）；版本号未变，已跑环境不重放。
ALTER TABLE `system_users`
  ADD COLUMN `remark`     VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `nickname`,
  ADD COLUMN `dept_id`    BIGINT       NOT NULL DEFAULT 0  COMMENT '部门ID' AFTER `remark`,
  ADD COLUMN `post_ids`   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '岗位ID数组' AFTER `dept_id`,
  ADD COLUMN `mobile`     VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '手机号' AFTER `post_ids`,
  ADD COLUMN `sex`        TINYINT      NOT NULL DEFAULT 0  COMMENT '性别 0未知 1男 2女' AFTER `mobile`,
  ADD COLUMN `login_ip`   VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP' AFTER `status`,
  ADD COLUMN `login_date` DATETIME     NULL COMMENT '最近登录时间' AFTER `login_ip`,
  ADD COLUMN `deleted`    TINYINT(1)   NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删，仅落列）' AFTER `login_date`,
  ADD COLUMN `creator`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

-- 混合历史库（重命名被 20261008000001 跳过）下需补齐的列，存在即跳过：
-- * email/avatar：导入库已带这两列（且有数据），直接并入上方 ALTER 会报 ERROR 1060 重复列。
-- * tenant_id：常规链路由 20260408000004 加在旧 users 上、随重命名带过来；
--   重命名跳过时 system_users 缺该列，模型 TenantID 查询会打空列报错。
SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'email'),
  'ALTER TABLE `system_users` ADD COLUMN `email` varchar(50) NOT NULL DEFAULT '''' COMMENT ''邮箱'' AFTER `post_ids`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'avatar'),
  'ALTER TABLE `system_users` ADD COLUMN `avatar` varchar(100) NOT NULL DEFAULT '''' COMMENT ''头像'' AFTER `sex`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'tenant_id'),
  'ALTER TABLE `system_users` ADD COLUMN `tenant_id` bigint unsigned NOT NULL DEFAULT 0 AFTER `status`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
