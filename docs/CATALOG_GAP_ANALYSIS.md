# Stage 1 Catalog Gap Analysis

## 目的

実在する複数Repositoryの匿名化Inventoryを、個別組織・製品構成・Versionを保存せずに抽象化し、Stage 1 Catalogの構造的な欠落だけを確認した。

原Inventoryは公開成果物へ収録しない。Repository数、正確なVersion、採用件数、内部構成、移行状況、業務用途は、組み合わせにより組織やSystemを推定できるためである。

## 結論

Inventoryに登場する個別技術の大半は、既存のField Atlas内の代表製品またはProduct Packとして扱える。製品名ごとのStage 1 Repository追加は行わない。

一方、次の4分野は独立した完成境界を持ち、既存Catalogでは責任が分散していたためStage 1 Subjectへ追加した。

| 追加Subject | 独立させる理由 |
|---|---|
| `backend-application-engineering` | 言語、API、DBの間にあるApplication Runtime、Framework、DI、永続化、Lifecycle、Error Boundaryの実装責任を閉じるため |
| `build-toolchain-package-engineering` | CIやSupply Chainより前段のBuild Graph、依存解決、Lock、Toolchain、Package、再現性を閉じるため |
| `application-configuration-feature-management` | IaCやSecret管理とは異なる、Application構成のSource、型、安全な変更、Flag、失敗時挙動を閉じるため |
| `technical-documentation-architecture-as-code` | 文書、ADR、Diagram、API記述、実行例の生成・検証・陳腐化防止を技術成果として閉じるため |

## 新規Subjectにしない項目

| Inventory上の項目群 | 収容先 |
|---|---|
| 依存自動更新 | `software-maintenance-migration`、`software-supply-chain` |
| SBOM、依存Lock、署名、Image Scan | `software-supply-chain`、`build-toolchain-package-engineering` |
| Frontend Test | `frontend-behavior`、`software-quality-engineering` |
| Log、Metric、Trace、Collector、Dashboard | `observability-engineering` |
| IaCとCLIのVersion固定 | `infrastructure-as-code-configuration`、`build-toolchain-package-engineering` |
| Backend Framework、ORM、Servlet Container | `backend-application-engineering`のProduct Pack |
| REST、RPC、GraphQL、IDL、Schema、Serialization | `api-integration-contracts` |
| RDB、Cache、Embedded DB、Managed DB | `relational-database-sql`、`operational-data-stores`、該当Product Atlas |
| Queue、Pub/Sub、Reactive Messaging | `messaging-event-driven-systems` |
| Container、Kubernetes、Chart、Overlay、Autoscaling | `oci-container-engineering`、`container-orchestration` |
| Service Mesh、Proxy、Ingress、DNS | `service-networking-traffic-management` |
| OAuth、OIDC、Identity、Authorization、JWT | `identity-authentication-authorization`、`zero-trust` |
| Mobile状態管理、Local DB、Device Plugin | `cross-platform-client`、`android-platform-engineering`、`iot-device-connectivity` |
| Load Test | `performance-capacity-engineering` |
| Coding Agent、MCP、Agent Skills | `agentic-systems`、各Subject AtlasのSkill Closure |

## Coverageへ追加する横断Proof Obligation

各該当Subjectは、製品の利用手順ではなく次の横断的な失敗と回復を証明する。

- VersionまたはDigest未固定を検出し、Releaseを拒否できる。
- Lockなしの依存解決結果が変化することを再現し、Lock導入後の決定性を比較できる。
- 複数世代が共存する場合に互換性境界、移行順序、Rollbackを説明・実行できる。
- 廃止予定のRuntime、Framework、Protocol、PluginをCatalog状態として追跡できる。
- Build、Deploy、RuntimeのVersionが異なる場合にDriftを検出できる。
- 静的解析で不検出だった技術を「不使用」と断定せず、Unknownとして保持できる。
- Third-party Service、Commercial SDK、Managed Serviceは、License、契約、代替、Offline動作の境界を記録できる。

## 公開時のInventory Redaction Gate

外部共有用Inventoryは、技術名だけが一般情報でも、次の組み合わせを公開しない。

- 正確なRepository数と技術別利用数
- 正確なPatch Versionの集合
- Legacyと移行先の組み合わせ
- Cloud、Identity、Network、Data Storeの具体的Topology
- 業務端末、顧客、拠点、内部Serviceを推定できる用途説明
- 内部Assetが存在する事実、命名、Tag規則

比較入力は、Major帯、頻度区分、一般化した役割、Stage 1 Subject IDまでへ丸める。
