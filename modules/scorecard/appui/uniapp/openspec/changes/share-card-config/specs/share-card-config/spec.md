# share-card-config Delta Spec

## Purpose

三页（首页/座位模拟器/工资记账）分享卡的 title 与图片由后台配置驱动：小程序拉取、缓存、失败回退，并在微信转发钩子中消费，使运营可在管理后台随时替换分享文案与图片而无需发版。

## ADDED Requirements

### Requirement: 分享卡配置拉取与缓存

小程序 SHALL 在应用启动后拉取 `GET /api/public/scorecard/share-config`（免鉴权），将返回的 `{index, simulator, salary} → {title, imageUrl}` 写入 storage（键 `scorecard:shareCards`）并作为内存缓存；后续读取 SHALL 优先内存、次之 storage、最后回退内置默认（与历史硬编码一致：首页 title「登录后开房记账 · 好友互通」+ 七牛 logo）。拉取失败 SHALL 静默降级（不 toast、不清已有缓存），且 MUST NOT 阻塞任何页面渲染或转发钩子。

#### Scenario: 首次启动拉取成功
- **WHEN** 应用冷启动且配置接口返回 code=0
- **THEN** 三页配置写入 storage，分享钩子读到后台最新 title/imageUrl

#### Scenario: 配置接口失败回退默认
- **WHEN** 配置接口网络失败或返回非 0 code
- **THEN** 分享钩子使用内置默认配置（等价历史硬编码），无任何错误提示弹出

#### Scenario: 二次启动免请求读缓存
- **WHEN** 进程被杀后重新冷启动且 storage 已有缓存
- **THEN** 分享钩子在配置请求完成前即可读到上次缓存值

### Requirement: 首页分享消费配置

首页 `onShareAppMessage` SHALL 使用缓存配置的 `index.title` 与 `index.imageUrl`，`onShareTimeline` SHALL 使用 `index.title`；`inv=<我的 user_id>` 拼接与未登录不带 inv 的既有语义 MUST 保持不变。

#### Scenario: 首页转发读后台标题
- **WHEN** 后台把 `scorecard.share.index` 的 title 改为「新标题」并被拉取后，用户经胶囊菜单转发首页
- **THEN** 分享卡片标题为「新标题」，path 仍为 `/pages/index/index?scene=inv=<uid>`（已登录）

#### Scenario: 已登录 inv 语义不变
- **WHEN** 已登录用户转发首页
- **THEN** path 携带 `inv=<我的 user_id>`；未登录转发时 path 不带 inv（与改造前一致）

### Requirement: 座位模拟器与工资记账新增分享

`pages/fun/compass/simulator` 与 `pages/fun/salary/index` SHALL 各自提供 `onShareAppMessage` 与 `onShareTimeline`，标题分别取 `simulator.title`、`salary.title`，卡片图取各自 `imageUrl`；path SHALL 落各自页面（H5 无原生转发入口，不新增 H5 分享 UI）。

#### Scenario: 模拟器页转发
- **WHEN** 用户在座位模拟器页经胶囊菜单转发
- **THEN** 分享卡标题为 `simulator.title`（后台可改），path 为 `/pages/fun/compass/simulator`

#### Scenario: 工资记账页转发
- **WHEN** 用户在工资记账页经胶囊菜单转发
- **THEN** 分享卡标题为 `salary.title`（后台可改），path 为 `/pages/fun/salary/index`

### Requirement: imageUrl 空值降级

当某页配置 `imageUrl` 为空串时，`onShareAppMessage` SHALL 不返回 `imageUrl` 字段（微信回退当前页截图），title 照常生效。

#### Scenario: 未配置分享图
- **WHEN** `salary.imageUrl` 为空串且用户转发工资记账页
- **THEN** 转发请求不含 `imageUrl`，标题仍为后台配置值
