-- 反向：与 up 对称删表，先删子表 system_member_identity（引用 system_social_app 的平台配置）。
DROP TABLE IF EXISTS `system_member_identity`;
DROP TABLE IF EXISTS `system_social_app`;
