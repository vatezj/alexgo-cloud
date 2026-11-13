# starfuyu 运势分享卡·图像文字合成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** DeepSeek 每日抓取新增分享卡字段入库，`ImageService` 按新模板底图（1728×2397）重写渲染，替换旧版运势图，供微信软文/公众号封面使用。

**Architecture:** 沿用既有流水线（cron 抓取 → fortune 表 → gg 渲染 → storage 落盘 → 文章封面），只动三处：`Fortune` 模型+迁移加 10 列、DeepSeek prompt 扩字段、`render` 整体重写为新模板布局（布局坐标集中在 `shareCardLayout` 常量表）。图片 key 升版 `fortune/v2/...` 解决换模板的幂等缓存问题。

**Tech Stack:** Go、gg (fogleman/gg)、gorm + golang-migrate SQL 迁移、httptest、sqlite 内存库测试。

**Spec:** `docs/superpowers/specs/2026-11-13-starfuyu-fortune-card-design.md`（计划从 spec 推导，执行者需两份同读）

## Global Constraints

- 图片 key 恒为 `fortune/v2/{date}/{constellation}.jpg`（date 钉 YYYY-MM-DD、星座钉 12 枚举，rel 由 service 生成）。
- 模板（`assets/bgimage/bg.png`，1728×2397）上没印的元素一律不画：无星座名+日期标题、无幸运色/数字、无 ★ 星级（spec §6.4 用户裁定）。
- 字段为空 → 对应槽位留白，不画 `-`/`—`、不报错（存量旧数据出图不崩）。
- 字体缺失/加载失败 → 30502，不回退 ASCII（决定 19）。错误码只有 30501/30502/30507，不新增。
- 内容表 `tenant_id` 恒 0；DeepSeek 解析要求恰好 12 行、nil 行报错（既有合同不变）。
- DeepSeek 漏字段 = 落零值不判失败；`match_constellation` 非 12 枚举 → 存空串。
- 分数绘制前 clamp 0-100；输出 JPEG q90。
- prompt 字数上限为初值（简述≤30、建议/避免≤24、三小运≤14、必做 3×≤14 字），出图后视觉迭代可调，测试只断言字段名存在、不断言字数。
- 直接提交 main，TDD 一票一提交；`go test ./modules/starfuyu/...` 全程保持绿。

## Review Focus

（spec 隐含但最容易咬人的五类输入，各自测试钉在所属任务）

1. **DeepSeek 响应漏新字段** → 期望：解析不失败、字段零值、出图留白。钉在 Task 2（`TestFetchFortunes_MissingShareFields`）+ Task 5（`TestRender_EmptyShareFields`）。
2. **文案超长溢出模板槽位** → 期望：按槽宽换行、超行截断加 `…`、绝不画出 rect。钉在 Task 4（`TestWrapByWidth_*`、`TestTruncateLines_*`）。
3. **换模板后旧图被 `Ensure` 幂等跳过**（当天已生成 v1 图）→ 期望：key 升 v2 后重新生成。钉在 Task 3（key 断言）。
4. **空字段槽位画了残留符号** → 期望：槽位区域像素与底图一致（未画任何东西）。钉在 Task 5（`TestRender_EmptyShareFields` 的区域像素对比）。
5. **DeepSeek 给出越界分数**（-5 / 300）→ 期望：clamp 到 0-100，进度条不画穿卡。钉在 Task 4（`TestClampScore`、`TestProgressFillWidth`）。

---

## File Structure

| 文件 | 动作 | 职责 |
|---|---|---|
| `modules/starfuyu/model/fortune.go` | 修改 | +10 字段（含 `DailyTodos []string` JSON 列） |
| `modules/starfuyu/model/model_test.go` | 修改 | `TestDailyTodosRoundtrip` 新增；`TestAutoMigrateColumns` 列名清单 +10 |
| `modules/starfuyu/migrations/20261113000001_starfuyu_fortune_share_fields.up.sql` | 创建 | MySQL ADD COLUMN ×10 |
| `modules/starfuyu/migrations/20261113000001_starfuyu_fortune_share_fields.down.sql` | 创建 | DROP COLUMN ×10 |
| `modules/starfuyu/service/deepseek.go` | 修改 | prompt 扩字段 + 解析后枚举校验 |
| `modules/starfuyu/service/deepseek_test.go` | 创建 | httptest 假 DeepSeek：prompt 断言 / 全字段 / 漏字段 / 非法枚举 |
| `modules/starfuyu/service/fortune_test.go` | 修改 | `twelveRows` 夹具补分享卡字段 |
| `modules/starfuyu/service/image.go` | 修改 | `fortuneImageKey` v2；`render` 重写为新模板渲染（调 share_card.go） |
| `modules/starfuyu/service/share_card.go` | 创建 | `shareCardLayout` 布局常量表 + 纯函数（wrap/truncate/clamp/进度条宽度） |
| `modules/starfuyu/service/share_card_test.go` | 创建 | 纯函数单测 |
| `modules/starfuyu/service/image_test.go` | 修改 | key 断言 v2 + 空字段/满字段渲染测试 + 区域像素对比 |
| `modules/starfuyu/controller/app/app_test.go` | 修改 | stub 与 URL 断言同步 v2 |
| `modules/starfuyu/assets/bgimage/bg.png` | 替换 | `frontend/static/start/bg.png` 复制进来（源文件保留） |
| `docs/modules.md` | 修改 | §13 key 格式与图片描述回填 |

---

### Task 1: Fortune 模型加列 + SQL 迁移

**Files:**
- Modify: `modules/starfuyu/model/fortune.go`（LuckyColor 结构体之后、Fortune 结构体字段区）
- Modify: `modules/starfuyu/model/model_test.go:11-30`（仿 `TestLuckyColorRoundtrip`）、`:126`（列名清单）
- Create: `modules/starfuyu/migrations/20261113000001_starfuyu_fortune_share_fields.up.sql` / `.down.sql`

**Interfaces:**
- Consumes: 无（首任务）
- Produces（后续任务全依赖，字段名与 json tag 钉死）:
  - `Fortune.MatchConstellation string` `json:"match_constellation"` 列 `match_constellation varchar(20)`
  - `Fortune.Advice string` `json:"advice"` 列 `advice text`
  - `Fortune.Avoid string` `json:"avoid"` 列 `avoid text`
  - `Fortune.ReunionTip / SingleTip / JobTip string` `json:"reunion_tip|single_tip|job_tip"` 列 text
  - `Fortune.ReunionScore / SingleScore / JobScore int` `json:"reunion_score|single_score|job_score"` 列 int default 0
  - `Fortune.DailyTodos []string` `json:"daily_todos"` 列 `daily_todos json`，`Value()/Scan()` 与 `LuckyColor` 同构（`[]byte`+`string` 双分支，`nil` 不动已有值）

- [ ] **Step 1: 写失败测试**

`model_test.go` 新增两个测试：

```go
// TestDailyTodosRoundtrip DailyTodos 的 Valuer/Scan JSON 往返 + nil 不动已有值。
func TestDailyTodosRoundtrip(t *testing.T) — 断言:
    in := []string{"喝八杯水", "给前任点赞前先想想", "19点后别做决定"}
    v, err := driver.Valuer(in 的持有者) → 无错，[]byte 含 "喝八杯水"
    Scan([]byte(`["a","b"]`)) → ["a","b"]
    Scan(string(`["x"]`)) → ["x"]
    Scan(nil) → 无错且值不变
```

`TestAutoMigrateColumns`（:126）Fortune 期望列名数组追加：
`"match_constellation","advice","avoid","reunion_tip","single_tip","job_tip","reunion_score","single_score","job_score","daily_todos"`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/model/ -run 'TestDailyTodosRoundtrip|TestAutoMigrateColumns' -v`
Expected: FAIL（编译错：DailyTodos 未定义；列清单不匹配）

- [ ] **Step 3: 实现模型字段**

`model/fortune.go`：Fortune 结构体 `HealthTip` 之后追加 10 个字段（gorm+json tag 见 Interfaces）；文件内新增 `DailyTodos []string` 类型级 `Value()/Scan()`，模式逐行照抄 `LuckyColor`（含 sqlite string 分支注释）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/starfuyu/model/ -v`
Expected: PASS（含既有 `TestLuckyColorRoundtrip` 等全部）

- [ ] **Step 5: 写迁移 SQL**

up.sql（列注释与 model 同构，仿 `20261112000001_starfuyu_tables.up.sql` 风格）:

```sql
ALTER TABLE `starfuyu_fortune`
  ADD COLUMN `match_constellation` varchar(20) NOT NULL DEFAULT '' COMMENT '合拍星座（12星座枚举）' AFTER `constellation`,
  ADD COLUMN `advice` text COMMENT '建议' AFTER `summary`,
  ADD COLUMN `avoid` text COMMENT '避免' AFTER `advice`,
  ADD COLUMN `reunion_tip` text COMMENT '复合运' AFTER `avoid`,
  ADD COLUMN `single_tip` text COMMENT '脱单运' AFTER `reunion_tip`,
  ADD COLUMN `job_tip` text COMMENT '求职运' AFTER `single_tip`,
  ADD COLUMN `reunion_score` int NOT NULL DEFAULT 0 COMMENT '复合运分数0-100' AFTER `job_tip`,
  ADD COLUMN `single_score` int NOT NULL DEFAULT 0 COMMENT '脱单运分数0-100' AFTER `reunion_score`,
  ADD COLUMN `job_score` int NOT NULL DEFAULT 0 COMMENT '求职运分数0-100' AFTER `single_score`,
  ADD COLUMN `daily_todos` json COMMENT '今日必做 3条 JSON数组' AFTER `job_score`;
```

down.sql: 对应 10 个 `DROP COLUMN`（一条 ALTER 多个 drop，与 up 对称）。

- [ ] **Step 6: 全模型测试 + 提交**

Run: `go test ./modules/starfuyu/model/ ./modules/starfuyu/repository/ -v` → PASS
（repository_test 走 AutoMigrate，加列后应仍绿；若 sqlite 对 `type:json` 报错，照 LuckyColor 的 tag 写法对齐。）

迁移 SQL 正确性说明：仓库无迁移测试 harness（golang-migrate 仅 mysql 生产路径）——up/down 以「列名/类型与 model tag 同构 + up/down 对称」自查为准；若本机 `mysql` 客户端可用，可手动连库执行 up/down 一遍作可选验证，不可用则跳过（spec §7 已记此取舍）。

```bash
git add modules/starfuyu/model modules/starfuyu/migrations
git commit -m "feat(starfuyu): fortune 表加分享卡 10 列（模型+迁移）"
```

---

### Task 2: DeepSeek prompt 扩字段 + 解析容错

**Files:**
- Modify: `modules/starfuyu/service/deepseek.go:128-234`（`callAPI` 的 prompt 与解析收尾）
- Create: `modules/starfuyu/service/deepseek_test.go`
- Modify: `modules/starfuyu/service/fortune_test.go:45-60`（`twelveRows` 夹具）

**Interfaces:**
- Consumes: Task 1 的 `Fortune` 新字段；既有 `IsValidConstellation(c string) bool`（`service/constellation.go`）
- Produces: `FortuneFetcher.FetchFortunes` 返回的行带新字段（漏字段=零值）；`twelveRows(date, mark)` 夹具填充分享卡字段（mark 可区分），Task 5 渲染测试复用

- [ ] **Step 1: 写失败测试（三个）**

`deepseek_test.go`，用 `httptest.NewServer` 假 DeepSeek（cfg: `APIKey="k"`、`BaseURL=srv.URL`，`NewDeepSeekFetcher(cfg)`）：

```go
func TestFetchFortunes_PromptAndShareFields(t *testing.T):
    handler 捕获请求 body → 断言 Messages[0].Content 含:
        "match_constellation" / "advice" / "avoid" / "reunion_tip" /
        "single_tip" / "job_tip" / "reunion_score" / "daily_todos"
    响应 content = 十二行完整 JSON（marshal twelveRows(date,"深")）→
        rows 有 12 行；任取一行断言 Advice/DailyTodos(match_constellation 等) 已带值
        DailyTodos 恰 3 条

func TestFetchFortunes_MissingShareFieldsTolerated(t *testing.T):
    content = twelveRows marshal 成 []map 后 delete 上述全部新 key →
        err == nil，len==12，行字段为零值（Advice=="" 、DailyTodos==nil、ReunionScore==0）

func TestFetchFortunes_InvalidMatchConstellationToEmpty(t *testing.T):
    content 中一行 "match_constellation": "麒麟座" →
        err == nil，该行 MatchConstellation == ""，其余行不受影响
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestFetchFortunes' -v`
Expected: FAIL（prompt 断言缺字段名；非法枚举未清空）

- [ ] **Step 3: 实现**

a) `callAPI` prompt 的「JSON对象结构」示例追加 9 个字段（值为占位说明，字数按 Global Constraints）+ `daily_todos` 恰 3 条示例；「生成要求」追加：`match_constellation 不得等于本星座自身`、`advice/avoid 每日不同`、`summary ≤30字`。
b) 既有收尾循环（钉 Date/TenantID 处）追加：`if !IsValidConstellation(row.MatchConstellation) { row.MatchConstellation = "" }`。
c) `fortune_test.go` 的 `twelveRows` 追加分享卡字段填充（用 mark 拼值，如 `Advice: mark+"建议"`，DailyTodos 3 条）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/starfuyu/service/ -run 'TestFetchFortunes' -v` → PASS

- [ ] **Step 5: 全包回归 + 提交**

Run: `go test ./modules/starfuyu/... ` → 全绿（`twelveRows` 变更波及 image/article 测试，只加字段应无破坏）

```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): DeepSeek prompt 扩分享卡字段 + 枚举校验/漏字段容错"
```

---

### Task 3: 图片 key 升版 fortune/v2

**Files:**
- Modify: `modules/starfuyu/service/image.go:51-59`（`fortuneImageKey`）
- Modify: `modules/starfuyu/service/image_test.go:78`（key 断言）
- Modify: `modules/starfuyu/controller/app/app_test.go:140,362`（stub 返回值与 URL 断言）

**Interfaces:**
- Consumes: 无（独立小任务）
- Produces: `fortuneImageKey(date, constellation) → "fortune/v2/{date}/{constellation}.jpg"`（含既有参数校验：date 格式、12 星座枚举、非法 → 30503 param invalid）

- [ ] **Step 1: 改断言造失败**

`image_test.go:78` want 改为 `"fortune/v2/2026-11-12/白羊座.jpg"`；
`app_test.go:140` stub 返回改 `"fortune/v2/" + date + "/" + constellation + ".jpg"`，`:362` want 同步改。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/service/ -run TestEnsure_WritesJPEG -v && go test ./modules/starfuyu/controller/app/ -run TestFortuneImage -v`
Expected: FAIL（key 不含 v2）

- [ ] **Step 3: 实现**

`fortuneImageKey` 的 `fmt.Sprintf` 改 `"fortune/v2/%s/%s.jpg"`；函数注释同步 v2 语义（v1=旧布局，v2=分享卡模板，换模板升版解决幂等跳过）。

- [ ] **Step 4: 跑测试确认通过**

Run（两条如上）→ PASS；再 `go test ./modules/starfuyu/...` 全绿。

- [ ] **Step 5: 提交**

```bash
git add modules/starfuyu/service/image.go modules/starfuyu/service/image_test.go modules/starfuyu/controller/app/app_test.go
git commit -m "feat(starfuyu): 运势图 key 升版 fortune/v2（换模板防旧图幂等跳过）"
```

---

### Task 4: 底图替换 + 渲染纯函数（wrap/truncate/clamp/进度条）

**Files:**
- Replace: `modules/starfuyu/assets/bgimage/bg.png` ← `cp frontend/static/start/bg.png`（源文件保留并随本任务提交）
- Create: `modules/starfuyu/service/share_card.go` / `share_card_test.go`

**Interfaces:**
- Consumes: 无
- Produces（Task 5 render 依赖，签名钉死）:
  - `func clampScore(s int) int` → 截到 [0,100]
  - `func wrapByWidth(text string, maxW float64, measure func(string) float64) []string` → 按测宽贪心断行（`measure` 收整串返回像素宽；遇 `\n` 强制断行；单字超宽也独占一行不丢字）
  - `func truncateLines(lines []string, maxLines int) []string` → 超 `maxLines` 截最后一条并追加 `…`（maxLines<=0 原样）
  - `func progressFillWidth(barW float64, score int) float64` → `barW * clampScore(score)/100`，非负

- [ ] **Step 1: 写失败测试**

`share_card_test.go`:

```go
func TestClampScore: -5→0, 0→0, 66→66, 100→100, 300→100
func TestWrapByWidth:
    measure = 按 rune 数 ×10 的假函数
    "一二三四五" maxW=35 → ["一二三","四五"]（3×10=30 ≤35 <4×10）
    "ab\ncd" → ["ab","cd"]（\n 强制断行）
    单字超宽 "中" maxW=5 → ["中"]（不丢字）
func TestTruncateLines:
    4 行 maxLines 3 → 3 行，末行以 … 结尾
    2 行 maxLines 3 → 原样
func TestProgressFillWidth:
    (300, 0)→0, (300, 50)→150, (300, 300)→300, (300, -5)→0
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestClampScore|TestWrapByWidth|TestTruncateLines|TestProgressFillWidth' -v`
Expected: FAIL（编译错：函数未定义）

- [ ] **Step 3: 实现纯函数**

`share_card.go` 实现四个函数（包注释说明：分享卡渲染支撑——布局常量表在 Task 5 加入本文件）。`wrapByWidth` 贪心累加 rune 宽度，超 `maxW` 断行；`truncateLines` 截断行加 `…`。

- [ ] **Step 4: 跑测试确认通过 + 换底图**

Run: 上述四测试 → PASS
`cp frontend/static/start/bg.png modules/starfuyu/assets/bgimage/bg.png`
Run: `go test ./modules/starfuyu/service/ -run TestEnsure -v` → PASS（此时 render 还是旧代码，只验证新底图不破坏出图/字体缺测）

- [ ] **Step 5: 提交**

```bash
git add frontend/static/start/bg.png modules/starfuyu/assets/bgimage/bg.png modules/starfuyu/service/share_card.go modules/starfuyu/service/share_card_test.go
git commit -m "feat(starfuyu): 分享卡底图替换 + 渲染纯函数（wrap/truncate/clamp/进度条）"
```

---

### Task 5: render 重写为分享卡布局（核心任务）

**Files:**
- Modify: `modules/starfuyu/service/image.go:140-258`（`render` 与 `drawFortuneLine` 删除/重写）
- Modify: `modules/starfuyu/service/share_card.go`（追加 `shareCardLayout` 布局常量表）
- Modify: `modules/starfuyu/service/image_test.go`（新增两个渲染测试）

**Interfaces:**
- Consumes: Task 1 字段、Task 3 v2 key、Task 4 四个纯函数、`wrapText`/`formatDate` 等既有 helper（无用的删）
- Produces: `render(f *model.Fortune) ([]byte, error)`（签名不变，输出新模板 JPEG）；`shareCardLayout` 各槽位 rect 常量

**布局数据结构（钉死，坐标值由 Step 3 量图填入）：**

```go
type slot struct{ x, y, w, h float64 } // 左上角+宽高，画布 1728×2397
var shareCardLayout = struct {
    Summary, Score, Match, Advice, Avoid slot   // 上卡五槽（标签已印底图）
    LaneWealth, LaneCareer, LaneLove, LaneHealth slot // 四运程条文字区
    Reunion, Single, Job slot                   // 三小卡文案区
    BarReunion, BarSingle, BarJob struct{ x, y, w, h float64 } // 进度条槽（画填充段）
    Todos [3]slot                               // 今日必做三行文字区
}{...}
```

- [ ] **Step 1: 写失败测试**

`image_test.go` 新增（复用 `newImageFixture`/`seed`）:

```go
// TestRender_EmptyShareFields 空值槽位：留白不崩，且槽位区域像素≈底图（未画残留符号）。
func TestRender_EmptyShareFields(t *testing.T):
    rows := twelveRows(date,"空"); 逐行清空新字段（Advice/Avoid/两tip组/三分数/MatchConstellation/DailyTodos=nil; Summary 也置空）
    Ensure → err nil、JPEG 出图
    解码 JPEG，与 ../assets/bgimage/bg.png 原图比较 shareCardLayout.Summary 槽区域:
        平均像素差 < 6（JPEG q90 噪声容差）→ 留白确认
    再取一个有值字段（Summary 非空的对照行）同区域: 存在像素差 > 20 → 有值必画确认

// TestRender_FullShareFields 满值出图不崩 + 关键槽区域确有绘制。
func TestRender_FullShareFields(t *testing.T):
    十二行满值（twelveRows 默认）→ GenerateAll ok==12
    白羊座图解码后: Advice 槽区域存在像素差 > 20；进度条 BarReunion 中点颜色与底图不同
```

像素比较辅助：`regionMeanDiff(img, bg image.Image, r slot) float64` 与 `regionMaxDiff`（测试文件内私有 helper，读两图同 rect 取逐通道差）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestRender_' -v`
Expected: FAIL with compile error `undefined: shareCardLayout`（布局表在 Step 3 才建——确定性红灯；实现后红转绿靠像素断言本身）

- [ ] **Step 3: 量图 + 实现**

a) **量图**：用 Read 工具打开 `modules/starfuyu/assets/bgimage/bg.png`，对照视觉量出各槽坐标，填 `shareCardLayout`（上卡标签「今日简述:」等的冒号之后为槽起点；四条运程条在胶囊内侧；三小卡在卡题下方文字区与底部进度条；今日必做三行对齐已印勾选框右侧）。先估后调，坐标只存在于该常量表。
b) **重写 `render`**：加载底图 → 逐槽绘制（值来源与规则钉死于 spec §4.2）：
   - Summary/Advice/Avoid/三 tip：`wrapByWidth`(measure 用 `dc.MeasureString`) → `truncateLines`(Summary 2 行、Advice/Avoid 1 行、tip 2 行) → 逐行 `DrawString`，rect 内顶对齐；空值 continue。
   - Score 槽：`fmt.Sprintf("%d分", clampScore(...))`。
   - 三小卡进度条：先铺底图浅色段同色矩形，再画 `progressFillWidth` 宽的深色圆角段（`DrawRoundedRectangle`），颜色 橙 #F6A93B（脱单）/ 粉 #F08C8C（复合、求职）。
   - 今日必做：`DailyTodos` 最多 3 条逐行画；不足 3 条留空（底图框保留，spec §6.2）。
   - 字体：走既有 `findFontPath` + `dc.LoadFontFace`；字号/文本色在常量表旁定义（初值：上卡值 44、运程条 40、小卡 34、必做 40；色 #5B4A4A 深棕，与底图印字同系）。
   - 删除不再使用的 `drawFortuneLine`、`scoreToStars`（若 article 未引用——先 grep 确认 `scoreToStars` 在 article.go 的引用，被引用则保留）。
c) JPEG q90 编码不变。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./modules/starfuyu/service/ -v` → 全绿（含 Task 4 纯函数、既有 Ensure 系列、两个新渲染测试）
若像素断言不过 → 调实现（确认画在 rect 内），不得放宽容差掩盖留白失败。

- [ ] **Step 5: 出样图视觉验收（迭代步）**

写一次性测试 `TestRenderSampleForVisual`（`t.Skip` 以外方式不受 CI 影响：用 `testing.Short()` 跳过，`go test -short` 默认跳、本地 `-run` 显式跑）：渲染白羊座（满值）落 `/tmp/starfuyu-card-sample.jpg`，`t.Logf` 打路径。
Run: `go test ./modules/starfuyu/service/ -run TestRenderSampleForVisual -v`
用 Read 工具打开样图，对照 `bg.png` 检查：五槽/四条/三卡/清单文字是否居槽、被截断处是否有 `…`、进度条比例、有无溢出或压印字。**按目测结果调 `shareCardLayout` 坐标与字号，重跑本步直到满意**（这是视觉迭代步，允许多轮；顺带微调 prompt 字数初值的决定反馈到 Task 2 的 prompt 常量）。
验收后保留该采样测试（-short 跳过），或按实现者判断删除并把迭代结论记进提交信息。

- [ ] **Step 6: 全模块回归 + 提交**

Run: `go test ./modules/starfuyu/... && go build ./...` → 全绿
```bash
git add modules/starfuyu/service
git commit -m "feat(starfuyu): 运势图 render 重写为分享卡模板（10 槽位布局）"
```

---

### Task 6: 回归 + 文档回填 + 上线注记

**Files:**
- Modify: `docs/modules.md` §13（`:1151` 图片行与相关描述）
- Modify: `docs/superpowers/specs/2026-11-13-starfuyu-fortune-card-design.md`（状态：已实施）

**Interfaces:**
- Consumes: Task 1-5 全部产出
- Produces: 可上线状态

- [ ] **Step 1: 全量回归**

Run: `go test ./... -short` → 全绿（仓库级，确认无跨模块波及：key 格式只在 starfuyu 内、modules.md 为文档）

- [ ] **Step 2: 文档回填**

`docs/modules.md` §13：key 行改 `fortune/v2/{date}/{constellation}.jpg`（v2=分享卡模板，v1 旧图留 storage 过期）；图片描述补「新模板底图 1728×2397、分享卡 10 字段、cron 顺序 fetch→generate 不变」；运维注记一行：**上线当天需 admin force 重抓（默认补缺不带新字段）并先删 storage `fortune/v2/{当日}/` 再 execute-all**。spec 状态行改「已实施」。

- [ ] **Step 3: 提交**

```bash
git add docs/modules.md docs/superpowers/specs/2026-11-13-starfuyu-fortune-card-design.md
git commit -m "docs(starfuyu): 分享卡改造回填 modules.md 与 spec 状态"
```
