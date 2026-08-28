# 技術実証アトラス Core

`reference-atlas-core` は、`akaitigo` のGitHubアカウントで公開する技術実証アトラス群の共通契約です。

アトラスは単なる記事集やサンプル集ではありません。対象技術の範囲を有限に固定し、能力、主張、実装、試験、証拠、運用判断、Agent Skillを追跡可能に接続します。固定したCoverage Epochに対してのみ「完成」を宣言します。

## このリポジトリが所有するもの

- 全Subject Atlasが従う機械可読Schema
- Stage 1の技術分野Catalog
- 完成判定とVersion規則
- 各分野のAuthority由来Coverageを8 Outcome・14 Surfaceで検証するMastery契約
- 日本語を正本とする文書化規則
- Agent Skillの生成・配布契約
- 権利、出典、第三者素材のPublication Gate
- 既存リポジトリを移行するMigration Contract
- 会社の技術棚卸しを安全に受け渡す匿名化SchemaとPrompt
- 共通検証CLI `atlas`

このリポジトリは個別技術の実装や、複数技術を組み合わせる統合システムを所有しません。個別技術はSubject Atlas、分野横断試験は`atlas-interoperability-lab`が所有します。

## Control Plane

```text
reference-atlas-core
        |
        +--> Subject Atlases
        |       |
        |       +--> technology-atlas-skills
        |       +--> atlas-interoperability-lab
        |
        +--> executable-technology-atlas
```

依存は固定Release、Package、OCI Digestまたは署名済みManifestを介します。他リポジトリのSource TreeやFloating Branchには依存しません。

## 現在の状態

Control Plane v1の既存Certificateは固定したCoverage Epoch `2026-08-28`に対する`bounded-complete`履歴です。`subject-definitive`とは区別されます。Definitive Gate v2はAuthority由来Surface全件、Atomic Behavior/Variant、10 Scenario Matrix、固定FE Depth Parity、実Runtime／Platform Artifact/Trace、公開main Non-regressionを別Certificateで検証します。

## 開発用検証

```bash
go test ./...
go run ./cmd/atlas validate catalog/stage1.yaml
go run ./cmd/atlas validate examples/company-inventory.yaml
go run ./cmd/atlas validate examples/frontend-behavior-atlas/atlas.yaml
go run ./cmd/atlas validate examples/frontend-behavior-atlas/coverage.yaml
go run ./cmd/atlas audit examples/frontend-behavior-atlas
go run ./cmd/atlas audit .
make release-check
```

CLIの全Commandは`docs/CLI.md`、互換性は`docs/VERSIONING.md`、公開手順は`docs/RELEASING.md`を正本とします。

TemplateとDocsの記述規約は[`docs/NEUTRAL_AUTHORING.md`](docs/NEUTRAL_AUTHORING.md)に従い、検証不能な優越表現ではなくEvidence、比較条件、既知の制約を記録します。

## 言語

利用者向け説明、Skill、CLIメッセージ、判断記録は日本語を正本にします。リポジトリ名、Schema Key、Capability ID、API名、コード識別子、上流仕様の正式名称は追跡可能性を守るため英語表記を維持します。
