# FE Authority Locator Extraction参照契約

## 固定参照

Authority Locator Extractionのmetadata-only参照実装は、`frontend-behavior-atlas` commit `cabf687bab769b17928d950acc416f3f77eb4ca3`へ固定する。

| Input | SHA-256 |
|---|---|
| `authority/extraction.snapshot.json` | `4708796bd7c05c3ae750c87a9d01291f66ea2a7cc1f54731172638bfffd9a629` |
| 84 `authority/surfaces-draft/*.json`のGit tree listing | `782008e8e1664d3c05b17c5f1b23f390532decfc40a286b4eccaab73a3e30203` |
| `scripts/lib/authority-extraction.ts` | `d0efb14e943384b363f6596cff32371368ad69d4aa319de4c7f5ecf189cb8c7c` |
| `scripts/extract-authority-surfaces.ts` | `b17bf28e55dd0d778a2b697f3242631315e18e3a0f717431c97920590a7e98b2` |
| `scripts/verify-authority-extraction.ts` | `420acbe08bff848786c4a28febb4443c671afdd53ff1643413317a9ce175d9aa` |

Core同梱の`profiles/FE_AUTHORITY_EXTRACTION_REFERENCE.json`は上記Snapshotとbyte-identicalである。CoreのGo Verifierは84 Draftすべてを新Schemaで検証し、FE Repository上で次の状態を再計算した。

`locked=84 matched=81 stale=3 failed=0 candidate_edges=235 classified_edges=235 unclassified_edges=0 missing_locators=0 deferred_locators=4 text_exhaustive=false human_reviewed=0 core_v2_eligible=0`

## 保存境界

Extraction Artifactには次の情報だけを保存する。

- Source URL、HTTP status、final URL、content type、response size
- locked/fetched body digestと一致状態
- Locator、context offset、offset unit
- context digest、heading digest
- Domain reference metadata digest
- Domain側のID、Variant、Surface候補、未Review分類
- fetch、stale、locator評価の状態

第三者のresponse body、本文、抜粋、引用、heading文字列、context文字列、HTML、Markdownは保存しない。SnapshotとDraft Schemaは全階層をallowlistにし、未知keyや`body`、`text`、`excerpt`、`heading`、`content`、`html`等の本文fieldを拒否する。

## 完了意味論

`reference_edges_classified=235`と`unclassified_reference_edges=0`は、既存Domain reference edgeをcandidateへ損失なく投影したことだけを表す。`authority_text_surfaces_exhaustive=false`である限り、Authority本文全体からSurfaceを抽出したことにはならない。

Coreの通常Authority Extraction Auditはこの未完状態を正直なstaging artifactとして検証する。Subject Definitive Gateは次を独立に要求し、1項目でも未完なら失敗する。

- stale body 0
- fetch failed 0
- fragment-not-found 0
- locator evaluation deferred 0
- unclassified reference edge 0
- `authority_text_surfaces_exhaustive=true`
- Human reviewed Surface 1件以上
- Core v2 eligible Surface 1件以上
- `status=eligible-for-core-v2`
- eligible件数とreview済みAuthority Surface Artifact実体の一致

候補件数や分類率だけでは上記条件を代替できない。
