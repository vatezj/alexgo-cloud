# fun-salary Delta Spec

## ADDED Requirements

### Requirement: 首页直达入口

首页 SHALL 提供「工资到账计算器」入口卡（位于趣味方位入口卡之后、同款视觉），点击 SHALL 直达 `pages/fun/salary/index` 并在返回时落回首页；该入口 SHALL 受 `fun_compass_enable` 门控（显式 false 时不渲染，与趣味方位入口同隐）。【我的】页分组入口 MUST 保持可用。

#### Scenario: 首页一键直达
- **WHEN** 在首页点击「工资到账计算器」入口卡
- **THEN** 直接进入工资到账计算器页，左上返回落回首页

#### Scenario: 开关双卡同隐
- **WHEN** `fun_compass_enable` 为 false
- **THEN** 首页趣味方位与工资到账两张入口卡均不渲染，其余首页内容不受影响
