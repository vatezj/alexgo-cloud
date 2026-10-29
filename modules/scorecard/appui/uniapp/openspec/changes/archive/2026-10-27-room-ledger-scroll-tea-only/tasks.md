# Tasks · room-ledger-scroll-tea-only

## 1. 后端：Expense 放行只记茶

- [x] 1.1 `service/expense.go` 参数校验放宽：`(Score<=0 && Tea<=0)` 才拒；`ToUserID==0` 仅 Score>0 时拒；Tea 超上限/ClientRequestID 空/ToUserID==本人拒不变
- [x] 1.2 事务内记账分支：Score==0 时跳过输家 -S、AddCumLose、赢家 +S，仅走 DIY 扣茶（来源在房校验、来源 -T、池 +T）；流水写 type=1/score=0/to=0/tea=T
- [x] 1.3 `service/expense_test.go` 新增用例：只记茶成功（分/池/守恒）、score=0 且 tea=0 拒、tea_from 非房内成员拒、score>0 记分附茶回归不变；`go test ./modules/scorecard/...` 全绿

## 2. 房间页：账板固定高度内部滚动

- [x] 2.1 `RoomLedger.vue` 增加 `scrollable` prop（默认 false）：true 时 `.ledger-rows` 用 scroll-view（scroll-y + flex:1 + min-height:0）、「查看更多」移出滚动区固定组件底部；false 保持自然流式（弹层复用不变）
- [x] 2.2 `pages/room/index.vue` 账板外层包 `flex:1; min-height:0` 容器并给 RoomLedger 传 `scrollable`，删除 `.grow` 占位；确认筹码轨 `.rail` 常驻视口底部

## 3. 支出页：只记茶 + 账板渲染分支

- [x] 3.1 `pages/expense/index.vue`：canSubmit 与校验分支（手动模式 score=0 且 tea>0 可提交、不要求选人；分数>0 仍须选人；自动模式仍须分数）；提交参数 score=0 时 to_user_id=0；按钮文案「记茶水 T 分」；「记给谁」横选区 score>0 才展示；语音播报分支
- [x] 3.2 `RoomLedger.vue` type=1 分支：score=0 且 tea>0 渲染「{付茶人} 茶饭 −T 入池」（jade，无箭头/记分对象）；score>0 结构不变

## 4. 验证

- [x] 4.1 `pnpm lint`、`pnpm type-check`、`pnpm build:h5` 全绿
- [x] 4.2 走查：结算页/流水相关展示对 score=0 流水无按 score 累加的误伤路径（结算排行/IOU 走 cur_score 账本、无流水累加；发现并修复房间页「最近一手」误把 score=0 只记茶展示成 A −0 → B +0）
- [x] 4.3 H5 与微信开发者工具冒烟（用户或代验）：流水 >10 条账板内部滚动且筹码轨贴底、「查看更多」常驻；手动茶档只记茶成功回房见茶水行；自动模式与记分附茶回归不变
      - H5 代验通过（2026-10-27，Playwright + 后端 9090 + 造数房 TL34）：15 条流水内部滚动、筹码轨贴底、查看更多常驻可点开分页弹层；UI 只记茶（记茶水 10 分）回房见「茶饭 −10 入池」、池 29→39、只扣付茶人；自动模式（PUT config tea_auto_enabled=true）显示「按 0.1% 自动抽取」、隐藏手动茶档、score=0 提交禁用；记分附茶（10/茶5/乙）回房见「甲−10→乙+10 茶+5」、池→44。
      - 冒烟中发现并修复三处 H5 布局根因（详见 design.md D1「H5 冒烟修正」）：`fixed` class 撞 UnoCSS、`.room-page` 无定高、`calcLedgerHeight` 异步回调 proxy 早退。
      - 微信开发者工具侧待用户勾验（组件逻辑双端同一，MP 页面容器定高已具备）。
