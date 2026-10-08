CREATE TABLE IF NOT EXISTS `outbox_events` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `aggregate_id` varchar(128) NOT NULL,
  `event_type` varchar(128) NOT NULL,
  `payload` json NOT NULL,
  `status` varchar(32) NOT NULL,
  `created_at` datetime NOT NULL,
  `published_at` datetime DEFAULT NULL,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_outbox_status_id` (`status`,`id`),
  KEY `idx_outbox_tenant_id` (`tenant_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

