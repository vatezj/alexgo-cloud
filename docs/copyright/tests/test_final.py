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


# ---- 修复轮回归（评审 C1/C2/I1/I2/I3）----

# 必须呈现已加载数据的表格页（空表/加载态 std<5，有数据 std>=8）
TABLE_SHOTS = [
    "03-users.png", "04-roles.png", "05-menus.png", "06-depts.png",
    "08-dict.png", "09-configs.png", "10-notices.png", "12-tenants.png",
    "14-operate-logs.png", "15-online-users.png", "18-oauth2-clients.png",
    "19-social-apps.png", "20-order.png", "21-codegen.png",
]


def test_screenshots_all_unique():
    """I1：截图不允许字节级重复。"""
    import hashlib
    seen: dict[str, str] = {}
    for p in sorted((ROOT / "screenshots").glob("*.png")):
        h = hashlib.md5(p.read_bytes()).hexdigest()
        assert h not in seen, f"重复截图: {p.name} == {seen.get(h)}"
        seen[h] = p.name


def test_table_shots_show_loaded_data():
    """C1：表格页截图不得拍在加载态/空表上（区域像素 std 判别）。"""
    import numpy as np
    from PIL import Image

    for name in TABLE_SHOTS:
        p = ROOT / "screenshots" / name
        assert p.exists(), f"缺截图 {name}"
        im = np.asarray(Image.open(p).convert("L"), dtype=float)
        std = float(im[150:800, 250:1400].std())
        assert std >= 5.0, f"{name} 疑似加载态/空表 (std={std:.2f})"


def test_real_manual_captions_contiguous():
    """I3：真品手册图题 1..25 连续。"""
    doc = _xml(MAN, "word/document.xml")
    nums = [int(n) for n in re.findall(r"图 (\d+)", doc)]
    assert len(nums) == 25, f"图题数 {len(nums)} != 25"
    assert nums == list(range(1, 26)), f"图题不连续: {nums}"


def test_source_doc_includes_member_and_order():
    """I2：spec §2 要求 member/order 进册源码文档。"""
    doc = _xml(SRC, "word/document.xml")
    assert "modules/member/" in doc, "源码文档缺 member 模块"
    assert "modules/order/" in doc, "源码文档缺 order 模块"


# PII 打码区域：单一来源 mask_pii.RECTS（区域应近乎纯色）
from mask_pii import RECTS as PII_RECTS


def test_pii_columns_masked():
    """C2：PII 列已打码（区域内无亮字：max 亮度 < 80）。"""
    import numpy as np
    from PIL import Image

    for name, rects in PII_RECTS.items():
        im = np.asarray(Image.open(ROOT / "screenshots" / name).convert("L"), dtype=float)
        for r in rects:
            x1, y1, x2, y2 = r
            mx = float(im[y1:y2, x1:x2].max())
            assert mx < 80.0, f"{name} 区域 {r} 未打码 (max={mx:.0f})"
