-- 回滚 scorecard 模块菜单（按 path 精确删，含子页）。
-- 超集删除：除收敛后的 3 页（rooms/records/friends）外，一并带上脚手架期
-- 6 页里已废弃的 3 条（room_users/room_logs/friend_blacks）——本地库若在
-- 脚手架版迁移已落库后才升级到本版（版本已记账、up 不重跑），行会残留，
-- 本 down 作为幂等清理兜底（down→up 一轮即可洗成 3 页形态）。
DELETE FROM `menus`
WHERE `tenant_id`=0
  AND `path` IN ('/scorecard', '/scorecard/rooms', '/scorecard/records', '/scorecard/friends', '/scorecard/room_users', '/scorecard/room_logs', '/scorecard/friend_blacks')
  AND `type` IN ('dir','menu');
