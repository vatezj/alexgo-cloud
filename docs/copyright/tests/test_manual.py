import re
import zipfile
from pathlib import Path

from gen_manual import render
from manual_content import CHAPTERS

SHOTS = Path(__file__).resolve().parents[1] / "screenshots"
SHOT_LIST = Path(__file__).resolve().parents[1] / "shot_list.tsv"


def _mini_chapters(tmp_path: Path) -> list[dict]:
    """两图章节：一张真 PNG、一张缺失（触发占位符）。"""
    import base64
    real = tmp_path / "real.png"
    # 1×1 真实 PNG（python-docx 需解析头获取尺寸）
    real.write_bytes(base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
    ))
    return [
        {
            "title": "第一章",
            "sections": [
                {
                    "title": "1.1 节",
                    "steps": ["步骤一"],
                    "figures": [
                        {"file": "real.png", "caption": "真图", "path": str(real)},
                        {"file": "missing.png", "caption": "缺图", "path": str(tmp_path / "missing.png")},
                    ],
                }
            ],
        }
    ]


def test_missing_shot_renders_placeholder(tmp_path):
    out = tmp_path / "m.docx"
    render(_mini_chapters(tmp_path), out)
    with zipfile.ZipFile(out) as z:
        doc = z.read("word/document.xml").decode("utf-8")
    assert "截图待补" in doc
    assert "missing.png" in doc


def test_all_figures_rendered_as_images(tmp_path):
    out = tmp_path / "m.docx"
    render(_mini_chapters(tmp_path), out)
    with zipfile.ZipFile(out) as z:
        doc = z.read("word/document.xml").decode("utf-8")
    assert doc.count("<w:drawing>") == 1, "只有 1 张真图应有 1 个 drawing"


def test_figure_numbers_contiguous(tmp_path):
    out = tmp_path / "m.docx"
    render(_mini_chapters(tmp_path), out)
    with zipfile.ZipFile(out) as z:
        doc = z.read("word/document.xml").decode("utf-8")
    nums = [int(n) for n in re.findall(r"图 (\d+)", doc)]
    assert nums == list(range(1, len(nums) + 1)), f"图题编号不连续: {nums}"


def test_header_present(tmp_path):
    out = tmp_path / "m.docx"
    render(_mini_chapters(tmp_path), out)
    with zipfile.ZipFile(out) as z:
        headers = [n for n in z.namelist() if n.startswith("word/header")]
        header_xml = "".join(z.read(n).decode("utf-8") for n in headers)
    assert "alexGo-cloud 后台管理系统软件" in header_xml


def test_content_covers_spec_sections():
    titles = [c["title"] for c in CHAPTERS]
    for want in ["软件概述", "运行环境", "安装与启动", "通用功能", "功能模块操作", "常见问题"]:
        assert any(want in t for t in titles), f"缺章节: {want}"


def test_every_figure_file_in_shot_list():
    listed = {l.split("\t")[0] for l in SHOT_LIST.read_text(encoding="utf-8").splitlines() if l.strip()}
    used = set()
    for c in CHAPTERS:
        for s in c.get("sections", []):
            for f in s.get("figures", []):
                used.add(f["file"])
    assert used, "manual_content 没有引用任何图"
    assert used <= listed, f"引用了清单外的图: {used - listed}"
