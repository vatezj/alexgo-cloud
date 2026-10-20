-- 回滚 scorecard 模块菜单（按 path 精确删，含子页）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND (`path`='/scorecard' OR `path`='/scorecard/rooms' OR `path`='/scorecard/room_users' OR `path`='/scorecard/room_logs' OR `path`='/scorecard/records' OR `path`='/scorecard/friends' OR `path`='/scorecard/friend_blacks')
  AND `type` IN ('dir','menu');
