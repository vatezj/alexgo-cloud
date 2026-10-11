-- oauth2 两表（spec §9 D4）：client 注册 + 授权码。
-- oauth2_client 无 tenant_id/deleted（硬删除，租户插件因模型缺字段跳过）。
CREATE TABLE IF NOT EXISTS `oauth2_client` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `client_id` VARCHAR(64) NOT NULL,
  `client_secret` VARCHAR(128) NOT NULL DEFAULT '',
  `client_name` VARCHAR(128) NOT NULL DEFAULT '',
  `website` VARCHAR(255) NOT NULL DEFAULT '',
  `grant_types` VARCHAR(255) NOT NULL DEFAULT '',
  `redirect_uris` TEXT NOT NULL,
  `account_types` VARCHAR(64) NOT NULL DEFAULT '',
  `scopes` VARCHAR(64) NOT NULL DEFAULT 'all',
  `activity` INT NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth2_client_id` (`client_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 授权码表：tenant 列由 Go 字段 OwnerTenantID 映射（列名 tenant_id）。
CREATE TABLE IF NOT EXISTS `oauth2_authorize_code` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(64) NOT NULL,
  `client_id` VARCHAR(64) NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `username` VARCHAR(64) NOT NULL DEFAULT '',
  `account_type` VARCHAR(16) NOT NULL DEFAULT '',
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `redirect_uri` TEXT NOT NULL,
  `scopes` VARCHAR(64) NOT NULL DEFAULT 'all',
  `code_challenge` VARCHAR(128) NOT NULL DEFAULT '',
  `code_challenge_method` VARCHAR(16) NOT NULL DEFAULT '',
  `expires_at` DATETIME(3) NOT NULL,
  `revoked` TINYINT(1) NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_oauth2_code` (`code`),
  KEY `idx_oauth2_client_id` (`client_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
