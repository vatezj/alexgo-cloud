-- 逆序删除：先角色关联，再按钮/页面/目录（模式抄 20261011000002 codegen down）。
DELETE rm FROM `role_menus` rm
JOIN `menus` m ON m.`id` = rm.`menu_id`
WHERE m.`permission` LIKE 'oauth2:%' OR m.`path` IN ('/oauth2','/oauth2/clients');

DELETE FROM `menus`
WHERE `permission` LIKE 'oauth2:%'
   OR `path` IN ('/oauth2','/oauth2/clients');
