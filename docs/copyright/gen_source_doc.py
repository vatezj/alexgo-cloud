"""源代码文档生成器：60 页 × 50 行，页眉含软件全称+版本号+页码域（spec §6）。"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Sequence

from docx import Document
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_BREAK
from docx.oxml import OxmlElement
from docx.oxml.ns import qn
from docx.shared import Cm, Pt

from file_manifest import BACK_FILES, FRONT_FILES, SEGMENT_TARGET, build_manifest, count_lines

MARKER = "// ===== {path} ====="
TRUNCATED = "// ... (文件续略)"


def take_lines(root: Path, files: Sequence[str], target: int) -> list[str]:
    """按序读文件，每文件前插标记行；累计到 target；不足抛 ValueError。

    标记行计入 target 配额；短缺行数按源内容行计（不含标记行）。
    """
    lines: list[str] = []
    src_total = 0
    for rel in files:
        if len(lines) >= target:
            break
        p = root / rel
        if not p.is_file():
            continue
        lines.append(MARKER.format(path=rel))
        src = p.read_text(encoding="utf-8", errors="replace").splitlines()
        src_total += len(src)
        for ln in src:
            if len(lines) >= target:
                break
            lines.append(ln)
    if len(lines) < target:
        raise ValueError(f"清单行数不足：源内容 {src_total} 行，缺 {target - src_total} 行")
    if _cut_mid_file(root, lines):
        lines[-1] = TRUNCATED
    return lines[:target]


def _cut_mid_file(root: Path, lines: list[str]) -> bool:
    """最后一行是否落在某文件中途（该文件还有剩余行未收）。"""
    for i in range(len(lines) - 1, -1, -1):
        if lines[i].startswith("// ===== "):
            rel = lines[i][len("// ===== "): -len(" =====")]
            p = root / rel
            if p.is_file():
                src = p.read_text(encoding="utf-8", errors="replace").splitlines()
                consumed = len(lines) - i - 1
                return consumed < len(src)
            return False
    return False


def paginate(lines: Sequence[str], pages: int = 60, per_page: int = 50) -> list[list[str]]:
    assert len(lines) == pages * per_page, f"need {pages * per_page}, got {len(lines)}"
    return [list(lines[i * per_page:(i + 1) * per_page]) for i in range(pages)]


def _add_field(paragraph, instr: str) -> None:
    """插入 Word 域（PAGE / NUMPAGES）。"""
    fld = OxmlElement("w:fldSimple")
    fld.set(qn("w:instr"), instr)
    paragraph._p.append(fld)


def render(pages: Sequence[Sequence[str]], out: Path, name: str, version: str) -> None:
    doc = Document()

    section = doc.sections[0]
    section.top_margin = Cm(2.0)
    section.bottom_margin = Cm(2.0)
    section.left_margin = Cm(2.5)
    section.right_margin = Cm(2.5)

    # 页眉：左=全称+版本号；右=第 PAGE 页 / 共 NUMPAGES 页
    hp = section.header.paragraphs[0]
    hp.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = hp.add_run(f"{name} {version}\t第 ")
    run.font.size = Pt(9)
    _add_field(hp, "PAGE")
    r2 = hp.add_run(" 页 / 共 ")
    r2.font.size = Pt(9)
    _add_field(hp, "NUMPAGES")
    r3 = hp.add_run(" 页")
    r3.font.size = Pt(9)

    for pi, page in enumerate(pages):
        for text in page:
            p = doc.add_paragraph()
            p.paragraph_format.space_before = Pt(0)
            p.paragraph_format.space_after = Pt(0)
            p.paragraph_format.line_spacing = 1.0
            r = p.add_run(text)
            r.font.name = "Consolas"
            r._element.rPr.rFonts.set(qn("w:eastAsia"), "Consolas")
            r.font.size = Pt(9)
        if pi < len(pages) - 1:
            doc.add_paragraph().add_run().add_break(WD_BREAK.PAGE)

    out.parent.mkdir(parents=True, exist_ok=True)
    doc.save(out)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()
    root: Path = args.root.resolve()

    # 校验清单可凑满（同时打印统计）
    for label, files in (("FRONT", FRONT_FILES), ("BACK", BACK_FILES)):
        total = sum(count_lines(p) for p in build_manifest(root, files))
        if total < SEGMENT_TARGET:
            print(f"ERROR {label}={total} < {SEGMENT_TARGET}", file=sys.stderr)
            return 1

    front_lines = take_lines(root, FRONT_FILES, SEGMENT_TARGET)
    back_lines = take_lines(root, BACK_FILES, SEGMENT_TARGET)
    pages = paginate(front_lines + back_lines, pages=60, per_page=50)

    out: Path = args.out
    render(pages, out, name="alexGo-cloud 后台管理系统软件", version="V1.0")

    print(f"pages=60 lines={60 * 50} breaks=59")
    print(f"written: {out} ({out.stat().st_size} bytes)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
