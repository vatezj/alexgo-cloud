"""截图 PII 打码：登录日志的用户名/IP/信息列与会员昵称列。

坐标与 tests/test_final.py 的 PII_RECTS 同源；重拍截图后重跑本脚本。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

from PIL import Image, ImageDraw

# (文件名, [(x1, y1, x2, y2), ...]) —— 纯色覆盖，坐标基于 1440×900 视口截图
# 13 的列边界由表头文字位置测定（用户名 488 / 用户ID 632 / IP 795 / 信息 1090 / 时间 1300）；
# 重拍时鼠标移出表格，无 tooltip，故无 tooltip 矩形。
RECTS: dict[str, list[tuple[int, int, int, int]]] = {
    "13-login-logs.png": [
        (458, 150, 630, 795),    # 用户名列（昵称）——止于用户ID列前
        (744, 150, 928, 795),    # IP 列——止于结果列前
        (986, 150, 1240, 795),   # 信息列（openid）——止于时间列前
    ],
    "16-member-search.png": [
        (595, 232, 855, 892),    # 会员昵称列——ID 列(525-548)与手机列(>863)保留
    ],
}

FILL = (40, 40, 40)  # 与深色主题表格底色接近


def mask(shots_dir: Path) -> list[str]:
    done = []
    for name, rects in RECTS.items():
        p = shots_dir / name
        if not p.is_file():
            print(f"SKIP missing {name}", file=sys.stderr)
            continue
        im = Image.open(p).convert("RGB")
        draw = ImageDraw.Draw(im)
        for r in rects:
            draw.rectangle(r, fill=FILL)
        im.save(p, "PNG")
        done.append(name)
    return done


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--shots", type=Path, default=Path("screenshots"))
    args = ap.parse_args()
    done = mask(args.shots)
    print(f"masked: {', '.join(done)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
