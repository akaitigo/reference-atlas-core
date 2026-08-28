# 会社リポジトリ技術棚卸し用プロンプト

以下を、会社の全プロジェクトリポジトリが入ったディレクトリを開いているClaudeへ、そのまま渡してください。読み取り専用の静的棚卸しです。会社の規程、秘密保持、外部AIへの持出し規則が最優先であり、許可されない情報は共有しません。

```text
このディレクトリ直下および配下にある会社プロジェクトのリポジトリを、読み取り専用で静的解析し、実際に使われている技術を棚卸ししてください。

目的:
- 個人で構築する「技術実証アトラス」のStage 1技術分野カタログと比較するため
- 製品名の羅列ではなく、技術分野、役割、Version帯、利用件数、技術間の組み合わせを把握するため
- 出力は会社を特定できない、外部共有可能な匿名化済み情報だけにするため

絶対条件:
1. すべて読み取り専用。ファイル変更、生成、Commit、外部送信、ネットワークアクセス、ビルド、テスト、依存取得、コンテナ起動、スクリプト実行は禁止。
2. 会社名、組織名、リポジトリ名、ディレクトリ名、絶対・相対パス、Branch名、Commit SHA、内部URL、Domain、IP、Port、Cloud Account ID、Project ID、Cluster名、Bucket名を出力しない。
3. 顧客名、業務名、商品名、画面名、Endpoint、Table名、Topic名、Queue名、個人情報、業務データ、ソースコード断片を出力しない。
4. Token、Password、Cookie、秘密鍵、証明書、接続文字列、環境変数の値、Secret名、脆弱性の具体的悪用情報は、発見しても内容・存在場所・識別子を出力しない。
5. `.env*`、秘密鍵、証明書、認証設定、実データ、生成物、Vendor、Build Cacheは開かない。秘密らしい内容を見つけたら直ちにそのファイルの解析を中止する。
6. 会社の規程上、外部共有不可または判断不能な情報は出力から除外する。除外した事実は集計数だけで表す。
7. 推測と確認済み事実を分ける。VersionはLockfileやManifestで確認できた範囲だけを記載し、Patch Versionが個別案件の指紋になり得る場合はMajor帯またはMajor.Minor帯へ丸める。

調査対象の信号:
- 依存ManifestとLockfile
- Build設定、Workspace設定、Compiler設定
- Containerfile/Dockerfile、Compose、Dev Container
- CI/CD設定、GitOps、Release設定
- IaC、構成管理、Kubernetes Manifest、Helm/Kustomize
- DB MigrationとSchema管理ツール。ただし業務上のTable/Column名は読取・出力しない
- Source Importは、Manifestだけでは利用確認できない場合の補助信号。コード断片や内部Module名は出力しない
- README/ADR等は技術名と一般的役割だけを補助確認する

除外:
- `.git`, `node_modules`, `vendor`, build/dist/target, coverage, cache, generated, binary, archive
- Fork、Mirror、Tutorial、検証用一時リポジトリと明示されるもの
- コメントや文書に名前だけ登場し、Manifest・設定・Importの裏付けがない技術

分類:
- 計算・ソフトウェア基礎
- 計算機システム
- ソフトウェア設計・開発
- フロントエンド・クライアント
- クラウド・デリバリー・運用
- データ・メッセージング
- セキュリティ・プライバシー
- AI・データサイエンス
- 組込み・物理・メディア・対話
- 新興・専門計算

手順:
1. リポジトリ境界を数える。名前は保持・表示せず、内部でrepo-001のような一時番号に置換する。
2. 各リポジトリの信号から技術名、一般的役割、Version帯を抽出する。
3. 同義語を公式の一般名へ正規化する。フレームワーク、ランタイム、DB、Broker、CI/CD、IaC、Observability、Securityも落とさない。
4. 宣言だけ、Build時利用、Runtime利用、Test専用、運用基盤を役割として区別する。
5. 同一技術は集約し、利用リポジトリ数だけをusage_countにする。どのリポジトリかは出力しない。
6. 技術間連携は、一般化した関係と件数だけを集約する。内部サービス構成は再現しない。
7. 下記Stage 1 Subject候補へ対応付ける。該当しなければ空配列にし、勝手な内部情報を追加しない。
8. 最後にRedaction監査を行い、禁止情報が一つでも残れば削除してから出力する。

出力は次の2部だけ:

A. 日本語サマリー
- 解析対象リポジトリ数、除外数
- 分類別の技術数
- 高頻度技術 上位20件: 技術名、一般的役割、Major帯、利用件数だけ
- 主要な技術組み合わせ 上位20件: 一般化した関係、利用件数だけ
- Stage 1カタログへ新規追加を検討すべき技術分野。ただし会社固有情報を根拠欄へ書かない
- 静的解析では判定できなかった点

B. `company-inventory.yaml`
- 下記形式だけをYAMLコードブロックで出力する
- コメント、ファイルパス、リポジトリ識別子、証拠断片を含めない

schema_version: 1
inventory_id: company-technology-inventory-sanitized
generated_at: <ISO 8601。時刻自体が機微なら日付の00:00:00+09:00>
scope:
  root_kind: multi-repository
  repository_count: <整数>
  excluded_repository_count: <整数>
redaction:
  confirmed: true
  removed_categories:
    - organization
    - repository-names
    - filesystem-paths
    - internal-urls
    - customer-data
    - personal-data
    - secrets
    - source-code
    - business-domain
    - vulnerability-details
technologies:
  - id: <一般名をkebab-case化したID>
    name: <一般公開されている技術名>
    category: <上記10分類の英語ID>
    roles: [<一般化した日本語の役割>]
    versions: [<MajorまたはMajor.Minor帯。安全に出せなければ空配列>]
    usage_count: <利用リポジトリ数>
    signals: [dependency-manifest|lockfile|build-config|container|ci-config|iac|runtime-config|source-import|database-migration|documentation]
    confidence: high|medium|low
    stage1_subjects: [<対応する一般技術分野ID。判断不能なら空配列>]
    notes: <任意。会社固有情報を含まない短い注意>
integrations:
  - from: <technologies内のID>
    to: <technologies内のID>
    relation: <一般化した日本語の関係>
    usage_count: <確認できたリポジトリ数>
    confidence: high|medium|low
unknowns:
  - <静的・匿名化解析上の限界だけを書く>

出力直前チェック:
- 禁止された名前、パス、URL、ID、データ、コード、秘密がゼロである
- technologiesのidが重複していない
- integrationsのfrom/toがtechnologies内に存在する
- usage_countは1以上
- redaction.confirmedがtrue

条件を守れない場合は棚卸し結果を出さず、「会社規程または匿名化要件により安全な出力を作成できない」とだけ答えてください。
```

受け取ったYAMLはファイルへ保存してから、次で検証できます。

```bash
go run ./cmd/atlas validate company-inventory.yaml
```
