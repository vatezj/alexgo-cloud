-- 逆序删除：先角色关联，再按钮/页面/目录（模式抄 20261011000003 oauth2_client_menu down）。
DELETE rm FROM `role_menus` rm
JOIN `menus` m ON m.`id` = rm.`menu_id`
WHERE m.`permission` LIKE 'social:%' OR m.`path` IN ('/social','/social/apps');

DELETE FROM `menus`
WHERE `permission` LIKE 'social:%'
   OR `path` IN ('/social','/social/apps');
