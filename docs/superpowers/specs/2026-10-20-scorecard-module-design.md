# scorecard — 打牌记账业务模块设计

日期：2026-10-20
状态：已确认（六节设计逐节获批）
需求来源：[`docs/modules/dapaijizhang.md`](../../../modules/dapaijizhang.md) V1.2（微信小程序侧不在本仓库）

## 背景与目标

alexGo-cloud 需落地「打牌记账」的后端业务：线下打牌纯分数记账工具的房间、计分、茶饭、结算、战绩、好友全链路。产品红线：**纯分数、不涉及金钱**，代码/字段/接口命名一律 `score`/`tea`，规避资金类词汇（文档 §2）。

**成功标准**：`go build` + `go test ./...` 通过；app 端业务接口可用；admin 监管页可查可运维；菜单/权限落库；`docs/modules.md` 回填。

## 已确认决策

| # | 决策项 | 结论 |
|---|---|---|
| 1 | 交付边界 | **A**：后端全量业务 API（app 端）+ admin 只读监管 + 强制关房。**不含**小程序前端、WebSocket、语音、广告、微信 openid 登录、二维码生成 |
| 2 | 模块名 | **`scorecard`**（目录 `modules/scorecard`、端点前缀 `/api/app/scorecard` 与 `/api/admin/scorecard`、权限前缀 `scorecard:*`、菜单路径 `/scorecard`） |
| 3 | 用户体系 | **复用 member 模块**：不自建用户表/登录；app 端 Bearer token → `token.Validator` → `Claims{UserID,TenantID}`；微信相关（openid 登录等）**后续单独建模块**，本模块只留干净接缝（用户一律 `member_user.id`） |
| 4 | admin 范围 | 只读监管（房间/战绩/好友列表 + 房间详情）+ 唯一运维写动作「强制关房」 |
| 5 | 实现路线 | **方案 1**：`make new-module` 脚手架打底（原子落定 main.go 装配、config 开关、权限码登记、菜单种子触点）+ 手工业务深化；脚手架生成的 6 列示例实体层（模型/DDL/CRUD）**全部重写** |
| 6 | 实时同步 | 无 WS：**快照轮询 + 乐观锁 version**，以快照为准（文档 §9 语义在无 WS 下成立） |
| 7 | `record` 结构 | **每成员一行**（关系型、可统计），不用 json；写入时机见 §3.4 |
| 8 | tenant_id | **全部 6 表带 `tenant_id`**，走 GORM 租户插件隔离（不重蹈 `orders` 无隔离的 gap） |

## 1. 总体架构与鉴权

### 1.1 装配

- 新建 `modules/scorecard/`（module / controller{admin,app} / service / repository / model + migrations），fx 装配进 `alexgo-server/cmd/main.go`，`config.yaml` 加 `modules.scorecard: true` 开关。
- `RegisterRoutes` 挂载两组：`/api/app/scorecard/**`、`/api/admin/scorecard/**`（与 member 模块同构）。

### 1.2 鉴权（两套，沿用仓库既有模式）

| 端 | 模式 |
|---|---|
| app | handler 内手工解析 `Authorization: Bearer` → `token.Validator.Validate` → 校验 `UserType==Member` + 取 `UserID/TenantID`。不走 casbin（与 member app 端同构）。未登录/无效 → `2xxxx` 码段 |
| admin | 管理员 token + casbin（user_type=1 门槛中间件已有），三组前缀登记 `permission_routes.go`：`scorecard:room` / `scorecard:record` / `scorecard:friend`（脚手架自动插；**死权限陷阱**：前缀未登记则 403，有 `permission_routes_test` 兜底） |

### 1.3 跨模块依赖

- scorecard 展示成员/好友昵称头像 → **member 模块新增窄只读接口** `BriefProvider`（`GetBriefs(ctx, ids []uint64) → []{ID, Nickname, Avatar, Status}`），fx 注入消费。沿用 order → system `UserService.GetUserByID` 先例，不跨模块查表。
- 依赖面收敛：`pkg/resp`（信封）、`pkg/errors`（五位码）、`pkg/token`、member `BriefProvider`。

### 1.4 实时一致（无 WS）

- `GET /rooms/:id/snapshot` 返回全量状态（version、成员分、配置、茶饭池、日志游标）。
- 支出/变更响应带回最新 `version` + 受影响数据；客户端拉快照对齐；`version` 单调递增去重/丢弃过期。

## 2. 数据模型

表名照文档 §7 原名（单数口径；仓库已有 `orders`/`member_user` 混用先例）。全部 6 表带 `tenant_id`；均含 `created_at`/`updated_at`。

| 表 | 关键列（相对脚手架 6 列骨架的差异） |
|---|---|
| `room` | `room_code`(varchar 唯一，4 位字母数字)、`creator_id`(当前房主)、`status` 0 进行中/1 已结算/2 已关闭、`expire_time`、`tea_target_amount`、`tea_percent` decimal(5,2)、`tea_pool_total`、`tea_auto_enabled`、`max_lose_score`、`round_no`、`pending_tea_target_amount`/`pending_tea_percent`/`pending_max_lose_score`/`pending_apply_round_no`、**`version`** 乐观锁 |
| `room_user` | `room_id`+`user_id`、`cur_score`、`cum_lose_score`、`is_creator`、`session_no`；唯一键 `(room_id,user_id)` |
| `room_log` | `room_id`、`from_user_id`、`to_user_id`、`score`、`tea`、`type` 1 记分/2 茶饭/3 进房/4 退房/5 结算/6 上限提醒/7 配置变更/8 状态变更/9 转让房主、`client_request_id`(幂等键)、`extra` json；索引 `(room_id,created_at)` |
| `record` | 每成员一行：`room_id`、`user_id`(归属人)、`score`(终局分)、`cum_lose_score`、终局配置冗余 `tea_pool_total`/`tea_percent`/`tea_target_amount`/`max_lose_score`、`settle_type` 1 正常结算/2 房主退房自动结算/3 退房/4 零数据空战绩；索引 `(user_id,created_at)`、`(room_id)` |
| `friend` | `user_id`+`friend_user_id`、`rank_enabled`；唯一键 `(user_id,friend_user_id)` |
| `friend_black` | `user_id`+`blocked_user_id`；唯一键同上 |

**落地说明**：

- `room_log.extra`（json）保留——流水扩展备注，非统计数据（文档原样）。
- `record` 房间级信息（status/settle_by 等）留在 `room` 表，不重复进 record；统计查询不 JOIN（终局配置已冗余快照）。
- 脚手架生成的示例 DDL/模型/CRUD 全部手工覆盖为上表；`migration_source.go` 保留脚手架产物，迁移 SQL 幂等 `CREATE TABLE IF NOT EXISTS`。
- 迁移对放 `modules/scorecard/migrations/`，`make migrate` 注册执行。

## 3. 核心业务规则

### 3.1 支出事务（`POST /rooms/:id/expense`，文档 §5.3 流程）

单个 DB 事务内依序：

1. 鉴权成员 + `room.status=0`（已结算/已关闭 → `3xxxx` 业务错误）。
2. **幂等**：`(room_id, client_request_id)` 命中 `room_log` → 返回上次结果。
3. **并发**：`SELECT ... FOR UPDATE` 锁 room 行 + `WHERE version=?` 乐观锁递增；同房间支出串行（总分守恒）。
4. 应用到期 pending：`round_no+1` 后若 `pending_apply_round_no <= round_no`，刷入生效字段并清 pending。
5. 记账：输家 `cur_score-=S`、`cum_lose_score+=S`；赢家 `cur_score+=S-T`；茶饭 `tea_pool_total+=T`，`T=round(S×percent)`（总分守恒模型，文档 4.4 默认口径）。
6. 达标：`tea_pool_total >= tea_target_amount(>0)` → `tea_auto_enabled=0` + 日志提示（文档决策 4）。
7. 上限频控：`cum_lose_score >= max_lose_score(>0)` → 首次全房提醒（type=6），后续仅写日志不重复播报（文档 4.5）。
8. 写 `room_log(type=1)`，提交；响应带 version + 全员最新分 + 茶饭池。

手动 DIY 茶饭：请求带 `tea` 且未开自动百分比时，从 `tea_from_user_id`（缺省=支出方）扣 `T` 进池、该成员 `cur_score-=T`（文档 4.4-4；`T` 不计入 `cum_lose_score`）。

### 3.2 配置修改（「从下把生效」，文档 §5.2）

只写 `pending_*` + `pending_apply_round_no = 当前 round_no + 1`；当前把不受影响，下一笔支出时刷入。

### 3.3 结算 / 退房 / 转让房主 / 超时（文档 §5.5 边界）

| 动作 | 行为 |
|---|---|
| 结算（房主） | `status 0→1`，**批量写全员 `record` 行**，禁再记分；type=5 日志 |
| 普通玩家退房 | 移除 `room_user`、type=4 日志、**为自己写一行 `record`**（`score` 按当时 `cur_score`；全零 `settle_type=4` 空战绩）；再进房 = 新会话（`session_no+1`、`cum_lose_score` 清零，文档决策 2） |
| 房主退房 | 事务内先结算再退房；房间 `status=1` 只读（文档决策 5） |
| 转让房主 | 校验 `status=0`、操作人 `is_creator=1`、目标在房内、双方不互拉黑 → `creator_id` 换、两行 `is_creator` 互换、type=9 日志；V1 直接转让不需对方确认（文档决策 8） |
| 全员退房 | `status=2` |
| 超时关房 | 后台 ticker（复用 system `SweepExpired` 先例）每分钟扫 `status=0 AND expire_time < now` → `status=2` + type=8 日志；「无操作」= 支出/进房/退房/配置变更时刷 `expire_time = now+2h`（心跳/刷新不刷，文档 5.1） |

### 3.4 `record` 写入时机

| 时机 | 行数 | 说明 |
|---|---|---|
| 结算 | 全员 N 行 | 正常/房主退房自动结算 |
| 玩家退房 | 仅本人 1 行 | 终局分按当时 `cur_score` |
| 零数据退房 | 1 行 0 值 | `settle_type=4` 空战绩（文档决策 6） |

### 3.5 好友

增删查 / 拉黑 / 排名开关基于 `friend`、`friend_black`；拉黑**双向**拦截进房与好友添加（文档 4.8）。

## 4. 端点清单

### 4.1 app 端 `/api/app/scorecard`（Bearer，全部）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/rooms` | 开房（房号 4 位冲突重试；可带默认配置；创建者入房 `is_creator=1`） |
| POST | `/rooms/join` | 按 `room_code` 进房（幂等返回现状；拉黑校验；`session_no+1` 清累计） |
| GET | `/rooms/:id/snapshot` | 快照（version/成员/配置/茶饭池/最近日志；昵称头像走 BriefProvider） |
| PUT | `/rooms/:id/config` | 改配置（写 pending）；仅房主 |
| POST | `/rooms/:id/expense` | 支出（score/tea/to_user_id/client_request_id/`tea_from_user_id` 可选）；§3.1 事务。tea>0 且未开自动百分比 = 手动 DIY：从 `tea_from_user_id`（缺省=支出方）扣 T 进池 |
| GET | `/rooms/:id/logs` | 流水分页（size≤100） |
| POST | `/rooms/:id/settle` | 结算；仅房主 |
| POST | `/rooms/:id/leave` | 退房（房主自动先结算） |
| POST | `/rooms/:id/transfer-owner` | 转让房主，body `target_user_id` |
| GET | `/records` | 我的战绩（时间倒序分页） |
| GET | `/records/:recordId` | 战绩明细（按 room_id 反查全房成员行） |
| DELETE | `/records/:recordId` | 删除（仅本人行） |
| GET | `/friends` | 好友排名列表 |
| POST | `/friends` | 加好友（校验存在/未拉黑） |
| DELETE | `/friends/:userId` | 删除好友 |
| POST | `/friends/:userId/black` | 拉黑/解除（body `blocked` 0/1） |
| PUT | `/friends/:userId/rank` | 排名参与开关 |
| GET | `/stat` | 我的统计（总积分/赢输场次/胜率，聚合 record） |

### 4.2 admin 端 `/api/admin/scorecard`（casbin）

| 方法 | 路径 | 前缀 | 说明 |
|---|---|---|---|
| GET | `/rooms` | `scorecard:room` | 房间分页列表（状态/房号筛） |
| GET | `/rooms/:id` | `scorecard:room` | 房间详情（成员+最近流水） |
| POST | `/rooms/:id/close` | `scorecard:room` | 强制关房（唯一运维写动作） |
| GET | `/records` | `scorecard:record` | 战绩分页（用户/房间筛） |
| GET | `/friends` | `scorecard:friend` | 好友/拉黑关系分页（只读） |

### 4.3 错误码

沿用五位分段（1xxxx 参数 / 2xxxx 鉴权 / 3xxxx 业务 / 5xxxx 系统）；统一 `pkg/resp` 信封。房号冲突、转让目标不在房、已结算后记分等走 3xxxx 中文文案。

## 5. 前端监管页（apps/web-antd）

脚手架按实体生成的 CRUD 页**全部弃用重写**为 4 页；`src/api/scorecard.ts` 保留骨架后收敛（requestClient 统一信封）。

| 路由 | 组件 | 功能 | v-access |
|---|---|---|---|
| `/scorecard/rooms` | `views/scorecard/ScorecardRoomsPage.vue` | 房间分页列表（筛选+详情入口+强制关房二次确认） | `scorecard:room` |
| `/scorecard/rooms/detail` | `RoomDetailPage.vue`（query id） | 房间信息+成员表+流水分页 | `scorecard:room` |
| `/scorecard/records` | `ScorecardRecordsPage.vue` | 战绩分页+明细弹窗（全房成员行） | `scorecard:record` |
| `/scorecard/friends` | `ScorecardFriendsPage.vue` | 好友/拉黑关系只读列表 | `scorecard:friend` |

- 页面惯例：`VxeGrid` + `Page` + `useVbenModal`。
- 菜单：目录「打牌记账」`/scorecard` → 3 页面菜单；种子由脚手架生成（system migrations，英文占位名后台可改中文）。
- 按钮级权限**不拆**（后端只认前两段前缀，拆了不增加隔离）。

## 6. 测试与验收

### 6.1 单测（service 层）

- 支出：总分守恒、茶饭入池、幂等重放、version 冲突、达标自动关、上限频控（首次全房/后续仅日志）。
- 「从下把生效」：改配置当前把不变、下一笔生效（文档 12.1-5 数字用例）。
- 结算/退房/转让：房主退房自动结算、零数据空战绩、退房重进清零、转让校验矩阵。
- record 写入时机与统计聚合（胜率）。

### 6.2 集成

- `go build ./...` + `go test ./...`；迁移幂等；三组新权限前缀的 policy 断言。

### 6.3 验收对照

文档 §12.1 的 13 条用例逐条映射；扫码/语音/断网/锁屏等小程序端用例标注「前端职责，不在本仓库验收」。

## 范围外（Out of Scope）

小程序前端、WebSocket、语音 5 档、广告位、二维码生成、微信 openid 登录（**后续单独建模块**）、资金结算（产品红线）、战绩自定义统计（V2）、语音偏好服务端同步（V2）、踢人（产品明确无此权限）。

## 实现顺序（供 writing-plans 参考）

1. `make new-module NAME=scorecard ENTITIES=room,room_user,room_log,record,friend,friend_black` 打底。
2. member 加 `BriefProvider` 窄接口 + fx 映射。
3. 重写迁移 DDL + model（6 表）。
4. service 核心：支出事务 → 配置 pending → 结算/退房/转让 → 超时 ticker → 好友/战绩/统计。
5. app 控制器（Bearer 鉴权）+ admin 控制器。
6. 前端 4 页 + api 收敛。
7. 测试补齐 + `docs/modules.md` 回填。
