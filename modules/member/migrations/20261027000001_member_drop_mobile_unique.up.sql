-- 去掉 uk_mobile_tenant：手机号可为空。
-- 微信 MP/Web 登录创建的空壳会员 mobile=''（social/service.go resolveAndIssue），
-- 空串被唯一键当真实值，同租户第二个空壳必撞 Error 1062（uk_mobile_tenant）。
-- 注册/BindMobile 的防重走应用层 GetByMobile 查询（member/service、social/service），
-- 不依赖该唯一键保证正确性。
ALTER TABLE `member_user` DROP INDEX `uk_mobile_tenant`;
