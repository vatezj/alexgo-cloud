-- 公告已读回执（P0-1 通知已读）：每租户每用户对每公告一行。
-- 幂等：CREATE TABLE IF NOT EXISTS 可重复跑。
-- 唯一键 uk(tenant_id,user_id,notice_id) 是幂等写的基础：service 层用
-- gorm clause.OnConflict DoNothing（MySQL 方言译作 INSERT IGNORE），
-- 重复标记不报错、不落重复行。
-- 表结构对齐 20260408000006_basic_admin（id 自增、created_at/updated_at、tenant_id、InnoDB utf8mb4）。
CREATE TABLE IF NOT EXISTS `system_notice_reads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL,
  `notice_id` bigint unsigned NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_reads_tenant_user_notice` (`tenant_id`,`user_id`,`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='公告已读回执';
