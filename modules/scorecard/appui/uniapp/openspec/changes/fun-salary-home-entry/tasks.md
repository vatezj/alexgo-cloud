# Tasks: fun-salary-home-entry

## 1. 实现

- [x] 1.1 `src/pages/index/index.vue`：新增 `goSalary()`（→ `/pages/fun/salary/index`）；funentry 卡后插入第二张同款卡（`¥` 黄铜图标 + 标题「工资到账计算器」+ 副题「算日薪 · 每秒入账」，`v-if="funEnabled"`）
- [x] 1.2 样式：`.funentry + .funentry { margin-top: 10px }` 与 `.funentry-y`（Oswald 黄铜 22px）

## 2. 验证

- [x] 2.1 `pnpm lint` + `pnpm type-check` + `pnpm build:h5` + `pnpm build:mp-weixin` 全绿
- [ ] 2.2 H5 手验：首页两卡并存、点击工资卡直达、返回落首页；`openspec validate fun-salary-home-entry` 通过

> 2.2 备注：validate 已过；点击直达的浏览器自动验证因 Playwright MCP 断线改人工——dev 已起，打开 http://localhost:5173 首页点「工资到账计算器」卡即验（router.push 与已 e2e 过的 goFunCompass 同模式，编译/路由产物 build 已证）。
