# Tasks: fun-salary-confirm-flow

## 1. 状态机与模板

- [x] 1.1 脚本：`phase` 两态 + `confirmed` 分离；去默认值（初始空数组占位）；`onConfirm`（齐全校验 + 倒挂拦截 + rebase + 折叠）；删 watch 自动重算；tick/rebase 改用 confirmed
- [x] 1.2 模板：空态仅输入卡（占位样式）；输入卡底部加「确认入账」；result 态 = 摘要行（一行三段 + i-carbon-chevron-down，点击展开）+ hero + 明细；编辑态结果区 v-if 隐藏

## 2. 验证

- [x] 2.1 lint + type-check + build:h5 + build:mp-weixin 全绿
- [x] 2.2 Playwright e2e 五场景（空态/确认折叠/再展开更新/倒挂/不全）；断线则人工清单 + `openspec validate` 通过

> 2.2 备注（2026-11-09 H5 实测，dev :5173）：① 空态仅表单+占位，点确认 toast「请把工资和上下班时间都选齐」保持展开；② 选齐确认 → 摘要「¥15,000 · 09:00–12:00 · 13:30–18:00」+ hero/明细，输入卡隐藏；③ 点摘要展开（结果隐藏）→ 下午上班改 14:00 再确认 → 摘要同步、每日工时 7.5→7h、日薪 ¥689.66 不变、时薪升 ¥98.52；④ 下午下班改 13:00（早于 14:00）确认 → toast「下班时间要晚于上班时间」保持展开无结果；⑤ 轮选回显不即时生效（编辑期结果区始终隐藏）。picker 滚轮自动化走 wd-picker-view 实例 `selectWithValue` + CDP touch（合成 mouse/wheel 事件 uni 不响应）。

## 3. 修订轮（2026-11-09 用户三点：窄屏溢出 / 月薪可输入 / 时间默认）

- [x] 3.1 布局修复：`.fcn-nav, .sal-content, .sal-footer` 共享规则补 `box-sizing: border-box`；390px 视口 scrollWidth=viewport 零溢出、宽屏 480 居中不回归（H5 Playwright 实测）
- [x] 3.2 月薪双通道：`salaryStr` 输入框（type=number、默认 15000、placeholder「请输入」）+「选」按钮开既有 wd-picker（独立 `salaryPickV`，确认回填 `salaryStr`）；`draft` 改读 salaryStr（>0 校验）；onConfirm 空/非法 toast「请输入工资」；时间默认 09:00/12:00/13:30/18:00
- [x] 3.3 修订轮验证：lint + type-check + build:h5 + build:mp-weixin 全绿；H5 e2e（默认值就绪不出结果 / 键入月薪确认生效 / 轮选回填 / 月薪清空拦截 / 倒挂拦截 / 摘要折叠闭环）；390px 溢出复测；`openspec validate` 通过

> 3.3 备注（2026-11-09 H5 实测，390×844）：A 进页默认 15000/09:00/12:00/13:30/18:00 就绪、无结果区、scrollWidth=390 零溢出；B 清空月薪确认 → toast「请输入工资数额」保持展开；C 键入 20000 确认 → 摘要「¥20,000 · 09:00–12:00 · 13:30–18:00」+ hero/明细，日薪 919.54（20000/21.75）；D 点「选」轮选 25000 完成 → 输入框回填 25000、结果区仍隐藏；E 下午下班改 13:00（早于 13:30）确认 → toast「下班时间要晚于上班时间」保持展开；F 摘要点击展开（结果隐藏、输入值保留）→ 再确认闭环。宽屏 1200px 复测 480 居中。
