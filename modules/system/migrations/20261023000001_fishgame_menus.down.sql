-- 回滚 fishgame 模块菜单（按 path 精确删，含子页与 codegen 旧页）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND `path` IN ('/fishgame', '/fishgame/fishes', '/fishgame/configs', '/fishgame/events', '/fishgame/players', '/fishgame/stats', '/fishgame/fish')
  AND `type` IN ('dir','menu');
