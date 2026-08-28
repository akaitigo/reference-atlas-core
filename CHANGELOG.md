# Changelog

## v1.1.0 — 2026-08-28

- v1 `complete`を公開上`bounded-complete`として明示し、`subject-definitive`と分離した。
- Authority Surface Artifact、完全Inventory、Behavior専用Proof、10 Scenario Matrix、Runtime Profile互換性、Reference System、Comparison、Skill Eval v2を追加した。
- v1 Certificateを不変履歴へ移すMigrationと、独立Definitive Certificate生成・検証を追加した。
- 公開mainのTest/Lab/Target/Claim/Proof/Evidence/Source/Skill Eval/Profile/Matrix/CIを単調下限として固定するNon-regression Gateを追加した。
- Authority body candidate denominator、Human Review Queue、read-only Review Export、stale relock、Definitive Skill Routerを独立Gateとして追加した。
- Non-regression Baselineの同一ID項目を`(id, fingerprint)` multisetとして保持し、Claim間で重複するProof Obligationを欠落なく比較する。
- Integrated Reference Systemの10 Scenario Traceと個別Atomic Behavior Proofを分離し、統合Trace流用、未結合Authority、source／harness／environment／runtime identity不足を拒否するScenario/Trace Gateを追加した。
- Scenario Closureをexact Pattern＋Scenario＋全Variantのretry 0専用Runtime suite、first-attempt pass、Oracle、source／harness digest、専用Traceへ限定し、Capture identity補完を拒否した。
- 残存Scenario Gapをrisk順・最大4 Pattern row/trancheへ完全包含するClosure Plan Gateと、row削除・順序退避・batch肥大化を拒否するNon-regression Collectionを追加した。

## v1.0.0 — 2026-08-28

- 既存Architecture、Completion Model、Mastery ContractをControl Plane v1として固定した。
- Claim／Evidence Graph、Artifact Digest、Skill Eval、SPDX SBOM、Provenance、Completion Certificateの生成・検証を完成Gateへ追加した。
- Subject Scaffold、Skill Reference Generator、非破壊Migration、互換性Test、CI、Release Gateを追加した。
