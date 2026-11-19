#!/usr/bin/env python3
"""Deterministic input generators for the cross-language suite (#190).

Both fixtures come from one seeded random.Random(190), so every machine
and every run produces byte-identical files. The wordcount corpus is
single-space-separated words with no blank fields; the filterstream
fixture never contains the Zhex EOF sentinel "~EOF~" (see
benchmarks/filterstream/filterstream.zhex).
"""

import pathlib
import random
import sys

TARGET_BYTES = 2_000_000
FILTER_LINES = 60_000

BASES = [
    "alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
    "india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
    "quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey",
    "xray", "yankee", "zulu", "amber", "basalt", "cobalt", "dune", "ember",
    "fjord", "glacier", "harbor", "iris", "jasper", "kelp", "lagoon", "moss",
    "narrows",
]
MODIFIERS = ["red", "blue", "green", "swift", "still"]


def wordcount_text(rng: random.Random) -> str:
    words = [base + modifier for base in BASES for modifier in MODIFIERS]
    lines = []
    size = 0
    while size < TARGET_BYTES:
        count = rng.randint(8, 12)
        line = " ".join(rng.choice(words) for _ in range(count))
        lines.append(line)
        size += len(line) + 1
    return "\n".join(lines) + "\n"


def filterstream_text(rng: random.Random) -> str:
    levels = ["INFO", "WARN", "ERROR", "INFO", "INFO", "DEBUG", "INFO", "WARN"]
    out = []
    for i in range(FILTER_LINES):
        level = levels[rng.randrange(len(levels))]
        out.append(f"2026-11-19T00:00:{i % 60:02d} line {i} {level} request id={i:06d} handled")
    return "\n".join(out) + "\n"


def main() -> None:
    here = pathlib.Path(__file__).resolve().parent
    rng = random.Random(190)
    for name, text in (("wordcount.txt", wordcount_text(rng)), ("filterstream.txt", filterstream_text(rng))):
        path = here / name
        path.write_text(text, encoding="utf-8")
        print(f"{path} {len(text.encode())} bytes")


if __name__ == "__main__":
    sys.exit(main())
