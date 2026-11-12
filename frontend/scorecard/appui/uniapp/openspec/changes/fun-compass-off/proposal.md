# Proposal: fun-compass-off

## Why

微信小程序审核再次拒绝：「涉及算卜算运势、算命……请移除相关内容或功能」。上一轮仅做文案隐晦化（fun-compass-review-copy）无效——审核针对的是**出生时间 → 推荐方位/选座**的算卜功能链本身，而非措辞。需要从提交包与产品面移除该功能，保留工资计算器。

## What Changes

- **A 打包剔除**：删除 `src/pages/fun/compass/` 六页；重建后 `pages.json` 分包仅剩 salary；首页/我的「趣味方位」入口下线。
- **B 拆因果链**：删除 zhiwei 客户端、出生档案/推荐 store、funCompass copy/config、免责声明组件（salary 不用）；`config/tools.ts` 去掉 zhiwei 端点；清理 `.env*` 的 `VITE_ZHIWEI_BASE_URL`。
- **保留**：`pages/fun/salary`（工资到账计算器）、共享 `ModNav`（salary 在用，sub 已自带中性副题）。
- **不改**：记账主流程；后端 `plugin/zhiwei` 仓库代码与生产 `/api/calc` 路由（非端侧包内容，本轮不动服务端）。

## Capabilities

### Modified Capabilities

- `fun-compass`：能力下线——包内不再提供趣味方位/排盘/选座推荐。

## Impact

- Affected specs: `specs/fun-compass/spec.md`（ADDED 下线要求）
- Affected code: `src/pages/fun/compass/*` 删除；`src/pages/index/index.vue`、`src/pages/mine/index.vue` 入口；`src/utils/funCompass`、`src/store/funCompass`、`src/composables/funCompass` 删除；`src/components/funCompass/ModDisclaimer.vue` 删除；`src/config/tools.ts`；`.env*`
- 依赖方：salary 仅依赖 `ModNav.vue`（保留）
