# Agent Skill契約

## 目的

最終状態では、各技術分野についてAgentが「何を使うべきか」「どう実装・診断・移行するか」を、Atlasの証拠へ戻りながら判断できます。

SkillはAtlasの内容を別の文章として複製しません。Canonical Manifestと生成Referenceを検索し、必要なLab、判断表、Runbook、Evidenceへ段階的に誘導します。

## 1分野1 Router

通常はSubject Atlasごとに一つのRouter Skillを公開します。

```text
.agents/skills/<subject>/
├── SKILL.md
├── agents/openai.yaml
├── scripts/
├── references/
└── assets/
```

数千個のMicro SkillをGlobal一覧へ登録しません。設計、実装、診断、復旧、移行、ReviewはRouterのModeと、必要時だけ読むReferenceへ分けます。

## Progressive Disclosure

1. `name`と`description`で正確に発見する。
2. `SKILL.md`は目的、Mode選択、重要な境界だけを持つ。
3. Capability Index、Decision Matrix、Runbookは`references/`へ置く。
4. 決定論的な検索・検証・変換は`scripts/`へ置く。
5. 出力へコピーするTemplateだけ`assets/`へ置く。

一般論、モデルが既に判断できる説明、巨大な公式Manualの複製はSkillへ入れません。

## Portable CoreとAdapter

```text
portable/
├── SKILL.md
├── references/
├── scripts/
└── assets/

adapters/
├── codex/
├── claude-code/
├── cursor/
└── generic-agent-skills/
```

Portable CoreはAgent固有Tool名やInstall Pathへ依存しません。AdapterはUI Metadata、Invocation Policy、Tool Mapping、Sandbox差分だけを所有します。

Codexで動くことは他Agentでの完全互換を保証しません。探索Directory、Tool名、Permission、MCP、Shell、Resource形式をAdapter Evalで検証します。

## PackageとVersion

`skill.package.yaml`は次を固定します。

- Atlas IDとRelease
- Router IDとPath
- 生成元Manifest
- 対応Adapter
- Eval Pathと最低合格率

Skill Releaseは対応するAtlas Releaseを変更せず参照します。Atlas更新なしにSkillの技術的主張だけを更新してはいけません。

OpenAIのSkills APIはSkill Bundle、Version、Default Versionを扱えるため、将来のAPI Project配布先として利用できます。ただしGitHub RepositoryをSource of Truthとし、外部Registryは派生配布先として扱います。

## Install UX

`technology-atlas-skills`が次を提供します。

```text
atlas skills list
atlas skills install <subject>
atlas skills install --all
atlas skills install --target codex <subject>
atlas skills update
atlas skills verify
atlas skills rollback <subject> <version>
atlas skills uninstall <subject>
```

Installerは既存Skillを黙って上書きしません。変更予定、Source、Version、Target Directory、Conflictを表示し、実行前に利用者の承認を得ます。

## Eval

最低限、以下を検証します。

- 正しいSubject／CapabilityへRouteする。
- 近いが異なる技術を区別する。
- Coverage外を既存機能として捏造しない。
- 推奨、条件付き、非推奨、廃止を区別する。
- Authorityより外部記事を優先しない。
- 実行が必要なTaskで再現コマンドとEvidenceを使う。
- 変更権限がないTaskで勝手に実装・公開しない。
- Security Taskで対象と許可範囲を守る。

文言一致ではなく、選択、Evidence、Observable Outcomeを評価します。

Definitive Gate v2では、pass Case集合が8 Outcomeと14 SurfaceをすべてCoverageし、Coverage外をGapとして返すCaseと、変更・公開・Security Taskの権限境界を守るCaseを含むことを機械検証する。
