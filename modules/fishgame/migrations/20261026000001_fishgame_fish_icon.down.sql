-- fishgame：回滚 icon 列（数据随列删除，无需备份回填）。
ALTER TABLE `fishgame_fish` DROP COLUMN `icon`;
