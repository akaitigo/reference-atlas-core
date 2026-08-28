# Evidence Dependency Graph契約

`evidence/dependency-graph.json`は、Source、Harness、Runtime、Profileと、それらから生成されたEvidenceの依存関係および実再実行を固定する。Graphの目的は、入力変更後に成果物のDigestだけを更新してClosureを維持することを防ぐことである。

## 入力とDigest

`inputs`の`kind`は`source`、`harness`、`runtime`、`profile`のいずれかである。`members`はRepository相対Pathの集合であり、CoreはPath順に並べた次の配列をCanonical JSONとしてDigest計算する。

```json
[{"path":"path/to/file","digest":"sha256:..."}]
```

`current_digest`はこの集合Digestと一致しなければならない。`baseline_digest`と異なる場合、その入力から到達可能な全outputをstale対象とする。`observed_at`は変更を観測した時刻であり、影響outputのrunはこの時刻以後に開始されなければならない。

## Outputと実再実行

各outputは実FileのDigest、依存node、状態、生成runを持つ。runはCommand、first attempt、開始・完了時刻、現在の全祖先input digest、生成した全output IDを記録する。Runtime／Platform runは実Runtime identityも必要である。

CoreはDefinitive manifestのScenario Proof、Closure Plan、Evidence durability Report、Skill Eval、Skill Routerに加え、存在するlocal／container E2E、Capture、Benchmark、Compatibility、Reference System結果とScenario index配下の全Proofを機械列挙する。これらを`required_outputs`または`outputs`から外すと監査は失敗する。変更入力から到達するoutputがrunの`output_ids`に一つでもなければ、再実行対象漏れとして失敗する。

`status: stale`は移行中の正直な状態としてSchema適合するが、`atlas audit <root> --gate evidence-dependency`とSubject Definitive Gateは失敗する。入力、output Digest、Graph Digestだけを書き換えても、変更観測後のrunと入力bindingがない限りClosureできない。

## Proof／Closure Plan構造

`structures`は次の構造Digestを固定する。

- `scenario-proof-index`: indexのID、Authority denominator、全rowのID、Pattern、Scenario、Path、および各ProofのTarget、Target Set、Behavior scope、VariantとSource Pathの対応。
- `scenario-closure-plan`: Policy、Authority由来baseline、risk順、tranche ID／row membership／上限、completedとplannedを連結した全row順。

実行時刻、Runtime値、Artifact Digest、Gapが専用Proofで閉じたことによるcompleted/planned状態遷移は構造Digestへ入れない。したがって再固定は許可しつつ、Proof削除、Target／Variant／Source Pathの差替え、Closure row削除、順序退避、tranche肥大化は同じ構造として扱わない。構造baseline自体はNon-regression Collectionへ固定される。

このGateは実行結果の意味的正しさを単独で証明しない。各Runtime Report、Scenario Oracle、Artifact、Trace、Compatibility、Benchmark等の専用Gateと併用する。

参照事例は`frontend-behavior-atlas` commit `4cb290ca0343bf9f3a3d6ed3970c2003783a22b2`である。Oracleがreduced-motion時のduration／delay不整合を検出した後、Source修正だけで閉じず、local／Docker E2E、Capture、Benchmark、Compatibility、Integrated Reference System、Scenario Runtime／Proof、Skill Evalを再実行・再固定した。この件数を他Subjectの閾値には用いず、依存先の種類と再実行predicateだけを共通契約として扱う。
