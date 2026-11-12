# Tasks: fun-compass-off

## 1. 实现

- [x] 1.1 删除 `src/pages/fun/compass/`、`src/utils/funCompass/`、`src/store/funCompass/`、`src/composables/funCompass/`、`src/components/funCompass/ModDisclaimer.vue`
- [x] 1.2 `index.vue`：删方位入口卡 + `goFunCompass` + `useFunCompassConfig`；工资卡门控改为恒显
- [x] 1.3 `mine.vue`：删方位行 + `goCompass` + config/copy import；分组恒显；脚注中性化
- [x] 1.4 `src/config/tools.ts` 去 zhiwei；`.env*` 删 `VITE_ZHIWEI_BASE_URL`（无引用后删 tools.ts）

## 2. 验证

- [x] 2.1 `pnpm lint` + `pnpm type-check` + `pnpm build:h5` + `pnpm build:mp-weixin`
- [x] 2.2 `dist/build/mp-weixin/pages.json`：subPackages 无 compass 页；主包无 pages/fun/compass
- [x] 2.3 grep：`zhiwei|funCompass|fun_compass|排盘|测算` 于 `src/` 用户路径 0 命中（ModNav 注释可留历史说明则改注释）；`openspec validate fun-compass-off`
