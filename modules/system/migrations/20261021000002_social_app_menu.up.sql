-- 渠道配置菜单 + 按钮（微信第三方登录一期：第三方应用管理入口，挂系统管理下）。
-- 幂等：全部 NOT EXISTS 守卫（目录按 path+type，页面按 path，按钮按 permission），可重复跑。
-- 权限码前缀 social:app 必须已在 permission_routes.go 注册（Task 9），
-- 否则本迁移产生的页面/按钮是死权限（403）。
-- 表结构对齐 20261009000001（menus：type 'dir'/'menu'/'button'、sort、deleted、tenant_id=0）。
-- 父目录定位：系统管理 由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定，
-- 403 闭环不依赖嵌套层级）。模式抄 20261011000003 oauth2_client_menu。

-- 1) 目录「渠道配置」（挂在系统管理下；fresh 库父不存在则 parent=0）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'dir', '渠道配置', '/social', '', 'lucide:link', 'social', 92, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/social' AND `type`='dir' AND `deleted`=0);

-- 2) 页面菜单「渠道配置」（挂在目录下；component 由 access.ts glob 自动解析，零路由改动）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '渠道配置', '/social/apps', 'views/system/SocialAppsPage', 'lucide:link', 'social:app', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/social' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/social/apps' AND `deleted`=0);

-- 3) 按钮：新建 / 编辑 / 删除（挂在页面菜单下；前端 v-access 逐字匹配这些码）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '新建' AS `name`, 'social:app:create' AS perm, 1 AS `sort`
  UNION ALL SELECT '编辑', 'social:app:update', 2
  UNION ALL SELECT '删除', 'social:app:delete', 3
) t
WHERE m.`tenant_id`=0 AND m.`permission`='social:app' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 4) admin 角色挂菜单（抄 oauth2 ④：JOIN roles 而非标量子查询——
--    fresh 库尚无 admin 角色时产出 0 行，由 seed 尾部 SetRoleMenus 全量兜底）
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path` IN ('/social','/social/apps') OR m.`permission` LIKE 'social:%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);
