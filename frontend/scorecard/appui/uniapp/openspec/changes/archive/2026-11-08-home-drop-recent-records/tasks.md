# Tasks: home-drop-recent-records

## 1. 删除实现

- [x] 1.1 模板：移除「最近战绩」整块（~782–815 行），原位留注释「预留广告位（2026-11-06 移除最近战绩，此区域未来放广告）」；`adslot` 注释草稿保留
- [x] 1.2 脚本：删 `recentRecords`/`loadingData`/`emptyRecordsText`/`SETTLE_TEXT`/`settleText`/`recordDate`；`myRecords` 与 `RecordItem` 引用移除（保留 `stat`/`StatResult`/`formatDelta`/`friendCount`）；`loadHomeData` 收敛为 `[stat(), listFriends()]` 并同步注释
- [x] 1.3 样式：删 `.section.records`/`.h2`/`.flows`/`.flow*`/`.cap`/`.rec-cap`（保留 `.capsule*`/`.adslot*`/`.grow`）

## 2. 验证

- [x] 2.1 `pnpm lint` + `pnpm type-check` + `pnpm build:h5` 全绿
- [x] 2.2 H5 手验：首页无战绩区与相关文案；身份牌统计/趣味方位入口/开房 CTA 正常；`grep myRecords src/pages/index/index.vue` = 0；Network 仅 stat/friends/me/snapshot 无 records
- [x] 2.3 `openspec validate home-drop-recent-records` 通过
