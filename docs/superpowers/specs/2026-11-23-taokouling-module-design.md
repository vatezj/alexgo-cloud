# taokouling 淘口令转换模块设计（taoquan 移植）

> 日期：2026-11-23 ｜ 状态：已实现（2026-11-24，SDD 16 任务全过门）；原待批准（builder-check 一轮已回，3 项全部处置：2 项用户拍板「都按推荐」，
> 1 项设计定调——唯一键复合化；见 §3 决定 25/26 与 §5）
> 需求与来源（**移植口径以这两处为准**）：
> - `/Users/alex/Desktop/mywork/taoquan/docs/淘口令转换-移植文档.md` —— 完整业务逻辑、外部接口契约、已知坑点（PHP + CodeIgniter 3 源项目）
> - 源码索引见移植文档 §14（`Product.php` / `Weixin.php` / `TopSdk.php` 等）
>
> 本文是 alexGo-cloud 侧的实现设计；与原实现冲突时，**业务语义以移植文档为准**，
> 鉴权/信封/租户/模块触点等横切惯例以本仓库为准。移植文档 §11 的 12 条改进清单
> **全部采纳**（不照搬已知问题）。

---

## 1. 意图与背景

用户粘贴任意淘宝口令（或含口令的分享文案），系统：

1. **解析**口令拿到 `item_id`（第三方：折搞定 zhetaoke）
2. **换成自己推广位（PID）的带券链接**（第三方：好单库 haodanku，按渠道切换 PID）
3. **生成自己的新淘口令**（淘宝官方 TOP `taobao.tbk.tpwd.create`）
4. **拼营销文案**（原价/券后价/最高返利/必买理由）

两种消费形态（移植文档 §1）：

- **轻量**：只解析出 `item_id`，调用方拿 ID 自行跳详情页（本仓库无淘宝商品体系，跳转落点归调用方）。
- **完整**：一步出整段可复制文案（完整链路是轻量的超集）。

现把它转化为 alexGo-cloud 标准业务模块 `modules/taokouling`，与 starfuyu（第三方 API 密集型）
同款路径融入；本期双端交付：**C 端两条公开转换接口 + admin 两页**（渠道配置 CRUD、转换记录查询）。

## 2. 目标 / 非目标 / 反目标

**目标**

- 移植文档 Step 1~5 全链路等价重现（正则口令提取 → zhetaoke 解析 → haodanku 换链
  （渠道 PID）→ TOP 生成新口令 → 文案拼装），轻量链路 = Step 1+2。
- 采纳移植文档 §11 改进清单：统一错误码、正则提取口令、`signurl` 固定 0 只留一套解析、
  密钥外置 config、C 端限流、TTL 缓存、每跳超时+重试+降级、禁止任意 PID 透传、返利率可配。
- 标准模块触点齐全：`tools/newmodule` 脚手架、fx 装配、`modules.taokouling` 开关、
  迁移源、`cfg.Taokouling` 配置段、错误码 **307xx**、`permissionRoutes` 登记、
  system 迁移菜单种子、`docs/modules.md` 回填。
- TDD：口令提取/文案渲染/渠道解析/TOP 签名为纯函数级单测；client 层 httptest；service 层桩编排测试。

**非目标（不做）**

- 微信公众号自动回复链路（移植文档 §9.3）——仓库无公众号消息服务器，独立面，后续票。
- C 端页面（scorecard appui「淘口令」页）——本期只交付 API 契约，前端接入后续票。
- 淘口令转二维码图（zhetaoke `:10002`，移植文档 §7.3）、反向链路
  `url_get`/`pic_post`（已知推广链接 → 口令，§7.4）、`tbk.tpwd.parse` 官方解析兜底（§7.5）。
- Redis 缓存/限流（进程内存实现，单进程 mono 形态够用；多实例一致性后续需要再换）。
- 异步转换、Swagger 注解、21ds.cn 历史方案、`$_GET` 式任意 `pid/apikey` 透传（安全，明确禁止）。
- admin 端手动「链接转口令」工具（反向链路的管理端形态）。

**反目标（做出来算失败）**

- 照搬移植文档 §11 的 12 条已知问题（解析失败仍返成功、`$`/`€` 误判、密钥硬编码、无限流……）。
- C 端接口挂 casbin 或要求登录态（公开 + 限流，已定）。
- 新表缺 `tenant_id`（重蹈 `orders` gap）。
- 权限前缀未登记 `permissionRoutes` 就种菜单/权限码（死权限 403）。
- 任意请求参数可覆盖 `pid/apikey/tb_name`（原项目 `$_GET` 注入问题，明确禁止）。

## 3. 关键约束与既定决定

| # | 决定 | 依据 |
|---|---|---|
| 1 | 模块名 `taokouling`，目录 `modules/taokouling`，开关 `cfg.Modules["taokouling"]` | snake_case 惯例；starfuyu 同为拼音名 |
| 2 | 构建路径 **B + B2**：仓库惯例重写 + 采纳改进清单；第三方调用收进 `client/` 子包（接口注入，可 mock） | 用户选定；social `PlatformAdapter` 同款结构，TDD 最省力 |
| 3 | 本期交付 = 后端 + admin 两页（渠道配置、转换记录）；C 端页面后续票 | 用户选定（交付形态选项 2） |
| 4 | C 端两接口**公开 + 按 IP 限流**（默认 10 次/分），不挂 Bearer | 用户选定（选项 1）；保持工具类语义，原契约无鉴权头 |
| 5 | 轻量链路按纯 API 契约交付（返回 `item_id` 由调用方跳转）；**降级搜索目标 URL 可配置**（`fallback_search_url`，空则降级返回纯文案不拼死域名） | 本仓库无淘宝详情页/站内搜索落点（已向用户声明） |
| 6 | 微信公众号自动回复（§9.3）不做 | 已向用户声明，无公众号消息服务器 |
| 7 | 错误码段 **307xx**（301~306 已占）；常量 + registry 双处登记 | `pkg/errors/code.go` 分段惯例，code_test 钉注册 |
| 8 | `signurl` 固定 `0`，只保留 `tbk_privilege_get_response.result.data.item_id` 一套解析 | 移植文档 §4.2/§11-5：两套结构不兼容，选轻量链路那套 |
| 9 | 口令提取：**按符号优先级依次做「成对货币符 + 8~24 位字母数字」正则匹配**（`￥` 优先，其余按原优先级 `₳ ₴ € ¢ ￥ $ ₤`）；另支持 `?tk=` URL 型口令（取参后按空格截断） | §3.2/§3.4 + 改进 3；成对匹配消解 split 截错与单侧误判，`￥` 优先降低 `$`/`€` 正文误命中 |
| 10 | 渠道 PID 白名单**落表**（`taokouling_channel`，partner/host 两级匹配），admin CRUD；**禁止**请求参数直接透传 `pid/apikey/tb_name` | §6 + 改进 8；未知 partner 回退 host → 默认渠道，不报错 |
| 11 | 迁移**只种子默认渠道一行**（partner/host 空串）；移植文档 §6 的 8 个 partner、3 个 host 渠道由 admin 页面录入 | 避免把可能过期的业务 PID 静默灌库；§6 全表见本文 §10 录入参考 |
| 12 | 每次转换**落库记录**（输入/口令体/item_id/渠道/状态/耗时/IP），admin 只读查询 | 用户选定交付形态含「转换记录查询」；排查与对账 |
| 13 | 解析失败（轻量链路）→ **硬错误 30705**；完整链路业务失败 → **降级 200**（`degraded:true` + 可用文案） | 改进 1（轻量接口失败必须显式，不再 `data:null` 混过）；完整链路保持原语义「用户总拿到点得动的东西」 |
| 14 | 第三方业务失败（口令过期/无券/商品下架）**一律降级**不报 5xx；配置缺失（appkey/secret 空）**快速硬错 30702**（运维问题不被降级掩盖） | 改进 1/11；验收用例 3 |
| 15 | 每跳超时默认 5s（短链 3s）、网络错/5xx 重试 1 次、业务失败不重试 | 改进 11 |
| 16 | 缓存两条：`item_id+pid → {click_url, coupon}`、`click_url → 新口令`，进程内存 TTL（默认 1800s） | 改进 12；券链/口令有效期数小时，命中即免两跳 |
| 17 | 非密钥配置（base_url/超时/sid/parse_pid/user_id/返利/限流/缓存/降级 URL）进 `config.yaml` 的 `taokouling:` 段；**密钥值（`zhetaoke.appkey`、`top.secret`、`shorturl.apikey`）不进 git**——yaml 只提交空串占位，值由本地工作区（不暂存）或环境变量 `TAOKOULING_ZHETAKE_APPKEY` / `TAOKOULING_TOP_SECRET` / `TAOKOULING_SHORTURL_APIKEY`（经 `applyEnvOverrides` 显式覆盖，仓库既有密钥注入模式）提供，缺值运行时 30702 fail-fast；**上线后轮换**移植文档中的值 | 改进 6 + `.git/hooks/pre-commit` 密钥护栏（2026-11-16 裁定「密钥不进 git」，builder 阶段发现，覆盖原「yaml 明文入库」写法） |
| 18 | 返利率可配 `rebate_rate`（默认 0.05）、返利教程链接可配 `rebate_tutorial_url`（空则省略该行） | 改进 9；原模板硬编码 `suo.im/4CYwC4` |
| 19 | TOP SDK：**无可用官方 Go SDK，自实现 TOP 协议签名**（参数按 key 升序拼接 + md5，细节对照官方文档，单测钉样例） | 移植文档 §7.1 只约束算法；PHP SDK 语义等价即可 |
| 20 | C 端响应走 `pkg/resp` 统一信封（成功 `code:0`），**不保留**原项目 `{"code":200,...}` 老信封 | starfuyu 先例（决定 5）；前端 `request.ts` 按 `code===0` 解包 |
| 21 | 脚手架 `make new-module NAME=taokouling ENTITIES=channel,log ARGS="-skip-frontend"` 起手，骨架按业务重写；前端两页手写 | `channel` 实体近似 CRUD 可留；`log` 收敛只读列表；codegen 式脚手架页面形状不对，直接手写（scorecard 删除脚手架页先例） |
| 22 | admin 权限两前缀 `taokouling:channel` / `taokouling:log`，**脚手架触点 4 生成后按本设计校正** | permissionRoutes 惯例；前缀必须登记否则死权限 |
| 23 | 渠道解析顺序：请求参数 `partner`（剥离 `#` 前缀）→ `Host` 头匹配 `channel.host` → 默认行（partner/host 均空串）→ 无默认行 30703 | §6 优先级语义；host 从请求头取而非信任参数 |
| 24 | 转换记录 `input_text` 截断 500 字符、不存整段输出文案 | 隐患面与存储成本；item_id/状态/错误已够排查 |
| 25 | **全模块数据面恒钉 `tenant_id=0`**：渠道查询、日志写入、渠道/记录的 admin 查询一律 service 层显式 pin（starfuyu 内容表先例），**不读请求租户**；种子 tid=0。表保留 `tenant_id` 列（仓库存例 + 未来租户化留位），但 v1 语义 = 平台级——渠道是平台运营方的推广位，不是租户资产；伪造 `X-Tenant-ID` 切不到数据、也写不进记录 | builder-check P0 裁决（用户拍板推荐项）；原设计按租户插件过滤会在 Host/头解析出 tid≠0 时查不到种子默认行 → 必然 30703 |
| 26 | **完成定义**：本期 = 代码 + 单测/桩测试全绿 + 冒烟清单成文；**真钱冒烟为挂起项**——凭据按移植文档值先配 config、由用户确认可用性后补验 §15-1/2/6/7；若 TOP appkey 已无 `tbk.tpwd.create` 授权，Step 4 按既有降级口径交付并记后续票 | builder-check P1 裁决（用户拍板推荐项）；凭据可用性是外部事实，不能当代码完成门禁 |
| 27 | 渠道表唯一键 = **复合 `UNIQUE(tenant_id, partner, host)`**；admin 查重按 `(partner, host)` 组合；同 host 多行取启用首行（id 升序） | builder-check P0 设计定调：单列 `UNIQUE(tenant_id, partner)` 会让默认行 `('','')` 与两条 host 渠道（partner 均空串）互撞，§10 录入不可行 |

## 4. 端点设计

### 4.1 C 端 `/api/app/taokouling`（公开 + 限流，统一信封）

限流：进程内按 `ClientIP` 固定窗口（两接口合计计数），超限 **HTTP 429** + 信封
`{code:30704, msg:"请求过于频繁，请稍后再试"}`（handler 直写 429，不走 `statusFor` 的 3xxxx→400）。
限流在业务逻辑之前；被限请求**不落转换记录**。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/parse` | **轻量链路**。body `{password: string, partner?: string}`。成功 `data:{item_id:"<字符串>"}`；无口令 → 30701；zhetaoke 解析失败 → **30705**（硬错，不降级）；配置缺失 → 30702 |
| POST | `/convert` | **完整链路**。body 同上。成功 `data:{text, item_id, degraded, title?, coupon_money?, new_tkl?}` —— `text` 为整段可复制文案（成功）或降级文案；`degraded:true` 表示未走完全链路（口令过期/无券/换链或生成失败，仍 HTTP 200） |

契约映射（对移植文档 §9，路径按仓库惯例改名，信封换 `pkg/resp`）：

| 原（PHP） | 新 | 差异 |
|---|---|---|
| `POST /api/product/conversion` → `{code:200, data:item_id}` | `POST /api/app/taokouling/parse` → `code:0, data.item_id` | 失败显式错误码（原无失败分支） |
| `POST /api/product/change_password_post` → text/plain | `POST /api/app/taokouling/convert` → `code:0, data.text` | 结构化附加字段；失败文案也走 `text` + `degraded` |

校验：`password` 空 → `10001`（仓库标准校验码）。`partner` 未知不报错（回退默认渠道，见 §7）。

### 4.2 admin `/api/admin/taokouling`（casbin + 统一信封，handler 不取 claims）

| 方法 | 路径 | 权限码 | 说明 |
|---|---|---|---|
| GET | `/channels` | `taokouling:channel` | 渠道列表（`resp.Page` 分页，size≤100） |
| POST | `/channels` | `taokouling:channel` | 新建渠道（partner/host/pid/apikey/tb_name/name/status；**`(partner, host)` 组合查重**（决定 27），pid/apikey/tb_name 必填 10001；数据恒写 tid=0（决定 25）） |
| PUT | `/channels/:id` | `taokouling:channel` | 更新（路径 id 优先；同上校验） |
| DELETE | `/channels/:id` | `taokouling:channel` | 删除（默认行禁删——删了全站换链断供 → 30703 类语义，用 10001 文案「默认渠道不可删除」） |
| GET | `/logs` | `taokouling:log` | 转换记录分页（筛选：`start/end` 时间、`partner`、`status`、`item_id` 精确；id 倒序） |

> 脚手架若生成 `GET /logs/:id` 等多余 CRUD 路由，收敛掉——记录只读。

## 5. 数据模型（2 表）

迁移 `modules/taokouling/migrations/20261123000001_taokouling_tables.{up,down}.sql`
（`CREATE TABLE IF NOT EXISTS`、无外键），**2 表全带 `tenant_id`** + `created_at`/`updated_at`，
走 GORM 租户插件隔离：

| 表 | 要点 |
|---|---|
| `taokouling_channel` | 渠道 PID 白名单：`name`、`partner`（空串=非 partner 渠道）、`host`（空串=无 host 匹配，普通索引 `(tenant_id, host)`——多 partner 行 host 均为 ''，不设唯一）、`pid`、`apikey`、`tb_name`（haodanku 三元组）、`status` 1 启用 0 停用。**唯一键 = 复合 `UNIQUE(tenant_id, partner, host)`**（决定 27：默认行 `('','')`、host 行 `('','song.vate.ren')`、partner 行 `('tang','')` 互不冲突；同 (partner,host) 查重拦截）。**种子**：默认行一行（name='默认渠道'、partner=''、host=''、status=1、**tenant_id=0**，值取移植文档 §6 默认三元组），`NOT EXISTS` 幂等 |
| `taokouling_conversion_log` | 转换记录（追加写，**恒写 tenant_id=0**（决定 25））：`endpoint`（parse/convert）、`partner`、`input_text`（varchar 500，截断）、`extracted_tkl`（varchar 64）、`item_id`（varchar 32，可空——解析前失败无 ID）、`status` 1 成功 / 2 降级 / 3 无口令 / 4 硬错误、`error`（varchar 255 可空）、`duration_ms`、`ip`（varchar 45）。索引：`(tenant_id, created_at)`、`(item_id)`、`(partner)` |

**租户口径（决定 25）**：两表虽带 `tenant_id` 列，但 v1 全部读写在 service 层显式钉 0，
不走租户插件的请求上下文——C 端公开接口无 claims 回写，插件按 Host/头解析出的 tid≠0
会查不到种子行（builder-check P0）；admin 查询同样 pin 0（模块数据即平台数据，
非平台租户管理员经 casbin 放行后看到的是同一份渠道/记录）。

渠道查无（含默认行被删）→ 30703 硬错误（见 §7）；记录只在通过限流后写，
写失败仅记日志不影响响应。

## 6. 分层结构与编排（B2）

```
modules/taokouling/
├── module.go                 # fx 装配 + RegisterRoutes（modules.taokouling 开关前置）
├── migration_source.go
├── client/                   # B2：第三方薄客户端，接口定义 + 装配期注入
│   ├── client.go             # 公共 HTTP 助手（超时/重试策略/URL encode）
│   ├── zhetaoke.go           # ParseTkl(ctx, tkl) → item_id（GET open_gaoyongzhuanlian_tkl，signurl=0，
│   │                         #   取 tbk_privilege_get_response.result.data.item_id；见 error_response → 业务失败）
│   ├── haodanku.go           # RatesURL(ctx, pid, apikey, tbName, itemID) → {click_url, coupon_money}
│   │                         #   （POST form，code!=1 → 业务失败）；ItemDetail(ctx, itemID) → itemdesc
│   ├── top.go                # TpwdCreate(ctx, text, url, logo) → 新口令；ItemInfo(ctx, itemID) →
│   │                         #   {title, pict_url, zk_final_price, reserve_price}（TOP 签名自实现，决定 19）
│   └── shorturl.go           # Shorten(ctx, longURL) → 短链（失败返回空，调用方回退长链/纯文案）
├── service/
│   ├── password.go           # Step 1 纯函数：Extract(text) → (tkl, ok)（决定 9，含 tk= 变体）+ TitleOf(text)（【...】抓标题）
│   ├── render.go             # Step 5 纯函数：Render(input) → 文案（券/无券两分支，决定 18）
│   ├── resolve.go            # 渠道解析（决定 23）+ 默认行查询
│   ├── cache.go              # 进程内存 TTL（决定 16），两键位
│   ├── ratelimit.go          # 按 IP 固定窗口（决定 4）
│   ├── convert.go            # ConversionService：Parse（轻量）/ Convert（完整编排 + 降级 + 缓存 + 落日志）
│   └── *_test.go             # TDD 主战场
├── repository/
│   ├── channel.go            # CRUD + 按 partner/host/默认行查询（恒显式 tenant_id=0，决定 25）
│   └── conversion_log.go     # 追加 + 分页筛选（恒写/恒查 tenant_id=0，决定 25）
├── model/                    # Channel / ConversionLog（TableName + gorm 标签）
├── controller/
│   ├── app.go                # 两端点：限流 → 绑参 → service → resp.OK/Fail（429 直写）
│   └── admin.go              # 5 端点：casbin 已放行，参数/领域校验
└── migrations/
    ├── 20261123000001_taokouling_tables.{up,down}.sql   # §5 两表 + 默认渠道种子
    └── （菜单种子在 system 模块迁移，见 §9）
```

**完整链路编排（`Convert`，同步串行，每跳超时/重试见决定 15）：**

```
输入 text
  ├─ Step 1  Extract → 无口令 → 30701（落 log status=3）
  ├─ Step 2  zhetaoke.ParseTkl(tkl) → 失败 → 降级路径 D（log status=2, error 记因）
  │           命中缓存 skip？—— 不缓存解析（口令时效短，YAGNI）
  ├─ Step 3  resolveChannel(partner, host) → 无默认行 → 30703（log status=4）
  │           缓存 key= item_id+pid → 命中则跳过 RatesURL
  │           haodanku.RatesURL → code!=1 → 降级路径 D
  ├─ Step 3b haodanku.ItemDetail(itemdesc)   —— 失败不阻断，理由行降级省略
  │           top.ItemInfo(标题/图/价)        —— 失败回退 TitleOf(input) 抓标题、价格行省略
  ├─ Step 4  缓存 key= click_url → 命中跳过
  │           top.TpwdCreate → 失败 → 降级路径 D
  └─ Step 5  Render → data{text, item_id, degraded:false, title, coupon_money, new_tkl}（log status=1）

降级路径 D：
  TitleOf(input) 拼 fallback_search_url（配置非空时经 shorturl 缩短，短链失败回退长链）
  → text = 「点击搜索其它店铺{链接}」或纯提示文案（fallback 空）
  → data{text, item_id(可能有), degraded:true}（log status=2）
```

实际编排次序：`Resolve`（渠道）先于 `ParseTkl`（口令解析）——与上文流程图 Step 2/3 顺序相反，系决定 14「配置缺失快速硬错」的落地（渠道断供时先 30703，不被降级掩盖）。
目录树中渠道解析文件为 `service/resolve.go`（非 `channel.go`；脚手架 channel.go 已随 T13 删除）。

**轻量链路（`Parse`）**：Step 1 → Step 2，失败硬错（决定 13），不走降级、不查渠道、不落
`degraded`。成功落 log status=1。

**失败分层**：配置缺失（zhetaoke/top appkey 等空）在 service 入口 fail-fast → 30702（log status=4）；
HTTP 网络错/5xx 重试 1 次后按业务失败处理（convert 降级 / parse 30705）；panic 不外抛（gin Recovery 兜底）。

## 7. 渠道解析（细节）

```
resolveChannel(ctx, partner, hostHeader):
  partner = strings.TrimPrefix(partner, "#")            # 防链接锚点污染（§6）
  if partner != "":
      ch = repo.GetEnabledByPartner(ctx, partner)       # 命中 → 用
      # 未命中不报错，继续回退（原 info_data 语义）
  if hostHeader != "":
      ch = repo.GetEnabledByHost(ctx, hostHeader)       # Host 头（不信任请求参数）
  ch = repo.GetDefault(ctx)                             # partner='' AND host='' AND status=1
  if ch == nil: return 30703                            # 配置断供，硬错误
```

- 仅 `status=1` 行参与匹配；停用渠道等同不存在。
- 全部查询恒 `tenant_id=0`（决定 25，显式条件而非租户插件）；同 host 多行取启用首行（id 升序，决定 27）。
- **解析阶段（Step 2）的 zhetaoke `pid` 是配置写死的**（决定 8/§6 注），与渠道无关——
  佣金归属只发生在 Step 3；移植文档 §6 的「解析也归属渠道」诉求本期不做（后续票可把
  channel.pid 传入 zhetaoke）。

## 8. 配置段（`pkg/config/config.go` + `config.yaml`）

```yaml
modules:
  taokouling: true

taokouling:
  zhetaoke:
    appkey: "089b310070524ab5b70fc89ad548c7eb"   # 移植文档值，上线后轮换
    sid: "21649"
    parse_pid: "mm_106702626_354300208_99225400058"   # 解析阶段固定 pid（决定 7/§6 注）
    base_url: "https://api.zhetaoke.com:10001"
    timeout: 5          # 秒，<=0 取 5
    max_retries: 1      # <=0 取 1
  haodanku:
    base_url: "http://v2.api.haodanku.com"   # 源站明文 HTTP，改造需其方支持（改进 10 记风险）
    timeout: 5
    max_retries: 1
  top:
    appkey: "25922261"
    secret: "daffa6bd435c8e8043326d5f3e2903e1"   # 轮换
    user_id: "1060631727"
    base_url: "https://eco.taobao.com/router/rest"
    timeout: 5
    max_retries: 1
  shorturl:
    base_url: "http://api.ft12.com/api.php"
    apikey: "7OYGbioweTYB2YBN7S@ddd"   # 轮换
    timeout: 3
  fallback_search_url: ""    # 如 "https://<站内搜索>?keyword="；空 = 降级返回纯文案（决定 5）
  rebate_rate: 0.05          # 最高返利 = 展示价 × rate（改进 9）
  rebate_tutorial_url: ""    # 返利教程行链接；空 = 省略该行
  rate_limit:
    enabled: true
    limit: 10                # 每 IP 每窗口次数（两接口合计）
    window_seconds: 60
  cache_ttl_seconds: 1800    # <=0 取 1800；0 语义 = 关缓存用 <=0 区分？—— <=0 取缺省，关闭用 enabled:false
  cache_enabled: true
```

- 渠道三元组（pid/apikey/tb_name）**不在** config——落表（决定 10）；haodanku 段只留
  base_url/超时。
- 配置加载测试照 `pkg/config/starfuyu_test.go` 模式补一节。

## 9. 装配触点清单（脚手架 + 手工）

| # | 触点 | 动作 |
|---|---|---|
| 1 | `make new-module NAME=taokouling ENTITIES=channel,log ARGS="-skip-frontend"` | 起手（触点：目录/main.go/config.yaml modules 项/permission_routes/菜单迁移/迁移源）；`go build` 验证 |
| 2 | 脚手架产物按本设计**重写**：model/repository/service/controller/module.go/migrations | scorecard/starfuyu 先例；`log` admin 面收敛只读 |
| 3 | `permission_routes.go` 校正为两前缀 | `taokouling:channel` → `/api/admin/taokouling/channels(+/*)`；`taokouling:log` → `/api/admin/taokouling/logs`（若脚手架按实体生成了别的键，改成本设计） |
| 4 | system 菜单迁移校正（脚手架触点 5 生成的那对） | 目录「淘口令」`/taokouling`（perm `taokouling`、sort=86）+ 页面「渠道配置」`/taokouling/channels`（perm `taokouling:channel`，`views/taokouling/TaokoulingChannelsPage`）+ 页面「转换记录」`/taokouling/logs`（perm `taokouling:log`，`views/taokouling/TaokoulingLogsPage`）；`NOT EXISTS` 幂等、`tenant_id=0`；**按钮不拆**（后端只认前两段） |
| 5 | `pkg/config/config.go` + `config.yaml` | `Taokouling` 结构段（§8）+ `modules.taokouling: true` + `taokouling:` 配置节 |
| 6 | `pkg/errors/code.go` | 30701–30705 常量 + registry 文案（§11） |
| 7 | `alexgo-server/cmd/main.go` | 脚手架已插 FxModule 装配，核对即可 |
| 8 | 前端 `frontend/admin/apps/web-antd` | `src/api/taokouling.ts`（5 函数 + 类型）+ `views/taokouling/` 两页（手写，VxeGrid/Page/Modal 惯例） |
| 9 | `docs/modules.md` 回填 | §1.1 模块地图行、§9.1 前缀两行、§8.2 菜单树、新 §14 章节（脚手架不自动改，人工项） |
| 10 | `alexgo-server/configs/config.yaml` 已有改动注意 | 工作区有未提交改动，新配置节追加时保留现状 |

## 10. 渠道录入参考（移植文档 §6，供 admin 手工录入，不入迁移种子）

| 匹配 | pid | apikey | tb_name |
|---|---|---|---|
| （默认，已种子） | `mm_106702626_399850233_108222550160` | `vate` | `呵颓废呵` |
| host `song.vate.ren` | `mm_351080076_377450234_103423450199` | `whatever` | `东北来的方丈` |
| host `taoquan.luojin.ink` | `mm_102153740_8016819_27096650` | `luojin` | `luojin24` |
| partner `tang` | `mm_359020133_392650105_105996150468` | `may` | `晚安twj` |
| partner `bao` | `mm_106702626_451200291_108545250377` | `vate` | `呵颓废呵` |
| partner `jin` | `mm_106702626_444500415_108514050266` | `vate` | `呵颓废呵` |
| partner `wey` | `mm_106702626_451300217_108547950257` | `vate` | `呵颓废呵` |
| partner `song` | `mm_351080076_377450234_103423450199` | `whatever` | `东北来的方丈` |
| partner `xinhua` | `mm_305470007_294050238_80239450206` | `Xinhua` | `华769858562` |
| partner `jiaming` | `mm_354110024_383200065_104442100474` | `jiaming` | `hjmhh5028` |
| partner `mingzhi` | `mm_48008455_382300476_104622350435` | `mingzhi` | `stylegilr青春女孩` |

## 11. 错误码（30701–30705）

`pkg/errors/code.go` 常量 + registry 双登记（code_test 会钉）；HTTP 映射走 `resp.statusFor`
（3xxxx → 400），**限流 30704 例外直写 429**：

| 码 | 出站文案 | 语义 | 出处 |
|---|---|---|---|
| `30701` | `未识别到淘口令` | Step 1 无口令体（两接口同） | 改进 2 |
| `30702` | `淘口令服务配置缺失` | zhetaoke/top 等关键密钥为空，fail-fast | 改进 6 |
| `30703` | `未配置可用推广渠道` | 默认渠道行缺失/停用（配置断供） | §7 |
| `30704` | `请求过于频繁，请稍后再试` | IP 限流超限（HTTP 429） | 改进 7 |
| `30705` | `淘口令解析失败` | 轻量链路口令过期/商品不存在（zhetaoke 业务失败） | 改进 1 |

其余：参数校验 `10001`、内部错 `50000`（仓库标准）。完整链路业务失败**不进错误码**（降级 200，决定 13）。

## 12. 前端页面（frontend/admin/apps/web-antd，2 页）

| 路由 | 组件 | 功能 |
|---|---|---|
| `/taokouling/channels` | `views/taokouling/TaokoulingChannelsPage.vue` | 渠道分页表（name/partner/host/pid/apikey/tb_name/status）；新建/编辑 Modal（partner/host 可空，说明文案注明空串语义）、删除 Popconfirm（默认行后端拒）；status Switch |
| `/taokouling/logs` | `views/taokouling/TaokoulingLogsPage.vue` | 记录只读分页：时间范围/ partner / status / item_id 筛选；status 中文映射（成功/降级/无口令/硬错误）；error 列 tooltip |

- API 封装 `src/api/taokouling.ts`（listChannels/create/update/delete/listLogs + 类型，
  `requestClient` 统一信封）。
- 页内不使用 `v-access`——可见性靠菜单、操作靠后端 casbin（scorecard/fishgame 同款）。
- 布局照现有页面：`Page` + VxeGrid + `useVbenModal`/`useVbenForm`。

## 13. 测试计划（TDD）

| 层 | 用例要点 |
|---|---|
| `service/password.go` | 移植文档 §3.1 两条真实样例文案提取；`￥`/`¢`/`€` 成对命中与优先级；正文含 `$` 不误判（成对+长度约束）；多符号文本取首个合法口令体；`?tk=xxx` 变体（含空格截断）；无口令 false；【...】标题抓取 |
| `service/render.go` | `coupon=0` 分支（促销价/限时特惠/划线价）；`coupon>0` 分支（券后价=zk-coupon、理由=itemdesc、返利=展示×rate 两位小数）；`rebate_tutorial_url` 空省行；itemdesc 空省理由行；价格缺失省价格行 |
| `service/channel.go` | partner 命中 > host > 默认；`#` 前缀剥离；未知 partner 回退；停用行跳过；无默认行 30703 |
| `service/ratelimit.go` | 窗口内计数、超限 429、窗口重置、enabled=false 放行 |
| `service/cache.go` | TTL 过期、命中/未命中、enabled=false |
| `client/*` | httptest：zhetaoke signurl=0 结构 + `error_response`；haodanku `code:1`/`code:0`；top 签名样例钉死（固定参数 → 期望 md5）+ `data.model` 解析；shorturl 正常/异常 |
| `service/convert.go` | 桩 client：全链路成功；Step2 失败→降级；Step3 失败→降级；Step4 失败→降级；缓存命中跳过；配置空→30702；Parse 失败→30705；log 落库状态正确 |
| `pkg/config` | `taokouling` 段加载（星型测试同款） |
| controller | 绑参校验 10001、429 直写、信封形状（httptest + 轻 mock） |

真钱口令回归（移植文档 §13 验收 1/2/6/7）需**有效第三方凭据 + 现生成口令**，进手工冒烟清单
（计划阶段列出步骤，不进 CI）。

## 14. 方案取舍（已定）

| 方案 | 内容 | 结论 |
|---|---|---|
| A. 1:1 照搬 | 硬编码密钥、固定优先级 split、无错误分支 | 否——移植文档 §11 明令不照搬 |
| **B. 惯例重写 + 改进清单** | 脚手架起手、统一错误码、正则提取、外置密钥、限流/缓存/超时降级 | **选定**（用户） |
| C. 最小闭环 | 无 admin 页/记录表 | 否——用户选交付形态 2 |
| B1 薄直连 | service 内直接 HTTP | 否——测试打桩脏、扩展动 service |
| **B2 client 子包** | `client/` 接口注入 | **选定**（用户）；social `PlatformAdapter` 同款 |

## 15. 验收用例（移植文档 §13 适配）

> **完成定义（决定 26）**：本期门禁 = 代码 + 单测/桩测试全绿 + 下列冒烟清单成文。
> 需真凭据的 1/2/6/7 为**挂起项**：凭据按移植文档值配入 config 后由用户确认可用性再补验；
> TOP 无 `tbk.tpwd.create` 授权时 1/2 的「新口令可用」按降级口径记后续票。

1. **有券商品**：粘贴含 `￥xxx￥` 完整文案 → `convert` 返回文案含「券后价」「新口令」，新口令在淘宝能打开同商品（真凭据冒烟）。
2. **无券商品**：`coupon_money=0` → 「促销价/限时特惠」分支（单测 + 冒烟）。
3. **口令过期/下架**：convert → `degraded:true` + 降级文案，HTTP 200；parse → 30705；不 500。
4. **无口令文本**：两接口均 30701。
5. **URL 型口令**：`?tk=` 可解析（单测钉）。
6. **渠道归属**：带 `partner` 请求，haodanku `ratesurl` 的 pid 与 §10 表一致（service 桩断言 + 冒烟抓包）。
7. **限流**：同 IP 超 10 次/分 → 429 + 30704。
8. **权限**：未登记前缀不存在（permission_routes 断言）；admin 两页菜单可见、记录页只读。
9. **配置断供**：删默认渠道行 → 30703；zhetaoke appkey 置空 → 30702。

## 16. 后续票（本期明确切出）

1. C 端页面（scorecard appui 淘口令页，接 §4.1 契约）。
2. 微信公众号自动回复链路（§9.3 关键词菜单 + conversionEnd 语义）。
3. 淘口令转二维码图、反向链路（推广链接 → 口令）、`tbk.tpwd.parse` 官方解析兜底。
4. 解析阶段 zhetaoke pid 跟渠道走（决定 7 的扩展）。
5. Redis 缓存/限流（多实例部署时）。
6. 短链/haodanku 明文 HTTP 的 HTTPS 化（依赖对方，改进 10 风险记录）。
7. （挂起，决定 26）真钱冒烟补验：凭据可用性用户确认后跑 §15-1/2/6/7；TOP 授权确认与
   `tbk.tpwd.create` 不可用时的替代口径。
