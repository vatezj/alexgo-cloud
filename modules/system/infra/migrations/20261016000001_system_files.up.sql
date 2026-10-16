-- 文件元数据表（子项目 B / P1-1 spec §2）：管理端受控读写带 tenant_id，
-- public 读按 public_key 直取不走过滤（GetByKey，产品语义）。
CREATE TABLE IF NOT EXISTS `system_files` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL DEFAULT '',
  `ext` varchar(32) NOT NULL DEFAULT '',
  `mime` varchar(128) NOT NULL DEFAULT '',
  `size` bigint NOT NULL DEFAULT 0,
  `rel_path` varchar(255) NOT NULL,
  `public_key` varchar(32) NOT NULL,
  `public` tinyint(1) NOT NULL DEFAULT 0,
  `biz_type` varchar(64) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_system_files_rel_path` (`rel_path`),
  UNIQUE KEY `uk_system_files_public_key` (`public_key`),
  KEY `idx_system_files_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
