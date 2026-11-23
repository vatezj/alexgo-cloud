-- taokouling 模块菜单种子（spec §9-4）：1 目录 + 2 页面。幂等 NOT EXISTS；tenant_id=0。
-- admin 角色绑定由启动 seed（modules/system/service/seed.go）全量 SetRoleMenus + RebuildPolicies 兜底。

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, 0, 'dir', '淘口令', '/taokouling', '', 'lucide:link', 'taokouling', 86, 1, 0, NOW(), NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/taokouling' AND type='dir' AND deleted=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '渠道配置', '/taokouling/channels', 'views/taokouling/TaokoulingChannelsPage', 'lucide:link', 'taokouling:channel', 1, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/taokouling' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/taokouling/channels' AND deleted=0);

INSERT INTO `menus` (`tenant_id`,`parent_id`,`type`,`name`,`path`,`component`,`icon`,`permission`,`sort`,`status`,`deleted`,`created_at`,`updated_at`)
SELECT 0, d.`id`, 'menu', '转换记录', '/taokouling/logs', 'views/taokouling/TaokoulingLogsPage', 'lucide:history', 'taokouling:log', 2, 1, 0, NOW(), NOW()
FROM `menus` d
WHERE d.`tenant_id`=0 AND d.`path`='/taokouling' AND d.`type`='dir' AND d.`deleted`=0
  AND NOT EXISTS (SELECT 1 FROM `menus` WHERE `tenant_id`=0 AND path='/taokouling/logs' AND deleted=0);
