# Atlas CLI v1

```text
atlas validate <manifest...>
atlas audit <atlas-directory>
atlas scaffold <directory> <atlas-id> <日本語title> [epoch]
atlas generate skill-reference <atlas-directory>
atlas migrate v1 <atlas-directory>
atlas certificate generate <atlas-directory> [--issued-at RFC3339] [--commit SHA]
atlas certificate verify <atlas-directory>
atlas version
```

`scaffold`は空の出力先だけへ`incomplete` Subjectを生成する。`generate skill-reference`はCanonical `coverage.yaml`から派生Referenceを再生成する。`migrate v1`は既存Fileを上書きせず、不足するMasteryと`migrations/core-v1.yaml`だけを作る。

`complete`の`audit`は5 Manifestに加え、Claim／Evidence実体、Source／Harness／Artifact Digest、Required Profile、Skill Eval、SPDX SBOM、Provenance、Completion Certificateを検査する。
