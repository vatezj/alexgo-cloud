# Proposal: home-resume-room

## Why

牌局进行中用户回到首页后无法回房：冷启动自动回房（`resumeLastRoom`）是模块级 once 语义，房间页 ⌂ 用 `replaceAll` 回首页后同一启程不再触发；且首页从未实现 store 注释里规划的「继续上局」入口。冷启动自动回房若遇网络类错误还静默失败（不 toast、不清 id），用户既没进房也没任何提示，同样困在首页。

## What Changes

- 首页新增「回到牌局」入口条：已登录且 `authStore.lastRoomId > 0` 时展示（位于扇形 CTA 上方），显示房号提示，点击后校验房况再进房。
- 点击行为：`snapshot(rid)` → `status===0`（进行中）→ 进房间页；房已结束/关闭（status 1/2）→ 清 `lastRoomId` + toast「牌局已结束」；30101/30102 → 清 `lastRoomId` + toast；网络类错误 → 保留 id + toast 失败提示（可重试，非静默）。
- 冷启动自动回房逻辑保持 once 语义不变（避免 ⌂ 回首页被拽回的回环）；入口条作为手动回房兜底，同时覆盖「同会话 ⌂ 回首页」与「冷启动静默失败」两种滞留场景。
- 未登录不展示入口条（`lastRoomId` 在退出登录后保留，但无 token 无法进房）。

## Capabilities

### New Capabilities

- `home-resume-room`: 首页「回到牌局」手动回房入口——展示条件、点击校验与进房/清理/提示分支、与冷启动自动回房的优先级关系。

### Modified Capabilities

（无——冷启动 once 自动回房行为不变，仅补充手动入口。）

## Impact

- 主要改动：`src/pages/index/index.vue`（新入口条 UI + `onResumeRoom` 处理函数；`resumeLastRoom` 可小幅复用同一校验逻辑，不改其 once 语义）。
- 依赖已有 API：`snapshot`（`src/api/scorecard/room`）、`authStore.lastRoomId`（persist 已有）。
- 无后端/契约变更；无新组件依赖；主题 token 与全局工具类复用（零硬编码色）。
- 跨端：H5 与微信小程序行为一致（纯页面 UI + 已有接口，无扫码/胶囊等端差异）。
