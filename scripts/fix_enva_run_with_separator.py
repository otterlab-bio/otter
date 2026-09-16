#!/usr/bin/env python3
"""
Fix enva run commands in Snakemake files by adding -- separator.

This script adds the -- separator after the environment name to prevent
clap from misinterpreting command flags as enva flags.

Before: enva run fastqc fastqc -o {params.dir} -t {threads} --extract
After:  enva run fastqc -- fastqc -o {params.dir} -t {threads} --extract
"""

import re
import sys
from pathlib import Path


def fix_enva_run_line(line):
    """
    Fix enva run commands by adding -- separator after environment name.

    Patterns to fix:
    - enva run <env> <cmd> -flag <value> ...
    + enva run <env> -- <cmd> -flag <value> ...

    Skip if:
    - Already has -- separator
    - Uses --command flag
    - Is a comment
    - Command part is already quoted
    """

    # Skip if line is a comment
    if line.strip().startswith('#'):
        return line

    # Skip if already uses --command flag
    if '--command' in line:
        return line

    # Pattern: enva run <env_name> <command> [flags...]
    # We want to insert -- after <env_name>
    # Pattern matches: enva run word word [more words...]
    pattern = r'(enva run\s+)([a-z_]+)(\s+)([a-z]+[a-z0-9/_\-\.]*)(\s+)'

    def replacement(match):
        prefix = match.group(1)  # "enva run "
        env_name = match.group(2)  # environment name
        space1 = match.group(3)  # " "
        command_name = match.group(4)  # command name (e.g., "fastqc")
        space2 = match.group(5)  # " "

        # Check if -- separator already exists
        remaining_line = match.group(0)[match.end():]
        if remaining_line.strip().startswith('--'):
            return match.group(0)  # Skip, already has --

        # Check if command is already quoted
        if command_name.startswith('"'):
            return match.group(0)  # Skip, already using quotes

        return f'{prefix}{env_name}{space1}--{space2}{command_name}{space2}'

    return re.sub(pattern, replacement, line)


def fix_file(filepath):
    """Fix all enva run commands in a file."""
    with open(filepath, 'r') as f:
        lines = f.readlines()

    fixed_lines = []
    modified = False

    for line in lines:
        fixed_line = fix_enva_run_line(line)
        if fixed_line != line:
            modified = True
            print(f"  Line changed: {line.strip()[:60]}...")
            print(f"  → {fixed_line.strip()[:60]}...")
        fixed_lines.append(fixed_line)

    if modified:
        # Backup original
        backup_path = filepath.with_suffix('.smk.bak')
        filepath.rename(backup_path)

        # Write fixed version
        with open(filepath, 'w') as f:
            f.writelines(fixed_lines)

        print(f"✓ Fixed: {filepath}")
        return True
    else:
        print(f"- Skipped: {filepath} (no changes needed)")
        return False


def main():
    """Main entry point."""
    # Support command-line argument for single file testing
    if len(sys.argv) > 1:
        test_file = Path(sys.argv[1])
        if not test_file.exists():
            print(f"Error: File not found: {test_file}")
            return 1

        print(f"Testing on single file: {test_file}")
        fix_file(test_file)
        return 0

    directories = [
        'inst/root_rules',
        'inst/rootless_rules',
        '/data_center_01/home/zhengyanhua/beaverflow-go/rules',  # Added beaverflow-go
    ]

    total_files = 0
    fixed_files = 0

    for directory in directories:
        dir_path = Path(directory)
        if not dir_path.exists():
            print(f"⚠ Skipping non-existent directory: {directory}")
            continue

        print(f"\n📁 Processing {directory}...")

        for smk_file in dir_path.glob('*.smk'):
            total_files += 1
            if fix_file(smk_file):
                fixed_files += 1

    print()
    print("=" * 50)
    print(f"Summary:")
    print(f"  Total files processed: {total_files}")
    print(f"  Files fixed: {fixed_files}")
    print(f"  Files unchanged: {total_files - fixed_files}")
    print("=" * 50)

    if fixed_files > 0:
        print()
        print("✓ Fix complete!")
        print()
        print("Next steps:")
        print("  1. Review changes with: git diff")
        print("  2. Test with: enva run fastqc -- fastqc --version")
        print("  3. If satisfied, remove .bak files:")
        print("     find . -name '*.smk.bak' -delete")

    return 0


if __name__ == '__main__':
    sys.exit(main())
