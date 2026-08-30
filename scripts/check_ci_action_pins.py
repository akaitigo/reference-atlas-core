#!/usr/bin/env python3
"""GitHub Actionsの外部Action参照をexact commitへ固定する。"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / ".github" / "workflows" / "ci.yml"
SHA40 = re.compile(r"[0-9a-f]{40}")
USES = re.compile(r"^\s*(?:-\s*)?uses:\s*([^@\s]+)@([^\s#]+)", re.MULTILINE)
REQUIRED_PINS = {
    "actions/checkout": "11d5960a326750d5838078e36cf38b85af677262",
    "actions/setup-go": "40f1582b2485089dde7abd97c1529aa768e1baff",
}
PUBLISHED_SOURCE_ENV = "DCO_AUDIT_REF: ${{ github.event.pull_request.head.sha || github.sha }}"


class PinError(RuntimeError):
    pass


def verify(text: str) -> None:
    observed: dict[str, list[str]] = {}
    for action, reference in USES.findall(text):
        if action.startswith("./") or action.startswith("docker://"):
            continue
        if SHA40.fullmatch(reference) is None:
            raise PinError(f"外部Actionがexact commitではありません: {action}@{reference}")
        observed.setdefault(action, []).append(reference)

    for action, expected in REQUIRED_PINS.items():
        references = observed.get(action)
        if not references:
            raise PinError(f"必須Actionがありません: {action}")
        if any(reference != expected for reference in references):
            raise PinError(f"必須Actionの固定commitが不一致です: {action}")
    if PUBLISHED_SOURCE_ENV not in text:
        raise PinError("Release gateがpublished source commitをDCO_AUDIT_REFへ固定していません")


def expect_rejection(name: str, text: str, expected: str) -> None:
    try:
        verify(text)
    except PinError as error:
        if expected not in str(error):
            raise PinError(f"negative fixture {name} の拒否理由が不一致です: {error}") from error
        return
    raise PinError(f"negative fixture {name} が受理されました")


def self_test(text: str) -> None:
    fixtures = [
        (
            "checkout-mutable-tag",
            text.replace(REQUIRED_PINS["actions/checkout"], "v4", 1),
            "exact commitではありません",
        ),
        (
            "setup-go-mutable-tag",
            text.replace(REQUIRED_PINS["actions/setup-go"], "v5", 1),
            "exact commitではありません",
        ),
        (
            "required-action-removed",
            re.sub(r"^\s*-\s*uses:\s*actions/setup-go@[^\n]+\n", "", text, count=1, flags=re.MULTILINE),
            "必須Actionがありません",
        ),
        (
            "new-mutable-external-action",
            text + "\n      - uses: actions/cache@v4\n",
            "exact commitではありません",
        ),
        (
            "mutable-reusable-workflow",
            text + "\n  mutable-reusable:\n    uses: example/workflows/.github/workflows/ci.yml@v1\n",
            "exact commitではありません",
        ),
        (
            "pr-synthetic-merge-ref",
            text.replace(PUBLISHED_SOURCE_ENV, "DCO_AUDIT_REF: ${{ github.sha }}", 1),
            "published source commit",
        ),
    ]
    for name, fixture, expected in fixtures:
        expect_rejection(name, fixture, expected)
    print(f"CI Action pin negative fixtures: {len(fixtures)}/{len(fixtures)} rejected")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()

    text = WORKFLOW.read_text(encoding="utf-8")
    verify(text)
    if args.self_test:
        self_test(text)
    print("GitHub Actions exact commit pinを検証しました")


if __name__ == "__main__":
    main()
