-- starfuyu：回滚 fortune 分享卡 10 列（与 up 对称）
ALTER TABLE `starfuyu_fortune`
  DROP COLUMN `match_constellation`,
  DROP COLUMN `advice`,
  DROP COLUMN `avoid`,
  DROP COLUMN `reunion_tip`,
  DROP COLUMN `single_tip`,
  DROP COLUMN `job_tip`,
  DROP COLUMN `reunion_score`,
  DROP COLUMN `single_score`,
  DROP COLUMN `job_score`,
  DROP COLUMN `daily_todos`;
