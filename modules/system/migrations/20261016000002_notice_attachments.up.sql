-- 公告附件位（子项目 B / P1-1 spec §2）：JSON 数组文本，后端原样存取不解析。
-- ALTER 非幂等——老环境走应用内 migrate（pkg/migrate 按版本记录），勿重复执行。
ALTER TABLE `system_notices` ADD COLUMN `attachments` varchar(4096) NOT NULL DEFAULT '';
