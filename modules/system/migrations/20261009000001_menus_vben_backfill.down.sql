-- 逆序还原。注意：fresh 库上 seed（新代码）用新 icon 建的 11 行也会被 ③ 还原成旧 ant icon——
-- 属接受语义（down 本就是回到迁移前代码形态；再跑 up 会重新 UPDATE 回新值）。

-- ① 先解绑 5 行的 role_menus（经 path 定位菜单）
DELETE `rm` FROM `role_menus` `rm`
JOIN `menus` `m` ON `m`.`id` = `rm`.`menu_id`
WHERE `rm`.`tenant_id`=0 AND `m`.`tenant_id`=0
  AND `m`.`path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders');

-- ② 删 5 行菜单
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND `path` IN ('/dashboard','/dashboard/analytics','/dashboard/workspace','/order','/order/orders')
  AND `type` IN ('dir','menu');

-- ③ 11 处 icon 还原（新 → 旧，过滤条件与 up 对称）
UPDATE `menus` SET `icon`='settings'   WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:settings';
UPDATE `menus` SET `icon`='user'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:user';
UPDATE `menus` SET `icon`='team'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:users';
UPDATE `menus` SET `icon`='menu'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:list-tree';
UPDATE `menus` SET `icon`='apartment'  WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:building-2';
UPDATE `menus` SET `icon`='idcard'     WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:id-card';
UPDATE `menus` SET `icon`='book'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:book-open';
UPDATE `menus` SET `icon`='setting'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:settings-2';
UPDATE `menus` SET `icon`='bell'       WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:bell';
UPDATE `menus` SET `icon`='history'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:history';
UPDATE `menus` SET `icon`='profile'    WHERE `tenant_id`=0 AND `type` IN ('dir','menu') AND `icon`='lucide:file-text';
