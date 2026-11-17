# starfuyu 分享卡·顶部星座名 + 左上角角色图 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 分享卡顶部居中画特大星座名（珊瑚粉+白描边），左上角画该星座 Q 版角色抠图，素材由一次性脚本从 `data/starfuyu/cute/` 抠出。

**Architecture:** Python 脚本离线抠图（四边 flood-fill 去底 + 水印连通域清除 + bbox 裁边）产出 12 张透明 PNG；Go `image.go` render 新增两绘制元素（常量集中在 `share_card.go`）；新配置 `starfuyu.cute_dir` 定位素材，缺文件 30502；存图 key 升 v3 强制重生成。

**Tech Stack:** Go + fogleman/gg v1.3.0（DrawImageAnchored 走当前矩阵，可 Scale）、Python3 + PIL/numpy、viper 配置。

**Spec:** `docs/superpowers/specs/2026-11-16-starfuyu-poster-title-photo-design.md`

## Global Constraints

- 画布 1728×2397（bg.png 固有）；坐标微调只动 `shareCardLayout`/`shareFont*` 常量表。
- 错误码：素材/字体/底图缺失一律 30502（`errs.CodeStarfuyuImageFailed`），不静默回退——与 `font_file`/`bg_file` 既有精神一致。
- 存图 key 恒为 `fortune/v3/{date}/{constellation}.jpg`（v3=顶部标题+角色图；升版是幂等跳过的强制要求）。
- 直接提交 main，不建功能分支；每任务一提交，提交信息 `feat(starfuyu): …` 中文（对齐仓库近期风格）。
- 抠图脚本一次性运行；产物 `data/starfuyu/cute/cutout/cute_{en}_cut.png` 与源图一起入库（`data/starfuyu/bg.png` 已被 git 跟踪，同目录不忽略）。
- 12 星座中文名钉死枚举（`service.ConstellationList`），slug 映射 12 条硬表。

## Review Focus

1. **key 不升 v3 → 线上老图被幂等跳过永不重生成** —— 归 Task 4，测试断言 `fortune/v3/…` key（`TestEnsure_WritesJPEGAndSecondCallSkips`、`TestRender_FullShareFields`）。
2. **fixture 未设 `CuteDir` → 既有渲染测试全红 30502** —— 归 Task 3，改完 fixture 后跑整个 `image_test.go` 全套通过。
3. **水印残留 / 角色肚皮被抠穿** —— 归 Task 1，脚本自检（水印区残余不透明像素=0、前景像素数>30 万）+ 拼图人工验收门。
4. **标题压上卡/右上太阳、角色图越界压运程条** —— 归 Task 5，视觉样图 `TestRenderSampleForVisual` 肉眼对照参考图。
5. **cute_dir 缺文件静默跳过** —— 归 Task 3，`TestRender_CuteFileMissing30502` 断言 30502 且错误文本含 `cute`、不落盘。

---

### Task 1: 抠图脚本 + 12 张产物 + 预览验收门

**Files:**
- Create: `scripts/starfuyu_cutout.py`
- Create: `data/starfuyu/cute/cutout/cute_{en}_cut.png` ×12（脚本产物）
- Create: `/tmp/starfuyu_cutout_sheet.png`（4×3 拼图，验收用，不入库）

**Interfaces:**
- Consumes: 源图 `data/starfuyu/cute/cute_{en}_solid_bg_1.png`（1536×1536 纯色底、右下角「豆包AI生成」水印）。
- Produces: `cute_{en}_cut.png`（RGBA、alpha bbox 裁边+8px 透明边），slug 集 `{aries,taurus,gemini,cancer,leo,virgo,libra,scorpio,sagittarius,capricorn,aquarius,pisces}` —— Task 3 的 Go 映射表必须与之逐字一致。

- [ ] **Step 1: 写 `scripts/starfuyu_cutout.py`**

算法钉死（签名与常量如下，实现自由）：
- 遍历 `data/starfuyu/cute/cute_*_solid_bg_1.png`，slug = 文件名 `cute_` 与 `_solid_bg_1.png` 之间段。
- 底色种子 = 四角(2px 内缩)均值；候选底 mask = 逐通道 max 绝对差 ≤ **18**。
- **四边 BFS flood-fill** 只删连通到边的底色（角色内部同色不穿洞）→ alpha 0/255 二值。
- 水印：对前景做 4 连通域标注，**整块 bbox 落在 x>0.70w 且 y>0.90h 的连通域置透明**（字形独立于角色；角色 bbox 不可能整块落入该区）。
- 裁边：alpha bbox 外扩 8px，保存 `data/starfuyu/cute/cutout/cute_{slug}_cut.png`。
- 每张打印自检行：前景像素数、bbox、删除连通域数、**原始坐标下水印区残余不透明像素数**。
- 最后拼 4×3 联系表（粉底 `#FFE6EB`、格 ~320px、含 slug 标注）存 `/tmp/starfuyu_cutout_sheet.png`。

- [ ] **Step 2: 运行脚本并核自检**

Run: `python3 scripts/starfuyu_cutout.py`
Expected: 12 行报告，每行前景像素 >300000、水印区残余 =0；`ls data/starfuyu/cute/cutout/` 恰 12 个文件。

- [ ] **Step 3: 人工验收（门）**

Read `/tmp/starfuyu_cutout_sheet.png`：12 张均无水印、无缺角、肚皮/浅色部位无洞。
**把拼图展示给用户并等确认**；有问题回 Step 1 调常量（水印区范围/容差）重跑。

- [ ] **Step 4: Commit**

```bash
git add scripts/starfuyu_cutout.py data/starfuyu/cute
git commit -m "feat(starfuyu): cute 角色素材抠图脚本与 12 张透明产物"
```

---

### Task 2: 顶部星座名标题渲染

**Files:**
- Modify: `modules/starfuyu/service/share_card.go`（Title 槽 + 3 个样式常量）
- Modify: `modules/starfuyu/service/image.go`（`drawShareTitle` + render 调用）
- Test: `modules/starfuyu/service/image_test.go`

**Interfaces:**
- Consumes: `f.Constellation string`（12 枚举，Ensure 路径必非空）、`drawShareText` 同款槽位/测宽惯例。
- Produces: `shareCardLayout.Title slot`；`drawShareTitle(dc *gg.Context, fontPath, text string, r slot) error`——Task 5 视觉迭代只调这些常量。

- [ ] **Step 1: 写失败测试 `TestRender_TitleDrawn`**

```go
// 顶部标题槽：星座名必画，区域与底图最大像素差 > 20。
func TestRender_TitleDrawn(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "题"))
	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	// … err nil 断言
	img := decodeRendered(t, f, context.Background(), key)
	bg := loadShareBG(t)
	if d := regionMaxDiff(img, bg, shareCardLayout.Title); d <= 20 {
		t.Fatalf("Title 槽区域最大像素差 = %.2f, want > 20（标题必画）", d)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run TestRender_TitleDrawn -v`
Expected: FAIL（`shareCardLayout.Title undefined`）

- [ ] **Step 3: 实现**

`share_card.go`：
- `shareFontTitle = 240.0`、`shareTitleColor = "#F2837F"`、`shareTitleStroke = 12.0`（白描边半径）。
- layout 结构体加 `Title slot`，初值 `Title: slot{164, 60, 1400, 280}`（槽中心 x=864=画布中线；ink 底 ~310 < 上卡顶 403；右缘 ~1224 < 右上太阳 x~1450）。

`image.go`：
- `drawShareTitle`：LoadFontFace(shareFontTitle) → MeasureString 水平居中（`cx = r.x+(r.w-w)/2`）→ 垂直基线 `cy = r.y+r.h/2 + shareFontTitle*0.36`（与 drawShareText 的 0.36 惯例一致）→ 16 向圆周偏移 ±shareTitleStroke 画白描边 → 原位画 `shareTitleColor` 面字；`text==""` 直接返回 nil。
- `render` 在画完 bg 后、上卡五槽前调用：`drawShareTitle(dc, fontPath, f.Constellation, shareCardLayout.Title)`。
- `image.go` 需新增 `"math"` import。

- [ ] **Step 4: 跑测确认通过**

Run: `go test ./modules/starfuyu/service/ -run TestRender_TitleDrawn -v` → PASS
Run: `go test ./modules/starfuyu/service/ -run 'TestRender_' -v` → 既有 Render 测试全 PASS

- [ ] **Step 5: Commit**

```bash
git add modules/starfuyu/service/share_card.go modules/starfuyu/service/image.go modules/starfuyu/service/image_test.go
git commit -m "feat(starfuyu): 分享卡顶部星座名大字（珊瑚粉+白描边）"
```

---

### Task 3: 左上角角色图（配置 + slug 映射 + 渲染 + 30502）

**Files:**
- Modify: `pkg/config/config.go`（Starfuyu 段加 `CuteDir`）
- Modify: `pkg/config/loader.go`（SetDefault）
- Modify: `pkg/config/starfuyu_test.go`（默认值断言）
- Modify: `alexgo-server/configs/config.yaml`（starfuyu 段加 `cute_dir`）
- Modify: `modules/starfuyu/service/constellation.go`（slug 映射）
- Modify: `modules/starfuyu/service/image.go`（cuteDir 字段、resolveCutePath、render 画角色）
- Test: `modules/starfuyu/service/image_test.go`（fixture + 3 个新测试）

**Interfaces:**
- Consumes: Task 1 产物路径规则 `cute_{slug}_cut.png`；`cfg.Starfuyu.CuteDir`。
- Produces:
  - `constellationSlug(c string) (string, bool)` —— 中文星座→英文 slug，12 条。
  - `(*imageService).resolveCutePath(constellation string) (string, error)` —— 30502 语义：cute_dir 空、slug 未命中、文件 stat 失败。
  - `imageService.cuteDir` 字段（NewImageService 从 `cfg.Starfuyu.CuteDir` 取）。

- [ ] **Step 1: 写失败测试**

`image_test.go` 追加三个测试：

```go
// TestConstellationSlug 12 星座→slug 硬表全量覆盖且无重复。
func TestConstellationSlug(t *testing.T) {
	want := map[string]string{
		"白羊座": "aries", "金牛座": "taurus", "双子座": "gemini", "巨蟹座": "cancer",
		"狮子座": "leo", "处女座": "virgo", "天秤座": "libra", "天蝎座": "scorpio",
		"射手座": "sagittarius", "摩羯座": "capricorn", "水瓶座": "aquarius", "双鱼座": "pisces",
	}
	// 断言 12 条逐一命中、ConstellationList 覆盖齐全
}

// TestRender_MascotDrawn 角色槽必画：Mascot 区域与底图最大像素差 > 20。
// （写法同 TestRender_TitleDrawn，槽换 shareCardLayout.Mascot）

// TestRender_CuteFileMissing30502 cute_dir 指向空目录 → 30502 含 "cute"、不落盘。
func TestRender_CuteFileMissing30502(t *testing.T) {
	f := newImageFixture(t, "")
	f.svc.(*imageService).cuteDir = t.TempDir() // 同包测试直改字段，免动夹具签名
	// seed → Ensure(白羊座) → 断言 errs.CodeStarfuyuImageFailed、
	// strings.Contains(err.Error(), "cute")、key==""、saves==0
}
```

同时 `pkg/config/starfuyu_test.go` 加：`cfg.Starfuyu.CuteDir != "./data/starfuyu/cute/cutout"` 即 Fatal。

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./modules/starfuyu/service/ -run 'TestConstellationSlug|TestRender_MascotDrawn|TestRender_CuteFileMissing30502' -v && go test ./pkg/config/ -run Starfuyu -v`
Expected: 全 FAIL（undefined / 默认值不符）

- [ ] **Step 3: 实现配置层**

- `config.go` Starfuyu 段（BgFile 之后）：`CuteDir string \`mapstructure:"cute_dir"\`` + 注释（源码树外，如 ./data/starfuyu/cute/cutout；空串不回退，渲染时 30502）。
- `loader.go`：`v.SetDefault("starfuyu.cute_dir", "./data/starfuyu/cute/cutout")`（挨着 bg_file 默认行）。
- `config.yaml` starfuyu 段加：`cute_dir: "./data/starfuyu/cute/cutout"   # 角色抠图目录（12 张透明 PNG，脚本 scripts/starfuyu_cutout.py 产出）`。

- [ ] **Step 4: 实现映射与渲染**

- `constellation.go`：`var constellationSlugs = map[string]string{…12 条…}` + `func constellationSlug(c string) (string, bool)`。
- `image.go`：
  - `imageService` 加 `cuteDir string`；`NewImageService` 里 `cuteDir = cfg.Starfuyu.CuteDir`（cfg nil 时留空）。
  - `resolveCutePath`：cuteDir=="" → 30502「cute_dir 未配置」；slug miss → 30502「非法星座」；`os.Stat(filepath.Join(cuteDir, "cute_"+slug+"_cut.png"))` 失败 → 30502 指明完整路径。
  - `render` 在 bg 加载后立刻 `cutePath, err := s.resolveCutePath(f.Constellation)`（fail-fast，先于任何绘制）。
  - 角色绘制（上卡五槽之前任意处）：
    ```go
    mascot, err := gg.LoadImage(cutePath)          // 失败 30502 指明路径
    // contain 缩放：s = min(r.w/w, r.h/h)，左上角 (r.x+(r.w-w*s)/2, r.y+(r.h-h*s)/2)
    dc.Translate(px, py); dc.Scale(s, s)
    dc.DrawImageAnchored(mascot, 0, 0, 0, 0)        // gg v1.3.0 走当前矩阵，BiLinear 重采样
    dc.Identity()
    ```
  - 常量：`Mascot: slot{40, 330, 720, 710}`（share_cardLayout 加字段；右让上卡 x≥856、下让运程条 y≈1066）。

- [ ] **Step 5: fixture 补 CuteDir 并跑全套**

- `newImageFixtureCfg` 内加 `cfg.Starfuyu.CuteDir = "../../../data/starfuyu/cute/cutout"`（相对 service 包 cwd；**所有**既有渲染测试都依赖它，Review Focus #2）。
- Run: `go test ./modules/starfuyu/service/ -v` → 全 PASS（含既有 Font/Bg 缺失类测试——它们在 cute 检查前先炸，顺序不受影响）
- Run: `go test ./pkg/config/ -v` → PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/config/config.go pkg/config/loader.go pkg/config/starfuyu_test.go alexgo-server/configs/config.yaml \
  modules/starfuyu/service/constellation.go modules/starfuyu/service/image.go \
  modules/starfuyu/service/share_card.go modules/starfuyu/service/image_test.go
git commit -m "feat(starfuyu): 分享卡左上角角色抠图（cute_dir 配置+slug 映射+缺文件 30502）"
```

---

### Task 4: 存图 key 升 v3

**Files:**
- Modify: `modules/starfuyu/service/image.go:70-81`（fortuneImageKey + 接口注释）
- Modify: `modules/starfuyu/service/image_test.go:95,416`
- Modify: `modules/starfuyu/controller/app/app_test.go:140,362`（stub 同步）
- Modify: `modules/starfuyu/module.go:129`、`modules/starfuyu/controller/admin/admin.go:277,292`（注释里的 key 形如）

**Interfaces:**
- Consumes: 无（纯升版）。
- Produces: key 恒 `fortune/v3/{date}/{constellation}.jpg`。

- [ ] **Step 1: 改断言（先测后改会全红，此任务为纯重命名，断言与实现同提交）**

- `image_test.go`：`"fortune/v2/2026-11-12/白羊座.jpg"` → v3；`"fortune/v2/"+date+` → v3。
- `app_test.go` stub 与 want URL：v2 → v3（stub 是假 Ensure 返回值，与真实实现对齐防误导）。

- [ ] **Step 2: 改实现与注释**

- `fortuneImageKey` 返回 `fortune/v3/…`，函数注释与 `v1=旧布局，v2=分享卡模板` 追加 `v3=顶部星座名+角色图`。
- `image.go:27` 接口注释、`module.go:129`、`admin.go:277,292` 注释中 v2 → v3。

- [ ] **Step 3: 跑测**

Run: `go test ./modules/starfuyu/... -v`
Expected: 全 PASS（grep 仓库内 `fortune/v2` 仅剩历史 spec 文档，代码 0 命中）

- [ ] **Step 4: Commit**

```bash
git add modules/starfuyu
git commit -m "feat(starfuyu): 运势图 key 升 v3（顶部标题+角色图强制重生成）"
```

---

### Task 5: 视觉样图验收 + 常量微调 + 全量回归

**Files:**
- Modify: `modules/starfuyu/service/share_card.go`（仅当视觉不合格时调 Title/Mascot 常量）
- 无新测试（验收 = 样图对照参考图）

**Interfaces:**
- Consumes: Task 2/3 常量；`TestRenderSampleForVisual` 既有采样（白羊座满值、顶格字数）。

- [ ] **Step 1: 出样图**

Run: `go test ./modules/starfuyu/service/ -run TestRenderSampleForVisual -v -short=false`
Expected: `样图已落盘: /tmp/starfuyu-card-sample.jpg`

- [ ] **Step 2: 对照参考图验收（门）**

Read `/tmp/starfuyu-card-sample.jpg`，对照用户给的参考图检查：
1. 「白羊座」大字居中、珊瑚粉+白描边、不压上卡与太阳；
2. 白羊角色在左上、不变形、不越界压卡/运程条；
3. 原 19 槽文字无回归。
不合格 → 只调 `shareCardLayout.Title/.Mascot`、`shareFontTitle`、`shareTitleStroke` → 回 Step 1。
**样图展示给用户确认**。

- [ ] **Step 3: 全量回归**

Run: `go test ./modules/starfuyu/... ./pkg/config/...` → 全 PASS
Run: `go vet ./modules/starfuyu/...` → clean

- [ ] **Step 4: Commit（如有常量微调；无微调则跳过本任务提交）**

```bash
git add modules/starfuyu/service/share_card.go
git commit -m "feat(starfuyu): 分享卡标题/角色槽视觉微调"
```
