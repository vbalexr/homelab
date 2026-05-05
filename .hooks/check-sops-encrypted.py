#!/usr/bin/env python3
"""
Block commits where YAML files contain plaintext values in 'data' or
'stringData' fields that should be encrypted by SOPS.

A valid SOPS-encrypted file must:
  1. Have a top-level 'sops:' key with encryption metadata.
  2. Have every value under 'data'/'stringData' start with 'ENC['.
"""
import re
import os
import sys

SOPS_META = re.compile(r"^sops:", re.MULTILINE)
SECRET_SECTION = re.compile(r"^(data|stringData):", re.MULTILINE)
SECRET_KIND = re.compile(r"^kind:\s*Secret\s*$", re.MULTILINE)
ENC_VALUE = re.compile(r"^ENC\[")
VALUE_LINE = re.compile(r"^([\w-]+):\s+(.+)$")


def check(path):
    with open(path) as f:
        content = f.read()

    # Only enforce encryption checks for Secret manifests.
    if not SECRET_KIND.search(content):
        return []

    if not SECRET_SECTION.search(content):
        return []

    errors = []

    if not SOPS_META.search(content):
        errors.append(f"{path}: has 'data'/'stringData' but no 'sops:' metadata — encrypt with SOPS first")
        return errors

    in_section = False
    section_indent = -1

    for lineno, line in enumerate(content.splitlines(), 1):
        stripped = line.lstrip()
        if not stripped or stripped.startswith("#"):
            continue
        indent = len(line) - len(stripped)

        if re.match(r"^(data|stringData):\s*$", stripped):
            in_section = True
            section_indent = indent
            continue

        if in_section:
            if stripped and indent <= section_indent:
                in_section = False
            else:
                m = VALUE_LINE.match(stripped)
                if m and not ENC_VALUE.match(m.group(2).strip()):
                    errors.append(
                        f"{path}:{lineno}: '{m.group(1)}' is not encrypted "
                        f"(value starts with: {m.group(2)[:30]!r})"
                    )

    return errors


def main():
    errors = []
    for path in sys.argv[1:]:
        if os.path.isfile(path):
            errors.extend(check(path))

    if errors:
        print("SOPS encryption check FAILED — plaintext secrets detected:\n")
        for e in errors:
            print(f"  {e}")
        print("\nEncrypt the file(s) before committing:")
        print("  sops --encrypt --in-place <file>")
        sys.exit(1)


if __name__ == "__main__":
    main()
