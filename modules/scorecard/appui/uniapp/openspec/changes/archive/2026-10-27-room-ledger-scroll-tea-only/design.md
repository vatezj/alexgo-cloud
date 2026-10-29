# 房间流水固定滚动 + 茶水单独记账 · 设计

## Context

房间页为 flex 纵向布局（nav → 合规横幅 → 252px 牌桌 → RoomLedger → `.grow` → `.rail` 筹码轨），`min-height:100vh` 只保下限：流水一多账板自然撑高、整页变长，筹码轨被推出视口。RoomLedger 内部是普通 view 列表，无滚动约束；「全部流水」弹层复用同一组件，外层已有 `.lp-scroll`（55vh）负责滚动。

支出页手动 DIY 茶档：`tea` 只能随一笔 `score>0` 的支出提交；后端 `Expense`（`modules/scorecard/service/expense.go:57`）强制 `req.Score <= 0` 直接 10001 拒，事务内输家 -S / 赢家 +S / cum_lose 记账均以 score 为前提。用户确认语义：**可只记茶不记分**（付茶人扣 T 入池、无分数转移）。

## Goals / Non-Goals

- 账板滚动与筹码轨常驻：双端（H5 / 微信小程序）同一布局语义，不引入平台分叉。
- 只记茶：仅手动 DIY 模式；自动抽茶模式、记分附茶、分页弹层契约不变。
- 非目标：不拆双流水；不改 type=2/5 等系统行；不动流水分页；不做茶水档在自动模式下的单独入口。

## Decisions

### D1 账板滚动：flex 剩余空间 + 组件内 scroll-view（备选：写死高度）

页面侧删除 `.grow`，账板外层容器 `flex: 1; min-height: 0` 让账板吃满牌桌与筹码轨之间的剩余空间；RoomLedger 根节点改 flex column，三段结构：

1. header（标题/条数）——固定；
2. `.ledger-rows`——`scroll-view scroll-y` + `flex: 1; min-height: 0`，唯一滚动区；
3. 「查看更多」——移出 rows，`v-if="showMore && logs.length"` 固定在组件底部。

否决「写死 300px 之类高度」：小屏挤压牌桌、大屏浪费空间，flex 方案自适应。

小程序 `scroll-view` 必须有确定高度：由 `min-height: 0` 沿 flex 链传导保证（页面根 → 账板容器 → ledger → rows），H5 同款 CSS 即可。

**真机冒烟修正（微信开发者工具截图反馈后落地）**：

1. **账板高度改为显式计算**（用户要求「减去上面的高度和按钮的高度」）：`calcLedgerHeight` 用 `createSelectorQuery` 测 `.table.bottom`（上部总高：nav+横幅+牌桌）与 `.rail.height`（筹码轨），`h = windowHeight − table.bottom − rail.height` 写入 `.ledger-zone` 内联 `height`（`flex:none`）；`flex:1` 仅作测量失败兜底。触发：`onReady` / `onShow` / `onResize` / 横幅 `@dismiss`（横幅可关会改上部高度）。`.table` / 全局 `.comply` 补 `flex:none` 防压缩。
2. **scroll-view 高度用 `height:0; flex:1`**：仅 `min-height:0` 在 MP 拿不到确定高度，rows 塌缩导致「查看更多」贴标题。
3. **行排布移入内层 view**（`.ledger-rows-inner` 承载 display:flex + gap）：小程序 scroll-view 自带 `display:flex` 会破坏滚动布局。
4. 修正后 `pnpm lint` / `type-check` / `build:h5` 全绿。

**H5 冒烟修正（2026-10-27 代验 4.3 时落地，三处根因）**：

1. **「查看更多」class `fixed` 撞 UnoCSS**：项目启用 UnoCSS，class 里的字面量 `fixed` 被编译成 `position:fixed` 工具类，该行飞出流式布局叠在 header 上——改名 `as-footer`（保留语义 class 勿用 UnoCSS 关键字：`fixed/grow/relative/…`）。
2. **`.room-page` 无定高**：仅 `min-height:100vh` 时 H5 页面根是内容驱动高度，flex 兜底失效（MP 页面容器自带定高所以只在 H5 暴露）——补 `height:100vh`，`calcLedgerHeight` 写入前靠 flex 链也能正确兜底。
3. **`getCurrentInstance()?.proxy` 在异步回调为 null**：`calcLedgerHeight` 经 `nextTick/setTimeout` 触发，回调里取不到实例 → 早退 → 内联高度从未写入（此前误以为 MP 截图正确是 calc 的功劳，实际是 flex 兜底）——setup 期捕获实例存模块变量传用。
4. 修正后实测（H5 390×844）：页面不滚动、zone 内联 height、筹码轨贴底、rows 内部滚动（15 条）、`as-footer` 更多钉账板底可点开分页弹层；lint/type-check/build:h5 三绿。

**弹层复用**：RoomLedger 新增 `scrollable?: boolean`（默认 `false`）。页面账板传 `true`（内部滚动）；「全部流水」弹层不传（`false`，rows 自然流式撑高，由外层 `.lp-scroll` 55vh 滚动），避免嵌套双滚动。

**rows 容器标签**：uniapp 编译器不支持 `<component :is>` 切换内置标签（`[vite:vue] <component is=""/> is not supported`），故 rows 始终为 `scroll-view`——`scrollable` 时 `scroll-y=true` + `.sc` flex 约束；false 时 `scroll-y=false` 自然流式（弹层外层滚动）。

### D2 只记茶：后端放行 score=0 特例 + 前端提交分支

**后端**（`service/expense.go`）：

- 参数校验改为：`(Score <= 0 && Tea <= 0)` 或 `ToUserID == userID` 或 `ClientRequestID == ""` 或 `Tea > maxTeaPerExpense` → 10001；`ToUserID == 0` 仅在 `Score > 0` 时拒（只记茶允许 to=0，避免虚构记分对象）。
- 事务内记账分支：`Score == 0`（此时必为手动 DIY，因自动模式前端不放行且 `req.Tea` 校验已拒空）时跳过输家 -S、`AddCumLose`、赢家 +S，仅执行现有 DIY 扣茶分支（来源在房校验 30113、来源 -T、池 +T）。
- 流水行：type=1、score=0、to_user_id=0、tea=T。幂等回放、达标自动关、上限频控（cum_lose 无变化不触发）、ExpireTime 续期、version 乐观锁全部复用现有路径。
- `expense_test.go` 新增：只记茶成功（池/分/守恒）、score=0 且 tea=0 拒、tea_from 非房内成员拒、score>0 回归不变。

**前端支出页**：

- `canSubmit`：`(score > 0 || (!teaAuto && tea > 0)) && (score > 0 ? toUserId > 0 : true) && !submitting`。
- 校验分支：score=0 且手动茶>0 → 直接提交（跳过「请先选择支出分数/记分对象」）；其余现状。
- 提交参数：score=0 时 `to_user_id: 0`、`tea`/`tea_from_user_id` 照传（来源默认=我）。
- 按钮文案：`score > 0 ? '支出 X 分' : '记茶水 T 分'`。
- 「记给谁」横选区 `v-if="score > 0"`：只记茶时不展示（无记分对象概念）。
- 语音播报：score>0 现状「A支出X分」；score=0 时「A茶水T分」。

### D3 账板茶水行渲染分支

RoomLedger type=1 内分叉：`log.score === 0 && log.tea > 0` → 单行「{付茶人} 茶饭 −{tea} 入池」（jade 色 ScoreText，无箭头/记分对象/茶标签）；`score > 0` 现状结构不变。房间页账板与弹层（同组件）自动一致。付茶人取 `log.from_user_id`（与现状 from 语义一致）。

## Risks / Trade-offs

- [小程序 flex + scroll-view 高度传导失效（rows 不滚）] → 保证链路每层 `min-height: 0`；真机/开发者工具冒烟流水 >10 条场景，tasks 含双端验证。
- [score=0 流水被结算等下游按 score 累加误伤] → 结算读 `cur_score` 快照（表达式列更新），不重放流水；score=0 行对累计无贡献，风险低。结算页展示如有「按流水求和」逻辑需走查——tasks 列走查项。
- [幂等键跨用途复用：同 reqId 先失败（score=0 被旧后端拒）再改参数重试] → 页面失败保留 reqId 的现状契约不变；仅记茶成功路径新生成语义无冲突（同页同参数重试才命中回放）。
- [旧客户端对新后端 to_user_id=0 兼容] → 旧前端从不发 score=0，行为不变；后端向前兼容。

## Migration Plan

后端与前端同仓库、同发布窗口：后端先放行校验（旧前端不受影响），前端随后发布。回滚：前端回滚后后端多放行的 score=0 路径无调用方，无脏数据。

## 验证方案

- 前端：`pnpm lint`、`pnpm type-check`、`pnpm build:h5` 全绿；H5 与微信开发者工具冒烟——流水 >10 条账板内部滚动、筹码轨贴底、「查看更多」常驻；手动茶档只记茶成功回房见「A 茶饭 −T 入池」行；自动模式、记分附茶回归不变。
- 后端：`go test ./modules/scorecard/...` 新增用例 + 既有用例全绿。

## Open Questions

（无——语义已由用户裁定为「可只记茶不记分」。）
