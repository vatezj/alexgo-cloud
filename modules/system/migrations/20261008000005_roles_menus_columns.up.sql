ALTER TABLE `roles`
  ADD COLUMN `sort`                INT          NOT NULL DEFAULT 0  COMMENT '显示顺序' AFTER `name`,
  ADD COLUMN `data_scope`          TINYINT      NOT NULL DEFAULT 1  COMMENT '数据范围 1全部 2自定义 3本部门 4本部门及以下 5仅本人' AFTER `sort`,
  ADD COLUMN `data_scope_dept_ids` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '自定义数据范围部门ID数组' AFTER `data_scope`,
  ADD COLUMN `type`                TINYINT      NOT NULL DEFAULT 2  COMMENT '角色类型 1系统内置 2自定义' AFTER `data_scope_dept_ids`,
  ADD COLUMN `remark`              VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `type`,
  ADD COLUMN `deleted`             BIT(1)       NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `remark`,
  ADD COLUMN `creator`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

ALTER TABLE `menus`
  ADD COLUMN `deleted` BIT(1)      NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `status`,
  ADD COLUMN `creator` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;
