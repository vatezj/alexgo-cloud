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
