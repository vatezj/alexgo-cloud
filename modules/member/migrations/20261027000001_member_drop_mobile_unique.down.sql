-- 重建手机号租户唯一键。若表内已有重复 (mobile, tenant_id)（多空壳或重复手机号），
-- 本 down 会失败——回滚前需先清理重复行。
ALTER TABLE `member_user` ADD UNIQUE KEY `uk_mobile_tenant` (`mobile`, `tenant_id`);
