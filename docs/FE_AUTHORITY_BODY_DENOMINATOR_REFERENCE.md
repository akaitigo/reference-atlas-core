# FE Authority Body Candidate Anchor Denominator参照契約

## 固定参照

Candidate anchor denominatorの参照実装は`frontend-behavior-atlas` commit `841ec2fa399606a10305021a8bcd396713b8cee5`へ固定する。

| Input | SHA-256 |
|---|---|
| `authority/body-inventory.snapshot.json` | `c7a93b40b539f9c6a5207558f1f17e9add0e9ee9788c57f1240c8e6c838070d8` |
| 73 `authority/body-inventory-draft/*.json`のGit tree listing | `4d56e385f656ca397eebe65b3d2d13b39bf965595ff480e47d3ac1a358a8f38e` |
| `scripts/lib/authority-body-inventory.ts` | `04f62a0b63981c62a7ab90f39637c71745642e84a3bdd4404ce715a0163ebe76` |
| `scripts/extract-authority-body-inventory.ts` | `1ce38aae8e9adf2c8095310bf54348eacc482cb19f5edbd7c013a4bf55e6d38c` |
| `scripts/lib/authority-body-baseline.ts` | `0dc48dc9e62fdc9cd8493e9b5827b4cf5948c4b72df3374d5ebcc73ac344009c` |
| `baselines/authority-body-inventory-v1.json` | `22b4eefbadff2e2c0117b70584a2bc5e9ae8ed3997f3dcb35e01358db2ddd82c` |

Core同梱の`profiles/FE_AUTHORITY_BODY_DENOMINATOR_REFERENCE.json`は上記Snapshotとbyte-identicalである。参照状態は84 Source entry、73 unique document、70 digest match、3 stale、15,963 raw anchor、15,963 unclassifiedであり、classified、Human reviewed、Core v2 eligibleはすべて0、`authority_semantics_exhaustive=false`である。

## 型境界

Body Inventoryは`candidate anchor denominator`であり、Authority Surface、Behavior、Target、Depth達成件数ではない。selectorがlocked bodyに対してexhaustiveでも、Authorityの意味論がexhaustiveであることを示さない。raw anchor Artifactは常に`classification_status=pending-human`、`surface_ids=[]`で固定し、本文・抜粋・heading文字列を保存しない。

Core v2 Authority Artifactへの昇格は`authority/review-queue.snapshot.json`、`authority/review-queue-draft/*.json`、`authority/reviews/decisions.json`を通る場合だけ許可する。各decisionはstable anchor IDから次の処理を記録する。

- `include`: 1 anchorから1 qualified Authority Surface
- `exclude`: 1件以上のanchorからSurfaceを生成しない
- `merge`: 複数anchorから1 qualified Authority Surface
- `split`: 1 anchorから複数qualified Authority Surface
- `defer`: 未完として残し、Definitive Gateを通さない

全decisionにreviewer、reviewed timestamp、40文字以上の理由、locked source digest、抽出tool digest、old anchor IDとnew qualified Authority Surface IDのmappingを要求する。`authority_artifact_id.authority_surface_id`形式のnew ID集合と最終Authority Artifact実体が完全一致しない場合は昇格を拒否する。

## 非退行

`baselines/authority-body-inventory-v1.json`はSource、document、selector、15,963 stable anchor IDを固定する。anchor置換は`migrations/authority-body-inventory-v1.json`にold→newの1対多mapping、reviewer、時刻、理由、source/tool digest、実行Proof、Migration Evidenceがある場合だけ許可する。通常Non-regression Baselineにもbody denominatorの進捗を別collectionとして保存し、document/anchor縮小、stale/failed/unclassified/deferred増加、semantic exhaustive後退、専用baseline削除を拒否する。
