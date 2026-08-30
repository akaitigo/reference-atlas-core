#!/usr/bin/env python3
import re
import sys

records = [record for record in sys.stdin.read().split("\0") if record.strip()]
failed = []
for index in range(0, len(records), 2):
    commit = records[index].strip()
    message = records[index + 1] if index + 1 < len(records) else ""
    if not re.search(r"(?im)^Signed-off-by: .+ <[^>]+>$", message):
        failed.append(commit)
if failed:
    print("DCO Signed-off-byがないCommit:", *failed, sep="\n", file=sys.stderr)
    raise SystemExit(1)
