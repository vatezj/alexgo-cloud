# Design: fun-minigames-24-huarong

## Context

需求来源：`docs/superpowers/specs/2026-11-23-scorecard-minigames-design.md`。现状：小程序已有 `pages/fun`（工资计算器等）、`pages/starfuyu` 两个分包，`vite.config.ts:37` 的 `subPackages` 数组注册分包扫描根，`@uni-helper/vite-plugin-uni-pages` 自动生成 pages.json；【我的】页 `src/pages/mine/index.vue` 已有「更多趣味小工具」折叠组（登录门 + 工资/星座/日期计算器行）。墨金主题与自绘导航为既有约定。动机与范围见 proposal.md。

## Goals / Non-Goals

**Goals:**

- 算24、华容道核心逻辑（求值/24 判定、棋盘移动合法性）纯函数化并 TDD 覆盖。
- 两个独立分包按既有 `subPackages` 模式注册，路由自动生成。
- 我的页入口沿用同组惯例，最小改动。

**Non-Goals:**

- 后端、API、成绩落库、排行榜（见 proposal 非目标）。
- 改造 `pages/fun` 分包或首页结构。
- 华容道求解器/提示、算24 键盘输入。

## Decisions

| 决策点 | 选择 | 备选与理由 |
|---|---|---|
| 分包结构 | 两个独立分包 `src/pages/game24`、`src/pages/huarong` | 合并进 `pages/fun` 被否：fun 是工资计算器地盘，且需求明确各占一分包；独立分包体积互不牵连 |
| 分包注册 | `vite.config.ts` `subPackages` 追加两行扫描根 | 手工写 pages.json 被否：插件单一来源，手工维护产生重复源 |
| 算24 交互 | 点选拼式（数牌 + `+ − × ÷ ( )` 按钮） | 键盘输入被否：需手写解析器、手机切键盘体验差；点选通道受控判定可靠 |
| 24 判定求值 | 有理数（分数）精确比较 ×=24；发牌前穷举验证有解 | 浮点比较被否：误差可误判；穷举在 1–13×4 规模下本地开销可忽略 |
| 华容道交互 | 点中方块高亮可移动方向、点击方向移动 | 直接滑动：实现等价，点选在小程序端更稳 |
| 成绩 | 华容道本地 storage 单键最佳步数；算24 零存储 | 落库被否（彩蛋级，零后端） |
| 入口归属 | 新 capability `mine-fun-entry`（delta spec） | 归入 `fun-salary` 被否：其入口 requirement 只约束工资行；`home` 不涉及我的页 |

**代码影响**：

- 新增 `src/pages/game24/`、`src/pages/huarong/`（页面、核心逻辑模块、测试）。
- `src/pages/mine/index.vue` 折叠组追加两行（模板 + 跳转函数，预计 <30 行）。
- `vite.config.ts` `subPackages` 追加 `'src/pages/game24'`、`'src/pages/huarong'`。
- 不改任何现有 API/store/组件契约。

**兼容性**：

- H5 与微信小程序双端走 uni-app 统一 API（`uni.navigateTo`、`uni.setStorageSync`），核心逻辑零平台分支；不用仅浏览器可用的 API。
- pages.json 由插件生成，不手工编辑；MP-WEIXIN 产物须含两个新 subPackages。
- 我的页既有行行为不变；登录门复用同组现有机制。

**验证方案**：

- 核心逻辑 TDD：24 点判定/求值（含分数精度、优先级、有解发牌）、华容道移动合法性/阻挡/通关判定。
- `pnpm lint` + `pnpm type-check` + `pnpm build:h5` + `pnpm build:mp-weixin` 全绿。
- 手动冒烟：我的页两入口进页/返回；算24 完整一局（提交/看答案/换题）；华容道移动/阻挡/重置/通关后最佳步数落 storage。
- `openspec validate fun-minigames-24-huarong` 通过。

## Risks / Trade-offs

- [发牌穷举在低端机偶发卡顿] → 4 张 1–13 全枚举规模极小（组合与排列有界），同步计算即可；若仍慢可限制单轮重摇次数上限并降级提示。
- [分数表示（分子/分母）实现疏漏导致误判] → TDD 覆盖除不尽中间结果与优先级用例，判定统一走分数约分比较。
- [华容道初始布局摆错（横竖将方向）] → 以经典「横刀立马」标准图为单一常量来源，测试断言初始格位。
- [subPackages 注释/顺序与既有两行冲突] → 追加而非重排，保持既有行不动。
- [折叠组行数增多导致【我的】页变长] → 组本身可折叠，符合既有交互，不额外处理。

## Migration Plan

纯新增功能，无数据迁移；回滚即 revert 本变更提交（入口行与两分包删除即回原状）。

## Open Questions

（无——入口顺序、存储键命名等小细节由实现时按同组惯例就近决定，不影响 specs 与任务拆分。）
