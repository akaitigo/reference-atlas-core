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
2. `sources.lock.yaml`の一次資料response metadataとbody digestを固定する。第三者本文・抜粋・heading文字列は保存せず、`authority/extraction.snapshot.json`と`authority/surfaces-draft/*.json`へDigest、Locator offset、context/heading digest、Domain metadata digestだけを保存する。
3. 公開mainから`atlas baseline generate`でNon-regression Baselineを固定し、以後のCoverageを単調追加にする。capture契約へ新しいCollectionを追加する場合は旧Baselineを保存し、`baseline_upgrade.mode=monotonic-capture-contract-extension`で旧全項目、Scope、閾値の不変性とMigration Evidenceを検証する。既存Baselineの上書きや縮小には使用できない。
4. 移行中は`depth.parity.yaml`を`completion_status: incomplete`として保存し、未解消軸をGapとして残す。空の`rows: []`も有効であり、Parityを捏造しない。
5. locked documentごとのcandidate anchor denominatorと専用stable-ID baselineを生成する。raw anchor件数はDepth達成へ算入しない。
6. `authority/review-queue.snapshot.json`と`authority/review-queue-draft/*.json`で全stable anchorを包含し、staleおよびunavailable/failed documentをholdへ隔離する。`authority/reviews/decisions.json`へinclude／exclude／merge／split／deferを人間reviewer、時刻、40文字以上の理由、`manual-primary-source`、source/tool digest、URL/locator、old→new mapping、Surface/Atomic result付きで記録する。
7. read-only packet exportを用いる場合は`mode=read-only`、`write_decisions=false`、`promote_human_review=false`を維持する。machine proposalをHuman decisionとしてLedgerへ転記しない。
8. stale Lockを更新する場合は候補reportから人が明示選択し、旧→新ID mapping、実行Proof、専用Migration Evidence、Non-regression Evidenceを`authority/relock-decisions/*.json`へ固定してから再抽出する。自動Lock更新は禁止する。
9. stale、fetch failed、fragment-not-found、locator deferredをそれぞれ0にし、Authority本文全体からのSurface抽出を`authority_text_surfaces_exhaustive=true`として証明し、Human review済みのeligible Surfaceだけを最終Authority Artifactへ昇格する。candidate edgeやraw anchorの件数を本文全体の完備性として扱わない。
10. Artifactから抽出した全Behavior／CapabilityとVariantを`surface.inventory.yaml`へ一対一で分類する。
11. 各Inventory項目へ専用required Target、専用Claim、Scenarioごとの反証可能Proofを割り当てる。
12. 正常、境界、拒否、障害、回復、移行、運用、Security、性能、互換性の10 Scenarioを全Behaviorについて分類する。
13. required Scenarioへ専用Runtime／Platform Evidenceと専用Artifactを接続する。KLIB、bytecode、compile-only、static fixtureは代替にしない。
14. `evidence/scenarios/index.json`へAuthority由来Behavior × 10 Scenarioの全rowを列挙する。各rowをsource、harness、environment、runtime identity、専用Evidence／Artifact、Atomic Authority bindingへ接続し、未Closureは明示Gapとして残す。個別GapはPattern＋Scenario＋全Variantを駆動したretry 0の専用suite、first-attempt pass、Scenario固有Oracle、source／harness digest、Traceが揃う場合だけ閉じる。Capture identityで補完しない。
15. 残存Gapを`evidence/scenarios/closure-plan.json`へrisk順で完全包含し、同一Scenario内を安定Pattern順、1 tranche最大4 Pattern rowへ分割する。各trancheを全Variantの専用Runtime ProofとNon-regression確認付きで閉じ、Plan row削除、順序退避、batch肥大化を行わない。
16. 専用Runtime Reporterを`publish_on=full-run-passed`、`failed_run=retain-prior-success`、`swap=staged-directory-rename-with-rollback`へ固定する。全Artifactをstaging directoryへ生成してからdirectory renameし、置換失敗はrollbackする。failed／no-match run前後の成功Evidence Digest集合が一致するnegative fixtureを実行し、`atlas audit <root> --gate evidence-durability`を通す。
17. `evidence/dependency-graph.json`へSource／Harness／Runtime／Profileと全Evidenceの推移依存を固定する。入力変更時は影響するlocal／container E2E、Capture、Benchmark、Compatibility、Reference System、Scenario Proof、Skill Eval等を全件再実行し、変更観測後のrun、現在の入力binding、生成output IDを記録する。Digestだけを更新しない。`atlas audit <root> --gate evidence-dependency`で対象漏れとProof／Closure Plan構造縮小を検証する。
18. Integrated Reference Systemのmanifest、10 Scenario runtime結果、action／network／resource streamを持つTraceを固定する。統合成功や統合Traceを個別Behavior Proofとして流用せず、同一Runtime Artifactの複数row共有もしない。
19. Commit `4a0b2df8e2091a963bd0e0e1bbccef9c84b49a45`の`FE_DEPTH_REFERENCE.json`をDigest固定する。Frontendの現状は`incomplete`、1軸`satisfied`、17軸`partial`のまま保持する。
20. 同Referenceの18軸を、Subject自身のAuthority由来denominatorについてBehavior/Variantごとの専用Proof、Oracle、Evidence、Artifact、Traceへ接続しGapを0にする。Frontend固有のTarget、Variant、Test件数を閾値として転用しない。
21. Architecture／Integration Surfaceがあれば複数Behaviorを接続するReference Systemを、Decision Surfaceがあれば複数方式Comparisonを追加する。
22. Skill Evalに加えてDefinitive Skill Routerの112 cellを実Target、Variant、Authority、Runtime Evidence digestへ接続する。mutation authorization、人手Authority decision、stale relockの独立停止を検証し、routing gap、partial、未実施Forward Evalを正直なCompletion limitとして残す。
23. `atlas certificate generate-definitive`でv1とは別のCertificateを発行する。
24. `atlas audit <root> --gate definitive`が`completion_class=subject-definitive`を返すまで公開上は未完とする。

## 互換性分類

- v1 Schema、v1 Audit、v1 Certificate検証は維持する。
- v1の`complete`表示はCLI上`completion_class=bounded-complete`として明示する。
- v2は加法的な別Gateだが、`subject-definitive`という主張へ移行する場合は新しいAuthority Inventory、Proof Matrix、Certificateが必須になる。
