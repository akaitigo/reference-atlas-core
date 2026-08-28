# FE Depth Reference入力契約

## 固定入力

Depth ParityはFrontendの観測件数ではなく、次の固定入力にある18軸のportable criterionとdenominator semanticsを参照する。

- Repository: `frontend-behavior-atlas`
- Commit: `4a0b2df8e2091a963bd0e0e1bbccef9c84b49a45`
- Reference: `FE_DEPTH_REFERENCE.json`
- SHA-256: `2452696f9807b7d4a8ffb22b3ba37f079a25a34ac2370d78423445b96064582a`
- 現状: `incomplete`、1軸`satisfied`、17軸`partial`、Frontendは`subject-definitive`ではない

Coreに同梱した`profiles/FE_DEPTH_REFERENCE.json`は上記Blobと同一である。Subject Repositoryの`authority/FE_DEPTH_REFERENCE.json`も同じDigestへ固定する。参照元の`observedDensity`はFrontend自身の非後退観測値であり、別SubjectのTarget、Variant、Lab、Test、Evidence件数の閾値には使用しない。

## Subject側の判定

Subjectは自身の一次資料本文または固定Authority ArtifactからSurface Inventoryを導出し、そのInventoryに含まれるAtomic BehaviorとVariantを母集団にする。`depth.parity.yaml`の各Rowは、18軸ごとに専用の`proof_id`、反証可能な`oracle`、Evidence、Artifact、Traceを持つ。実Runtimeを要する軸ではstatic、mock、fixture、compile-onlyを代替として扱わない。

Frontend参照の軸が現時点で`partial`であることと、Subject側Rowの`satisfied`は別の状態である。前者は参照元Repositoryの観測状態、後者はportable criterionをSubject固有のAuthority由来denominatorについて満たした実証状態を表す。Frontendの不足を別Subjectへ免除として移さず、Frontendを自動昇格もしない。

## Companion入力の確認

同Commitの次のBlobを、入力契約の解釈と非後退境界の確認に使用した。

| Path | SHA-256 | Coreでの扱い |
|---|---|---|
| `baselines/definitive-gate-v2.json` | `b6685fac1e11429ed7c203e35aac55c7a82c276dc32aadb97f225af801c3bb67` | FE固有ID、Assertion、Budget、Runtime、CIの非後退floor。絶対件数は移植しない |
| `migrations/non-regression-v2.json` | `bef89e6ff88067f7a6809ae12804f588bd0500e9f84d261f030ba7dc5113c347` | `active`かつ置換Mappingなし。暗黙の縮小承認として扱わない |
| `fixtures/definitive-gate-v2/authority-surface-inventory.fixture.json` | `01c29cdd29f61968b06791edf3d0d0674462d279422bd312d523ab7964e2a1e4` | 未Review InventoryをCompletion Evidenceとして扱わない |
| `fixtures/definitive-gate-v2/evidence-granularity.fixture.json` | `94506646f1e30429cb84a87927e1af7e1e890d401efd0335ebf911aed6f85126` | Bundle EvidenceをBehavior・Scenario専用Proofの代替にしない |
| `fixtures/definitive-gate-v2/profile-incompatibility.fixture.json` | `95c07992da4b78db5c5551d7ed0c4425668dde9188c0acb53c3b76b17a91e364` | Container、実Device、支援技術、Cloud、staticの相互代替を拒否する |
| `fixtures/definitive-gate-v2/variant-comparison.fixture.json` | `2964d596201033761c083d06dc705d21a428de043a91a0f1e71e7d5b841f59d3` | 同じObservable Contractを共有する方式を専用Proofで比較する |
| `docs/DEFINITIVE_GATE_V2_REFERENCE.md` | `280a398ed1251438ad244e999c9c9cef9b0b6b78217db82e2b55ef882306d241` | 参照元の境界、未完理由、Fixtureの位置づけを保持する |

Companion fixtureはCore SchemaでもCompletion Evidenceでもない。Core Gateは同じ拒否条件をSchema、Audit、negative fixture testで実装し、Subject側の入力はCore Schemaへ適合させる。
