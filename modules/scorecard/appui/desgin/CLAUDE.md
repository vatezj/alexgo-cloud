# 项目说明

本项目通过 Figma MCP 操作 Figma 来设计 UI。

## 固定 Figma 文件

默认操作以下 Figma 文件，**除非用户明确给出新的 URL，否则不要再询问地址**，直接使用：

- URL: https://www.figma.com/design/dHMgzmrrWJ8ztjWytPXdIW/Untitled?node-id=0-1&t=dWyyr1Xpvlr84l9a-1
- fileKey: `dHMgzmrrWJ8ztjWytPXdIW`
- 文件名: Untitled
- 默认节点: `0-1`（即 `0:1`，页面根节点）

## 使用要点

- 调用 Figma 工具时 fileKey 一律用 `dHMgzmrrWJ8ztjWytPXdIW`
- 写入设计前先加载 `/figma-use` skill；生成页面布局用 `/figma-generate-design`
- 回答使用中文

## 当前项目：打牌记账 App（进行中）

进度账本：`/tmp/design-system-state-paijv.json`（组件/样式/页面 ID 都在里面）

**已完成（2026-10-09）**：
- 设计 token：5 个变量集合 49 变量（Primitives/Color/Spacing/Radius/Size），语义色走别名
- 文字样式 10 个（Oswald 金额阶梯 + Noto Sans SC）、效果样式 3 个（shadow/card、glow/win、glow/lose）
- 组件页 🧩：ScoreButton、Button、BaseScoreControl、PlayerCard、SettleCard、TabBar（均已视觉校验）
- 屏幕页 📱：`01 首页` 已完成（node 19:2）

**待办（脚本已备好，见 `figma-scripts/`）**：开局设置/记分页/结算页/战绩统计 4 屏 + 🎨 Foundations 规范页 + 最终截图校验 → 按 `figma-scripts/README.md` 顺序执行，共约 8 次调用

**设计规范**：完整规范见 [`design-rules.md`](design-rules.md)（配色/字体/间距/组件/动效/命名全量规则）

**重设计方向（2026-10-09，creative-director 流程产出）**：
- Top-3 方向提案 → [`design-proposal.md`](design-proposal.md)（推荐「牌面」8.35 分）
- 推荐方向详案 → [`design-onepager-paimian.md`](design-onepager-paimian.md)（token 草案 + 5 页逐屏）
- **HIG 审查** → [`hig-review.md`](hig-review.md)（2 必改已落进 HTML 稿：小字红 #B32D18、底部安全区 34pt）
- **HTML 视觉稿** → [`preview/paimian.html`](preview/paimian.html)（5 屏高保真，浏览器打开 `http://localhost:8931/paimian.html` 可看，需 `python3 -m http.server 8931`；截图 [`preview/paimian-final.png`](preview/paimian-final.png)）
- **小程序 6 屏（真实产品，png 参考图重设计）** → [`preview/miniapp.html`](preview/miniapp.html)（首页/房间/邀请弹框/支出/结算/使用手册，牌面暗色 + 合规横幅 + 茶饭/语音/广告位规则；截图 [`preview/miniapp-final.png`](preview/miniapp-final.png)）
- **突破版（用户要求"不参考图片 UI、要突破"）** → [`preview/miniapp-breakout.html`](preview/miniapp-breakout.html)：**12 屏**——俯视牌桌房间页（顶部头像座位列表·人多左右滚动·流水全量记录）、扇形 CTA 首页、扇牌二维码邀请、筹码加减档支出、竖排开牌结算、花色卡手册、战绩(左滑流水/排行/删除)、好友排名(左滑删除拉黑+参与开关)、茶饭设置弹层、头像弹窗(语音5档/退出)、自定义统计(开发中)；截图 [`preview/breakout-final.png`](preview/breakout-final.png)。**此版为小程序 UI 最终方向**
- **4 套配色方案** → [`preview/themes.html`](preview/themes.html)（运行时 fetch 复用 12 屏，零复制；①墨金牌桌/②深海筹码/③红黑赌场/④青瓷茶室；合规红+红中印全局锁定、赢色与 chrome 解耦；总览图 [`preview/themes-final.png`](preview/themes-final.png)）
- **Figma 进场计划** → [`FIGMA-PLAN.md`](FIGMA-PLAN.md)（决策：牌面进新文件，v1 封存；预算 11–14 次调用）
- 注意：**牌面与 v1（墨绿+暖金）是两套并行方案**；v1 已部分落地 Figma，牌面未进（等额度）

**阻塞**：Figma MCP Starter 套餐 + View 席位 = **每月 20 次工具调用**，2026-10-09 已用尽。
恢复方式：升级到 Dev/Full 席位（200 次/天），或等月度重置后继续。

## Open Design MCP（第二设计通道，2026-11-05 启用）

Figma 额度受限期间，用本机 **Open Design.app** 的 MCP 做设计（用户口径：opendesign-mcp）：

- 连接方式：`node "/Applications/Open Design.app/Contents/Resources/app/prebundled/daemon/daemon-cli.mjs" mcp --daemon-url http://127.0.0.1:<port>`（stdio JSON-RPC；端口看 `lsof -nP -iTCP -sTCP:LISTEN -a -p <sidecar pid>`，app 需在运行）
- 核心工具：`create_project` / `start_run`（委派 OD 自己的 agent 生成，5–30 分钟，轮询 `get_run`，**期间无新文件是正常思考，勿取消勿代写**）/ `get_run`（previewUrl、artifactRef）/ `get_artifact` / `write_file`
- 已登录 vela 云端；prompt 必须自带全部设计 token（OD agent 读不到本仓库文件）

## 趣味方位 · 座位模拟器（2026-11-05 完成，v3 = 填生辰排八字求座）

- PRD：[`xuanzuo.md`](xuanzuo.md)（模块全量 PRD）；核心流程按用户要求 = **填生日 → 排八字 → 按五行出推荐座位**（东=木 南=火 西=金 北=水，喜用→推荐）
- OD 项目：`project-f851`「趣味方位-座位模拟器」，artifact `seat-simulator.html`（run1 单屏初稿 33min → run2 视觉精修 13min → run3 双屏流程重构 ~68min，中途内部 agent 报过一次 Write 参数错误自愈）
- 产出：[`preview/xuanzuo-seat-simulator.html`](preview/xuanzuo-seat-simulator.html)（**v3 终稿：并排两屏 01 填写生辰 / 02 出结果**，390×844×2）+ [`preview/xuanzuo-final.png`](preview/xuanzuo-final.png)；v1/v2 存档 `preview/xuanzuo-seat-simulator-v1|-v2.html` + `xuanzuo-v1|-v2.png`
- v3 结构：左屏生辰表单（年/月/日/12时辰 chip、「仅存本机不上传」、从档案选择入口、CTA「排八字·求个座」）；右屏摘要行+修改、四柱 chips（庚辰/甲申/戊午/己未）、五行点阵、「喜用为木·东座属木」+朱红推荐印、桌上推荐座黄铜描边+推荐标、卡片属木/自检、CTA「换个推荐/重置」
- 交互态机（实测）：`.sel` 当前选中 / `.rec.stale` 推荐被手动改选后降态保留；点座卡片联动（西→西座·属金·已手动改选，caption 同步）
- 风格：牌面方向（ink #14120F + 骨白 #F2EDE3 + 黄铜 #C9A24B），骨白牌片、罗盘刻度+方位水印、红中印式推荐印、Serif 标题 + Oswald 干支数字
- 合规实测：禁词 0 命中（财运/收益/进账/生意/回款/财源/利于赚钱/旺收入/赚钱）、两句免责逐字 ×2 屏、OD 语法校验 pass
- 备注：run3 `endedWithUnfinishedWork=true` 但 artifact 完整、语法 pass、交互可用

### 模块其余 5 页（2026-11-05 并行 3 run 完成，六屏齐了）

- **3 个并行 OD 项目**（独立 project 互不抢会话，总耗时 ≈ 最慢一路 ~45min）：
  - `project-2131` 趣味方位-首页与关于 → `index-about.html`（17min）：01 模块首页（📌提示条逐字 + 生辰档案/实时罗盘/座位模拟器 三骨白卡 + 关于入口）/ 02 关于与免责（PRD 免责全文逐字四段 + 朱红「清除本工具全部档案」描边钮 + 只清本机数据 caption）
  - `project-b713` 趣味方位-生辰与档案 → `bazi-archive.html`：01 生辰排盘（表单+四柱+五行点阵+喜用结论+存入档案）/ 02 档案列表（老王/小林/阿珍 三档，五行标签 chips，**删除确认态**画在小林卡上，+新增档案 CTA）
  - `project-7e8d` 趣味方位-罗盘 → `compass.html`：单屏大罗盘（360° 刻度+四正四隅+指针+342°·西北 读数 chip+方位水印）、四方位映射条、磁力计权限提示条（不获取定位）、校准罗盘 CTA
- 产出（含截图）：`preview/index-about.{html,png}`、`preview/bazi-archive.{html,png}`、`preview/compass.{html,png}`
- 合规实测：三稿全部禁词 0、免责逐字（A/B ×2 屏、C ×1 屏）、about 全文四段逐字、OD 语法 pass
- 踩坑：`create_project` 后立刻 `start_run` 可能报 `no project matches`（索引延迟）→ 隔 1-2 秒重试即可；并行 run 的 runId 要在创建当次就落盘，别等批次结束

### 实现轮修订（2026-11-06 用户四点，覆盖 PRD 两条红线——已备案于 openspec proposal）

1. 生辰录入 = 年月日 + **时间(时:分) picker** + 性别（zhiwei 接口要 gender）；12 时辰 chip 废止
2. **首页突出入口**（fan-cta 下方黄铜描边卡）→ 一键进模块；【我的】折叠组保留为次级（PRD「禁止首页入口」被用户明确覆盖）
3. 模块六页进**分包** root=`pages/fun`（vite 插件 `subPackages: ['src/pages/fun']`），主包只留宿主页
4. **排盘计算移到后端** zhiwei 服务（`plugin/zhiwei`，POST /api/calc，紫微+奇门+评分，37 测试全过；本地 `PORT=3100`）；前端删 lunar-javascript；结果区改 top3 推荐列表 + 8→4 聚合桌高亮（PRD「生辰不上传」被覆盖——文案已如实化，档案仍本机）

- 变更工件：`uniapp/openspec/changes/fun-compass/`（spec/design/tasks 已同步修订；进度 27/29，剩 devtools 手工2项）
- 环境变量：`VITE_ZHIWEI_BASE_URL`（dev=127.0.0.1:3100）
- **生产已部署（2026-11-09 实测）**：`https://alexcloud.vate.ren/api/calc`（网关路由，与主后端同域）；**改地址唯一入口 = `src/config/tools.ts`（文件头有指引）+ `.env.*`（只填域名根，改后需重新 build）**
5. **罗盘对向联动（同日二轮）**：模拟器排座写共享推荐 `fun_compass_rec` → 罗盘页目标卡（差 N° / \|夹角\|≤15° 已对准·可入座）+ 目标字标黄铜+盘面菱标；空态「去排座」回模拟器（openspec §7，31/33）

### UI 融合稿（2026-11-05，run D = `53813783`，复用 project-2131）

- 产出：[`preview/integration.html`](preview/integration.html) + `integration.png`（宿主首页参照 `host-home-ref.png`）
- 融入方案（PRD 入口红线落地）：宿主没有【我的】页 → 融合稿**新建**之：01 首页身份牌右上新增「⚙ 我的」小钮（点头像=改资料的现有交互不动）→ 02【我的】页（宿主行卡语言：修改资料/使用手册/客服 + **更多趣味小工具折叠组**：趣味方位参考·新 + 日期计算器灰占位，脚注含 fun_compass_enable 关闭整组隐藏）→ 03 承接模块首页（复用 index-about 的屏，仅加返回标注）
- 画框底部 FLOW 注释：enable 开关 → 路由 pages/fun/compass/* → 返回回我的 → fun_compass_archives 与账本存储隔离
- 合规实测：禁词 0、两句免责逐字、三屏要素齐；检查脚本教训：**比对前双方都要去空白**（带空格的关键词匹配去空白文本会假阴性）

**Figma 踩坑记录**：
- `createAutoLayout()` 默认白色填充，结构容器必须 `fills = []`
- Starter 计划最多 3 页（已用满：Screens/Components/Foundations）
- `appendChild()` 无返回值，不能链式设置属性
- 实例内文字覆盖：先 loadFont，再 `findAllWithCriteria({types:['TEXT']})[0].characters = ...`
