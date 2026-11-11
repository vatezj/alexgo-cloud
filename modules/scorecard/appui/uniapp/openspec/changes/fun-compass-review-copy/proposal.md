# Proposal: fun-compass-review-copy

## Why

小程序上架审核对封建迷信/占卜算命类表述敏感。趣味方位模块当前用户可见文案含「排八字」「排盘」「生辰」「五行」，后端推荐 reason 含「奇门/生门/财气/吉凶/煞星/旺相/命宫」等原话，会原样展示在结果卡，存在被打回风险。需要在不改交互与评分逻辑的前提下，把用户可见文案整体隐晦化为中性民俗娱乐口径。

## What Changes

- 前端六页 + 【我的】+ 首页入口：清除「排八字 / 排盘 / 生辰 / 五行」等刺眼词，改为「测方位 / 出生时间 / 出生档案」等中性表述；导航标题、toast、loading、空态同步。
- `copy.ts` 锁定文案中涉及排盘/生辰/五行的句子改为合规口径（两句底部免责与「民俗娱乐」框架保留）。
- 后端 zhiwei `scoring.js`：**仅改 reason 文案**，计分/排序/过滤逻辑与分值不变；测试断言同步为新文案。
- `zhiwei.ts` 客户端错误 toast 中「排盘」改「测算」。
- 新增/扩展自测黑名单：模块可见文案对「八字/排盘/运势/吉凶/财气/煞」等 0 命中。

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `fun-compass`：模块用户可见文案与后端 reason 的审核安全口径（交互、档案本机隔离、免责框架、评分逻辑不变）。

## Impact

- Affected specs: `openspec/changes/fun-compass-review-copy/specs/fun-compass/spec.md`（ADDED）
- Affected code: `src/pages/fun/compass/*`、`src/pages/mine/index.vue`、`src/pages/index/index.vue`、`src/utils/funCompass/copy.ts`、`src/utils/funCompass/zhiwei.ts`；`modules/scorecard/plugin/zhiwei/src/services/scoring.js` + `test/scoring.test.js`
- 不影响：路由/分包、Pinia schema、API 契约字段（reason 仍为 string）、分享卡默认 title、两句锁定免责主干
