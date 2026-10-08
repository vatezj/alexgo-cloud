CREATE TABLE IF NOT EXISTS `member_user` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `nickname`    VARCHAR(30)  NOT NULL DEFAULT '' COMMENT '用户昵称',
  `avatar`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT '用户头像',
  `status`      TINYINT      NOT NULL DEFAULT 1  COMMENT '状态 1启用 0停用',
  `mobile`      VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '用户手机号（登录账号）',
  `password`    VARCHAR(100) NOT NULL DEFAULT '' COMMENT '密码 bcrypt，可为空=未设密码',
  `register_ip` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '注册IP',
  `login_ip`    VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP',
  `login_date`  DATETIME     NULL COMMENT '最近登录时间',
  `deleted`     TINYINT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删）',
  `creator`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者',
  `create_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updater`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者',
  `update_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `tenant_id`   BIGINT       NOT NULL DEFAULT 0  COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mobile_tenant` (`mobile`, `tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会员用户表';
