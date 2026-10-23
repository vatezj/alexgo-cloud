-- fishgame 模块菜单种子（Task 10 手工收敛：codegen 2 页 → 5 页，对齐 permissionRoutes）。
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。
-- admin 角色绑定不在本迁移做：启动 seed（modules/system/service/seed.go）
-- 每次全量 SetRoleMenus + RebuildPolicies 兜底。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', 'Fishgame', '/fishgame', '', 'lucide:package', 'fishgame', 85, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame' AND type='dir' AND deleted=0);

-- ② 图鉴条目页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Fishes', '/fishgame/fishes', 'views/fishgame/FishgameFishesPage', 'lucide:fish', 'fishgame:fish', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame/fishes' AND deleted=0);

-- ③ 运营配置页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Configs', '/fishgame/configs', 'views/fishgame/FishgameConfigsPage', 'lucide:settings', 'fishgame:config', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame/configs' AND deleted=0);

-- ④ 埋点事件页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Events', '/fishgame/events', 'views/fishgame/FishgameEventsPage', 'lucide:activity', 'fishgame:event', 3, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame/events' AND deleted=0);

-- ⑤ 玩家监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Players', '/fishgame/players', 'views/fishgame/FishgamePlayersPage', 'lucide:users', 'fishgame:player', 4, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame/players' AND deleted=0);

-- ⑥ 数据看板页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Stats', '/fishgame/stats', 'views/fishgame/FishgameStatsPage', 'lucide:bar-chart-3', 'fishgame:stats', 5, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/fishgame/stats' AND deleted=0);
