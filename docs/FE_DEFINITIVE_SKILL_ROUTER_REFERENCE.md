# FE Definitive Skill Router参照契約

参照元は`frontend-behavior-atlas` commit `8a9e34a89a55cc53702032783c06ede7246a286f`の`evals/fe-behavior-advisor.definitive-skill-eval.json`である。

同Artifactは8 Outcome × 14 Surfaceの112 cellを持ち、Router契約評価は112 pass、実Routeは110、Mastery routing gapは2、Coverageは全112 cellが`partial`である。曖昧Query、未知Query、未許可Mutation、人手Authority decision、stale relockの5境界はfail-closedだが、独立Agent Forward Evalは未実施である。したがって参照状態はSkill/Subject Completionではない。

Core契約は件数を閾値として転用しない。各Subjectは自身のAuthority由来TargetとVariantへ112 cellを接続し、file/source/evidence digestを実体照合する。Source bindingのURLとdigestは必須だが、`bytes`は固定response/body metadataを取得できる場合だけ記録し、第三者本文の保存量やDepth達成を表さない。`result=pass`はRouterが期待された停止・routeを返したことだけを示す。全Target covered、routing gap 0、partial 0、5境界pass、独立Forward Eval完了、Human Authority Queue closure、stale relock手順の独立性が揃う場合だけDefinitive Skill RouterをCompletionへ使用できる。
