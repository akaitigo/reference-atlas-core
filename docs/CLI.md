# Atlas CLI v1

```text
atlas validate <manifest...>
atlas audit <atlas-directory>
atlas audit <atlas-directory> --gate definitive
atlas audit <atlas-directory> --gate non-regression
atlas audit <atlas-directory> --gate authority-extraction
atlas audit <atlas-directory> --gate authority-body
atlas audit <atlas-directory> --gate authority-review
atlas audit <atlas-directory> --gate authority-review-export
atlas audit <atlas-directory> --gate authority-relock
atlas audit <atlas-directory> --gate skill-router
atlas audit <atlas-directory> --gate scenario-trace
atlas audit <atlas-directory> --gate scenario-plan
atlas audit <atlas-directory> --gate evidence-durability
atlas audit <atlas-directory> --gate evidence-dependency
atlas baseline generate <atlas-directory> <output> --commit SHA [--captured-at RFC3339]
atlas scaffold <directory> <atlas-id> <日本語title> [epoch]
atlas generate skill-reference <atlas-directory>
atlas migrate v1 <atlas-directory>
atlas migrate definitive-v2 <atlas-directory>
atlas certificate generate <atlas-directory> [--issued-at RFC3339] [--commit SHA]
atlas certificate verify <atlas-directory>
atlas certificate generate-definitive <atlas-directory> [--issued-at RFC3339] [--commit SHA]
atlas certificate verify-definitive <atlas-directory>
atlas version
```

`scaffold`は空の出力先だけへ`incomplete` Subjectを生成する。`generate skill-reference`はCanonical `coverage.yaml`から派生Referenceを再生成する。`migrate v1`は既存Fileを上書きせず、不足するMasteryと`migrations/core-v1.yaml`だけを作る。`migrate definitive-v2`はv1 Certificateを不変履歴へコピーして未完Actionを記録するだけで、自己申告ScopeからAuthority Inventoryを捏造しない。

既定`audit`はv1の`complete`を`completion_class=bounded-complete`として表示する。`--gate definitive`だけがAuthority Inventory、Atomic Behavior/Variant、Behavior Proof、10 Scenario Matrix、`FE_DEPTH_REFERENCE` Parity、Runtime Profile、Artifact/Trace、Reference System、Comparison、Skill Eval v2、Non-regression、独立Certificateを検査し、成功時に`completion_class=subject-definitive`を返す。

`baseline generate`はTest関数、`examples/**`、`labs/**`、`testdata/**`、`evals/run.*`、主要`script` harness、Target／Target Set、Claim／Proof、Evidence、Source、Authority Extraction、Skill Eval、Required Profile、Verification Matrix、CI Jobを固定する。`--gate non-regression`は削除、skip、Scope退避、Target弱化、Assertion／閾値／Matrix／CI縮小、Runtime Evidenceのstatic化、失敗Evidenceの消去を拒否する。Authority Extractionはstale/deferredからmatched/located/reviewed/eligibleへの単調強化だけを許し、同一失敗状態の上書き、candidate contract削除、exhaustiveの後退を拒否する。

`--gate authority-extraction`はmetadata-only SnapshotとSourceごとのDraftをSchema、Digest、Source Lock、fetch状態、Locator状態、集計値へ照合する。このGateの成功はstaging artifactの整合性を表し、`text_exhaustive=false`やHuman review 0をCompletionへ昇格しない。`--gate definitive`だけが全open stateのClosureを要求する。

`--gate authority-body`は全locked documentのcandidate anchor denominatorと専用stable-ID baselineを検証する。`--gate authority-review`はQueue/Batch/Ledger、stale・unavailable hold、manual-primary-source、source/tool/locator binding、old→new mapping、Surface/Atomic resultを検証する。raw anchor、Queue、batch、clusterの件数はDepth達成へ算入しない。`--gate authority-review-export`はpriority packetのread-only投影を検証し、machine proposalとHuman decisionを分離し、decision書込やHuman review昇格を拒否する。

`--gate authority-relock`はstale candidate reportとSource Lockを照合する。Lockが変わった場合だけ、人の明示選択、旧/new Lock、全旧IDのmapping、実行Proof、Migration Evidence、Non-regression Evidenceを持つ専用decisionを要求する。

`--gate skill-router`は8 Outcome × 14 Surfaceのroute、実Target、Variant file digest、Authority Source Lock、Evidence Artifact、mutation authorization、5つのfail-closed境界を照合する。routing gap、partial Coverage、未実施Forward Evalは`completion_limited=true`として表示し、matrixの`result=pass`だけではSkillやSubjectをcompleteにしない。

`--gate scenario-trace`はAuthority由来Behavior × 10 Scenarioのdenominator、個別row、source／harness／environment／runtime identity、統合Reference Systemの実行結果とTrace digestを照合する。`runtime_identity`はboundedなCapture等を含む実行identity、`dedicated_scenario_runtime_rows`はexact Pattern＋Scenario＋全Variantをretry 0で駆動した専用suite Closureとして分けて表示する。stagingではGapを保持したまま集計の真正性を検証できるが、`--gate definitive`は全rowに専用Runtime Artifact、Scenario固有Oracle、Atomic Authority bindingを要求し、Capture補完と統合Traceの個別Proofへの流用を拒否する。

`--gate scenario-plan`は残存`pattern-specific-gap`をrisk順に完全包含し、1 tranche最大4 Pattern row、全Variant実行数、次trancheを再計算する。Plan row削除、risk順から後段への退避、tranche肥大化、Proof／Variant／Gap契約の弱化を拒否する。

`--gate evidence-durability`は専用Runtime Reportの公開Profileと、Report／Trace／Screenshotからなる1世代の完全Artifact集合を検証する。全run pass以外の公開、部分上書き、成功Evidence消去、新旧世代混在、directory外参照を拒否する。

`--gate evidence-dependency`はSource／Harness／Runtime／ProfileからEvidenceへの推移依存を検証する。入力変更後はlocal／container E2E、Capture、Benchmark、Compatibility、Reference System、Scenario Proof等の到達可能な全outputをstale対象にし、現在の入力digestへ結ばれた変更観測後の実再実行が揃うまで失敗する。Digestだけの再固定、再実行対象漏れ、Proof／Closure Planの構造縮小を拒否する。
