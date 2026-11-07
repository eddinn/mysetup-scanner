#!/usr/bin/env python3
"""Checks a report against docs/report-format.md: python3 tests/validate_report.py report.txt"""

from __future__ import annotations

import re
import sys

WORD = r"[A-Za-z0-9._+/-]+"
TEXT = r"[ -~]{1,120}"
WHOLE = r"[1-9][0-9]*"
PCI = r"[0-9a-f]{4}:[0-9a-f]{4}"
CAPABILITIES = "cpu|gpu|motherboard|memory_modules|configured_memory|storage|displays"

FIELDS: list[tuple[str, str]] = [
    (r"source", r"(script-(sh|ps1)|app)/[0-9]+\.[0-9]+\.[0-9]+"),
    (r"tested", r"yes|no"),
    (r"os\.family", r"linux|macos|windows|freebsd|openbsd|netbsd|illumos|other"),
    (r"os\.name", WORD),
    (r"os\.version", WORD),
    (r"os\.architecture", r"x86-64|arm64|x86|arm|riscv64|ppc64le|ppc64|s390x|other"),
    (r"environment", r"physical|virtual_machine|container|wsl|unknown"),
    (r"cpu\.(vendor|model)", TEXT),
    (r"cpu\.(cores|threads)", WHOLE),
    (r"gpu\.[0-9]+\.(pci|subsystem)", PCI),
    (r"gpu\.[0-9]+\.(vendor|model)", TEXT),
    (r"motherboard\.(vendor|model)", TEXT),
    (r"memory\.total_mb", WHOLE),
    (r"memory\.[0-9]+\.(size_mb|speed_mts|configured_speed_mts)", WHOLE),
    (r"memory\.[0-9]+\.(maker|part)", TEXT),
    (r"memory\.[0-9]+\.(type|form)", WORD),
    (r"storage\.[0-9]+\.model", TEXT),
    (r"storage\.[0-9]+\.size_bytes", WHOLE),
    (r"storage\.[0-9]+\.interface", WORD),
    (r"storage\.[0-9]+\.media", r"ssd|hdd"),
    (rf"capability\.({CAPABILITIES})", r"available|unavailable"),
    (rf"capability\.({CAPABILITIES})\.reason", r"needs-root|not-exposed|no-source|not-collected"),
]

# Shapes no hardware name has. docs/report-format.md: never carried.
PROHIBITED = [
    ("a MAC address", re.compile(r"\b[0-9a-f]{2}([:-][0-9a-f]{2}){5}\b", re.I)),
    ("a UUID", re.compile(r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", re.I)),
    ("an IP address", re.compile(r"\b[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\b")),
    ("an email address", re.compile(r"[^ ]@[^ ]")),
    ("a file path", re.compile(r"/home/|/users/|c:\\", re.I)),
]

MAX_ITEMS = 64


def report_lines(text: str) -> list[str] | None:
    """The lines between the markers, or None when the report is cut short. Notes around them are ignored."""
    lines = text.replace("\r\n", "\n").split("\n")
    try:
        start = lines.index("mysetup-report 1")
        end = lines.index("mysetup-report-end", start)
    except ValueError:
        return None
    return lines[start + 1 : end]


def problems(text: str) -> list[str]:
    lines = report_lines(text)
    if lines is None:
        return ["the report has no 'mysetup-report 1' or no 'mysetup-report-end' line"]

    found: list[str] = []
    seen: dict[str, str] = {}
    for line in lines:
        name, separator, value = line.partition("=")
        if not separator:
            found.append(f"not a field: {line!r}")
            continue
        if name in seen:
            found.append(f"{name} appears twice")
        seen[name] = value
        for pattern, shape in FIELDS:
            if re.fullmatch(pattern, name):
                if not re.fullmatch(shape, value):
                    found.append(f"{name} has a value it may not have: {value!r}")
                break
        else:
            found.append(f"no such field: {name}")
        for label, shape in PROHIBITED:
            if shape.search(value):
                found.append(f"{name} looks like {label}: {value!r}")

    for required in ("source", "tested", "os.family", "os.architecture", "environment"):
        if required not in seen:
            found.append(f"{required} is missing")

    for part in ("gpu", "memory", "storage"):
        numbers = sorted({int(match.group(1)) for name in seen if (match := re.fullmatch(rf"{part}\.([0-9]+)\..+", name))})
        if numbers != list(range(len(numbers))):
            found.append(f"{part} numbers have gaps: {numbers}")
        if len(numbers) > MAX_ITEMS:
            found.append(f"more than {MAX_ITEMS} of {part}")

    for name, value in seen.items():
        if re.fullmatch(rf"capability\.({CAPABILITIES})", name):
            has_reason = f"{name}.reason" in seen
            if value == "unavailable" and not has_reason:
                found.append(f"{name} is unavailable without a reason")
            if value == "available" and has_reason:
                found.append(f"{name} is available but has a reason")
    return found


def main(arguments: list[str]) -> int:
    if len(arguments) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    with open(arguments[1], encoding="utf-8", errors="replace", newline="") as handle:
        found = problems(handle.read())
    for problem in found:
        print(problem)
    if not found:
        print("ok")
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
