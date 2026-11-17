# starfuyu 图片版微信软文 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 公众号整版软文改为图片为主版式（日期徽章+重点星象+总述+四象分组三张一行的 v3 海报+星福柚关注区），当日星象/总述由 DeepSeek 随 12 星座一并生成、存新表 `starfuyu_daily`。

**Architecture:** DeepSeek 响应升级为对象包裹（数组回退容错）→ 新表落日级数据 → `article.go` 新纯函数 `buildImageArticleHTML(daily, rows, imgURL)` 供预览（相对公开路径）与上传（微信 CDN URL）两路复用 → `MPClient.UploadContentImage`（media/uploadimg）供正文图上传。

**Tech Stack:** Go + gorm 迁移 + viper、fogleman/gg 无关（本文不动生图）、微信草稿箱 API。

**Spec:** `docs/superpowers/specs/2026-11-16-starfuyu-image-article-design.md`

## Global Constraints

- 错误码合同：DeepSeek 失败 30501、微信失败 30507（`wrapMpErr`）、参数非法 10001；daily 缺失/空字段**永不失败**，文章降级少画块。
- 微信正文 HTML 只用 inline style + `<table>`（编辑器安全子集）；草稿 content 内图片必须是 uploadimg 返回的微信 CDN URL，预览才允许 `/api/public/...` 相对路径。
- 四象分组钉死：火=白羊/狮子/射手、土=金牛/处女/摩羯、风=双子/天秤/水瓶、水=巨蟹/天蝎/双鱼，顺序 火→土→风→水。
- 直接提交 main；每任务一提交，`feat(starfuyu): …` / `test(starfuyu): …` 中文。
- TDD：每任务先写失败测试再实现；桩签名变更（FetchFortunes/NewArticleService）与消费方适配同任务内完成，保持编译绿。

## Review Focus

1. **模型返回旧纯数组格式** → 12 行不丢、daily=nil、不报错 —— Task 2 测试 `TestFetchFortunes_ArrayFallbackNoDaily`。
2. **预览相对路径混进草稿** → UploadToMP 的 content 必须含 12 个 uploadimg URL 且不含 `/api/public/` —— Task 6 测试 `TestUploadToMP_ContentUsesCDNUrls`。
3. **daily 缺失降级** → 文章无「重点星象/总述」块，但日期徽章、四象胶囊、12 图齐 —— Task 5 测试 `TestBuildHTML_MissingDailyDegrades`。
4. **正文图单张失败仍发草稿** → 第 N 张上传失败时 AddDraft 调用次数=0 —— Task 6 测试 `TestUploadToMP_ContentUploadFailNoDraft`。
5. **四象映射/顺序错误** → 断言四个胶囊文案顺序 + 每组 3 张图的星座顺序 —— Task 5 测试 `TestBuildHTML_ElementGroups`。

---

### Task 1: starfuyu_daily 迁移 + model + repository

**Files:**
- Create: `modules/starfuyu/migrations/20261116000001_starfuyu_daily.up.sql` / `.down.sql`
- Create: `modules/starfuyu/model/daily.go`
- Create: `modules/starfuyu/repository/daily.go`、`modules/starfuyu/repository/daily_test.go`
- Modify: `modules/starfuyu/module.go`（fx.Provide 加 `repository.NewDailyRepository`，紧挨 `NewFortuneRepository`）
- Modify: 测试 `newTestDB` 的 AutoMigrate 列表（`repository/repository_test.go`、`service/fortune_test.go`）加 `&model.DailyHoroscope{}`

**Interfaces:**
- Consumes: 无。
- Produces:
  - `model.DailyHoroscope{ID, TenantID, Date, Astro, Intro, CreatedAt, UpdatedAt}`，`TableName() = "starfuyu_daily"`。
  - `repository.DailyRepository`：`GetByDate(ctx context.Context, date string) (*model.DailyHoroscope, bool, error)`（miss → nil,false,nil）、`Upsert(ctx context.Context, row *model.DailyHoroscope) error`（(tenant_id,date) 冲突更新 astro/intro）。
  - `repository.NewDailyRepository(db *gorm.DB) DailyRepository`。

- [ ] **Step 1: 写失败测试 `repository/daily_test.go`**

```go
// TestDailyRepo_UpsertAndGet：首插 → GetByDate 命中；同 date 改 astro/intro 再 Upsert →
// 覆盖更新、ID 不变；GetByDate 未知 date → (nil, false, nil)。
func TestDailyRepo_UpsertAndGet(t *testing.T) { /* newTestDB + AutoMigrate DailyHoroscope */ }
```

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/repository/ -run TestDailyRepo -v`
Expected: FAIL（编译错：model/repository 未定义）

- [ ] **Step 3: 实现迁移 + model + repository**

- up.sql：`CREATE TABLE starfuyu_daily (id BIGINT AUTO_INCREMENT PRIMARY KEY, tenant_id BIGINT NOT NULL DEFAULT 0, date VARCHAR(10) NOT NULL, astro VARCHAR(64) NOT NULL DEFAULT '', intro TEXT, created_at DATETIME NULL, updated_at DATETIME NULL, UNIQUE KEY uk_daily_tenant_date (tenant_id, date));`
- down.sql：`DROP TABLE starfuyu_daily;`
- `model/daily.go`：字段 gorm tag 对齐上表（与 fortune 同风格，注释中文）。
- `repository/daily.go`：照 `fortune.go` 的 repo 风格；Upsert 用 gorm `clause.OnConflict{Columns: [tenant_id,date], DoUpdates: astro/intro}`。
- `module.go`：`fx.Provide` 列表加 `repository.NewDailyRepository`。
- 两个 `newTestDB` AutoMigrate 列表加 `&model.DailyHoroscope{}`。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/repository/ -v && go build ./modules/starfuyu/...` → PASS/clean

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): starfuyu_daily 日级星象表（迁移+model+repository）"
```

---

### Task 2: DeepSeek 对象包裹 + daily 解析（含数组回退）

**Files:**
- Modify: `modules/starfuyu/service/deepseek.go`（prompt、callAPI 解析、`FortuneFetcher` 接口签名）
- Modify: `modules/starfuyu/service/fortune.go:15-16`（接口注释）与 `Fetch` 适配新签名（daily 暂 `_,` 丢弃——Task 3 接）
- Modify: 桩两处——`service/fortune_test.go` stubFetcher、`module_test.go` stubFetcher
- Test: `modules/starfuyu/service/deepseek_test.go`

**Interfaces:**
- Consumes: Task 1 的 `model.DailyHoroscope`（仅构造，不落库）。
- Produces:
  - `FortuneFetcher.FetchFortunes(ctx, date) ([]*model.Fortune, *model.DailyHoroscope, error)`（daily 可 nil）。
  - prompt 输出对象 `{daily_astro, daily_intro, fortunes:[12]}`；解析先对象后数组。

- [ ] **Step 1: 写失败测试**

`deepseek_test.go` 追加：

```go
// TestFetchFortunes_ObjectWithDaily 对象包裹 payload → 12 行 + daily 两字段齐。
// payload 手工构造：{"daily_astro":"月水星，月土六合","daily_intro":"…","fortunes":[…12行]}
func TestFetchFortunes_ObjectWithDaily(t *testing.T) { /* 断言 rows=12、daily.Astro/Intro 非空 */ }

// TestFetchFortunes_ArrayFallbackNoDaily 纯数组 payload（既有格式）→ 12 行、daily==nil、err==nil。
func TestFetchFortunes_ArrayFallbackNoDaily(t *testing.T) { /* 复用 twelveRows marshal */ }
```

并在既有 `TestFetchFortunes_PromptAndShareFields` 加 prompt 断言：
`strings.Contains(prompt, "daily_astro")`、`strings.Contains(prompt, "对象包裹")`。

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestFetchFortunes_' -v`
Expected: 新测试 FAIL/编译错（签名未改），旧测试红在签名编译——属预期 RED。

- [ ] **Step 3: 实现**

- `deepseek.go`：接口签名改三返回值；prompt「输出规范 1」改对象包裹措辞 + JSON 结构示例加两字段 + 生成要求加「daily_astro/daily_intro 与星座无关，全篇同一条」。
- `callAPI` 返回 `([]*model.Fortune, *model.DailyHoroscope, error)`：
  ```go
  var wrap struct {
      DailyAstro string           `json:"daily_astro"`
      DailyIntro string           `json:"daily_intro"`
      Fortunes   []*model.Fortune `json:"fortunes"`
  }
  // 先 json.Unmarshal(content, &wrap) 且 len(wrap.Fortunes)>0 → 用之；
  // 否则回退 Unmarshal 到 []*model.Fortune（数组）→ daily=nil；
  // 两路都失败 → 现有解析错误处理。
  ```
- `FetchFortunes` 外层循环与日志适配；`fortune.go` `Fetch` 改 `rows, _, err := …`（Task 3 再接 daily）。
- 两个 stubFetcher 签名同步。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/... -count=1` → 全 PASS（含旧数组测试不改仍绿）

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): DeepSeek 对象包裹生成当日星象/总述（数组回退容错）"
```

---

### Task 3: FortuneService.Fetch 落 daily

**Files:**
- Modify: `modules/starfuyu/service/fortune.go`（NewFortuneService 注入 DailyRepository、Fetch 落库）
- Modify: `modules/starfuyu/module.go`（fx 装配若需显式传参——看 NewFortuneService 调用点）
- Test: `modules/starfuyu/service/fortune_test.go`

**Interfaces:**
- Consumes: Task 1 `DailyRepository.Upsert`；Task 2 `FetchFortunes` 第二返回值。
- Produces: `NewFortuneService(fortunes repository.FortuneRepository, daily repository.DailyRepository, fetcher FortuneFetcher)`（参数序变化，调用点全量适配）。

- [ ] **Step 1: 写失败测试**

```go
// TestFetch_UpsertsDaily rows 落库后 daily 同步 Upsert：stub 返回 rows+daily，
// 断言 DailyRepository 收到且 GetByDate 可读回。
func TestFetch_UpsertsDaily(t *testing.T) {}

// TestFetch_DailyNilSkips stub 返回 daily=nil → Fetch 正常、无 daily 行。
func TestFetch_DailyNilSkips(t *testing.T) {}
```

（stub 结构：fortune_test 的 stubFetcher 加 `daily *model.DailyHoroscope` 字段返回。）

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestFetch_UpsertsDaily|TestFetch_DailyNilSkips' -v`
Expected: FAIL（未落 daily）

- [ ] **Step 3: 实现**

- `Fetch` 中 rows 校验通过、`UpsertBatch` 成功后：
  ```go
  if daily != nil {
      daily.Date = date; daily.TenantID = 0
      if err := s.daily.Upsert(ctx, daily); err != nil {
          log.Printf("[Fortune] 日级星象落库失败 date=%s err=%v", date, err) // 增强数据：记日志不失败（spec §3.3 裁定）
      }
  }
  ```
- `NewFortuneService` 加 daily 参数；模块装配与测试构造点全量适配；两个 `newTestDB` 已含 AutoMigrate（Task 1 做过）。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/... -count=1` → 全 PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): 抓取落库同步写当日星象（失败降级不阻断）"
```

---

### Task 4: MPClient.UploadContentImage（media/uploadimg）

**Files:**
- Modify: `modules/starfuyu/service/wechat_mp.go`（接口 + 实现）
- Test: `modules/starfuyu/service/wechat_mp_test.go`

**Interfaces:**
- Consumes: 既有 token/`checkErrCode`/mpFail 基建。
- Produces: `UploadContentImage(ctx context.Context, data []byte) (url string, err error)` —— POST `{base}/cgi-bin/media/uploadimg?access_token=…`，multipart `media` 字段，响应取 `url`；errcode≠0/空字节 → 30507。

- [ ] **Step 1: 写失败测试**

```go
// TestWeChatMPClient_UploadContentImage_Success 200 {"url":"https://mmbiz.qpic.cn/x"} → 回 url。
func TestWeChatMPClient_UploadContentImage_Success(t *testing.T) {}
// TestWeChatMPClient_UploadContentImage_ErrCode30507 errcode≠0 → 30507；空字节 → 30507。
func TestWeChatMPClient_UploadContentImage_ErrCode30507(t *testing.T) {}
```

（handler 模式照既有 `newMPForTest`。）

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run UploadContentImage -v`
Expected: 编译错（方法未定义）

- [ ] **Step 3: 实现**

照 `UploadImageMaterial` 骨架：路径换 `media/uploadimg`、解析响应 JSON `{url}`；`MPClient` 接口加方法注释（合同：失败一律 30507）。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/service/ -run 'UploadContentImage|UploadImageMaterial' -v` → PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu/service/wechat_mp.go modules/starfuyu/service/wechat_mp_test.go
git commit -m "feat(starfuyu): 正文图片上传接口 media/uploadimg（回微信 CDN URL）"
```

---

### Task 5: 图片版 HTML 纯函数 + BuildHTML 切换

**Files:**
- Modify: `modules/starfuyu/service/article.go`（`elementGroups`、`buildImageArticleHTML`、`NewArticleService` 加 daily、`BuildHTML` 整版分支）
- Modify: `modules/starfuyu/module.go`（NewArticleService 装配）
- Modify: `modules/starfuyu/service/article_test.go`（fixture 注入 stub daily、旧整版测试改写、新增 3 测试）
- Test: 同上

**Interfaces:**
- Consumes: Task 1 `DailyRepository`（GetByDate）；`constellationMonth`、`formatDate` 既有工具。
- Produces:
  - `var elementGroups = []struct{ Name string; Members []string }{ {"火象星座", […狮子射手…]}, … }`（顺序火土风水）。
  - `buildImageArticleHTML(daily *model.DailyHoroscope, rows []*model.Fortune, date string, imgURL func(constellation string) string) string` —— 纯函数。
  - `NewArticleService(fortunes, images, mp, driver, cfg, daily repository.DailyRepository)`。
  - `BuildHTML` 整版：取 rows+daily → `buildImageArticleHTML(..., imgURL=相对公开路径)`；单星座分支不动。

- [ ] **Step 1: 写失败测试（3 个 + 改写 1 个）**

```go
// TestBuildHTML_ElementGroups 四胶囊文案顺序 火土风水；每组 3 个 img，src 含
// fortune/v3/{date}/{星座}.jpg；每格含星座名与 constellationMonth 文案。
func TestBuildHTML_ElementGroups(t *testing.T) {}

// TestBuildHTML_MissingDailyDegrades daily 仓库 miss → HTML 无「重点星象」「开篇」字样，
// 但日期徽章（/）、四胶囊、12 img 齐。
func TestBuildHTML_MissingDailyDegrades(t *testing.T) {}

// TestBuildHTML_SingleConstellation 回归：单星座分支仍是旧文字版（含「整体运势」）。
func TestBuildHTML_SingleConstellation(t *testing.T) —— 已有，确认不改仍绿。
```

- 改写既有 `TestBuildHTML_ContainsAllConstellations` → 断言新版式（12 星座名 + 12 img + 底部「星福柚」；旧断言「stars 字样」删除——版式不再画分数）。
- fixture：`stubArticleFortunes` 旁加 `stubArticleDaily{row *model.DailyHoroscope}` 实现 `GetByDate`；`newArticleFixture` 传入（默认带 astro/intro）。

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestBuildHTML' -v`
Expected: FAIL（函数 undefined / 断言不过）

- [ ] **Step 3: 实现**

- `elementGroups` 硬表（四象成员与 Global Constraints 一致）。
- `buildImageArticleHTML` 按 spec §4.1 版式逐块：日期徽章（`2006/01/02` 斜杠）→ astro 非空画「重点星象：」加粗行 → intro 非空画段 → 四组（粉底胶囊 + `<table><tr><td width="33%">`×3：名字小标 + `<img style="width:100%;border-radius:8px;">`，缺行整格跳过）→ 底部边框块两行（运势早知道…/关注星福柚…）→ 免责小字。全部 inline style。
- `BuildHTML` 整版分支：`GetByDate` + `daily.GetByDate`（miss → nil）→ imgURL func 拼 `/api/public/starfuyu/image/fortune/v3/{date}/{c}.jpg`（用 `fortuneImageKey` 生成 key 保证同源）。
- `articleService` 加 `daily repository.DailyRepository` 字段与构造参数；module.go 与 fixture 适配。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/... -count=1` → 全 PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): 整版软文改图片版四象分组布局（预览走公开读路径）"
```

---

### Task 6: UploadToMP 重排（Ensure 12 张 + 正文图上传 + 草稿）

**Files:**
- Modify: `modules/starfuyu/service/article.go`（UploadToMP 流程）
- Modify: `modules/starfuyu/service/article_test.go`（fakeWeChat 加 uploadimg handler、既有成功测试改写、新增 2 测试）

**Interfaces:**
- Consumes: Task 4 `UploadContentImage`；Task 5 `buildImageArticleHTML`；既有 `images.GenerateAll`、`UploadImageMaterial`、`AddDraft`。
- Produces: UploadToMP 新流程（spec §5 步骤 1-5）；单张正文图失败 → 错误返回且 `AddDraft` 未被调用。

- [ ] **Step 1: 写失败测试**

- fakeWeChat mux 加：
  ```go
  mux.HandleFunc("/cgi-bin/media/uploadimg", …) // 返回 {"url":"https://mmbiz.qpic.cn/img{i}"}
  ```
  handler 内自增计数器供断言。
- 改写 `TestUploadToMP_SuccessReturnsMediaIDs`：
  - 断言 uploadimg 调用 =12、add_material =1；
  - 捕获 draft/add 请求体：`TestUploadToMP_ContentUsesCDNUrls`——content 含 12 个 `https://mmbiz.qpic.cn/`、**不含** `/api/public/`、含四个象限胶囊文案。
- 新增 `TestUploadToMP_ContentUploadFailNoDraft`：让 uploadimg 第 3 次返回 errcode≠0 → 断言 err=30507、draft/add 调用 =0。

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestUploadToMP' -v`
Expected: FAIL（流程未改/计数不符）

- [ ] **Step 3: 实现 UploadToMP**

```go
// 1 密钥短路 30507（不动）
// 2 if _, err := s.images.GenerateAll(ctx, date); err != nil { return nil, err }
// 3 封面：Ensure 白羊 → Open → UploadImageMaterial（照旧）
// 4 循环 ConstellationList：Ensure → Open → UploadContentImage → map[c]url；
//    任一失败 → 直接 return（不调 AddDraft）
// 5 buildImageArticleHTML(daily, rows, date, imgURL=map 取) → AddDraft（title/digest 不变）
```
daily 取自 `s.daily.GetByDate`（miss → nil，降级）。rows 取 `s.fortunes.GetByDate`。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/... -count=1` → 全 PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): 草稿箱上传改图片版（12 张正文图逐传，失败不发半成品）"
```

---

### Task 7: 全量回归 + 预览验收门

**Files:** 无新改动（不合格才回 Task 5/6 调常量/文案）

- [ ] **Step 1: 全量回归**

Run: `go test -count=1 ./modules/starfuyu/... ./pkg/config/...` → 全 PASS
Run: `go vet ./modules/starfuyu/...` → clean

- [ ] **Step 2: 预览手工验收（门）**

起本地服务或用测试出样：写一次性小测试/`go run` 把 `BuildHTML("2026-11-16","")` 输出落 `/tmp/starfuyu-article.html`，浏览器打开核对：
1. 日期徽章、重点星象、总述段在顶部；
2. 四个粉底胶囊顺序火土风水，每组一行三图、图下名字+日期范围；
3. 底部「关注星福柚」块；
4. 手机宽度目测不破版（table 塌缩正常）。
**给用户路径让其打开确认**；不合格回 Task 5 调版式。

- [ ] **Step 3: 收尾提交（如 Step 2 有微调）**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): 图片版软文版式微调"
```
无微调则跳过提交，直接 task-done。
