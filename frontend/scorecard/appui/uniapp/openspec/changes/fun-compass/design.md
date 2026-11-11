# Design: fun-compass

## Context

- 设计稿已定稿：`modules/scorecard/appui/desgin/preview/`（六屏 + 融合稿）；PRD `desgin/xuanzuo.md`；**2026-11-06 用户四点修订覆盖 PRD 两条红线**（见 proposal 备案：首页入口、生辰后端计算）。
- 首版实现已完成（19/21 任务，H5 全链路验证过）；本设计描述**修订轮**的落地方式。
- zhiwei 插件 = 现成 Node 服务（`modules/scorecard/plugin/zhiwei`）：`POST /api/calc`，入参 `birthDate('YYYY-MM-DD') + birthHourIndex(0–12) + gender(male|female) + playTime(带偏移 ISO8601)`，返回 `recommendations[{rank,direction(八方位),score,reason}]`；37 项测试全过；Express + cors，`PORT` 环境变量（默认 80）。
- 工程约定同前：`pages.config.ts` + `@uni-helper/vite-plugin-uni-pages` 自动生成 pages.json；色值零硬编码；验证 = lint + type-check + build + 手动清单；Node v22.22.0 PATH。

## Goals / Non-Goals

**Goals:**
- 四点修订全部落地且过双端构建；zhiwei 端到端可用；文案与行为如实。

**Non-Goals:**
- 改造 zhiwei 服务本体（只消费）；后端 fun_compass_enable 接口；日期计算器；埋点。（zhiwei 生产部署已于 2026-11-09 完成，见 D12）

## Decisions

### D1 分包：插件 `subPackages` 目录扫描（vite.config.ts）

```ts
UniHelperPages({
  dts: 'src/uni-pages.d.ts',
  exclude: ['**/components/**/*.*'],
  subPackages: ['src/pages/fun'],   // 新增：扫描为子包根
})
```
- 插件机制（dist/index.mjs 实读）：subPackages 目录下的页面自动从主包 `pages` 剔除（`mergePageMetaData` 删除），root 由 `relative(src, dir)` 派生 → **`pages/fun`**，页 path 相对 root（`compass/index`…），与 PRD 路由前缀 `pages/fun/compass/*` 完全一致；`definePage` 照常生效。
- `pages/mine` 与首页等宿主页留主包；`pages.config.ts` 无需手写 pages 列表。
- **替代**：手写 `pages.config.subPackages[{root,pages}]` —— 要维护6条 path 且与文件扫描双源，易漂移。

### D2 修订轮文件结构

```
vite.config.ts                                  + subPackages 扫描
.env.development|.production|.staging           + VITE_ZHIWEI_BASE_URL（域名根；dev=http://127.0.0.1:3100）
src/config/tools.ts                             统一地址配置（唯一改动点，文件头含指引）
src/utils/funCompass/zhiwei.ts                  排盘客户端：calcRecommend() + timeToHourIndex() + localISO()
src/utils/funCompass/copy.ts                    隐私/关于文案如实化（见 D8）
src/utils/funCompass/bazi.ts                    ✗ 删除（lunar 排盘废止）
src/types/lunar-javascript.d.ts                 ✗ 删除
package.json                                    - lunar-javascript
src/store/funCompass/archives.ts                schema → {name,year,month,day,hour,minute,gender}
src/pages/fun/compass/bazi.vue                  表单（年月日+时间 picker+性别）+ 结果=top3 推荐
src/pages/fun/compass/simulator.vue             同上 + 桌高亮/改选/自检（结果区换推荐列表）
src/pages/fun/compass/archive.vue               卡片展示 生辰+时间+性别（删五行标签——本地不再算）
src/pages/index/index.vue                       fan-cta 下方突出入口卡（v-if=enabled → 模块首页）
src/pages/mine/index.vue                        保留（次级入口，门控不变）
```

### D3 档案 schema v2（无迁移）

`{id, name, year, month, day, hour, minute, gender, createdAt}`；`fun_compass_archives` 键不变。功能未上线、无存量用户数据，直接换 schema（开发期旧数据不兼容即弃，不写迁移）。编辑预填/列表展示同步改为「年月日 时间 · 性别」。

### D4 zhiwei 客户端与 8→4 聚合

```ts
// utils/funCompass/zhiwei.ts
export interface CalcRecommendation { rank: number; direction: string; score: number; reason: string }
export interface CalcResult { recommendations: CalcRecommendation[]; meta?: {...}; disclaimer?: string }

export function timeToHourIndex(hour: number): number
// 23→12(晚子), 0→0(早子), 其余 floor((h+1)/2)：1-2→1 … 21-22→11（与 iztro 0–12 对齐）

export function localISO(d = new Date()): string   // 本地时区偏移 ISO8601，喂 playTime
export async function calcRecommend(input: {date:'YYYY-MM-DD', hour:number, minute:number, gender:'male'|'female'}): Promise<CalcResult>
// uni.request POST TOOL_ENDPOINTS.zhiweiCalc（= ${VITE_ZHIWEI_BASE_URL}/api/calc），body {birthDate, birthHourIndex, gender, playTime}
```

**8→4 聚合（桌高亮）**：`seatScore[东|南|西|北]` 初值 0；每条 recommendation：direction 为正向 → `+score`；为间向 → 相邻两座各 `+score/2`（东北→北&东、东南→东&南、西南→南&西、西北→西&北）。取最高分座（并列按 东→南→西→北 首现），理由 = 归因于该座的 recommendations 的 reason 拼接（空 → 用 top1 reason；全空 → 随缘文案）。top3 列表仍按服务原样展示（含间向）。

**错误态**：`uni.request` fail / 非 2xx / 无 recommendations 字段 → toast + 结果区可重试（保留表单输入）。**断网排盘不再成立**（后端计算），原离线 spec 场景已删除。

**替代（弃）**：
- Go 网关反代 zhiwei —— 已采用：生产网关把 `/api/calc` 路由到 zhiwei，与 `/api/app/*` 同域不同路径（2026-11-09 实测）。
- 前端只取正向座（丢间向信息）—— 聚合更充分利用服务评分。

### D5 罗盘跨端（不变）

MP：`wx.onCompassChange` onShow/onHide + 无读数权限引导，零定位 API；H5：touch+mouse 拖动 +「模拟」角标。已实现，本轮不动。

### D6 配置缝（不变）

`useFunCompassConfig()` 默认 true；**修订轮新增消费方：首页突出入口卡 `v-if="enabled"`**（与我的分组同源，风控关闭时两处一起消失）。

### D7 结果区 UI（替代原四柱/五行/喜用）

- bazi/simulator 结果卡：**top3 推荐列表**（rank 徽标 + 方位 Serif 大字 + score 用 Oswald + reason 小字），替代四柱 chips/五行点阵/喜用结论；「参考」朱红印保留在推荐座行。
- 桌高亮逻辑不变（rec/sel/stale 类与复核行文案沿用）；喜用土随缘分支由「空推荐/无正分」分支替代。
- 桌下/详情卡的五行属相标签（东座·属木）删除——数据源没了；详情卡改显「推荐 · 理由摘要」或仅环境自检。
- 视觉 token、牌片、罗盘纹、免责条全部沿用（骨白/黄铜/漆红/ink）。

### D8 文案如实化（copy.ts）

| 常量 | 旧 | 新 |
|---|---|---|
| `LOCAL_ONLY` | 仅存本机，不会上传 | 档案仅存本机 · 排盘即算即走 |
| `LOCAL_ONLY_LIST` | 仅保存在本机 · 不会上传 | 档案仅存本机 · 不关联账号 |
| `MINE_GROUP_FOOT` | 民俗趣味 · 数据只存本机 | 民俗趣味 · 档案只存本机 |
| about 第 3 段 | …不会上传服务器，不会和记账数据打通… | 本模块录入的生辰档案仅保存在当前设备本地；排盘计算经后端趣味服务完成，出生年月日时间与性别仅用于本次计算，不保存、不与您的记账数据或记账业务关联。 |

`HOME_LOCAL_DOT`（生辰档案仅存本机，不上传）语义仍真，保留。两句免责、前置提示、黑名单不动。

### D9 首页突出入口（C2）

fan-cta（我要开房/扫码进房）下方、chips 行上方，新增**全宽入口卡**：骨白或黄铜描边横卡「🧭 趣味方位 · 罗盘选座」+ 小字「生辰 · 罗盘 · 选座小彩蛋」+ 右箭头；`v-if="enabled"`；点击 → `/pages/fun/compass/index`。视觉权重低于主 CTA、高于普通 chips（突出但不抢开房主线）；类名前缀 `funentry-` 避撞名。

### D10 时间采集控件

`<picker type="time">` 包胶囊展示（`时:分`，如 `14:30`）——MP/H5 双端原生滚轮；change 拆 hour/minute 入表单 state 与 store。年月日仍用数字输入（沿用）。性别 = 两枚 chip（男/女，默认男）。

### D11 选座 ↔ 罗盘联动（2026-11-06 第二轮修订）

**目标**：罗盘为选座服务——排座结果能带到罗盘上「对向入座」。

- **共享推荐状态**：新 store `store/funCompass/recommendation.ts`，$id=`fun_compass_rec`（persist 插件自动落盘，与档案键分离）。State：`{ direction: SeatDirection | null, reasons: string[], birthLabel: string, updatedAt: number }`。模拟器**每次排盘成功**写入（含空推荐→direction 置 null）。
- **模拟器侧**：详情卡在有推荐座时展示「用罗盘对准 X 座 →」入口 → `router.push` 罗盘页（推荐已在 compute 时写入，跳转不重复写）。
- **罗盘侧**（读 `fun_compass_rec`）：
  - 读数 chip 下方**目标卡**：`目标 · 北座（0°）` + 夹角 `差 N°`；夹角 = `((target − heading + 540) % 360) − 180`（有符号 −180…180）。
  - **对准判定**：`|夹角| ≤ 15°` → 已对准态：目标卡黄铜高亮 + 「已对准 · 可入座」；读数 chip 同步描边。
  - **表盘标记**：目标方位字标（如「北」）染黄铜 + 其外侧一枚黄铜小三角（挂在旋转盘面上，随盘转；转到顶部固定索引下 = 对准，与夹角判定同源）。
  - **空态**：无推荐 → 目标卡换成「还没排个座？去排座 →」按钮 → 模拟器。
  - 目标度数表：`东90 南180 西270 北0`；MP 磁力计与 H5 拖动读数同样驱动判定。
- **替代（弃）**：罗盘页内嵌排盘表单——两页职责重叠、屏内过载；推荐只读不落盘——跨页跳转即丢。

### D12 生产接入与配置（2026-11-09）

- **地址**：生产 `https://alexcloud.vate.ren/api/calc`（网关路由，已实测：推荐正常返回、缺参 400）。
- **唯一改动点**：`src/config/tools.ts`（读 `VITE_ZHIWEI_BASE_URL`、去尾斜杠、本地兜底 `127.0.0.1:3100`、`TOOL_ENDPOINTS` 汇总）；按环境的具体值在 `.env.development/.staging/.production`，**只填域名根**，客户端拼 `/api/calc`；改 env 后需重新 build。

## Risks / Trade-offs

- [PRD 红线被覆盖] 首页入口 + 生辰后端化有审核/隐私问询风险 → proposal 顶部备案，需求方已明确指示；文案如实化降低隐私风险。
- [zhiwei 可用性] 排盘依赖常驻服务，挂了即不可用 → 错误态可重试；dev 用 PORT=3100 本地起；生产必须部署（Docker/进程守护）。
- [~~小程序合法域名~~ 已解决 2026-11-09] 生产实测 `https://alexcloud.vate.ren/api/calc` 可用（与主后端同域，走既有白名单）；地址集中在 `src/config/tools.ts` + `.env.*`，改构建期变量需重新 build。
- [playTime 时区] 服务按传入偏移解释时间 → 客户端一律带设备本地偏移（localISO），不传裸 Z。
- [间向聚合启发式] 半分归邻座是近似（非服务原生语义）→ 桌高亮标「推荐」、列表保留服务原样 top3，理由可溯源；后续可让 zhiwei 直出四座分。
- [档案 schema 变更] 开发期旧数据不兼容 → 未上线直接弃（D3）。
- [UnoCSS 撞名] 新增 `funentry-`/`fcn-` 等前缀类，落码前对照 34 类清单。

## Migration Plan

纯增量 + 一处 schema 换代（无存量）；回滚 = revert 本轮提交（分包配置、入口卡、客户端、文案），首版功能不受影响。zhiwei 服务独立部署，可单独启停。

## 验证方案

1. zhiwei：`npm test`（37/37）+ `PORT=3100 node src/index.js` 常驻 + `curl POST /api/calc` 冒烟（含晚子/性别分支）。
2. 前端：`pnpm lint` + `pnpm type-check` + `pnpm build:h5` + **`pnpm build:mp-weixin`**（核对产物 pages.json subPackages）。
3. 黑名单 grep（同 5.1）+ 两句免责逐字复核。
4. H5 e2e（dev:h5 + 本地 zhiwei）：首页入口→模块→排盘（时间/性别）→top3+高亮→改选降态→存档→关于清除隔离→开关双入口隐藏→罗盘模拟。
5. 手动清单（微信开发者工具）：分包真机加载、首页入口观感、磁力计权限引导、Network 面板核对仅 /api/calc 携带出生字段。

## Open Questions

- 生产 `VITE_ZHIWEI_BASE_URL` 具体前缀（网关路径）→ 部署时与运维定，不阻塞开发（dev 值已定）。
