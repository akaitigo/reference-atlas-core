# Versioning Policy

Core ReleaseはSemVer、Manifestは各Fileの`schema_version`、Completion Policyは`completion.policy_version`で独立してVersion化する。v1系列では既存の有効な`schema_version: 1`入力をMinor／Patch Releaseで拒否しない。

## 変更分類

- Patch: 診断改善、Bug修正、文書修正。受理集合とGate意味論を変更しない。
- Minor: Optional Field、Command、Schema、Archetype Overlayの追加。既存v1入力は引き続き有効。
- Major: Required Field追加、Field削除／改名、Enum縮小、既存入力の拒否、Completion Gateの意味論変更。

Gateを強める変更は、既存Certificateへの影響とMigration Guideを同じ変更へ含める。発行済みCertificateは対応ReleaseとPolicy Versionに対して不変であり、新Policyによる再認証を自動継承しない。

Definitive Gate v2はv1 Certificate Schemaを変更せず、独立Manifestと独立Certificateを追加する。旧`complete`の意味は`bounded-complete`履歴として保持し、`subject-definitive`へ昇格しない。
