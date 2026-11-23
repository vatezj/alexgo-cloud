# Tasks: fun-minigames-24-huarong

## 1. 算24 核心逻辑（TDD）

- [x] 1.1 写失败测试 `src/utils/game24/eval.test.ts`（优先级/括号/负数/除零→null/非法序列→null/`8/(3-8/3)` 精确 24），运行确认 FAIL
- [x] 1.2 实现 `src/utils/game24/eval.ts`（有理数 `rat`/`ratEq`、递归下降 `evalTokens`，语法非法或除零返回 `null`），测试 ALL PASS
- [x] 1.3 Commit：`test(scorecard-appui): 算24 有理数中缀表达式求值器（优先级/括号/除零/非法序列）`
- [x] 1.4 写失败测试 `src/utils/game24/solve.test.ts`（`[3,3,8,8]` 有解且求值精确 24、`[1,1,1,1]` 返回 null、50 组随机发牌必有解、`formatExpr` 剥最外层括号），运行确认 FAIL
- [x] 1.5 实现 `src/utils/game24/solve.ts`（`findSolution` 穷举每二元运算带括号、`dealCards` 有解发牌带迭代上限、`formatExpr`），测试 ALL PASS
- [x] 1.6 Commit：`test(scorecard-appui): 算24 穷举求解器与有解发牌（findSolution/dealCards/formatExpr）`
- [x] 1.7 `pnpm lint` 覆盖新增 util 文件

## 2. 算24 页面与分包

- [x] 2.1 `vite.config.ts` `subPackages` 追加 `'src/pages/game24'`
- [x] 2.2 实现 `src/pages/game24/index.vue`（ModNav 自绘导航、点选拼式、提交判定/看答案/换题/清空、墨金主题、`g24-` 前缀、零存储零接口）
- [x] 2.3 `pnpm lint && pnpm type-check && pnpm build:h5` 全绿，`src/pages.json` 生成物含 `pages/game24` 分包（不手改，随提交入库）
- [x] 2.4 Commit：`feat(scorecard-appui): 算24 小游戏分包页面（点选拼式/提交判定/看答案/换题）`

## 3. 华容道核心逻辑（TDD）

- [x] 3.1 写失败测试 `src/utils/huarong/board.test.ts`（初始布局有效、初始可动性逐格推演断言、越界/占格→false、`movePiece` 不 mutate、`isWin` 底中判真/偏位判假），运行确认 FAIL
- [x] 3.2 实现 `src/utils/huarong/board.ts`（`INITIAL_PIECES` 横刀立马 10 枚、`canMove`/`movableDirs`/`movePiece`/`isWin`、`COLS`/`ROWS`），测试 ALL PASS
- [x] 3.3 Commit：`test(scorecard-appui): 华容道棋盘核心（初始布局/移动合法性/胜利判定）`
- [x] 3.4 `pnpm lint` 覆盖新增 util 文件

## 4. 华容道页面与分包

- [x] 4.1 `vite.config.ts` `subPackages` 追加 `'src/pages/huarong'`（与 2.1 合并为完整四元素数组）
- [x] 4.2 实现 `src/pages/huarong/index.vue`（ModNav、4×5 棋盘渲染、点选高亮方向移动计步、重置、`huarong:best` 本地 storage 通关更新、`hr-` 前缀、无计时无提示无接口）
- [x] 4.3 `pnpm lint && pnpm type-check && pnpm build:h5` 全绿，`src/pages.json` 含 `pages/huarong` 分包（随提交入库）
- [x] 4.4 Commit：`feat(scorecard-appui): 华容道分包页面（横刀立马/点箭头移动/计步/最佳本地存储）`

## 5. 【我的】页入口

- [x] 5.1 `src/pages/mine/index.vue` 加 `goGame24`/`goHuarong` 跳转函数（与 goSalary 同款无登录门）
- [x] 5.2 「更多趣味小工具」组「日期计算器（占位）」前插入算24、华容道两行（结构仿工资行，`g24-`/`hr-` 变体不撞 UnoCSS 撞名清单）
- [x] 5.3 `pnpm lint && pnpm type-check && pnpm build:h5` 全绿；我的页两入口点击进页、返回落回
- [x] 5.4 Commit：`feat(scorecard-appui): 【我的】趣味工具组接入算24与华容道入口`

## 6. 验证与归档

- [x] 6.1 全量门禁复跑：`pnpm lint && pnpm type-check && pnpm build:h5` + 三个 node 测试文件全绿
- [x] 6.2 人工冒烟：`pnpm dev:h5` 验证我的页入口、算24 一局、华容道移动/重置/造胜后最佳步数落 storage（环境不通则如实记录）——H5 冒烟完成：我的页五行走序与两入口跳转、算24 发牌/看答案/换题/正误提交/标红/清空/零存储、华容道选中箭头移动计步/重置/通关 overlay/`huarong:best` 落 storage/重玩保留最佳均验证通过（通关为注入近胜局面后走真实 onArrow 路径）
- [x] 6.3 按实际完成情况勾选本 tasks.md；调 Skill `openspec-verify-change` 核对 spec/任务/实现一致
- [x] 6.4 调 Skill `openspec-archive-change` 归档变更
- [x] 6.5 Commit：`docs(scorecard-appui): fun-minigames-24-huarong 核对归档`（`git add -A frontend/scorecard/appui/uniapp/openspec`）
