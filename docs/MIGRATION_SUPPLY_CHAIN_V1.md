# Supply-chain ecosystem移行

## 変更分類

Core Policy 1.0.0内の後方互換なSchema拡張である。既存の`go-module`は意味を変えず、JVM／JavaScript Subjectが依存Packageを正しいecosystemで表現できるよう`maven-package`と`npm-package`を追加する。

## 移行

1. Maven lock由来のPackageは`kind: maven-package`とし、`name`、`version`、`license`をSPDX Packageと一致させる。
2. npm lock由来のPackageは`kind: npm-package`とし、同じくSPDX Packageへ接続する。
3. `source`には可能ならPackage URL（purl）を使用する。
4. `go.mod`は`go-module`を宣言するAtlasだけで必須とする。Maven/npmだけのSubjectへ空の`go.mod`を追加しない。
5. 既存の`source`、`asset`、`github-action`等はDependency closureの照合対象へ暗黙昇格しない。

Package identityは`name`と正規化した`version`の組である。同じPackageの複数Versionがlock closureに共存する場合は、それぞれを独立Artifact／SPDX Packageとして保持する。

この変更はCompletion Gateを弱めない。SPDXに列挙した第三者Packageは、対応ecosystemの第三者Manifest実体とversion／licenseが一致しなければ引き続き拒否される。
