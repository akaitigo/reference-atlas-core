# Repository instructions

このリポジトリは技術実証アトラス群の共通契約を所有する。個別分野の機能一覧や製品固有実装をCoreへ追加しない。

## Canonical sources

- `schemas/*.schema.json`をManifest形式の正本とする。
- `catalog/stage1.yaml`をStage 1対象分野の正本とする。
- `docs/ARCHITECTURE.md`と`docs/COMPLETION_MODEL.md`を境界・完成意味論の正本とする。
- 各Subjectの`mastery.yaml`を「その分野の決定版」が満たすOutcomeとSurfaceの正本とする。
- Templateや生成物を先に変更せず、正本を変更して再生成または移行する。

## Language

- 利用者向け文書、Skill、CLIメッセージは日本語を正本とする。
- Schema Key、ID、Repository名、Path、API名、上流の正式名称は英語のCanonical表記を保つ。
- 英語文書を機械的に複製して二重保守しない。

## Boundaries

- 「全技術」や「完成」を無期限・無境界に主張しない。必ずCoverage Epoch、Authority Lock、明示的除外に結び付ける。
- Subject Atlas同士のSource依存やGit submoduleを導入しない。
- 統合試験をCoreへ置かない。`atlas-interoperability-lab`へ置く。
- 製品固有Schemaを共通必須項目へ昇格させない。Archetype Overlayで表現する。

## Rights and safety

- 独自コードと独自文書はApache-2.0を既定とする。
- 第三者素材、依存関係、商標、Datasetは別々に出典と利用条件を記録する。
- ライセンスまたは権利が不明な素材は公開を拒否する。
- Security Labは防御、検証、教育を目的とし、実在する第三者環境を標的にしない。

## Change discipline

- Schema変更には互換性分類とMigration Guideを伴わせる。
- Completion Gateを弱める変更は、理由と既存Certificateへの影響を明記する。
- `complete`への変更は個別Schema検証だけでなく`atlas audit`を通す。
- Catalog項目は件数目標のために分割・統合しない。独立したAuthority、Version、検証環境、完成条件を持つかで判断する。
