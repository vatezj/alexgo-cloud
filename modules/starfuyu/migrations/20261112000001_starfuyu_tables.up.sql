-- starfuyu：6 表（列与 modules/starfuyu/model 严格同构；无外键；内容表 fortune/sign_cache 的 tenant_id 恒 0，用户表跟 member 租户）
CREATE TABLE IF NOT EXISTS `starfuyu_profile` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `member_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属member用户（uk含tenant）',
  `constellation` varchar(20) NOT NULL DEFAULT '' COMMENT '12星座枚举值',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_profile_member` (`tenant_id`,`member_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户星座偏好';

CREATE TABLE IF NOT EXISTS `starfuyu_fortune` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID（内容表恒0）',
  `date` varchar(10) NOT NULL DEFAULT '' COMMENT 'YYYY-MM-DD（uk: tenant+date+constellation）',
  `constellation` varchar(20) NOT NULL DEFAULT '' COMMENT '12星座枚举值',
  `daily_overview` text COMMENT '每日概述',
  `overall_score` int NOT NULL DEFAULT 0 COMMENT '综合运势 0-100',
  `love_score` int NOT NULL DEFAULT 0 COMMENT '爱情运势',
  `career_score` int NOT NULL DEFAULT 0 COMMENT '事业运势',
  `wealth_score` int NOT NULL DEFAULT 0 COMMENT '财运',
  `health_score` int NOT NULL DEFAULT 0 COMMENT '健康运势',
  `lucky_color` json COMMENT '幸运颜色 JSON {chinese,english,hex}',
  `lucky_number` int NOT NULL DEFAULT 0 COMMENT '幸运数字',
  `summary` text COMMENT '今日总评',
  `love_tip` text COMMENT '爱情小贴士',
  `career_tip` text COMMENT '事业小贴士',
  `wealth_tip` text COMMENT '财运小贴士',
  `health_tip` text COMMENT '健康小贴士',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_fortune_date_const` (`tenant_id`,`date`,`constellation`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每日星座运势内容（tid恒0）';

CREATE TABLE IF NOT EXISTS `starfuyu_bazi` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `member_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属member用户（uk含tenant）',
  `birth_date` varchar(10) NOT NULL DEFAULT '' COMMENT '出生日期 YYYY-MM-DD',
  `birth_time` varchar(8) NOT NULL DEFAULT '' COMMENT '出生时间 HH:MM:SS',
  `birth_place` varchar(100) NOT NULL DEFAULT '' COMMENT '出生地点（可选）',
  `bazi_info` json COMMENT '八字信息 JSON 四柱/五行/节气/农历/生肖/星座',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_bazi_member` (`tenant_id`,`member_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='我的八字';

CREATE TABLE IF NOT EXISTS `starfuyu_divination` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `member_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属member用户',
  `bazi_id` bigint unsigned DEFAULT NULL COMMENT '关联八字ID（可空，无八字降级路径）',
  `question` text COMMENT '求签问题（可空）',
  `sign_number` int NOT NULL DEFAULT 0 COMMENT '签号 1-100',
  `sign_content` text COMMENT '签文（starfuyu_sign_cache 命中值，同号稳定）',
  `interpretation` text COMMENT '签文解读（实时生成落库）',
  `category` varchar(20) NOT NULL DEFAULT '' COMMENT '签文类别 事业/爱情/健康/财运/综合',
  `lucky_direction` varchar(50) NOT NULL DEFAULT '' COMMENT '幸运方位',
  `lucky_color` varchar(50) NOT NULL DEFAULT '' COMMENT '幸运颜色（文本，非运势表JSON）',
  `advice` text COMMENT '具体建议',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_div_member_created` (`tenant_id`,`member_user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='求签记录';

CREATE TABLE IF NOT EXISTS `starfuyu_request_limit` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `member_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属member用户（uk含tenant）',
  `date` varchar(10) NOT NULL DEFAULT '' COMMENT 'YYYY-MM-DD 上海时区日期（uk含tenant）',
  `type` varchar(20) NOT NULL DEFAULT '' COMMENT '限流类型 bazi_calculate|divination_draw',
  `count` int NOT NULL DEFAULT 0 COMMENT '当日已用次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_limit_user_date_type` (`tenant_id`,`member_user_id`,`date`,`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='每日请求限流';

CREATE TABLE IF NOT EXISTS `starfuyu_sign_cache` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID（签文全局内容恒0）',
  `sign_number` int NOT NULL DEFAULT 0 COMMENT '签号 1-100（uk含tenant）',
  `sign_content` text COMMENT '签文内容（同号稳定）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sign_number` (`tenant_id`,`sign_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='签文按签号缓存（tid恒0）';
