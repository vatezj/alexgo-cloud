# scorecard-appui — 打牌记账小程序前端设计

日期：2026-10-22
状态：范围扩大修订版（待用户确认；原「核心闭环 7 页面」范围已被 2026-10-22 用户指令推翻）
需求来源：终版 UI [`modules/scorecard/appui/desgin/preview/miniapp-breakout.html`](../../../modules/scorecard/appui/desgin/preview/miniapp-breakout.html)（突破版 12 屏，「此版为小程序 UI 最终方向」）+ 主题稿 [`preview/themes.html`](../../../modules/scorecard/appui/desgin/preview/themes.html) + 后端 scorecard 21 接口 + oauth2/social 微信登录

## 背景与目标

打牌记账后端（scorecard + member + social 登录）已全量落地；本子项目在 `modules/scorecard/appui/uniapp`（官方 **Wot Starter v2** 脚手架，用户已初始化）上实现小程序前端，打通「开房 → 邀请 → 记分 → 结算 → 战绩」核心闭环，并**全量落地 breakout 12 屏**：好友排名、使用手册、茶饭设置弹层、头像弹窗、扫码邀请、微信登录、语音播报、4 主题运行时切换。

**范围变更依据（2026-10-22 用户指令）**：「好友排名/手册/茶饭弹层/头像弹窗/扫码/微信登录/语音/4 主题切换 这些都要做丫」——原决策 #2/#4 的「下一期」清单全部转正。

**成功标准**：`pnpm dev:h5` 可跑通主流程联调（连本地后端）；`pnpm build:mp-weixin` 构建通过；房间记分/结算/战绩/好友/微信登录全链路走真实接口；UI 落地 breakout 终版「牌面」视觉；4 主题可运行时切换且合规元素不随主题变。

## 已确认决策

| # | 决策项 | 结论 |
|---|---|---|
| 1 | 脚手架 | Wot Starter v2（vitesse-uni-app 底座 + Wot UI v2 + Alova + UnoCSS + 文件路由 + `@wot-ui/router`），位于 `modules/scorecard/appui/uniapp`，**不引入第二套脚手架** |
| 2 | 本期范围 | **全量 12 屏**（2026-10-22 用户指令扩大）：首页 / 开房 / 房间 / 邀请（真二维码+扫码）/ 支出 / 结算 / 战绩 / **好友排名** / **使用手册** / **茶饭设置弹层** / **头像弹窗（语音 5 档/退出）** / **微信登录** / **4 主题切换**。自定义统计（稿内自带「开发中」标注）仍置灰占位 |
| 3 | 数据接入 | **直连真实接口** `/api/app/scorecard/*` + `/app/member/auth/login` + **`/app/social/wechat/mp/login`**（微信登录，后端已落地）+ `/app/friends*`；不用 mock 数据渲染 |
| 4 | 主题 | **4 套运行时切换**：①墨金牌桌 t1（BASELINE，炭墨底 `#14120F` + 墨绿桌 `#12261E` + 骨白 `#F2EDE3` + 黄铜 `#C9A24B`）②深海筹码 t2（`--ink-900:#0D1420; --brass:#D98E5F; --win-dark:#F0A93B`）③红黑赌场 t3（`#151113/#C4C9D0/#F0546A`）④青瓷茶室 t4（`#0B1512/#9CC4A1/#DDB455`）；themes.html 覆盖类为 token 源；**合规红 + 红中印全局锁定，不随主题变** |
| 5 | 设计依据 | HTML 视觉稿 `miniapp-breakout.html` 为**唯一样式源**（token/组件类名直接翻译）；`themes.html` 为主题 token 源；Figma 牌面方案未进 Figma（额度阻塞），不作为实现依据 |
| 6 | 目标平台 | 主目标 `mp-weixin`；开发期用 H5（`dev:h5`）联调。布局/交互避免仅 H5 可用的 API；**微信登录/扫码/语音仅 mp-weixin 可用，H5 走降级路径**（见 §3.6/§3.9/§3.10） |
| 7 | 身份缺口 | 后端**开小票前置**（一个票两端点）：`GET /app/member/me`（user_id/昵称/头像/注册时间）+ **`PUT /app/member/me`**（更新昵称/头像，头像弹窗「修改昵称」消费；全仓现无任何 member 资料写接口）。登录接口本身不动；前端登录成功后调 me 存 store |
| 8 | 战绩行降级 | 一期 record 行显示「日期 + ±分 + 结算类型」，排行只显分数不显昵称；**后置票**（下一期）：record 补 room_code/昵称冗余 |
| 9 | 底分字段 | breakout 稿的「底分」后端全链路无此概念，**删除**；中央铭牌只保留茶饭池 + 茶饭配置摘要（点击进茶饭弹层，见 §3.9.2） |
| 10 | 导航结构 | **不做 tabbar**：删 about 演示页，全部页面 push 页（navigationStyle custom 自绘 nav）；弹层（邀请/茶饭/头像/房号输入/流水更多）为页内 popup 非路由 |
| 11 | 流程裁定 | 以本设计文档 + superpowers 规格流程为准；`uniapp/CLAUDE.md` 的 OpenSpec 强制视为脚手架模板残留，**豁免不走** |
| 12 | 登录方式 | **微信一键登录为主**：`wx.login` 取 code → `POST /app/social/wechat/mp/login {code}`（公开端点，后端 code2session + unionid 合并 + 壳账号自动注册），返回 `{token, refresh_token, expires_in}` 与 member auth 同构，直接复用同一 token 存取链路。**手机号+密码保留为辅**（H5 开发期/无微信环境兜底）；绑定手机走后端已有 `bind-mobile`（非本期 UI 必达） |
| 13 | 语音方案 | **5 档循环设置 + 播报器抽象**（稿内明文：全部关闭 → 全部播放 → 娱乐关闭 → 流水关闭 → 操作播放；三类 = 娱乐/流水/房间动态）。TTS 引擎：mp-weixin 用**微信同声传义插件（WechatSI）**（manifest 声明 + 运行时探测，插件未申请/不可用 → 静默降级只 toast 不播报，不阻塞业务）；H5 用 Web Speech API（不可用同样静默降级）。播报词模板化（昵称+数字拼接，无词库依赖） |
| 14 | 扫码邀请 | **小程序码**（2026-10-22 用户指令：「扫码邀请 使用小程序二维码」）：邀请弹层展示**后端生成的小程序码图片**（`getwxacodeunlimit`，scene=房号，落地页=首页）。**含后端小票**：`GET /app/scorecard/rooms/:id/wxacode`（Bearer 鉴权 + 房存在校验，返回 image/png；内部复用 oauth2/social 的微信 appid/secret 配置与客户端，social 模块补 stable_token + getwxacodeunlimit 方法）。进房路径：① 小程序码图片/分享卡片被点开 → 小程序 onShow 解析 `options.scene` → 登录后 `joinRoom`；② 首页「扫码进房」`wx.scanCode`（扫相册/另一屏上的码，结果取 `result.path`/`scene`）→ 同链路。**转发给好友** = open-type=share 分享卡片（路径带房号）。H5 降级为「显示房号+复制」（无小程序码）。开发期注意：page 须已发布，体验版用 `env_version=trial`；未配置 appid 时降级房号展示 |
| 15 | 流程裁定补充 | 主题入口、加好友入口等稿内未给定的细节为**我的裁定**（文中逐处标注），builder 可微调不改范围 |

## 1. 目录与路由

### 1.1 目录（脚手架约定内扩展）

```
modules/scorecard/appui/uniapp/src/
├── pages/
│   ├── index/index.vue        # 首页（扇面牌桌 01）
│   ├── room/index.vue          # 房间（俯视牌桌 02 + 流水 + 筹码轨）
│   ├── expense/index.vue       # 支出（筹码加减档 04）
│   ├── settle/index.vue        # 结算（竖排开牌 05）
│   ├── records/index.vue       # 战绩（07 左滑流水/排行/删除）
│   ├── friends/index.vue       # 好友排名（08 左滑删除/拉黑 + 参与开关）
│   ├── manual/index.vue        # 使用手册（06 花色卡长页，静态内容）
│   └── login/index.vue         # 登录（微信一键 + 手机号密码兜底）
├── components/scorecard/       # 模块组件（自动加载）
│   ├── PlayCard.vue            # 牌面卡（pcard 底衬/角标/花色条纹）
│   ├── ChipButton.vue          # 筹码圆钮（±，brass/dark，sm/xs）
│   ├── ScoreText.vue           # Oswald 数字（win/lose/jade 色）
│   ├── RoomRoster.vue          # 顶部头像座位列表（横滚）
│   ├── RoomLedger.vue          # 本局流水账板
│   ├── ExpenseLane.vue         # 支出档（分数档/茶饭档）
│   ├── RecordSwipable.vue      # 战绩左滑行
│   ├── FriendRow.vue           # 好友排名行（左滑 + 参与开关）
│   ├── QrInvitePanel.vue       # 邀请弹层（扇牌小程序码 + 转发）
│   ├── TeaConfigSheet.vue      # 茶饭设置底部弹层（09）
│   ├── AvatarPopup.vue         # 头像弹窗（10：昵称/语音/主题/退出）
│   ├── VoiceToggle.vue         # 语音 5 档循环开关
│   └── ComplianceBanner.vue    # 合规横幅（记账工具不涉金钱）
├── api/scorecard/              # API 层（手写，不走脚手架 codegen）
│   ├── types.ts                # 对齐后端 DTO（RoomSnapshot 等）
│   ├── room.ts / record.ts / friend.ts / auth.ts / me.ts / social.ts
├── store/scorecard/            # pinia：roomStore / themeStore / voiceStore（persist）
├── composables/
│   ├── useRoomPolling.ts       # 快照轮询
│   ├── useVoice.ts             # 播报器抽象（TTS 引擎探测 + 档位过滤）
│   └── useSceneRoom.ts         # onShow scene 解析 → 房号 → joinRoom
└── theme/scorecard.scss        # 牌面 token（.theme-t1..t4 四套变量 + 工具类）
```

### 1.2 路由（pages.json，文件路由生成）

| 路由 | 页面 | 导航 |
|---|---|---|
| `pages/index/index` | 首页 | 普通 push 页（**无 tabbar**，删 about 演示页与 tabbar 布局） |
| `pages/room/index?roomId=` | 房间 | 非 tab，开房/进房/扫码后 `router.push` |
| `pages/expense/index?roomId=` | 支出 | 房间筹码轨「支出」进入 |
| `pages/settle/index?roomId=` | 结算 | 房间内结算入口 |
| `pages/records/index` | 战绩 | 首页「战绩流水」/结算托盘「历史房间」 |
| `pages/friends/index` | 好友排名 | 首页「好友排名」pill |
| `pages/manual/index` | 使用手册 | 首页「使用手册」pill / 结算托盘「使用手册」/ 合规横幅「了解详情」 |
| `pages/login/index` | 登录 | 401 兜底跳转（脚手架 handlers 已有）/ 首次进入 |

## 2. 鉴权与请求层

### 2.1 登录与「我是谁」（含前置后端小票）

- **主路径（微信）**：`wx.login` 取 code → `POST /app/social/wechat/mp/login {code}`（公开，无需验证码）→ `{token, refresh_token, expires_in}` → 调 me 存 store。code 一次性、失败 toast 提示重试。
- **辅路径（手机号+密码）**：`POST /app/member/auth/login`（captcha 由配置开关 + `/oauth2/captcha` 自适应）；H5/开发期用。
- **前置后端小票（本子项目第一步先做，一个票两端点）**：
  - `GET /app/member/me`：Bearer 自鉴权，返回 user_id/昵称/头像/注册时间（builder-check 确认全仓无 member 端「我是谁」接口，`/api/user/info` 对 member 403）。
  - `PUT /app/member/me`：更新昵称/头像（头像弹窗「修改昵称」消费；member 模块现无任何资料写接口）。头像上传复用文件存储子项目的上传能力（builder 实现时确认 app 端上传端点，若不可用则一期仅支持改昵称 + 头像用临时 URL 本地图，标注）。
- **后端小票（同批，决策 #14）**：`GET /app/scorecard/rooms/:id/wxacode`——小程序码生成（getwxacodeunlimit，scene=房号，page=首页路径，体验版 env_version=trial），Bearer 鉴权 + 房存在校验（30101）；图片字节直出（image/png）。后端实现：social 模块补 `stable_token` 获取 + `getwxacodeunlimit` 调用（复用 SocialApp 渠道配置里的 appid/secret 与 WechatMP baseURL；secret 泄露防护沿用 wechat_mp.go 的 URL 剥离先例），scorecard 模块挂 app 路由消费；同 scene 结果短 TTL 缓存（如 5 分钟，量小防频调）。
- 前端流程：登录成功 → 存 token → 调 me → `{token, user}` 一起持久化 `store/persist`；`beforeRequest` 统一注入 `Authorization: Bearer <token>`。
- 401/403：脚手架 `handlers.ts` 已实现清态 + 跳 `login`，沿用。

### 2.2 API 层（`api/scorecard/`）

手写 alova method 工厂（不跑脚手架 apiDefinitions codegen），覆盖本期消费的接口：

| 方法 | 接口 | 消费页 |
|---|---|---|
| `createRoom` / `joinRoom` | `POST /rooms` · `POST /rooms/join` | 首页/扫码/邀请 |
| `snapshot` | `GET /rooms/:id/snapshot` | 房间/支出/结算（轮询） |
| `logs` | `GET /rooms/:id/logs` | 房间流水分页弹层 |
| `expense` | `POST /rooms/:id/expense` | 支出（带 `client_request_id` 幂等） |
| `updateConfig` | `PUT /rooms/:id/config` | **茶饭设置弹层（09，本期正接）**；仅房主，数值项「从下把生效」 |
| `settle` / `leave` | `POST /rooms/:id/settle` · `/leave` | 结算页 / 头像弹窗「退出房间」 |
| `myRecords` / `recordDetail` / `deleteRecord` / `stat` | `/records*` · `/stat` | 战绩页 |
| `listFriends` / `removeFriend` / `setBlack` / `setRank` / `addFriend` | `/friends*` | **好友排名页（08，本期正接）** |
| `wxLogin` | `POST /app/social/wechat/mp/login` | 登录页（微信一键） |
| `getRoomWxacode` | `GET /rooms/:id/wxacode`（**后端小票**） | 邀请弹层（小程序码图片） |
| `login` / `refresh` | `/app/member/auth/login` · `/refresh` | 登录页（辅路径）、token 刷新 |
| `getMe` / `updateMe` | `GET/PUT /app/member/me` | **前置后端小票（决策 #7）** |

- 响应信封 `{code, msg, data, total}` 与脚手架 `handleAlovaResponse` 对齐；业务错误码 toast 沿用。
- **X-Tenant-ID**：app 端 handler 以 token 租户覆盖 header（伪造头无效），请求层不硬编租户头；如中间件层有必填校验，实现时补一个占位头即可（builder 自行验证）。

### 2.3 房间实时性（无 WS）

- `useRoomPolling(roomId)`：进入房间页起 **3s 间隔轮询 snapshot**（前后台切换/下拉刷新时立即拉），按 `version` 去重，过期丢弃；支出成功后以响应内嵌 snapshot 立即更新并重置轮询计时。
- 首页「进行中房间」：进首页拉一次 snapshot 恢复（存 store 的 lastRoomId）。

## 3. 页面设计（对齐 breakout 终版 12 屏）

视觉 token 直接翻译自 `miniapp-breakout.html` `:root`（`--ink-900/--felt/--tile/--brass/--vermillion/--indigo/--jade` 等）；字体 Oswald（数字，网络字体或本地化）+ 系统中文（Noto Serif SC 标题以系统 serif 降级，小程序字体嵌入体积受限，允许降级）。

### 3.1 首页（01 扇面牌桌）

- 身份牌（pcard）：昵称/头像（me 接口）+ 总积分/胜率（`stat`）+ 注册天数（me.created_at 推算）+ 好友数（`listFriends.length`）。
- 扇形 CTA：**我要开房**（`createRoom`，默认配置固定值）/ **扫码进房**（mp 端 `wx.scanCode` 扫小程序码图片，解析 `result.path`/`scene` 取房号 → `joinRoom`；扫到其他小程序码微信侧直接打开对应小程序，本端只处理含本小程序 scene 的结果；H5 降级为房号输入弹层，文案保留）。
- 快捷 pill：**好友排名** → `pages/friends`；**战绩流水** → `pages/records`；**使用手册** → `pages/manual`（全部真实页面，无置灰）。
- 最近战绩：`myRecords(page=1,size=5)` → 条目（日期·±分·结算类型）。
- 底部邀请入口：房间进行中时显示邀请按钮 → 邀请弹层（§3.9.1）。
- 广告位：占位虚线卡（无投放）；底部 footpills 静态。

### 3.2 房间（02 俯视牌桌）

- 合规横幅 `ComplianceBanner`（可关，关后本会话不再显示；「了解详情」→ 手册页）。
- 俯视牌桌：顶部 `RoomRoster` 横滚座位（头像/昵称/当前分，me 描红——对比 me.user_id，房主徽标对比 `creator_id`）；中央最近一手（from −score → to +score，flyer 动画）+ 茶饭池铭牌（**无底分**，显示茶饭池 + `tea_percent` 摘要；**点击 → 茶饭设置弹层** §3.9.2，仅房主可改，非房主只读查看）。
- 本局流水 `RoomLedger`：snapshot.recent_logs 20 条 + 「查看更多」→ logs 分页弹层（`GET /rooms/:id/logs` 上拉加载）。
- 筹码轨：中央大筹码「支出」→ expense 页；右侧「我的」→ 头像弹窗（§3.9.3）；左侧客服占位。
- 邀请：顶部房号区/••• → 邀请弹层（§3.9.1）。
- 成员点击：快捷支出（预填 to_user_id 进 expense 页）；长按或头像 → 头像弹窗（该成员资料 + 我在房内时含语音/退出）。
- 状态门：`status!=0`（已结算/已关闭）跳 settle 页只读。

> 注：视觉稿中央铭牌的「底分 10」为幽灵字段（后端无 base score 概念），按决策 #9 删除，不实现。

### 3.3 支出（04 筹码加减档）

- 分数档：−/＋筹码钮（步进 10 可长按连加，builder 定）+ 预设 10/50/100/200；收款方 = 快捷进入时预选成员，否则房间成员横选。
- 茶饭档：手动 DIY（`tea` + `tea_from_user_id` 缺省支出方）；仅当 `tea_auto_enabled=false` 显示，否则显示自动抽取说明（`tea_percent`）。
- 提交：`expense{score, tea, to_user_id, client_request_id: uuid}`；成功回房间页 + toast + **流水播报**（§3.11）；幂等键本地生成一次，重试复用。
- 底部说明「只记分数，不涉及金钱」。

### 3.4 结算（05 竖排开牌）

- 触达：房间内房主「结束牌局」或 status=1 进入只读。
- 排行大牌：总分第一 = 赢家牌（win-card）；竖排「战斗胜利」标题。
- IOU 行：非零分成员按分数生成**贪心简化转账策略**（客户端计算，最小转账数；标注「策略参考，可不按」）。
- 操作托盘：直接退房（`leave`）/ 房间结算单（明细列表）/ 总分排行榜 / 房间流水 / 历史房间（=战绩页）/ **使用手册（→ pages/manual 真页）**。
- 房主专属：「结束牌局」`settle` 二次确认。

### 3.5 战绩（07 房间记录 + 左滑）

- 顶部 stat-strip：`stat`（总局数=win+lose+draw / 胜率 / 累计净成绩）。
- 玩法筛选 chip：**数据侧 record 无玩法/房号字段**（决策 #8）——「全部」为唯一有效筛选，玩法 chip 视觉保留置灰。
- 房间记录列表：`myRecords` 分页，**行内容降级为「日期 + ±分 + 结算类型」**；`RecordSwipable` 左滑：流水（recordDetail 全房反查，行= user_id 序号 + ±分，**无昵称**）/ 排行（detail 按分排序，只显名次+分数）/ 删除（`deleteRecord` 二次确认）。
- record 增强（补 room_code/昵称冗余）为**后端后置票**，届时行布局升级为视觉稿完整版。
- 「自定义统计」入口置灰「开发中」（**稿内自带该标注**，非我方裁剪）。

### 3.6 登录（微信一键 + 手机号兜底）

- **微信一键登录**（mp-weixin 主路径）：`wx.login` → code → `wxLogin` → token + me → 回跳原页面。无需用户表单。
- **手机号 + 密码**（辅路径，折叠区或 H5 默认展示）：`login` 接口；错误 toast；不做注册（微信登录自动壳账号；密码路径用后端已有/admin 创建账号）。
- 登录后回跳原页面；登录页整体品牌沿用牌面视觉（骨白 pcard 底 + 黄铜 CTA）。

### 3.7 好友排名（08 左滑删除/拉黑 + 参与开关）

- 列表：`listFriends`（后端已按 total_score 降序，含昵称/头像/胜率/我的标记）；头部 stat：好友数 + 我的排名位。
- 我的行开关：「加入排名/关闭后不参与好友榜单，互相不可见」→ `setRank`（我方指向…开关，后端 `PUT /friends/:userId/rank`；**稿中「加入排名」是对整榜的参与开关——实现为对每个好友行的 rank_enabled 开关展示 + 顶部汇总开关（我的裁定：顶部开关联动逐行）**）。
- 内页左滑（`FriendRow`）：**删除**（`removeFriend` 二次确认）/ **拉黑**（`setBlack(true)`）；拉黑行展示标记可解除。
- **加好友入口**：稿内未给 UI（**我的裁定**：空态 + 头部「+」弹输入框，输入对方 user_id → `addFriend`；好友数据初始为空，无入口则页面不可用）。昵称由后端 briefs 补齐展示。
- 「点图标可清零战绩」：稿内文案，对应删除好友（战绩榜清出）；一期按删除实现，不做独立清零动作（**我的裁定**，builder 可在文案上对齐稿）。

### 3.8 使用手册（06 花色卡长页）

- 静态内容页（无接口）：**完整转写 breakout 06 屏各章节文案**——①重要提示（合规红线/网络/下拉刷新）②结算·退房·历史记录 ③茶饭设置 ④房间语音（三类 + 5 档循环说明）⑤好友排名 ⑥其他技巧；版本号展示与 `package.json` 对齐。
- 花色卡章节视觉：pcard 章节头（♠♥♦♣ 角标 + 花色条纹），滚动长页，自绘 nav。
- 入口：首页 pill / 结算托盘 / 合规横幅。

### 3.9 弹层组（页内 popup，非路由）

#### 3.9.1 邀请弹层（03 扇牌小程序码）

- 扇形牌背景 + 房号大字 + **小程序码图片**（`getRoomWxacode(roomId)` 返回的 image/png，`<image>` 直接展示，无前端绘码）+ 文案「邀请好友扫描牌背小程序码加入房间」。
- 进房主路径：好友点开小程序码图片/长按识别 → 微信直接拉起小程序并带 `scene`（如 `qr=7AVW`）→ `useSceneRoom`：App onShow 解析 `options.scene`（decodeURIComponent）暂存房号 → 完成登录后 `joinRoom` → 进房间页；未登录先登录再进房。
- 「转发给好友」：`button open-type="share"` 分享卡片（路径带房号，被分享者进房）；H5 降级为「复制房号」。
- 扫码辅路径：首页「扫码进房」`wx.scanCode`（扫相册/另一屏上的码）→ `result.path`/`scene` 解析房号 → `joinRoom`；识别不出 toast「无效二维码」。

#### 3.9.2 茶饭设置弹层（09 底部弹层）

- `TeaConfigSheet`（`wd-popup` position=bottom）：茶饭目标金额 / 茶饭比例 percent / 自动抽取开关 / 单人累计上限 max_lose_score。
- **仅房主可编辑**（对比 snapshot.creator_id 与 me.user_id）；非房主只读展示当前值。
- 保存 → `updateConfig`（数值项后端写 pending「从下把生效」，前端提示「下一把生效」）；开关即时生效（后端语义）；成功以响应 snapshot 刷新。
- 展示当前生效值 + pending 值（snapshot 有 `pending_apply_round_no` 时显示「已暂存，第 N 把生效」——字段如快照未下发 pending 明细则只显示文案提示，builder 以实际 DTO 为准）。

#### 3.9.3 头像弹窗（10 昵称/语音/主题/退出）

- 触达：房间筹码轨「我的」/ 点自己头像。
- 内容：头像 + 昵称 + 「修改昵称」（`updateMe`，输入框弹层；头像更换走微信头像选择能力 + PUT，受上传能力限制时降级仅昵称，见 §2.1）。
- **语音 5 档循环开关**（`VoiceToggle`）：全部关闭 → 全部播放 → 娱乐关闭 → 流水关闭 → 操作播放 → 循环（档位文案与手册 §3.8 一致；写 voiceStore persist）。
- **主题入口**（**我的裁定**：稿内未给主题切换 UI，themes.html 仅为配色展示页——入口放头像弹窗，横排 4 色卡：墨金/深海/红黑/青瓷，点选即切 themeStore）。
- **退出房间**：`leave`（房主先自动结算，后端语义；二次确认）；不在房时显示「退出登录」（清 token 回登录页）。
- 「查看资料」：展示 me 信息（昵称/ID/注册时间），只读。

### 3.10 语音播报（useVoice）

- **档位模型**（voiceStore persist）：三类开关 `{entertain, ledger, roomDyn}`，5 档循环映射：
  1. 全部关闭 = 三类全 false
  2. 全部播放 = 三类全 true
  3. 娱乐关闭 = entertain false
  4. 流水关闭 = ledger false
  5. 操作播放 = 仅 roomDyn true（房间动态/操作类）
- **播报事件源**（一期接线）：支出成功 → ledger 类（「A 向 B 记 50 分」模板）；进出房/结算/上限提醒 → roomDyn 类；结算胜利等氛围播报 → entertain 类。事件由页面在成功回调处调 `useVoice().speak(text, category)`。
- **引擎**：`mp-weixin` → 微信同声传译插件（WechatSI，manifest.json plugins 声明；**需小程序后台添加插件**——若未添加，运行时探测失败静默降级，只 toast 一次「语音插件未启用」）；H5 → `speechSynthesis`（不可用静默降级）。
- 播报失败/静音环境一律静默，**永不阻塞业务链路**。

### 3.11 主题切换（4 套运行时）

- `theme/scorecard.scss` 定义四组变量类：`.theme-t1`（默认 BASELINE 墨金）/ `.theme-t2` / `.theme-t3` / `.theme-t4`，token 照抄 themes.html 覆盖（`--ink-900/--brass/--win-dark/--felt` 等整组）。
- `themeStore`（persist）存当前主题 id；App 启动恢复 → 根节点（app 或每页根）挂对应 class；全部组件只引用 CSS 变量，无硬编码色值。
- **合规锁定**：`--vermillion` 合规红 + 红中印元素不进主题覆盖（全局恒定），保证任意主题下合规视觉一致。
- 切换入口：头像弹窗（§3.9.3）；切换即时生效无需重启。

## 4. 组件规范

- Wot UI 组件按需用（`wd-toast` 已全局、`wd-popup`（底部弹层）、`wd-swipe`（左滑）、`wd-icon`）；**牌桌/筹码/牌面等强定制视觉不用 Wot 组件硬套**，自绘组件为主。
- UnoCSS 原子类与 scss 变量并存；避免行内 style 泛滥（token 类优先）。
- 暗色底全局：`pages.json` globalStyle 背景 `var(--ink-900)`；导航栏自绘（`navigationStyle: custom`）。
- 底部安全区：`--pad-safe` 用 `env(safe-area-inset-bottom)`。
- 所有色值走 CSS 变量（主题切换前提）；`.num`（Oswald tabular）、`.win-n/.lose-n/.jade-n`、`.btn.brass/.bone/.ghost`、`.pcard`、`.chip`、`.comply`、`.sheet` 工具类照翻译。

## 5. 非目标 / 后置

- 自定义统计（**稿内自带「开发中」标注**，入口置灰）、广告投放（占位卡）、转让房主 UI（后端接口在，稿内无 UI）、管理端页面、bind-social/绑定手机 UI（接口在，非本期必达）。
- 后端改动仅限**前置 me 票 GET+PUT**（决策 #7）**+ 小程序码票**（决策 #14，social 补 getwxacodeunlimit + scorecard 挂路由）；其余接口 gap（含 record 增强票，决策 #8）记票后置。
- 微信同声传译插件的**小程序后台申请/添加**为运营侧动作（非代码），未完成时语音走降级路径不阻塞发版。

## 6. 约束与风险

- **产品红线**：界面文案与代码命名沿用后端口径 `score`/`tea`，不出现货币符号/资金词汇；合规横幅必达（房间/支出页）。注：手册稿内「💰流水关闭」等 emoji 为视觉稿原文，落地时优先用中性图标（builder 酌情，不改变语义）。
- **小程序字体**：Oswald 用 CDN 或精简子集；失败降级系统字体，数字仍 tabular。
- **轮询代价**：3s 轮询仅房间页生命周期内；后台/离开即停。
- **主题回归面**：4 主题 × 全页面色值必须全走变量，代码评审检查硬编码色；合规红/红中印锁定项单列白名单。
- **微信能力降级矩阵**：wx.login/wx.scanCode/小程序码/同声传译/分享 —— mp-weixin 全量，H5 各自降级（登录→密码、小程序码/扫码→输房号+复制、语音→Web Speech/静默、分享→复制房号）；开发期以 H5 联调为主，发版前 mp-weixin 真机回归。
- **小程序码约束**：`getwxacodeunlimit` 的 page 须为已发布页面路径（开发期体验版传 `env_version=trial` 指向体验版）；scene 上限 32 位（4 位房号远小于上限）；appid/secret 缺失或微信接口失败时弹层降级为房号大字 + 复制，不阻塞邀请。
- **脚手架升级冲突**：`appui/uniapp` 保持独立 git 价值（当前在 alexGo-cloud monorepo 未跟踪/或一并提交，builder 按仓库惯例）；不向上游脚手架回馈。
- **联调环境**：`VITE_API_BASE_URL` 指向本地后端；H5 跨域开发期用 vite proxy（builder 确认脚手架是否已配，没有则补）。
