# Tasks: fun-compass

## 1. 依赖与文案基座

- [x] 1.1 `pnpm add lunar-javascript`；若无 TS 类型则补 `src/types/lunar-javascript.d.ts`（`declare module 'lunar-javascript'`），`pnpm type-check` 通过
- [x] 1.2 新建 `src/utils/funCompass/copy.ts`：导出两句免责（逐字）、关于页全文四段（逐字）、模块首页前置提示（逐字）、方位映射小字、座位模拟器备注句、黑名单词数组（9 词）；文件内不含任何其他涉钱文案
- [x] 1.3 新建 `src/utils/funCompass/bazi.ts`：`calcBazi({year,month,day,hourIndex})` → `{siZhu,wuXing,xiYong,seat}`（lunar-javascript 取四柱；时辰代表小时 `(index*2)%24`；喜用=计数最少、并列按木火土金水；映射 东木/南火/西金/北水、土→`seat:null`）；纯函数无 IO
- [x] 1.4 排盘抽样自检（node 脚本一次性运行，不入库）：3 组生辰两次调用结果一致；给定日期四柱与万年历一致；喜用土用例返回 `seat:null` —— 结果贴进任务备注后删脚本（11/11 PASS：官方夹具 2005-12-23 辰时=乙酉戊子辛巳壬辰、1988-02-02 亥时=丁卯癸丑丁亥辛亥 逐字对上；1996-08-15 未时=丙子丙申甲申辛未；确定性×3；五行合计=8；土用例 1990-01-01→seat=null；hourLabel/代表小时正确。脚本已删）

## 2. 存储与配置

- [x] 2.1 新建 `src/store/funCompass/archives.ts`（Pinia）：`archives: FunCompassArchive[]` + `load/save`（`uni.setStorageSync('fun_compass_archives', …)`）、`add/update/remove/clearAll`；`clearAll` 只清该键；类型 `FunCompassArchive = { id, name, year, month, day, hourIndex, createdAt }`
- [x] 2.2 新建 `src/composables/funCompass/useFunCompassConfig.ts`：模块级 `enabled = ref(true)`，导出 `useFunCompassConfig()`；注释标明后端接入只改此文件（`!== false` 语义、失败保持 true）
- [x] 2.3 `pnpm lint` + `pnpm type-check` 通过（此后每任务同要求，不重复列出）

## 3. 我的页与首页入口

- [x] 3.1 新建 `src/pages/mine/index.vue`（`definePage` 自绘 nav，`fc-` 类，token 无硬编码）：迷你身份牌行（点开 ProfileGate edit）+ 分组行 修改资料 / 使用手册（`/pages/manual/index`）/ 客服 / 获取头像昵称（后两行先 grep 宿主现有实现，复用其跨端分支，不新造）+「更多趣味小工具」分组（`v-if="enabled"`：趣味方位参考行→`/pages/fun/compass/index`，带「新」标；日期计算器行 dim「敬请期待」；脚注三段文案）+ 版本行（package.json version）
- [x] 3.2 修改 `src/pages/index/index.vue`：`.idcard` 右上加「我的」小钮（token 描边胶囊，`fc-`/`idcard-` 前缀类）→ `router.push('/pages/mine/index')`；头像 click 与既有布局零改动
- [ ] 3.3 手动回归：首页点头像仍开修改资料、点「我的」进我的页、首页其余视觉无位移（微信开发者工具）

## 4. 模块六页

- [x] 4.1 `src/pages/fun/compass/index.vue` 模块首页：返回→`/pages/mine/index`（栈深>1 用 navigateBack 兜底 replace）、前置提示条（copy.ts 逐字）、三张入口卡（生辰档案→bazi / 实时罗盘→compass / 座位模拟器→simulator）、「关于与免责」行、只存本机小字、底部两句免责
- [x] 4.2 `src/pages/fun/compass/bazi.vue` 生辰排盘：年/月/日输入 + 12 时辰 chip + 称呼行 + 「仅存本机」隐私行 + CTA「排八字 · 看五行」→ 结果卡（四柱 chips/五行点阵/喜用结论+朱红参考印/映射小字）+「存入档案」写入 store；空名兜底
- [x] 4.3 `src/pages/fun/compass/archive.vue` 档案列表：store 列表卡（称呼+生辰+五行标签）、行内编辑/删除（删除弹确认条「只删本机数据」）、空态行动指引、CTA「+ 新增档案」→ bazi 页（edit 模式带 id 预填）、底部免责
- [x] 4.4 `src/pages/fun/compass/simulator.vue` 座位模拟器：生辰表单（复用 4.2 输入模式，含从档案选择入口读 store）→ CTA「排八字 · 求个座」→ 四柱/五行/喜用结果行 + 俯视桌四座（选中 `.sel`、推荐 `.rec`、改选后 `.rec.stale` 且卡片/复核行含「已手动改选」）+ 座位详情卡与环境自检 4 项 +「换个推荐/重置」+ 底部免责；喜用土用例出随缘文案
- [x] 4.5 `src/pages/fun/compass/compass.vue` 实时罗盘：360° 表盘 + 读数 chip（度数+8 方位）+ 权限引导条（`#ifdef MP-WEIXIN`：onShow/onHide 注册注销 `wx.onCompassChange`，不碰定位；未出读数显示「去开启」可重试）+ H5 `pointer` 拖动模拟与「模拟」角标 + 四方位映射条 + 校准/说明按钮 + 只存本机小字 + 底部免责
- [x] 4.6 `src/pages/fun/compass/about.vue` 关于与免责：copy.ts 全文四段逐字排版 + 朱红描边「清除本工具全部档案」→ 确认弹层 → `clearAll()` + toast「已清除，账本数据不受影响」+ 底部两句免责
- [x] 4.7 `pnpm lint` + `pnpm type-check` + `pnpm build:h5` 全绿（路由由插件扫描生成，不手改 pages.json）

## 5. 验证

- [x] 5.1 黑名单检索：grep 9 词于 `src/pages/fun/compass`、`src/pages/mine`、`src/utils/funCompass` → 0 命中（「收支/投资」仅允许出现在 copy.ts 免责常量）；两句免责在六页各恰一次出现路径确认
- [ ] 5.2 微信开发者工具手动清单（2026-11-06 按修订 spec）：临时改 config 默认 false 验证首页入口+我的分组双隐藏后还原；关于页清除后账本/战绩完好；六页底部免责逐字；首页突出入口一键达模块、我的折叠组次级可达；模块返回落进入前页、子页栈内回退；排盘（时间+性别→top3+高亮）/罗盘未授权引导/选座改选可用；**罗盘对向**（排座→目标卡→转到对准/偏差显示/空态去排座）；Network 面板仅 /api/calc 携带出生字段
- [x] 5.3 H5 手动清单：`pnpm dev:h5` 打开我的页→模块链路全通、罗盘出「模拟」角标可拖动、免责完整
- [x] 5.4 `openspec validate fun-compass` 通过；按 `.agents/skills/openspec-verify-change` 核对规格-任务-实现一致

## 6. 修订轮（2026-11-06 用户四点：时间输入 / 首页入口 / 分包 / 后端排盘）

> 说明：1.3/1.4（lunar 前端排盘）、4.2/4.4 的四柱五行喜用结果区、档案五行标签**被本轮取代**（历史勾选保留作首版记录）；其余首版任务不受影响。

- [x] 6.1 分包：`vite.config.ts` 的 UniHelperPages 加 `subPackages: ['src/pages/fun']`；核对生成 pages.json 含 `subPackages` root=`pages/fun` 六页、主包 pages 无 fun 页；`build:h5` + `build:mp-weixin` 双通过
- [x] 6.2 zhiwei 客户端：新建 `src/utils/funCompass/zhiwei.ts`（`calcRecommend` 走 `uni.request POST ${VITE_ZHIWEI_BASE_URL}/api/calc`；`timeToHourIndex` 换算表 0–12；`localISO` 带本地偏移；类型 CalcResult/CalcRecommendation）+ 三份 `.env*` 加 `VITE_ZHIWEI_BASE_URL`（dev=`http://127.0.0.1:3100`）
- [x] 6.3 zhiwei 服务本地常驻：`PORT=3100 node src/index.js` 后台起；`curl POST /api/calc` 冒烟（正常 + 晚子 23 点 + female 分支）；`npm test` 37/37 复跑
- [x] 6.4 表单改造：bazi.vue + simulator.vue 改「年/月/日 + 时间 picker(时:分) + 性别 chip(默认男)」，删 12 时辰 chips；`store/funCompass/archives.ts` schema → `{name,year,month,day,hour,minute,gender}`；archive.vue 卡片/编辑预填同步（删 calcBazi 五行标签）
- [x] 6.5 结果改造：bazi/simulator 结果区 → zhiwei top3 推荐列表（rank/方位/分数/理由）+ 8→4 聚合桌高亮（间向半分归邻座、并列东南西北序）+ 空推荐随缘文案 + 失败可重试；删 `utils/funCompass/bazi.ts`、`src/types/lunar-javascript.d.ts`，`pnpm remove lunar-javascript`
- [x] 6.6 首页突出入口：index.vue fan-cta 下方全宽入口卡（`funentry-` 前缀类，`v-if="enabled"` → `/pages/fun/compass/index`）；我的折叠组保留
- [x] 6.7 文案如实化：copy.ts `LOCAL_ONLY/LOCAL_ONLY_LIST/MINE_GROUP_FOOT` 改档案例口径、about 第 3 段改「档案本机 + 排盘经后端仅用于本次计算」（design D8 表）；模块首页/表单隐私行同步
- [x] 6.8 修订轮验证：lint + type-check + build:h5 + build:mp-weixin；黑名单 grep 复检 0 命中；两句免责逐字；H5 e2e（首页入口→时间性别排盘→top3 高亮→改选→存档→清除隔离→开关双隐藏→罗盘模拟）；`openspec validate` 复跑

## 7. 罗盘对向联动（2026-11-06 第二轮：选座与罗盘打通）

- [x] 7.1 新建 `src/store/funCompass/recommendation.ts`（$id=`fun_compass_rec`，persist 落盘）：`{direction: SeatDirection|null, reasons: string[], birthLabel, updatedAt}` + `setRecommendation/clear`
- [x] 7.2 模拟器接入：排盘成功后写入共享推荐（含空推荐置 null）；详情卡有推荐时展示「用罗盘对准 X 座 →」→ 跳罗盘页
- [x] 7.3 罗盘页对向：读数下目标卡（目标座+度数+差 N°）；`|夹角|≤15°` → 已对准态（黄铜高亮+「已对准 · 可入座」）；表盘目标字标染黄铜+外侧黄铜三角随盘转；无推荐 →「去排个座」按钮；东90/南180/西270/北0 度数表，MP/H5 读数同驱动
- [x] 7.4 验证：lint + type-check + build:h5 + build:mp-weixin；H5 e2e（排座→罗盘见目标→拖动到对准态→空态引导）；`openspec validate` 复跑

## 8. 生产接入（2026-11-09 八卦 API 部署）

- [x] 8.1 新建 `src/config/tools.ts` 统一配置（读 env + 去尾斜杠 + 本地兜底 + TOOL_ENDPOINTS 汇总，文件头含修改指引）；`zhiwei.ts` 改为消费 `TOOL_ENDPOINTS.zhiweiCalc`
- [x] 8.2 `.env.production` 修正为 `https://alexcloud.vate.ren`（原 `/zhiwei` 前缀是错的）；staging 同步「域名根」约定；生产 `POST https://alexcloud.vate.ren/api/calc` 实测通过（推荐返回 + 缺参 400）
- [x] 8.3 lint + type-check + build:h5 + build:mp-weixin 全绿；产物含根域名、无 `zhiwei/api/calc` 错误拼接
