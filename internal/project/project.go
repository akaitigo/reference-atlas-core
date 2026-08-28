package project

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func Scaffold(dir, id, title, epoch string) error {
	if id == "" || title == "" {
		return fmt.Errorf("Atlas IDと日本語Titleは必須です")
	}
	if _, err := time.Parse("2006-01-02", epoch); err != nil {
		return fmt.Errorf("epochはYYYY-MM-DDで指定してください: %w", err)
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("出力先が空ではありません: %s", dir)
	}
	routerID := id + "-advisor"
	if len(routerID) > 64 {
		routerID = id[:52] + "-advisor"
	}
	routerPath := ".agents/skills/" + routerID + "/SKILL.md"
	sources := fmt.Sprintf(`schema_version: 1
atlas_id: %s
epoch: %q
generated_at: %q
sources:
  - id: reference-atlas-core-v1
    kind: specification
    title: Reference Atlas Core v1.1 Contract
    url: https://github.com/akaitigo/reference-atlas-core/releases/tag/v1.1.0
    version: v1.1.0
    retrieved_at: %q
    digest: sha256:%064d
    license: Apache-2.0
    redistribution: allowed
`, id, epoch, epoch+"T00:00:00Z", epoch, 0)
	authorityDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(sources)))
	files := map[string]string{
		"atlas.yaml": fmt.Sprintf(`schema_version: 1
id: %s
title: %s
owner: {github: akaitigo}
repository:
  url: https://github.com/akaitigo/%s
  visibility: public
  default_branch: main
archetypes: [field]
stage: 1
status: incomplete
language: {primary: ja, identifiers: en}
coverage: {epoch: %q, manifest: coverage.yaml}
authority: {lockfile: sources.lock.yaml, policy: primary-first}
mastery: {manifest: mastery.yaml, contract_version: 1.0.0}
scope:
  statement: %sについて、固定したCoverage Epochと一次資料へ追跡可能なReferenceを提供する。
  exclusions: [対象VersionとAuthority Corpusの外側にある未固定の将来仕様]
skills:
  router: {id: %s, path: %s}
  package_manifest: skill.package.yaml
  evals: evals/
completion:
  policy_version: 1.0.0
  required_profiles: [local]
  certificate: evidence/completion-certificate.json
license: {default: Apache-2.0, notice: NOTICE, third_party_manifest: third_party/manifest.yaml, sbom: sbom.spdx.json}
security: {disclosure: SECURITY.md, defensive_only: true}
`, id, title, id, epoch, title, routerID, routerPath),
		"sources.lock.yaml": sources,
		"coverage.yaml": fmt.Sprintf(`schema_version: 1
atlas_id: %s
epoch: %q
authority_lock_digest: %s
target_sets:
  - id: foundation
    title: 分野の基礎と利用
    sequence: 1
    completion_required: true
    exit_criteria: [対象範囲と原理を一次資料から説明できること, 代表Taskを再現可能な証拠で検証できること]
targets:
  - id: foundation.definitive-reference
    title: Authority追跡可能Reference
    target_set: foundation
    kind: capability
    requirement: required
    state: planned
    rationale: 分野の境界、原理、判断、実装、検証、運用を追跡可能に接続する必要がある。
    claim_ids: []
    evidence_ids: []
`, id, epoch, authorityDigest),
		"mastery.yaml": masteryTemplate(id, epoch, []string{"foundation"}),
		"skill.package.yaml": fmt.Sprintf(`schema_version: 1
atlas_id: %s
atlas_release: v0.1.0
router:
  id: %s
  path: %s
  description: %sについて、CoverageとEvidenceから適切なReferenceへ案内する。
generated_from: [coverage.yaml, mastery.yaml]
adapters: [codex, generic-agent-skills]
evals: {path: evals/, minimum_pass_rate: 1.0}
`, id, routerID, routerPath, title),
		filepath.FromSlash(routerPath): fmt.Sprintf(`---
name: %s
description: %sについて設計、実装、検証、診断、移行を一次資料とEvidenceへ接続する。
---

# %s Router

`+"`references/coverage.md`"+`を読み、Coverage外の要求はGapとして返す。
`, routerID, title, title),
		"README.md":                 "# " + title + "\n\nこのRepositoryはReference Atlas Core v1のScaffoldから生成されました。\n",
		"LICENSE":                   "Apache License 2.0: https://www.apache.org/licenses/LICENSE-2.0\n",
		"NOTICE":                    title + "\nCopyright 2026 akaitigo\n",
		"SECURITY.md":               "# Security\n\n脆弱性は公開Issueへ詳細を書かず、Repository ownerへ非公開で報告してください。\n",
		"CONTRIBUTING.md":           "# Contribution\n\nすべてのCommitへDCOのSigned-off-byを付けてください。\n",
		"third_party/manifest.yaml": "schema_version: 1\nartifacts: []\n",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	for _, path := range []string{"claims", "evidence", "evals", "migrations"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			return err
		}
	}
	return GenerateSkillReference(dir)
}

func GenerateSkillReference(dir string) error {
	coverage, err := readYAML(filepath.Join(dir, "coverage.yaml"))
	if err != nil {
		return err
	}
	atlas, err := readYAML(filepath.Join(dir, "atlas.yaml"))
	if err != nil {
		return err
	}
	skills, _ := atlas["skills"].(map[string]any)
	router, _ := skills["router"].(map[string]any)
	routerPath, _ := router["path"].(string)
	referencePath := filepath.Join(filepath.Dir(filepath.Join(dir, filepath.FromSlash(routerPath))), "references", "coverage.md")
	var output strings.Builder
	output.WriteString("# 生成Coverage Reference\n\nこのFileは `coverage.yaml` から生成される。手編集しない。\n\n")
	for _, raw := range toSlice(coverage["targets"]) {
		target, _ := raw.(map[string]any)
		fmt.Fprintf(&output, "- `%s` — %s (`%s`)\n", target["id"], target["title"], target["state"])
	}
	if err := os.MkdirAll(filepath.Dir(referencePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(referencePath, []byte(output.String()), 0o644)
}

// MigrateV1 adds the non-destructive Mastery contract and ID mapping drafts.
// Existing files are never overwritten.
func MigrateV1(dir, generatedAt string) ([]string, error) {
	atlas, err := readYAML(filepath.Join(dir, "atlas.yaml"))
	if err != nil {
		return nil, err
	}
	coverage, err := readYAML(filepath.Join(dir, "coverage.yaml"))
	if err != nil {
		return nil, err
	}
	id, _ := atlas["id"].(string)
	coverageConfig, _ := atlas["coverage"].(map[string]any)
	epoch, _ := coverageConfig["epoch"].(string)
	sets := []string{}
	for _, raw := range toSlice(coverage["target_sets"]) {
		item, _ := raw.(map[string]any)
		sets = append(sets, fmt.Sprint(item["id"]))
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("Coverage Target Setがありません")
	}
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	created := []string{}
	masteryPath := filepath.Join(dir, "mastery.yaml")
	if _, err := os.Stat(masteryPath); os.IsNotExist(err) {
		if err := os.WriteFile(masteryPath, []byte(masteryTemplate(id, epoch, sets)), 0o644); err != nil {
			return nil, err
		}
		created = append(created, masteryPath)
	}
	migrationPath := filepath.Join(dir, "migrations", "core-v1.yaml")
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		var mappings strings.Builder
		for _, raw := range toSlice(coverage["targets"]) {
			target, _ := raw.(map[string]any)
			targetID := fmt.Sprint(target["id"])
			fmt.Fprintf(&mappings, "  - legacy_id: %s\n    target_id: %s\n    disposition: preserved\n", targetID, targetID)
		}
		content := fmt.Sprintf("schema_version: 1\natlas_id: %s\nfrom: legacy-canonical\nto: core-v1\ngenerated_at: %q\nid_mappings:\n%scompatibility:\n  classification: additive\n  legacy_paths_preserved: true\n  sunset_date: null\n", id, generatedAt, mappings.String())
		if err := os.MkdirAll(filepath.Dir(migrationPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(migrationPath, []byte(content), 0o644); err != nil {
			return nil, err
		}
		created = append(created, migrationPath)
	}
	sort.Strings(created)
	return created, nil
}

// MigrateDefinitiveV2 preserves the v1 certificate as bounded history and
// creates only a migration checklist. Authority inventory is deliberately not
// generated from atlas.yaml scope because that would reproduce the v1 flaw.
func MigrateDefinitiveV2(dir, generatedAt string) (string, error) {
	path := filepath.Join(dir, "migrations", "definitive-v2.yaml")
	if _, err := os.Stat(path); err == nil {
		return "", nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	atlas, err := readYAML(filepath.Join(dir, "atlas.yaml"))
	if err != nil {
		return "", err
	}
	id, _ := atlas["id"].(string)
	completion, _ := atlas["completion"].(map[string]any)
	certificatePath, _ := completion["certificate"].(string)
	certificateData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(certificatePath)))
	if err != nil {
		return "", fmt.Errorf("v1 Certificateを履歴化できません: %w", err)
	}
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	skill, err := readYAML(filepath.Join(dir, "skill.package.yaml"))
	if err != nil {
		return "", err
	}
	release, _ := skill["atlas_release"].(string)
	historicalRelative := filepath.ToSlash(filepath.Join("evidence", "history", release, "completion-certificate.json"))
	historicalPath := filepath.Join(dir, filepath.FromSlash(historicalRelative))
	if _, err := os.Stat(historicalPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(historicalPath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(historicalPath, certificateData, 0o644); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		historicalData, err := os.ReadFile(historicalPath)
		if err != nil {
			return "", err
		}
		if sha256.Sum256(historicalData) != sha256.Sum256(certificateData) {
			return "", fmt.Errorf("既存のbounded Certificate履歴が現在のv1 Certificateと一致しません: %s", historicalRelative)
		}
	}
	content := fmt.Sprintf(`schema_version: 2
atlas_id: %s
from: bounded-complete-v1
to: subject-definitive-v2
generated_at: %q
historical_certificate:
  path: %s
  digest: sha256:%x
  classification: bounded-complete
status: inventory-required
required_actions:
  - lock-authority-artifacts
  - extract-authority-locators
  - inventory-authority-body-anchors
  - review-authority-body-anchors
  - lock-authority-body-baseline
  - review-stale-relock-explicitly
  - review-authority-text-surfaces
  - lock-non-regression-baseline
  - classify-all-surfaces
  - split-behavior-proofs
  - complete-scenario-matrix
  - complete-integrated-scenario-trace-closure
  - execute-bounded-scenario-closure-plan
  - verify-evidence-durability
  - collect-runtime-evidence
  - add-reference-system-if-applicable
  - add-comparisons-if-applicable
  - upgrade-skill-eval
  - complete-skill-router-forward-eval
  - issue-definitive-certificate
`, id, generatedAt, historicalRelative, sha256.Sum256(certificateData))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func masteryTemplate(id, epoch string, sets []string) string {
	setList := "[" + strings.Join(sets, ", ") + "]"
	outcomes := []struct{ id, title string }{{"understand", "原理と境界を説明できる"}, {"choose", "条件から方式を選べる"}, {"build", "安全に構築できる"}, {"verify", "主張を証拠で検証できる"}, {"operate", "観測して運用できる"}, {"troubleshoot", "失敗を診断し復旧できる"}, {"evolve", "互換性を保って進化できる"}, {"delegate", "Agentへ安全に委任できる"}}
	surfaces := []struct{ id, title, deliverables string }{
		{"orientation-scope", "定義・目的・境界", "[concept, decision]"}, {"foundations-mechanics", "原理・内部機構", "[concept, lab]"},
		{"architecture-design", "Architecture・設計", "[decision, reference-implementation]"}, {"implementation-construction", "実装・構築", "[reference-implementation, lab]"},
		{"testing-verification", "試験・検証", "[test, evidence]"}, {"failure-recovery", "失敗・診断・回復", "[failure-catalog, runbook]"},
		{"operations-observability", "運用・Observability", "[runbook, evidence]"}, {"security-privacy-safety", "Security・Privacy・Safety", "[security-review, test]"},
		{"performance-capacity-cost", "性能・容量・Cost", "[benchmark, evidence]"}, {"compatibility-integration", "互換性・Integration", "[test, evidence]"},
		{"migration-evolution-deprecation", "移行・進化・廃止", "[migration-guide, test]"}, {"decision-comparison", "選択・比較", "[decision, benchmark]"},
		{"provenance-rights", "出典・来歴・権利", "[provenance-record, evidence]"}, {"agent-skill", "Agent Skill・評価", "[skill-reference, skill-eval]"},
	}
	var output strings.Builder
	fmt.Fprintf(&output, "schema_version: 1\natlas_id: %s\nepoch: %q\npromise: 対象分野の境界、原理、判断、実装、検証、運用、移行を固定した一次資料と再実行可能な証拠へ接続し、既知の制約、適用条件、比較条件、未解決Gapを追跡可能なReferenceとして提供する。\naudiences: [learner, practitioner, architect, operator, maintainer, reviewer, educator, agent]\noutcomes:\n", id, epoch)
	for _, item := range outcomes {
		fmt.Fprintf(&output, "  - id: %s\n    title: %s\n    target_sets: %s\n", item.id, item.title, setList)
	}
	output.WriteString("surfaces:\n")
	for _, item := range surfaces {
		fmt.Fprintf(&output, "  - id: %s\n    title: %s\n    applicability: required\n    rationale: このSurfaceをAuthorityと再実行可能なEvidenceへ追跡可能に接続するために必要である。\n    target_sets: %s\n    required_deliverables: %s\n", item.id, item.title, setList, item.deliverables)
	}
	return output.String()
}

func readYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func toSlice(value any) []any { result, _ := value.([]any); return result }
