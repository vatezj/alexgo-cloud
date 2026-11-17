#!/usr/bin/env python3
"""starfuyu 软文封面合成：cover_bg 去右下「豆包AI生成」水印 + 左上 logo + 日期（参考图风格）。

用法:
    python3 scripts/starfuyu_cover.py [YYYY-MM-DD] [-o 输出路径] [--bg 背景图] [--clean-only]
缺省日期=今天，输出=/tmp/starfuyu_cover_{YYYY-MM-DD}.png，背景=cover_bg.png。

清水印源机制（2026-11-17）：`--clean-only` 把背景去水印后存为 `{stem}.clean.png`
（如 cover_bg.clean.png）；合成时若 `.clean` 源存在则直接读取、跳过去水印步骤
——原始带水印文件保留作底稿，以后不再重复处理。

三步:
  1. 水印清除：右下白字区整块用左侧同排像素克隆覆盖（保留水彩色触感），mask 羽化过渡。
  2. 日期：Arial Rounded Bold 画数字、AlibabaPuHuiTi Bold 画「星期X」，
     多层描边仿参考图气泡字（投影 → 浅粉外描边 → 奶油填充），锚点中心定位。
  3. logo：缩放+圆角遮罩贴左上角（方形 JPEG 直贴会有硬边）。
"""
from __future__ import annotations

import argparse
import datetime as dt
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageFilter, ImageFont

ROOT = Path(__file__).resolve().parent.parent
COVER_BG = ROOT / "data" / "starfuyu" / "cover_bg.png"
LOGO = ROOT / "data" / "starfuyu" / "logo.jpg"
FONT_DIGIT = "/System/Library/Fonts/Supplemental/Arial Rounded Bold.ttf"
FONT_CJK = str(ROOT / "data" / "fonts" / "AlibabaPuHuiTi-3-85-Bold.ttf")

# 水印清除：右下白字块（实测 2140..2299 × 941..973，外扩），克隆源左移量。
WM_BOX = (2126, 926, 2304, 986)   # x0,y0,x1,y1
WM_CLONE_DX = -370                # 同排左移克隆
FEATHER = 12                      # mask 羽化半径

# 日期版式（参考图：1117 左中、星期二右中，气泡字）
DATE_COLOR_SHADOW = (232, 150, 150)   # 投影 深粉
DATE_COLOR_OUTLINE = (246, 186, 182)  # 外描边 浅粉
DATE_COLOR_FILL = (253, 244, 228)     # 填充 奶油
DATE_POS = {
    "digits": ((0.305, 0.40), 250),   # (中心比例 x,y), 字号
    "weekday": ((0.685, 0.615), 215),
}
DATE_SHADOW_OFFSET = (7, 9)
DATE_OUTLINE_R = 17                  # 外描边半径（圆周偏移画法）
RING_DIRS = 20                       # 圆周偏移份数

LOGO_SIZE = 168                      # 边长 px
LOGO_POS = (62, 96)                  # 视觉反馈 2026-11-17：再下来点（48→96）
LOGO_RADIUS = 75                     # 视觉反馈 2026-11-17：再圆点（34→75，近圆角方块）


def remove_watermark(im: Image.Image) -> Image.Image:
    """右下水印块克隆清除：源=同排左移区域，mask 羽化过渡。"""
    a = np.array(im)
    x0, y0, x1, y1 = WM_BOX
    donor = a[y0:y1, x0 + WM_CLONE_DX : x1 + WM_CLONE_DX].copy()
    assert donor.shape == (y1 - y0, x1 - x0, 3), donor.shape

    mask = Image.new("L", (x1 - x0, y1 - y0), 255)
    mask = mask.filter(ImageFilter.GaussianBlur(FEATHER))
    mask_a = np.array(mask)[:, :, None] / 255.0
    region = a[y0:y1, x0:x1].astype(np.float64)
    a[y0:y1, x0:x1] = (donor.astype(np.float64) * mask_a + region * (1 - mask_a)).astype(np.uint8)
    return Image.fromarray(a)


def stroke_text(base: Image.Image, xy: tuple[float, float], text: str, font: ImageFont.FreeTypeFont,
                shadow_dx: int = 0) -> None:
    """气泡字：投影 → 浅粉外描边（圆周偏移）→ 奶油填充，xy=锚点中心。"""
    layer = Image.new("RGBA", base.size, (0, 0, 0, 0))
    d = ImageDraw.Draw(layer)
    dx, dy = DATE_SHADOW_OFFSET
    d.text((xy[0] + dx + shadow_dx, xy[1] + dy), text, font=font,
           fill=(*DATE_COLOR_SHADOW, 230), anchor="mm")
    base.alpha_composite(layer)

    ring = Image.new("RGBA", base.size, (0, 0, 0, 0))
    dr = ImageDraw.Draw(ring)
    import math
    for i in range(RING_DIRS):
        ang = 2 * math.pi * i / RING_DIRS
        ox, oy = DATE_OUTLINE_R * math.cos(ang), DATE_OUTLINE_R * math.sin(ang)
        dr.text((xy[0] + ox, xy[1] + oy), text, font=font, fill=(*DATE_COLOR_OUTLINE, 255), anchor="mm")
    dr.text(xy, text, font=font, fill=(*DATE_COLOR_FILL, 255), anchor="mm")
    base.alpha_composite(ring)


def paste_logo(im: Image.Image) -> None:
    logo = Image.open(LOGO).convert("RGBA").resize((LOGO_SIZE, LOGO_SIZE), Image.Resampling.LANCZOS)
    mask = Image.new("L", (LOGO_SIZE, LOGO_SIZE), 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, LOGO_SIZE - 1, LOGO_SIZE - 1],
                                           radius=LOGO_RADIUS, fill=255)
    logo.putalpha(mask)
    im.alpha_composite(logo, LOGO_POS)


def clean_source(bg: Path) -> Path:
    """清水印源：{stem}.clean.png 存在即用；否则原图（合成时现场去水印）。"""
    cand = bg.with_name(bg.stem + ".clean" + bg.suffix)
    return cand if cand.exists() else bg


def compose(date: dt.date, bg: Path = COVER_BG) -> Image.Image:
    src = clean_source(bg)
    if src == bg:
        im = remove_watermark(Image.open(bg).convert("RGB")).convert("RGBA")
    else:
        im = Image.open(src).convert("RGBA")  # 已是清水印源，直接用

    digits = ImageFont.truetype(FONT_DIGIT, DATE_POS["digits"][1])
    weekday_font = ImageFont.truetype(FONT_CJK, DATE_POS["weekday"][1])
    (dcx, dcy), _ = DATE_POS["digits"]
    (wcx, wcy), _ = DATE_POS["weekday"]
    stroke_text(im, (dcx * im.width, dcy * im.height), date.strftime("%m%d"), digits)
    stroke_text(im, (wcx * im.width, wcy * im.height), "星期" + "一二三四五六日"[date.weekday()],
                weekday_font, shadow_dx=4)

    paste_logo(im)
    return im


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("date", nargs="?", default=dt.date.today().isoformat())
    ap.add_argument("-o", "--out", default=None)
    ap.add_argument("--bg", default=str(COVER_BG), help="背景图路径（默认 cover_bg.png）")
    ap.add_argument("--clean-only", action="store_true", help="只去水印存 {stem}.clean.png，不合成")
    args = ap.parse_args()
    bg = Path(args.bg)

    if args.clean_only:
        out_clean = bg.with_name(bg.stem + ".clean" + bg.suffix)
        cleaned = remove_watermark(Image.open(bg).convert("RGB"))
        cleaned.save(out_clean)
        # 复核：清水印产物右下角不应再有低饱和字形（粗检，人工看图为准）
        print(f"clean: {out_clean}")
        return 0

    date = dt.date.fromisoformat(args.date)
    out = Path(args.out) if args.out else Path(f"/tmp/starfuyu_cover_{date.isoformat()}.png")

    im = compose(date, bg)
    im.convert("RGB").save(out, quality=95)
    print(f"cover: {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
