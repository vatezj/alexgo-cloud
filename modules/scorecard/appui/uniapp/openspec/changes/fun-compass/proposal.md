# Proposal: fun-compass

## Why

打牌记账需要一个低权重的休闲彩蛋模块「趣味方位参考」（生辰/罗盘/选座），提升用户停留与打开频次。设计稿（`desgin/preview/` 六屏 + 融合稿）与 PRD（`desgin/xuanzuo.md`）已定稿并完成首版实现；**2026-11-06 用户提出四点修订**（时间输入、首页入口、子包、后端排盘），本变更在首版基础上吸收修订。

> ⚠️ **PRD 冲突备案（用户 2026-11-06 明确指示，覆盖 PRD 红线）**：
> 1. PRD §1/§2「入口必须藏深、禁止放首页」→ 修订为**首页突出入口**（审核风险由需求方承担）；
> 2. PRD §1/§4「生辰不上传后端」→ 修订为**排盘计算走后端 zhiwei 服务**（出生信息随排盘请求发送，档案仍仅存本机、不关联账本）。
> 关于页/隐私文案随之改为如实表述（不再宣称「不会上传服务器」）。

## What Changes

- **出生信息录入改为「年月日 + 时间 + 性别」**：替换原 12 时辰 chip（zhiwei 接口以 `birthDate` + `birthHourIndex` + `gender` 为入参；时间 picker 采集时分，前端换算时辰序号）。
- **排盘计算移到后端**：前端调用 zhiwei 插件服务 `POST /api/calc`（`modules/scorecard/plugin/zhiwei`，紫微 iztro + 奇门 qimen-rs + 评分引擎，已有 37 项测试全过）；返回 top3 推荐方位（direction/score/reason），**四座高亮按 8→4 聚合**（间向半分归邻座）；删除前端 lunar-javascript 排盘。
- **首页突出入口**：宿主首页 fan-cta 下方新增醒目入口卡 → `pages/fun/compass/index`（同样受 `fun_compass_enable` 门控）；【我的】折叠组保留为次级入口。
- **模块整体改为分包**：`pages/fun/compass/*` 六页进 subPackages（root=`pages/fun`），主包只留宿主页与【我的】；路由前缀不变。
- 档案 store schema 升级：`{name, year, month, day, hour, minute, gender}`（原 `hourIndex` 字段废止；本功能未上线，无迁移负担）。
- 文案如实化：隐私行/关于页第 3 段改为「排盘经后端服务计算、仅用于本次计算不存档；档案仅存本机」。

**非目标**：zhiwei 服务本身的开发（已存在）与其部署运维；后端 `fun_compass_enable` 配置接口（仍为前端默认）；日期计算器；埋点；对既有记账页面除入口外的行为改动。

**验收标准**：`pnpm lint` + `pnpm type-check` + `pnpm build:h5` + `pnpm build:mp-weixin` 全绿（mp 产物含 subPackages）；zhiwei 本地 `npm test` 37/37 且 `/api/calc` 冒烟通；H5 端到端（填时间性别 → 推荐座高亮 → 手动改选）；黑名单词 0 命中；两句免责逐字；手动清单覆盖 PRD §8 修订项。

### 跨端差异（H5 vs 微信小程序）

- **罗盘传感器**：MP-WEIXIN 用 `wx.onCompassChange` + 磁力计权限引导（不申请定位）；H5 无传感器，降级为可拖动模拟表盘 + 明示「模拟模式」。
- **排盘请求域名**：H5 走 `VITE_ZHIWEI_BASE_URL`（dev 直连本地服务）；**MP 生产要求 zhiwei 挂在已配置 request 合法域名的网关路径下**（部署项，见 design 风险）。
- **入口胶囊**：MP-WEIXIN 显示微信胶囊占位，H5 显示自定义胶囊（沿用 `#ifdef` 模式）。
- **微信客服会话**：「客服」行 MP 走 `open-type="contact"`；H5 行可见但无平台能力。
- 其余（存储、路由、文案）双端一致。

## Capabilities

### New Capabilities

- `fun-compass`: 趣味方位参考模块——首页/我的双入口门控、六页行为、生辰档案本机存储隔离、后端 zhiwei 排盘与四座推荐、合规免责文案、罗盘跨端行为。

### Modified Capabilities

（无——现有 `expense-tea-only`、`room-ledger-layout` 规格不受影响）

## Impact

- **代码（前端）**：`src/pages/fun/compass/*`（六页改造：表单/结果/分包）；`src/pages/index/index.vue`（突出入口）；`src/pages/mine/index.vue`（保留）；`src/utils/funCompass/{copy,zhiwei}.ts`（文案 + 新客户端；`bazi.ts` 删除）；`src/store/funCompass/archives.ts`（schema）；`vite.config.ts`（subPackages 扫描）；`.env.*`（`VITE_ZHIWEI_BASE_URL`）；`package.json`（移除 lunar-javascript）。
- **代码（后端/插件）**：无改动——直接消费现有 zhiwei 服务（`plugin/zhiwei`，`POST /api/calc`）。
- **依赖**：`-lunar-javascript`；zhiwei 运行需 Node 20+（已有 node_modules 与测试）。
- **数据**：storage 键 `fun_compass_archives`（schema 变更，未上线无迁移）。
- **部署**：zhiwei 服务需常驻（Docker 或 `PORT=3100 node src/index.js`），生产经网关挂到合法域名。
