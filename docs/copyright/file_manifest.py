"""进册文件清单：源码文档选材（spec §6.2）。

顺序即叙事：前段「启动 → 中间件/横切 → 核心 pkg」，
后段「system 代表文件 → 关键服务 → admin 前端 → member/order 顺位池」。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path, PurePosixPath
from typing import Sequence

EXCLUDE_PARTS = ("node_modules", "dist")
EXCLUDE_SUFFIXES = ("_test.go", ".d.ts")
EXCLUDE_NAMES = ("config.yaml",)


def is_excluded(rel: str) -> bool:
    parts = PurePosixPath(rel).parts
    name = parts[-1]
    if name in EXCLUDE_NAMES:
        return True
    if any(p in EXCLUDE_PARTS for p in parts):
        return True
    return any(name.endswith(s) for s in EXCLUDE_SUFFIXES)


def count_lines(path: Path) -> int:
    return len(path.read_text(encoding="utf-8", errors="replace").splitlines())


def build_manifest(root: Path, files: Sequence[str]) -> list[Path]:
    result: list[Path] = []
    for rel in files:
        if is_excluded(rel):
            continue
        p = root / rel
        if p.is_file():
            result.append(p)
    return result


# 前 30 页候选（≥1500 行）
FRONT_FILES: list[str] = [
    "alexgo-server/cmd/main.go",
    "pkg/middleware/middleware.go",
    "pkg/middleware/auth.go",
    "pkg/middleware/tenant.go",
    "pkg/middleware/access_log.go",
    "pkg/middleware/operate_log.go",
    "pkg/middleware/ratelimit_breaker.go",
    "pkg/limiter/rate_limiter.go",
    "pkg/trace/trace.go",
    "pkg/resp/resp.go",
    "pkg/resp/page.go",
    "pkg/errors/code.go",
    "pkg/errors/errors.go",
    "pkg/auth/auth.go",
    "pkg/auth/jwt.go",
    "pkg/auth/casbin.go",
    "pkg/tenant/tenant.go",
    "pkg/tenant/account.go",
    "pkg/tenant/datascope.go",
    "pkg/tenant/domain.go",
]

# 后 30 页候选（≥1500 行）。
# 顺序即叙事：system controller+service 代表 → member → order → admin 关键页。
# spec §6.2 要求 admin 登录/布局/用户管理页、§2 要求 member/order 进册，
# 故三者必须落在 1500 截断线内；basic.go/rbac.go 等大文件列后作顺位池。
BACK_FILES: list[str] = [
    "modules/system/controller/admin/user.go",        # system controller 代表
    "modules/system/service/datascope.go",            # system service 代表（数据权限）
    "modules/member/service/member.go",
    "modules/member/controller/admin/member_user.go",
    "modules/order/controller/admin/controller.go",
    "modules/order/service/order.go",
    "frontend/admin/apps/web-antd/src/views/_core/authentication/login.vue",
    "frontend/admin/apps/web-antd/src/layouts/basic.vue",
    "frontend/admin/apps/web-antd/src/views/system/SystemUsersPage.vue",
    # 顺位池（1500 之后兜底，缺文件时前移补位）
    "modules/system/controller/admin/auth.go",
    "modules/system/controller/admin/menu.go",
    "modules/system/controller/admin/basic.go",
    "modules/system/service/rbac.go",
    "modules/system/service/permission_routes.go",
    "frontend/admin/apps/web-antd/src/views/system/SystemRolesPage.vue",
    "frontend/admin/apps/web-antd/src/views/system/SystemOnlineUsersPage.vue",
]

SEGMENT_TARGET = 1500  # 30 页 × 50 行


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True, type=Path)
    args = ap.parse_args()
    root: Path = args.root.resolve()

    exit_code = 0
    for label, files in (("FRONT", FRONT_FILES), ("BACK", BACK_FILES)):
        paths = build_manifest(root, files)
        total = 0
        print(f"== {label} ==")
        for p in paths:
            n = count_lines(p)
            total += n
            print(f"{n:6d}  ({total // 50:3d}p)  {p.relative_to(root)}")
        ok = total >= SEGMENT_TARGET
        print(f"{label}={total} lines (>={'1500' if ok else '1500'}) {'OK' if ok else 'SHORT'}")
        if not ok:
            exit_code = 1
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
