"""操作手册生成器：渲染 manual_content.CHAPTERS → docx（spec §7）。"""
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

from manual_content import CHAPTERS

PLACEHOLDER = "（截图待补：{file}）"
FIG_CAPTION = "图 {n}：{caption}"


def _add_field(paragraph, instr: str) -> None:
    fld = OxmlElement("w:fldSimple")
    fld.set(qn("w:instr"), instr)
    paragraph._p.append(fld)


def _setup_header(doc: Document, name: str, version: str) -> None:
    hp = doc.sections[0].header.paragraphs[0]
    hp.alignment = WD_ALIGN_PARAGRAPH.LEFT
    run = hp.add_run(f"{name} {version}\t第 ")
    run.font.size = Pt(9)
    _add_field(hp, "PAGE")
    r2 = hp.add_run(" 页 / 共 ")
    r2.font.size = Pt(9)
    _add_field(hp, "NUMPAGES")
    r3 = hp.add_run(" 页")
    r3.font.size = Pt(9)


def _add_figure(doc: Document, fig: dict, number: int) -> int:
    """插入图片或占位符 + 图题；返回新编号（占位也占号，保证连续）。"""
    path = Path(fig["path"])
    if path.is_file():
        p = doc.add_paragraph()
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        p.add_run().add_picture(str(path), width=Cm(15.5))
    else:
        ph = doc.add_paragraph()
        ph.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r = ph.add_run(PLACEHOLDER.format(file=fig["file"]))
        r.italic = True
    cap = doc.add_paragraph()
    cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
    cr = cap.add_run(FIG_CAPTION.format(n=number, caption=fig["caption"]))
    cr.font.size = Pt(9)
    return number + 1


def render(chapters: Sequence[dict], out: Path,
           name: str = "alexGo-cloud 后台管理系统软件", version: str = "V1.0") -> dict:
    doc = Document()
    section = doc.sections[0]
    section.top_margin = Cm(2.5)
    section.bottom_margin = Cm(2.5)
    section.left_margin = Cm(2.8)
    section.right_margin = Cm(2.8)
    _setup_header(doc, name, version)

    fig_no = 1
    figures = 0
    missing = 0

    for ci, chapter in enumerate(chapters):
        if ci > 0:
            doc.add_paragraph().add_run().add_break(WD_BREAK.PAGE)
        h1 = doc.add_heading(chapter["title"], level=1)
        for sec in chapter.get("sections", []):
            doc.add_heading(sec["title"], level=2)
            for para in sec.get("paragraphs", []):
                doc.add_paragraph(para)
            for i, step in enumerate(sec.get("steps", []), 1):
                doc.add_paragraph(f"{i}. {step}")
            table_data = sec.get("table")
            if table_data:
                t = doc.add_table(rows=1, cols=len(table_data["headers"]))
                t.style = "Table Grid"
                for i, h in enumerate(table_data["headers"]):
                    t.rows[0].cells[i].text = h
                for row in table_data["rows"]:
                    cells = t.add_row().cells
                    for i, val in enumerate(row):
                        cells[i].text = val
            for fig in sec.get("figures", []):
                fig_no = _add_figure(doc, fig, fig_no)
                figures += 1
                if not Path(fig["path"]).is_file():
                    missing += 1

    out.parent.mkdir(parents=True, exist_ok=True)
    doc.save(out)
    return {"figures": figures, "missing": missing}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--shots", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--name", default="alexGo-cloud 后台管理系统软件")
    ap.add_argument("--version", default="V1.0")
    args = ap.parse_args()

    # 把相对 file 解析到 shots 目录
    chapters = []
    for ch in CHAPTERS:
        ch = dict(ch)
        secs = []
        for sec in ch.get("sections", []):
            sec = dict(sec)
            figs = []
            for f in sec.get("figures", []):
                f = dict(f)
                f["path"] = str(args.shots / f["file"])
                figs.append(f)
            sec["figures"] = figs
            secs.append(sec)
        ch["sections"] = secs
        chapters.append(ch)

    stats = render(chapters, args.out, name=args.name, version=args.version)
    print(f"figures={stats['figures']} missing={stats['missing']}")
    print(f"written: {args.out} ({args.out.stat().st_size} bytes)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
