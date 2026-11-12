#!/usr/bin/env python3
"""Prepare a local Zed extension without requiring access to a remote grammar."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tomllib


def prepare(output: Path) -> Path:
    extension = Path(__file__).resolve().parent
    root = extension.parent.parent
    manifest = (extension / "extension.toml").read_text()
    revision = tomllib.loads(manifest)["grammars"]["zhex"]["rev"]
    subprocess.run(
        ["git", "-C", str(root), "cat-file", "-e", f"{revision}:editors/tree-sitter-zhex/src/parser.c"],
        check=True,
    )
    remote = 'repository = "https://github.com/desireco/zhex"'
    marker = "[grammars.zhex]"
    metadata, grammar = manifest.split(marker, 1)
    grammar = grammar.replace(remote, "repository = " + json.dumps(root.as_uri()), 1)
    output.mkdir(parents=True, exist_ok=True)
    (output / "extension.toml").write_text(metadata + marker + grammar)
    shutil.copytree(extension / "languages", output / "languages", dirs_exist_ok=True)
    return output.resolve()


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    if len(sys.argv) > 2:
        raise SystemExit("usage: python3 editors/zed/prepare.py [output-directory]")
    output = Path(sys.argv[1]) if len(sys.argv) == 2 else root / "dist/editors/zed"
    print(prepare(output))
