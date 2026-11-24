-- taokouling 两表（列与 modules/taokouling/model 严格同构；无外键；数据面恒 tenant_id=0）
CREATE TABLE IF NOT EXISTS `taokouling_channel` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID（恒0）',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '渠道名称',
  `partner` varchar(32) NOT NULL DEFAULT '' COMMENT 'partner短码（空=非partner渠道）',
  `host` varchar(128) NOT NULL DEFAULT '' COMMENT '匹配Host（空=不匹配）',
  `pid` varchar(128) NOT NULL DEFAULT '' COMMENT '淘宝联盟推广位',
  `apikey` varchar(64) NOT NULL DEFAULT '' COMMENT '好单库apikey',
  `tb_name` varchar(64) NOT NULL DEFAULT '' COMMENT '淘宝账号昵称',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态 1=启用 0=停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_partner_host` (`tenant_id`,`partner`,`host`),
  KEY `idx_taokouling_channel_host` (`tenant_id`,`host`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='淘口令转换渠道PID白名单';

CREATE TABLE IF NOT EXISTS `taokouling_conversion_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID（恒0）',
  `endpoint` varchar(16) NOT NULL DEFAULT '' COMMENT 'parse|convert',
  `partner` varchar(32) NOT NULL DEFAULT '' COMMENT '解析到的partner短码',
  `input_text` varchar(500) NOT NULL DEFAULT '' COMMENT '用户输入（截断500）',
  `extracted_tkl` varchar(64) NOT NULL DEFAULT '' COMMENT '提取的口令体',
  `item_id` varchar(32) NOT NULL DEFAULT '' COMMENT '商品ID（可空串）',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '1成功 2降级 3无口令 4硬错误',
  `error` varchar(255) NOT NULL DEFAULT '' COMMENT '失败原因（可空串）',
  `duration_ms` int NOT NULL DEFAULT 0 COMMENT '耗时毫秒',
  `ip` varchar(45) NOT NULL DEFAULT '' COMMENT '客户端IP',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_taokouling_log_tenant_created` (`tenant_id`,`created_at`),
  KEY `idx_taokouling_log_item` (`item_id`),
  KEY `idx_taokouling_log_partner` (`partner`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='淘口令转换记录';

-- 默认渠道种子（spec 决定 11：只种默认行，partner/host 渠道走 admin 录入）
INSERT INTO `taokouling_channel` (`tenant_id`,`name`,`partner`,`host`,`pid`,`apikey`,`tb_name`,`status`)
SELECT 0, '默认渠道', '', '', 'mm_106702626_399850233_108222550160', 'vate', '呵颓废呵', 1 FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `taokouling_channel` WHERE `tenant_id`=0 AND `partner`='' AND `host`='');
