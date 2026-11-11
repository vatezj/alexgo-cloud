# Proposal: fun-salary-home-entry

## Why

用户 2026-11-09 要求：工资到账计算器也需要放在首页，要一个**直达入口**（与趣味方位入口同级、一键进页，不再只依赖【我的】折叠组）。

## What Changes

- 首页新增第二张入口卡「工资到账计算器」（紧随趣味方位入口卡之后，同款骨白+黄铜描边样式），点击直达 `pages/fun/salary/index`。
- 与趣味方位入口共用 `fun_compass_enable` 门控（false 时两张卡同隐）。
- 【我的】页分组入口保持不变（双入口并存）。

**非目标**：改动快捷 pill/广告预留区/其他首页结构；新页面或新路由。

**验收标准**：`pnpm lint` + `type-check` + `build:h5` + `build:mp-weixin` 全绿；首页点击直达工资页、返回落回首页；开关 false 两卡同隐；`openspec validate` 通过。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `fun-salary`: 增加首页直达入口 requirement（原规格仅有【我的】分组入口）。

## Impact

- **代码**：仅 `src/pages/index/index.vue`（一个函数 + 一段模板 + 两行样式）。
- **路由/数据/网络**：无新增（复用既有分包路由）。
