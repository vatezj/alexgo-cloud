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
