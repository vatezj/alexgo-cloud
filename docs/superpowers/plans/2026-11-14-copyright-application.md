# 软著申请材料生成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 alexGo-cloud 生成一套软著申请材料：60 页源代码文档、≥25 页图文操作手册、申请表信息单，全部由脚本产出、可重跑。

**Architecture:** Python + python-docx 两个生成器（源码文档、操作手册）吃一份有序文件清单和一份拍摄清单；截图前用 shell 脚本临时停用 scorecard/fishgame 菜单，拍完恢复校验；申请表信息单为纯 Markdown、身份信息强制占位符。

**Tech Stack:** Python 3（python-docx、pytest）、bash + mysql CLI、Playwright MCP（实拍）、WPS（人工抽查）

**Spec:** `docs/superpowers/specs/2026-11-13-copyright-application-design.md`

## Global Constraints

- 软件全称 `alexGo-cloud 后台管理系统软件`，版本 `V1.0`；两份 docx 每页页眉与文件名都用它。
- 源代码文档：恰好 60 页（前 30 + 后 30），每页恰好 50 行，**显式分页符**分页（不依赖流式回流）。
- 页眉：左 = 软件全称+版本号；右 = `第 N 页 / 共 60 页`（PAGE/NUMPAGES 域代码）。
- 手册 ≥25 页，图题连续编号（图 N），页眉同款。
- 开发完成日期 = 首次发表日期 = `2026-10-08`；信息单无任何真实身份信息（禁 18 位身份证正则命中）。
- 进册范围只含：`alexgo-server/`、`pkg/`、`modules/{system,member,order}`、`frontend/admin`。排除一切 `*_test.go`、`node_modules`、`dist`、自动生成 `.d.ts`、`configs/config.yaml`、scorecard/fishgame/starfuyu。
- 截图 1440×900，登录 `admin/admin123`，后端 `:8080`、前端 `:5666`。
- 产出目录 `docs/copyright/`，成品在 `docs/copyright/output/`；提交直接落 main（本仓库惯例）。

## Review Focus

1. **50 行在 WPS 里溢出到第 2 页**（字号/行距/页边距没配死）→ Task 3 定死 9pt/单倍行距/上下 2.0cm 边距并断言分页符数；Task 7 用 WPS 抽查第 1、30、31、60 页。
2. **菜单停用被启动 seed 回滚，或侧边栏不按 status 过滤** → Task 4 `verify` 子命令 + Task 5 拍前断言侧边栏无「打牌记账/摸鱼」、拍后断言恢复。
3. **登录态失效/反代 401，截图拍回登录页** → Task 5 每张图拍前断言当前 URL 非 `/login` 且页面含预期文案。
4. **manifest 混入敏感或生成文件**（`config.yaml`、测试、`.d.ts`）→ Task 2 排除规则的 pytest 断言。
5. **截图缺失导致手册生成崩溃或空图** → Task 6 占位符测试 + Task 5 结束时与 `shot_list.tsv` 对账。

---

### Task 1: 项目骨架 + 申请表信息单

**Files:**
- Create: `docs/copyright/README.md`
- Create: `docs/copyright/申请表信息单.md`
- Create: `docs/copyright/tests/test_info_sheet.py`

**Interfaces:**
- Produces: 目录结构（后续任务的落位约定）；`申请表信息单.md` 字段值被 Task 7 终验引用。

- [ ] **Step 1: 写失败测试**

`docs/copyright/tests/test_info_sheet.py`：

```python
import re
from pathlib import Path

SHEET = Path(__file__).resolve().parents[1] / "申请表信息单.md"
REQUIRED = ["alexGo-cloud 后台管理系统软件", "V1.0", "2026-10-08", "独立开发", "原始取得", "全部权利"]

def test_required_fields_present():
    text = SHEET.read_text(encoding="utf-8")
    for field in REQUIRED:
        assert field in text, field

def test_no_real_id_number():
    text = SHEET.read_text(encoding="utf-8")
    assert not re.search(r"\b\d{17}[\dXx]\b", text), "信息单含疑似真实身份证号"

def test_placeholders_for_identity():
    text = SHEET.read_text(encoding="utf-8")
    assert "【" in text and "】" in text, "身份字段必须是占位符"
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd docs/copyright && python3 -m pytest tests/test_info_sheet.py -v`
Expected: FAIL（文件不存在）

- [ ] **Step 3: 创建目录与信息单**

建 `docs/copyright/{,output,screenshots,tests}/`。`申请表信息单.md` 按 spec §8 表格逐字段落值：软件全称、版本号、开发完成日期、首次发表日期 = `2026-10-08`、开发方式=独立开发、权利取得方式=原始取得、权利范围=全部权利、软件分类=应用软件；著作权人/开发者姓名、证件号、联系地址、首次发表地点一律 `【占位符】`；文末附一句警示：本文件不得写入真实身份信息（仓库公开）。

`README.md` 写：用途、目录说明、依赖安装命令 `python3 -m pip install python-docx pytest`、重跑入口（先占位，Task 3/6 完成后补全）。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd docs/copyright && python3 -m pytest tests/test_info_sheet.py -v`
Expected: PASS（3 passed）

- [ ] **Step 5: Commit**

```bash
git add docs/copyright
git commit -m "feat(copyright): 软著材料骨架与申请表信息单（身份占位符）"
```

---

### Task 2: file_manifest.py — 进册文件清单

**Files:**
- Create: `docs/copyright/file_manifest.py`
- Create: `docs/copyright/tests/test_manifest.py`

**Interfaces:**
- Consumes: 仓库源码树。
- Produces:
  - `FRONT_FILES: list[str]`、`BACK_FILES: list[str]`（相对仓库根的有序路径，前段凑满 ≥1500 行、后段 ≥1500 行）
  - `build_manifest(root: Path, files: Sequence[str]) -> list[Path]`（应用排除规则、过滤不存在文件）
  - `count_lines(path: Path) -> int`（`len(text.splitlines())`，Task 3 必须用同一计数）
  - CLI：`python3 file_manifest.py --root <repo>` 打印每文件行数、累计页数（行数÷50 向下取整）、前后段合计

- [ ] **Step 1: 写失败测试**

`tests/test_manifest.py`（用 tmp_path 造合成树，不碰真仓库）：

```python
def test_excludes_test_generated_and_config(tmp_path): ...   # *_test.go、.d.ts、config.yaml、node_modules/ 下文件均被排除
def test_missing_file_skipped(tmp_path): ...                 # 清单里不存在的路径被跳过不抛异常
def test_count_lines_matches_splitlines(tmp_path): ...       # count_lines == len(p.read_text().splitlines())
def test_build_manifest_preserves_order(tmp_path): ...       # 返回顺序 == 传入清单顺序
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd docs/copyright && python3 -m pytest tests/test_manifest.py -v`
Expected: FAIL（module not found）

- [ ] **Step 3: 实现 `file_manifest.py`**

- `EXCLUDE_PARTS = ("_test.go", "node_modules", "dist", "auto-imports.d.ts", "components.d.ts", ".d.ts")` 与精确名 `config.yaml`：命中任一即排除。
- `FRONT_FILES` 起始候选（spec §6.2 叙事「启动→配置→中间件→横切→核心 pkg」）：`alexgo-server/cmd/main.go`、`pkg/resp/{resp.go,page.go}`、`pkg/errors/code.go`、`pkg/auth/{jwt.go,casbin.go}`、`pkg/tenant/{tenant.go,account.go,datascope.go}`、`pkg/middleware/` 与 `pkg/limiter/` 下无测试文件，用 CLI 统计凑满 ≥1500 行。
- `BACK_FILES` 起始候选（「system 代表文件 → member → order → admin 关键页」）：`modules/system/controller/admin/{basic.go,user.go,auth.go,role.go,menu.go,online_controller.go,tenant.go}`、`modules/system/service/{rbac.go,permission_routes.go,datascope.go}`、member/order 各 controller+service 代表、`frontend/admin/apps/web-antd/src/views/_core/authentication/login.vue`、`layouts/basic.vue`、`views/system/System{Users,Roles,OnlineUsers}Page.vue`，CLI 统计凑满 ≥1500 行。
- 单文件 >600 行的（`basic.go` 697、`rbac.go` 599）保留在清单里，截断是 Task 3 的职责。
- CLI 输出末尾打印断言行：`FRONT=xxxx lines (>=1500) BACK=xxxx lines (>=1500)`，不足则退出码 1。

- [ ] **Step 4: 跑测试确认通过 + CLI 统计达标**

Run: `cd docs/copyright && python3 -m pytest tests/test_manifest.py -v && python3 file_manifest.py --root ../..`
Expected: PASS；CLI 打印前后段均 ≥1500 行（不达标则顺位补/删候选文件重跑）

- [ ] **Step 5: Commit**

```bash
git add docs/copyright/file_manifest.py docs/copyright/tests/test_manifest.py
git commit -m "feat(copyright): 进册文件清单与排除规则"
```

---

### Task 3: gen_source_doc.py — 源代码文档生成器

**Files:**
- Create: `docs/copyright/gen_source_doc.py`
- Create: `docs/copyright/tests/test_source_doc.py`

**Interfaces:**
- Consumes: Task 2 的 `FRONT_FILES`、`BACK_FILES`、`build_manifest`、`count_lines`。
- Produces:
  - `take_lines(root: Path, files: Sequence[str], target: int) -> list[str]`：按序读文件，每文件前插标记行 `// ===== {path} =====`，累计到 `target` 行；超界则截断并追加 `// ... (文件续略)`；不足 `target` 抛 `ValueError(缺多少行)`。
  - `paginate(lines: list[str], pages: int = 60, per_page: int = 50) -> list[list[str]]`：断言 `len(lines) == pages * per_page`，均分。
  - `render(lines, out: Path, name: str, version: str) -> None`：docx 落盘。
  - CLI：`python3 gen_source_doc.py --root ../.. --out output/<文件名>.docx`，文件名 = `alexGo-cloud后台管理系统软件V1.0-源代码文档.docx`。

- [ ] **Step 1: 写失败测试**

`tests/test_source_doc.py`（合成小文件树 + 缩小参数，不生成真 60 页）：

```python
def test_take_lines_inserts_marker_and_truncates(tmp_path): ...
    # 两文件共 30 行，target=20 → 恰 20 行；含两个 // ===== 标记；末行是续略标记
def test_take_lines_raises_on_shortfall(tmp_path): ...
    # target=100 只有 30 行 → ValueError 且消息含 "70"
def test_paginate_rejects_wrong_total(): ...
    # len != pages*per_page → AssertionError
def test_render_builds_docx_with_header_and_breaks(tmp_path): ...
    # 用 2 页 × 3 行渲染 → docx 里 PAGE break 恰 1 个；document.xml 含文件名；页眉 xml 含
    #   "alexGo-cloud 后台管理系统软件" 与 "PAGE" 域指令（w:instrText）
```

docx 断言方式：`zipfile` 解包读 `word/document.xml` / `word/header1.xml` 做字符串包含检查，不依赖 Word 渲染。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd docs/copyright && python3 -m pytest tests/test_source_doc.py -v`
Expected: FAIL（module not found）

- [ ] **Step 3: 实现 `gen_source_doc.py`**

- `take_lines`：读文件用 `encoding="utf-8", errors="replace"`；计数与 `count_lines` 同为 `splitlines()`；标记行计入 50 行配额。
- `paginate`：纯均分，最后一页同样 50 行（spec §6.1：按 50 设计则无需例外）。
- `render`：
  - 页面 A4，页边距 上下 2.0cm、左右 2.5cm。
  - 正文字体 Consolas（回退 Courier New）**9pt**，行距单倍，段前段后 0——50 行必须整页容下（Review Focus #1）。
  - 每页 50 个段落后 `add_page_break()`；第 60 页不加分页符 → PAGE break 恰 59 个。
  - 页眉：段落左对齐软件全称+版本号，右对齐 `第 ` + PAGE 域 + ` 页 / 共 ` + NUMPAGES 域 + ` 页`；域用 `OxmlElement("w:fldSimple")` 或 `w:instrText` 写入（python-docx 无高层 API）。
- CLI：`build_manifest` → 前段 `take_lines(..., 1500)` + 后段 `take_lines(..., 1500)` → `paginate(3000, 60, 50)` → `render`；打印成品路径与断言行 `pages=60 lines=3000 breaks=59`。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd docs/copyright && python3 -m pytest tests/test_source_doc.py -v`
Expected: PASS

- [ ] **Step 5: 生成真品并自检**

Run: `cd docs/copyright && python3 gen_source_doc.py --root ../.. --out "output/alexGo-cloud后台管理系统软件V1.0-源代码文档.docx"`
Expected: 打印 `pages=60 lines=3000 breaks=59`；文件存在且 >50KB。

- [ ] **Step 6: Commit**

```bash
git add docs/copyright/gen_source_doc.py docs/copyright/tests/test_source_doc.py
git commit -m "feat(copyright): 源代码文档生成器（60页×50行+页眉域代码）"
```

---

### Task 4: menu_tools.sh — 业务菜单停用/恢复

**Files:**
- Create: `docs/copyright/menu_tools.sh`
- Create: `docs/copyright/tests/test_menu_tools.sh`

**Interfaces:**
- Consumes: 本地 MySQL（`alexgo` 库，DSN 默认 `root:root`，可用 `MYSQL_PWD`/`MYSQL_CMD` 覆盖）。
- Produces:
  - `menu_tools.sh backup|hide|restore|verify-hidden|verify-restored`（cwd 任意，路径自定位）
  - 状态备份 `docs/copyright/.menu_backup.tsv`（列：`id<TAB>path<TAB>status`，仅 gitignore 外的本地文件——**加入 `.gitignore`**）
  - 选择集：`path LIKE '/scorecard%' OR path LIKE '/fishgame%'`（根目录 + 子菜单全覆盖）

- [ ] **Step 1: 写失败测试（不依赖数据库的部分）**

`tests/test_menu_tools.sh`：`bash -n menu_tools.sh` 语法检查；`grep` 断言脚本含 `scorecard`、`fishgame`、`status=0`、`verify-hidden`、`verify-restored` 五个关键字；断言 `.gitignore` 覆盖 `.menu_backup.tsv`。

Run: `bash docs/copyright/tests/test_menu_tools.sh`
Expected: FAIL（脚本不存在）

- [ ] **Step 2: 实现 `menu_tools.sh`**

- `MYSQL_CMD` 默认 `mysql -uroot ${MYSQL_PWD:+-p$MYSQL_PWD} alexgo`（可用环境变量整串覆盖）。
- `backup`：`SELECT id,path,status FROM menus WHERE deleted=0 AND (path LIKE '/scorecard%' OR path LIKE '/fishgame%')` → TSV；已有备份不覆盖（防止二次 hide 冲掉原状态）。
- `hide`：先 `backup`，再 `UPDATE menus SET status=0 WHERE ...`（同选择集）。
- `restore`：按 TSV 逐行 `UPDATE menus SET status=<s> WHERE id=<id>`，完成后删 TSV。
- `verify-hidden`：选择集内 `status<>0` 行数为 0 才退出 0，否则打印违规行退出 1。
- `verify-restored`：TSV 每行 `id` 的当前 `status` 与备份一致才退出 0（TSV 已删则报错提示先 backup）。
- 每个子命令开头 `set -euo pipefail`，SQL 静态拼接（无用户输入注入面）。

- [ ] **Step 3: 跑静态测试通过**

Run: `bash docs/copyright/tests/test_menu_tools.sh`
Expected: PASS

- [ ] **Step 4: 有库则做冒烟（无库则记录跳过）**

Run: `docs/copyright/menu_tools.sh hide && docs/copyright/menu_tools.sh verify-hidden && docs/copyright/menu_tools.sh restore && docs/copyright/menu_tools.sh verify-restored`
Expected: 全部退出 0；若本机无 MySQL，此步在提交信息里注明「DB 冒烟跳过」，留给 Task 5 preflight 补做。

- [ ] **Step 5: Commit**

```bash
git add docs/copyright/menu_tools.sh docs/copyright/tests/test_menu_tools.sh .gitignore
git commit -m "feat(copyright): 业务菜单停用/恢复工具（含状态备份校验）"
```

---

### Task 5: 截图采集（实拍 + 对账）

**Files:**
- Create: `docs/copyright/shot_list.tsv`（规范：`文件名<TAB>路由/入口<TAB>拍摄描述`，一行一张）
- Create: `docs/copyright/screenshots/*.png`
- Modify: `docs/copyright/README.md`（补启动/截图操作说明）

**Interfaces:**
- Consumes: Task 4 的 `menu_tools.sh`；本地 MySQL、`make run`（:8080）、admin dev（:5666）；Playwright MCP。
- Produces: `screenshots/` 下与 `shot_list.tsv` 一一对账的 PNG；文件名清单被 Task 6 的 `manual_content.py` 引用。

- [ ] **Step 1: 定稿拍摄清单**

`shot_list.tsv` 共 24 张（spec §7.1 目录反推；文件名即最终名）：

```
01-login.png	/login	登录页
02-dashboard.png	dashboard/workspace	仪表盘工作台
03-users.png	系统管理/用户管理	用户列表
04-roles.png	系统管理/角色管理	角色列表
05-menus.png	系统管理/菜单管理	菜单树（业务菜单呈停用态，属预期）
06-depts.png	系统管理/部门管理	部门树
07-posts.png	系统管理/岗位管理	岗位列表
08-dict.png	系统管理/字典管理	字典类型+明细
09-configs.png	系统管理/配置管理	配置项列表
10-notices.png	系统管理/公告通知	公告列表
11-notice-upload.png	公告编辑器	附件上传入口（文件能力）
12-tenants.png	系统管理/租户管理	租户列表
13-login-logs.png	系统管理/登录日志	登录日志
14-operate-logs.png	系统管理/操作日志	操作日志
15-online-users.png	系统管理/在线用户	在线用户列表
16-member-search.png	在线用户页会员搜索	会员能力入口
17-my-notices.png	我的通知	站内通知已读
18-oauth2-clients.png	系统管理/OAuth2客户端	客户端列表
19-social-apps.png	系统管理/社交应用	社交应用列表
20-order.png	订单管理	订单列表
21-codegen.png	基础设施/代码生成	代码生成配置表
22-profile.png	个人中心	个人信息
23-logout.png	右上角用户菜单	退出登录入口
24-notice-detail.png	公告通知	公告详情/编辑
```

- [ ] **Step 2: Preflight — 起服务**

1. MySQL 冒烟：`menu_tools.sh hide && verify-hidden && restore && verify-restored`（补 Task 4 跳过的 DB 冒烟）。
2. 后端：`make run` 后台起，`curl -sf http://localhost:8080/health` 重试至 `{"status":"ok"...}`。
3. 前端：`cd frontend/admin && pnpm install && pnpm --filter @vben/web-antd dev` 后台起，等待 :5666 可访问。
4. 任一步失败 → 停止本任务，走 spec §7.2 降级（Task 6 占位符路径），不阻塞其他材料。

- [ ] **Step 3: 停用业务菜单并断言侧边栏干净**

Run: `docs/copyright/menu_tools.sh hide && docs/copyright/menu_tools.sh verify-hidden`
Playwright：`:5666` 登录 `admin/admin123`，断言侧边栏文本**不含**「打牌记账」「摸鱼」（含则说明 status 未生效 → 改用角色解绑兜底：记录 `roles_menus` 中对应 `menu_id` 绑定到 TSV 再删，恢复时插回）。

- [ ] **Step 4: 按清单逐张实拍**

Playwright 统一 `browser_resize` 1440×900。每张的固定节奏（Review Focus #3）：
1. 导航到目标（点菜单或直接 goto 路由）；
2. **断言**：URL 不含 `/login`，且页面含该页特征文案（如用户管理页含「用户名称」或表格行）；
3. `browser_take_screenshot` 存 `screenshots/<文件名>`（PNG，视口截图）。

顺序按 `shot_list.tsv` 行序；`11-notice-upload.png` 需点开公告编辑器并露出上传控件；`23-logout.png` 只拍用户下拉菜单展开态（**拍完别真退出**，后续图还需登录态；若已退出则重新登录）。

- [ ] **Step 5: 对账 + 恢复 + 校验**

1. 对账：`shot_list.tsv` 每行文件名在 `screenshots/` 存在且 >10KB，缺一张补拍一张，输出 `24/24 OK`。
2. 恢复：`menu_tools.sh restore && menu_tools.sh verify-restored`；Playwright 刷新断言侧边栏「打牌记账/摸鱼」回来（或角色解绑路径则绑定已插回）。
3. 收尾：停掉前端 dev 与后端进程（保留 MySQL）。

- [ ] **Step 6: Commit**

```bash
git add docs/copyright/shot_list.tsv docs/copyright/screenshots docs/copyright/README.md
git commit -m "feat(copyright): 24 张后台实拍截图与拍摄清单（业务菜单已恢复）"
```

> 截图是二进制成品，直接入库；`screenshots/` 不加 gitignore。

---

### Task 6: gen_manual.py — 操作手册生成器

**Files:**
- Create: `docs/copyright/manual_content.py`（纯数据：章节、步骤、图引用）
- Create: `docs/copyright/gen_manual.py`
- Create: `docs/copyright/tests/test_manual.py`

**Interfaces:**
- Consumes: Task 5 的 `screenshots/*.png` 与 `shot_list.tsv`（文件名）。
- Produces:
  - `manual_content.CHAPTERS: list[dict]` —— spec §7.1 六章结构；每节含 `title`、`steps: list[str]`、`figures: list[dict]`（`file`、`caption`、`path`）。
  - `render(chapters, shots_dir: Path, out: Path) -> None`
  - CLI：`python3 gen_manual.py --shots screenshots --out "output/alexGo-cloud后台管理系统软件V1.0-操作手册.docx"`

- [ ] **Step 1: 写失败测试**

`tests/test_manual.py`：

```python
def test_missing_shot_renders_placeholder(tmp_path):
    # 1 张真 PNG + 1 个引用不存在 → 生成成功，docx 含占位文字「截图待补」
def test_all_figures_rendered_as_images(tmp_path):
    # 2 张真 PNG → document.xml 含 2 个 <w:drawing>
def test_figure_numbers_contiguous(tmp_path):
    # 从渲染结果提取「图 N」→ N 从 1 连续递增无跳号
def test_header_present(tmp_path):
    # 页眉含 "alexGo-cloud 后台管理系统软件"
def test_content_covers_spec_sections():
    # CHAPTERS 的章标题含：软件概述/运行环境/安装与启动/通用功能/功能模块操作/常见问题
def test_every_figure_file_in_shot_list():
    # 所有 figure file 都能在 ../shot_list.tsv 第一列找到
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd docs/copyright && python3 -m pytest tests/test_manual.py -v`
Expected: FAIL（module not found）

- [ ] **Step 3: 实现 `manual_content.py`**

按 spec §7.1 写六章：
1. **软件概述**：一段定位（模块化单体后台基座）+ 功能清单表（从 README 内置功能表裁进册范围的行）。
2. **运行环境**：Go 1.25+、MySQL 8.x、Node ^22.18、可选 Redis/NATS；硬件无特殊要求。
3. **安装与启动**：后端三步（建库 → `make migrate` → `make run`）、验证 `curl /health`；前端 `cd frontend/admin && pnpm install && pnpm --filter @vben/web-antd dev`、访问 `:5666`、默认账号 `admin/admin123`。
4. **通用功能**：登录（图 01）、布局（图 02/23）、个人中心（图 22）、我的通知（图 17）。
5. **功能模块操作**：按 `shot_list.tsv` 顺序组织，每模块 = 进入路径 + 步骤列表 + 图 + 关键字段说明（用户/角色/菜单/字典四页给字段表）；会员能力并入在线用户节（图 16）、文件能力并入公告节（图 11）——spec §3 裁决。
6. **常见问题**：端口占用、DB 连不上、前端 401、菜单不见（业务菜单被停用）4 条。

图题格式 `图 N：{caption}`，N 由渲染时按出场顺序生成（不手写编号，防跳号）。

- [ ] **Step 4: 实现 `gen_manual.py`**

- 标题层级：H1 章（分页符起新页——顺带抬高页数，护住 ≥25 页）、H2 节、正文 11pt 宋体/Calibri、步骤用编号列表。
- 图：宽度 15.5cm 居中（1440×900 等比 ≈9.7cm 高），图题紧随其下居中 9pt。
- 缺图 → 段落占位 `（截图待补：{file}）` 灰字，不抛异常（Review Focus #5）。
- 页眉与源码文档同款（全称+版本号，页码域用 PAGE/NUMPAGES，无「共 60 页」——改「共 N 页」同域即可）。
- CLI 末尾打印：`figures=<实际入图数> missing=<占位数>`。

- [ ] **Step 5: 跑测试 + 生成真品**

Run: `cd docs/copyright && python3 -m pytest tests/test_manual.py -v && python3 gen_manual.py --shots screenshots --out "output/alexGo-cloud后台管理系统软件V1.0-操作手册.docx"`
Expected: PASS；`figures=24 missing=0`（若 Task 5 降级未拍到则 `missing>0` 属预期，占位即可）；文件 >200KB。

- [ ] **Step 6: Commit**

```bash
git add docs/copyright/manual_content.py docs/copyright/gen_manual.py docs/copyright/tests/test_manual.py
git commit -m "feat(copyright): 操作手册生成器（六章结构+图题连续+缺图占位）"
```

---

### Task 7: 终验 + README 定稿

**Files:**
- Modify: `docs/copyright/README.md`（补全重跑步骤）
- Create: `docs/copyright/tests/test_final.py`

**Interfaces:**
- Consumes: Task 1–6 全部产出。
- Produces: 满足 spec §9 全部勾选项的成品集。

- [ ] **Step 1: 写终验测试**

`tests/test_final.py`：

```python
def test_source_doc_is_60_pages_of_50():
    # zipfile 解 output/*源代码文档.docx：
    #   document.xml 中 w:type="page" 的分页符 == 59
    #   页眉含全称与 "PAGE"
def test_manual_has_all_shots_and_header():
    # output/*操作手册.docx：drawing 数 == 24（或 == shot_list 行数）；页眉含全称
def test_info_sheet_clean(): # 复跑 Task 1 两个断言（身份证正则、占位符）
def test_outputs_named_with_software_name():
    # output/ 下两份 docx 文件名均含 "alexGo-cloud后台管理系统软件V1.0"
def test_no_secrets_in_output_xml():
    # 两份 docx 的全部 xml 不含 "password="、"root:root"、"JWT_SECRET"
```

- [ ] **Step 2: 跑全量测试**

Run: `cd docs/copyright && python3 -m pytest tests/ -v`
Expected: 全 PASS（Task 4 的 shell 测试由 `bash tests/test_menu_tools.sh` 单独跑，也 PASS）

- [ ] **Step 3: WPS 人工抽查（Review Focus #1 的兜底）**

Run: `open -a WPSOffice "docs/copyright/output/alexGo-cloud后台管理系统软件V1.0-源代码文档.docx"`
人工核对：总页数 = 60；第 1、30、31、60 页每页 50 行；页眉三要素齐全；抽查页无溢出/空白页。手册打开核对：≥25 页、图题 1→24 连续、截图全可见。
若手册 <25 页 → 回 Task 6 补「关键字段说明」表格行（确定性扩页），重跑 `gen_manual.py` 再验。

- [ ] **Step 4: README 定稿**

`docs/copyright/README.md` 补全：依赖安装 → `file_manifest.py` 统计 → `gen_source_doc.py` → 起服务/`menu_tools.sh`/截图 → `gen_manual.py` → `pytest tests/` 一键重跑顺序；顶部红字警示「申请表信息单禁写真实身份信息」；注明代码更新后只需重跑生成器（清单不够行数时先调 `FRONT_FILES/BACK_FILES`）。

- [ ] **Step 5: Commit**

```bash
git add docs/copyright
git commit -m "feat(copyright): 软著材料终验测试与重跑文档"
```
