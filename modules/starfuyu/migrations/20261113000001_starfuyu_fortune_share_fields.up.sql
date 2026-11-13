ALTER TABLE `starfuyu_fortune`
  ADD COLUMN `match_constellation` varchar(20) NOT NULL DEFAULT '' COMMENT '合拍星座（12星座枚举）' AFTER `constellation`,
  ADD COLUMN `advice` text COMMENT '建议' AFTER `summary`,
  ADD COLUMN `avoid` text COMMENT '避免' AFTER `advice`,
  ADD COLUMN `reunion_tip` text COMMENT '复合运' AFTER `avoid`,
  ADD COLUMN `single_tip` text COMMENT '脱单运' AFTER `reunion_tip`,
  ADD COLUMN `job_tip` text COMMENT '求职运' AFTER `single_tip`,
  ADD COLUMN `reunion_score` int NOT NULL DEFAULT 0 COMMENT '复合运分数0-100' AFTER `job_tip`,
  ADD COLUMN `single_score` int NOT NULL DEFAULT 0 COMMENT '脱单运分数0-100' AFTER `reunion_score`,
  ADD COLUMN `job_score` int NOT NULL DEFAULT 0 COMMENT '求职运分数0-100' AFTER `single_score`,
  ADD COLUMN `daily_todos` json COMMENT '今日必做 3条 JSON数组' AFTER `job_score`;
