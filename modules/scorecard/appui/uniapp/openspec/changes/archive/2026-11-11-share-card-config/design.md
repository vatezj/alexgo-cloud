# Design: share-card-config

## Context

现状（见 proposal.md - Why）：分享 title/图在 `src/pages/index/index.vue` 硬编码为 `SHARE_TITLE` 常量与七牛 logo URL；模拟器、工资记账两页无 `onShareAppMessage`/`onShareTimeline`。服务端侧 `GET /api/public/scorecard/share-config`（同仓库 Go：`modules/scorecard/controller/app/public.go` + `service/sharecfg.go`，免鉴权、统一信封、key 缺失/坏 JSON 回退内置默认）与后台参数页可视化编辑已落地，本变更只做小程序端消费。

约束：微信转发钩子是同步函数（不能 await 请求后返回 title），必须读缓存；配置请求失败不得打扰转发；H5 无原生转发场景。

## Goals / Non-Goals

- **Goals**：三页分享 title/图后台可配；转发同步可读（内存 → storage → 内置默认三级回退）；接口失败零打扰。
- **Non-Goals**：不改后端契约与默认值；不做分享数据埋点/统计；不做 H5 端分享 UI（含 Web Share API）；不改 `inv=` 邀请绑定与 scene 协议（`share-inviter-bind` 承载）；不做图片上传（后台已完成）。

## Decisions

1. **取数放 store（Pinia）而非页面各自请求**
   - `src/store/scorecard/shareConfig.ts`：state = `cards`（内存缓存）；action `ensureFresh(force?)` 拉取后写 storage + state；getter `get(page)` 读内存 → `uni.getStorageSync('scorecard:shareCards')` → 内置 DEFAULTS。
   - 理由：三页共享一份数据，store 是项目现成模式（`store/scorecard/auth` 同款）；页面只读 getter，钩子里零异步。
   - 备选：composable + 模块级变量——storage 回读要每个页面重复接线，放弃。

2. **拉取时机 = App 启动 + 各页 onShow 轻刷新**
   - `App.vue` onLaunch/onShow 调 `ensureFresh()`（不 await）；三页 onShow 再调一次做后台刷新（去重：在途请求不重复发起）。
   - 理由：保证「后台改完、用户下次进页就生效」；钩子本身永不同步等待。
   - 备选：只在 onShare 钩子内同步读 storage——后台改完后当前进程永远看不到新值（钩子同步、发不出请求），不行。

3. **分享钩子内联读 getter，空 imageUrl 不传字段**
   - 三页 `onShareAppMessage` 模板一致：`title = get('x').title`；`imageUrl` 非空才放进返回对象（微信对空串行为不保证，显式省略最稳）。首页保留原 `inv` 分支原样。
   - 模拟器/工资页 path 落自身页面；两页不带 inv（它们不是绑好友主入口，且 scene 协议 `inv=` 目前绑定在首页消费链上——避免半接线）。首页 inv 语义不动。

4. **DEFAULTS 常量与服务端 `sharecfg.go` 同值**
   - store 内置默认逐字等于服务端 `shareCardDefaults`（title「登录后开房记账 · 好友互通」+ 七牛 logo 页；另两页各带默认 title、空图）。理由：三级回退的最终态 = 硬编码时代行为，接口全挂也不劣化。
   - 备选：默认值从别处注入——过度设计，两处常量靠 spec 场景回归。

## Risks / Trade-offs

- [storage 被清后首发无缓存] → 该次转发用内置 DEFAULTS，下次 onShow 拉取即恢复；可接受。
- [onShow 频繁拉取] → 在途去重 + 响应本身极小（~300B）；仍超预期时可加 TTL（Open Question）。
- [后台改了非法 title（超 64 字等微信限制）] → 微信端截断/拒转由平台决定；后台表单不加限长，观察为主。
- [两新页转发带来 scene/inv 消费歧义] → 新页 path 不带 inv、不触碰 pendingScene，与 `share-inviter-bind` 零交集。

## Migration Plan

无数据迁移。发布顺序：后端与后台已先落（本变更前）→ 本变更随小程序版本发布；回滚 = 撤销本变更（后台配置残留无害，端不再读）。

## Open Questions

- onShow 刷新是否需要 TTL（如 60s 内不重复拉）——先按在途去重上线，观测请求量后再定，不影响 spec 与任务拆分。
