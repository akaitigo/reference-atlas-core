# 既存Atlas一斉移行Contract

## 対象

- `frontend-behavior-atlas`
- Flutter実用Reference
- Kotlin Engineering Reference
- PostgreSQL Reference
- RabbitMQ Reference
- Zero Trust Engineering Reference
- Argo CD Executable Reference

## 原則

- Git履歴と既存Evidenceを破壊しない。
- 動作中のDomain固有Testを先に共通化しない。
- 既存のCanonical Manifestから共通Manifestへ一方向Adapterを作る。
- Core SchemaにないDomain固有情報を削除せず、Archetype Overlayとして保持する。
- 既存の「完成」を自動継承しない。新しいCompletion Gateで再検証する。

## 移行順序

1. Repository IDと正式な日本語Titleを確定する。
2. `atlas.yaml`、`mastery.yaml`、`sources.lock.yaml`、`coverage.yaml`、`skill.package.yaml`を追加する。
3. 既存Canonical RecordとCoverage TargetのID対応表を`migrations/core-v1.yaml`へ記録する。
4. 既存Test／Capture／BenchmarkをEvidence Schemaへ束縛するAdapterを追加する。
5. Router Skillを一つ定義し、既存Skillは内部ModeまたはReferenceへ移す。
6. Apache-2.0、NOTICE、第三者Manifest、SBOM、DCOを適用する。
7. 日本語を利用者向け正本にし、機械識別子は英語のまま維持する。
8. `atlas validate`をCIへ追加する。
9. 旧Commandと旧Pathに互換期間が必要ならAliasと期限を明記する。
10. 全Gate通過前は`status: incomplete`を維持する。

## Rename

Repository名は原則`<subject>-reference-atlas`に統一します。既に適切な`frontend-behavior-atlas`は名称を維持できます。GitHub Rename後も旧URL Redirectだけに依存せず、CatalogへAliasと移行日を記録します。

## Frontend固有方針

`experiments/**/pattern.json`、Runner Protocol、決定論Capture、Benchmark、Accessibility ContractはFrontend固有の正本として維持します。Coreへ移すのは次だけです。

- Atlas Identity
- Mastery Outcome／Surface
- Authority Lock
- Coverage状態意味論
- Claim／Evidence接続
- Skill Package Metadata
- Completion Certificate
- License／Provenance Gate

## 完了条件

- 共通5 ManifestがSchema適合し、`atlas audit`の横断監査を通る。
- 既存Canonical IDから新IDへの欠落がない。
- 既存TestとEvidenceが失われていない。
- Skill Evalが新Router経由で通る。
- Release前の権利・秘密・第三者素材Gateが通る。
- Core v1依存が固定Releaseで指定される。

## CLI

`atlas migrate v1 <repository-root>`は既存`atlas.yaml`と`coverage.yaml`から、不足している`mastery.yaml`と`migrations/core-v1.yaml`のDraftだけを生成する。既存Fileは上書きせず、再実行は変更ゼロになる。生成後はOutcome／Surfaceの割当、旧IDのDisposition、互換期限を人が確認し、5 Manifestと移行Mapを`atlas validate`、Repository全体を`atlas audit`で検証する。
