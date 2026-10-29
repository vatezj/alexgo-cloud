# Proposal: mp-auto-login

## Why

小程序端目前每次都需要用户手动点「微信一键登录」：冷启动未登录、token 过期被401 清态跳登录页后，都要再点一次按钮。微信 `wx.login` 本身是静默接口（无需用户授权弹窗），后端 `POST /api/app/social/wechat/mp/login` 也是公开的 code2session 登录（openid 未命中自动注册），完全具备免点击自动登录的条件。用户诉求：小程序自动登录，不要手动点击。

## What Changes

- **小程序端冷启动静默登录**：`App.vue onLaunch`（条件编译 MP-WEIXIN）在无 token 时静默执行 `uni.login` → `wxLogin(code)` → 写登录态（setLogin + fetchMe），用户直接进入应用、不经过登录页；失败静默降级（保持未登录态，首页引导照旧）。
- **登录页自动触发**：小程序端进入登录页且无 token 时自动执行同一静默登录；成功走既有 `afterLogin` 收口（scene 绑好友 / 进房 / 回首页）；失败才展示手动按钮兜底重试。登录页已有 token 时直接离开（回首页或续走 scene 意图），不停留。
- **共享静默登录模块**：抽取 `useSilentLogin` composable（模块级 in-flight promise 去重），App 冷启动与登录页两处共用，避免并发双发 `wxLogin`。
- **H5 行为不变**：无 `wx.login`，登录页维持手机号密码手动表单默认展开，App 冷启动不加静默登录。

## Capabilities

### New Capabilities

- `auth/auto-login`: 小程序端免点击自动登录——冷启动静默会话恢复、登录页自动触发与失败兜底、已登录进登录页即离、并发去重。

### Modified Capabilities

（无——既有 specs 中无登录相关能力；各页「未登录跳登录」的既有行为不改，只是登录页内恢复方式变化，不构成既有 spec 需求变更。）

## Impact

- 前端（`modules/scorecard/appui/uniapp/src`）：
  - `App.vue`：onLaunch 增加 mp 条件编译的静默登录调用
  - `pages/login/index.vue`：onShow 自动触发 + 已登录即离；复用既有 `afterLogin` 收口（不抽出）
  - 新增 `composables/scorecard/useSilentLogin.ts`（静默登录 + in-flight 去重）
  - `store/scorecard/auth.ts`：可能新增 silentLogin action（setLogin+fetchMe 收口）
- 后端：零改动（`social/wechat/mp/login` 已支持 code2session + openid 未命中自动注册）
- 跨端差异：仅 MP-WEIXIN 生效；H5 登录路径完全不变
- 非目标：不接入 `refresh_token` 刷新流（`/auth/refresh` 前端仍不消费；401 仍走登录页，由登录页自动恢复兜底）；不做 H5 静默登录；不改后端签发策略
