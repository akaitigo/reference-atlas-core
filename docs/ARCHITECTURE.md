# 技術実証アトラス全体Architecture

## 1. 目的

最終状態は、GitHub個人アカウント`akaitigo`の配下に、プログラム可能な計算システムを対象とする技術分野の実行可能Referenceと、それをAgentが安全に利用するSkillが揃っていることです。

「すべて」は無期限の世界知識を意味しません。各Atlasが固定したCoverage Epoch、対象Version、Authority Corpus、明示的除外に対して、未分類・未検証・未説明の項目を残さないことを意味します。

## 2. Architecture原則

1. **完成境界で分ける。** 独立したAuthority、Version、検証環境、障害モデル、完成条件を持つ対象は別Repositoryにする。
2. **正本は機械可読にする。** 記事本文ではなくManifest、Lock、Claim、Evidenceを正本にする。
3. **Fieldを先に閉じる。** Stage 1は分野の原理、判断、代表実装、運用、Skillを完成させる。全製品の完全閉包は要求しない。
4. **統合を分離する。** Subject Atlasへ他技術の統合都合を持ち込まず、Stage 2のInteroperability Labで検証する。
5. **証拠をSourceへ束縛する。** EvidenceはSource、Harness、EnvironmentのDigestを持ち、いずれかが変われば失効する。
6. **Skillを複製知識にしない。** SkillはCanonical Atlasを検索・実行・検証するRouterであり、独立した手書き百科事典ではない。
7. **日本語を利用者向け正本にする。** 機械識別子と上流の正式名称だけ英語を維持する。
8. **公開可能性をRelease条件にする。** 権利、出典、商標、秘密、個人情報が不明な成果物は公開しない。

## 3. Repository Topology

### 3.1 Control Plane

| Repository | 所有責任 |
|---|---|
| `reference-atlas-core` | Schema、CLI、Policy、Template、Migration Contract |
| `executable-technology-atlas` | Catalog、横断検索、Coverage Portal、Certificate検証 |
| `technology-atlas-skills` | Skill Package Index、導入、更新、Rollback、Agent Adapter |
| `atlas-interoperability-lab` | 複数Subject Releaseを組み合わせる統合試験 |

### 3.2 Subject Atlas

Subject Atlasは次のArchetypeを一つ以上宣言します。

| Archetype | 完成時の主張 | 固有構造の例 |
|---|---|---|
| `field` | この技術分野の原理・判断・代表実装ならここ | `taxonomy/`, `patterns/`, `decision-matrices/`, `product-packs/` |
| `product` | 固定Versionのこの製品ならここ | `surface/`, `versions/`, `extensions/`, `upgrade-matrix/` |
| `specification` | 固定仕様への実装・適合ならここ | `spec/`, `conformance/`, `test-vectors/`, `registries/` |
| `language-platform` | 言語、SDK、Platform Surfaceならここ | `language/`, `platforms/`, `interop/`, `tooling/` |
| `construction` | 内部実装を構築・比較するならここ | `implementation/`, `models/`, `differential-tests/`, `profilers/` |
| `hardware-simulation` | Simulatorと実機証拠を扱うならここ | `boards/`, `hdl/`, `firmware/`, `hil/`, `measurement/` |

Archetype固有構造は必要なものだけ作り、空Directoryを完成の証拠にしません。

## 4. 依存方向

```text
reference-atlas-core release
          |
          v
   Subject Atlas releases
       |             |
       v             v
technology-      atlas-interoperability-
atlas-skills     lab
       \             /
        v           v
     executable-technology-atlas
```

- Subject Atlasは単独でClone、Build、Test、Validateできる。
- 他SubjectのSource Tree、Default Branch、Git submoduleへ依存しない。
- 共有物は固定Release、Package Version、OCI Digest、署名済みManifestで参照する。
- Subject間の循環依存をCatalog Gateで拒否する。
- PortalはManifestと公開EvidenceをIndexし、Subject実装を所有しない。

## 5. Repository内の共通Topology

```text
/
├── atlas.yaml
├── mastery.yaml
├── sources.lock.yaml
├── coverage.yaml
├── skill.package.yaml
├── README.md
├── LICENSE
├── NOTICE
├── SECURITY.md
├── CONTRIBUTING.md
├── AGENTS.md
├── atlas/
│   ├── concepts/
│   ├── capabilities/
│   ├── claims/
│   ├── proof-obligations/
│   ├── decisions/
│   ├── failures/
│   ├── migrations/
│   └── exclusions/
├── reference-systems/
├── labs/
├── tests/
├── benchmarks/
├── environments/
├── operations/
├── .agents/skills/
├── evals/
├── docs/
├── third_party/
└── evidence/
```

### Canonical chain

```text
Authority Source
  -> Fixed Authority Surface Artifact
  -> Complete Surface Inventory
  -> Mastery Surface / Outcome
  -> Coverage Target
  -> Capability
  -> Claim
  -> Proof Obligation
  -> Lab / Reference System
  -> Test / Oracle
  -> Evidence
  -> Skill Eval
  -> Completion Certificate
```

v1 CertificateはこのChainのうち自己宣言Coverageを閉じる`bounded-complete`履歴である。`subject-definitive`はFixed Authority Surface Artifactから導出された全Behavior／CapabilityがInventory、専用Target、Claim、Proof、Scenario Matrix、Runtime Evidenceへ一対一で接続された場合だけ発行する。

孤立したNodeはReleaseを阻止します。v1 bounded Gateだけは理由、責任者、再評価日を持つ明示的Exclusionを履歴化できるが、v2 definitive Gateのrequired Surfaceには例外を認めない。

## 6. Stage

### Stage 0 — Control Plane

Schema、CLI、Catalog、Evidence、Skill、権利管理、Migrationの共通契約を完成させます。

### Stage 1 — Field Closure

`catalog/stage1.yaml`で定義した全技術分野についてField Atlasを完成させます。代表製品はFieldの判断と実証に必要な範囲で使用します。既に計画済みのKotlin、Flutter、PostgreSQL、RabbitMQ、Argo CDはSeed Product Atlasとして並行利用できます。

### Stage 2 — Interoperability

複数の完成済みSubject Releaseを組み合わせ、中立Primitiveによる相互運用、障害伝播、Telemetry、Security Boundaryを検証します。

### Stage 3 — Product Closure

必要性が確認された製品について、公開Surface、Version、Upgrade、運用まで完全に閉じるProduct Atlasを追加します。

### Stage 4 — Coverage Epoch更新

上流のMajor、LTS、重要Security更新または仕様改訂を契機に新しいEpochを作ります。過去の完成ReleaseとCertificateは不変の履歴として保持します。

## 7. Countを目標にしない

Repository数はArchitecture上の成果ではありません。初期Catalogは網羅性確認のために広く定義しますが、次の場合は分割・統合します。

- AuthorityとRelease Cycleが独立している。
- 必須EnvironmentやProof Methodが大きく異なる。
- 一方の完成が他方の未完成により永久に阻害される。
- 利用者の問いが「この分野ならここ」と独立して成立する。

単に流行している、製品名が違う、Libraryが違うという理由だけではRepositoryを増やしません。

## 8. 既存Frontendから継承するもの

`frontend-behavior-atlas`から次を共通原則へ昇格します。

- 有限でVersion化されたCoverage Target
- 同じObservable Contractに対するVariant比較
- Source Digestへ束縛されたCaptureとBenchmark
- Lifecycle、Accessibility、Fallback、Performance、Provenance Gate
- Canonical Registryから生成されるSkill Reference
- 未対応項目を隠さず`planned`、`partial`、`excluded`等で表現する状態機械

一方、DOM、Canvas、Motion、Browser ProtocolなどはFrontend固有Overlayとして残します。
