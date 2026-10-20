-- scorecard 模块菜单种子（脚手架生成后手工收敛：6 页 → 3 页）
-- 幂等：全部 INSERT 走 NOT EXISTS；tenant_id=0 全局菜单。

-- ① 模块根目录
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', 'Scorecard', '/scorecard', '', 'lucide:package', 'scorecard', 80, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard' AND type='dir' AND deleted=0);

-- ② 房间监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Rooms', '/scorecard/rooms', 'views/scorecard/ScorecardRoomsPage', 'lucide:building', 'scorecard:room', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/rooms' AND deleted=0);

-- ③ 战绩监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Records', '/scorecard/records', 'views/scorecard/ScorecardRecordsPage', 'lucide:trophy', 'scorecard:record', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/records' AND deleted=0);

-- ④ 好友监管页
INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', 'Friends', '/scorecard/friends', 'views/scorecard/ScorecardFriendsPage', 'lucide:users', 'scorecard:friend', 3, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/scorecard' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/scorecard/friends' AND deleted=0);
