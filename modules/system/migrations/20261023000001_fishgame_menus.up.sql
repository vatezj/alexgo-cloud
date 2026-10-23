-- fishgame 模块菜单种子（newmodule 生成）：1 目录 + 2 页面。
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。
-- admin 角色绑定不在本迁移做：启动 seed（modules/system/service/seed.go）
-- 每次全量 SetRoleMenus + RebuildPolicies 兜底。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', 'Fishgame', '/fishgame', '', 'lucide:package', 'fishgame', 80, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/fishgame' AND `type`='dir' AND `deleted`=0);
-- ② Fish 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Fish', '/fishgame/fish', 'views/fishgame/fish/list', 'lucide:list', 'fishgame:fish:*', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/fishgame/fish' AND `deleted`=0);
-- ② Player 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Player', '/fishgame/players', 'views/fishgame/player/list', 'lucide:list', 'fishgame:player:*', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/fishgame' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/fishgame/players' AND `deleted`=0);
