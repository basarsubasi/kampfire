#!/usr/bin/env python3
"""
Phase 1: Rename Go module and update all Go import paths across the codebase.
Old module: campfire
New module: github.com/basarsubasi/kampfire
"""

import os
import re
import sys
from pathlib import Path

OLD_MODULE = "campfire"
NEW_MODULE = sys.argv[1] if len(sys.argv) > 1 else "github.com/basarsubasi/kampfire"

ROOT_DIR = Path(__file__).resolve().parent.parent

def update_go_mod(go_mod_path: Path):
    if not go_mod_path.exists():
        print(f"❌ {go_mod_path} does not exist!")
        return False

    content = go_mod_path.read_text(encoding="utf-8")
    new_content, count = re.subn(rf"^module\s+{re.escape(OLD_MODULE)}", f"module {NEW_MODULE}", content, flags=re.MULTILINE)
    if count > 0:
        go_mod_path.write_text(new_content, encoding="utf-8")
        print(f"✓ Updated {go_mod_path.relative_to(ROOT_DIR)}: module {OLD_MODULE} -> {NEW_MODULE}")
        return True
    else:
        print(f"ℹ️ {go_mod_path.relative_to(ROOT_DIR)} already had module {NEW_MODULE} or no match found.")
        return False

def update_go_files(root: Path):
    modified_files = 0
    total_replacements = 0

    # Pattern to match "campfire/..." inside import statements or strings
    pattern = re.compile(rf'"{re.escape(OLD_MODULE)}/([^"]+)"')

    for go_file in root.rglob("*.go"):
        # Skip vendor or hidden directories if any
        if any(p.startswith(".") or p == "vendor" for p in go_file.parts):
            continue

        content = go_file.read_text(encoding="utf-8")
        new_content, count = pattern.subn(f'"{NEW_MODULE}/\\1"', content)

        if count > 0:
            go_file.write_text(new_content, encoding="utf-8")
            print(f"✓ Updated {go_file.relative_to(ROOT_DIR)} ({count} import{'s' if count != 1 else ''})")
            modified_files += 1
            total_replacements += count

    return modified_files, total_replacements

def main():
    print("=" * 60)
    print(f"🚀 Phase 1: Renaming Go module to '{NEW_MODULE}'")
    print(f"📁 Repository Root: {ROOT_DIR}")
    print("=" * 60)

    go_mod = ROOT_DIR / "go.mod"
    update_go_mod(go_mod)

    mod_files, replacements = update_go_files(ROOT_DIR)

    print("-" * 60)
    print(f"🎉 Phase 1 Complete: {mod_files} Go files updated, {replacements} imports modified.")
    print("=" * 60)

if __name__ == "__main__":
    main()
