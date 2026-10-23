# fishgame《摸鱼》游戏后端模块设计

> 日期：2026-10-23 ｜ 状态：待评审
> 需求与契约来源（**库外文件，以这两份为准**）：
> - `/Users/alex/Desktop/game/godot/fishgame/docs/backend-api.md` —— 接口契约精确字段形状
> - `/Users/alex/Desktop/game/godot/fishgame/docs/game-overview.md` —— 玩法背景与业务规则
>
> 本文是 alexGo-cloud 侧的实现设计；与来源契约冲突时，**客户端契约字段形状以 backend-api.md 为准**。

---

## 1. 意图与背景

《摸鱼》是 Godot 4.6 → 微信小游戏的休闲钓鱼游戏（离线优先）。客户端已实现全部后端交互
（`scripts/online_service.gd`）：`base_url` 填上后端地址即可联调。本模块给 alexGo-cloud 增加
一个游戏后端子系统，一次性实现全部契约端点，并在其上扩展三块来源文档没有、但需求明确要的能力：

1. **图鉴条目表后端可配**：48 条鱼种条目落库，客户端可拉取、运营可改（不发版改游戏数据）。
2. **玩家 dex 进度后端记账**：好友榜的唯一指标（图鉴点亮数）由后端维护，双路采集（见 §4.3）。
3. **管理后台**：vben 页面 + 菜单种子，覆盖图鉴配置、运营覆盖、埋点浏览、玩家/榜单、漏斗看板。

好友榜**不调微信关系链 API**（资质未取）：好友关系后端自存，无好友时回退同 app（同租户）
全体玩家维度；微信关系链留接口位（§4.6）。

## 2. 目标 / 非目标 / 反目标

**目标**
- 契约 7 端点全部实现，字段形状与 backend-api.md 逐字一致。
- 微信 code2session 真接（appid/secret 已具备），`X-Session` 自持会话体系。
- 图鉴条目可配 + dex 双路记账 + 后端自存好友榜。
- 管理后台页面 + 菜单/权限种子 + modules.md 回填。
- 响应快（正常 <500ms）、字段全（缺字段 = 客户端走本地 + backend_fail）。

**非目标（不做）**
- 微信关系链 getfriends 调用（留接口位）。
- 激励视频（客户端直连微信流量主，不经后端）。
- 分析看板的复杂 BI（只做基础漏斗/计数聚合，见 §4.9）。
- 多游戏/多 app 租户化运营（单小游戏，单租户数据，但表照仓库惯例带 `tenant_id`）。
- 对客户端失败重试的支持（客户端失败即放弃，服务端不做幂等重试风暴设计；但 track 落库
  本身幂等，见 §4.4）。

**反目标（做出来算失败）**
- 把 `/api/v1/*` 的裸 JSON 包进统一信封 `{"code":0,...}`——客户端 `JSON.parse_string` 后
  按形状校验，包信封 = 形状不符 = 全部静默走本地，联调白做。
- 移动/改写契约 7 端点的路径或字段名（客户端写死）。
- 让游戏 C 端接口依赖 `member_user` / oauth2 token（游戏身份是 openid，不是会员）。

## 3. 关键约束与既定决定

| # | 决定 | 依据 |
|---|---|---|
| 1 | 模块名 `fishgame`，目录 `modules/fishgame`，开关 `cfg.Modules["fishgame"]` | 仓库 snake_case 惯例；scorecard 同款开关语义 |
| 2 | 游戏客户端段挂 `/api/v1/*`（**契约锁死路径**），admin 段挂 `/api/admin/fishgame/*` | 契约路径不可挪；`RegisterRoutes` 入参是 `/api` group，`r.Group("/v1")` 可达 |
| 3 | `/api/v1/**` **裸 JSON 直出，豁免统一信封**（`pkg/resp` 不经手），错误也用裸形状 | 客户端形状校验语义；豁免先例：`/api/oauth2/*` RFC6749 信封、scorecard wxacode PNG 直出 |
| 4 | `/api/admin/fishgame/**` 走标准统一信封 + casbin + `user_type=1` 门槛 | 仓库 admin 惯例 |
| 5 | 游戏身份 = `openid`（code2session 换取），自持 `X-Session` 随机串会话，**不复用** oauth2/member token | 契约 §Auth 写死 `X-Session`；客户端只认这个头 |
| 6 | 微信 appid/secret 放 `config.yaml` 的 `fishgame.wechat.*` | 本模块自己的小游戏，与 oauth2 social 渠道表（另一 app）解耦；联调期改配置比改表轻 |
| 7 | 错误码段 **304xx**（301 scorecard / 302 online / 303 social 已占），仅 admin 段使用 | `pkg/errors/code.go` 分段惯例 |
| 8 | 表全带 `tenant_id` + `created_at`/`updated_at`，走 GORM 租户插件 | 不重蹈 `orders` 无 tenant_id 的 gap（modules.md §7.4） |
| 9 | 运营覆盖（运势/鱼王）服务端**自带同款确定性算法**兜底：无覆盖配置时按 epoch 天数/ISO 周 sha256 种子算出与客户端一致的值 | 契约要求 `config/daily` 响应字段恒全；客户端「服务端形状合法才覆盖」，空缺即走本地，服务端给出与本地一致的值最稳 |
| 10 | 脚手架 `make new-module NAME=fishgame ENTITIES=fish,player` 起手，实体 CRUD 骨架按业务重写（scorecard 先例：脚手架产物全部重写/删除） | usage.md §5；`-menu-prefix` 不可自定义 |
| 11 | 管理后台页面自写（4 业务页 + 1 看板页），**删除**脚手架生成的 CRUD 视图 | scorecard §11.9 先例 |
| 12 | `docs/modules.md` 不由脚手架自动改，收尾人工补 fishgame 章节 | usage.md §5.2 触点 7 |

## 4. 端点设计

### 4.1 游戏客户端段 `/api/v1`（裸 JSON，公开，`X-Session` 自鉴权）

除 `auth/login` 外，全部要求 header `X-Session: <token>`；缺失/无效/过期 →
**HTTP 401 + 裸 `{"error":"unauthorized"}`**（客户端判形状不符 → 走本地 + `backend_fail`，
正是契约预期的静默回退）。不走 casbin、不走统一信封、不回写 tenant（租户恒 0）。

| Method | Path | 请求 | 响应 | 说明 |
|---|---|---|---|---|
| POST | `/api/v1/auth/login` | `{"code":"..."}` | `{"session":"..."}` | 微信 `sns/jscode2session`（appid/secret 取 `fishgame.wechat.*`）→ openid → 建/取 player → 签发 session（随机 32B base64url，落表带过期）。微信接口失败 → 401 裸 `{"error":"login_failed"}` |
| GET | `/api/v1/config/daily` | — | `{"luck":{"period":"day"\|"dusk"\|"night"\|"any","depth_bias":0.0~0.3},"fish_king":{"fish_id":"<id>","week":"2026-W43"}}` | 先查运营覆盖表（见 §4.7），命中且在有效期内用覆盖；否则服务端确定性算法自算（epoch 天数/ISO 周 sha256），与客户端算法同款。`fish_id` 必须是图鉴表内真实 id（服务端校验后才下发覆盖） |
| POST | `/api/v1/daily/sign` | `{"date":"YYYY-MM-DD","streak":N}` | `{"ok":true,"bait":N}` / `{"ok":false}` | 同日重复 → `{"ok":false}`；否则按连击规则算 bait（隔 1 天 +1、隔 ≥2 天归 1、1/2/3 天 +1/+2/+3、4–6 天 +3、7 天起 +5），`(player,date)` 唯一键落库。`date` 作对账参考，服务端同时记服务器日期（防改表刷连击的权威口径留二期，当前信任客户端，契约原文） |
| GET | `/api/v1/basket?since=<unix秒>` | — | `{"bait":N,"capped":bool}` | 纯计算：`min(floor(elapsed/2400s), 12)`；`elapsed ≥ 8h` 时 `capped=true`（超时作废部分）。fire-and-forget 记账行（对账/审计用），不参与发奖 |
| GET | `/api/v1/friends/rank` | — | `{"list":[{"nick":"...","dex":N}]}` | 按 dex 降序。好友表非空 → 好友 ∪ 自己；为空 → 同租户全体玩家（同 openid/app 维度，见 §4.6）。`nick` 取 player 表昵称（微信授权资料，占位期用 `微信用户+openid尾4`） |
| POST | `/api/v1/track` | `{"events":[{"name":"...","t":<epoch秒>,"props":{...}}]}` | **204 无 body** | 批量落库 + 从 `catch` 事件推导 dex（§4.3）。只认 2xx、空 body 合法（契约）；单条非法（缺 name/t）跳过该条不整批拒；整批非法 → 仍 204（客户端不关心 body） |
| POST | `/api/v1/share/image` | multipart（字段 `image`） | `{"url":"https://..."}` | 分享卡图托管。存 `pkg/storage` 驱动，落元数据行，URL 指向本服务公开读端点（§4.5）。客户端当前未接线（预留），实现照契约 |

**新增客户端端点**（契约没有，为三项扩展能力而加；路径同挂 `/api/v1`，客户端后续接线）：

| Method | Path | 请求 | 响应 | 说明 |
|---|---|---|---|---|
| GET | `/api/v1/dex/table` | — | `{"list":[{"id":"...","name":"...","rarity":"normal\|misc\|rare\|epic","period":"day\|dusk\|night\|any","depth":"shallow\|mid\|deep","flavor":"..."}]}` | 图鉴条目表下发（运营改完客户端下次拉取即生效）。只发 `status=1` 条目 |
| POST | `/api/v1/dex/report` | `{"dex":{"<fish_id>":N,...}}` | `{"ok":true,"dex":<点亮总数>}` | 整表上报（存档里的 `{条目id: 钓获次数}` 字典），与 catch 推导**合并取 max(count)**，未知 fish_id 忽略（客户端表可能滞后于运营改动） |
| POST | `/api/v1/friends` | `{"openid":"..."}` | `{"ok":true}` | 添加好友（好友关系后端自存的写入口；客户端 UI 后接）。自己加自己 / 目标不存在 → 401 之外统一 400 裸 `{"error":"..."}`，幂等（已存在返回 ok） |

> **信封豁免的实现方式**：`/api/v1` 段 handler 直接 `c.JSON(...)` / `c.Status(204)`，
> 不调 `pkg/resp.OK/Fail`。admin 段正常走 `pkg/resp`。

### 4.2 管理端段 `/api/admin/fishgame`（统一信封 + casbin）

权限前缀注册进 `modules/system/service/permission_routes.go`（**必须登记，否则死权限 403**，
`permission_routes_test.go` 会兜底）：

| 前缀 | 路由 |
|---|---|
| `fishgame:fish` | `/api/admin/fishgame/fishes`, `.../fishes/*` |
| `fishgame:config` | `/api/admin/fishgame/configs`, `.../configs/*` |
| `fishgame:event` | `/api/admin/fishgame/events`, `.../events/*` |
| `fishgame:player` | `/api/admin/fishgame/players`, `.../players/*` |
| `fishgame:stats` | `/api/admin/fishgame/stats` |

| Method | Path | 权限码 | 说明 |
|---|---|---|---|
| GET | `/fishes` | `fishgame:fish` | 图鉴条目分页（`rarity`/`status` 筛选，`page`/`size`≤100） |
| POST | `/fishes` | `fishgame:fish` | 新建条目（`fish_id` 查重；`rarity`/`period`/`depth` 枚举校验） |
| PUT | `/fishes/:id` | `fishgame:fish` | 更新条目（含 status 启停） |
| DELETE | `/fishes/:id` | `fishgame:fish` | 删除（已被 dex/事件引用的 id 拒删 → 30402，改启停即可） |
| GET | `/configs` | `fishgame:config` | 运营覆盖列表（运势 + 鱼王两类） |
| POST | `/configs` | `fishgame:config` | 新建覆盖：`type=luck`（`period`+`depth_bias`≤0.3）或 `type=fish_king`（`fish_id`+`week`，fish_id 必须表内真实存在 → 30403） |
| PUT | `/configs/:id` | `fishgame:config` | 更新（含启停 status） |
| DELETE | `/configs/:id` | `fishgame:config` | 删除 |
| GET | `/events` | `fishgame:event` | 埋点浏览分页（`name`/`player_id`/起止时间筛选，倒序） |
| GET | `/players` | `fishgame:player` | 玩家分页（openid/nick 模糊搜，行含 `dex` 点亮数、注册/活跃时间） |
| GET | `/players/:id` | `fishgame:player` | 玩家详情：基本信息 + 点亮明细（fish_id → count）+ 最近 20 条事件 |
| GET | `/players/:id/rank` | `fishgame:player` | 该玩家当前榜单位次（复用 §4.1 rank 逻辑，便于客服排查「榜不见了」类问题） |
| GET | `/stats` | `fishgame:stats` | 看板聚合：玩家总数/今日新增、事件按 name 计数（近 7 日）、四类钓鱼结果漏斗（catch/empty/escape/broken 计数与占比）、签到/领筐/广告事件计数 |

### 4.3 dex 双路采集（合并规则）

- **路 A（零客户端改动）**：`track` 落库时解析 `name=catch` 且 `props.kind=="fish"` 的事件，
  按 `props.fish_id` upsert `fishgame_dex (player_id, fish_id)` `count = count + 1`。
  仅当该事件**首次插入**（dedup 命中跳过）才递增——防止客户端失败重发同一快照导致膨胀
  （契约：track 失败保留队列下次再发）。`fish_id` 不在图鉴表 → 记 warn 跳过（客户端表
  可能旧于运营改动）。
- **路 B（新端点）**：`POST /api/v1/dex/report` 整表合并：逐条 `count = max(旧, 上报)`，
  上报集合内出现的新条目插入。未知 fish_id 忽略。
- **点亮数 dex** = 该 player 在 `fishgame_dex` 的行数（`count ≥ 1`）。榜单/详情都从这张表聚合。

### 4.4 track 落库与幂等

- 表 `fishgame_track_event`：`player_id, name, t, props(json), dedup_key, created_at, tenant_id`。
- `dedup_key = sha1(openid + "|" + name + "|" + t + "|" + canonical_json(props))`，**唯一键**，
  `INSERT ... ON DUPLICATE KEY IGNORE`（gorm 先查后插亦可，唯一键是最终防线）。
- 上限防御：单批 `events` 截 200 条（客户端 aq 上限 100，防御畸形请求）；props 截 4KB。
- 索引：`(name, t)`、`(player_id, t)`、`(created_at)`（看板与筛选）。

### 4.5 分享图托管

- 上传走契约 multipart → `pkg/storage`（`Driver.Save`，复用 infra file 的存储驱动，
  **不复用** `system_files` 表——文件归属本模块，独立元数据行 `fishgame_share`）。
- 扩展名白名单 png/jpg/webp，≤5MB，双 MIME 校验（照 `file_service.go` 策略）。
- 公开读端点 `GET /api/public/fishgame/share/:key`（免鉴权、inline、随机 key、
  不存在统一 30010 语义但走裸 404——本段不在信封豁免清单内，按 `pkg/resp` 常规即可）。
- 响应 `{"url":"<绝对 URL>"}`：由请求 `Host` + https 拼接（`fishgame.public_base_url`
  配置可覆盖，部署在反代后用得上）。

### 4.6 好友榜口径（我的解读，已确认方向）

- 表 `fishgame_friend (player_id, friend_player_id)` 唯一键，双向各存一行（加好友时成对写）。
- `GET /api/v1/friends/rank`：
  - 好友表非空 → 取「我的好友 ∪ 我」的 dex 聚合，降序；
  - 好友表为空 → 同租户**全体玩家**降序（「同 openid 维度凑一版」= 同小游戏 app 的所有玩家）；
  - 离线回退语义在客户端（服务端只要 5s 内给形状合法的响应）。
- **微信关系链留位**：`RankSource` 走接口 `FriendSource`（`FriendsOf(ctx, openid) []openid`），
  一期实现 `StoredFriendSource`（库内关系），二期可插 `WeChatRelationSource` 而不动 handler。
- `POST /api/v1/friends` 的目标 openid 由客户端从分享卡片/口令拿到（客户端 UI 后接）；
  服务端不校验微信关系。

### 4.7 运营覆盖（config/daily）

- 表 `fishgame_config`：`type`（`luck`/`fish_king`）、`period`、`depth_bias`、`fish_id`、
  `week`、`start_at`/`end_at`（可空 = 永久）、`status`、`tenant_id`。
- 读取优先级：**有效期内 status=1 的覆盖 → 服务端确定性自算**。
  - luck 自算：`sha256(当日 epoch 天数)` → period 四选一 + `depth_bias ∈ [0, 0.2]`
    （客户端钳 0.3，服务端给 0~0.2 更安全）；
  - fish_king 自算：`sha256(ISO 周文本 YYYY-Www)` → rare+epic（15 条）中确定性选一。
    **自算依赖图鉴表**：rarity 过滤取当前 `status=1` 的 rare/epic 条目；表内不足 15 条照常算。
- 覆盖的 `fish_id` 建/改时校验存在于图鉴表（→ 30403）；被删/停用后读取时二次过滤，
  不合法则回退自算（保证响应恒合法）。

### 4.8 登录与会话

- `POST /api/v1/auth/login`：`sns/jscode2session?appid&secret&js_code&grant_type=authorization_code`
  （HTTP 5s 超时）→ `openid` → `fishgame_player` 按 `(tenant_id, openid)` upsert →
  签发 `fishgame_session`（token 随机 32B、`expire_at = now + 30d`，落表）→ `{"session":"<token>"}`。
  微信 errcode ≠ 0 / 超时 → 401 `{"error":"login_failed"}`（客户端形状不符走本地）。
- 中间件（模块内自建，非全局）：`/api/v1/*`（除 login）解析 `X-Session` → 查 session 表
  （未过期）→ 把 player 塞进 `c.Set("fishgame_player", ...)`；无效 → 401 裸 JSON。
- session 过期滑续：命中时若剩 <7d 顺延 `expire_at`（低成本保活）。
- **与 oauth2 完全无关**：不签 member token、不查 `member_user`、不走 `UnifiedLogin`。

### 4.9 看板聚合（admin `/stats`）

按日聚合近 7 日：事件 name 计数（`GROUP BY name, date`）、四类结果漏斗
（catch/empty/escape/broken）、玩家总数与当日新增（按 player 注册日）、签到/领筐/广告计数。
纯 SQL 聚合，无预聚合表（量级：小游戏 DAU，直接查 + `(name, t)` 索引够用）。

## 5. 数据模型（8 表，迁移 `modules/fishgame/migrations/`）

全部 `CREATE TABLE IF NOT EXISTS`、无外键、带 `tenant_id` + `created_at`/`updated_at`，
up/down 对称，照 scorecard 惯例。

| 表 | 关键列 | 说明 |
|---|---|---|
| `fishgame_player` | `openid`（uk 含 tenant）、`nick`、`avatar`、`last_login_at` | 游戏账号 = openid；昵称清 `:` 防串（member 同款教训） |
| `fishgame_session` | `token`（唯一）、`player_id`、`expire_at`、索引 `(expire_at)` | 过期行由惰性判定兜底；不设清扫 ticker（量小，惰性够） |
| `fishgame_fish` | `fish_id`（uk 含 tenant）、`name`、`rarity`、`period`、`depth`、`flavor`、`sort`、`status` | 48 条目；**种子数据取自客户端 `fish_db.gd` 的 `ENTRIES`**（实现时从 `/Users/alex/Desktop/game/godot/fishgame/scripts/fish_db.gd` 抄录，id 逐字一致——id 是后端唯一认可标识） |
| `fishgame_dex` | `player_id`+`fish_id` 唯一、`count` | 点亮进度；dex = 行数 |
| `fishgame_track_event` | `player_id`、`name`、`t`、`props`、`dedup_key`（唯一） | 追加型事实流；索引见 §4.4 |
| `fishgame_sign` | `player_id`+`date` 唯一、`streak`、`bait` | 签到对账 |
| `fishgame_friend` | `player_id`+`friend_player_id` 唯一 | 双向成对写 |
| `fishgame_config` | `type`、`period`、`depth_bias`、`fish_id`、`week`、`start_at`/`end_at`、`status` | 运营覆盖 |
| `fishgame_share` | `key`（唯一）、`player_id`、`size`、`mime` | 分享图元数据（文件本体在 storage 驱动） |

鱼篓不落表（纯计算 + 每次请求记审计可选——**决定：不落表**，契约明说后端不做合并、
不参与发奖，落表只增写放大；埋点 `basket_claim` 已在 track 里可审计）。

## 6. 装配与配置

```yaml
# alexgo-server/configs/config.yaml
modules:
  fishgame: true
fishgame:
  wechat:
    appid: "wx..."        # 已具备
    secret: "..."         # 已具备
  public_base_url: ""     # 空 = 由请求 Host 拼接（share/image 的 url）
```

- `modules/fishgame/module.go`：`FxModule`（Provide repository/service/controller +
  `server.Module` + `migrate.Source`），照 scorecard 结构；`RegisterRoutes` 里
  `cfg.Modules["fishgame"]==false` 直接 return。
- 装配进 `alexgo-server/cmd/main.go`（脚手架 AST 插入）；迁移源进 `group:"migration_sources"`。
- 无后台 ticker、无 gRPC、无事件、无跨模块窄接口依赖（不依赖 member/token/oauth2——
  这是本模块与既有模块的最大结构差异，micro 形态天然可装配）。
- 微信 HTTP 客户端：模块内 `wechat.go`，5s 超时、只依赖 appid/secret 配置。

## 7. 前端（apps/web-antd，5 页）

| 路由 | 组件 | 功能 | 权限码 |
|---|---|---|---|
| `/fishgame/fishes` | `views/fishgame/FishgameFishesPage.vue` | 图鉴条目分页 + 新建/编辑/删除（rarity/period/depth 下拉、启停） | 菜单行 `fishgame:fish` |
| `/fishgame/configs` | `FishgameConfigsPage.vue` | 运营覆盖列表 + 新建/编辑/删除（type 切换 luck/fish_king，鱼王 fish_id 下拉取自条目表） | `fishgame:config` |
| `/fishgame/events` | `FishgameEventsPage.vue` | 埋点浏览（name/时间/player 筛选，props JSON 折叠展示） | `fishgame:event` |
| `/fishgame/players` | `FishgamePlayersPage.vue` | 玩家分页 + 详情抽屉（点亮明细 + 最近事件 + 榜单位次） | `fishgame:player` |
| `/fishgame/stats` | `FishgameStatsPage.vue` | 看板：总数卡、近 7 日事件趋势、四结果漏斗 | `fishgame:stats` |

- API 封装 `src/api/fishgame.ts`（`requestClient` 统一信封，照 `scorecard.ts` 模式）。
- 菜单种子：**system 模块迁移**新建 SQL 对（幂等 `NOT EXISTS`，照
  `20261020000001_scorecard_menus.up.sql`）：目录「Fishgame」`/fishgame`（perm `fishgame`、
  sort 85、直挂根级）+ 5 页面（英文占位名，后台可改中文）；启动 seed 全量绑 admin。
- 页内不使用 `v-access`（可见性靠菜单、操作靠 casbin，scorecard 同款）。
- 脚手架生成的实体 CRUD 视图**全部删除**，只留上表 5 页。

## 8. 错误码（304xx，仅 admin 段；`pkg/errors/code.go` 注册 + 文案进 registry）

| 码 | 文案 | 语义 |
|---|---|---|
| 30401 | 图鉴条目不存在 | fish 查无/跨租户（软查 nil 统一映射，scorecard 惯例） |
| 30402 | 条目已被进度或事件引用，不可删除 | 被 dex/track_event 引用 → 引导改启停 |
| 30403 | 鱼王指定的条目不存在或已停用 | config 建/改时 fish_id 校验 |
| 30404 | 运营配置不存在 | config 查无/跨租户 |

游戏端 `/api/v1` 段**不用五位码**：错误一律裸 `{"error":"<英文短语>"}` +
适当 HTTP 状态（401/400），客户端只做形状判定。

## 9. 明确不做 / 二期留位

- 微信关系链好友榜（`FriendSource` 接口位已留）。
- 签到 date 的服务端权威时区推导防刷（契约：当前信任客户端，仅记录参考）。
- `share/image` 的 CDN 化与过期清理（表已带时间，清理 ticker 二期）。
- session 泄露吊销的管理端能力（量小，二期）。
- 埋点实时告警/导出（Excel 导出可复用 `pkg/excel`，二期）。
- 图鉴条目的多语言、客户端表 diff 增量下发（全量 48 条几十 KB，一期全量）。

## 10. 实现顺序建议（供 plan 参考）

1. 脚手架 `make new-module NAME=fishgame ENTITIES=fish,player` 起手（触点 1-6 落盘 + build 过）
2. 迁移 9 表 + 图鉴种子（从 `fish_db.gd` 抄 ENTRIES）
3. 微信 client + session 中间件 + `auth/login`
4. `config/daily`（覆盖表 + 确定性自算）+ `daily/sign` + `basket`
5. `track`（dedup 落库）+ dex 双路（catch 推导 + report）+ `dex/table`
6. 好友（add + rank 双口径）+ `share/image` + 公开读
7. admin 五组端点 + 错误码 + casbin 前缀登记
8. 前端 5 页 + `api/fishgame.ts` + 菜单种子迁移
9. 收尾：modules.md 补 §12 章节、禁词/构建回归、契约逐字段核对（对照 backend-api.md）
10. 客户端联调清单（`base_url` 填上 → Network 看 `/track`；下发指定 fish_id 验证鱼王切换）

## 11. 验收口径

- 契约 7 端点 + 3 新端点字段形状与 §4.1 逐字对照通过（人工核对 backend-api.md）。
- `/api/v1/**` 响应**无信封**（测试断言无 `code` 字段顶层键）。
- `go build ./...` + `go test ./...` 通过；`permission_routes_test` 覆盖 5 个新前缀。
- 菜单种子幂等（重复迁移不重复插入）；全新库启动后 admin 角色可见 5 页面。
- dex 双路：同一 catch 事件重复上报 count 不膨胀；report 与 catch 交集取 max。
- 离线语义反向验证：关掉 `modules.fishgame` 后 `/api/v1/*` 404，客户端走本地不受影响。
