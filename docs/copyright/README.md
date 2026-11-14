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
├── manual_content.py      # 操作手册章节数据
├── gen_manual.py          # 操作手册生成器
├── screenshots/           # 实拍截图 PNG
├── output/                # 成品 docx
└── tests/                 # pytest 测试
```

## 依赖安装

```bash
python3 -m pip install python-docx pytest
```

## 重跑

（Task 3/6 完成后补全具体命令）

1. `python3 -m pytest tests/ -v` — 全量测试
