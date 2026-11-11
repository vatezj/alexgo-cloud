# Tasks: home-resume-room

## 1. 首页入口逻辑

- [x] 1.1 `src/pages/index/index.vue`：新增 `resuming` ref 与 `onResumeRoom` 处理函数——`gate.ensureProfileThen` 包裹，`snapshot(lastRoomId)` 校验：status=0 → `router.push` 进房；status=1/2 → 清 `lastRoomId` + toast「牌局已结束」；30101/30102 → 清 id + toast；其他错误 → 保留 id + `toastApiError`
- [x] 1.2 确认 `resumeLastRoom` once 语义与静默 catch 不变（仅注释微调说明手动入口兜底），scene 优先级不变

## 2. 首页入口 UI

- [x] 2.1 fan-cta 上方加「回到牌局」入口条：`v-if="isLoggedIn && authStore.lastRoomId > 0"`，文案「回到牌局」+「回到上一局 · 校验后进入」副文 + 右侧 chevron（`wd-icon css-icon i-carbon-chevron-right`），点击 `onResumeRoom`，`resuming` 时禁点
- [x] 2.2 样式：主题 token（`--brass` 边框 / `--ink-700` 底 / `--on-ink`、`--dim` 文字），零硬编码色；对齐 fan-quick 圆角与间距观感

## 3. 验证

- [x] 3.1 `pnpm lint` 与 `pnpm type-check` 通过
- [ ] 3.2 H5 手动验证：进房 → ⌂ 回首页 → 入口条出现 → 点击回房；lastRoomId 指向已结束房 → 点击 toast「牌局已结束」+ 条消失；登出 → 条消失
