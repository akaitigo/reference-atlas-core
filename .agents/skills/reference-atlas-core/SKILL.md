---
name: reference-atlas-core
description: 技術実証アトラスの設計、Schema、監査、Subject生成、Migration、Completion Certificateを既存Core v1契約に沿って扱う。
---

# Reference Atlas Core Router

最初にRepository Rootの `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/COMPLETION_MODEL.md` を読む。

- Manifest変更は `schemas/*.schema.json` を正本にし、互換性分類とMigrationを同時に扱う。
- Subject生成は `atlas scaffold`、派生Skill Referenceは `atlas generate skill-reference` を使う。
- 完成判定は `atlas audit` を使い、個別 `validate` の成功だけで `complete` にしない。
- Coverage外の分野固有要件はCoreへ追加せず、SubjectのArchetype OverlayまたはGapとして返す。
- 公開、外部変更、Security検証は依頼者の権限と対象範囲を確認する。

詳細なCommandとGateは `references/coverage.md` を参照する。
