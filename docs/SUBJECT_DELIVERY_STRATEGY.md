# Subject Atlas実装順序

## 目的

対象分野は`catalog/stage1.yaml`で固定する。Repository数を増やすことではなく、各Subjectを「その分野ならここを見ればよい」状態まで閉じることを優先する。

## Wave 0 — Completion System

`reference-atlas-core`のSchema、Mastery、Coverage、Evidence、Skill、Publication、Auditを先に固定する。Gateが変わるたびに97 Repositoryを手戻りさせないため、Core v1 Release前に代表Atlasで契約を破壊検査する。

## Wave 1 — 代表Atlas

既に着手済みの次を、Archetype別のGolden Implementationとして完成させる。

- `frontend-behavior-atlas`: field + construction
- `kotlin-reference-atlas`: product + language-platform
- `flutter-reference-atlas`: product + language-platform
- `postgresql-reference-atlas`: product
- `rabbitmq-reference-atlas`: product
- `zero-trust-reference-atlas`: field
- `argocd-reference-atlas`: product

追加で、`specification`と`hardware-simulation`の代表を一つずつ選び、同じCompletion Gateが異なる証明方法でも成立することを確認する。

## Wave 2 — Enabling Subjects

他分野から頻繁に参照される基盤を先に閉じる。

- 計算・言語・Software Engineering基礎
- OS、Network、Distributed System、Storage、Database
- API・契約、Quality Engineering、Security Engineering
- Container、CI/CD、Supply Chain、Observability

依存先は固定Releaseで参照し、未完成RepositoryのDefault Branchへ依存しない。

## Wave 3 — Cluster Closure

Client、Cloud・Operations、Data・Messaging、Security・Privacy、AI、Physical・Media、Emergingの各Clusterを、Mastery Surface単位で閉じる。Cluster内で同じ説明やLabを複製せず、共通PrimitiveとSubject固有責任を分離する。

## Wave 4 — Interoperability

完成済みSubject Releaseだけを`atlas-interoperability-lab`で組み合わせる。Subject単体のCompletionと組合せのCompletionを混ぜない。

## Work-in-Progress制限

- Golden Implementation確立前に全Repositoryを一斉生成しない。
- 新規着手より、`incomplete` Atlasの必須Mastery Surfaceを閉じることを優先する。
- Scaffold、目次、空Directory、文章量を進捗として数えない。
- 進捗は`covered` Target、再生成可能Evidence、Skill Eval、閉じたMastery Surfaceで測る。

## 各Subjectの着手完了条件

着手時にAuthority、Scope、Mastery、Coverage、Environment、Skill Eval計画を固定する。完成時には8 Closureと`atlas audit`を通し、固定EpochのCompletion Certificateを発行する。上流更新は新Epochで扱い、完成済みReleaseを後から書き換えない。
