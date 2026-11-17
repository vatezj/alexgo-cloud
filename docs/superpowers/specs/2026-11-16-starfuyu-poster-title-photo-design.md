# starfuyu 分享卡·顶部星座名 + 左上角角色抠图 设计文档

日期：2026-11-16
状态：待批准
前置：`2026-11-13-starfuyu-fortune-card-design.md`（分享卡槽位布局，本文只加顶部两个新元素）

## 1. 背景与意图

分享卡（`data/starfuyu/bg.png` 1728×2397 + `image.go` gg 绘制）顶部与左列目前空白。
用户要对齐参考样式：**顶部居中一个特大星座名（珊瑚粉+白描边）**，**左上角一个该星座的 Q 版角色图**。
角色素材已备好：`data/starfuyu/cute/cute_{en}_solid_bg_1.png` 12 张，1536×1536 纯色奶油底，
但带「豆包AI生成」右下角水印，需抠图后使用。

## 2. 目标 / 非目标

**目标**
- 离线抠图脚本：四边 flood-fill 去底 + 清水印 → 透明 PNG，12 张一次处理。
- `render` 新增：顶部星座名大字（描边）、左上角角色图（等比缩放入槽）。
- 存图 key `fortune/v2` → `fortune/v3`（幂等跳过，不升版老图不重生成）。

**非目标**
- 不动现有 19 槽布局、字体、底图、DeepSeek 抓取。
- 不做标题/照片的后台可配置（坐标字号走常量表，视觉迭代微调）。
- 不清理 storage 已落盘的 v2 旧图（自然过期）。

## 3. 抠图（方案 A，已选定）

脚本 `scripts/starfuyu_cutout.py`（Python3 + PIL + numpy，均可用）：

1. **去底**：四角色像素均值为种子色，容差 ≤18（max abs channel）做候选底色 mask，
   **从四边 flood-fill** 只删连通到边的底色——角色肚皮等同色内部不穿洞。
2. **清水印**：flood-fill 后，删掉完全落在右下水印区（x>0.70w, y>0.90h）的独立连通域
   （水印字形与角色不相连，逐 glyph 连通域整块移除；若某张与角色相交则该张单独处理并人工过目）。
3. **裁边**：按 alpha bbox 裁紧（留 8px 透明边），角色大小不一但构图可预期。
4. **产出**：`data/starfuyu/cute/cutout/cute_{en}_cut.png`（aries…pisces 12 枚，与源文件同 slug）。

一次性运行，产物作为运行时素材入库目录（`data/` 下，与 bg.png 同级习惯）。
出图前先拼 12 张抠图 + 一版标题合成预览给用户过目。

## 4. 渲染（image.go + share_card.go）

### 4.1 常量（share_cardLayout 增两槽 + 标题样式常量）

| 项 | 初值 | 说明 |
|---|---|---|
| `Title` 槽 | x=164, y=60, w=1400, h=280 | 顶部水平居中绘制（锚点中心），ink 不压上卡（卡顶 y≈403）与右上太阳 |
| 标题字号 | 240 | 「狮子座」3 字 ≈720px 宽，视觉迭代微调 |
| 标题色 / 描边 | `#F2837F` / 白 12px | gg 无描边 API：白字 8 向偏移画底 + 粉字盖面 |
| `Mascot` 槽 | x=40, y=330, w=720, h=710 | 左列：上接标题带下缘、左贴边、右让开上卡（x≥856）、下让开运程条（y≈1066） |

星座名文本取 `f.Constellation`（已有字段，12 枚举）。
角色图 **等比 contain 缩放**居中于 Mascot 槽（各张 bbox 比例不同，contain 不变形不越界）。

### 4.2 素材定位与缺失误

新增配置 `starfuyu.cute_dir`（config.go + config.yaml + loader 默认 `./data/starfuyu/cute/cutout`），
与 `font_file`/`bg_file` 同精神：**缺目录/缺该星座文件 → 30502 指明路径，不静默跳过**。

中文星座 → 文件 slug 映射（12 条硬表，放 constellation.go 旁）：
白羊座→aries、金牛座→taurus、双子座→gemini、巨蟹座→cancer、狮子座→leo、处女座→virgo、
天秤座→libra、天蝎座→scorpio、射手座→sagittarius、摩羯座→capricorn、水瓶座→aquarius、双鱼座→pisces。

### 4.3 key 升版

`fortuneImageKey`：`fortune/v2/` → `fortune/v3/`（注释同步：v3=顶部星座名+角色图）。

## 5. 测试

- `image_test.go`：v2→v3 断言更新；新增① 顶部标题像素断言（标题带内出现粉色系像素）、
  ② Mascot 槽内出现非底图像素（角色已画入）、③ cute 文件缺失 30502、④ slug 映射表全 12 枚覆盖。
- `TestRenderSampleForVisual` 样图重跑，肉眼验收标题+角色。
- 全模块 `go test ./modules/starfuyu/...` 回归。

## 6. 实施顺序

1. 抠图脚本 → 跑 12 张 → 拼预览给用户过目（**先验收素材再改渲染**）。
2. 配置/映射/key 升版 + 渲染两元素 + 测试。
3. 样图视觉迭代（字号/坐标微调只动常量表）→ 回归 → 提交（直接落 main）。
