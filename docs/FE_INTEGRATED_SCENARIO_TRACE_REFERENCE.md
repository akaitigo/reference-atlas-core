# FE Integrated Scenario／Trace参照契約

## 固定参照

入力契約の参照実装は`frontend-behavior-atlas` commit `deadad18b6588d2c907170a451c3b5cea5ea4192`へ固定する。参照時点の観測値は次のとおりであり、完成条件の件数閾値ではない。

| 項目 | 観測値 |
|---|---:|
| Integrated Scenario Runtime | 10／10 pass |
| Pattern × Scenario row | 85 × 10 = 850 |
| Pattern-specific row | 429 |
| Runtime identity row | 170 |
| 明示Gap | 421 |
| Atomic Authority binding | 0 |
| Completion eligible row | 0 |

この状態は`incomplete`である。850 rowや10 Scenario passはdenominatorと統合境界の実行を示すが、Authority本文由来のAtomic Behavior Closureを示さない。

## Subject共通入力

- `integrations/reference-system/manifest.json`は10 Scenario、参加Pattern、runtime boundary、assertionを列挙する。
- `artifacts/reference-system/results.json`はsource／harness／environment、Scenarioごとの実行結果、action／network／resource streamを持つTraceとScreenshotのDigestを固定する。
- `evidence/scenarios/index.json`はSubject自身のAuthority由来Behavior × 10 Scenarioを母集団とし、全rowのDigest、状態、Gap集計を持つ。
- 各`evidence/scenarios/**/*.proof.json`は個別Behavior／Scenarioのsource binding、harness、environment、runtime identity、専用Evidence／Artifact、Atomic Authority binding、明示Gapを持つ。

## Completion境界

`atlas audit <root> --gate scenario-trace`はstaging状態でもrow、Digest、母集団、集計、統合Runtimeの整合性を検証し、Gap、Authority未結合、Runtime identity不足を`completion_limited=true`として返す。

`atlas audit <root> --gate definitive`は次を追加で要求する。

- Subject自身のHuman Review済みAuthority Inventoryにある全Atomic Behavior × 10 Scenarioが存在する。
- 全rowが専用のpass Runtime／Platform Evidenceと専用Artifactを持つ。
- rowのsource、harness、environment、runtime identity Digestが実体と一致する。
- Atomic Authority bindingがInventoryとAuthority Artifact Digestに一致する。
- Gapが0で、Completion eligible rowが母集団全件と一致する。
- 統合Traceを個別Behavior Artifactにせず、同一Runtime Artifactを複数rowへ共有しない。

絶対件数、統合成功、Pattern mapping、Trace存在だけをDepth／Completion creditへ変換しない。
