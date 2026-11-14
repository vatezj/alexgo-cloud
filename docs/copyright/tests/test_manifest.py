from pathlib import Path

from file_manifest import build_manifest, count_lines


def _touch(root: Path, rel: str, lines: int = 3) -> Path:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text("\n".join(f"line{i}" for i in range(lines)), encoding="utf-8")
    return p


def test_excludes_test_generated_and_config(tmp_path):
    kept = _touch(tmp_path, "pkg/resp/resp.go")
    _touch(tmp_path, "pkg/resp/resp_test.go")
    _touch(tmp_path, "src/auto-imports.d.ts")
    _touch(tmp_path, "src/components.d.ts")
    _touch(tmp_path, "node_modules/foo/index.js")
    _touch(tmp_path, "dist/bundle.js")
    _touch(tmp_path, "alexgo-server/configs/config.yaml")

    result = build_manifest(tmp_path, [
        "pkg/resp/resp.go",
        "pkg/resp/resp_test.go",
        "src/auto-imports.d.ts",
        "src/components.d.ts",
        "node_modules/foo/index.js",
        "dist/bundle.js",
        "alexgo-server/configs/config.yaml",
    ])
    assert result == [kept]


def test_missing_file_skipped(tmp_path):
    kept = _touch(tmp_path, "a.go")
    result = build_manifest(tmp_path, ["a.go", "does/not/exist.go"])
    assert result == [kept]


def test_count_lines_matches_splitlines(tmp_path):
    p = _touch(tmp_path, "a.go", lines=7)
    assert count_lines(p) == len(p.read_text(encoding="utf-8").splitlines())


def test_build_manifest_preserves_order(tmp_path):
    _touch(tmp_path, "z.go")
    _touch(tmp_path, "a.go")
    _touch(tmp_path, "m.go")
    result = build_manifest(tmp_path, ["z.go", "a.go", "m.go"])
    assert [p.name for p in result] == ["z.go", "a.go", "m.go"]
