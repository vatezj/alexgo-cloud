-- codegen 配置表（spec §4）：元数据快照 + 可编辑配置。
-- 系统级功能：无 tenant_id（§1 非目标）、硬删除无 deleted 列（§9.3）。
-- deleted 语义见 §9.4：codegen_column.deprecated 标记消失列，不物理删。

CREATE TABLE IF NOT EXISTS `codegen_table` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `table_name` varchar(128) NOT NULL,
  `table_comment` varchar(255) NOT NULL DEFAULT '',
  `module` varchar(64) NOT NULL DEFAULT '',
  `business_name` varchar(128) NOT NULL DEFAULT '',
  `class_name` varchar(128) NOT NULL DEFAULT '',
  `template_type` tinyint NOT NULL DEFAULT 1,
  `front_type` tinyint NOT NULL DEFAULT 1,
  `parent_table_id` bigint unsigned NOT NULL DEFAULT 0,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_codegen_table_name` (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `codegen_column` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `table_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `type` varchar(64) NOT NULL DEFAULT '',
  `comment` varchar(255) NOT NULL DEFAULT '',
  `go_type` varchar(64) NOT NULL DEFAULT '',
  `json_name` varchar(128) NOT NULL DEFAULT '',
  `is_pk` tinyint(1) NOT NULL DEFAULT 0,
  `auto_increment` tinyint(1) NOT NULL DEFAULT 0,
  `nullable` tinyint(1) NOT NULL DEFAULT 0,
  `html_type` varchar(32) NOT NULL DEFAULT '',
  `list_enable` tinyint(1) NOT NULL DEFAULT 1,
  `form_enable` tinyint(1) NOT NULL DEFAULT 1,
  `query_enable` tinyint(1) NOT NULL DEFAULT 1,
  `query_operation` varchar(16) NOT NULL DEFAULT 'eq',
  `list_required` tinyint(1) NOT NULL DEFAULT 0,
  `form_required` tinyint(1) NOT NULL DEFAULT 0,
  `dict_type` varchar(64) NOT NULL DEFAULT '',
  `sort_order` int NOT NULL DEFAULT 0,
  `deprecated` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_codegen_column_table_name` (`table_id`,`name`),
  KEY `idx_codegen_column_table` (`table_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
