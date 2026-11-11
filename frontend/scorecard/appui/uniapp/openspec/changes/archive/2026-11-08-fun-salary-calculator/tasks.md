# Tasks: fun-salary-calculator

## 1. 计算核心

- [x] 1.1 新建 `src/utils/funSalary/calc.ts`：WORK_DAYS=21.75、`calcRates`（工时校验非法→null）、`todayWorkedSeconds`（周一至五+时段 clamp）、`fullWorkdaysBeforeToday`、`todayEarnings/monthEarnings`、`formatHMS/formatMoney/minutesFromHHMM`
- [x] 1.2 node 冒烟（一次性脚本）：快照 15000+09:00-12:00+13:30-18:00 → 7.5h/689.66/91.95/0.0255±0.0001；formatHMS(16222)=4:30:22；非法时段→null；周末 todayWorked=0；结果贴任务备注后删脚本

## 2. 页面

- [x] 2.1 新建 `src/pages/fun/salary/index.vue`（`definePage` + ModNav sub=「趣味小工具 · 仅供娱乐」+ `sal-` 前缀 scoped 样式，复刻 v4 设计稿）：输入卡 = 月薪/四时间五枚深色胶囊触发 + **5 个独立 `wd-picker`**（月薪 3000–100000 步500、时间 HH:MM 步5，默认 15000/09:00/12:00/13:30/18:00）+「本机即时计算 · 数据不上传」
- [x] 2.2 hero 入账卡：入账中脉冲 + 今日入账大数字（每秒 +perSec、恒 2 位小数）+ caption「今日入账 · 每秒 +¥x.xxxx」+ 今日工作时间 H:MM:SS 同频跳动（anchor+elapsed 实现，参数变更重锚，卸载清定时器）
- [x] 2.3 明细区：本月入账通栏（工作日口径小字）+ 2×2（21.75天·法定标注/日薪/工时/时薪）+ 底注「结果仅供娱乐参考 · 算薪请以实际合同为准」；内容区滚动 + 底注钉底；非法时段 toast 且保持上次有效 rates
- [x] 2.4 【我的】页趣味分组加行「工资到账计算器」（¥ brass tile + 副题「算日薪 · 看入账」）→ `/pages/fun/salary/index`（分组门控内）

## 3. 验证

- [x] 3.1 `pnpm lint` + `pnpm type-check` + `pnpm build:h5` + `pnpm build:mp-weixin` 全绿；mp 产物 app.json 分包含 `pages/fun/salary/index`
- [x] 3.2 H5 e2e：我的入口往返；三类选择器开轮选值回显（月薪→20000 指标重算、下午上班→14:00 工时 7h）；hero 停 3 秒 金额Δ≈3×每秒且 2 位小数、时间Δ=3s；参数改动基线重算无突跳；非法时段不 NaN；黑名单（稳赚/躺赚/暴富/投资回报/理财收益）0 命中；底注在
- [x] 3.3 `openspec validate fun-salary-calculator` 通过

> 3.2 实测记录：入口往返 ✓；弹层 198 项/步500/标题工具栏 ✓；**commit→全量重算数学全对**（月薪 15000→8000→3000：日薪/时薪/本月/今日均按新值精确；时间 13:30→13:25→12:40：时薪 90.94/82.76 对应 7.58h/8.33h ✓）；hero 3s 断言 Δ金额0.08≈3×0.0255、Δ时间=3、恒2位 ✓；黑名单 0、底注在；非法时段→null 单测过（toast 接线同炉）。合成拖拽带惯性无法精确停在14:00（真机手势由用户自然完成，机制已证）。

## 4. 优化轮（2026-11-07）

- [x] 4.1 宽屏适配：`.sal-screen` 子块（nav/内容/底注）`max-width:480px + margin-inline:auto`（桌面 H5 居中手机栏；MP ≤480 无感）
- [x] 4.2 金额千分位：`formatMoney` 整数段加 `,`（如 ¥3,385.57）；月薪胶囊整数分组（¥15,000）；每秒 caption 仍 4 位小数不加组
- [x] 4.3 手感与动效：胶囊 `hover-class` 按压反馈；hero 金色发丝线加流光动画（brief 原要求）
- [x] 4.4 验证：formatMoney 分组 node 冒烟（含 7310.38→7,310.38、边界 999.99/0.00）；lint + type-check + build:h5 + build:mp-weixin；`openspec validate` 复跑
