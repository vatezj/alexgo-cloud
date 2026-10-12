-- 回填 alexgo-admin 回调白名单：vben 后台 SSO 登录的 redirect_uri 由
-- window.location.origin 派生（`<origin>/callback`），dev 端口 5666 不在
-- 20261011000002 种子里 → 登录卡在 400「redirect_uri 不在白名单」。
-- 幂等：NOT LIKE 守卫，可重复跑。
UPDATE `oauth2_client`
SET `redirect_uris` = CONCAT(`redirect_uris`, '\nhttp://localhost:5666/callback'),
    `updated_at` = NOW()
WHERE `client_id` = 'alexgo-admin'
  AND `redirect_uris` NOT LIKE '%http://localhost:5666/callback%';
