#!/usr/bin/env python3
"""
Phase 2: Adjust all project references, documentation, CLI metadata, configs,
scripts, and tests to 'kampfire'.
"""

import os
import re
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent

def replace_in_file(file_path: Path, replacements: list) -> int:
    if not file_path.exists():
        print(f"⚠️ {file_path.relative_to(ROOT_DIR)} does not exist, skipping.")
        return 0

    content = file_path.read_text(encoding="utf-8")
    original = content
    total = 0

    for old, new in replacements:
        if isinstance(old, str):
            count = content.count(old)
            if count > 0:
                content = content.replace(old, new)
                total += count
        elif isinstance(old, re.Pattern):
            content, count = old.subn(new, content)
            total += count

    if content != original:
        file_path.write_text(content, encoding="utf-8")
        print(f"✓ Updated {file_path.relative_to(ROOT_DIR)} ({total} replacement{'s' if total != 1 else ''})")
        return total
    return 0

def adjust_cmd_files():
    print("\n📦 Updating CLI commands (cmd/)...")
    cmd_dir = ROOT_DIR / "cmd"
    for path in cmd_dir.glob("*.go"):
        replacements = [
            # Root command
            ('Use:   "campfire"', 'Use:   "kampfire"'),
            ('🔥 campfire', '🔥 kampfire'),
            ('campfire is a developer-first', 'kampfire is a developer-first'),
            ('path to campfire config file', 'path to kampfire config file'),
            ('Campfire Configuration (%s)', 'Kampfire Configuration (%s)'),
            # Command examples
            ('campfire run', 'kampfire run'),
            ('campfire ps', 'kampfire ps'),
            ('campfire exec', 'kampfire exec'),
            ('campfire rm', 'kampfire rm'),
            ('campfire cp', 'kampfire cp'),
            ('campfire port-forward', 'kampfire port-forward'),
            ('campfire forward', 'kampfire forward'),
            ('campfire pf', 'kampfire pf'),
            ('campfire ide', 'kampfire ide'),
            ('campfire config', 'kampfire config'),
            ('campfire -n', 'kampfire -n'),
            ('campfire --config', 'kampfire --config'),
            # Config / token references
            ('~/.config/campfire/config.json', '~/.config/kampfire/config.json'),
        ]
        replace_in_file(path, replacements)

    # Special handling for cmd/config.go token resolution
    cfg_go = cmd_dir / "config.go"
    if cfg_go.exists():
        content = cfg_go.read_text(encoding="utf-8")
        old_token_block = """\t\ttoken := os.Getenv("CAMPFIRE_API_TOKEN")
\t\ttokenSource := ""
\t\tif token != "" {
\t\t\ttokenSource = " " + ui.MutedStyle.Render("(from $CAMPFIRE_API_TOKEN)")
\t\t} else if cfg.Token != "" {"""
        new_token_block = """\t\ttoken := os.Getenv("KAMPFIRE_API_TOKEN")
\t\ttokenSource := ""
\t\tif token != "" {
\t\t\ttokenSource = " " + ui.MutedStyle.Render("(from $KAMPFIRE_API_TOKEN)")
\t\t} else if legacyToken := os.Getenv("CAMPFIRE_API_TOKEN"); legacyToken != "" {
\t\t\ttoken = legacyToken
\t\t\ttokenSource = " " + ui.MutedStyle.Render("(from $CAMPFIRE_API_TOKEN)")
\t\t} else if cfg.Token != "" {"""
        if old_token_block in content:
            content = content.replace(old_token_block, new_token_block)
            cfg_go.write_text(content, encoding="utf-8")
            print(f"✓ Enhanced token precedence in {cfg_go.relative_to(ROOT_DIR)}")

def adjust_pkg_files():
    print("\n📦 Updating Core Packages (pkg/)...")
    
    # pkg/config/config.go
    cfg_file = ROOT_DIR / "pkg" / "config" / "config.go"
    if cfg_file.exists():
        content = cfg_file.read_text(encoding="utf-8")
        old_default = """// DefaultConfigPath returns ~/.config/campfire/config.json.
func DefaultConfigPath() (string, error) {
\thome, err := os.UserHomeDir()
\tif err != nil {
\t\treturn "", fmt.Errorf("failed to get user home directory: %w", err)
\t}
\treturn filepath.Join(home, ".config", "campfire", "config.json"), nil
}"""
        new_default = """// DefaultConfigPath returns ~/.config/kampfire/config.json (or ~/.config/campfire/config.json if it exists).
func DefaultConfigPath() (string, error) {
\thome, err := os.UserHomeDir()
\tif err != nil {
\t\treturn "", fmt.Errorf("failed to get user home directory: %w", err)
\t}
\tkampfirePath := filepath.Join(home, ".config", "kampfire", "config.json")
\tif _, err := os.Stat(kampfirePath); err == nil {
\t\treturn kampfirePath, nil
\t}
\tcampfirePath := filepath.Join(home, ".config", "campfire", "config.json")
\tif _, err := os.Stat(campfirePath); err == nil {
\t\treturn campfirePath, nil
\t}
\treturn kampfirePath, nil
}"""
        if old_default in content:
            content = content.replace(old_default, new_default)
            content = content.replace("Campfire user configuration", "Kampfire user configuration")
            cfg_file.write_text(content, encoding="utf-8")
            print(f"✓ Updated DefaultConfigPath in {cfg_file.relative_to(ROOT_DIR)}")

    # pkg/k8s/client.go
    client_file = ROOT_DIR / "pkg" / "k8s" / "client.go"
    if client_file.exists():
        content = client_file.read_text(encoding="utf-8")
        old_token = """\t// Token override: check CAMPFIRE_API_TOKEN env var or config token
\ttoken := os.Getenv("CAMPFIRE_API_TOKEN")"""
        new_token = """\t// Token override: check KAMPFIRE_API_TOKEN / CAMPFIRE_API_TOKEN env var or config token
\ttoken := os.Getenv("KAMPFIRE_API_TOKEN")
\tif token == "" {
\t\ttoken = os.Getenv("CAMPFIRE_API_TOKEN")
\t}"""
        if old_token in content:
            content = content.replace(old_token, new_token)
            content = content.replace("Campfire configuration", "Kampfire configuration")
            client_file.write_text(content, encoding="utf-8")
            print(f"✓ Updated token override in {client_file.relative_to(ROOT_DIR)}")

    # pkg/sandbox/sandbox.go
    sb_file = ROOT_DIR / "pkg" / "sandbox" / "sandbox.go"
    if sb_file.exists():
        content = sb_file.read_text(encoding="utf-8")
        content = content.replace('"agents.x-k8s.io/created-by": "campfire"', '"agents.x-k8s.io/created-by": "kampfire"')
        sb_file.write_text(content, encoding="utf-8")
        print(f"✓ Updated created-by label in {sb_file.relative_to(ROOT_DIR)}")

    # pkg/transfer/cp.go
    cp_file = ROOT_DIR / "pkg" / "transfer" / "cp.go"
    if cp_file.exists():
        content = cp_file.read_text(encoding="utf-8")
        content = content.replace('"campfire-ssh-*"', '"kampfire-ssh-*"')
        cp_file.write_text(content, encoding="utf-8")
        print(f"✓ Updated temp prefix in {cp_file.relative_to(ROOT_DIR)}")

def adjust_build_and_scripts():
    print("\n📦 Updating Build & E2E Scripts...")
    
    # Makefile
    makefile = ROOT_DIR / "Makefile"
    replace_in_file(makefile, [
        ("bin/campfire", "bin/kampfire"),
    ])

    # scripts/e2e.sh
    e2e_sh = ROOT_DIR / "scripts" / "e2e.sh"
    replace_in_file(e2e_sh, [
        ('CLUSTER_NAME="campfire-e2e"', 'CLUSTER_NAME="kampfire-e2e"'),
        ('KUBECONFIG_FILE="/tmp/campfire-e2e-kubeconfig"', 'KUBECONFIG_FILE="/tmp/kampfire-e2e-kubeconfig"'),
        ('🔥 Campfire E2E Test Runner (KinD)', '🔥 Kampfire E2E Test Runner (KinD)'),
        ('go build -o bin/campfire .', 'go build -o bin/kampfire .'),
    ])

    # test/e2e/e2e_test.go
    e2e_test = ROOT_DIR / "test" / "e2e" / "e2e_test.go"
    if e2e_test.exists():
        content = e2e_test.read_text(encoding="utf-8")
        content = content.replace('campfireBin', 'kampfireBin')
        content = content.replace('filepath.Join(root, "bin", "campfire")', 'filepath.Join(root, "bin", "kampfire")')
        content = content.replace('campfire-user', 'kampfire-user')
        content = content.replace('runCampfire', 'runKampfire')
        content = content.replace('CAMPFIRE_API_TOKEN', 'KAMPFIRE_API_TOKEN')
        e2e_test.write_text(content, encoding="utf-8")
        print(f"✓ Updated {e2e_test.relative_to(ROOT_DIR)}")

def adjust_documentation():
    print("\n📦 Updating Documentation (README & docs/)...")
    
    # README.md
    readme = ROOT_DIR / "README.md"
    replace_in_file(readme, [
        ("# campfire", "# kampfire"),
        ("campfire", "kampfire"),
        ("Campfire", "Kampfire"),
        ("CAMPFIRE_API_TOKEN", "KAMPFIRE_API_TOKEN"),
        ("https://github.com/your-org/campfire.git", "https://github.com/basarsubasi/kampfire.git"),
        ("https://github.com/basarsubasi/campfire.git", "https://github.com/basarsubasi/kampfire.git"),
        ("https://github.com/basarsubasi/campfire", "https://github.com/basarsubasi/kampfire"),
    ])

    # docs/setup/*.md
    docs_dir = ROOT_DIR / "docs"
    for md_file in docs_dir.rglob("*.md"):
        replace_in_file(md_file, [
            ("campfire-user", "kampfire-user"),
            ("campfire", "kampfire"),
            ("Campfire", "Kampfire"),
            ("CAMPFIRE_API_TOKEN", "KAMPFIRE_API_TOKEN"),
            ("https://github.com/your-org/campfire", "https://github.com/basarsubasi/kampfire"),
            ("https://github.com/basarsubasi/campfire", "https://github.com/basarsubasi/kampfire"),
        ])

def main():
    print("=" * 60)
    print("🚀 Phase 2: Adjusting all project references to 'kampfire'")
    print(f"📁 Repository Root: {ROOT_DIR}")
    print("=" * 60)

    adjust_cmd_files()
    adjust_pkg_files()
    adjust_build_and_scripts()
    adjust_documentation()

    print("\n" + "=" * 60)
    print("🎉 Phase 2 Complete: All references updated to kampfire!")
    print("=" * 60)

if __name__ == "__main__":
    main()
