# Atlas CLI v1

```text
atlas validate <manifest...>
atlas audit <atlas-directory>
atlas audit <atlas-directory> --gate definitive
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

既定`audit`はv1の`complete`を`completion_class=bounded-complete`として表示する。`--gate definitive`だけがAuthority Inventory、Behavior Proof、10 Scenario Matrix、Runtime Profile、Reference System、Comparison、Skill Eval v2、独立Certificateを検査し、成功時に`completion_class=subject-definitive`を返す。
