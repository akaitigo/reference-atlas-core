# Mastery Contract v1移行

## 変更分類

Pre-release中のBreaking Schema変更である。Subject Atlasは`atlas.yaml`へMastery参照を追加し、Repository Rootへ`mastery.yaml`を追加する。

```yaml
mastery:
  manifest: mastery.yaml
  contract_version: 1.0.0
```

Apache-2.0 Publication Closureを機械監査するため、`license.sbom: sbom.spdx.json`も必須になった。

## 移行手順

1. 既存Coverage Target Setを8 Outcomeへ割り当てる。
2. 14 Surfaceをすべて列挙する。
3. 適用SurfaceはTarget Setと必須成果物を指定する。
4. 非適用Surfaceは空の`target_sets`と具体的理由を持つ`not-applicable`にする。
5. `atlas validate`で5 Manifestを検証する。
6. `atlas audit <repository-root>`でID、Epoch、Target Set、Routerの横断整合性を検証する。
7. `complete`候補ではAuthority Digest、Evidence実体、Required Profile、License、SBOM、Skill、Certificateまで監査する。

既存のDomain固有Manifest、Test、Evidenceは削除しない。Masteryはそれらの正本を置き換えず、「この分野で答えられるべき問い」と既存成果物を接続する。
