# starfuyu 图片版微信软文 设计文档

日期：2026-11-16
状态：待批准
前置：`2026-11-12-starfuyu-module-design.md`（模块整体）、`2026-11-13-starfuyu-fortune-card-design.md`（分享卡/生图）、`2026-11-16-starfuyu-poster-title-photo-design.md`（v3 海报）

## 1. 背景与意图

现有公众号软文是纯文字版（幸运色 + 五维分数段，`article.go` goLucky 照搬样式）。
运营要改成参考图（桃桃每日好运）那种**图片为主**的版式：

- 顶部：日期徽章 + 「重点星象：…」一行 + 开篇总述段；
- 中部：**火/土/风/水四象分组**，每组一个粉底胶囊标题，组内**三张一行**放该组 3 个星座的图片（即我们生成的 v3 运势海报 `fortune/v3/{date}/{constellation}.jpg`）；
- 底部：好运句 + 关注引导（**星福柚**，不放社媒行——用户已裁定）。

「重点星象 + 开篇总述」是**当日全局一条**数据，现库没有（fortune 表是每星座一行）——
用户裁定：**DeepSeek 抓取时一并生成，新表 `starfuyu_daily` 存储**。

## 2. 目标 / 非目标

**目标**
- DeepSeek 单次调用同时产出 12 星座运势 + 当日星象/总述，落新表。
- 整版软文 HTML 重写为图片版四象分组布局；正文图片用微信 CDN URL。
- `UploadToMP`：Ensure 12 张 v3 海报 → 逐张传正文图片 → 组装 → 草稿箱。
- admin 预览接口返回同版式 HTML（图片用公开读相对路径）。

**非目标**
- 不改单星座软文（`constellation!=""` 分支维持旧文字版）。
- 不改海报生图、标题/角色布局（前一计划的事）。
- 不做社媒行、不做人工星象后台。
- 不清理旧文字版草稿。

## 3. 数据层

### 3.1 新表 `starfuyu_daily`（迁移 20261116000001）

| 列 | 类型 | 说明 |
|---|---|---|
| id | bigint PK auto | |
| tenant_id | bigint not null default 0 | 恒 0，与 fortune 同精神 |
| date | varchar(10) not null | YYYY-MM-DD |
| astro | varchar(64) not null default '' | 重点星象（如「月水星，月土六合，木火合」） |
| intro | text | 开篇总述（≤60 字） |
| created_at / updated_at | datetime | gorm 自动 |

唯一索引 `uk_daily_tenant_date (tenant_id, date)`。down 迁移 DROP TABLE。
`model/daily.go`：`DailyHoroscope` 结构 + `TableName()`；repository `DailyRepository`：
`GetByDate(ctx, date) (*DailyHoroscope, bool, error)`、`Upsert(ctx, row) error`（冲突更新 astro/intro）。

### 3.2 DeepSeek 响应结构升级

`callAPI` 输出从「纯 JSON 数组」改为**对象包裹**：

```json
{
  "daily_astro": "月水星，月土六合，木火合",
  "daily_intro": "这天是一个积极热情有能量的日子，……（≤60字）",
  "fortunes": [ /* 12 个星座对象，结构不变 */ ]
}
```

- prompt 同步改：输出规范 1 改对象包裹；新增字段说明（daily_astro ≤20 字、daily_intro ≤60 字，
  两者与星座无关、全篇同一条；生成要求追加一条）。
- **解析容错**（既有精神）：先按对象解；失败回退按纯数组解（模型偶发旧习惯），
  daily 落空、12 行照常——**不判失败**。对象解出但 `fortunes` 缺/空 → 才算失败。
- `FortuneFetcher` 接口签名改为
  `FetchFortunes(ctx, date) ([]*model.Fortune, *model.DailyHoroscope, error)`
  （daily 可为 nil）。桩两处同步：`fortune_test.go` stubFetcher、`module_test.go` stubFetcher。

### 3.3 落库

`FortuneService.Fetch`：rows 校验通过后，daily 非 nil → 同事务/紧随其后 `Upsert` 日表
（daily upsert 失败 → 整体 30501/30502 归类为 fetch 失败，与「整批失败不落」精神一致——
实现取：daily upsert 放 rows upsert **之后**，失败返回错误但 rows 已落，记日志不回滚
——**裁定：日表是增强数据，缺了文章降级渲染，不值得两表事务**；ledger 记此 Ruling）。

## 4. 文章生成（article.go）

### 4.1 版式（整版 `constellation==""` 新实现）

自上而下（全部 inline style + table，微信编辑器安全子集）：

1. **日期徽章**：`<div style="border:1.5px solid #F2A8A8; border-radius:8px; display:inline-block; padding:6px 16px; color:#E86A6A; font-size:16px;">2026/11/16</div>`（MM/DD/YYYY 斜杠格式）。
2. **重点星象**（daily.Astro 非空才画）：加粗一行 `重点星象：{astro}`。
3. **开篇总述**（daily.Intro 非空才画）：一段 `<p>`，16px、行高 1.8。
4. **四象分组**，顺序 **火→土→风→水**，组结构：
   - 组标题：居中粉底胶囊（`background:#FDEEF0; color:#E86A6A; border-radius:999px; padding:8px 28px; font-size:18px; font-weight:bold;`）文案「火象星座」等；
   - `<table width="100%">` 一行三 `<td width="33%">`，每格：
     - 居中小标：`白羊座（3月21日-4月19日）`（日期范围复用 `constellationMonth`），14px、色 `#E86A6A`；
     - `<img src="{imgURL}" style="width:100%; border-radius:8px;">`。
   - 四象映射（硬表 `elementGroups`）：
     火=白羊/狮子/射手，土=金牛/处女/摩羯，风=双子/天秤/水瓶，水=巨蟹/天蝎/双鱼。
   - 缺行处理：该格只画名字不画图？**裁定：缺行整格跳过**（td 不出现，微信表格自然塌缩），与旧版「跳过缺失行」一致。
5. **底部关注区**：带边框块，居中两行：
   `运势早知道，好运跑不了。` / `关注星福柚，每天解锁今日好运~`（15px，色 #666/#E86A6A）。
6. 结尾免责小字沿用旧版（`以上运势仅供参考…`）。

**图片 src 规则**：
- `BuildHTML`（admin 预览）：`/api/public/starfuyu/image/fortune/v3/{date}/{c}.jpg` 相对路径
  （前端 dev 代理 /api 下 srcdoc 可显；不传 host，签名不变）。
- `UploadToMP`：微信 CDN URL（见 §5），走内部 `buildImageArticleHTML(daily, rows, imgURL func(c string) string)`——
  纯函数，两个调用方只差 imgURL 来源。
- 预览时 Ensure 不强制：`BuildHTML` 直接拼路径（图没生成则预览裂图，UploadToMP 才 Ensure）。

### 4.2 取数

`BuildHTML`/`UploadToMP`：`GetByDate` + `DailyRepository.GetByDate`；daily 不存在/空字段 → 对应块降级跳过（§4.1 已定）。

## 5. 微信正文图片上传（wechat_mp.go）

`MPClient` 新增方法：

```go
// UploadContentImage 正文图片上传（/cgi-bin/media/uploadimg），返回可嵌正文的微信 CDN URL。
UploadContentImage(ctx context.Context, data []byte) (url string, err error)
```

- multipart `media` 字段，响应 JSON 取 `url` 字段；错误包 30507（合同不变）。
- `UploadToMP` 流程改为：
  1. 密钥短路 30507（不变）；
  2. Ensure 12 星座 v3 图（`GenerateAll` 或逐个 Ensure——**取逐个 Ensure 白羊以外也走同一循环**：直接 `GenerateAll`，失败即返回，保证 12 张齐）；
  3. 封面：读白羊座字节 → `UploadImageMaterial`（不变）；
  4. 逐星座 `driver.Open` → `UploadContentImage` → `map[constellation]url`（单张失败 → 30502/30507 中止，不发半成品草稿）；
  5. `buildImageArticleHTML(...)` → `AddDraft`（title/digest 不变）。

## 6. 测试（TDD 逐条）

| 层 | 测试 |
|---|---|
| migration | 既有迁移测试框架内新表可建可删（跟随仓库既有 migration 测试方式） |
| model/repo | DailyHoroscope Upsert 冲突更新、GetByDate miss → (nil,false,nil) |
| deepseek | ① 对象 payload → daily+12 行齐；② 纯数组 payload（既有测试不改）→ daily=nil 不失败；③ prompt 含 `daily_astro`/`daily_intro`/对象包裹措辞 |
| fortune service | Fetch 落 rows 后 upsert daily；daily=nil 不炸 |
| article | ① 整版含日期徽章、`重点星象`、四象四个胶囊文案、12 个 img 路径（v3 key）、网格 td width=33%、底部「星福柚」；② daily 缺失 → 无星象/总述块但四象照画；③ 单星座分支仍旧文字版（回归） |
| wechat_mp | uploadimg httptest：成功回 url、errcode 非 0 → 30507、空字节 → 30507 |
| UploadToMP | mock MPClient：1 次 UploadImageMaterial + 12 次 UploadContentImage + AddDraft；content 含 12 个 mock url；中途上传失败 → 不调 AddDraft |

## 7. 实施顺序

1. 迁移 + model + repository + 测试。
2. DeepSeek prompt/解析/签名 + 测试（含数组回退）。
3. FortuneService.Fetch 接 daily + 测试。
4. MPClient.UploadContentImage + 测试。
5. article.go 图片版纯函数 + BuildHTML 切换 + 测试。
6. UploadToMP 重排 + 测试。
7. 全量回归 + admin 预览手工验一次 → 提交。

## 8. 风险与裁定记录

- **微信正文只认自家 CDN**：预览相对路径仅限 admin，草稿必须走 uploadimg——已按两条路径分离。
- **daily upsert 不与 rows 同事务**：日表是文章增强数据，失败降级渲染（§3.3）。
- **数组回退**：模型偶发返回旧格式，12 行不丢、daily 空 → 文章少两块，不失败。
- 旧文字版单星座分支不动，避免影响既有 admin 单篇预览。
