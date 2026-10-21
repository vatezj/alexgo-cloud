-- P1-3 在线用户 + 代登录菜单种子（抄 20261011000003 oauth2_client_menu 模式）。
-- 幂等：全部 NOT EXISTS 守卫，可重复跑。
-- 权限码 system:online:list / :kick / :impersonate 必须与 permission_routes.go 的
-- "system:online" 键同源（prefix 前两段 system:online），否则本迁移产出死权限（403）。
-- 父目录「系统管理」由 StartSeeder 在迁移之后创建（fresh 库），故 parent 用
-- COALESCE 兜底 0——已有库挂系统管理下，fresh 库落根（seed 尾部 SetRoleMenus 全量绑定）。

-- 1) 页面菜单「在线用户」
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, COALESCE((SELECT p.`id` FROM `menus` p WHERE p.`tenant_id`=0 AND p.`name`='系统管理' AND p.`type`='dir' AND p.`deleted`=0 LIMIT 1), 0),
       'menu', '在线用户', '/system/online-users', 'views/system/SystemOnlineUsersPage', 'lucide:monitor', 'system:online:list', 95, 1, 0, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/system/online-users' AND deleted=0);

-- 2) 按钮：踢人 / 代登录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, m.`id`, 'button', t.`name`, '', '', '', t.`perm`, t.`sort`, 1, 0, NOW(), NOW()
FROM `menus` m
JOIN (
  SELECT '踢人' AS `name`, 'system:online:kick' AS perm, 1 AS `sort`
  UNION ALL SELECT '代登录', 'system:online:impersonate', 2
) t
WHERE m.`tenant_id`=0 AND m.`permission`='system:online:list' AND m.`type`='menu' AND m.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` b WHERE b.`tenant_id`=0 AND b.`permission`=t.`perm` AND b.`deleted`=0);

-- 3) admin 角色挂菜单
INSERT INTO `role_menus` (`role_id`,`menu_id`,`tenant_id`)
SELECT r.`id`, m.`id`, 0
FROM `menus` m
JOIN `roles` r ON r.`tenant_id`=0 AND r.`code`='admin' AND r.`deleted`=0
WHERE m.`tenant_id`=0 AND m.`deleted`=0
  AND (m.`path`='/system/online-users' OR m.`permission` LIKE 'system:online%')
  AND NOT EXISTS (SELECT 1 FROM `role_menus` rm WHERE rm.`tenant_id`=0 AND rm.`role_id`=r.`id` AND rm.`menu_id`=m.`id`);
