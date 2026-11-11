# 打牌记账 App — 设计规范（Design Rules）

> 固化于 2026-10-09。Figma 文件：`dHMgzmrrWJ8ztjWytPXdIW`
> 本文件是设计的唯一文字准则；Figma 内 🎨 Foundations 页为其可视化镜像（待额度恢复后补齐）。

## 0. 产品原则

1. **金额是绝对主角** — 任何界面里，数字的视觉权重永远最大。
2. **1 秒记一分** — 单手拇指可达，主操作全部位于屏幕下 1/3。
3. **防色弱** — 每个金额**必须**带 `+` / `−` / `0` 符号，颜色只是增强，符号才是信息。
4. **牌桌氛围** — 深色绒布桌面上的一局夜牌，不是办公软件。

## 1. 颜色 Token

### 基础色（Primitives，隐藏于选择器）

| Token | Hex | 来源意象 |
|---|---|---|
| `felt/900` | `#0B2B21` | 桌布最深处 |
| `felt/700` | `#123D30` | 卡片/面板 |
| `felt/500` | `#1A5040` | 按压态、描边 |
| `felt/300` | `#2E6A55` | 桌布高光 |
| `gold/500` | `#F2B544` | 黄铜筹码（赢） |
| `gold/600` | `#D99E2E` | 金按压态 |
| `red/500` | `#E4593E` | 大胜强调（次于金） |
| `blue/500` | `#5B8DEF` | 冷调（输） |
| `blue/600` | `#4578D6` | 蓝按压态 |
| `paper/500` | `#F4EDDC` | 账本米纸（中性） |
| `cream/100` | `#F2EDE1` | 主文字 |
| `sage/400` | `#8FA89C` | 辅助文字（带绿调灰） |

### 语义色（Color 集合，Dark 模式，全部走别名）

| Token | → 别名 | 用途 |
|---|---|---|
| `color/bg/page` | felt/900 | 页面底 |
| `color/bg/surface` | felt/700 | 卡片底 |
| `color/bg/pressed` | felt/500 | 按压/次级底 |
| `color/border/default` | felt/500 | 常规描边 |
| `color/border/gold` | gold/500 | 赢态/进行中描边（2px） |
| `color/text/primary` | cream/100 | 主文字 |
| `color/text/dim` | sage/400 | 辅助文字 |
| `color/text/on-gold` | felt/900 | 金底上的字 |
| `color/text/on-blue` | cream/100 | 蓝底上的字 |
| `color/score/win` | gold/500 | 赢/正分 |
| `color/score/win-strong` | red/500 | 大胜/破纪录 |
| `color/score/lose` | blue/500 | 输/负分 |
| `color/score/neutral` | paper/500 | 平/0 |
| `color/action/plus` (+pressed) | gold/500/600 | 加分按钮 |
| `color/action/minus` (+pressed) | blue/500/600 | 减分按钮 |
| `color/action/primary` → `primary-text` | gold/500 → felt/900 | 主 CTA |

**规则**：赢 = 金（红仅作大胜强调）；输 = 冷蓝；平 = 米白。禁止引入第四个"状态色"。

## 2. 字体

- **金额/分数**：`Oswald`（Medium / SemiBold / Bold）— 条形压缩、等宽数字，加分时不跳动
- **中文/UI**：`Noto Sans SC`（Regular / Medium / Bold）— 衬托数字

| 样式 | 字体 | 字号/行高 |
|---|---|---|
| `score/hero` | Oswald Bold | 72 / 76 |
| `score/xl` | Oswald SemiBold | 48 / 52 |
| `score/lg` | Oswald SemiBold | 32 / 36 |
| `score/md` | Oswald Medium | 24 / 28 |
| `score/sm` | Oswald Medium | 18 / 22 |
| `h1` | Noto Sans SC Bold | 26 / 34 |
| `h2` | Noto Sans SC Medium | 18 / 26 |
| `body` | Noto Sans SC Regular | 15 / 22 |
| `label` | Noto Sans SC Medium | 13 / 18 |
| `caption` | Noto Sans SC Regular | 11 / 16 |

字距统一 0。禁止全大写标签、禁止给单个词做高亮色（金额色除外）。

## 3. 间距 / 圆角 / 尺寸

- **间距**（4 制）：4 / 8 / 12 / 16 / 20 / 24 / 32 / 40；页面左右边距 **20**
- **圆角**：`sm 8`（小按钮）· `md 14`（输入/chip）· `lg 20`（卡片）· `xl 28`（底部面板）· `full 999`（胶囊/圆钮）
- **尺寸**：最小触达 `44` · 记分圆钮 `56` · TabBar `64+`
- **画布**：390 × 844

## 4. 效果

| 样式 | 参数 | 用途 |
|---|---|---|
| `shadow/card` | 0 8 24 rgba(0,0,0,.35) | 卡片抬起 |
| `glow/win` | 0 0 24 rgba(242,181,68,.35) | 赢态发光（暗底上阴影弱，用辉光表状态） |
| `glow/lose` | 0 0 20 rgba(91,141,239,.3) | 大额失分 |

## 5. 组件规范

| 组件 | 变体 | 要点 |
|---|---|---|
| **ScoreButton** | Type=Plus/Minus × State=Default/Pressed | 56 圆；＋金底深字，−蓝底米字；Pressed 用 pressed 色 |
| **PlayerCard** | State=Win/Lose/Tie | 350 宽，纵向；头行（头像 36+名字+局数）+ 分数行（−钮 / 48pt 分数 / ＋钮）；Win 金色 2px 描边 |
| **SettleCard** | Result=Win/Lose | 350×76 横向；左信息（名+净赢/净输）右金额 32pt |
| **BaseScoreControl** | 单一 | 胶囊：−44 / 底分+金额 / ＋44 |
| **Button** | Kind=Primary/Secondary | 56 高胶囊；主=金实心，次=描边；实例宽度 FILL |
| **TabBar** | Active=Home/Stats | 390 宽；激活项金色，未激活 sage；顶部 1px 分隔 |

所有组件：Auto Layout、语义图层命名、颜色/圆角/间距绑定变量。

## 6. 动效规范（记分页核心）

1. **按下 ±**：按钮切 `State=Pressed`（80ms）
2. **加分浮标**：分数上方弹出 `+N` 金色胶囊，**上移 12px、400ms 淡出**
3. **变色**：分数按结果变金/蓝，配 `glow/win` / `glow/lose` **300ms** 后回落中性（本局内保持结果色）
4. **撤销**：底部 toast「已撤销 老王 +10」，2s 消失
5. 非用户触发动效尽量少：全 App 只保留"加分浮标"这一个记忆点

## 7. 布局通则

- 主操作（开始/记分/保存/撤销）必须落在**下 1/3 拇指热区**
- 每屏只有一个视觉主角（首页=继续上局卡，记分页=分数，结算页=净成绩 hero，战绩=累计值）
- 屏幕结构：StatusBar(44) → Header → 内容区 → [CTA] → TabBar（首页/战绩才有）
- 空状态写行动指引，不写道歉文案

## 8. 命名规范（Figma）

- 页面：`📱 Screens` / `🧩 Components` / `🎨 Foundations`
- 屏幕：`01 首页` … `05 战绩统计`
- 组件集：`PlayerCard`，变体命名 `State=Win`；组件属性用 `Property=Value`
- 变量：`color/score/win` 层级斜杠；间距 `spacing/4`；圆角 `radius/lg`
- 图层：结构容器 `Header`/`ScoreRow`/`Section/xxx`，实例 `ScoreButton/Plus`

## 9. 已知 Figma 工程坑

- `createAutoLayout()` 默认**白色填充** → 结构容器一律 `fills = []`
- Starter 计划最多 **3 页**（已用满）
- `appendChild()` 无返回值，不可链式取节点
- 实例文字覆盖：先 `loadFontAsync`，再按 `characters` 查找后赋值
- 变量 scopes 必填（背景 `FRAME_FILL,SHAPE_FILL` / 文字 `TEXT_FILL` / 间距 `GAP` / 圆角 `CORNER_RADIUS`），WEB code syntax 带 `var()` 包裹
