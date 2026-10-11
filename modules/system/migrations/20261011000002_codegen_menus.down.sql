-- 逆序删除：先角色关联，再按钮/页面/目录。
DELETE rm FROM `role_menus` rm
JOIN `menus` m ON m.`id` = rm.`menu_id`
WHERE m.`permission` LIKE 'infra:codegen%' OR m.`path` IN ('/codegen','/codegen/config');

DELETE FROM `menus`
WHERE `permission` LIKE 'infra:codegen%'
   OR `path` IN ('/codegen','/codegen/config');
