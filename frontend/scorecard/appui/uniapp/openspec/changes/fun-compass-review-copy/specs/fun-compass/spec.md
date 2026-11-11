# fun-compass Delta Spec (fun-compass-review-copy)

## Purpose

（本 delta 依附既有 fun-compass 能力；Purpose 以主规格/首建 change 为准，此处只追加审核安全文案约束。）

## ADDED Requirements

### Requirement: 用户可见文案审核安全口径

趣味方位模块全部用户可见文案（首页入口、【我的】分组、六页导航标题/表单/CTA/loading/toast/空态、copy.ts 锁定句）SHALL 使用中性民俗娱乐表述，MUST NOT 出现「八字、排盘、运势、吉凶、算命、占卜」等占卜算命类词；「生辰」SHALL 表述为「出生时间/出生信息」，「五行」SHALL 不在用户可见文案中作为结论词出现。两句底部免责与「民俗趣味参考、无科学依据」框架 MUST 保留。

#### Scenario: 入口与 CTA 无八字/排盘字样
- **WHEN** 用户查看首页趣味方位入口卡、模块首页三卡、生辰/模拟器页 CTA、【我的】分组行
- **THEN** 可见文案不含「排八字」「排盘」；方位测算类动词为「测方位/测算」等中性词

#### Scenario: 导航与 toast 合规
- **WHEN** 进入生辰排盘页、触发校验失败或服务失败 toast、查看 loading 文案
- **THEN** 导航标题与 toast 不含「排盘/八字/生辰选择」；loading 为「计算中…」类表述

#### Scenario: 免责框架保留
- **WHEN** 用户打开关于页或查看任一页底部免责
- **THEN** 「传统民俗娱乐、无科学依据、与记账理财无关」语义与免责句仍完整可见

### Requirement: 后端推荐 reason 中性化

zhiwei `POST /api/calc` 返回的每条 `recommendations[].reason` SHALL 为中性方位评分说明，MUST NOT 出现「奇门、紫微、生门、命宫、财帛宫、吉星、煞星、旺相、衰囚、财气、吉凶」及具体星名/宫名；计分、排序、top3 截断与正分过滤逻辑 MUST 保持不变。

#### Scenario: 结果卡 reason 无玄学术语
- **WHEN** 前端排盘成功并在结果列表展示 `reason`
- **THEN** 文案为中性评分描述（如「该向综合评分较高」），无玄学/吉凶类词

#### Scenario: 评分行为不回归
- **WHEN** 运行 zhiwei scoring 单测（分值用例）
- **THEN** 各用例分数与排序与改文案前一致，仅 reason 断言文案更新
