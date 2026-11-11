# Design: home-drop-recent-records

## Context

首页 `src/pages/index/index.vue` 中「最近战绩」区块（模板 ~782–815 行）连带一串只服务于它的符号；其下方已有被注释的 `adslot` 广告占位草稿（breakout 设计遗留）。用户要求移除该区块、区域留给未来广告。

## Goals / Non-Goals

**Goals:**
- 最小 diff 删除区块及其独占代码，主流程零扰动。

**Non-Goals:**
- 广告位实现；改动战绩页/房间流水；重构首页其他部分。

## Decisions

### D1 删除清单（独占符号，逐一核对过引用）

| 层 | 删除 | 保留（易误伤） |
|---|---|---|
| 模板 | `<!-- 最近战绩 -->` 整块（含注释掉的 rec-cap 行） | 下方 `adslot` 注释草稿、`grow`、footpills 注释 |
| 脚本 | `recentRecords`、`loadingData`（仅被空态读）、`emptyRecordsText`、`SETTLE_TEXT`、`settleText`、`recordDate`、`myRecords` import、`RecordItem` 类型、`loadHomeData` 中 myRecords 请求与 r 处理 | `stat`/`listFriends`、`friendCount`（身份牌常驻桌友）、`formatDelta`（统计用）、`loadingData` 的读写一并去除 |
| 样式 | `.section.records`、`.h2`、`.flows`、`.flow`/`.flow .who`/`.flow .amt`/`.flow.empty`、`.cap`、`.rec-cap` | `.capsule*`（导航胶囊）、`.adslot*`（草稿）、`.grow` |

### D2 广告位预留标注

删除处留一行模板注释：`<!-- 预留广告位（2026-11-06 移除最近战绩，此区域未来放广告） -->`，与既有 adslot 草稿注释相邻，给未来实现指路；不渲染任何占位 DOM（用户只要求腾地方）。

### D3 请求面

`loadHomeData` 的 `Promise.allSettled` 收敛为 `[stat(), listFriends()]`；失败 toast 逻辑沿用。注释同步改为「stat + listFriends」。

## Risks / Trade-offs

- [删样式误伤] `.cap`/`.h2` 等通用名仅在本文件 scoped 使用，已逐一 grep 确认唯一 → 按清单删。
- [战绩入口减少] 战绩页/房间左滑仍可看流水（非本变更范围）→ spec 不动那些入口。

## Migration Plan

纯删除，回滚 = revert 单提交。

## 验证方案

1. `pnpm lint` + `pnpm type-check` + `pnpm build:h5`。
2. H5 打开首页：无「最近战绩」/无战绩行/无「登录后查看最近战绩」；身份牌统计、趣味方位入口、开房 CTA 在。
3. 静态：`grep myRecords src/pages/index/index.vue` = 0；`openspec validate`。
