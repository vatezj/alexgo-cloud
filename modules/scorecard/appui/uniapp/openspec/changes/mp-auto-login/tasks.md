# Tasks: mp-auto-login

## 1. store：主动登出语义细分

- [x] 1.1 `store/scorecard/auth.ts`：`logout(opts?: { explicit?: boolean })` —— explicit 时置 `explicitLogout` 并纳入 persist；`setLogin` 时清除该标记；handlers 401 调用点保持 `logout()` 不传参（非显式）
- [x] 1.2 `components/scorecard/AvatarPopup.vue`：「退出登录」调用改为 `logout({ explicit: true })`

## 2. 静默登录 composable

- [x] 2.1 新建 `composables/scorecard/useSilentLogin.ts`：模块级 `inFlight` promise 去重 + `failed` 生命周期内封顶；`silentWxLogin(): Promise<LoginResult>`（`#ifdef MP-WEIXIN` 内 uni.login → wxLogin → setLogin + fetchMe，失败置 failed 后抛出）与 `ensureSilentLogin(): Promise<boolean>`（有 token / explicitLogout / failed / H5 → 直接 false 或 true 的短路规则）
- [x] 2.2 确认 `ensureSilentLogin` 在途时复用同一 promise（并发触发不双发 wxLogin）

## 3. 接入点

- [x] 3.1 `App.vue`：onLaunch 内 `#ifdef MP-WEIXIN` 无 token 时 fire-and-forget 调用 `ensureSilentLogin()`（不阻塞启动、吞掉 rejection）
- [x] 3.2 `pages/index/index.vue`：onShow 在判定登录态 / 消费 pendingScene / loadHomeData 之前 `await ensureSilentLogin()`（H5 编译为空操作）
- [x] 3.3 `pages/login/index.vue`：onShow —— 已有 token `replaceAll` 首页即离；无 token 且 MP-WEIXIN 时自动 `silentWxLogin()` 成功后走既有 `afterLogin`；失败展示手动按钮（`logging` 防重入沿用，自动触发失败后不自动重试）

## 4. 验证

- [x] 4.1 `pnpm --dir <uniapp> lint` + `pnpm --dir <uniapp> type-check` 全绿
- [ ] 4.2 手动清单（微信开发者工具/真机）：冷启动无 token 不出现登录页；断网冷启动降级不循环；401 后登录页自动回首页；带 scene 房号冷启动自动登录后只进一次房；主动退出登录后不被自动登回（重启仍停在登出态）；H5 登录页无自动登录
