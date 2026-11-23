-- 回滚 taokouling 模块菜单（按 path 精确删，含子页）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND (`path`='/taokouling' OR `path`='/taokouling/channels' OR `path`='/taokouling/logs')
  AND `type` IN ('dir','menu');
