CREATE TABLE IF NOT EXISTS `tenants` (
  `id`            BIGINT      NOT NULL AUTO_INCREMENT,
  `name`          VARCHAR(64) NOT NULL COMMENT '租户名称',
  `package_id`    BIGINT      NOT NULL DEFAULT 0  COMMENT '租户套餐编号（预留）',
  `status`        TINYINT     NOT NULL DEFAULT 1  COMMENT '状态 1启用 0停用',
  `expire_time`   DATETIME    NULL COMMENT '过期时间（NULL=永久）',
  `account_limit` INT         NOT NULL DEFAULT -1 COMMENT '账号额度 -1不限',
  `domain`        VARCHAR(64) NOT NULL DEFAULT '' COMMENT '绑定域名（登录解析用，可空）',
  `deleted`       TINYINT(1)  NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64) NOT NULL DEFAULT '',
  `create_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64) NOT NULL DEFAULT '',
  `update_time`   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_domain` (`domain`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='租户表';
