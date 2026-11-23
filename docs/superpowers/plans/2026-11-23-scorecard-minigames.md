# 打牌记账小程序 · 算24 + 华容道 小游戏 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在记账局叙小程序新增两个独立分包的轻量休闲小游戏——算24（点选拼式、有解保证）与华容道（经典横刀立马、计步存最佳），并在【我的】页趣味工具组接入入口。

**Architecture:** 纯前端本地玩法，零后端。核心逻辑（有理数表达式求值、24 点穷举求解、棋盘移动合法性）放在 `src/utils/{game24,huarong}/` 为纯函数模块，用 node strip-types 单测（TDD）；页面层放 `src/pages/{game24,huarong}/`，经 `vite.config.ts` 的 `subPackages` 扫描注册为独立分包。

**Tech Stack:** uni-app + Vue 3 + Wot UI + TypeScript；`@uni-helper/vite-plugin-uni-pages` 分包扫描；测试 `node --experimental-strip-types` + `node:assert/strict`（沿用 funSalary 惯例，无 vitest）。

**Spec:** `docs/superpowers/specs/2026-11-23-scorecard-minigames-design.md`

## Global Constraints

- OpenSpec 强制流程（uniapp/CLAUDE.md）：先 `.agents/skills/openspec-propose` 建变更，实现后 `openspec-verify-change`、`openspec-archive-change`；变更名：`fun-minigames-24-huarong`。
- 直接在 main 上工作，不建功能分支；每任务一提交（仓库惯例）。
- 页面全部 `navigationStyle: custom` 自绘导航；墨金主题沿用 `var(--ink-900)` 等现有 token，底色 `#14120F`。
- 分包路由只通过 `vite.config.ts:37` 的 `subPackages` 数组注册扫描根，`src/pages.json` 是生成物**不得手改**。
- 不接任何 API / 后端；不用仅浏览器 API（须兼容 MP-WEIXIN）。
- wot-ui 组件生成/改动前查 `.agents/skills/wot-ui-v2/SKILL.md` 与 MCP（CLAUDE.md 要求）。
- 类名前缀避让 UnoCSS 撞名清单（现有前缀 `fcn-`、`sal-` 等惯例；本计划页面前缀 `g24-`、`hr-`）。
- 测试命令一律：`node --experimental-strip-types <test文件>`，文件内 import 带 `.ts` 扩展名（strip-types 要求）。
- 验证门禁：`pnpm lint` + `pnpm type-check` + `pnpm build:h5` 全绿才可提交页面任务。
- 提交信息用仓库惯例（`feat(scorecard-appui): ...` / `test(...)`，git log 可见先例）。

## Review Focus

以下输入/失败模式是 spec 隐含但最容易被漏测的，各归属任务的测试须覆盖：

1. **无解牌组**——四张数凑不出 24 却被发给玩家，「看答案」永远空。→ Task 3 测试：`findSolution([1,1,1,1])` 返回 `null`；`findSolution([3,3,8,8])` 返回可求值为 24 的 token 串。
2. **浮点误差与除零**——`8/(3−8/3)=24` 这类分数结果被浮点判成 23.99…，或除零产出 NaN 页面崩。→ Task 2 测试：有理数 `8/(3−8/3)` 精确等于 24；`evalTokens([5,'/',0])` 返回 `null`。
3. **非法拼式**——连点运算符、括号不闭合、数字后接数字。→ Task 2 测试：`[2,'+', '+', 3]`、`['(',1,'+',2]`、`[1,2]` 均返回 `null` 而非抛异常。
4. **华容道非法移动**——目标格越界或被占仍移动导致棋盘重叠。→ Task 5 测试：初始局面各 `canMove` 用例 + 越界/占格返回 `false`；`movePiece` 后全盘无重叠、无越界。
5. **胜利判定**——曹操未到底部中央就判赢（或反之）。→ Task 5 测试：初始局面 `isWin=false`；曹操移至 `(x=1,y=3)` 后 `isWin=true`；曹操在其他底部位置不判赢。

---

### Task 1: OpenSpec 变更创建

**Files:**
- Create: `frontend/scorecard/appui/uniapp/openspec/changes/fun-minigames-24-huarong/`（proposal.md / specs / tasks.md，由 skill 生成）

**Interfaces:**
- Consumes: 设计文档 `docs/superpowers/specs/2026-11-23-scorecard-minigames-design.md`
- Produces: OpenSpec 变更目录 `fun-minigames-24-huarong`，后续任务在其 tasks.md 打勾。

- [ ] **Step 1: 调用 openspec-propose skill 创建变更**

调 Skill `openspec-propose`，输入变更名 `fun-minigames-24-huarong`，需求描述取自设计文档（两独立分包小游戏 + 我的页入口，轻量彩蛋级，零后端）。delta spec 拆两个：`fun-game24`、`fun-huarong`（另入口改动归入现有 `home`/`mine` 相关 spec 或新 `mine-fun-entry`，以 skill 生成为准）。

- [ ] **Step 2: 校验变更存在且合法**

Run: `cd frontend/scorecard/appui/uniapp && pnpm exec openspec validate fun-minigames-24-huarong`
Expected: 通过（或按 skill 输出修正后通过）

- [ ] **Step 3: Commit**

```bash
git add frontend/scorecard/appui/uniapp/openspec/changes/fun-minigames-24-huarong
git commit -m "spec(scorecard-appui): OpenSpec 变更 fun-minigames-24-huarong（算24+华容道小游戏）"
```

---

### Task 2: 算24 有理数与表达式求值器

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/utils/game24/eval.ts`
- Test: `frontend/scorecard/appui/uniapp/src/utils/game24/eval.test.ts`

**Interfaces:**
- Consumes: 无（纯函数）
- Produces:
  - `type Token = number | '+' | '-' | '*' | '/' | '(' | ')'`
  - `interface Rational { n: number, d: number }`（既约分数，`d > 0`，gcd 约分；负号在 `n`）
  - `export function rat(n: number, d: number): Rational`（约分归一；`d === 0` 抛错仅供内部，求值路径不调它）
  - `export function ratEq(a: Rational, b: Rational): boolean`
  - `export function evalTokens(tokens: Token[]): Rational | null`（中缀、标准优先级、括号；语法非法或除零 → `null`，不抛异常）

- [ ] **Step 1: 写失败测试**

`eval.test.ts`（`node:assert/strict`，文件头注释运行命令，仿 `src/utils/funSalary/holidays.test.ts`）：

```ts
import assert from 'node:assert/strict'
import { evalTokens, ratEq } from './eval.ts'
import type { Token } from './eval.ts'

// 精确 24：8/(3-8/3)（浮点会误差的分数解）
const r1 = evalTokens([8, '/', '(', 3, '-', 8, '/', 3, ')'])!
assert.ok(r1 && ratEq(r1, { n: 24, d: 1 }), '8/(3-8/3) 应精确等于 24')
// 优先级：1+2*3 = 7，非 9
assert.ok(ratEq(evalTokens([1, '+', 2, '*', 3])!, { n: 7, d: 1 }))
// 括号改变优先级：(1+2)*3 = 9
assert.ok(ratEq(evalTokens(['(', 1, '+', 2, ')', '*', 3])!, { n: 9, d: 1 }))
// 减法可为负：1-5
assert.ok(ratEq(evalTokens([1, '-', 5])!, { n: -4, d: 1 }))
// 除零 → null
assert.equal(evalTokens([5, '/', 0]), null)
// 非法序列 → null（不抛）
assert.equal(evalTokens([2, '+', '+', 3]), null)
assert.equal(evalTokens(['(', 1, '+', 2]), null)      // 括号不闭合
assert.equal(evalTokens([1, 2]), null)                 // 数字连数字
assert.equal(evalTokens([]), null)
console.log('eval.test.ts: ALL PASS')
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend/scorecard/appui/uniapp && node --experimental-strip-types src/utils/game24/eval.test.ts`
Expected: FAIL（模块不存在 / SyntaxError）

- [ ] **Step 3: 实现 `eval.ts`**

递归下降（`parseExpr` 加减 → `parseTerm` 乘除 → `parseFactor` 数字/括号）+ 有理数四则（`rat` 内 gcd 约分）。全程 `Rational` 运算，命中 `d === 0`（除零）或语法不符即返回 `null`。`ratEq` 比较 `n1*d2 === n2*d1`。注意减法/除法操作数顺序（`a-b` ≠ `b-a`）。

- [ ] **Step 4: 运行确认通过**

Run: 同 Step 2
Expected: `eval.test.ts: ALL PASS`

- [ ] **Step 5: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/utils/game24/
git commit -m "test(scorecard-appui): 算24 有理数中缀表达式求值器（优先级/括号/除零/非法序列）"
```

---

### Task 3: 算24 求解器（有解发牌 + 答案）

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/utils/game24/solve.ts`
- Test: `frontend/scorecard/appui/uniapp/src/utils/game24/solve.test.ts`

**Interfaces:**
- Consumes: Task 2 的 `Token`、`evalTokens`、`ratEq`
- Produces:
  - `export function findSolution(cards: number[]): Token[] | null`——返回**每个二元运算都带括号**的中缀 token 序列（保证 `evalTokens` 可直接求值），无解返回 `null`
  - `export function formatExpr(tokens: Token[]): string`——展示串，剥除最外层一对冗余括号（如 `((1+2))` → `1+2`，其余保留）
  - `export function dealCards(rng?: () => number): number[]`——发 4 张 1–13（可重复），用 `findSolution` 验证有解才返回；`rng` 默认 `Math.random`

- [ ] **Step 1: 写失败测试**

`solve.test.ts`：

```ts
import assert from 'node:assert/strict'
import { evalTokens, ratEq } from './eval.ts'
import { dealCards, findSolution, formatExpr } from './solve.ts'

// 经典分数解 3,3,8,8：求出的解必须能被 evalTokens 精确判为 24
const s1 = findSolution([3, 3, 8, 8])!
assert.ok(s1, '[3,3,8,8] 应有解')
assert.ok(ratEq(evalTokens(s1)!, { n: 24, d: 1 }), '解求值须精确 24')

// 无解：四个 1
assert.equal(findSolution([1, 1, 1, 1]), null)

// 每个随机牌组都有解（抽 50 组）
for (let i = 0; i < 50; i++) {
  const cards = dealCards()
  assert.equal(cards.length, 4)
  assert.ok(cards.every(c => Number.isInteger(c) && c >= 1 && c <= 13))
  assert.ok(findSolution(cards), `发牌 [${cards}] 必须有解`)
}

// formatExpr 剥最外层括号
assert.equal(formatExpr(['(', 1, '+', 2, ')']), '1+2')
assert.equal(formatExpr([1, '+', 2]), '1+2')

console.log('solve.test.ts: ALL PASS')
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend/scorecard/appui/uniapp && node --experimental-strip-types src/utils/game24/solve.test.ts`
Expected: FAIL（`./solve.ts` 不存在）

- [ ] **Step 3: 实现 `solve.ts`**

穷举：每次取两张数 `a,b`，试 `a⊕b`、`b⊕a`（`-` `/` 不交换），连同余数递归；剩一张时用 `evalTokens` 判 `== 24`。构造 token：子结果一律包 `(`…`)`（简单、保证正确，冗余括号由 `formatExpr` 展示层处理）。递归深度 3，13 以内的整数分支有限，无须剪枝。`dealCards` 循环摇号直到 `findSolution !== null`（期望次数低，`while` 加迭代上限 1000 兜底）。

- [ ] **Step 4: 运行确认通过**

Run: 同 Step 2
Expected: `solve.test.ts: ALL PASS`（50 组随机有解 + 无解组返回 null）

- [ ] **Step 5: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/utils/game24/
git commit -m "test(scorecard-appui): 算24 穷举求解器与有解发牌（findSolution/dealCards/formatExpr）"
```

---

### Task 4: 算24 页面 + 分包注册

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/pages/game24/index.vue`
- Modify: `frontend/scorecard/appui/uniapp/vite.config.ts:37`（`subPackages` 数组追加 `'src/pages/game24'`）

**Interfaces:**
- Consumes: Task 2 `evalTokens/ratEq/Token`、Task 3 `findSolution/dealCards/formatExpr`
- Produces: 路由 `/pages/game24/index`（分包 `pages/game24`）

- [ ] **Step 1: 注册分包扫描根**

`vite.config.ts:37` 改为：
```ts
subPackages: ['src/pages/fun', 'src/pages/starfuyu', 'src/pages/game24'],
```
（保留原注释，追加一行说明：算24 分包 fun-minigames-24-huarong）

- [ ] **Step 2: 实现页面 `pages/game24/index.vue`**

结构与风格仿 `src/pages/fun/salary/index.vue`（`ModNav` 自绘导航 `import ModNav from '@/components/funCompass/ModNav.vue'`，`title="算24" sub="趣味小工具 · 仅供娱乐"`，`@back="uni.navigateBack()"`；`useSystemBar()`；墨金 token；类名前缀 `g24-`）。逻辑：

- `cards: number[]`（`dealCards()` 初始 + 每题重发）、`tokens: Token[]`（拼式缓冲，元素带 `cards` 索引或 `'op'`/`'paren'` 来源标记）
- **牌区**：4 张数牌，已点选用过置灰禁点（按索引消耗，重复数字互不影响）
- **运算区**：`+ − × ÷` 与 `(` `)` 六钮 + 「⌫ 删除」（回退 tokens 末位并把对应牌恢复可用）
- **式子区**：实时 `formatExpr(tokens)`（空时显示占位「点选数字与运算符」）
- **动作区**：「提交」（`evalTokens` 判 `ratEq(_, {n:24,d:1})`：对 → `useToast().success('正解！')` + 0.8s 后 `dealCards()` 换题并清 tokens；错 → toast 提示 + 式子短暂红显，不清空可改）；「看答案」（`findSolution(cards)` → toast/式子区展示 `formatExpr`；理论上发牌必有解，null 时兜底换题）；「换一题」（重发 + 清空）；「清空」（只清 tokens 恢复全部牌）
- 无存储、无接口

- [ ] **Step 3: 静态验证**

Run: `cd frontend/scorecard/appui/uniapp && pnpm lint && pnpm type-check && pnpm build:h5`
Expected: 全绿；`src/pages.json` 生成物里出现 `pages/game24` 分包（构建自动生成，确认后**不手改**，但随提交入库）

- [ ] **Step 4: Commit**

```bash
git add frontend/scorecard/appui/uniapp/vite.config.ts frontend/scorecard/appui/uniapp/src/pages/game24 frontend/scorecard/appui/uniapp/src/pages.json
git commit -m "feat(scorecard-appui): 算24 小游戏分包页面（点选拼式/提交判定/看答案/换题）"
```

---

### Task 5: 华容道棋盘核心

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/utils/huarong/board.ts`
- Test: `frontend/scorecard/appui/uniapp/src/utils/huarong/board.test.ts`

**Interfaces:**
- Consumes: 无（纯函数）
- Produces:
  - `const COLS = 4`, `const ROWS = 5`
  - `type Dir = 'up' | 'down' | 'left' | 'right'`
  - `interface Piece { id: string, name: string, w: number, h: number, x: number, y: number }`（`x,y` 为左上角格）
  - `export const INITIAL_PIECES: Piece[]`——经典「横刀立马」布局（见下）
  - `export function movableDirs(pieces: Piece[], id: string): Dir[]`
  - `export function canMove(pieces: Piece[], id: string, dir: Dir): boolean`（越界或目标格被占 → false）
  - `export function movePiece(pieces: Piece[], id: string, dir: Dir): Piece[]`（返回新数组，不 mutate 入参）
  - `export function isWin(pieces: Piece[]): boolean`（`cao` 占据 `x=1,y=3` 即底中 2×2 → true）

  经典布局（列 0–3，行 0–4，两空格在底行）：

  | | c0 | c1 | c2 | c3 |
  |---|---|---|---|---|
  | r0 | 张(竖1×2) | 曹(2×2) | 曹 | 马(竖1×2) |
  | r1 | 张 | 曹 | 曹 | 马 |
  | r2 | 黄(竖1×2) | 关(横2×1) | 关 | 赵(竖1×2) |
  | r3 | 黄 | 兵 | 兵 | 赵 |
  | r4 | 兵 | ⬜ | 兵 | ⬜ |

  ids 与格位（左上角 `x,y`）逐枚对死：
  `cao(2×2)=(1,0)`、`zf(1×2)=(0,0)`、`zy(1×2)=(3,0)`、`gy(2×1)=(1,2)`、`hz(1×2)=(0,2)`、`mc(1×2)=(3,2)`、`p1=(0,4)`、`p2=(1,3)`、`p3=(2,3)`、`p4=(2,4)`（兵均为 1×1）。name 用中文（曹操/关羽/张飞/赵云/马超/黄忠/兵）。

  由该表推演的初始可动性（测试期望的依据）：`p1` 上方是 `hz`、下方越界、左越界、右是空 → `movableDirs=['right']`；`p2/p3` 下方 r4c1 是空（`p2` 可 down、`p3` 下方 c2 是 `p4` 不可）… 实现者**逐格推演填全**，凡 `canMove` 期望与此表矛盾，以布局表为准修期望而非改实现。

- [ ] **Step 1: 写失败测试**

`board.test.ts`：

```ts
import assert from 'node:assert/strict'
import { COLS, ROWS, canMove, INITIAL_PIECES, isWin, movableDirs, movePiece } from './board.ts'
import type { Dir, Piece } from './board.ts'

function assertValid(pieces: Piece[]) {
  for (const p of pieces) {
    assert.ok(p.x >= 0 && p.y >= 0 && p.x + p.w <= COLS && p.y + p.h <= ROWS, `${p.id} 越界`)
    for (const q of pieces)
      if (p !== q)
        assert.ok(p.x + p.w <= q.x || q.x + q.w <= p.x || p.y + p.h <= q.y || q.y + q.h <= p.y, `${p.id} 与 ${q.id} 重叠`)
  }
}

assertValid(INITIAL_PIECES)
assert.equal(INITIAL_PIECES.length, 10)
assert.equal(isWin(INITIAL_PIECES), false)

// 初始可动性（依据上方布局表逐格推演写死）
assert.deepEqual(movableDirs(INITIAL_PIECES, 'p1'), ['right'])  // (0,4)：右邻 (1,4) 空
assert.deepEqual(movableDirs(INITIAL_PIECES, 'gy'), [])         // 关羽 (1,2)：上曹、下 p2/p3、左右 hz/mc 全挡
// 越界
assert.equal(canMove(INITIAL_PIECES, 'p1', 'down'), false)      // y=4 再下越界
// 占格
assert.equal(canMove(INITIAL_PIECES, 'cao', 'up'), false)       // (1,0) 上方越界
assert.equal(canMove(INITIAL_PIECES, 'p2', 'down'), true)       // (1,3)→(1,4) 空：合法
assert.equal(canMove(INITIAL_PIECES, 'p3', 'down'), false)      // (2,3)→(2,4) 是 p4：占格

// 合法移动后局面有效
const after = movePiece(INITIAL_PIECES, 'p1', 'right')
assertValid(after)
assert.equal(after.find(p => p.id === 'p1')!.x, 1)              // 入参不 mutate：原件仍 x=0
assert.equal(INITIAL_PIECES.find(p => p.id === 'p1')!.x, 0)

// 胜利判定
assert.equal(isWin(INITIAL_PIECES), false)
const won = INITIAL_PIECES.map(p => p.id === 'cao' ? { ...p, x: 1, y: 3 } : p)
assert.equal(isWin(won), true)
const notWon = INITIAL_PIECES.map(p => p.id === 'cao' ? { ...p, x: 0, y: 3 } : p)
assert.equal(isWin(notWon), false) // 底部但不在正中

console.log('board.test.ts: ALL PASS')
```

注：测试中「按布局核对」的 `movableDirs`/`canMove` 期望值，写测试时**按上表逐格推演后写死**（不许为凑断言改实现）；推演若有出入以初始布局表为准修期望。

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend/scorecard/appui/uniapp && node --experimental-strip-types src/utils/huarong/board.test.ts`
Expected: FAIL（`./board.ts` 不存在）

- [ ] **Step 3: 实现 `board.ts`**

- `INITIAL_PIECES` 按布局表硬编码 10 枚。
- `canMove`：算出移动 1 格后的矩形，先查 `0<=x && x+w<=COLS`（同 y），再遍历其余 9 枚查矩形相交（`!(a.x+a.w<=b.x || b.x+b.w<=a.x || ...)` 即相交）。
- `movableDirs` = 四个 `Dir` 过滤 `canMove`。
- `movePiece` = map 克隆目标枚 `{...p, x/y ±1}` 返回。
- `isWin` = `cao` 的 `x===1 && y===3`。

- [ ] **Step 4: 运行确认通过**

Run: 同 Step 2
Expected: `board.test.ts: ALL PASS`

- [ ] **Step 5: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/utils/huarong/
git commit -m "test(scorecard-appui): 华容道棋盘核心（初始布局/移动合法性/胜利判定）"
```

---

### Task 6: 华容道页面 + 分包注册

**Files:**
- Create: `frontend/scorecard/appui/uniapp/src/pages/huarong/index.vue`
- Modify: `frontend/scorecard/appui/uniapp/vite.config.ts:37`（追加 `'src/pages/huarong'`）

**Interfaces:**
- Consumes: Task 5 全部导出；`ModNav`、`useSystemBar`、`useToast`（auto-import）
- Produces: 路由 `/pages/huarong/index`（分包 `pages/huarong`）

- [ ] **Step 1: 注册分包扫描根**

`subPackages: ['src/pages/fun', 'src/pages/starfuyu', 'src/pages/game24', 'src/pages/huarong']`

- [ ] **Step 2: 实现页面 `pages/huarong/index.vue`**

风格同 Task 4（前缀 `hr-`，`ModNav title="华容道" sub="趣味小工具 · 经典横刀立马"`）。逻辑：

- 状态：`pieces: Piece[]`（`[...INITIAL_PIECES]`）、`steps: number`、`best: number | null`（初读 `uni.getStorageSync('huarong:best')`，无则 null）
- **棋盘渲染**：4×5 格子背景（绝对定位按 `COLS/ROWS` 换算 `rpx` 或 `%`），10 枚块按 `x,y,w,h` 绝对定位，曹操金色加大、兵小块（墨金主题区分色，中文名居中）
- **交互**：点块选中 → 高亮 `movableDirs` 四向箭头（不可动方向不渲染）→ 点箭头 `movePiece` + `steps++`；点空白/再点同块取消选中
- **胜利**：`isWin(pieces)` 为 true → toast 庆祝 + 若 `best === null || steps < best` 则 `uni.setStorageSync('huarong:best', steps)` 更新 `best`；锁定交互直到「重玩」
- **头部**：步数、最佳（null 显示「—」）、「重置」按钮（回初始、步数清零、重新解锁）
- 无计时、无解法提示、无接口

- [ ] **Step 3: 静态验证**

Run: `cd frontend/scorecard/appui/uniapp && pnpm lint && pnpm type-check && pnpm build:h5`
Expected: 全绿，生成 `pages.json` 含 `pages/huarong` 分包

- [ ] **Step 4: Commit**

```bash
git add frontend/scorecard/appui/uniapp/vite.config.ts frontend/scorecard/appui/uniapp/src/pages/huarong frontend/scorecard/appui/uniapp/src/pages.json
git commit -m "feat(scorecard-appui): 华容道分包页面（横刀立马/点箭头移动/计步/最佳本地存储）"
```

---

### Task 7: 【我的】页入口两行

**Files:**
- Modify: `frontend/scorecard/appui/uniapp/src/pages/mine/index.vue`（「更多趣味小工具」折叠组，约 233–285 行区域；script 区约 93–100 行）

**Interfaces:**
- Consumes: Task 4 `/pages/game24/index`、Task 6 `/pages/huarong/index`
- Produces: 无（纯入口）

- [ ] **Step 1: 加两个跳转函数**

script 内 `goSalary` 旁追加（同款无登录门——纯本地游戏，与 goSalary 一致，不走 goStarfuyu 的登录门）：

```ts
/** 算24（fun-minigames-24-huarong）→ 分包 pages/game24/index */
function goGame24() {
  router.push({ path: '/pages/game24/index' })
}
/** 华容道（fun-minigames-24-huarong）→ 分包 pages/huarong/index */
function goHuarong() {
  router.push({ path: '/pages/huarong/index' })
}
```

- [ ] **Step 2: 模板加两行**

「日期计算器（占位）」行之前，插入两个 `g-row fun-row`（结构逐字仿 `goSalary` 那行：`fc-tile` 图标块 + `r-main` 标题/副题 + `wd-icon chev`）：

- 算24：`fc-tile` 内文字 `24`，标题「算24」，副题「四张牌凑二十四」，`@click="goGame24"`
- 华容道：`fc-tile` 内文字 `华`，标题「华容道」，副题「经典横刀立马」，`@click="goHuarong"`

（`fc-tile` 变体色沿用现有 `y`/`i` 类之一或新增 `g24`/`hr` 变体，以不破坏 UnoCSS 撞名清单为准。）

- [ ] **Step 3: 静态验证**

Run: `cd frontend/scorecard/appui/uniapp && pnpm lint && pnpm type-check && pnpm build:h5`
Expected: 全绿

- [ ] **Step 4: Commit**

```bash
git add frontend/scorecard/appui/uniapp/src/pages/mine/index.vue
git commit -m "feat(scorecard-appui): 【我的】趣味工具组接入算24与华容道入口"
```

---

### Task 8: OpenSpec 核对与归档

**Files:**
- Modify: `frontend/scorecard/appui/uniapp/openspec/changes/fun-minigames-24-huarong/tasks.md`（打勾）

**Interfaces:**
- Consumes: Task 1–7 全部产出
- Produces: 归档变更

- [ ] **Step 1: 逐任务打勾 tasks.md**

按实际完成情况勾选 OpenSpec tasks.md。

- [ ] **Step 2: verify**

调 Skill `openspec-verify-change`（变更 `fun-minigames-24-huarong`），核对规格、任务、实现一致；不一致处修代码或修 spec 后复跑。

- [ ] **Step 3: 全量门禁复跑**

Run: `cd frontend/scorecard/appui/uniapp && pnpm lint && pnpm type-check && pnpm build:h5 && node --experimental-strip-types src/utils/game24/eval.test.ts && node --experimental-strip-types src/utils/game24/solve.test.ts && node --experimental-strip-types src/utils/huarong/board.test.ts`
Expected: 全绿

- [ ] **Step 4: 人工冒烟（记录结果）**

Run: `pnpm dev:h5` 后浏览器验证：我的页两入口出现且跳转正确；算24 发牌必有解、提交判定/看答案/换题可用；华容道可移动、计步、重置、造胜后最佳步数落 storage。若 dev 起不来或环境不通，如实记录待人工，不虚报。

- [ ] **Step 5: archive**

调 Skill `openspec-archive-change`（`fun-minigames-24-huarong`）。

- [ ] **Step 6: Commit**

```bash
git add -A frontend/scorecard/appui/uniapp/openspec
git commit -m "docs(scorecard-appui): fun-minigames-24-huarong 核对归档"
```
