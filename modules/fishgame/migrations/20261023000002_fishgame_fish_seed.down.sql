-- fishgame：回滚 48 条图鉴种子（仅 tenant_id=0 的客户端对齐条目）
DELETE FROM `fishgame_fish` WHERE `tenant_id` = 0 AND `fish_id` IN (
  'inkfish','saltfish','burnfish','rotfish','worryfish',
  'mixfish','joyfish','surplusfish','wastefish','fakefish',
  'overtime_fish','kpi_fish','ppt_fish','pie_fish','salaried_fish',
  'zen_fish','shore_fish',
  'blank_ppt','old_coffee','unread_msg','seen_no_reply','use_meeting',
  'torn_paper','salary_paper','takeout_receipt','gone_bonus','empty_folder',
  'koi_epic','raise_shell','ontime_badge','double_rest','resign_letter',
  'meeting_fish','duty_fish','reply_fish','form_fish','punch_fish',
  'tea_fish','elevator_fish','staplerfish','printer_fish',
  'wet_wipe','charging_cable','sticky_note','badge_lanyard',
  'promised_fish','headhunter_fish',
  'freedom_bell'
);
