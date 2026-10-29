# Design: mp-auto-login

## Context

现状（动机见 proposal.md Why）：

- 登录页 `pages/login/index.vue` 的 `onWxLogin` 是唯一微信登录入口：`uni.login` 取 code → `POST /api/app/social/wechat/mp/login` → `afterLogin`（setLogin → fetchMe → scene 绑好友/进房或回首页）。全部要用户手动点按钮。
- `store/scorecard/auth.ts` persist token/refreshToken；`api/core/handlers.ts` 401 时清态 + 延时 `replaceAll` 登录页；首页多处「未登录 → push 登录页」。
- 后端 `social/wechat/mp/login` 公开：code2session，openid 命中签发、未命中自动注册——静默登录无注册卡点。`wx.login` 无需用户授权弹窗。
- 前端无 vitest；验证靠 `pnpm lint` + `pnpm type-check` + 手动清单。

## Goals / Non-Goals

**Goals:**

- MP-WEIXIN 端两条路径全免点击：冷启动无 token 静默登录（不停留登录页）；落到登录页（401/主动跳入）自动触发登录。
- 失败与主动登出语义清晰：失败不循环、可手动兜底；主动登出后不被自动登回来。
- 并发去重：多入口同时触发只发一次登录请求。

**Non-Goals:**

- 不接入 `refresh_token` / `auth/refresh`（401 仍走登录页，由登录页自动恢复兜底；将来要做续期另立变更）。
- 不改 H5（无 wx.login，密码表单路径原样）。
- 不改后端签发/注册策略；不动 handlers 401 清态框架（仅 logout 语义细分）。

## Decisions

1. **静默登录收口：`composables/scorecard/useSilentLogin.ts` 模块级单例**。导出 `ensureSilentLogin(): Promise<boolean>` 与 `silentWxLogin(): Promise<LoginResult>`：
   - 模块级 `inFlight: Promise<LoginResult> | null` 去重——在途时后来者 await 同一 promise（spec「并发触发去重」）。
   - `failed` 标记：本次小程序生命周期内自动触发失败过一次即不再自动重试（返回 false），避免弱网下每次 onShow 打接口；用户在登录页的手动点击不受此标记限制。
   - 仅 MP-WEIXIN 编译分支内执行 `uni.login`；H5 调用直接返回 false（双保险，调用点也条件编译）。
   - 备选「各调用点各写一套」被否：去重与失败标记必须模块级共享，否则首页/登录页竞态双发。

2. **发起与等待两点位：App onLaunch 发起 + 首页/登录页 onShow `ensureSilentLogin` 等待或兜底发起**：
   - `App.vue onLaunch`（`#ifdef MP-WEIXIN`，无 token 时）fire-and-forget 发起——最早拿到 code，缩短首页看到未登录 UI 的窗口。
   - 首页 `onShow`：`await ensureSilentLogin()` 再走 `enterFromHome`/`loadHomeData`/scene 消费——冷启动 scene 竞态消解（静默登录完成后首页才判定登录态，已登录路径直接消费 pendingScene 进房）。
   - 登录页 `onShow`：已有 token → `replaceAll` 首页（spec「已登录即离」）；无 token → `await silentWxLogin()` 成功后调用既有 `afterLogin` 收口（scene 绑好友/进房逻辑不动）；失败展示手动按钮。
   - 备选「只放 App onLaunch」被否：401 跳登录页不经启动流程，登录页必须有自己的触发点；只放登录页则冷启动首页会先渲染未登录 UI 再跳走，闪现更明显。

3. **`afterLogin` 留在登录页，不抽公共 composable**。冷启动成功路径不需要它（首页 onShow 以「已登录」身份消费 scene，等效覆盖）；登录页自动触发与手动点击共用同一个 `afterLogin`，零重复。抽公共模块属于提前抽象，否。

4. **主动登出与 401 清态语义细分**（关键产品决策）：`authStore.logout()` 现被两处共用——AvatarPopup 主动退出、handlers 401 清态。若不区分，主动登出后首页 onShow 会自动 wxLogin 又把用户登回去，「退出登录」形同虚设。
   - 方案：`logout(opts?: { explicit?: boolean })`；主动退出传 `{ explicit: true }` 置 `explicitLogout=true` 并持久化；`ensureSilentLogin` 见标记直接 false。手动登录成功（setLogin）清除标记。handlers 401 调 `logout()` 不传参（非显式，正是要自动恢复的场景）。
   - 备选「登出后跳登录页并禁用自动」被否：标记放 store 才能跨页面/冷启动持久（storage 恢复），页面级标志进程被杀即丢。

5. **登录页自动触发的重入控制**：`logging` 既有 ref 继续作手动按钮防重入；自动触发由 `onShow` 内「无 token 且无在途」条件天然防重（in-flight 去重兜底）。自动失败后不自动重试，页面停留期间再次 onShow（如切后台回来）会再试一次——可接受（`failed` 标记封顶本生命周期一次，实际最多再试到标记生效前的极少数次；实现时失败路径立即置 failed，等价于只试一次）。

6. **token 过期（本地有 token 但后端已失效）不做本地校验**：首页带旧 token 请求 → 401 → handlers 清态跳登录页 → 登录页 onShow 自动登录 → afterLogin 回原目标（首页）。闭环已覆盖，避免引入本地过期时间戳状态。

## Risks / Trade-offs

- [主动登出后被自动登回] → 决策 4 的 explicitLogout 持久化标记；验收必测。
- [弱网下自动触发失败循环] → failed 标记本生命周期封顶；登录页失败只展示按钮。
- [onLaunch 发起与首页 onShow 竞态双发] → 模块级 in-flight 去重（决策 1）。
- [401 后登录页闪现再跳回] → 已知可接受（微信 code 登录通常 <1s）；不做全屏遮罩，避免过度设计。
- [自动登录静默建号（openid 未命中）] → 后端既定策略（wxLogin 与手动点击同一接口，行为一致），非本变更引入。

## Migration Plan

纯前端行为变更，无数据迁移、无后端发布依赖。回滚 = revert 前端提交（persist 多出的 explicitLogout 字段被旧代码忽略，无兼容问题）。

## Verification

- `pnpm lint` + `pnpm type-check`（无前端单测基建，不建 TDD）。
- 手动清单（微信开发者工具 + 真机）：
  1. 清缓存冷启动（无 token）→ 不出现登录页、首页已登录。
  2. 断网冷启动 → 停留在未登录首页（引导可见），恢复网络后点开房 → 登录页自动登录进房。
  3. 造 401（改 token 字符串）→ 触发任意请求 → 登录页闪现后自动回首页。
  4. 扫码/卡片带房号冷启动 → 自动登录后直接进房，仅一次。
  5. 头像弹窗「退出登录」→ 回登录页且停住（不被自动登回）；重启小程序仍不自动登（explicitLogout 持久化）；手动微信一键登录后恢复。
  6. H5：登录页仍为密码表单默认展开，无自动登录行为。
  7. 登录页自动触发失败（断网）→ 展示手动按钮，点按钮重试成功。

## Open Questions

（无）
