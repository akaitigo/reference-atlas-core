# Subject Definitive Gate v2 移行契約

## 互換性と履歴

v1の`status: complete`と`evidence/completion-certificate.json`は、当時宣言した有限Coverageに対する`bounded-complete`履歴として引き続き検証できる。削除、意味の上書き、`subject-definitive`への自動昇格は行わない。

```bash
atlas migrate definitive-v2 <repository-root>
```

このCommandはv1 Certificateを`evidence/history/<release>/completion-certificate.json`へ不変コピーし、`migrations/definitive-v2.yaml`へDigestと未完Actionを記録する。Authority InventoryやProofを`atlas.yaml.scope`から自動生成しない。自動生成すると、狭いScopeを自己申告するだけで通るv1の欠陥を再現するためである。

`atlas.status: incomplete`へ戻して正直に移行している間、Definitive Auditは`definitive.yaml.historical_certificates`のv1 CertificateについてSchema、Atlas ID、payload署名を検証し、`bounded-complete`履歴基盤として認識する。その後にrequired Target、Inventory、Matrix、Depth Parity等の実際のv2 Gapを報告する。履歴Certificateを現在のManifestから再計算したり上書きしたりしない。全v2 Gateが通った後の昇格とDefinitive Certificate生成には`atlas.status: complete`が必要である。

## 移行手順

1. v1 Certificateを`bounded-complete`履歴として固定する。
2. `sources.lock.yaml`の一次資料本文または固定Surface Artifactを保存し、Digestを固定する。
3. 公開mainから`atlas baseline generate`でNon-regression Baselineを固定し、以後のCoverageを単調追加にする。
4. 移行中は`depth.parity.yaml`を`completion_status: incomplete`として保存し、未解消軸をGapとして残す。空の`rows: []`も有効であり、Parityを捏造しない。
5. Artifactから抽出した全Behavior／CapabilityとVariantを`surface.inventory.yaml`へ一対一で分類する。
6. 各Inventory項目へ専用required Target、専用Claim、Scenarioごとの反証可能Proofを割り当てる。
7. 正常、境界、拒否、障害、回復、移行、運用、Security、性能、互換性の10 Scenarioを全Behaviorについて分類する。
8. required Scenarioへ専用Runtime／Platform Evidenceと専用Artifactを接続する。KLIB、bytecode、compile-only、static fixtureは代替にしない。
9. Commit `4a0b2df8e2091a963bd0e0e1bbccef9c84b49a45`の`FE_DEPTH_REFERENCE.json`をDigest固定する。Frontendの現状は`incomplete`、1軸`satisfied`、17軸`partial`のまま保持する。
10. 同Referenceの18軸を、Subject自身のAuthority由来denominatorについてBehavior/Variantごとの専用Proof、Oracle、Evidence、Artifact、Traceへ接続しGapを0にする。Frontend固有のTarget、Variant、Test件数を閾値として転用しない。
11. Architecture／Integration Surfaceがあれば複数Behaviorを接続するReference Systemを、Decision Surfaceがあれば複数方式Comparisonを追加する。
12. Skill Evalを8 Outcome、14 Surface、Coverage Gap応答、権限境界へ接続する。
13. `atlas certificate generate-definitive`でv1とは別のCertificateを発行する。
14. `atlas audit <root> --gate definitive`が`completion_class=subject-definitive`を返すまで公開上は未完とする。

## 互換性分類

- v1 Schema、v1 Audit、v1 Certificate検証は維持する。
- v1の`complete`表示はCLI上`completion_class=bounded-complete`として明示する。
- v2は加法的な別Gateだが、`subject-definitive`という主張へ移行する場合は新しいAuthority Inventory、Proof Matrix、Certificateが必須になる。
