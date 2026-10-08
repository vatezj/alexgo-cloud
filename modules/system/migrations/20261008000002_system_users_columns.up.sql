ALTER TABLE `system_users`
  ADD COLUMN `remark`     VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `nickname`,
  ADD COLUMN `dept_id`    BIGINT       NOT NULL DEFAULT 0  COMMENT '部门ID' AFTER `remark`,
  ADD COLUMN `post_ids`   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '岗位ID数组' AFTER `dept_id`,
  ADD COLUMN `email`      VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '邮箱' AFTER `post_ids`,
  ADD COLUMN `mobile`     VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '手机号' AFTER `email`,
  ADD COLUMN `sex`        TINYINT      NOT NULL DEFAULT 0  COMMENT '性别 0未知 1男 2女' AFTER `mobile`,
  ADD COLUMN `avatar`     VARCHAR(100) NOT NULL DEFAULT '' COMMENT '头像' AFTER `sex`,
  ADD COLUMN `login_ip`   VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP' AFTER `status`,
  ADD COLUMN `login_date` DATETIME     NULL COMMENT '最近登录时间' AFTER `login_ip`,
  ADD COLUMN `deleted`    BIT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删，仅落列）' AFTER `login_date`,
  ADD COLUMN `creator`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;
