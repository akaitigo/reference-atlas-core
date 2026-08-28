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
| `complete` | v1 Manifest上、宣言済みの有限Coverageに対する全必須Gate通過。公開上の分類は`bounded-complete` |
| `superseded` | 新しい完成Releaseが存在する |
| `archived` | 保守終了。証拠と履歴は保持 |

`partial`やMVPをRepositoryの完成状態にしません。v1の`complete`は`subject-definitive`と同義ではなく、固定した自己宣言Coverageに対する`bounded-complete`です。

## 8つのClosure

### 1. Authority Closure

- 対象Versionと仕様を固定している。
- 公式仕様、公式文書、Release Note、Security Advisory、Source、Runtime Inventoryの採否が記録されている。
- 各SourceはURL、Version、取得日、Digest、再配布方針を持つ。
- `sources.lock.yaml`自体のDigestをCoverageへ束縛する。

### 2. Coverage Closure

- v1 bounded Gateでは、すべての必須Targetが`covered`、`excluded`、`infeasible`のいずれかである。
- `covered`にはClaimとEvidenceが一つ以上ある。
- `excluded`と`infeasible`には具体的理由と再評価日がある。
- v2 definitive Gateでは、固定Authority Artifact由来Surfaceの未分類をゼロにし、required Targetをすべて`covered`にする。

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

Profileは開発段階ではなく検証環境です。Profileを省略して完成条件を弱めることはできません。v1では高額・実機不足を`infeasible`としてbounded履歴に残せますが、v2では環境不足を未完として残し、static／compile-onlyを同等証拠とは扱いません。

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

## Schema横断Audit v1

`atlas audit <atlas-directory>`は、個別Schema適合だけでなく次を横断検査する。

- 5 ManifestのAtlas IDとCoverage Epochが一致する。
- Mastery OutcomeとSurfaceが全件存在する。
- Masteryが参照するTarget SetがCoverageに存在する。
- AtlasとSkill PackageのRouter ID・Pathが一致する。
- `complete`の場合、必須Targetに`missing`、`planned`、`partial`、`expired`がない。

Control Plane v1のRelease Gateは、Evidence IDの実体、Artifact Digest、Claim Graph、Skill Eval、SBOM、Provenance、Payload Digest付きCertificateまで同じ監査へ含める。宣言だけを増やして完成扱いにはしない。

## Completion Class

| Class | 意味 | Certificate |
|---|---|---|
| `incomplete` | 必須Closureが残る | なし |
| `bounded-complete` | v1の自己宣言Coverage Epochに対する履歴証明 | `completion-certificate.json` |
| `subject-definitive` | Authority由来Surface全件とBehavior Proofを閉じた証明Class | `definitive-certificate.json` |

公開UIとCLIは`complete`だけを表示せず、必ずCompletion Classを表示する。
`epoch-complete`は固定Epochに対する有限Closureを表す一般名であり、v1 CertificateのCLI識別子は曖昧さを避けて`bounded-complete`へ固定する。どちらも`subject-definitive`ではない。

## Subject Definitive Gate v2

v2はRaw Target件数を要求しない。固定Authority Artifactから抽出されたSurface集合とInventory集合が完全一致することを要求する。

- required Targetはすべて`covered`でなければならず、`excluded`／`infeasible`は未完として拒否する。
- Authority Surface Artifactは一次資料Source IDとDigestへ束縛し、Inventoryの未分類を0にする。Authority由来項目をSubject都合で除外できない。
- 各Behavior／Capabilityは専用required Target、専用accepted Claim、Scenarioごとの専用Proof Obligationを持つ。Target、Claim、Proof、Evidence、Artifactの共有による集約Closureを拒否する。
- 全Behaviorに正常、境界、拒否、障害、回復、移行、運用、Security、性能、互換性の10 Scenario Rowを要求する。Surfaceから必須となるScenarioは`not-applicable`にできない。
- required Scenarioは指定Profileの実RuntimeまたはPlatform Evidenceを必要とする。static、fixture、compile-only、KLIBやbytecode生成は代替にならない。
- Architecture／Compatibility Surfaceでは複数Behaviorを接続するReference System、Decision Surfaceでは2方式以上のComparisonを要求する。
- Skill Evalは8 Outcomeと14 Surfaceの全件、Coverage Gap応答、権限境界をpass Caseへ接続する。
- v1 Certificateは`bounded-complete`履歴としてv2 CertificateへDigest参照し、自動昇格させない。
- 公開main由来Non-regression Baselineを単調下限とし、Test/Lab/Target/Claim/Proof/Evidence/Source/Skill Eval/Profile/Matrix/CIの削除・弱化・Scope退避を拒否する。正当な置換は旧IDから新IDへの一対多または同等以上Mapping、Runtime実行Proof、Migration Evidence、理由をすべて必要とし、多対一の粗い集約は認めない。
- 固定`FE_DEPTH_REFERENCE`のAuthority消化、Atomic Behavior/Variant、Runtime Lab、10 Scenario、Artifact/Trace、統合Reference System、Skill Eval、Provenance、Non-regression軸を各Behavior/Variantへ展開し、Depth Parity MatrixのGap 0を要求する。TargetやRowの件数だけではParityとしない。

移行途中の`depth.parity.yaml`は`completion_status: incomplete`と`rows: []`、または`status: gap`のRowを保存できる。Schema適合はGapの正直な記録を許すが、Definitive Gateは`completion_status: parity`、全軸の専用Evidence/Artifact/Trace、Gap 0が揃うまで失敗する。

詳細な移行は`docs/MIGRATION_DEFINITIVE_V2.md`を正本とする。
