# 打牌记账资料弹框优化 — 设计

## Context

动机与范围见 proposal.md；需求契约见 specs/profile-gate/spec.md。此处只记现状约束与技术选型。

现状：

- `pages/index/index.vue` 内联了开房资料门的全部逻辑：状态（showProfileGate/gateNickname/gateAvatarPath/savingProfile）、占位昵称与资料完整度判定（`isPlaceholderNickname`/`isProfileComplete`）、`openProfileGate`/`onSaveProfile`（uploadAvatar→updateMe→回写 authStore→doCreateRoom）、模板里 ~90 行 wd-popup（MP chooseAvatar/nickname 原生能力 + H5 chooseImage/wd-input 双分支）及 `.gate-*` 样式。
- 四条进局路径无资料检查：`enterRoomByCode`（房号/扫码）、`consumePendingScene`（已登录 scene）、`pages/login/index.vue` 的 `afterLogin`（登录后 scene）、`resumeLastRoom`（冷启动回房）。
- `components/scorecard/AvatarPopup.vue` 的「✎ 修改昵称」打开的是仅改昵称的内嵌小输入层，改不了头像。
- 判定函数只存在于 index.vue，login 页与 AvatarPopup 无法复用。

约束：

- uniapp CLAUDE.md 强制 OpenSpec 工作流；wot-ui 组件代码须先读 wot-ui-v2 SKILL。
- 双端（H5 / 微信小程序）：小程序端头像昵称必须走 `open-type="chooseAvatar"` / `type="nickname"` 官方能力，H5 走 chooseImage + wd-input。
- 后端接口（uploadAvatar / updateMe）与 authStore 回写链路复用，不改契约。

## Goals / Non-Goals

**Goals:**

- 一套 ProfileGate 组件覆盖 gate / reminder / edit 三模式，消除 index.vue 内联重复。
- 判定与提醒编排抽成共享工具与 composable，index 与 login 两页共用。
- 牌局内改昵称/头像与开房弹框视觉、交互一致。

**Non-Goals:**

- 不改后端任何接口与错误码（30114 已由 toastApiError 透出）。
- 不做「默认资料」的后端判定或服务端提醒。
- 不改开房硬门语义（gate 模式无跳过）。
- 不重构 index.vue 其余逻辑（进房流程、场景处理等保持原样，只挂检查点）。

## Decisions

### D1：ProfileGate 三模式一个组件，模式用 `mode` prop 区分

`mode: 'gate' | 'reminder' | 'edit'`。gate/edit 结构同构（取消 + 主按钮 + 关闭图标），差异仅主按钮文案与保存成功后的行为——由父组件经 `saved` 事件接管（gate 续走开房、edit 关弹框刷数据），组件自身不绑定任何业务动作。reminder 无 X、`close-on-click-modal=false`、仅小号弱色「跳过」文本按钮。

备选：拆成两个组件（Gate + Reminder）——重复头像/昵称/双端输入区 ~70 行，放弃。

### D2：useProfileGate composable 承载提醒编排，index/login 两页各挂一个 ProfileGate

```ts
// 语义骨架（非最终代码）
const show = ref(false)
const mode = ref<'gate' | 'reminder' | 'edit'>('gate')
const pendingAction = ref<(() => void | Promise<void>) | null>(null)

/** gate 硬门：弹框，保存成功后执行 action；取消/关闭则什么都不做 */
function requireProfileForAction(action: () => void): boolean
/** reminder 软提醒：资料完整直接执行 action 返回 true；否则弹框挂起，保存/跳过后续走 */
function ensureProfileThen(action: () => void): boolean
/** ProfileGate 的 saved / skip 统一入口：清 pending、关弹框、续走 */
function proceed(): void
```

- index.vue：挂载 `<ProfileGate>` 绑定该 composable；`onCreateRoom` 走 `requireProfileForAction(doCreateRoom)`（替代现有内联门）；三条进局路径走 `ensureProfileThen(() => join…)`。
- login/index.vue：同样挂载并绑 composable；`afterLogin` 的 scene 分支在 fetchMe 后走 `ensureProfileThen(join)`。不采用「afterLogin 跳过消费、交给 index 消费」的备选——那会先闪现首页再跳房，改变现有直进房的体验。

备选：不用 composable，login 页复制一份检查逻辑——两页 × 双模式状态机重复，放弃。

### D3：资料判定抽为 `utils/scorecard/profile.ts` 纯函数

`isPlaceholderNickname(name)`（`/^微信用户[0-9a-f]{4}$/` / `/^用户\d{4}$/` 或空）与 `isProfileComplete(nickname, avatar)` 原样迁出 index.vue，无状态纯函数，供 composable、ProfileGate、AvatarPopup 复用。判定规则本身不变（proposal 非目标：不做后端判定）。

### D4：reminder 触发时机 = 进局动作发起前（不先 join 再弹）

四条路径统一在调用 join/快照恢复之前检查。理由：与「进入牌局优先设置头像和昵称」语义一致；避免已进房后弹窗遮挡牌桌；保存失败时用户还没进房，状态更干净。备选：进房后在房间页弹——房间页需新增弹框挂载与「跳过后无动作」编排，且牌桌已渲染又弹窗体验割裂，放弃。

`resumeLastRoom` 纳入 reminder（我的裁定）：冷启动回房也是「进入牌局」，资料默认同样先提醒；房已结束时跳过后仍按现状清 id 留首页（spec 已钉）。

### D5：AvatarPopup 内嵌改昵称层整体移除，换挂 ProfileGate（edit）

AvatarPopup 自己挂一个 ProfileGate，「修改昵称」按钮改为打开它；保存成功后 emit/回写刷新房间快照昵称头像。原内嵌 nickname-only 弹层、`onSaveNickname` 等死代码删除。按钮文案改「修改资料」或保留「修改昵称」由实现时顺手定，不改 spec。

### D6：保存链路整体迁移进 ProfileGate

uploadAvatar（头像变更时）→ updateMe → authStore 回写 → emit('saved', me) 全部进组件内部（自 index.vue `onSaveProfile` 平移），三处消费方零重复。失败 toast（toastApiError）+ 保持弹框打开。

## Risks / Trade-offs

- [index.vue 抽取回归风险（开房门行为漂移）] → gate 逻辑原样平移不重写；spec 钉住「取消不开房、保存后自动开房」验收；lint/type-check 兜底。
- [reminder 的「跳过」若样式过弱导致 H5 不可点] → 保证可点热区 ≥ 最小触控、仅视觉弱化（字号/颜色），不 `display:none` 级隐藏。
- [login 页新增弹框与登录跳转时序] → afterLogin 在 fetchMe 之后、join 之前同步判断；pendingAction 仅存内存，页面卸载即弃，无泄漏。
- [MP chooseAvatar 弹层与 wd-popup 层级冲突] → 沿用现有 index.vue 门的已验证结构（同一 open-type 按钮 + wd-popup 组合线上已跑通），不改层叠方案。

## Migration Plan

纯前端、无数据迁移。按 tasks.md 顺序：先抽组件/composable（行为等价、index.vue 用 gate 模式回归），再挂进局提醒，最后换 AvatarPopup。回滚 = revert 对应提交。

## Open Questions

（无——模式划分、触发时机、回房纳入均已定案。）
