# Proposal: home-drop-recent-records

## Why

用户 2026-11-06 指示：首页「最近战绩」区块移除，该区域以后用来放广告。战绩列表在首页属低频信息（战绩页已有完整承载），移除可为广告位腾出首屏下方空间。

## What Changes

- 删除首页「最近战绩」区块：标题、战绩行列表、空态文案及其整块模板。
- 连带清理仅供该区块使用的代码：`recentRecords` 状态、`myRecords` 接口请求、`emptyRecordsText`/`loadingData`、`SETTLE_TEXT`/`settleText`/`recordDate` 辅助函数、`RecordItem` 类型引用、对应 scoped 样式（`.section.records`/`.h2`/`.flows`/`.flow*`/`.cap`/`.rec-cap`）。
- 原区块位置保留注释标明「预留广告位」；既有注释掉的 `adslot` 广告占位草稿不动。
- **不动**：身份牌统计（总分/胜率/胜负次数，`stat()`）、好友数（`listFriends` → 常驻桌友）、`formatDelta`（统计仍用）、开房主流程、趣味方位入口、其余首页结构。

**非目标**：实现广告位本身（以后再做）；战绩页/房间流水等其他战绩入口。

**验收标准**：`pnpm lint` + `pnpm type-check` + `pnpm build:h5` 全绿；打开首页无「最近战绩」标题与战绩行；Network 中首页不再请求 `myRecords`；身份牌统计与趣味方位入口正常。

## Capabilities

### New Capabilities

- `home`: 首页信息架构——主流程区块构成与请求面（本变更定义「无最近战绩区块」契约）。

### Modified Capabilities

（无）

## Impact

- **代码**：仅 `src/pages/index/index.vue`（模板/脚本/样式三处收敛）。
- **API**：首页不再调用 `GET myRecords`（接口本身保留，战绩页仍用）。
- **数据**：无 storage 变更。
