#!/usr/bin/env python3
"""往 swag 生成的 spec 注入 x-tagGroups（Swagger UI 侧栏分组扩展）。

命名约定（docs/swagger-guide.md）：tag = 模块-侧，后缀 `-admin` / `-app`。
分组规则：
  - tag 以 `-admin` 结尾 → 「后台管理」组
  - tag 以 `-app` 结尾  → 「App 端」组
  - 其他 tag            → 忽略（不进侧栏分组，仍按 swag 默认列出）

注意：gin-swagger 运行时提供的是 docs.go 里内嵌的 docTemplate（编译进二进制），
swagger.json 只是给人看的——所以两处都注入，保持一致。swagger.yaml 不影响
运行时，不注入。
"""

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SWAGGER_DIR = ROOT / "docs" / "swagger"
GROUPS = [("后台管理", "-admin"), ("App 端", "-app")]


def operation_tags(spec: dict) -> set:
    tags = set()
    for ops in spec.get("paths", {}).values():
        for op in ops.values():
            if isinstance(op, dict):
                tags.update(op.get("tags", []) or [])
    return tags


def build_tag_groups(tags: set) -> list:
    groups = []
    for name, suffix in GROUPS:
        matched = sorted(t for t in tags if t.endswith(suffix))
        if matched:
            groups.append({"name": name, "tags": matched})
    return groups


def inject_json(path: Path, groups: list) -> None:
    spec = json.loads(path.read_text(encoding="utf-8"))
    spec["x-tagGroups"] = groups
    path.write_text(json.dumps(spec, ensure_ascii=False, indent=4) + "\n", encoding="utf-8")


def inject_docs_go(path: Path, groups: list) -> None:
    """docTemplate 是 Go 反引号 raw string 里的 JSON——把 x-tagGroups 插到最后一个 `}` 前。"""
    src = path.read_text(encoding="utf-8")
    m = re.search(r"const docTemplate = `(.*?)`", src, re.DOTALL)
    if not m:
        sys.exit("docs.go: 未找到 docTemplate 常量")
    tpl = m.group(1)
    inject = ',"x-tagGroups": ' + json.dumps(groups, ensure_ascii=False)
    close = tpl.rfind("}")
    if close == -1:
        sys.exit("docs.go: docTemplate 不是 JSON 形状")
    new_tpl = tpl[:close] + inject + tpl[close:]
    path.write_text(src[: m.start(1)] + new_tpl + src[m.end(1) :], encoding="utf-8")


def main() -> None:
    json_path = SWAGGER_DIR / "swagger.json"
    gengo = SWAGGER_DIR / "docs.go"
    if not json_path.exists() or not gengo.exists():
        sys.exit(f"先跑 swag init 生成 {SWAGGER_DIR} 下的文件")
    spec = json.loads(json_path.read_text(encoding="utf-8"))
    groups = build_tag_groups(operation_tags(spec))
    inject_json(json_path, groups)
    inject_docs_go(gengo, groups)
    print(f"injected x-tagGroups: {json.dumps(groups, ensure_ascii=False)}")


if __name__ == "__main__":
    main()
