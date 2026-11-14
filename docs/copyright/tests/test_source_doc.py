import zipfile
from pathlib import Path

import pytest

from gen_source_doc import paginate, render, take_lines


def _touch(root: Path, rel: str, n_lines: int) -> Path:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text("\n".join(f"L{i}" for i in range(n_lines)), encoding="utf-8")
    return p


def test_take_lines_inserts_marker_and_truncates(tmp_path):
    _touch(tmp_path, "a.go", 15)
    _touch(tmp_path, "b.go", 15)
    lines = take_lines(tmp_path, ["a.go", "b.go"], target=20)
    assert len(lines) == 20
    assert lines[0] == "// ===== a.go ====="
    assert any(l.startswith("// ===== b.go =====") for l in lines)
    assert lines[-1] == "// ... (文件续略)"


def test_take_lines_raises_on_shortfall(tmp_path):
    _touch(tmp_path, "a.go", 30)
    with pytest.raises(ValueError) as ei:
        take_lines(tmp_path, ["a.go"], target=100)
    assert "70" in str(ei.value)


def test_paginate_rejects_wrong_total():
    with pytest.raises(AssertionError):
        paginate(["x"] * 10, pages=2, per_page=6)


def test_render_builds_docx_with_header_and_breaks(tmp_path):
    out = tmp_path / "doc.docx"
    lines = [f"L{i}" for i in range(6)]  # 2 页 × 3 行
    render(paginate(lines, pages=2, per_page=3), out,
           name="alexGo-cloud 后台管理系统软件", version="V1.0")

    with zipfile.ZipFile(out) as z:
        doc = z.read("word/document.xml").decode("utf-8")
        headers = [n for n in z.namelist() if n.startswith("word/header")]
        header_xml = "".join(z.read(n).decode("utf-8") for n in headers)

    assert doc.count('w:type="page"') == 1, "2 页应恰有 1 个分页符"
    assert "L0" in doc and "L5" in doc
    assert "alexGo-cloud 后台管理系统软件" in header_xml
    assert "PAGE" in header_xml and "NUMPAGES" in header_xml
