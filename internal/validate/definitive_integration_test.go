package validate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akaitigo/reference-atlas-core/internal/project"
	"gopkg.in/yaml.v3"
)

type definitiveCase struct {
	Mutation       string `yaml:"mutation"`
	WantError      string `yaml:"want_error"`
	RefreshBounded bool   `yaml:"refresh_bounded"`
}

func TestDefinitiveGateFixtures(t *testing.T) {
	casePaths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "definitive", "cases", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(casePaths) < 6 {
		t.Fatalf("偽陽性・偽陰性Fixtureが不足しています: %d", len(casePaths))
	}
	for _, casePath := range casePaths {
		casePath := casePath
		t.Run(strings.TrimSuffix(filepath.Base(casePath), ".yaml"), func(t *testing.T) {
			data, err := os.ReadFile(casePath)
			if err != nil {
				t.Fatal(err)
			}
			var tc definitiveCase
			if err := yaml.Unmarshal(data, &tc); err != nil {
				t.Fatal(err)
			}
			dir := createDefinitiveRepositoryFixture(t)
			applyDefinitiveMutation(t, dir, tc.Mutation)
			if tc.RefreshBounded {
				if _, err := GenerateCertificate(dir, "2026-08-28T00:00:00Z", strings.Repeat("a", 40)); err != nil {
					t.Fatal(err)
				}
				bounded, err := AuditDir(dir)
				if err != nil || bounded.CompletionClass != "bounded-complete" {
					t.Fatalf("v1の偽陽性を再現できません: result=%+v err=%v", bounded, err)
				}
			}
			result, err := AuditDefinitive(dir)
			if tc.WantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				if result.CompletionClass != "subject-definitive" || result.AuthoritySurfaces != 2 || result.RequiredMatrixRows != 6 {
					t.Fatalf("偽陰性: valid Fixtureを決定版として受理できません: %+v", result)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.WantError) {
				t.Fatalf("偽陽性を拒否できません: want=%q err=%v", tc.WantError, err)
			}
		})
	}
}

func TestNonRegressionNegativeFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "non-regression", "cases", "*.yaml"))
	if err != nil || len(paths) < 4 {
		t.Fatalf("non-regression negative fixtureが不足しています: paths=%d err=%v", len(paths), err)
	}
	for _, path := range paths {
		path := path
		t.Run(strings.TrimSuffix(filepath.Base(path), ".yaml"), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var tc definitiveCase
			if err := yaml.Unmarshal(data, &tc); err != nil {
				t.Fatal(err)
			}
			dir := createDefinitiveRepositoryFixture(t)
			applyDefinitiveMutation(t, dir, tc.Mutation)
			_, err = AuditNonRegression(dir)
			if err == nil || !strings.Contains(err.Error(), tc.WantError) {
				t.Fatalf("回避的変更を拒否できません: mutation=%s want=%q err=%v", tc.Mutation, tc.WantError, err)
			}
		})
	}
}

func TestIncompleteMigrationUsesHistoricalBoundedBaseBeforePromotionBlock(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	atlasPath := filepath.Join(dir, "atlas.yaml")
	atlas := mustReadYAML(t, atlasPath)
	atlas["status"] = "incomplete"
	mustWriteYAML(t, atlasPath, atlas)
	_, err := AuditDefinitive(dir)
	if err == nil || !strings.Contains(err.Error(), "historical bounded-complete基盤は検証済み") || !strings.Contains(err.Error(), "atlas.status=complete") {
		t.Fatalf("historical bounded基盤を認識した後にpromotionを止める必要があります: %v", err)
	}
}

func TestIncompleteMigrationRejectsTamperedHistoricalBoundedBase(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	atlasPath := filepath.Join(dir, "atlas.yaml")
	atlas := mustReadYAML(t, atlasPath)
	atlas["status"] = "incomplete"
	mustWriteYAML(t, atlasPath, atlas)
	historicalPath := filepath.Join(dir, "evidence", "history", "v0.1.0", "completion-certificate.json")
	historical := mustReadYAML(t, historicalPath)
	historical["commit"] = strings.Repeat("c", 40)
	mustWriteJSON(t, historicalPath, historical)
	_, err := AuditDefinitive(dir)
	if err == nil || !strings.Contains(err.Error(), "payload署名が一致しません") {
		t.Fatalf("改変されたhistorical bounded Certificateを基盤にできてはいけません: %v", err)
	}
}

func TestDepthParityRejectsSharedProofAcrossAuthorityDenominator(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	path := filepath.Join(dir, "depth.parity.yaml")
	doc := mustReadYAML(t, path)
	rows := doc["rows"].([]any)
	rows[1].(map[string]any)["proof_id"] = rows[0].(map[string]any)["proof_id"]
	mustWriteYAML(t, path, doc)
	ctx, err := loadDefinitiveContext(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auditSurfaceInventory(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = auditDepthParity(ctx)
	if err == nil || !strings.Contains(err.Error(), "専用の反証可能Proof") {
		t.Fatalf("1件のProofを複数Axis/Behavior/Variantへ集約できてはいけません: %v", err)
	}
}

func createDefinitiveRepositoryFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "counter-reference-atlas")
	if err := project.Scaffold(dir, "counter-reference-atlas", "Counter Protocol決定版アトラス", "2026-08-28"); err != nil {
		t.Fatal(err)
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/counter-reference-atlas\n\ngo 1.26\n")
	write("counter_test.go", "package counter\n\nimport \"testing\"\n\nfunc TestCounterRuntime(t *testing.T) {\n\tif 1+1 != 2 { t.Fatal(\"runtime assertion failed\") }\n}\n")
	write("harness.txt", "counter runtime harness v1\n")
	harnessDigest := fileDigest(t, filepath.Join(dir, "harness.txt"))
	environmentDigest := fileDigest(t, filepath.Join(dir, "go.mod"))
	sourcesData, err := os.ReadFile(filepath.Join(dir, "sources.lock.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	authorityLockDigest := shaDigest(sourcesData)

	atlas := mustReadYAML(t, filepath.Join(dir, "atlas.yaml"))
	atlas["status"] = "complete"
	completion := atlas["completion"].(map[string]any)
	completion["policy_version"] = "2.0.0"
	completion["definitive"] = map[string]any{"manifest": "definitive.yaml", "contract_version": "2.0.0"}
	mustWriteYAML(t, filepath.Join(dir, "atlas.yaml"), atlas)

	behaviors := []string{"counter.increment", "counter.reset"}
	scenarios := []string{"normal", "boundary", "rejection"}
	allEvidence := []string{}
	for _, behavior := range behaviors {
		claimID := behavior
		proofText := ""
		for _, scenario := range scenarios {
			proofText += fmt.Sprintf("  - id: %s.%s\n    statement: %sの%s条件を反証可能なRuntime観測で検証する。\n    acceptance_criteria: [期待値と実際のRuntime出力が一致すること]\n", behavior, scenario, behavior, scenario)
		}
		write("claims/"+strings.ReplaceAll(behavior, ".", "-")+".claim.yaml", fmt.Sprintf("schema_version: 1\nid: %s\natlas_id: counter-reference-atlas\ncapability_id: %s\nstatement: %sは固定Protocol v1に従い入力ごとの状態遷移を決定論的に実行する。\nstatus: accepted\nsource_ids: [reference-atlas-core-v1]\nproof_obligations:\n%s", claimID, behavior, behavior, proofText))
		for _, scenario := range scenarios {
			evidenceID := strings.TrimPrefix(behavior, "counter.") + "." + scenario
			allEvidence = append(allEvidence, evidenceID)
			reportPath := "evidence/reports/" + evidenceID + ".json"
			report := fmt.Sprintf("{\"behavior\":%q,\"scenario\":%q,\"result\":\"pass\"}\n", behavior, scenario)
			write(reportPath, report)
			write("evidence/"+evidenceID+".evidence.yaml", fmt.Sprintf("schema_version: 1\nid: %s\natlas_id: counter-reference-atlas\nclaim_ids: [%s]\nkind: test-report\nproducer: counter-runtime\ncommand: counter-test --behavior %s --scenario %s\ncreated_at: \"2026-08-28T00:00:00Z\"\nenvironment: {profile: local, manifest_digest: %s}\nsource_digest: %s\nharness_digest: %s\nharness_path: harness.txt\nexecution_mode: runtime\nruntime_identity: counter-runtime-v1\nartifact: {uri: %s, digest: %s, media_type: application/json, size_bytes: %d}\nverdict: pass\nretention: git\n", evidenceID, claimID, behavior, scenario, environmentDigest, authorityLockDigest, harnessDigest, reportPath, shaDigest([]byte(report)), len(report)))
		}
		for _, axis := range feDepthAxes {
			evidenceID := strings.TrimPrefix(behavior, "counter.") + ".depth." + axis
			allEvidence = append(allEvidence, evidenceID)
			reportPath := "evidence/reports/" + evidenceID + ".json"
			report := fmt.Sprintf("{\"behavior\":%q,\"axis\":%q,\"trace\":%q,\"result\":\"pass\"}\n", behavior, axis, evidenceID+".trace")
			write(reportPath, report)
			write("evidence/"+evidenceID+".evidence.yaml", fmt.Sprintf("schema_version: 1\nid: %s\natlas_id: counter-reference-atlas\nclaim_ids: [%s]\nkind: test-report\nproducer: counter-runtime\ncommand: counter-depth-test --behavior %s --axis %s\ncreated_at: \"2026-08-28T00:00:00Z\"\nenvironment: {profile: local, manifest_digest: %s}\nsource_digest: %s\nharness_digest: %s\nharness_path: harness.txt\nexecution_mode: runtime\nruntime_identity: counter-runtime-v1\nartifact: {uri: %s, digest: %s, media_type: application/json, size_bytes: %d}\nverdict: pass\nretention: git\n", evidenceID, claimID, behavior, axis, environmentDigest, authorityLockDigest, harnessDigest, reportPath, shaDigest([]byte(report)), len(report)))
		}
	}
	coverage := fmt.Sprintf("schema_version: 1\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\nauthority_lock_digest: %s\ntarget_sets:\n  - id: foundation\n    title: Authority由来Behavior\n    sequence: 1\n    completion_required: true\n    exit_criteria: [全BehaviorをRuntimeで検証すること, 全Scenarioの適用判断を記録すること]\ntargets:\n", authorityLockDigest)
	for _, behavior := range behaviors {
		ids := []string{}
		for _, scenario := range scenarios {
			ids = append(ids, strings.TrimPrefix(behavior, "counter.")+"."+scenario)
		}
		for _, axis := range feDepthAxes {
			ids = append(ids, strings.TrimPrefix(behavior, "counter.")+".depth."+axis)
		}
		coverage += fmt.Sprintf("  - id: %s\n    title: %s Behavior\n    target_set: foundation\n    kind: capability\n    requirement: required\n    state: covered\n    rationale: Authority Artifactから導出したBehaviorを専用Proofで検証する必要がある。\n    claim_ids: [%s]\n    evidence_ids: [%s]\n", behavior, behavior, behavior, strings.Join(ids, ", "))
	}
	coverage += fmt.Sprintf("  - id: support.execution\n    title: v1互換実行Support\n    target_set: foundation\n    kind: operation\n    requirement: recommended\n    state: covered\n    rationale: v1 bounded Gateで全Evidence実体を保持するための互換Targetである。\n    claim_ids: [%s]\n    evidence_ids: [%s]\n", strings.Join(behaviors, ", "), strings.Join(allEvidence, ", "))
	write("coverage.yaml", coverage)

	v1Cases := []string{"routing", "near-neighbor", "coverage-gap", "lifecycle", "authority", "execution", "authorization", "security"}
	evalCases := ""
	for _, category := range v1Cases {
		evalCases += fmt.Sprintf("    {\"id\":%q,\"category\":%q,\"result\":\"pass\",\"assertion\":\"%s契約を正しく評価する。\"}", "case."+category, category, category)
		if category != "security" {
			evalCases += ",\n"
		}
	}
	write("evals/counter.skill-eval.json", fmt.Sprintf("{\n  \"schema_version\":1,\"id\":\"counter.router-v1\",\"atlas_id\":\"counter-reference-atlas\",\"atlas_release\":\"v0.1.0\",\"skill_id\":\"counter-reference-atlas-advisor\",\"generated_at\":\"2026-08-28T00:00:00Z\",\"cases\":[\n%s\n  ]\n}\n", evalCases))

	sbom := "{\"spdxVersion\":\"SPDX-2.3\",\"dataLicense\":\"CC0-1.0\",\"SPDXID\":\"SPDXRef-DOCUMENT\",\"name\":\"counter-reference-atlas\",\"documentNamespace\":\"https://example.invalid/sbom/counter/v1\",\"packages\":[{\"name\":\"counter-reference-atlas\",\"SPDXID\":\"SPDXRef-Package-Counter\",\"versionInfo\":\"0.1.0\",\"licenseConcluded\":\"Apache-2.0\",\"licenseDeclared\":\"Apache-2.0\"}]}\n"
	write("sbom.spdx.json", sbom)
	provenance := "schema_version: 1\natlas_id: counter-reference-atlas\ngenerated_at: \"2026-08-28T00:00:00Z\"\nartifacts:\n"
	for _, evidenceID := range allEvidence {
		path := "evidence/reports/" + evidenceID + ".json"
		provenance += fmt.Sprintf("  - {path: %s, digest: %s, kind: test-report, license: Apache-2.0, source_ids: [reference-atlas-core-v1], generated_by: counter-runtime}\n", path, fileDigest(t, filepath.Join(dir, path)))
	}
	provenance += fmt.Sprintf("  - {path: sbom.spdx.json, digest: %s, kind: sbom, license: CC0-1.0, source_ids: [reference-atlas-core-v1], generated_by: fixture-builder}\n", shaDigest([]byte(sbom)))
	write("provenance.yaml", provenance)

	authority := "schema_version: 2\nsource_id: reference-atlas-core-v1\nsource_digest: sha256:" + strings.Repeat("0", 64) + "\nextraction: {method: machine-readable-primary, tool: fixture-extractor-v1, reviewed_by: fixture-reviewer, reviewed_at: \"2026-08-28\"}\nsurfaces:\n"
	for _, behavior := range behaviors {
		authority += fmt.Sprintf("  - {id: %s, locator: protocol/%s, kind: behavior, capability_id: %s, behavior_id: %s, variant_ids: [%s.default], title: %s Behavior, surface_ids: [orientation-scope, testing-verification]}\n", behavior, strings.TrimPrefix(behavior, "counter."), behavior, behavior, behavior, behavior)
	}
	write("authority/counter.authority-surfaces.yaml", authority)
	authorityArtifactDigest := fileDigest(t, filepath.Join(dir, "authority/counter.authority-surfaces.yaml"))
	lockedDigest := "sha256:" + strings.Repeat("0", 64)
	sourceURL := "https://github.com/akaitigo/reference-atlas-core/releases/tag/v1.0.0"
	draftCandidates := []any{}
	for _, behavior := range behaviors {
		short := strings.TrimPrefix(behavior, "counter.")
		draftCandidates = append(draftCandidates, map[string]any{
			"edge_id": "edge.counter." + short + ".reference-atlas-core-v1", "source_id": "reference-atlas-core-v1",
			"reference_url": sourceURL, "locator": "document-root", "pattern_id": "counter/" + short, "pattern_kind": "atomic",
			"candidate_behavior_id": "candidate.counter." + short, "capability_id": behavior, "target_id": behavior,
			"claim_id": behavior, "variant_ids": []any{behavior + ".default"},
			"surface_ids":                      []any{"orientation-scope", "testing-verification"},
			"classification_basis":             "domain-contract-projection-unreviewed",
			"domain_reference_metadata_digest": lockedDigest, "locator_status": "root-document",
			"context_digest": lockedDigest, "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit",
			"heading_digest": nil, "classification": "candidate-included-unreviewed",
		})
	}
	draftPath := filepath.Join(dir, "authority", "surfaces-draft", "reference-atlas-core-v1.json")
	if err := os.MkdirAll(filepath.Dir(draftPath), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, draftPath, map[string]any{
		"schema_version": 1, "source_id": "reference-atlas-core-v1", "source_url": sourceURL, "locked_source_digest": lockedDigest,
		"fetch":              map[string]any{"status": "matched", "fetched_digest": lockedDigest, "locked_digest_match": true, "http_status": 200, "final_url": sourceURL, "content_type": "text/html", "fetched_bytes": 1, "error_digest": nil},
		"extraction":         map[string]any{"method": "locked-body-locator-context-digest", "tool": "counter-authority-extractor-v1", "review_status": "automated-unreviewed", "body_storage": "digest-and-locator-context-digest-only"},
		"candidate_surfaces": draftCandidates,
	})
	draftDigest := fileDigest(t, draftPath)
	mustWriteJSON(t, filepath.Join(dir, "authority", "extraction.snapshot.json"), map[string]any{
		"schema_version": 1, "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z",
		"status": "eligible-for-core-v2", "input_digest": lockedDigest, "body_storage": "digest-and-locator-context-digest-only",
		"summary": map[string]any{
			"locked_sources": 1, "fetched_digest_matched": 1, "fetched_digest_stale": 0, "fetch_failed": 0,
			"candidate_surfaces": 2, "root_locators": 2, "fragments_found": 0, "fragments_not_found": 0,
			"locator_evaluations_deferred": 0, "reference_edges_classified": 2, "unclassified_reference_edges": 0,
			"authority_text_surfaces_exhaustive": true, "human_reviewed_surfaces": 2, "core_v2_eligible_surfaces": 2,
		},
		"sources": []any{map[string]any{
			"id": "reference-atlas-core-v1", "path": "authority/surfaces-draft/reference-atlas-core-v1.json",
			"digest": draftDigest, "locked_digest_match": true, "candidate_surfaces": 2,
			"locator_status": map[string]any{"root-document": 2},
		}},
	})
	inventory := fmt.Sprintf("schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\nauthority_lock_digest: %s\nauthority_artifacts:\n  - {id: counter-protocol, source_id: reference-atlas-core-v1, path: authority/counter.authority-surfaces.yaml, digest: %s}\nitems:\n", authorityLockDigest, authorityArtifactDigest)
	for _, behavior := range behaviors {
		inventory += fmt.Sprintf("  - {id: %s, authority_artifact_id: counter-protocol, authority_surface_id: %s, locator: protocol/%s, kind: behavior, capability_id: %s, behavior_id: %s, variant_ids: [%s.default], target_id: %s, title: %s Behavior, surface_ids: [orientation-scope, testing-verification], classification: included, rationale: Authority ArtifactのBehaviorを省略せずInventoryへ分類する。, claim_ids: [%s]}\n", behavior, behavior, strings.TrimPrefix(behavior, "counter."), behavior, behavior, behavior, behavior, behavior, behavior)
	}
	write("surface.inventory.yaml", inventory)
	matrix := "schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\nrows:\n"
	for _, behavior := range behaviors {
		for _, scenario := range definitiveScenarios {
			if scenario == "normal" || scenario == "boundary" || scenario == "rejection" {
				evidenceID := strings.TrimPrefix(behavior, "counter.") + "." + scenario
				matrix += fmt.Sprintf("  - {behavior_id: %s, scenario: %s, applicability: required, rationale: 基本BehaviorとしてRuntimeで反証可能に検証する。, proof_obligation_id: %s.%s, evidence_ids: [%s], execution_requirement: runtime, profile: local}\n", behavior, scenario, behavior, scenario, evidenceID)
			} else {
				matrix += fmt.Sprintf("  - {behavior_id: %s, scenario: %s, applicability: not-applicable, rationale: 固定Protocol v1はこのScenarioの振る舞いを定義していない。, proof_obligation_id: null, evidence_ids: [], execution_requirement: not-applicable, profile: null}\n", behavior, scenario)
			}
		}
	}
	write("verification.matrix.yaml", matrix)
	depth := "schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\ncompletion_status: parity\nreference: {id: fe-depth-reference-v1, path: authority/FE_DEPTH_REFERENCE.json, digest: " + feDepthReferenceDigest + ", repository: frontend-behavior-atlas, commit: " + feDepthReferenceCommit + ", status_at_commit: incomplete}\ndenominator_policy: {source: authority-derived-subject-surface-inventory, transplant_absolute_counts: false}\nrows:\n"
	for _, behavior := range behaviors {
		for _, axis := range feDepthAxes {
			evidenceID := strings.TrimPrefix(behavior, "counter.") + ".depth." + axis
			depth += fmt.Sprintf("  - {behavior_id: %s, variant_id: %s.default, axis: %s, status: satisfied, gap_count: 0, proof_id: %s.proof, oracle: Subject固有のAuthority由来母集団について期待結果と実行結果が一致すること。, evidence_ids: [%s], artifact_uri: evidence/reports/%s.json, trace_id: %s.trace, rationale: FEの絶対件数を転用せずportable criterionを専用ArtifactとTraceで検証する。}\n", behavior, behavior, axis, evidenceID, evidenceID, evidenceID, evidenceID)
		}
	}
	write("depth.parity.yaml", depth)
	feReference, err := os.ReadFile(filepath.Join("..", "..", "profiles", "FE_DEPTH_REFERENCE.json"))
	if err != nil {
		t.Fatal(err)
	}
	write("authority/FE_DEPTH_REFERENCE.json", string(feReference))
	write("reference/counter-system.txt", "increment and reset integrated runtime reference system\n")
	referenceDigest := fileDigest(t, filepath.Join(dir, "reference", "counter-system.txt"))
	allOutcomes := "understand, choose, build, verify, operate, troubleshoot, evolve, delegate"
	allSurfaces := "orientation-scope, foundations-mechanics, architecture-design, implementation-construction, testing-verification, failure-recovery, operations-observability, security-privacy-safety, performance-capacity-cost, compatibility-integration, migration-evolution-deprecation, decision-comparison, provenance-rights, agent-skill"
	write("evals/counter.definitive-skill-eval.json", fmt.Sprintf("{\"schema_version\":2,\"id\":\"counter.definitive-v2\",\"atlas_id\":\"counter-reference-atlas\",\"atlas_release\":\"v0.1.0\",\"skill_id\":\"counter-reference-atlas-advisor\",\"generated_at\":\"2026-08-28T00:00:00Z\",\"cases\":[{\"id\":\"all.contracts\",\"result\":\"pass\",\"outcome_ids\":[%s],\"surface_ids\":[%s],\"gap_behavior\":true,\"authorization_boundary\":true,\"assertion\":\"8 Outcomeと14 SurfaceとGapと権限境界を評価する。\"}]}\n", quoteList(allOutcomes), quoteList(allSurfaces)))
	write("definitive.yaml", fmt.Sprintf("schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\ncompletion_class: subject-definitive\nauthority_extraction: authority/extraction.snapshot.json\nsurface_inventory: surface.inventory.yaml\nverification_matrix: verification.matrix.yaml\ndepth_parity: depth.parity.yaml\nskill_eval: evals/counter.definitive-skill-eval.json\nnon_regression: non-regression.yaml\ncertificate: evidence/definitive-certificate.json\nhistorical_certificates:\n  - {path: evidence/history/v0.1.0/completion-certificate.json, classification: bounded-complete}\nreference_systems:\n  - {id: counter-system, path: reference/counter-system.txt, digest: %s, behavior_ids: [counter.increment, counter.reset]}\ncomparisons: []\n", referenceDigest))
	if _, err := GenerateCertificate(dir, "2026-08-28T00:00:00Z", strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	v1Certificate, err := os.ReadFile(filepath.Join(dir, "evidence", "completion-certificate.json"))
	if err != nil {
		t.Fatal(err)
	}
	write("evidence/history/v0.1.0/completion-certificate.json", string(v1Certificate))
	baselineRelative := "baselines/v0.1.0.non-regression-baseline.json"
	if _, err := GenerateNonRegressionBaseline(dir, filepath.Join(dir, baselineRelative), strings.Repeat("a", 40), "2026-08-28T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	write("non-regression.yaml", fmt.Sprintf("schema_version: 2\natlas_id: counter-reference-atlas\nbaseline: {path: %s, digest: %s}\nreplacements: []\n", baselineRelative, fileDigest(t, filepath.Join(dir, baselineRelative))))
	if _, err := GenerateDefinitiveCertificate(dir, "2026-08-28T00:00:00Z", strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	return dir
}

func applyDefinitiveMutation(t *testing.T, dir, mutation string) {
	t.Helper()
	switch mutation {
	case "none":
		return
	case "required-infeasible":
		doc := mustReadYAML(t, filepath.Join(dir, "coverage.yaml"))
		targets := doc["targets"].([]any)
		targets[0].(map[string]any)["state"] = "infeasible"
		targets[0].(map[string]any)["exclusion"] = map[string]any{"reason": "Runtime環境が不足しているため現時点では実行できない。", "reviewed_at": "2026-08-28"}
		mustWriteYAML(t, filepath.Join(dir, "coverage.yaml"), doc)
	case "inventory-omission":
		doc := mustReadYAML(t, filepath.Join(dir, "surface.inventory.yaml"))
		doc["items"] = doc["items"].([]any)[:1]
		mustWriteYAML(t, filepath.Join(dir, "surface.inventory.yaml"), doc)
	case "aggregate-evidence":
		doc := mustReadYAML(t, filepath.Join(dir, "verification.matrix.yaml"))
		rows := doc["rows"].([]any)
		rows[1].(map[string]any)["evidence_ids"] = rows[0].(map[string]any)["evidence_ids"]
		mustWriteYAML(t, filepath.Join(dir, "verification.matrix.yaml"), doc)
	case "aggregate-target":
		doc := mustReadYAML(t, filepath.Join(dir, "surface.inventory.yaml"))
		items := doc["items"].([]any)
		items[1].(map[string]any)["target_id"] = items[0].(map[string]any)["target_id"]
		mustWriteYAML(t, filepath.Join(dir, "surface.inventory.yaml"), doc)
	case "matrix-gap":
		doc := mustReadYAML(t, filepath.Join(dir, "verification.matrix.yaml"))
		rows := doc["rows"].([]any)
		doc["rows"] = append(rows[:1], rows[2:]...)
		mustWriteYAML(t, filepath.Join(dir, "verification.matrix.yaml"), doc)
	case "static-runtime":
		path := filepath.Join(dir, "evidence", "increment.normal.evidence.yaml")
		doc := mustReadYAML(t, path)
		doc["execution_mode"] = "fixture"
		delete(doc, "runtime_identity")
		mustWriteYAML(t, path, doc)
	case "skill-outcome-gap":
		path := filepath.Join(dir, "evals", "counter.definitive-skill-eval.json")
		doc := mustReadYAML(t, path)
		cases := doc["cases"].([]any)
		item := cases[0].(map[string]any)
		item["outcome_ids"] = item["outcome_ids"].([]any)[:7]
		mustWriteJSON(t, path, doc)
	case "baseline-target-deletion":
		doc := mustReadYAML(t, filepath.Join(dir, "coverage.yaml"))
		targets := doc["targets"].([]any)
		doc["targets"] = targets[1:]
		mustWriteYAML(t, filepath.Join(dir, "coverage.yaml"), doc)
	case "baseline-scope-exclusion":
		doc := mustReadYAML(t, filepath.Join(dir, "atlas.yaml"))
		scope := doc["scope"].(map[string]any)
		scope["exclusions"] = append(scope["exclusions"].([]any), "Runtime検証を新たにScope外へ移動する。")
		mustWriteYAML(t, filepath.Join(dir, "atlas.yaml"), doc)
	case "baseline-scope-statement":
		doc := mustReadYAML(t, filepath.Join(dir, "atlas.yaml"))
		scope := doc["scope"].(map[string]any)
		scope["statement"] = "Counterのincrementだけを対象とし、既存のreset、境界、拒否、障害、回復、移行、運用、Security、性能、互換性の各Behaviorを新たに対象外へ移動する縮小Scopeである。"
		mustWriteYAML(t, filepath.Join(dir, "atlas.yaml"), doc)
	case "baseline-skip":
		path := filepath.Join(dir, "counter_test.go")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(data), "if 1+1", "t.Skip(\"disabled\")\n\tif 1+1", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	case "baseline-threshold-relaxation":
		doc := mustReadYAML(t, filepath.Join(dir, "skill.package.yaml"))
		evals := doc["evals"].(map[string]any)
		evals["minimum_pass_rate"] = 0.5
		mustWriteYAML(t, filepath.Join(dir, "skill.package.yaml"), doc)
	case "depth-gap":
		doc := mustReadYAML(t, filepath.Join(dir, "depth.parity.yaml"))
		doc["completion_status"] = "incomplete"
		mustWriteYAML(t, filepath.Join(dir, "depth.parity.yaml"), doc)
	case "variant-omission":
		doc := mustReadYAML(t, filepath.Join(dir, "surface.inventory.yaml"))
		items := doc["items"].([]any)
		delete(items[0].(map[string]any), "variant_ids")
		mustWriteYAML(t, filepath.Join(dir, "surface.inventory.yaml"), doc)
	case "authority-candidates-not-exhaustive":
		path := filepath.Join(dir, "authority", "extraction.snapshot.json")
		doc := mustReadYAML(t, path)
		summary := doc["summary"].(map[string]any)
		summary["authority_text_surfaces_exhaustive"] = false
		doc["status"] = "incomplete-human-review-required"
		mustWriteJSON(t, path, doc)
	default:
		t.Fatalf("未知のFixture mutation: %s", mutation)
	}
}

func mustReadYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func mustWriteYAML(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	data, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteJSON(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return shaDigest(data)
}

func shaDigest(data []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(data)) }

func quoteList(csv string) string {
	parts := strings.Split(csv, ", ")
	for i, part := range parts {
		parts[i] = fmt.Sprintf("%q", part)
	}
	return strings.Join(parts, ",")
}
