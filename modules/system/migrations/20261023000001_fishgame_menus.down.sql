-- 回滚 fishgame 模块菜单（按 path 精确删，含子页）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND (`path`='/fishgame' OR `path`='/fishgame/fish' OR `path`='/fishgame/players')
  AND `type` IN ('dir','menu');
