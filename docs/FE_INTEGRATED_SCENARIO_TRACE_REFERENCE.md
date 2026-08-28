# FE Integrated Scenario／Trace参照契約

## 固定参照

統合Traceの参照実装は`frontend-behavior-atlas` commit `deadad18b6588d2c907170a451c3b5cea5ea4192`、専用Scenario Closure predicateはcommit `f2e4c4b19156f8e993f48cdcbce23679ad881924`へ固定する。後者では652 Captureを同一Browser profileで再実行しidentityを固定している。観測値は次のとおりであり、完成条件の件数閾値ではない。

| 項目 | 観測値 |
|---|---:|
| Integrated Scenario Runtime | 10／10 pass |
| Pattern × Scenario row | 85 × 10 = 850 |
| Pattern-specific／bounded runtime row | 431 |
| 明示Gap | 419 |
| exact Pattern＋Scenario＋全Variantの専用Runtime suite row | 2 |
| Scenario Closure Gap | 848 |
| Atomic Authority binding | 0 |
| Completion eligible row | 0 |

この状態は`incomplete`である。431 bounded runtime rowには固定Browser Capture、Benchmark、Compatibilityのidentityを含むが、Capture identityを未実行ScenarioのClosureへ転用しない。850 rowや10 Scenario passもdenominatorと統合境界の実行を示すだけで、Authority本文由来のAtomic Behavior Closureを示さない。

## Subject共通入力

- `integrations/reference-system/manifest.json`は10 Scenario、参加Pattern、runtime boundary、assertionを列挙する。
- `artifacts/reference-system/results.json`はsource／harness／environment、Scenarioごとの実行結果、action／network／resource streamを持つTraceとScreenshotのDigestを固定する。
- `evidence/scenarios/index.json`はSubject自身のAuthority由来Behavior × 10 Scenarioを母集団とし、全rowのDigest、状態、Gap集計を持つ。
- 各`evidence/scenarios/**/*.proof.json`は個別Behavior／Scenarioのsource binding、harness、environment、runtime identity、専用Evidence／Artifact、Atomic Authority binding、明示Gapを持つ。
- 専用Scenario suiteを使用するrowは`scenario_runtime_report`、reportと同一のenvironment、Pattern／Scenario／Variantごとのrecordを持つ。reportはsource／harness digest、retry 0、Trace mode、first-attempt結果を固定する。

## Completion境界

`atlas audit <root> --gate scenario-trace`はstaging状態でもrow、Digest、母集団、集計、統合Runtimeの整合性を検証し、Gap、Authority未結合、Runtime identity不足を`completion_limited=true`として返す。

`atlas audit <root> --gate definitive`は次を追加で要求する。

- Subject自身のHuman Review済みAuthority Inventoryにある全Atomic Behavior × 10 Scenarioが存在する。
- 全rowが専用のpass Runtime／Platform Evidenceと専用Artifactを持つ。
- 各rowの専用Scenario record集合がPattern＋Scenario＋`source_bindings`の全Variantと完全一致し、余分、欠落、重複がない。
- 専用suiteがretry 0で実行され、各recordがfirst attempt pass、Scenario固有Oracle、source digest、action／network／resource stream付きTraceを持つ。
- rowのsource、harness、environment、runtime identity Digestが実体と一致する。
- Atomic Authority bindingがInventoryとAuthority Artifact Digestに一致する。
- Gapが0で、Completion eligible rowが母集団全件と一致する。
- 統合Traceを個別Behavior Artifactにせず、同一Runtime Artifactを複数rowへ共有しない。
- Capture Browser identityの補完、Capture record、統合Reference System TraceだけでScenario Closureを閉じない。

絶対件数、統合成功、Pattern mapping、Trace存在だけをDepth／Completion creditへ変換しない。

残存Gapの段階的実行順とNon-regression契約は`docs/FE_SCENARIO_CLOSURE_PLAN_REFERENCE.md`に記録する。
