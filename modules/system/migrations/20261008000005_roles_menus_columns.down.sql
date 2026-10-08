ALTER TABLE `roles`
  DROP COLUMN `sort`, DROP COLUMN `data_scope`, DROP COLUMN `data_scope_dept_ids`,
  DROP COLUMN `type`, DROP COLUMN `remark`, DROP COLUMN `deleted`,
  DROP COLUMN `creator`, DROP COLUMN `updater`;

ALTER TABLE `menus`
  DROP COLUMN `deleted`, DROP COLUMN `creator`, DROP COLUMN `updater`;
