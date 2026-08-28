# 完成判定モデル

## 完成は固定Epochに対する証明

Atlasの完成は、世界中の知識を網羅したという主張ではありません。次の組を固定したRelease Certificateです。

```text
(Atlas ID, Atlas Release, Coverage Epoch, Authority Lock Digest,
 Core Policy Version, Required Environment Profiles, Evidence Set Digest)
```

完成Releaseは後から書き換えません。上流更新は新しいEpochとReleaseを作り、旧Releaseは`superseded`になっても当時のCertificateを失いません。

## Status

| Status | 意味 |
|---|---|
| `planned` | Catalogに登録済みで実装未開始 |
| `active` | Authority Inventoryまたは実装を更新中 |
| `incomplete` | Release候補は存在するが必須Gate未通過 |
| `complete` | 固定Epochに対する全必須Gate通過 |
| `superseded` | 新しい完成Releaseが存在する |
| `archived` | 保守終了。証拠と履歴は保持 |

`partial`やMVPをRepositoryの完成状態にしません。作業は小さく分割しても、公開上の完成は`complete`だけです。

## 8つのClosure

### 1. Authority Closure

- 対象Versionと仕様を固定している。
- 公式仕様、公式文書、Release Note、Security Advisory、Source、Runtime Inventoryの採否が記録されている。
- 各SourceはURL、Version、取得日、Digest、再配布方針を持つ。
- `sources.lock.yaml`自体のDigestをCoverageへ束縛する。

### 2. Coverage Closure

- すべての必須Targetが`covered`、`excluded`、`infeasible`のいずれかである。
- `covered`にはClaimとEvidenceが一つ以上ある。
- `excluded`と`infeasible`には具体的理由と再評価日がある。
- 未分類のAuthority Surfaceがゼロである。

### 3. Mastery Closure

「その分野を知りたければこのRepositoryを見ればよい」という約束を`mastery.yaml`で固定する。

- Outcomeは`understand`、`choose`、`build`、`verify`、`operate`、`troubleshoot`、`evolve`、`delegate`の8つを欠かさない。
- Surfaceは定義と境界、原理、設計、実装、検証、失敗回復、運用、安全性、性能、統合、移行、比較、来歴、Agent Skillの14面を欠かさない。
- 適用するSurfaceはCoverage Target Setへ接続する。
- 非適用Surfaceは削除せず、技術的理由を持つ`not-applicable`として残す。
- 初学者、実務者、Architect、Operator、Maintainer、Reviewer、Educator、Agentの利用経路を持つ。
- 読了だけでなく、説明、選択、構築、検証、運用、診断、進化、委任をObservable Outcomeで評価する。

### 4. Claim Closure

- 技術的主張がCapabilityとProof Obligationへ接続されている。
- すべての必須Claimに、反証可能なAcceptance Criteriaがある。
- Claimだけ、Testだけ、Evidenceだけの孤立Nodeがない。

### 5. Execution Closure

- 正常、拒否、障害、復旧、移行、互換性のうち適用対象を実行できる。
- Setup、Execute、Verify、Cleanupが再実行可能である。
- Cleanup後にResource、Process、Credential、Cloud Assetが残らない。
- 同じ入力とEnvironmentで決定論的に比較できる範囲を明示する。

### 6. Operational Closure

- Observability、Runbook、Backup／Restore、Upgrade、Incident、Capacityの適用判断がある。
- 非適用項目は理由を持つ。
- Failure Injectionが実環境へ波及しないIsolationを持つ。

### 7. Skill Closure

- Router SkillがCoverageとCapabilityから生成されたReferenceへ到達できる。
- 設計、実装、診断、移行、Reviewの代表TaskをEvalする。
- Skillが存在しない機能を捏造せずCoverage Gapを返す。
- Skill VersionがAtlas Releaseへ固定されている。

### 8. Publication Closure

- License、NOTICE、第三者Manifest、SBOMが揃う。
- 秘密、個人情報、内部URL、権利不明素材を含まない。
- SourceとAssetのProvenanceが独立して検証される。
- Release ArtifactとCertificateがDigestで固定される。

## Required Profile

`local`、`container`、`simulator`は適用可能なAtlasで必須とします。`cloud-live`と`hardware-in-the-loop`は、Scope上その証拠なしに主要Claimを立証できない場合だけ必須です。

Profileは開発段階ではなく検証環境です。Profileを省略して完成条件を弱めることはできません。高額・実機不足の場合は`infeasible`として理由と代替証拠を公開し、完全に同等であるとは主張しません。

## Certificate

`evidence/completion-certificate.json`は最低限次を含みます。

- Atlas IDとRelease
- Coverage Epoch
- Authority Lock Digest
- Core Policy Version
- Required Profileと結果
- Coverage／Claim／Evidence Graph Digest
- Skill Package DigestとEval結果
- License／SBOM／Provenance結果
- 発行時刻、Commit、署名情報

Certificateは生成物であり手編集しません。

## Subject Definitive Gate

`atlas audit <atlas-directory>`は、個別Schema適合だけでなく次を横断検査する。

- 5 ManifestのAtlas IDとCoverage Epochが一致する。
- Mastery OutcomeとSurfaceが全件存在する。
- Masteryが参照するTarget SetがCoverageに存在する。
- AtlasとSkill PackageのRouter ID・Pathが一致する。
- `complete`の場合、必須Targetに`missing`、`planned`、`partial`、`expired`がない。

将来のRelease Gateでは、Evidence IDの実体、Artifact Digest、Claim Graph、Skill Eval、SBOM、署名済みCertificateまで同じ監査へ加える。宣言だけを増やして完成扱いにはしない。
