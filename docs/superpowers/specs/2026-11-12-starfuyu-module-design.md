# starfuyu 星座运势后端模块设计（goLucky 融入）

> 日期：2026-11-12 ｜ 状态：已定稿（builder-check 一轮已回，3 项产品裁决 + 2 项设计定调已并入）
> 需求与来源（**库内待迁移文件，以这两处为准**）：
> - `modules/starfuyu/goLucky/` —— 被迁移的独立后端（约 4000 行 Go，Gin + GORM + MySQL）
> - `modules/starfuyu/goLucky/README.md`、`docs/生辰八字求签功能开发规划.md` —— 功能口径
>
> 本文是 alexGo-cloud 侧的实现设计；与 goLucky 原实现冲突时，**业务语义以 goLucky 源码为准**，
> 鉴权/信封/租户等横切惯例以本仓库为准。`luckyU` 小程序前端**不在本期范围**（后续单独处理）。

---

## 1. 意图与背景

goLucky 是一个独立部署的星座运势后端：每日 DeepSeek 抓取 12 星座运势、gg 绘制运势图片、
生成微信公众号格式文章并传草稿箱、微信小程序登录（自有用户表 + JWT）、生辰八字计算、
求签解签（带每日限流），外加两条凌晨定时任务。现把它转化为 alexGo-cloud 的标准业务模块
`modules/starfuyu`，与 fishgame（摸鱼小游戏）同款路径融入：

1. **用户并进 member 会员体系**——弃用 goLucky 自有用户表与 JWT，C 端身份一律
   `member_user.id`，登录走仓库现成的微信小程序 social 登录
   （`POST /api/app/social/wechat/mp/login`，见 modules.md §6.7）。
2. **存量数据不迁移**——golucky 旧库不搬；迁移只建新表，从零开始。
3. **双端都做**——C 端业务接口（`/api/app/starfuyu`）+ 管理端点
   （`/api/admin/starfuyu`：公众号文章、手动触发任务、运势查询）；管理端前端页面后续补。

## 2. 目标 / 非目标 / 反目标

**目标**
- goLucky 全部业务能力在模块内等价重现：运势抓取/查询、图片生成、公众号文章与草稿箱上传、
  八字计算、求签解签、每日限流、两条定时任务。
- C 端鉴权走 member Bearer（scorecard app 同构 `claims()` 自鉴权），admin 走 casbin。
- 标准模块触点齐全：fx 装配、`server.Module.RegisterRoutes`、迁移源、`cfg.Starfuyu` 配置、
  `cfg.Modules["starfuyu"]` 开关、错误码段、`docs/modules.md` 回填。
- 脚手架 `tools/newmodule` 起手，实体骨架按业务重写（scorecard/fishgame 先例）。

**非目标（不做）**
- luckyU 小程序前端的任何改动（含登录方式切换）——后续票。
- admin 管理前端页面（vben 页面 + 菜单种子可随后续票补；本期只落 admin API）。
- goLucky 存量数据迁移。
- goLucky 自有 JWT / `users` 表 / `/api/auth/login` / `/api/test/token` 的保留。
- Swagger 注解与 `swag init` 文档生成（仓库 admin 文档走既有 swagger 体系，本模块不单搞）。
- 运势内容的多租户分发运营（运势是全局编辑内容，见 §3 决定 6）。

**反目标（做出来算失败）**
- C 端接口依赖 casbin 或 admin 中间件（app 端一律 handler 内 `claims()` 自鉴权）。
- 把 goLucky 的 `{"code":200,"message","data"}` 信封原样搬进来——admin 段必须走
  `pkg/resp` 统一信封；app 段信封口径见 §4.1（scorecard app 同构，**不是** goLucky 老信封）。
- 新表缺 `tenant_id`（重蹈 `orders` gap）。
- 权限前缀未登记 `permissionRoutes` 就种菜单/权限码（死权限 403）。

## 3. 关键约束与既定决定

| # | 决定 | 依据 |
|---|---|---|
| 1 | 模块名 `starfuyu`，目录 `modules/starfuyu`，开关 `cfg.Modules["starfuyu"]` | 目录已存在 `modules/starfuyu`；snake_case 惯例 |
| 2 | C 端挂 `/api/app/starfuyu/*`，admin 段挂 `/api/admin/starfuyu/*` | 仓库 app/admin 双前缀惯例（scorecard/member 同款） |
| 3 | C 端身份 = member（`token.Validator` + `UserType==Member`，`claims()` 写回租户），**不建用户表、不做登录** | 用户决议：并进 member 体系；scorecard `controller/app/app.go` 同构 |
| 4 | 登录入口复用 `/api/app/social/wechat/mp/login`（已有能力，零新码） | modules.md §6.7；goLucky 的 code 登录语义等价物 |
| 5 | app 段响应走统一信封 `pkg/resp`（`code/data/msg`），错误码 3xxxx→HTTP 400 | scorecard app 18 接口先例；goLucky 老信封不保留（luckyU 后续按新契约接） |
| 6 | 运势内容与图片 = **全局编辑内容**，读写钉 `tenant_id=0`（cron/admin/app 读运势均 pin）；用户数据（profile/bazi/divination/limit）跟 member 租户 | 深聊结论：内容只抓一份，不按租户复制；fishgame `pinTidZero` 先例（但只对内容表生效，用户表照常走插件） |
| 7 | 用户星座偏好落 `starfuyu_profile`（`member_user_id` + `constellation`），不改 `member_user` | goLucky `users.constellation` 的归宿；跨模块不加列 |
| 8 | 错误码段 **305xx**（301 scorecard / 302 online / 303 social / 304 fishgame 已占） | `pkg/errors/code.go` 分段惯例 |
| 9 | 表全带 `tenant_id` + `created_at`/`updated_at`；内容表恒写 0，用户表走插件 | 不重蹈 orders gap |
| 10 | 定时任务用 `robfig/cron/v3`（新依赖），fx OnStart 启动 / OnStop 停止，00:10 抓运势、00:20 生图（秒级 spec 保留） | goLucky 原语义；仓库无现成 wall-clock 调度器，scorecard ticker 是分钟级扫描不适合定点任务 |
| 11 | 图片绘制依赖 `github.com/fogleman/gg`（新依赖）；背景图与字体从 `modules/starfuyu/assets/` 读（路径可配） | goLucky `service/image.go` 原样迁移 |
| 12 | 运势图片落 `pkg/storage` 本地盘（`cfg.Storage.Root` 下子目录），公开读走 `/api/public/starfuyu/image/:key` | fishgame share 同款；免自管静态目录 |
| 13 | DeepSeek / 公众号 appid·secret / 限流次数进 `config.yaml` 的 `starfuyu:` 段 | fishgame `cfg.Fishgame` 先例；与 oauth2 social 渠道表解耦 |
| 14 | 脚手架 `go run ./tools/newmodule -name=starfuyu -entities=profile,bazi,divination,fortune,limit -skip-frontend` 起手，骨架按业务重写 | usage.md §5；`-skip-frontend`（本期无 admin 页） |
| 15 | **签文按签号持久缓存**：公开 `GET /divination/sign/:number` 首次生成后按签号缓存（`starfuyu_sign_cache` 小表，见 §5），后续直读；**draw 落库的签文即缓存命中值**（同号稳定）；解读仍实时 DeepSeek + 限流 | builder-check Q3 裁决：免鉴权端点不做开放 LLM 出口 |
| 16 | 每日限流表 `starfuyu_request_limit`（user+date+type 唯一），配置次数进 `starfuyu.limits.*` | goLucky `models/request_limit.go` 语义 |
| 17 | **抓取语义照源码**：一次 DeepSeek 调用拉全部 12 星座、缺一整批报错；cron **只补缺不覆盖**（已有当日 12 条则跳过）；admin `POST /tasks/fetch-fortune` 带可选 `force:true` 覆盖重抓 | builder-check Q2 裁决：成本 ×1 且保留「抓错重抓」运维路径 |
| 18 | **时区模块内钉 `Asia/Shanghai`**：cron spec 加 `CRON_TZ=Asia/Shanghai`，模块内所有「今日」日期串（限流 date、fortune date、文章日期）一律 `time.Now().In(loc)`；**不改共享 `alexgo-server/Dockerfile`** | builder-check Q1 定调：共享镜像 TZ 归属全系统运维，超出模块范围；容器默认 UTC 下不钉则整条每日链路错位 |
| 19 | **中文字体仓库内置**：`modules/starfuyu/assets/fonts/` 放 OFL 协议 CJK 字体（Noto Sans SC 或子集），`assets_dir` 配置一并指向；背景图同目录 | builder-check Q4 裁决：部署零改动，避免动共享镜像 |
| 20 | admin 任务端点**同步返回**（照源码）；文档标注 DeepSeek 最坏 120s×重试 可能撞网关读超时，超时用 force/重试兜底 | builder-check Q5 定调：不引入任务状态表/异步受理（YAGNI） |

## 4. 端点设计

### 4.1 C 端 `/api/app/starfuyu`（Bearer member，统一信封）

鉴权与 scorecard app 同构：handler 内 `claims()` 解析 `Authorization: Bearer` →
`token.Validator.Validate` → `UserType != Member` → 403 `20003`；缺/坏 token → 401 `20001`；
成功后把 token 内 tenant 写回 ctx（用户数据走租户插件）。**不走 casbin。**
除注明公开外，均需登录。

| Method | Path | 说明 |
|---|---|---|
| GET | `/fortune/today` | 今日全部 12 星座运势（内容钉 tid=0）；无数据 → 空列表而非报错（凌晨任务未跑时客户端可兜底） |
| GET | `/fortune/:date` | 指定日期运势（`YYYY-MM-DD` 校验 10001） |
| GET | `/fortune/my/today` | 我的今日运势（需 `starfuyu_profile.constellation`；未设星座 → 30505） |
| GET | `/fortune/my/:date` | 我的指定日期运势（同上） |
| GET | `/fortune/image/:constellation/:date` | 该星座当日运势图（公开读 key 的 JSON 包装：`{url}`；图不存在且开关允许时现场生成） |
| POST | `/bazi/calculate` | 计算八字：`{birth_date,birth_time,birth_place}` → DeepSeek 算八字并**覆盖保存**我的八字；限流 `bazi_calculate`（超 → 30503） |
| GET | `/bazi/my` | 我的八字（无 → 30504） |
| PUT | `/bazi/my` | 更新我的八字（同 calculate，同样限流） |
| POST | `/divination/draw` | 求签：`{question?,category?}` → 随机 1–100 签（签文经 `starfuyu_sign_cache` 取/写，同号稳定）+ DeepSeek 解读（结合我的八字，无八字则降级纯签文）；限流 `divination_draw`（超 → 30503）；落 `starfuyu_divination` |
| GET | `/divination/history` | 我的求签历史（分页 `page/size`，size≤100） |
| GET | `/divination/sign/:number` | 指定签号签文（1–100，越界 10001；**公开放行 + 按签号缓存**，见决定 15——首访生成后落 `starfuyu_sign_cache`，此后直读，不实时打 DeepSeek） |
| GET | `/profile` | 我的星座偏好（未设 → `constellation:""` 不报错） |
| PUT | `/profile` | 设置/更新星座（`{constellation}`，校验 12 星座枚举 10001） |

> goLucky 老路径 `/api/auth/login`、`/api/user/*`、`/api/test/token` **不迁移**。

### 4.2 管理端 `/api/admin/starfuyu`（管理员 token + casbin，统一信封）

全部走 `pkg/resp`；casbin 前缀两条（见 §7）；handler 不取 claims（scorecard admin 同构）。

| Method | Path | 权限码前缀 | 说明 |
|---|---|---|---|
| GET | `/fortunes` | `starfuyu:fortune` | 运势分页/筛 `date`、`constellation`（缺省不过滤） |
| GET | `/fortunes/:date` | `starfuyu:fortune` | 指定日期全星座运势 |
| GET | `/fortunes/:date/article` | `starfuyu:fortune` | 生成今日/指定日公众号文章 HTML（预览，不上传） |
| POST | `/fortunes/:date/upload-mp` | `starfuyu:fortune` | 生成文章 + 图片素材 → 传公众号草稿箱；返回每篇 `{title,media_id}` |
| POST | `/tasks/fetch-fortune` | `starfuyu:task` | 手动触发：抓取指定日（body `date` 缺省今日）；**默认只补缺，`force:true` 覆盖重抓**（决定 17）；同步返回，可能长挂（决定 20） |
| POST | `/tasks/generate-images` | `starfuyu:task` | 手动触发：生成指定日（body `date` 缺省今日）全部星座图；同步返回 |
| POST | `/tasks/execute-all` | `starfuyu:task` | 抓取（同样支持 `force`）+ 生图一条龙；同步返回 |
| GET | `/limits` | `starfuyu:task` | 今日各类型限流消耗概览（运维排查用，聚合 count） |

> goLucky 的 GET 型 task 触发（`GET /api/task/*`）不保留——写操作一律 POST。

### 4.3 公开端 `/api/public/starfuyu`（免鉴权）

| Method | Path | 说明 |
|---|---|---|
| GET | `/image/:key` | 运势图公开读（storage key → inline 图片；不存在统一 30010） |

## 5. 数据模型（6 表）

迁移 `modules/starfuyu/migrations/20261112000001_starfuyu_tables.{up,down}.sql`
（`CREATE TABLE IF NOT EXISTS`、无外键）。**6 表全带 `tenant_id`** + `created_at`/`updated_at`：

| 表 | 要点 |
|---|---|
| `starfuyu_profile` | 用户星座偏好：`member_user_id`（唯一索引含 tenant_id）、`constellation`（12 星座枚举值） |
| `starfuyu_fortune` | 运势内容：`(date, constellation)` 唯一、分值字段照源码全集（overall/love/career/wealth/health + `lucky_number` + `daily_overview`）、`lucky_color` JSON、`summary`/四 tip 文本；**tenant_id 恒 0** |
| `starfuyu_bazi` | 我的八字：`member_user_id` 唯一（含 tenant）、`birth_date/birth_time/birth_place`、`bazi_info` JSON（四柱/五行/节气/农历/生肖/星座） |
| `starfuyu_divination` | 求签记录：`member_user_id`、`bazi_id` 可空、`question`、`sign_number`(1–100)、`sign_content`、`interpretation`、`category`、`lucky_direction/lucky_color/advice`；索引 `(member_user_id, created_at)` |
| `starfuyu_request_limit` | 限流：`(member_user_id, date, type)` 唯一（含 tenant）、`count`；type ∈ `bazi_calculate` / `divination_draw` |
| `starfuyu_sign_cache` | 签文缓存（决定 15）：`sign_number`(1–100) 唯一、`sign_content`；**tenant_id 恒 0**（签文是全局内容）；draw 与公开 GET 共用——draw 未命中先经此缓存取/写 |

图片**不落业务表**：storage 盘 + key（命名 `fortune/{date}/{constellation}.jpg`，确定性 key
天然幂等覆盖）。若后续要记 meta 再补表，一期 YAGNI。

`starfuyu_fortune.lucky_color` 等 JSON 列照 goLucky 的 `driver.Valuer/Scanner` 模式迁（同
`models/fortune.go`）。`bazi_info` 同理。

## 6. 定时任务与后台生命周期

`module.go` 内 fx OnStart 起 cron、OnStop 停（照 system `SweepExpired` / scorecard ticker
的生命周期惯例，但调度器换 robfig/cron）：

| 时刻（本地） | 任务 | 行为 |
|---|---|---|
| 每天 00:10 | 抓今日运势 | DeepSeek 逐星座拉取 → upsert `starfuyu_fortune`（tid=0）；单星座失败记 log 继续，不整批回滚 |
| 每天 00:20 | 生成全部星座图 | 读当日 12 条运势 → gg 绘图 → storage 落盘（确定性 key 覆盖） |

- cron 表达式与时刻从 `cfg.Starfuyu.Cron.*` 读（缺省 `"10 0 * * *"` / `"20 0 * * *"`），
  运维可改配置错峰；调度一律 **`CRON_TZ=Asia/Shanghai`** 前缀拼接（决定 18）。
- **时区决定 18 展开**：模块内公共 helper `nowSH()` = `time.Now().In(loc)`，限流 `date`、
  fortune `date`、文章日期、cron 触发后的「今日」全部走它——不依赖 `time.Local`
  （生产容器默认 UTC，不钉则 00:10 实际是北京时间 08:10，且凌晨时段日期串错位）。
- 启动不补跑：错过就等明天（与 goLucky 一致）；补跑靠 admin `POST /tasks/*` 手动触发。
- 抓取照决定 17：**一次 DeepSeek 拉 12 星座**；cron 走「已有当日 12 条 → 跳过」的补缺语义。
- DeepSeek key 为空时任务记 warn 跳过（不 panic，联调前可空配启动——fishgame wechat 同款）。

## 7. 权限、菜单与文档触点

- **casbin 前缀**（`modules/system/service/permission_routes.go` 登记，否则死权限）：
  - `starfuyu:fortune` → `["/api/admin/starfuyu/fortunes", ".../fortunes/*"]`
  - `starfuyu:task` → `["/api/admin/starfuyu/tasks", ".../tasks/*", "/api/admin/starfuyu/limits"]`
- **菜单种子**：本期 **无 admin 前端页面 → 不种菜单**（避免侧栏死链）。菜单 + vben 页面
  随后续票补，届时迁移 SQL 仿 `20261023000001_fishgame_menus.up.sql`（NOT EXISTS 幂等）。
  **casbin 前缀本期就登记进 `permission_routes.go`**（为后续票铺路，且有
  `permission_routes_test.go` 兜底）。
- **`docs/modules.md`**：新增 §13 starfuyu 章节（功能/端点/数据模型/定时任务），
  §1.1 模块地图表、§9.1 前缀表同步回填（脚手架不自动改，收尾人工补）。
- **错误码**（`pkg/errors/code.go` 305xx，registry 中文文案）：

| 码 | 文案 | 语义 |
|---|---|---|
| 30501 | 运势数据不存在 | 指定日期/星座无运势行（admin 详情与 my 类接口） |
| 30502 | 图片生成失败 | gg 绘制或资源缺失（背景图/字体） |
| 30503 | 今日请求次数已达上限 | 限流命中（bazi/divination 共用，响应带 type） |
| 30504 | 尚未保存生辰八字 | 求签依赖八字的降级路径之外，`GET /bazi/my` 无数据 |
| 30505 | 尚未设置星座 | `fortune/my/*` 需要 profile |
| 30506 | 运势抓取失败 | DeepSeek 返回不合法/超时（admin task 与 cron 记录） |
| 30507 | 公众号上传失败 | access_token / 草稿箱 API 错误（errcode 只记日志，上抛统文案） |

## 8. 配置（`config.yaml` 新增 `starfuyu:` 段 + `pkg/config` 结构体）

```yaml
modules:
  starfuyu: true          # 模块开关（默认 true 或缺省=开，照 scorecard/fishgame 惯例）

starfuyu:
  deepseek:
    api_key: ""
    base_url: "https://api.deepseek.com/v1/chat/completions"
    model: "deepseek-chat"
    timeout: 120
    max_retries: 3
    retry_delay: 2
  wechat_mp:              # 公众号（草稿箱上传）
    app_id: ""
    app_secret: ""
  limits:
    bazi_calculate_daily: 5
    divination_draw_daily: 10
  cron:
    fetch_fortune: "10 0 * * *"        # 模块内恒加 CRON_TZ=Asia/Shanghai 前缀
    generate_images: "20 0 * * *"
  assets_dir: "modules/starfuyu/assets"   # bgimage/bg.png + fonts/*.ttf（决定 19）
```

`pkg/config/config.go` 加 `Starfuyu struct{...} mapstructure:"starfuyu"`（Fishgame 段旁）。
**注意**：goLucky 现 `config.yaml` 里明文写着公众号 appid/secret——迁移时**不把密钥抄进
仓库**，留空由运维填（可顺手提醒用户轮换已泄露密钥）。
**字体**（决定 19）：`assets/fonts/` 内置一份 OFL 协议 CJK 字体（Noto Sans SC 或按
文案字符集子集化），`ImageService` 加载顺序 = 配置字体路径 → `assets/fonts/` → 系统探测
→ 报 30502（不再静默退化 ASCII 空白图）。

## 9. 目录结构与装配

```
modules/starfuyu/
├── module.go              # FxModule + RegisterRoutes（/app + /admin + /public 三段）
├── migration_source.go    # migrate.Source → migrations/
├── configs/config.yaml    # （可选样例，真源在 alexgo-server/configs/config.yaml）
├── migrations/            # 20261112000001_starfuyu_tables.{up,down}.sql
├── model/                 # Profile/Fortune/Bazi/Divination/RequestLimit（+ JSON valuer）
├── repository/            # 五仓储；内容表读写显式 tid=0，用户表走插件
├── service/               # Fortune(抓取/查询，一次拉12星座)、Image(gg+内置字体)、
│                          # Article(公众号)、Bazi、Divination(含 sign_cache)、
│                          # Limit、Task(cron 编排, CRON_TZ)
├── controller/
│   ├── app/               # claims() 自鉴权（scorecard 同构）
│   └── admin/             # casbin 已放行，只做参数/领域校验
├── assets/                # bgimage/bg.png 等静态资源
└── swagger_doc.go         # （脚手架产物，按需保留或删）
```

- 装配进 `alexgo-server/cmd/main.go` 的 fx 模块列表（fishgame/scorecard 旁）。
- 老 goLucky 目录：**迁移完成后整目录移除**（含自带 `.git`、build 产物、`images/` 样张）；
  `luckyU/` 原样保留。移除动作在实现 PR 的收尾提交，不与代码迁移混提。
- 新增依赖：`github.com/robfig/cron/v3`、`github.com/fogleman/gg`（+ 传递 `x/image` 若缺）。

## 10. 与 goLucky 的语义差异（迁移对照）

| goLucky | starfuyu 模块 | 原因 |
|---|---|---|
| 自有 `users` 表 + JWT + `/api/auth/login` | 不迁移；member + social 登录 | 用户决议 |
| `{"code":200,"message","data"}` | `pkg/resp` 统一信封 | 仓库惯例 |
| 路由 `/api/fortune/*`、`/api/bazi/*`… | `/api/app/starfuyu/*` + `/api/admin/starfuyu/*` | 双前缀惯例 |
| GET 型 task 触发 | POST only | 写操作语义 |
| 图片写 `images/` + `/api/fortune/image/file/*` | storage 盘 + `/api/public/starfuyu/image/:key` | 复用 infra 文件能力 |
| CORS 中间件、`cmd/gen-token`、Swagger 注解 | 不迁移 | 全局中间件已有；测试 token 无意义 |
| `GET /api/fortune/today/article`（C 端可读） | admin `GET /fortunes/:date/article` | 文章生成是运营动作，不进 C 端 |
| 限流配置 `limit.*` | `starfuyu.limits.*` | 归模块配置段 |

## 11. 验收

1. `go build ./...` + `go test ./modules/starfuyu/...` 全绿；`tools/newmodule` 单测不回归。
2. 迁移在空库幂等执行；`cfg.Modules["starfuyu"]=false` 时三段路由均不注册。
3. C 端：social 登录换 token → 设星座 → 查运势 → 算八字（限流生效）→ 求签 → 历史分页；
   非 member token 403；无 token 401。
4. admin：casbin 未授权 403；`POST /tasks/fetch-fortune` 真抓一条（或 DeepSeek 空配时
   30506 可观测）；已有数据时默认跳过、`force:true` 覆盖；`upload-mp` 在密钥空配时 30507。
5. cron 表达式生效且带 `CRON_TZ=Asia/Shanghai`（临时表达式验证起停），OnStop 干净退出；
   模块内日期串在 `TZ=UTC` 进程下仍按北京日期翻转。
6. 公开 `GET /divination/sign/:number` 首访后二次请求不再打 DeepSeek（缓存命中），
   同号签文稳定；仓库 `assets/fonts/` 有中文字体且生图中文不空白。
7. `docs/modules.md` §13 与模块地图/前缀表回填；本文档状态改「已定稿」。

## 12. 后续票（明确切出去的）

1. luckyU 小程序切换 member/social 登录 + 新 API 契约对接。
2. admin vben 页面（运势列表/任务中心/限流概览）+ 菜单种子 + `permission_routes` 联调。
3. （可选）签文模板库表化、运势内容租户化运营、公众号定时自动上传（现为手动触发）。
