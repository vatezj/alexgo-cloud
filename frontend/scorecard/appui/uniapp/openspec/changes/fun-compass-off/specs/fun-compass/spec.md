# fun-compass Delta Spec (fun-compass-off)

## Purpose

趣味方位/算卜类能力自本变更起从小程序端下线，以满足平台审核；本 delta 追加下线要求。

## ADDED Requirements

### Requirement: 端侧不再提供趣味方位算卜功能

小程序包内 SHALL NOT包含生辰/出生时间排盘、方位推荐、座位推荐、罗盘对向等页面或调用 `POST /api/calc` 的客户端代码；首页与【我的】SHALL NOT 提供趣味方位入口。工资到账计算器 MUST 保持可用。

#### Scenario: 分包无 compass 页
- **WHEN** 构建 `build:mp-weixin` 并读取 `pages.json`
- **THEN** `subPackages` 中不出现 `pages/fun/compass/*` 路由

#### Scenario: 入口不可见
- **WHEN** 打开首页或【我的】
- **THEN** 无「趣味方位/罗盘选座/测方位」入口；工资计算器入口仍在

#### Scenario: 无排盘请求
- **WHEN** 全量检索端侧 `src/`
- **THEN** 无 `zhiwei`/`api/calc` 客户端引用
