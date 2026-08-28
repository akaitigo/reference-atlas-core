# Evidence durability共通契約

## 固定参照

参照実装は`frontend-behavior-atlas` commit `7175de4305afb308722d5b83475e91c18da64957`のPattern Scenario Reporterと`artifacts/pattern-scenarios/results.json`へ固定する。参照時点の公開世代は1 Report、16 Trace、16 Screenshotの33 Artifactである。この件数はFrontendの観測値であり、他Subjectの達成閾値には使用しない。

## 必須Profile

専用Runtime Reportの`retention_contract`は次の3値を変更せず保持する。

```json
{
  "publish_on": "full-run-passed",
  "failed_run": "retain-prior-success",
  "swap": "staged-directory-rename-with-rollback"
}
```

Reporterは全Artifactを公開先の外にあるstaging directoryへ生成する。全runがpassした場合だけ、直前成功directoryをbackupへrenameし、staging directoryを公開先へrenameする。置換に失敗した場合はbackupを元の公開先へ戻す。failed、flaky、skipped、no-match、Reporter errorは公開条件を満たさず、直前成功世代へ書き込まない。

## 機械監査

`atlas audit <root> --gate evidence-durability`はReportを一つの公開世代のManifestとして扱う。

- Report自体、全recordのTrace、Screenshotを公開directoryの完全なファイル集合として照合する。
- Reportにない旧Artifact、staging残骸、backup残骸、symlinkを拒否する。
- Artifact path共有、directory外参照、欠落、byte数またはdigest不一致を拒否する。
- Report全体と各recordがretryなしのfirst-attempt passであることを要求する。
- TraceごとにAction、Network、Resource streamを要求する。
- Reportのrow、variant、pass／failure集計を実recordから再計算する。

この監査により、Reportだけの部分上書き、Traceだけの部分置換、失敗runによる直前成功Evidenceの消去、新旧Artifact混在はGateを通らない。Subject Definitive Certificateは公開世代ReportのDigestを`evidence_durability_digest`として固定する。

## Negative fixture

Core fixtureは参照と同じ33 Artifactについてfailed runとno-match runの前後Digest集合が完全一致することを確認する。さらに部分上書き、成功Artifact削除、旧Artifact混入、failed Report公開、Profile弱化を個別に注入し、共通Gateが拒否することを検証する。
