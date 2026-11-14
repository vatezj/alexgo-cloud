import re
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "output"
NAME = "alexGo-cloud后台管理系统软件V1.0"
SRC = OUTPUT / f"{NAME}-源代码文档.docx"
MAN = OUTPUT / f"{NAME}-操作手册.docx"


def _xml(path: Path, prefix: str = "word/") -> str:
    with zipfile.ZipFile(path) as z:
        return "".join(
            z.read(n).decode("utf-8", errors="replace")
            for n in z.namelist()
            if n.startswith(prefix)
        )


def test_source_doc_is_60_pages_of_50():
    doc = _xml(SRC, "word/document.xml")
    headers = _xml(SRC, "word/header")
    assert doc.count('w:type="page"') == 59, "应恰有 59 个分页符（60 页）"
    assert "alexGo-cloud 后台管理系统软件" in headers
    assert "PAGE" in headers
    # 每页 50 行：60 页共 3000 个正文段落（段落数 = 3000 行 + 59 个分页空段 + 页眉不算）
    body_paras = doc.count("<w:p>") + doc.count("<w:p ")
    # 3000 行 + 59 分页段 = 3059，留 2% 容差
    assert 3000 <= body_paras <= 3120, f"段落数异常: {body_paras}"


def test_manual_has_all_shots_and_header():
    doc = _xml(MAN, "word/document.xml")
    headers = _xml(MAN, "word/header")
    n_drawings = doc.count("<w:drawing>")
    shot_count = len(list((ROOT / "screenshots").glob("*.png")))
    # 手册引用 25 处（02-dashboard 引两次），截图文件 24 张
    assert n_drawings == 25, f"入图数 {n_drawings} != 25"
    assert shot_count == 24, f"截图文件 {shot_count} != 24"
    assert "alexGo-cloud 后台管理系统软件" in headers
    assert "截图待补" not in doc, "真品不应有占位符"


def test_info_sheet_clean():
    text = (ROOT / "申请表信息单.md").read_text(encoding="utf-8")
    assert not re.search(r"\b\d{17}[\dXx]\b", text)
    assert "【占位符】" in text
    for field in ["alexGo-cloud 后台管理系统软件", "V1.0", "2026-10-08"]:
        assert field in text


def test_outputs_named_with_software_name():
    names = sorted(p.name for p in OUTPUT.glob("*.docx"))
    assert len(names) == 2, names
    for n in names:
        assert NAME in n, n


def test_no_secrets_in_output_xml():
    # 禁词模式（不落真实凭据明文——仓库公开）
    banned = ["password=", "root:root", "JWT_SECRET", "dsn:", "@tcp("]
    for docx in (SRC, MAN):
        xml = _xml(docx)
        for b in banned:
            assert b not in xml, f"{docx.name} 含敏感串 {b}"
