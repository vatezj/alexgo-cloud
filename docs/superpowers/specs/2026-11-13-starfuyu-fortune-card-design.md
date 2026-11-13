# starfuyu 运势分享卡·图像文字合成 设计文档

日期：2026-11-13
状态：已实施
前置：`2026-11-12-starfuyu-module-design.md`（模块整体设计，本文只覆盖运势图重做这一块）

## 1. 背景与意图

starfuyu 已有一条完整流水线：DeepSeek 定时抓 12 星座运势 → `starfuyu_fortune` 表 → `ImageService`（gg 绘制旧版运势图）→ `ArticleService` 生成 HTML 传微信公众号草稿箱。

用户（运营）要拿合成图做**微信软文配图**。新模板底图已定稿：`frontend/static/start/bg.png`（1728×2397），是一张粉色系分享卡，留有明确的文字空位。旧图布局（goLucky 风格）作废，**新模板直接替换旧图**——cron 生图、公众号封面、软文插图统一换成新卡。

模板空位 vs 现有 `Fortune` 数据的差距是本设计的核心：合拍星座、建议、避免、复合运/脱单运/求职运、今日必做 均无数据来源，需扩 DeepSeek 抓取 prompt + `starfuyu_fortune` 表加列。

## 2. 目标 / 非目标

**目标**
- `starfuyu_fortune` 表新增模板所需字段，DeepSeek 每日抓取一次带全。
- `ImageService.render` 按新模板坐标重写，所有空位由表数据填充。
- 现有流水线（cron、任务编排、封面、文章）不改结构，只换图。

**非目标**
- 不做手动填词后台界面（内容全部来自 DeepSeek 自动抓取）。
- 不动生成的文章 HTML 版式（仅封面/配图随 ImageService 自动换新）。
- 不动 C 端接口的响应结构（新列是否出现在 app 接口里见 §6.4）。
- 不清理 storage 里已落盘的旧版图（自然过期）。

## 3. 数据层设计

### 3.1 新列（迁移 + model 同构）

新迁移文件 `modules/starfuyu/migrations/20261113000001_starfuyu_fortune_share_fields.{up,down}.sql`，对 `starfuyu_fortune` ADD COLUMN：

| 列名 | 类型 | 模板空位 | 取值 |
|---|---|---|---|
| `match_constellation` | varchar(20) NOT NULL DEFAULT '' | 合拍星座 | 12 星座枚举之一 |
| `advice` | text | 建议 | 短文案 |
| `avoid` | text | 避免 | 短文案 |
| `reunion_tip` | text | 复合运: | 短文案 |
| `single_tip` | text | 脱单运: | 短文案 |
| `job_tip` | text | 求职运: | 短文案 |
| `reunion_score` | int NOT NULL DEFAULT 0 | 复合运卡进度条 | 0-100 |
| `single_score` | int NOT NULL DEFAULT 0 | 脱单运卡进度条 | 0-100 |
| `job_score` | int NOT NULL DEFAULT 0 | 求职运卡进度条 | 0-100 |
| `daily_todos` | json | 今日必做 ×3 | 字符串数组（3 条） |
| `slogan` | text | 底部粉条口号 | 一句 ≤16 字（2026-11-13 增补，白字粉条内水平居中） |

down 迁移 DROP 这些列（mysql 不支持 DROP IF EXISTS，直接 DROP COLUMN，与模块既有迁移风格一致——列不存在时报错可接受，见既有 down 文件风格）。

`model/fortune.go` 同步加字段；`DailyTodos []string` 用与 `LuckyColor` 相同的 `Scan/Value` JSON 列模式。新增 `ShareFields` 之类不需要——平铺即可，与现有模型风格一致。

### 3.2 DeepSeek prompt 扩展（`service/deepseek.go` callAPI）

在 JSON 对象结构里追加：

```json
"advice": "今日建议，≤12字",
"avoid": "今日避免，≤9字",
"match_constellation": "合拍星座中文名（12星座之一）",
"reunion_tip": "复合运一句，≤14字",
"single_tip": "脱单运一句，≤14字",
"job_tip": "求职运一句，≤14字",
"reunion_score": 70,
"single_score": 65,
"job_score": 80,
"daily_todos": ["行动1", "行动2", "行动3"],
"slogan": "今日口号，≤16字（2026-11-13 增补）"
```

生成要求追加：
- advice/avoid 与四个 tip 风格一致、每日不同；match_constellation 不得等于本星座自身。
- daily_todos 恰好 3 条、每条 ≤14 字、可执行的行动建议。
- **字数上限全部为初值**：出图后按视觉效果微调 prompt（这是迭代项，不是一次定死的合同）。
- advice/avoid/summary（建议/避免/简述）字数 12/9/24 为 2026-11-13 视觉迭代回灌后的终值；再放宽需同步加高 Advice/Avoid 槽（share_cardLayout）。

解析容错：DeepSeek 偶发漏字段 → 缺失字段落零值/空串（Go json 解析天然如此），**不判失败**；`daily_todos` 缺失解析为空数组，图上画 0 行。`match_constellation` 非法值（不在 12 枚举）→ 存空串（入库前校验，复用 `IsValidConstellation`）。

`summary`（今日简述）复用现有列，prompt 微调为 ≤24 字的单句总评（现有 prompt 本就要求一句话风格，只需补字数约束）。

### 3.3 兼容与重抓

- 存量行新列为默认空值，不回填、不报错。
- cron `fetch_fortune` 默认只补缺——**当天已抓过就不会带新字段重抓**。上线当天需 admin 手动触发 force 抓取一次（已有 force 参数，无新代码）；文档里写明这一步。
- 抓取失败降级路径不变（30501/网络错误照旧）。

## 4. 图像层设计

### 4.1 底图与资源

- `frontend/static/start/bg.png` **复制**进 `modules/starfuyu/assets/bgimage/bg.png` 替换旧底图（源文件保留在 frontend/static，不动）。
- 字体（2026-11-13 两轮迭代终态）：阿里妈妈普惠体 3 `85-Bold`，路径走配置 `starfuyu.font_file`（默认 `./data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf`，**源码树外**）；配置非空直用、文件缺失 30502 不静默回退，空串回退 `AssetsDir/fonts` 字典序首个（测试/旧部署兼容）；加载失败 → 30502（决定 19 不回退，沿用）。

### 4.2 render 重写

`service/image.go` 的 `render` 整体重写为新模板布局；`drawFortuneLine`、`wrapText`、`scoreToStars` 等 helper 按需改造复用。布局数据结构：

```
上卡五槽（标签在底图上已印好，只画值）：
  今日简述:  ← Summary            （≤30字，超宽自动换行，最多2行）
  今日分数:  ← OverallScore        （"85" 或 "85分"）
  合拍星座:  ← MatchConstellation   （空值留白）
  建议:      ← Advice              （≤24字，1行）
  避免:      ← Avoid               （≤24字，1行）

中间四条运程条（标签 财富/事业/感情/健康 已印在底图）：
  条内文字 ← Score + Tip（如 "85分 · 短文案"，1行，超长截断）

三个小卡（复合运:/脱单运:/求职运: 标签已印）：
  卡内文案 ← *Tip（≤14字，最多2行）
  底部进度条 ← 对应 *Score/100 填充（在底图进度条槽位上画填充段）

今日必做：
  3 行 ← DailyTodos[i]（每行前画勾选框图标位对齐底图；空数组画 0 行，
  不画残留空框——底图自带 3 个框时，用底图裁剪或改用"整卡重绘可选区域"方案，见 §6.2）
```

关键规则：
- **坐标硬编码**：以 1728×2397 模板量出每个槽位 rect，集中放在一个布局常量表里（`shareCardLayout`），便于对照模板微调。
- **绝不溢出**：每槽 rect 内自动换行；超行数 → 截断加 `…`。文字绘制在 rect 内垂直居中或顶对齐（逐槽定）。
- **空值留白**：字段空 → 该槽不画任何字（不画 "—"、不报错），存量旧数据出图不崩。
- **文本色**：按模板底色取深色（模板文字区为浅粉/米白底，用深棕/深灰，与底图已印标签同色系）；阴影省略（旧图的黑影打法在浅底上难看）。
- 输出 JPEG q90 照旧（公众号素材友好）。

### 4.3 图片 key 升版（换模板必须解决的缓存问题）

`Ensure` 幂等逻辑（已存在跳过）会把换模板前生成的旧图当成"已完成"。裁定：**key 加模板版本段**：

```
旧：fortune/{date}/{constellation}.jpg
新：fortune/v2/{date}/{constellation}.jpg
```

- `fortuneImageKey` 改为 v2 格式，顺带更新其校验测试。
- 旧 key 文件留在 storage 自然过期（非目标，不写清理任务）。
- `ArticleService.UploadToMP` 经 `Ensure` 拿 key，透明跟随，无改动。

## 5. 流水线与配置

- cron 两个任务、`TaskRunner`、admin 手动触发端点：**全部不动**。
- 配置 `cfg.Starfuyu`：底图仍是 `AssetsDir/bgimage/bg.png`；**新增 `font_file`**（字体路径，源码树外，2026-11-13 增补）。
- 错误码：沿用 30502（图片）、30501（数据）、30507（公众号），不新增。
- 模块开关 `modules.starfuyu: false` 时不启动 cron——沿用，无改动。

## 6. 已裁定的开放点（builder 不用再问）

### 6.1 「今日简述」映射哪列？
`Summary`（今日总评，短句）。不用 `DailyOverview`（那是含日期前缀的长段落，app 端在用，不动）。

### 6.2 底图自带 3 个空勾选框怎么处理？
「今日必做」区底图印有 3 个固定勾选框。裁定：**文字画在框右侧即可，框全留底图原样**——必做恒 3 条（prompt 钉死），空值时该行留白、框保留。若实测 DeepSeek 偶发少于 3 条视觉突兀，允许实现时改为"用底图周边色 patch 掉多余框"（从同图邻近区域取样填充，gg 可做）——实现者按实测选，二选一即可。

### 6.3 进度条填充画法
底图小卡底部有浅色进度条槽（含心形图标端点）。实现：量出槽 rect，按 score 画深色圆角填充段覆盖在槽上，宽度 = 槽宽 × score/100，填充色取该卡同色系（复合=粉、脱单=橙、求职=粉，取自模板）。心形图标不覆盖。

### 6.4 模板上没有的元素一律不画（用户裁定「按照图片来」）

旧图的「星座名+日期」标题、幸运颜色/幸运数字、四条运程的 ★ 星级显示，**全部砍掉**——模板没印就不画，左上留白区保持原样。卡片的星座/日期辨识由文章上下文兜底。builder 无需再纠结这三处去留。

### 6.5 新列要不要出现在 C 端接口？
不主动加。`Fortune` model 加了列后 JSON 序列化自然带出（json tag 齐全），app 端多收几个字段无害；controller 不改。

## 7. 测试要求

- 迁移 SQL：仓库无迁移测试 harness（golang-migrate 仅 mysql 生产路径），正确性以「列名/类型与 model tag 同构 + up/down 对称」对照保证（无自动化测试，已知取舍）。
- `fortuneImageKey` v2 格式与参数校验。
- DeepSeek 解析：全字段、漏字段（容错）、非法 match_constellation（存空）。
- `render`：用测试资产（小底图+字体）出图不崩；空字段留白不崩；超长文案截断不溢出（可通过绘制后边界采样或仅测 wrap/截断纯函数）。
- `Ensure` 幂等 + v2 key 落盘。
- 既有测试全绿（key 格式变化会波及 image/article 测试，一并更新）。

## 8. 实现留给 builder 的部分

- 模板各槽位的**像素坐标**（量图确定，集中在 `shareCardLayout` 常量表）。
- 逐槽字号、行高、文本色的具体值（对照模板印字目测定初值，视觉校验收敛）。
- §6.2 的勾选框处理二选一、§3.2 的字数上限初值微调——都属出图后视觉迭代。
- 进度条画法：底图预印的浅色段要先铺底再画填充段，避免半透明叠色脏。
- 分数绘制前 clamp 到 0-100（DeepSeek 越界时不出图崩）。
- 上线顺序（纯运维，无新代码）：force 抓取只 upsert 行、不碰图；若当天 v2 图已生成会被幂等跳过——先删 storage `fortune/v2/{当天}/` 再 admin execute-all(force)。
- `frontend/static/start/bg.png` 属新增未跟踪文件，随实施一并提交。
- TDD 顺序与提交切分（按模块惯例：一票一提交，测试先行）。
