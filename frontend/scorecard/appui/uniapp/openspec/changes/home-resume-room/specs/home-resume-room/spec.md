# home-resume-room Delta Spec

## Purpose

首页提供「回到牌局」手动回房入口：已登录且持有 lastRoomId 的用户，无论冷启动自动回房静默失败还是同会话从房间页 ⌂ 返回首页，都能从首页看到入口并回到进行中的房间。

## ADDED Requirements

### Requirement: 回到牌局入口展示

首页 SHALL 在用户已登录且 `lastRoomId > 0` 时展示「回到牌局」入口条；未登录或无 `lastRoomId` 时不展示该入口条。入口条文案 SHALL 含「回到牌局」字样。

#### Scenario: 已登录且有 lastRoomId 时展示入口

- **WHEN** 用户已登录且本地存有 `lastRoomId > 0`
- **THEN** 首页展示「回到牌局」入口条

#### Scenario: 未登录不展示

- **WHEN** 用户未登录（无 token），即使本地存有 `lastRoomId`
- **THEN** 首页不展示「回到牌局」入口条

#### Scenario: 无 lastRoomId 不展示

- **WHEN** 用户已登录但 `lastRoomId` 为 0（从未进房 / 已被清理）
- **THEN** 首页不展示「回到牌局」入口条

### Requirement: 点击入口校验房况后进房

点击「回到牌局」SHALL 先请求该房间快照校验房况：房间进行中（status=0）→ 进入房间页；房间已结束或已关闭（status=1/2）→ 清除 `lastRoomId` 并 toast 提示「牌局已结束」，留在首页。点击处理进行中 SHALL 防重复触发。

#### Scenario: 房间进行中进房

- **WHEN** 用户点击「回到牌局」且快照返回 status=0（进行中）
- **THEN** 跳转到该房间的房间页

#### Scenario: 房已结束清 id 留首页

- **WHEN** 用户点击「回到牌局」且快照返回 status=1 或 2（已结算/已关闭）
- **THEN** 清除本地 `lastRoomId`、入口条消失，toast 提示「牌局已结束」，停留首页

### Requirement: 回房失败处理

点击「回到牌局」失败时 SHALL 分类处理：业务错误 30101（房不存在）/ 30102（非成员）→ 清除 `lastRoomId` 并 toast 错误信息；网络或其他错误 → 保留 `lastRoomId`、toast 失败提示，入口条仍展示允许重试。

#### Scenario: 房不存在或非成员清理陈旧 id

- **WHEN** 点击「回到牌局」请求返回 30101 或 30102
- **THEN** 清除本地 `lastRoomId`，toast 错误信息，停留首页且入口条消失

#### Scenario: 网络错误可重试

- **WHEN** 点击「回到牌局」请求因网络错误失败
- **THEN** 保留 `lastRoomId`，toast 失败提示，入口条仍展示，用户可再次点击重试

### Requirement: 冷启动自动回房与手动入口共存

本启程首次以登录态显示首页时的自动回房 SHALL 保持既有 once 语义（尝试一次，不再重复）；手动「回到牌局」入口不受 once 限制，任何时刻可点击回房。scene 进房优先级 SHALL 高于自动回房与手动入口（scene 消费后本启程不再自动回房）。

#### Scenario: 同会话 ⌂ 回首页后可手动回房

- **WHEN** 用户在房间页按 ⌂ 回到首页（同一启程内，自动回房已尝试过）
- **THEN** 首页仍展示「回到牌局」入口，点击可回到进行中的房间

#### Scenario: 冷启动自动回房静默失败后可手动回房

- **WHEN** 冷启动自动回房因网络错误未能进房（静默留首页，保留 lastRoomId）
- **THEN** 首页展示「回到牌局」入口，点击可校验房况后进房
