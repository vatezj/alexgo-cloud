-- ============================================================
-- alexgo-cloud 全量建库 SQL（schema + 迁移内种子数据）
-- 本文件由 scripts/gen-full-sql.sh 生成，请勿手工编辑。
-- 用途: 全新环境一键建库。已有环境请勿重复执行（ALTER 非幂等），
--       走应用内 migrate（pkg/migrate，按模块 schema_migrations_* 记录版本）。
-- 注意: 默认管理员/角色/系统管理菜单基线由 modules/system/service/seed.go
--       应用启动时补种，不在本文件内。
-- ============================================================

-- >>> [system/migrations] 20250408000001_init_system.up.sql

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `username` varchar(64) NOT NULL,
  `nickname` varchar(64) DEFAULT NULL,
  `status` tinyint NOT NULL DEFAULT 1,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- >>> [system/migrations] 20260408000002_outbox.up.sql

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


-- >>> [system/migrations] 20260408000004_rbac.up.sql

ALTER TABLE `users`
  ADD COLUMN `password_hash` varchar(255) NOT NULL DEFAULT '' AFTER `nickname`,
  ADD COLUMN `tenant_id` bigint unsigned NOT NULL DEFAULT 0 AFTER `status`;

CREATE TABLE IF NOT EXISTS `roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  `status` tinyint NOT NULL DEFAULT 1,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_roles_tenant_code` (`tenant_id`,`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `menus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `parent_id` bigint unsigned NOT NULL DEFAULT 0,
  `type` varchar(16) NOT NULL DEFAULT 'menu',
  `name` varchar(64) NOT NULL,
  `path` varchar(255) NOT NULL DEFAULT '',
  `component` varchar(255) NOT NULL DEFAULT '',
  `icon` varchar(64) NOT NULL DEFAULT '',
  `permission` varchar(128) NOT NULL DEFAULT '',
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint NOT NULL DEFAULT 1,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_menus_tenant_parent` (`tenant_id`,`parent_id`),
  KEY `idx_menus_tenant_status_sort` (`tenant_id`,`status`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `user_roles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `role_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_roles_tenant_user_role` (`tenant_id`,`user_id`,`role_id`),
  KEY `idx_user_roles_tenant_user` (`tenant_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `role_menus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_id` bigint unsigned NOT NULL,
  `menu_id` bigint unsigned NOT NULL,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_menus_tenant_role_menu` (`tenant_id`,`role_id`,`menu_id`),
  KEY `idx_role_menus_tenant_role` (`tenant_id`,`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `casbin_rule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ptype` varchar(100) DEFAULT NULL,
  `v0` varchar(100) DEFAULT NULL,
  `v1` varchar(100) DEFAULT NULL,
  `v2` varchar(100) DEFAULT NULL,
  `v3` varchar(100) DEFAULT NULL,
  `v4` varchar(100) DEFAULT NULL,
  `v5` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_casbin_rule` (`ptype`,`v0`,`v1`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- >>> [system/migrations] 20260408000005_audit.up.sql

CREATE TABLE IF NOT EXISTS `login_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `username` varchar(64) NOT NULL DEFAULT '',
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `ip` varchar(64) NOT NULL DEFAULT '',
  `user_agent` varchar(512) NOT NULL DEFAULT '',
  `success` tinyint NOT NULL DEFAULT 0,
  `message` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_login_logs_tenant_created` (`tenant_id`,`created_at`),
  KEY `idx_login_logs_username_created` (`username`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `operate_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `username` varchar(64) NOT NULL DEFAULT '',
  `method` varchar(16) NOT NULL DEFAULT '',
  `path` varchar(255) NOT NULL DEFAULT '',
  `status` int NOT NULL DEFAULT 0,
  `latency_ms` bigint NOT NULL DEFAULT 0,
  `error` varchar(255) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_operate_logs_tenant_created` (`tenant_id`,`created_at`),
  KEY `idx_operate_logs_user_created` (`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- >>> [system/migrations] 20260408000006_basic_admin.up.sql

CREATE TABLE IF NOT EXISTS `depts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `parent_id` bigint unsigned NOT NULL DEFAULT 0,
  `name` varchar(64) NOT NULL,
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint NOT NULL DEFAULT 1,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_depts_tenant_parent` (`tenant_id`,`parent_id`),
  KEY `idx_depts_tenant_sort` (`tenant_id`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint NOT NULL DEFAULT 1,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_posts_tenant_code` (`tenant_id`,`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `dict_types` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `type` varchar(64) NOT NULL,
  `name` varchar(64) NOT NULL,
  `status` tinyint NOT NULL DEFAULT 1,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dict_types_tenant_type` (`tenant_id`,`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `dict_datas` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `type_id` bigint unsigned NOT NULL,
  `label` varchar(64) NOT NULL,
  `value` varchar(64) NOT NULL,
  `sort` int NOT NULL DEFAULT 0,
  `status` tinyint NOT NULL DEFAULT 1,
  `remark` varchar(255) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_dict_datas_tenant_type` (`tenant_id`,`type_id`),
  KEY `idx_dict_datas_tenant_sort` (`tenant_id`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `system_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `key` varchar(128) NOT NULL,
  `value` varchar(512) NOT NULL DEFAULT '',
  `name` varchar(128) NOT NULL DEFAULT '',
  `remark` varchar(255) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_system_configs_tenant_key` (`tenant_id`,`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `system_notices` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `title` varchar(255) NOT NULL,
  `content` text NOT NULL,
  `status` tinyint NOT NULL DEFAULT 1,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_system_notices_tenant_created` (`tenant_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- >>> [system/migrations] 20261008000001_rename_users.up.sql

-- 条件重命名：目标表 system_users 已存在（从旧结构库导入的混合历史库）时跳过，
-- 否则 RENAME 因目标已存在报错，迁移被标记 dirty，卡死后续所有迁移。
-- 新库（system_users 不存在、users 存在）时行为与原 RENAME 完全一致。
-- [2026-10-09 回写] 本文件曾于 2026-10-09 由原无条件形态改写为条件 PREPARE（双谱系兼容，见提交 6d91959）；版本号未变，已跑环境不重放。
SET @rename_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'system_users') = 0
  AND
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'users') = 1,
  'RENAME TABLE `users` TO `system_users`',
  'DO 0'
);
PREPARE rename_stmt FROM @rename_sql;
EXECUTE rename_stmt;
DEALLOCATE PREPARE rename_stmt;

-- >>> [system/migrations] 20261008000002_system_users_columns.up.sql

-- 先加常规列（全部 AFTER 引用本迁移内已存在的列或表自带列），
-- 条件列随后补：fresh 链路 post_ids/sex 已由下方 ALTER 建好，AFTER 引用才成立。
-- [2026-10-09 回写] 本文件曾于 2026-10-09 由原无条件形态改写为条件 PREPARE（双谱系兼容，见提交 6d91959）；版本号未变，已跑环境不重放。
ALTER TABLE `system_users`
  ADD COLUMN `remark`     VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `nickname`,
  ADD COLUMN `dept_id`    BIGINT       NOT NULL DEFAULT 0  COMMENT '部门ID' AFTER `remark`,
  ADD COLUMN `post_ids`   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '岗位ID数组' AFTER `dept_id`,
  ADD COLUMN `mobile`     VARCHAR(11)  NOT NULL DEFAULT '' COMMENT '手机号' AFTER `post_ids`,
  ADD COLUMN `sex`        TINYINT      NOT NULL DEFAULT 0  COMMENT '性别 0未知 1男 2女' AFTER `mobile`,
  ADD COLUMN `login_ip`   VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '最近登录IP' AFTER `status`,
  ADD COLUMN `login_date` DATETIME     NULL COMMENT '最近登录时间' AFTER `login_ip`,
  ADD COLUMN `deleted`    TINYINT(1)   NOT NULL DEFAULT 0  COMMENT '是否删除（一期不启用软删，仅落列）' AFTER `login_date`,
  ADD COLUMN `creator`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

-- 混合历史库（重命名被 20261008000001 跳过）下需补齐的列，存在即跳过：
-- * email/avatar：导入库已带这两列（且有数据），直接并入上方 ALTER 会报 ERROR 1060 重复列。
-- * tenant_id：常规链路由 20260408000004 加在旧 users 上、随重命名带过来；
--   重命名跳过时 system_users 缺该列，模型 TenantID 查询会打空列报错。
SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'email'),
  'ALTER TABLE `system_users` ADD COLUMN `email` varchar(50) NOT NULL DEFAULT '''' COMMENT ''邮箱'' AFTER `post_ids`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'avatar'),
  'ALTER TABLE `system_users` ADD COLUMN `avatar` varchar(100) NOT NULL DEFAULT '''' COMMENT ''头像'' AFTER `sex`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'tenant_id'),
  'ALTER TABLE `system_users` ADD COLUMN `tenant_id` bigint unsigned NOT NULL DEFAULT 0 AFTER `status`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- >>> [system/migrations] 20261008000003_oauth2_access_token.up.sql

CREATE TABLE IF NOT EXISTS `system_oauth2_access_token` (
  `id`            BIGINT       NOT NULL AUTO_INCREMENT,
  `user_id`       BIGINT       NOT NULL COMMENT '用户编号',
  `user_type`     TINYINT      NOT NULL COMMENT '用户类型 1管理员 2会员',
  `access_token`  VARCHAR(255) NOT NULL COMMENT '访问令牌',
  `refresh_token` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '刷新令牌',
  `client_id`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '客户端编号',
  `scopes`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '授权范围',
  `expires_time`  DATETIME     NOT NULL COMMENT '过期时间',
  `deleted`       TINYINT(1)   NOT NULL DEFAULT 0,
  `creator`       VARCHAR(64)  NOT NULL DEFAULT '',
  `create_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updater`       VARCHAR(64)  NOT NULL DEFAULT '',
  `update_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `tenant_id`     BIGINT       NOT NULL DEFAULT 0 COMMENT '租户编号',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_access_token` (`access_token`),
  UNIQUE KEY `uk_refresh_token` (`refresh_token`),
  KEY `idx_user` (`user_id`, `user_type`),
  KEY `idx_expires` (`expires_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OAuth2 访问令牌表';

-- >>> [system/migrations] 20261008000004_tenants.up.sql

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

-- >>> [system/migrations] 20261008000005_roles_menus_columns.up.sql

ALTER TABLE `roles`
  ADD COLUMN `sort`                INT          NOT NULL DEFAULT 0  COMMENT '显示顺序' AFTER `name`,
  ADD COLUMN `data_scope`          TINYINT      NOT NULL DEFAULT 1  COMMENT '数据范围 1全部 2自定义 3本部门 4本部门及以下 5仅本人' AFTER `sort`,
  ADD COLUMN `data_scope_dept_ids` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '自定义数据范围部门ID数组' AFTER `data_scope`,
  ADD COLUMN `type`                TINYINT      NOT NULL DEFAULT 2  COMMENT '角色类型 1系统内置 2自定义' AFTER `data_scope_dept_ids`,
  ADD COLUMN `remark`              VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注' AFTER `type`,
  ADD COLUMN `deleted`             TINYINT(1)   NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `remark`,
  ADD COLUMN `creator`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater`             VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

ALTER TABLE `menus`
  ADD COLUMN `deleted` TINYINT(1)  NOT NULL DEFAULT 0  COMMENT '是否删除' AFTER `status`,
  ADD COLUMN `creator` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建者' AFTER `deleted`,
  ADD COLUMN `updater` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '更新者' AFTER `creator`;

-- >>> [system/migrations] 20261009000001_menus_vben_backfill.up.sql

-- vben-admin 菜单回填：工作台/订单 2 目录 + 3 子页 + admin 角色绑定 + 11 处 icon 换新。
-- 幂等：全部 INSERT 走 NOT EXISTS，UPDATE 按旧 icon 精确匹配（重跑命中 0 行）。
-- 过滤必须带 type IN ('dir','menu')：member 按钮 icon='team' 与 roles 菜单 icon='team'
-- 同值，不带 type 会把按钮图标改掉；/system（dir）也在这 11 行内。

-- ① 根目录 2 行
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '工作台', '/dashboard', '', 'lucide:layout-dashboard', '', 1, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard' AND `type`='dir' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '订单', '/order', '', 'lucide:shopping-cart', 'order', 2, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/order' AND `type`='dir' AND `deleted`=0);

-- ② 子页 3 行（parent 经父目录 JOIN 定位；component 与页面文件路径严格一致）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '分析看板', '/dashboard/analytics', 'views/dashboard/analytics/index', 'lucide:area-chart', '', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/dashboard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard/analytics' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '工作台', '/dashboard/workspace', 'views/dashboard/workspace/index', 'carbon:workspace', '', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/dashboard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/dashboard/workspace' AND `deleted`=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '订单列表', '/order/orders', 'views/order/OrderOrdersPage', 'lucide:list', 'order:order:*', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/order' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/order/orders' AND `deleted`=0);

-- ③ admin 角色绑定 5 新行（JOIN roles 而非标量子查询：fresh 库尚无 admin 角色时产出 0 行，
--    由 seed 尾部 SetRoleMenus 全量兜底；已有库立即绑定 → Casbin 重建可见 order 权限）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND m.`path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);

-- ④ 11 处 icon UPDATE（旧 ant 图标 → iconify；type 过滤见文件头注释）
UPDATE `menus` SET `icon`='lucide:settings'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='settings';
UPDATE `menus` SET `icon`='lucide:user'        WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='user';
UPDATE `menus` SET `icon`='lucide:users'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='team';
UPDATE `menus` SET `icon`='lucide:list-tree'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='menu';
UPDATE `menus` SET `icon`='lucide:building-2'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='apartment';
UPDATE `menus` SET `icon`='lucide:id-card'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='idcard';
UPDATE `menus` SET `icon`='lucide:book-open'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='book';
UPDATE `menus` SET `icon`='lucide:settings-2'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='setting';
UPDATE `menus` SET `icon`='lucide:bell'        WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='bell';
UPDATE `menus` SET `icon`='lucide:history'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='history';
UPDATE `menus` SET `icon`='lucide:file-text'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='profile';

-- >>> [system/migrations] 20261009000002_legacy_notnull_defaults.up.sql

-- 混合历史库遗留 NOT NULL 无默认列补默认值（R3 裁定，T14 断言② / T20 创建流同根因）：
-- * system_users.password / salt：旧结构导入列，Go 模型只有 password_hash（无 Password/Salt 字段），
--   gorm INSERT 不写这两列 → MySQL 严格模式 Error 1364（"Field 'password' doesn't have a default value"）。
-- * system_configs.type / title：同为导入库遗留列，model.SystemConfig 无 type/title 字段（fresh 迁移
--   20260408000006 定义里也没有这两列），createConfig 会撞同一面墙。
-- 列存在才 MODIFY（加 DEFAULT ''，不改存量行数据）；fresh 链路无这些列 → DO 0 全程 no-op。
-- 惯例照 20261008000002_system_users_columns.up.sql 的条件 PREPARE 写法，此处门控从 NOT EXISTS 翻为 EXISTS。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'password'),
  'ALTER TABLE `system_users` MODIFY COLUMN `password` varchar(128) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_users' AND column_name = 'salt'),
  'ALTER TABLE `system_users` MODIFY COLUMN `salt` varchar(32) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'type'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `type` varchar(32) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'title'),
  'ALTER TABLE `system_configs` MODIFY COLUMN `title` varchar(128) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- >>> [system/migrations] 20261009000003_system_configs_legacy_columns.up.sql

-- 混合历史库遗留 system_configs 缺列补齐（R5 裁定，T24 全页巡检撞 Error 1054 Unknown column 'tenant_id'）：
-- * 遗留结构（Ruoyi 谱系，含 type/title/sort_order 等导入列）无 tenant_id/remark：
--   repository configRepo.List 的 WHERE tenant_id 与 gorm INSERT 的 remark 均报 Unknown column
--   → GET/POST /api/admin/system/configs 500。
-- * fresh 库 20260408000006_basic_admin 建表已含两列 → NOT EXISTS 门控全程 no-op（仅针对遗留结构）。
-- 惯例照 20261009000002_legacy_notnull_defaults 的条件 PREPARE 写法，此处门控为 NOT EXISTS（缺则加）。
SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'remark'),
  'ALTER TABLE `system_configs` ADD COLUMN `remark` varchar(255) NOT NULL DEFAULT ''''',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(NOT EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'system_configs' AND column_name = 'tenant_id'),
  'ALTER TABLE `system_configs` ADD COLUMN `tenant_id` bigint unsigned NOT NULL DEFAULT 0',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- >>> [system/migrations] 20261009000004_legacy_email_uk.up.sql

-- 混合历史库遗留 system_users.email 唯一索引下线（R8 裁定，T26 总验收 用户页新建 撞 Error 1062）：
-- * 遗留导入库自带 UNIQUE `idx_system_users_email`（全仓迁移均未创建——20261008000002 只补 email 列不建索引），
--   而 baseline CreateUser 接口不收 email、INSERT 恒写 '' → 第二条空邮箱用户即
--   `Error 1062 (23000): Duplicate entry '' for key 'idx_system_users_email'`
--   → POST /api/admin/system/users 500（vben 用户页/旧 admin-web 同一 API，同样中招）。
-- * fresh 库 20250408000001_init_system 建表只带 `idx_username`，无此索引 → EXISTS 门控全程 no-op。
-- 惯例照 20261009000002/000003 的条件 PREPARE 写法，此处门控为「唯一签名的该索引存在才 DROP」。
SET @sql := IF(EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE() AND table_name = 'system_users'
      AND index_name = 'idx_system_users_email' AND non_unique = 0),
  'ALTER TABLE `system_users` DROP INDEX `idx_system_users_email`',
  'DO 0');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- >>> [system/migrations] 20261011000002_codegen_menus.up.sql

-- codegen 菜单 + 按钮（spec §4 admin 入口）。
-- 幂等：全部 NOT EXISTS 守卫（业务键：目录/页面按 path+type，按钮按 permission），可重复跑。
-- 权限码前缀 infra:codegen 必须已在 permission_routes.go 注册，否则本迁移产生的按钮是死权限（403）。
-- 表结构对齐 20261009000001（menus：type 'dir'/'menu'/'button'、sort、deleted、tenant_id=0）。
-- 父目录定位：系统管理 由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定，
-- 403 闭环不依赖嵌套层级）。

-- 1) 目录「代码生成」（挂在系统管理下；fresh 库父不存在则 parent=0）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'dir', '代码生成', '/codegen', '', 'lucide:code', 'infra', 90, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/codegen' AND `type`='dir' AND `deleted`=0);

-- 2) 页面菜单「代码生成配置」（挂在目录下）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '代码生成配置', '/codegen/config', 'views/infra/codegen/index', 'lucide:file-code', 'infra:codegen', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/codegen' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/codegen/config' AND `deleted`=0);

-- 3) 按钮：导入 / 同步 / 删除（挂在页面菜单下）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '导入' AS `name`, 'infra:codegen:import' AS perm, 1 AS `sort`
  UNION ALL SELECT '同步', 'infra:codegen:sync', 2
  UNION ALL SELECT '删除', 'infra:codegen:delete', 3
) t
WHERE m.`tenant_id`=0 AND m.`permission`='infra:codegen' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 4) admin 角色挂菜单（模式抄 20261009000001 ③：JOIN roles 而非标量子查询——
--    fresh 库尚无 admin 角色时产出 0 行，由 seed 尾部 SetRoleMenus 全量兜底）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path` IN ('/codegen','/codegen/config') OR m.`permission` LIKE 'infra:codegen%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);

-- >>> [system/migrations] 20261011000003_oauth2_client_menu.up.sql

-- OAuth2 客户端管理菜单 + 按钮（spec §4.5 管理端配套：复用 codegen 种子模式，挂系统管理下）。
-- 幂等：全部 NOT EXISTS 守卫（目录按 path+type，页面按 path，按钮按 permission），可重复跑。
-- 权限码前缀 oauth2:client 必须已在 permission_routes.go 注册（Task 13），
-- 否则本迁移产生的页面/按钮是死权限（403）。
-- 表结构对齐 20261009000001（menus：type 'dir'/'menu'/'button'、sort、deleted、tenant_id=0）。
-- 父目录定位：系统管理 由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定，
-- 403 闭环不依赖嵌套层级）。模式抄 20261011000002 codegen_menus。

-- 1) 目录「OAuth2」（挂在系统管理下；fresh 库父不存在则 parent=0）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'dir', 'OAuth2', '/oauth2', '', 'lucide:shield-check', 'oauth2', 91, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/oauth2' AND `type`='dir' AND `deleted`=0);

-- 2) 页面菜单「客户端管理」（挂在目录下；component 由 access.ts glob 自动解析，零路由改动）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '客户端管理', '/oauth2/clients', 'views/system/Oauth2ClientsPage', 'lucide:key-round', 'oauth2:client', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/oauth2' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/oauth2/clients' AND `deleted`=0);

-- 3) 按钮：新建 / 编辑 / 删除（挂在页面菜单下；前端 v-access 逐字匹配这些码）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '新建' AS `name`, 'oauth2:client:create' AS perm, 1 AS `sort`
  UNION ALL SELECT '编辑', 'oauth2:client:update', 2
  UNION ALL SELECT '删除', 'oauth2:client:delete', 3
) t
WHERE m.`tenant_id`=0 AND m.`permission`='oauth2:client' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 4) admin 角色挂菜单（抄 codegen ③：JOIN roles 而非标量子查询——
--    fresh 库尚无 admin 角色时产出 0 行，由 seed 尾部 SetRoleMenus 全量兜底）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path` IN ('/oauth2','/oauth2/clients') OR m.`permission` LIKE 'oauth2:%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);

-- >>> [system/migrations] 20261013000001_notice_reads.up.sql

-- 公告已读回执（P0-1 通知已读）：每租户每用户对每公告一行。
-- 幂等：CREATE TABLE IF NOT EXISTS 可重复跑。
-- 唯一键 uk(tenant_id,user_id,notice_id) 是幂等写的基础：service 层用
-- gorm clause.OnConflict DoNothing（MySQL 方言译作 INSERT IGNORE），
-- 重复标记不报错、不落重复行。
-- 表结构对齐 20260408000006_basic_admin（id 自增、created_at/updated_at、tenant_id、InnoDB utf8mb4）。
CREATE TABLE IF NOT EXISTS `system_notice_reads` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL,
  `notice_id` bigint unsigned NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_reads_tenant_user_notice` (`tenant_id`,`user_id`,`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='公告已读回执';

-- >>> [system/migrations] 20261016000002_system_notice_attachments.up.sql

-- 公告附件位（子项目 B / P1-1 spec §2）：JSON 数组文本，后端原样存取不解析。
-- ALTER 非幂等——老环境走应用内 migrate（pkg/migrate 按版本记录），勿重复执行。
ALTER TABLE `system_notices` ADD COLUMN `attachments` varchar(4096) NOT NULL DEFAULT '';

-- >>> [infra/migrations] 20261011000001_codegen_config.up.sql

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

-- >>> [infra/migrations] 20261016000001_system_files.up.sql

-- 文件元数据表（子项目 B / P1-1 spec §2）：管理端受控读写带 tenant_id，
-- public 读按 public_key 直取不走过滤（GetByKey，产品语义）。
CREATE TABLE IF NOT EXISTS `system_files` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL DEFAULT '',
  `ext` varchar(32) NOT NULL DEFAULT '',
  `mime` varchar(128) NOT NULL DEFAULT '',
  `size` bigint NOT NULL DEFAULT 0,
  `rel_path` varchar(255) NOT NULL,
  `public_key` varchar(32) NOT NULL,
  `public` tinyint(1) NOT NULL DEFAULT 0,
  `biz_type` varchar(64) NOT NULL DEFAULT '',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0,
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_system_files_rel_path` (`rel_path`),
  UNIQUE KEY `uk_system_files_public_key` (`public_key`),
  KEY `idx_system_files_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- >>> [oauth2/migrations] 20261011000001_oauth2_tables.up.sql

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

-- >>> [oauth2/migrations] 20261011000002_oauth2_seed_clients.up.sql

-- 种子 client（D10）：alexgo-admin 公开（secret 空，授权码流靠 PKCE）、alexgo-app 机密。
-- 幂等：NOT EXISTS 守卫，可重复跑。
INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-admin', '', 'alexGo 管理后台', '',
       'authorization_code,password,refresh_token',
       'http://localhost:5173/callback\nhttp://localhost:8080/callback\nhttp://localhost:5666/callback',
       'system', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-admin');

INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-app', '$2a$10$zIbcMCyAYwodY.mYbdDSNuzeZpbXjTsLxQ9ocIarutvqnwVJCwgd2', 'alexGo app 端', '',
       'password,refresh_token', '', 'member', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-app');

-- >>> [oauth2/migrations] 20261012000001_oauth2_admin_redirect_dev_origin.up.sql

-- 回填 alexgo-admin 回调白名单：vben 后台 SSO 登录的 redirect_uri 由
-- window.location.origin 派生（`<origin>/callback`），dev 端口 5666 不在
-- 20261011000002 种子里 → 登录卡在 400「redirect_uri 不在白名单」。
-- 幂等：NOT LIKE 守卫，可重复跑。
UPDATE `oauth2_client`
SET `redirect_uris` = CONCAT(`redirect_uris`, '\nhttp://localhost:5666/callback'),
    `updated_at` = NOW()
WHERE `client_id` = 'alexgo-admin'
  AND `redirect_uris` NOT LIKE '%http://localhost:5666/callback%';

-- >>> [member/migrations] 20261008000001_init_member.up.sql

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

-- >>> [order/migrations] 20261009000001_orders.up.sql

-- 订单表（R4 裁定，T24 全页巡检撞 Error 1146 Table 'orders' doesn't exist）：
-- 列对齐 modules/order/model/order.go 的 Order 结构；CREATE IF NOT EXISTS 幂等（已存在即跳过）；不加外键。
CREATE TABLE IF NOT EXISTS `orders` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID',
  `product_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商品ID',
  `amount`     BIGINT          NOT NULL DEFAULT 0 COMMENT '金额（分）',
  `status`     INT             NOT NULL DEFAULT 0 COMMENT '订单状态',
  `order_no`   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '订单号',
  `created_at` DATETIME        NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME        NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_orders_order_no` (`order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';

