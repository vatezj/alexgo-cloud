-- scorecard：打牌记账 6 表（列与 modules/scorecard/model 严格同构）
CREATE TABLE IF NOT EXISTS `room` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_code` varchar(8) NOT NULL DEFAULT '' COMMENT '房号4位字母数字',
  `creator_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '当前房主用户ID',
  `status` tinyint NOT NULL DEFAULT 0 COMMENT '状态 0进行中 1已结算 2已关闭',
  `expire_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '超时关闭时间',
  `tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '茶饭目标金额 0=不限',
  `tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '茶饭百分比生效值',
  `tea_pool_total` int NOT NULL DEFAULT 0 COMMENT '茶饭公共池累计',
  `tea_auto_enabled` tinyint NOT NULL DEFAULT 0 COMMENT '自动抽取开关 达标自动置0',
  `max_lose_score` int NOT NULL DEFAULT 0 COMMENT '输分上限 0=不限',
  `round_no` int NOT NULL DEFAULT 0 COMMENT '当前把序号',
  `pending_tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '待生效目标金额',
  `pending_tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '待生效百分比',
  `pending_max_lose_score` int NOT NULL DEFAULT 0 COMMENT '待生效上限',
  `pending_apply_round_no` int NOT NULL DEFAULT 0 COMMENT '待生效应用把序号 0=无',
  `version` int NOT NULL DEFAULT 0 COMMENT '乐观锁版本',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_room_code` (`room_code`),
  KEY `idx_room_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='打牌记账房间';

CREATE TABLE IF NOT EXISTS `room_user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '成员用户ID',
  `cur_score` int NOT NULL DEFAULT 0 COMMENT '当前分数',
  `cum_lose_score` int NOT NULL DEFAULT 0 COMMENT '本会话累计输分',
  `is_creator` tinyint NOT NULL DEFAULT 0 COMMENT '是否房主 1=是',
  `session_no` int NOT NULL DEFAULT 1 COMMENT '进房会话序号 重进+1',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_room_user` (`room_id`,`user_id`),
  KEY `idx_room_user_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='房间成员';

CREATE TABLE IF NOT EXISTS `room_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `from_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '来源用户ID',
  `to_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '目标用户ID',
  `score` int NOT NULL DEFAULT 0 COMMENT '分数',
  `tea` int NOT NULL DEFAULT 0 COMMENT '茶饭',
  `type` tinyint NOT NULL DEFAULT 1 COMMENT '类型 1记分2茶房3进房4退房5结算6上限7配置8状态9转让',
  `client_request_id` varchar(64) NOT NULL DEFAULT '' COMMENT '幂等键',
  `extra` text COMMENT '扩展JSON',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_room_log_room_created` (`room_id`,`created_at`),
  KEY `idx_room_log_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='房间流水';

CREATE TABLE IF NOT EXISTS `record` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `room_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '房间ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '归属用户ID',
  `score` int NOT NULL DEFAULT 0 COMMENT '终局分数',
  `cum_lose_score` int NOT NULL DEFAULT 0 COMMENT '终局累计输分',
  `tea_pool_total` int NOT NULL DEFAULT 0 COMMENT '终局茶饭池快照',
  `tea_percent` decimal(5,2) NOT NULL DEFAULT 0 COMMENT '终局百分比快照',
  `tea_target_amount` int NOT NULL DEFAULT 0 COMMENT '终局目标金额快照',
  `max_lose_score` int NOT NULL DEFAULT 0 COMMENT '终局上限快照',
  `settle_type` tinyint NOT NULL DEFAULT 1 COMMENT '1正常结算2房主退房自动3退房4零数据',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '结算时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_record_user_created` (`user_id`,`created_at`),
  KEY `idx_record_room` (`room_id`),
  KEY `idx_record_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='战绩每成员一行';

CREATE TABLE IF NOT EXISTS `friend` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '用户ID',
  `friend_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '好友用户ID',
  `rank_enabled` tinyint NOT NULL DEFAULT 1 COMMENT '参与排名 1=是',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friend` (`user_id`,`friend_user_id`),
  KEY `idx_friend_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='好友关系';

CREATE TABLE IF NOT EXISTS `friend_black` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '租户ID',
  `user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '拉黑人用户ID',
  `blocked_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT '被拉黑用户ID',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friend_black` (`user_id`,`blocked_user_id`),
  KEY `idx_friend_black_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='拉黑关系';
