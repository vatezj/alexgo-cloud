-- fishgame：48 条图鉴种子，逐字抄自客户端 fish_db.gd ENTRIES（id 为后端唯一认可标识）。
-- period 映射：times 全 [day,dusk,night] → any；否则按实际组合逗号拼接（如 ["night"]→night、["day","dusk"]→day,dusk）。
-- 幂等：WHERE NOT EXISTS；down 为 DELETE tenant_id=0 的 48 个 fish_id。
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'inkfish','墨鱼','normal','any','any','上班这件事，能摸就摸',1,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='inkfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'saltfish','咸鱼','normal','any','any','躺平，是生活的最优解',2,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='saltfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'burnfish','焦鱼','normal','any','any','心里有火，眼里有黑眼圈',3,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='burnfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'rotfish','摆鱼','normal','any','any','摆烂一时爽，一直摆一直爽',4,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='rotfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'worryfish','鱿鱼','normal','any','any','忧愁太多，鱿鱼来凑',5,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='worryfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'mixfish','混鱼','normal','any','any','混口饭吃，混个班上',6,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='mixfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'joyfish','愉鱼','normal','any','any','偷得浮生半日愉',7,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='joyfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'surplusfish','余鱼','normal','any','any','月底还有余粮，算是成功',8,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='surplusfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'wastefish','废鱼','normal','any','any','今天也是无效打工的一天',9,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='wastefish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'fakefish','摸瑜','normal','any','any','假装很努力，其实没干活',10,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='fakefish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'overtime_fish','加班鱼','rare','night','any','越夜越有，工位就是家',11,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='overtime_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'kpi_fish','KPI鱼','rare','any','any','数字很好看，没人知道为什么',12,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='kpi_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'ppt_fish','PPT鱼','rare','any','any','好看的皮囊，全靠 PPT 撑',13,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='ppt_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'pie_fish','画饼鱼','rare','day','any','老板画的饼，又大又圆',14,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='pie_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'salaried_fish','带薪鱼','rare','day,dusk','any','人在工位，心在摸鱼',15,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='salaried_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'zen_fish','佛系鱼','rare','any','any','工资随缘，KPI 随风',16,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='zen_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'shore_fish','上岸鱼','rare','any','any','考编上岸，从此风吹不着',17,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='shore_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'blank_ppt','空白PPT','misc','any','any','三页全是目录，最后一页是结束语',18,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='blank_ppt');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'old_coffee','过期咖啡','misc','day','any','喝了一口，精神了一整夜（并没有）',19,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='old_coffee');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'unread_msg','未读工作消息','misc','night','any','99+ 的红点，不点就不会有事',20,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='unread_msg');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'seen_no_reply','已读不回','misc','any','any','看见了，就是不回',21,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='seen_no_reply');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'use_meeting','无效会议记录本','misc','day','any','开了两小时，结论是下周再开',22,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='use_meeting');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'torn_paper','碎掉的方案纸','misc','any','any','第 18 版，客户喜欢第 1 版',23,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='torn_paper');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'salary_paper','带薪厕纸','misc','day,dusk','any','带薪拉屎，打工人的顶级享受',24,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='salary_paper');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'takeout_receipt','外卖小票','misc','day','any','今天的工资，一半交给了外卖',25,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='takeout_receipt');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'gone_bonus','消失的年终奖','misc','any','any','传说中的东西，没人见过',26,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='gone_bonus');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'empty_folder','空文件夹','misc','any','any','建了 8 个文件夹，全是空的',27,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='empty_folder');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'koi_epic','上岸锦鲤','epic','any','deep','欧皇本皇，一竿上岸',28,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='koi_epic');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'raise_shell','涨薪贝壳','epic','any','deep','打开看看，涨了 0 元',29,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='raise_shell');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'ontime_badge','准时下班勋章','epic','any','deep','6 点整，消失在工位',30,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='ontime_badge');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'double_rest','无痛双休海螺','epic','any','deep','双休不痛，海螺听听',31,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='double_rest');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'resign_letter','离职通知书','epic','night','deep','世界那么大，我想去看看',32,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='resign_letter');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'meeting_fish','会议室鱼','normal','any','any','一开两小时，结论是下周再读',33,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='meeting_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'duty_fish','值班鱼','normal','any','any','假期在工位守着空系统',34,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='duty_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'reply_fish','收到请回鱼','normal','any','any','好的收到收到好的',35,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='reply_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'form_fish','审批鱼','normal','any','any','卡在第三级审批',36,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='form_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'punch_fish','打卡鱼','normal','any','any','人到了，灵魂还在床上',37,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='punch_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'tea_fish','茶水间鱼','normal','any','any','带薪喝水第一名',38,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='tea_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'elevator_fish','电梯鱼','normal','any','any','上行摸鱼下行带屎',39,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='elevator_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'staplerfish','订书机鱼','normal','any','any','咔哒一声，今天的活白干',40,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='staplerfish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'printer_fish','打印机鱼','normal','any','any','卡纸是它唯一的抗议',41,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='printer_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'wet_wipe','湿巾','misc','any','any','擦了工位擦心情',42,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='wet_wipe');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'charging_cable','充电线','misc','any','any','抽屉里三根，全不能用',43,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='charging_cable');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'sticky_note','便利贴','misc','any','any','写着计划，没人执行',44,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='sticky_note');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'badge_lanyard','工牌挂绳','misc','any','any','人可以走，工牌先留',45,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='badge_lanyard');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'promised_fish','期权鱼','rare','night','any','承诺的财富自由，还在路上',46,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='promised_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'headhunter_fish','猎头鱼','rare','day,dusk','any','挖人很急，谈薪很抠',47,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='headhunter_fish');
INSERT INTO `fishgame_fish` (`tenant_id`,`fish_id`,`name`,`rarity`,`period`,`depth`,`flavor`,`sort`,`status`)
SELECT 0,'freedom_bell','自由钟','epic','any','deep','敲响它，就能准点下班',48,1
WHERE NOT EXISTS (SELECT 1 FROM `fishgame_fish` WHERE `tenant_id`=0 AND `fish_id`='freedom_bell');
