# 権利・出典・公開Gate

この文書は運用Policyであり、法律相談ではありません。訴訟や権利主張の可能性をゼロにはできません。公開物の出所を追跡し、不明なものをReleaseしないことでリスクを下げます。

## 既定License

- 独自コードと独自文書：Apache-2.0
- 独自に生成した画像等：原則Apache-2.0。再利用性のためCC BY 4.0等を選ぶ場合はFile単位で明示
- 第三者成果物：元のLicenseを維持し、`LICENSES/`と`third_party/manifest.yaml`へ記録

`LICENSE`、`NOTICE`、SPDX Identifier、SBOM、第三者ManifestをRelease必須とします。

## Separate Provenance Domains

以下を別々に判定します。コードのLicenseが適合していてもAsset利用が許可されるとは限りません。

- Source CodeとSnippet
- 仕様・文書の引用
- Image、Texture、Video、Audio
- Font、Icon
- Dataset、Model、Weight
- 3D Model、CAD、Board Design
- Brand Name、Logo、Trademark
- Generated Artifact

## Publication拒否条件

- LicenseまたはCopyright Holderが不明
- 利用条件が取得後に変化し、固定した証拠がない
- 社内、顧客、個人、秘密、Credentialを含む
- 公式文書やSourceを必要以上に転載している
- 製品Logoや特徴的なCompositionを許可なく同梱している
- Datasetの収集同意、再配布、個人情報処理が不明
- Model Weightの利用・再配布条件が不明
- 依存Licenseの義務をRelease Artifactが満たさない
- 商標上「公式」「認定」「提携」と誤認させる

## 自動Gate

- REUSE／SPDX Compliance
- Dependency License Scan
- SBOM生成と差分確認
- Secret Scan
- BinaryとAssetのAllowlist、Digest、Size確認
- Source URL、Version、取得日、LicenseのSchema検証
- 未登録File、未登録Asset、未知Licenseの拒否
- Generated Artifactの再生成一致

## Human Gate

- Similarity Review
- TrademarkとRepository名の確認
- DatasetとModel Cardの確認
- Security Labの公開危険度Review
- 第三者からの削除要請・訂正要請への対応

## Contributor

外部ContributionはDCO Sign-offを必須にします。Contributorは、自身が提出する権利を持ち、RepositoryのLicenseで配布できることを表明します。自動Gate通過だけではMergeせず、Repository maintainerのReviewを必要とします。
