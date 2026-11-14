# 软著申请材料（alexGo-cloud 后台管理系统软件 V1.0）

为「alexGo-cloud 后台管理系统软件 V1.0」生成软件著作权登记申请材料：源代码文档、操作手册、申请表信息单。全部由脚本产出，代码/截图变化后可重跑。

> ⚠️ `申请表信息单.md` **禁止写入真实身份信息**（本仓库公开）。身份字段保持 `【占位符】`。

## 目录说明

```
docs/copyright/
├── README.md              # 本文件
├── 申请表信息单.md         # 申请表字段底稿（身份占位符）
├── file_manifest.py       # 进册文件清单（源码文档选材）
├── gen_source_doc.py      # 源代码文档生成器（60 页 × 50 行）
├── menu_tools.sh          # 业务菜单停用/恢复（截图前后）
├── shot_list.tsv          # 截图清单（文件名/入口/描述）
├── mask_pii.py            # 截图 PII 打码（登录日志/会员昵称列）
├── manual_content.py      # 操作手册章节数据
├── gen_manual.py          # 操作手册生成器
├── screenshots/           # 实拍截图 PNG（24 张）
├── output/                # 成品 docx
└── tests/                 # pytest + shell 测试
```

## 依赖安装

```bash
python3 -m pip install python-docx pytest
```

页数验证（可选）：安装 LibreOffice 后用 `soffice --headless --convert-to pdf` 转换并数页。

## 重跑顺序

```bash
cd docs/copyright

# 1. 全量测试
python3 -m pytest tests/ -v
bash tests/test_menu_tools.sh

# 2. 源代码文档（代码更新后）
python3 file_manifest.py --root ../..          # 确认 FRONT/BACK ≥1500 行
python3 gen_source_doc.py --root ../.. \
  --out "output/alexGo-cloud后台管理系统软件V1.0-源代码文档.docx"

# 3. 截图（页面变化后；需先起服务）
#    后端：仓库根目录 make run（健康检查 curl localhost:<配置端口>/health）
#    前端：cd frontend/admin && pnpm install && pnpm --filter @vben/web-antd dev
DSN=<读 configs/config.yaml>
export MYSQL_CMD="mysql -h<host> -P<port> -u<user> -p<pass> <db>"
./menu_tools.sh hide && ./menu_tools.sh verify-hidden   # 拍前停用业务菜单
#    按 shot_list.tsv 逐张实拍到 screenshots/（1440×900，admin/admin123 登录）
./menu_tools.sh restore && ./menu_tools.sh verify-restored  # 拍后恢复并校验

# 4. 操作手册（截图更新后；13/16 含 PII 列，重拍后必须重新打码）
python3 mask_pii.py
python3 gen_manual.py --shots screenshots \
  --out "output/alexGo-cloud后台管理系统软件V1.0-操作手册.docx"

# 5. 页数/内容终验
python3 -m pytest tests/test_final.py -v
```

## 成品规格

| 文件 | 规格 |
|---|---|
| 源代码文档 | 恰 60 页（前 30 + 后 30）× 每页 50 行；页眉：全称+版本号+第 N 页/共 60 页 |
| 操作手册 | 25 页；目录 + 六章；25 处入图（24 张截图，02 号复用）；图题连续；页眉同款 |
| 申请表信息单 | 字段底稿，身份信息全部占位符 |

## 注意事项

- **业务菜单**：scorecard/fishgame 菜单由迁移种子写入数据库，截图前必须 `menu_tools.sh hide`，拍完必须 `restore` 并 `verify-restored`。
- **进册范围**：`alexgo-server/` + `pkg/` + `modules/{system,member,order}` + `frontend/admin`；scorecard/fishgame/starfuyu 不进册。
- **清单不足 1500 行**：调 `file_manifest.py` 的 `FRONT_FILES`/`BACK_FILES` 顺位补文件后重跑。
- **日期口径**：开发完成日期 = 首次发表日期 = 2026-10-08（GitHub public 仓库创建日）。
