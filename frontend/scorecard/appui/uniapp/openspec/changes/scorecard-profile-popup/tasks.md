## 1. 基础抽取（行为等价）

- [x] 1.1 读 `.claude/skills/wot-ui-v2/SKILL.md`，确认 wd-popup/wd-input 用法与版本 API
- [x] 1.2 新建 `src/utils/scorecard/profile.ts`：迁出 `isPlaceholderNickname` / `isProfileComplete` 纯函数，index.vue 改为引用该模块
- [x] 1.3 新建 `src/components/scorecard/ProfileGate.vue`：从 index.vue 平移保存链路（uploadAvatar→updateMe→authStore 回写）与双端头像/昵称输入区；支持 `v-model:show`、`mode: 'gate' | 'reminder' | 'edit'`、`primary-text` prop，emit `saved` / `skipped`；reminder 无 X、遮罩不可关、仅弱化「跳过」
- [x] 1.4 新建 `src/composables/scorecard/useProfileGate.ts`：mode + pendingAction 编排（requireProfileForAction / ensureProfileThen / proceed）
- [x] 1.5 index.vue 开房门改造：删除内联门状态/模板/样式，`onCreateRoom` 走 `requireProfileForAction(doCreateRoom)` 挂 ProfileGate（gate 模式）；手测开房硬门行为与现状一致

## 2. 进局软提醒

- [x] 2.1 `enterRoomByCode`（房号/扫码）挂 `ensureProfileThen`
- [x] 2.2 `consumePendingScene`（已登录 scene）挂 `ensureProfileThen`
- [x] 2.3 `resumeLastRoom`（冷启动回房）挂 `ensureProfileThen`，保留房已结束清 id 留首页语义
- [x] 2.4 `pages/login/index.vue` 挂 ProfileGate，`afterLogin` scene 分支 fetchMe 后挂 `ensureProfileThen(join)`，保证只进一次房
- [ ] 2.5 资料完整用户四路径不弹窗冒烟；MP 与 H5 各验一轮 reminder（跳过可关、遮罩/X 不可关、跳过后进局）

## 3. 牌局内编辑复用

- [x] 3.1 AvatarPopup 移除内嵌改昵称小输入层及死代码，挂 ProfileGate（edit 模式），「修改昵称」入口改为打开弹框
- [ ] 3.2 保存成功后刷新房间快照昵称头像可见；失败 toast 留弹框可重试
- [x] 3.3 清理 index.vue 残留未用样式/导入

## 4. 验证

- [x] 4.1 `pnpm lint`、`pnpm type-check`、`pnpm build:h5` 全绿
- [x] 4.2 对照 spec 逐 Scenario 走查（gate 硬门 / reminder 关闭约束 / 双端输入 / 局内编辑 / 失败留框）
