#!/usr/bin/env python3
"""全履歴のDCO trailerと署名済みremediationを検証する。"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
LEDGER = ROOT / "security" / "dco-remediations.json"
ALLOWED_SIGNERS = ROOT / "security" / "allowed_signers"
SHA40 = re.compile(r"[0-9a-f]{40}")
SIGN_OFF = re.compile(r"(?im)^Signed-off-by: .+ <[^>]+>$")
REMEDIATES = re.compile(r"(?im)^DCO-Remediates: ([0-9a-f]{40})$")


class DCOError(RuntimeError):
    pass


def parse_records(raw: str) -> list[tuple[str, str]]:
    values = [value for value in raw.split("\0") if value.strip()]
    if len(values) % 2 != 0:
        raise DCOError("DCO入力recordがcommit/messageの組ではありません")
    records: list[tuple[str, str]] = []
    for index in range(0, len(values), 2):
        commit = values[index].strip()
        if SHA40.fullmatch(commit) is None:
            raise DCOError(f"DCO入力のCommit SHAが不正です: {commit}")
        records.append((commit, values[index + 1]))
    return records


def load_ledger(path: Path = LEDGER) -> set[str]:
    document = json.loads(path.read_text(encoding="utf-8"))
    if document.get("schema_version") != 1:
        raise DCOError("DCO remediation ledger schema_versionが不正です")
    entries = document.get("remediations")
    if not isinstance(entries, list):
        raise DCOError("DCO remediation ledger entriesがありません")
    allowed: set[str] = set()
    for entry in entries:
        if not isinstance(entry, dict):
            raise DCOError("DCO remediation ledger entryがObjectではありません")
        commit = entry.get("commit")
        if not isinstance(commit, str) or SHA40.fullmatch(commit) is None:
            raise DCOError("DCO remediation対象Commitが不正です")
        if commit in allowed:
            raise DCOError(f"DCO remediation対象Commitが重複しています: {commit}")
        reason = entry.get("reason")
        if not isinstance(reason, str) or len(reason.strip()) < 40:
            raise DCOError(f"DCO remediation理由が不足しています: {commit}")
        source_commits = entry.get("source_signed_commits")
        if not isinstance(source_commits, list) or not source_commits:
            raise DCOError(f"DCO remediation元の署名Commitがありません: {commit}")
        if any(not isinstance(source, str) or SHA40.fullmatch(source) is None for source in source_commits):
            raise DCOError(f"DCO remediation元の署名Commitが不正です: {commit}")
        allowed.add(commit)
    return allowed


def git(*arguments: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *arguments],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )


def verify_remediation_source(source: str, target: str) -> None:
    target_exists = git("cat-file", "-e", f"{target}^{{commit}}")
    if target_exists.returncode != 0:
        raise DCOError(f"DCO remediation対象Commitを参照できません: {target}")
    ancestry = git("merge-base", "--is-ancestor", target, source)
    if ancestry.returncode != 0:
        raise DCOError(f"DCO remediationが対象Commitより後ではありません: {source} -> {target}")
    target_message = git("show", "-s", "--format=%B", target)
    if target_message.returncode != 0:
        raise DCOError(f"DCO remediation対象Commit本文を読めません: {target}")
    if SIGN_OFF.search(target_message.stdout):
        raise DCOError(f"DCO trailerがあるCommitをremediation対象にできません: {target}")
    signature = git(
        "-c",
        f"gpg.ssh.allowedSignersFile={ALLOWED_SIGNERS}",
        "verify-commit",
        source,
    )
    if signature.returncode != 0:
        raise DCOError(f"DCO remediation attestationの暗号署名を検証できません: {source}")


def verify_records(
    records: list[tuple[str, str]],
    allowed_targets: set[str],
    *,
    verify_git_signatures: bool = True,
) -> None:
    remediated: set[str] = set()
    errors: list[str] = []
    for source, message in records:
        targets = REMEDIATES.findall(message)
        if not targets:
            continue
        if SIGN_OFF.search(message) is None:
            errors.append(f"DCO remediation attestation自身にSigned-off-byがありません: {source}")
            continue
        for target in targets:
            if target not in allowed_targets:
                errors.append(f"未登録CommitをDCO remediationできません: {target}")
                continue
            try:
                if verify_git_signatures:
                    verify_remediation_source(source, target)
            except DCOError as error:
                errors.append(str(error))
                continue
            remediated.add(target)

    for commit, message in records:
        if SIGN_OFF.search(message) is None and commit not in remediated:
            errors.append(f"DCO Signed-off-byがないCommit: {commit}")
    if errors:
        raise DCOError("\n".join(errors))


def audit_records(reference: str) -> list[tuple[str, str]]:
    result = git("log", "--topo-order", "--format=%H%x00%B%x00", reference)
    if result.returncode != 0:
        raise DCOError(f"DCO audit対象refを読めません: {reference}")
    return parse_records(result.stdout)


def self_test() -> None:
    target = "a" * 40
    source = "b" * 40
    signed = "Signed-off-by: Test User <test@example.invalid>"
    remediation = f"attest\n\nDCO-Remediates: {target}\n{signed}\n"
    fixtures = 0

    verify_records([(source, remediation), (target, "missing\n")], {target}, verify_git_signatures=False)
    fixtures += 1

    rejected = [
        ([(target, "missing\n")], {target}, "Signed-off-byがないCommit"),
        ([(source, f"DCO-Remediates: {target}\n"), (target, "missing\n")], {target}, "attestation自身"),
        ([(source, f"DCO-Remediates: {target}\n{signed}\n")], set(), "未登録Commit"),
        ([(source, f"DCO-Remediates: {'c' * 40}\n{signed}\n"), (target, "missing\n")], {target}, "未登録Commit"),
    ]
    for records, allowed, expected in rejected:
        try:
            verify_records(records, allowed, verify_git_signatures=False)
        except DCOError as error:
            if expected not in str(error):
                raise DCOError(f"DCO negative fixtureの拒否理由が不一致です: {error}") from error
        else:
            raise DCOError("DCO negative fixtureが受理されました")
        fixtures += 1
    print(f"DCO remediation fixtures: {fixtures}/{fixtures} passed")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--audit-ref", default="HEAD", help="指定refから到達可能な全Commitを監査する")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()

    if args.self_test:
        self_test()
    records = audit_records(args.audit_ref)
    verify_records(records, load_ledger())
    print(f"DCO全履歴検証済み: commits={len(records)}")


if __name__ == "__main__":
    try:
        main()
    except (DCOError, json.JSONDecodeError) as error:
        print(error, file=sys.stderr)
        raise SystemExit(1)
