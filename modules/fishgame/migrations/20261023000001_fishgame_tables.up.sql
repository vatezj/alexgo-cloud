-- fishgame：摸鱼小游戏 9 表（列与 modules/fishgame/model 严格同构；无外键；游戏端 tenant_id 恒 0）
CREATE TABLE IF NOT EXISTS `fishgame_player` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID（游戏端恒0）',
  `openid` varchar(64) NOT NULL DEFAULT '' COMMENT '微信openid（uk含tenant）',
  `nick` varchar(64) NOT NULL DEFAULT '' COMMENT '昵称',
  `avatar` varchar(255) NOT NULL DEFAULT '' COMMENT '头像URL',
  `last_login_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近登录',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_player_openid` (`tenant_id`,`openid`),
  KEY `idx_player_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏账号';

CREATE TABLE IF NOT EXISTS `fishgame_session` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `token` varchar(64) NOT NULL DEFAULT '' COMMENT 'X-Session token 随机32B base64url',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属玩家',
  `expire_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '过期时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_token` (`token`),
  KEY `idx_session_expire` (`expire_at`),
  KEY `idx_session_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='自持会话';

CREATE TABLE IF NOT EXISTS `fishgame_fish` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `fish_id` varchar(32) NOT NULL DEFAULT '' COMMENT '图鉴id取自客户端fish_db.gd（uk含tenant）',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '名称',
  `rarity` varchar(16) NOT NULL DEFAULT 'normal' COMMENT '稀有度 normal|misc|rare|epic',
  `period` varchar(32) NOT NULL DEFAULT 'any' COMMENT '时段 any|day|dusk|night|组合',
  `depth` varchar(16) NOT NULL DEFAULT 'any' COMMENT '深度 any|shallow|mid|deep',
  `flavor` varchar(255) NOT NULL DEFAULT '' COMMENT '风味文案',
  `sort` int NOT NULL DEFAULT 0 COMMENT '插入序 1-48',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态 1=启用 0=停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_fish_fish_id` (`tenant_id`,`fish_id`),
  KEY `idx_fish_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='图鉴条目';

CREATE TABLE IF NOT EXISTS `fishgame_dex` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '玩家ID',
  `fish_id` varchar(32) NOT NULL DEFAULT '' COMMENT '图鉴id',
  `count` int NOT NULL DEFAULT 0 COMMENT '钓获次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dex_player_fish` (`player_id`,`fish_id`),
  KEY `idx_dex_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='玩家图鉴进度';

CREATE TABLE IF NOT EXISTS `fishgame_track_event` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '玩家ID',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '事件名',
  `t` bigint NOT NULL DEFAULT 0 COMMENT '客户端本地epoch秒',
  `props` varchar(4096) NOT NULL DEFAULT '' COMMENT '事件props JSON文本 ≤4KB',
  `dedup_key` varchar(64) NOT NULL DEFAULT '' COMMENT 'sha1(openid|name|t|canonical_json)',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_track_dedup` (`dedup_key`),
  KEY `idx_track_name_t` (`name`,`t`),
  KEY `idx_track_player_t` (`player_id`,`t`),
  KEY `idx_track_created` (`created_at`),
  KEY `idx_track_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='埋点事实流';

CREATE TABLE IF NOT EXISTS `fishgame_sign` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '玩家ID',
  `date` varchar(10) NOT NULL DEFAULT '' COMMENT 'YYYY-MM-DD（uk: player+date）',
  `streak` int NOT NULL DEFAULT 0 COMMENT '连续签到天数',
  `bait` int NOT NULL DEFAULT 0 COMMENT '本次发放鱼饵',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sign_player_date` (`player_id`,`date`),
  KEY `idx_sign_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='签到对账';

CREATE TABLE IF NOT EXISTS `fishgame_friend` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '玩家ID',
  `friend_player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '好友玩家ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friend_player` (`player_id`,`friend_player_id`),
  KEY `idx_friend_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系';

CREATE TABLE IF NOT EXISTS `fishgame_config` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `type` varchar(16) NOT NULL DEFAULT '' COMMENT 'luck|fish_king',
  `period` varchar(32) NOT NULL DEFAULT '' COMMENT 'luck：day|dusk|night|any',
  `depth_bias` decimal(3,2) NOT NULL DEFAULT 0 COMMENT 'luck：偏置≤0.30',
  `fish_id` varchar(32) NOT NULL DEFAULT '' COMMENT 'fish_king：图鉴id',
  `week` varchar(8) NOT NULL DEFAULT '' COMMENT 'fish_king：ISO周 YYYY-Www',
  `start_at` datetime DEFAULT NULL COMMENT '生效开始 可空=立即',
  `end_at` datetime DEFAULT NULL COMMENT '生效结束 可空=永久',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态 1=启用 0=停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_config_tenant` (`tenant_id`),
  KEY `idx_config_type_week` (`type`,`week`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运营覆盖配置';

CREATE TABLE IF NOT EXISTS `fishgame_share` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `key` varchar(64) NOT NULL DEFAULT '' COMMENT '随机key（唯一）路径 /share/<key>',
  `player_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属玩家',
  `size` bigint NOT NULL DEFAULT 0 COMMENT '字节数',
  `mime` varchar(32) NOT NULL DEFAULT '' COMMENT 'image/jpeg|image/png',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_share_key` (`key`),
  KEY `idx_share_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分享图元数据';
