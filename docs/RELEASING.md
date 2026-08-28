# Release手順

1. `VERSION`、CLI Version、`CHANGELOG.md`、`skill.package.yaml`の`atlas_release`を同じSemVerへ更新する。
2. Schema変更を`docs/VERSIONING.md`で分類し、Breakingの場合はMigration Guideと互換期限を追加する。
3. 一次資料Lock、第三者Manifest、SPDX SBOM、Provenanceを更新する。
4. `make release-check`をクリーンなLocal環境で実行する。
5. `atlas certificate generate . --issued-at <RFC3339> --commit <source-commit-sha>`でCertificateを再生成し、再度`make release-check`を実行する。
6. `git status --short`が空であること、全CommitにDCO `Signed-off-by`があることを確認する。
7. `vMAJOR.MINOR.PATCH`の署名付きTagを作り、`bin/atlas`とCertificateとSBOMをRelease Artifactにする。

`subject-definitive`を公開するSubjectは、上記bounded Releaseを履歴固定した後、`atlas audit . --gate evidence-durability`、`atlas audit . --gate evidence-dependency`、`atlas audit . --gate non-regression`、`atlas audit . --gate definitive`、`atlas certificate generate-definitive`、`atlas certificate verify-definitive`を別途実行する。Depth ParityのGapが0でない場合、成功Evidenceの原子的公開を検証できない場合、または入力変更後の再実行対象が残る場合は発行せず、v1 Certificateをv2 Certificateで上書きしない。

Certificateの`commit`はCertificate生成元のSource Commitを指す。Certificate自身を同じCommitへ含める自己参照は行わず、次のCommitまたはRelease Artifactとして固定する。

GitHub公開は有効な認証と明示的な公開対象が確認できた場合だけ行う。認証できない場合はDCO付きLocal Commitまで進め、Push／Release作成を阻害要因として報告する。
