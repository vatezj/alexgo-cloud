-- 微信第三方登录一期两表：
-- system_social_app 租户级渠道配置（每租户每平台一份；secret 明文存，admin 响应 json:"-" 不回显）；
-- system_member_identity 第三方身份绑定行，(tenant,platform,openid) 唯一。
-- 幂等：CREATE TABLE IF NOT EXISTS，可重复跑。
CREATE TABLE IF NOT EXISTS `system_social_app` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `platform` VARCHAR(32) NOT NULL DEFAULT '',
  `appid` VARCHAR(64) NOT NULL DEFAULT '',
  `secret` VARCHAR(128) NOT NULL DEFAULT '',
  `status` TINYINT NOT NULL DEFAULT 1,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_social_app_tenant_platform` (`tenant_id`, `platform`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `system_member_identity` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `member_user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `platform` VARCHAR(32) NOT NULL DEFAULT '',
  `openid` VARCHAR(128) NOT NULL DEFAULT '',
  `unionid` VARCHAR(128) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_identity_tenant_platform_openid` (`tenant_id`, `platform`, `openid`),
  KEY `idx_identity_tenant_unionid` (`tenant_id`, `unionid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
