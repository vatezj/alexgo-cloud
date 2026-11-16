#!/usr/bin/env python3
"""starfuyu cute 素材抠图：四边连通去纯色底 + 清右下水印 + alpha bbox 裁边。

一次性脚本（spec 2026-11-16-starfuyu-poster-title-photo §3）：
  输入 data/starfuyu/cute/cute_{slug}_solid_bg_1.png（1536×1536 纯色奶油底 + 右下水印）
  输出 data/starfuyu/cute/cutout/cute_{slug}_cut.png（RGBA、bbox 外扩 8px 透明边）

算法要点：
  - 底色种子 = 四角(2px 内缩)均值；候选底 mask = 逐通道 max 绝对差 ≤ 26。
  - 只删「连通到边」的底色（scipy.ndimage.label 取贴边标签）——角色内部同色不穿洞。
  - 水印：前景 4 连通域中整块 bbox 落在 x>0.70w 且 y>0.90h 的置透明
    （字形独立于角色；角色 bbox 不可能整块落入该区）。
  - 每张打印自检行；末尾拼 4×3 联系表 /tmp/starfuyu_cutout_sheet.png 供人工验收。
"""
from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw, ImageFont
from scipy import ndimage

ROOT = Path(__file__).resolve().parent.parent
SRC_DIR = ROOT / "data" / "starfuyu" / "cute"
OUT_DIR = SRC_DIR / "cutout"
SHEET_PATH = Path("/tmp/starfuyu_cutout_sheet.png")

TOL = 26                      # 逐通道 max 绝对差 ≤ TOL 视为候选底色
                               # （26 覆盖 gemini 背景淡粉晕 max≈24；角色内部同色
                               #  被描边隔断、不贴边连通，结构性不穿洞）
PAD = 8                       # alpha bbox 外扩透明边
WM_X, WM_Y = 0.70, 0.90       # 水印区：x > WM_X*w 且 y > WM_Y*h
MIN_FG = 300_000              # 自检：前景像素下限
CELL = 320                    # 联系表格边长
COLS = 4
SHEET_BG = (255, 230, 235)


def cut_one(src: Path) -> tuple[Image.Image, dict]:
    """单张抠图，返回 (RGBA 成品, 自检统计)。"""
    im = Image.open(src).convert("RGBA")
    arr = np.asarray(im)
    h, w = arr.shape[:2]
    rgb = arr[:, :, :3].astype(np.int16)

    # 底色种子：四角 2px 内缩处各取 1 像素均值。
    corners = np.array(
        [rgb[1, 1], rgb[1, w - 2], rgb[h - 2, 1], rgb[h - 2, w - 2]], dtype=np.float64
    )
    seed = corners.mean(axis=0)
    bg_cand = np.abs(rgb - seed).max(axis=2) <= TOL

    # 连通域：贴边标签 = 背景（只删连通到边的底色）。
    labels, n = ndimage.label(bg_cand, structure=np.array([[0, 1, 0], [1, 1, 1], [0, 1, 0]]))
    border_labels = np.unique(
        np.concatenate([labels[0, :], labels[-1, :], labels[:, 0], labels[:, -1]])
    )
    border_labels = border_labels[border_labels > 0]
    background = np.isin(labels, border_labels)
    fg = ~background

    # 水印：前景连通域中整块落入右下水印区的置透明。
    fg_labels, fn = ndimage.label(fg, structure=np.array([[0, 1, 0], [1, 1, 1], [0, 1, 0]]))
    removed = 0
    if fn:
        boxes = ndimage.find_objects(fg_labels)
        drop = np.zeros(fn + 1, dtype=bool)
        for i, sl in enumerate(boxes, start=1):
            if sl is None:
                continue
            ys, xs = sl
            if xs.start > WM_X * w and ys.start > WM_Y * h:
                drop[i] = True
                removed += 1
        if removed:
            fg = fg & ~drop[fg_labels]

    alpha = np.where(fg, 255, 0).astype(np.uint8)

    # 自检：水印区残余 = 区内非「最大连通域」的不透明像素。
    # 角色本体伸入水印区（钳/靴边）属正常内容（主域）；水印字形与
    # 背景粉斑是独立小域——清除后应为 0。
    wm_zone = np.zeros((h, w), dtype=bool)
    wm_zone[int(WM_Y * h) :, int(WM_X * w) :] = True
    comp_labels, comp_n = ndimage.label(fg, structure=np.array([[0, 1, 0], [1, 1, 1], [0, 1, 0]]))
    if comp_n:
        comp_sizes = ndimage.sum(fg, comp_labels, range(1, comp_n + 1))
        main_label = int(np.argmax(comp_sizes)) + 1
        wm_residual = int((fg & wm_zone & (comp_labels != main_label)).sum())
    else:
        wm_residual = 0

    # alpha bbox 外扩 PAD 裁边。
    ys, xs = np.where(fg)
    y0, y1 = max(int(ys.min()) - PAD, 0), min(int(ys.max()) + 1 + PAD, h)
    x0, x1 = max(int(xs.min()) - PAD, 0), min(int(xs.max()) + 1 + PAD, w)
    out_arr = arr[y0:y1, x0:x1].copy()
    out_arr[:, :, 3] = alpha[y0:y1, x0:x1]
    out = Image.fromarray(out_arr, "RGBA")

    stats = {
        "fg_px": int(fg.sum()),
        "bbox": (x0, y0, x1, y1),
        "removed": removed,
        "wm_residual": wm_residual,
    }
    return out, stats


def build_sheet(cutouts: list[tuple[str, Image.Image]]) -> None:
    """4×3 联系表：粉底、每格缩放居中、slug 标注。"""
    rows = (len(cutouts) + COLS - 1) // COLS
    pad, label_h = 12, 28
    cell_w = CELL + pad * 2
    cell_h = CELL + pad * 2 + label_h
    sheet = Image.new("RGB", (cell_w * COLS, cell_h * rows), SHEET_BG)
    draw = ImageDraw.Draw(sheet)
    try:
        font = ImageFont.truetype(
            "/System/Library/Fonts/Supplemental/Arial.ttf", 22
        )
    except OSError:
        font = ImageFont.load_default()
    for idx, (slug, img) in enumerate(cutouts):
        r, c = divmod(idx, COLS)
        thumb = img.copy()
        thumb.thumbnail((CELL, CELL), Image.Resampling.LANCZOS)
        cell_x, cell_y = c * cell_w, r * cell_h
        px = cell_x + pad + (CELL - thumb.width) // 2
        py = cell_y + pad + (CELL - thumb.height) // 2
        sheet.paste(thumb, (px, py), thumb)
        tw = draw.textlength(slug, font=font)
        draw.text(
            (cell_x + (cell_w - tw) // 2, cell_y + pad * 2 + CELL),
            slug,
            fill=(90, 60, 60),
            font=font,
        )
    sheet.save(SHEET_PATH)


def main() -> int:
    srcs = sorted(SRC_DIR.glob("cute_*_solid_bg_1.png"))
    if not srcs:
        print(f"ERROR: 无源图 {SRC_DIR}/cute_*_solid_bg_1.png", file=sys.stderr)
        return 1
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    cutouts: list[tuple[str, Image.Image]] = []
    bad = 0
    for src in srcs:
        slug = src.name[len("cute_") : -len("_solid_bg_1.png")]
        out, st = cut_one(src)
        dst = OUT_DIR / f"cute_{slug}_cut.png"
        out.save(dst)
        ok_fg = st["fg_px"] > MIN_FG
        ok_wm = st["wm_residual"] == 0
        flag = "" if (ok_fg and ok_wm) else "  <-- CHECK"
        if flag:
            bad += 1
        print(
            f"{slug:12s} fg={st['fg_px']:>8d} bbox={st['bbox']} "
            f"removed={st['removed']} wm_residual={st['wm_residual']}{flag}"
        )
        cutouts.append((slug, out))
    build_sheet(cutouts)
    print(f"sheet: {SHEET_PATH}  ({len(cutouts)} 张, 待查 {bad})")
    return 0 if bad == 0 else 2


if __name__ == "__main__":
    sys.exit(main())
