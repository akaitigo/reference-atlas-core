# FE Scenario Closure Plan参照契約

## 固定参照

段階的Closure Planの参照実装は`frontend-behavior-atlas` commit `8329cb3c09e034b36b8cbe35021f7dd7b52d4140`の`evidence/scenarios/closure-plan.json`へ固定する。

参照時点では、commit `f2e4c4b19156f8e993f48cdcbce23679ad881924`由来の419 Gap baselineに対して専用Runtime完了rowが4、残存`pattern-specific-gap`が417、計画trancheが107である。残存rowの内訳はSecurity 67、Refusal 56、Failure 45、Recovery 40、Migration 81、Operations 72、Boundary 41、Performance 0、Compatibility 0、Normal 15である。

これらの絶対件数はFrontendの観測値であり、別Subjectの達成閾値にしない。Coreは各Subjectの`evidence/scenarios/index.json`から残存row、scenario別母集団、tranche数を再計算する。

## 共通Policy

- risk順はSecurity、Refusal、Failure、Recovery、Migration、Operations、Boundary、Performance、Compatibility、Normalとする。
- 同一Scenario内はPattern IDの安定順とし、後段への順序退避を許さない。
- 1 trancheは最大4 Pattern rowとし、空tranche、5 row以上への肥大化、別Scenario混在を許さない。
- 各rowは元Proofのpath／digest、全Variant ID、現在のGapと一致する。
- ClosureにはPattern＋Scenario＋全Variantの実駆動、first attempt、retry 0、専用Runtime identity／Oracle、Variant別Trace／Screenshot、Action／Network／Resource stream、source／harness digestを要求する。
- metadata-only、Capture再利用、Integrated Trace再利用、mock／static Runtimeへの置換を禁止する。
- Authority Atomic、外部Profile、Agent Forward Eval等の独立未完軸をScenario Runtimeの進捗で閉じない。

## Non-regression

BaselineはPolicy、Plan rowのordinalとtranche所属、trancheのordinal／row集合／Variant実行数、completed rowを固定する。このため、Plan row削除、risk順の変更、tranche肥大化、契約弱化は失敗する。正当なClosureでPlan rowを置換する場合も、通常のNon-regression Mapping、Runtime実行Proof、Migration Evidence、理由を必要とする。
