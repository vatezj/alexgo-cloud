# Proposal: fun-minigames-24-huarong

## Why

记账小程序的用户在等账/休息间隙缺少轻量消遣；【我的】页「更多趣味小工具」组已有工资计算器等工具的先例，但没有纯前端小游戏。本变更新增**算24** 与 **华容道** 两个彩蛋级小游戏分包，增强停留与趣味性，且不引入任何后端接口。

## What Changes

- 新增独立分包 `pages/game24`：算24 点选答题页——随机 4 张 1–13 点数牌，发牌前本地穷举验证有解（无解重摇）；点选数牌与 `+ − × ÷ ( )` 拼式提交求值判定 =24；辅助「看答案」「换一题」；除法用有理数精确比较避免浮点误差；零存储、零后端。
- 新增独立分包 `pages/huarong`：华容道经典「横刀立马」一局（4×5 棋盘）；点中方块高亮可移动方向、点击移动并计步；重置回初始局面；最佳步数仅存本地 storage；不做关卡/计时/解法提示，零后端。
- 【我的】页「更多趣味小工具」折叠组追加两行入口：**算24** → `/pages/game24/index`、**华容道** → `/pages/huarong/index`；登录门与工资计算器行一致（无登录门）。
- `vite.config.ts` `subPackages` 扫描根数组追加 `src/pages/game24`、`src/pages/huarong`，路由由 `@uni-helper/vite-plugin-uni-pages` 自动生成。
- 两页均 `navigationStyle: custom` 自绘导航，沿用墨金主题；wot-ui 组件按需（toast/dialog 判定反馈）。

**非目标**：多人对战、排行榜、成绩落库；华容道关卡/每日局/计时榜；算24 计时计分与表达式键盘输入；改动首页（index）或现有 `pages/fun` 分包结构；任何 API 调用与后端代码。

**验收标准**：`pnpm lint` + `pnpm type-check` + `build:h5` + `build:mp-weixin` 全绿；24 点判定/求值器与华容道移动合法性 TDD 测试通过；我的页两入口点击进页、返回落回【我的】页；`openspec validate fun-minigames-24-huarong` 通过。

### 跨端差异（H5 vs 微信小程序）

- 两游戏逻辑纯前端本地运算，双端行为一致；仅 storage 键与路由走 uni-app 统一 API（`uni.setStorageSync` / `uni.navigateTo`），无平台分支。
- 分包注册靠 `vite.config.ts` `subPackages`，H5 与 MP-WEIXIN 均由构建生成路由；MP-WEIXIN 产物须含两个新 subPackages。
- 不用仅浏览器可用的 API（无 `localStorage`/`window.*` 直用）。

## Capabilities

### New Capabilities

- `fun-game24`: 算24 小游戏可观察行为——发牌有解保障、点选拼式提交判定、看答案/换一题、有理数精确求值、零存储。
- `fun-huarong`: 华容道小游戏可观察行为——横刀立马初始局、点选方向高亮移动、计步与重置、最佳步数本地存储、通关判定。
- `mine-fun-entry`: 【我的】页「更多趣味小工具」折叠组对算24、华容道两行入口的收纳（路由、登录门）。

### Modified Capabilities

（无——`home` 规格不涉及本次改动；工资计算器等既有入口行为不变。）

## Impact

- **代码**：新增 `src/pages/game24/*`、`src/pages/huarong/*`（页面 + 核心逻辑 + 测试）；【我的】页（`src/pages/mine` 或对应文件）折叠组追加两行；`vite.config.ts` 两行扫描根注册。
- **路由/构建**：pages.json 由插件自动生成两个新分包；无手工维护。
- **数据/网络**：无新增请求；华容道最佳步数用本地 storage 单键。
- **依赖**：无新增 npm 依赖。
