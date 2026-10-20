-- scorecard 模块菜单种子（newmodule 生成）：1 目录 + 6 页面。
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。
-- admin 角色绑定不在本迁移做：启动 seed（modules/system/service/seed.go）
-- 每次全量 SetRoleMenus + RebuildPolicies 兜底。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', 'Scorecard', '/scorecard', '', 'lucide:package', 'scorecard', 80, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard' AND `type`='dir' AND `deleted`=0);
-- ② Room 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Room', '/scorecard/rooms', 'views/scorecard/room/list', 'lucide:list', 'scorecard:room:*', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/rooms' AND `deleted`=0);
-- ② RoomUser 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Roomuser', '/scorecard/room_users', 'views/scorecard/room_user/list', 'lucide:list', 'scorecard:room_user:*', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/room_users' AND `deleted`=0);
-- ② RoomLog 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Roomlog', '/scorecard/room_logs', 'views/scorecard/room_log/list', 'lucide:list', 'scorecard:room_log:*', 3, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/room_logs' AND `deleted`=0);
-- ② Record 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Record', '/scorecard/records', 'views/scorecard/record/list', 'lucide:list', 'scorecard:record:*', 4, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/records' AND `deleted`=0);
-- ② Friend 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Friend', '/scorecard/friends', 'views/scorecard/friend/list', 'lucide:list', 'scorecard:friend:*', 5, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/friends' AND `deleted`=0);
-- ② FriendBlack 列表页（component 与 codegen 视图路径同源）
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Friendblack', '/scorecard/friend_blacks', 'views/scorecard/friend_black/list', 'lucide:list', 'scorecard:friend_black:*', 6, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND `path`='/scorecard/friend_blacks' AND `deleted`=0);
