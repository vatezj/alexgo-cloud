DROP TABLE IF EXISTS `role_menus`;
DROP TABLE IF EXISTS `user_roles`;
DROP TABLE IF EXISTS `menus`;
DROP TABLE IF EXISTS `roles`;
DROP TABLE IF EXISTS `casbin_rule`;

ALTER TABLE `users`
  DROP COLUMN `password_hash`,
  DROP COLUMN `tenant_id`;

