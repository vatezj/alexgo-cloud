-- 种子 client（D10）：alexgo-admin 公开（secret 空，授权码流靠 PKCE）、alexgo-app 机密。
-- 幂等：NOT EXISTS 守卫，可重复跑。
INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-admin', '', 'alexGo 管理后台', '',
       'authorization_code,password,refresh_token',
       'http://localhost:5173/callback\nhttp://localhost:8080/callback\nhttp://localhost:5666/callback',
       'system', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-admin');

INSERT INTO `oauth2_client`
  (`client_id`,`client_secret`,`client_name`,`website`,`grant_types`,`redirect_uris`,`account_types`,`scopes`,`activity`,`created_at`,`updated_at`)
SELECT 'alexgo-app', '$2a$10$zIbcMCyAYwodY.mYbdDSNuzeZpbXjTsLxQ9ocIarutvqnwVJCwgd2', 'alexGo app 端', '',
       'password,refresh_token', '', 'member', 'all', 1, NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `oauth2_client` WHERE `client_id`='alexgo-app');
