# 打牌记账资料弹框优化

## Why

牌局内只能通过小输入层改昵称、改不了头像，与开房前「完善资料」弹框（头像+昵称一体）交互不一致；且只有开房有资料检查，扫码/房号/小程序码/登录后进房都不检查，默认昵称（微信用户xxxx）和空头像用户直接进局，牌桌辨认困难。

## What Changes

- 抽出统一资料弹框组件 `ProfileGate`（头像+昵称，MP 官方 chooseAvatar/nickname 能力与 H5 chooseImage/wd-input 双端支持），承载三种模式：
  - **gate（开房硬门）**：现状保留——「取消」+「保存并开房」，保存成功续走开房，取消留首页；
  - **edit（局内编辑）**：「取消」+「保存」，供牌局内改资料复用；
  - **reminder（进局软提醒）**：无 X、无遮罩点击关闭，**仅一个视觉弱化的「跳过」可关**；保存或跳过都续走原进局动作。
- 牌局内 AvatarPopup「✎ 修改昵称」小输入层替换为 ProfileGate（edit 模式），头像与昵称都能改。
- 四条进局路径挂 reminder：房号/扫码进房、小程序码 scene 进房、登录后 scene 进房、冷启动回房——资料为默认（占位昵称或空头像）先弹提醒，保存/跳过后继续进局。
- 新增共享工具：占位昵称判定与资料完整度判定从 index.vue 抽出，供各进局点与弹框复用。
- 后端 30114「一人一局」拦截（已落 Go 侧）经现有 toastApiError 自动透出，前端无需专门处理。

### 跨端行为差异

- **微信小程序**：头像用 `open-type="chooseAvatar"` 按钮、昵称用 `type="nickname"` 原生输入（可快捷填充微信资料）；
- **H5/其他端**：头像走 `uni.chooseImage`、昵称走 `wd-input`；reminder 的「跳过」两端样式一致。

## 非目标

- 不改后端资料接口（PUT /api/app/member/me、上传接口契约不变）；
- 不做「默认资料」的后端判定（判定仍纯前端）；
- 不改开房硬门语义（开房仍必须保存资料才能开，无跳过）；
- 不动好友、战绩、房间页其余交互。

## Capabilities

### New Capabilities

- `profile-gate`: 统一资料弹框的三模式交互，以及进局默认资料软提醒的触发路径与关闭约束。

### Modified Capabilities

（无——openspec/specs/ 尚无既有规范）

## Impact

- 新增：`src/components/scorecard/ProfileGate.vue`、`src/composables/scorecard/useProfileGate.ts`、资料判定工具（`src/utils/scorecard/profile.ts`）
- 修改：`src/pages/index/index.vue`（内联门抽取 + 三条进局路径挂提醒）、`src/pages/login/index.vue`（afterLogin scene 进房挂提醒）、`src/components/scorecard/AvatarPopup.vue`（改昵称层换 ProfileGate）
- 依赖：wot-ui `wd-popup`/`wd-input`、既有 `uploadAvatar`/`updateMe` API 封装、authStore 回写链路（均复用，不新增依赖）

## 验收标准

1. 开房：资料不完整弹 gate，行为与现状一致（取消不开房、保存后自动开房）；
2. 扫码/输房号/小程序码/登录后进房：资料默认 → 弹 reminder，页面上除「跳过」外无任何可关闭途径（无 X、点遮罩不关）；「跳过」视觉弱化（小字号、弱色），点后正常进局；改完保存后正常进局且资料已更新；
3. 冷启动回房：资料默认 → 先弹 reminder，保存/跳过后进原房间；房已结束仍按现状清 id 留首页；
4. 牌局内「修改昵称」入口打开的是与开房同款 ProfileGate，可同时改头像与昵称，保存后首页/房间快照昵称头像刷新可见；
5. 资料完整的用户：四条进局路径均不弹窗，直接进局（无多余打扰）；
6. H5 与 MP 双端 lint、type-check 通过。
