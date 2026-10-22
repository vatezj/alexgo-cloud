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

**Figma 踩坑记录**：
- `createAutoLayout()` 默认白色填充，结构容器必须 `fills = []`
- Starter 计划最多 3 页（已用满：Screens/Components/Foundations）
- `appendChild()` 无返回值，不能链式设置属性
- 实例内文字覆盖：先 loadFont，再 `findAllWithCriteria({types:['TEXT']})[0].characters = ...`
