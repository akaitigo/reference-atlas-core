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
	scenarios := []string{"normal", "boundary", "refusal"}
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
	sourceURL := "https://github.com/akaitigo/reference-atlas-core/releases/tag/v1.1.0"
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
	bodyToolDigest := "sha256:" + strings.Repeat("4", 64)
	bodyAnchorIDs := []string{"anchor-root-counter", "anchor-counter-increment"}
	bodyDocumentPath := filepath.Join(dir, "authority", "body-inventory-draft", "document-reference-atlas-core-counter.json")
	if err := os.MkdirAll(filepath.Dir(bodyDocumentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, bodyDocumentPath, map[string]any{
		"schema_version": 1, "document_id": "document-reference-atlas-core-counter", "fetch_url": sourceURL,
		"source_ids": []any{"reference-atlas-core-v1"}, "locked_body_digest": lockedDigest,
		"fetch":      map[string]any{"status": "matched", "fetched_digest": lockedDigest, "locked_digest_match": true, "http_status": 200, "final_url": sourceURL, "content_type": "text/html", "fetched_bytes": 1, "error_digest": nil},
		"extraction": map[string]any{"method": "html-semantic-anchor-selector-v1", "tool": "counter-body-inventory-v1", "tool_digest": bodyToolDigest, "selector_contract": []any{"document-root", "h1"}, "selector_exhaustive_for_locked_body": true, "authority_semantics_exhaustive": false, "review_status": "automated-unreviewed", "body_storage": "digest-locator-and-offset-only"},
		"anchors": []any{
			map[string]any{"id": bodyAnchorIDs[0], "locator": "document-root", "locator_kind": "document-root", "semantic_kind": "document-root", "tag": "document", "heading_level": nil, "parent_anchor_id": nil, "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "context_digest": lockedDigest, "label_digest": nil, "classification_status": "pending-human", "surface_ids": []any{}},
			map[string]any{"id": bodyAnchorIDs[1], "locator": "#counter", "locator_kind": "fragment", "semantic_kind": "heading", "tag": "h1", "heading_level": 1, "parent_anchor_id": bodyAnchorIDs[0], "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "context_digest": "sha256:" + strings.Repeat("5", 64), "label_digest": "sha256:" + strings.Repeat("6", 64), "classification_status": "pending-human", "surface_ids": []any{}},
		},
	})
	bodyIndexPath := filepath.Join(dir, "authority", "body-inventory.snapshot.json")
	mustWriteJSON(t, bodyIndexPath, map[string]any{
		"schema_version": 1, "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z", "status": "incomplete-human-review-required",
		"input_digest": lockedDigest, "tool_digest": bodyToolDigest, "body_storage": "digest-locator-and-offset-only", "selector_contract": []any{"document-root", "h1"},
		"summary":   map[string]any{"source_entries": 1, "unique_documents": 1, "matched_documents": 1, "stale_documents": 0, "failed_documents": 0, "selector_exhaustive_documents": 1, "anchors": 2, "anchors_by_kind": map[string]any{"document-root": 1, "heading": 1}, "classified_anchors": 0, "unclassified_anchors": 2, "human_reviewed_anchors": 0, "core_v2_eligible_artifacts": 0, "authority_semantics_exhaustive": false},
		"documents": []any{map[string]any{"id": "document-reference-atlas-core-counter", "path": "authority/body-inventory-draft/document-reference-atlas-core-counter.json", "digest": fileDigest(t, bodyDocumentPath), "fetch_status": "matched", "source_entries": 1, "anchors": 2, "anchors_by_kind": map[string]any{"document-root": 1, "heading": 1}}},
	})
	reviewToolDigest := "sha256:" + strings.Repeat("7", 64)
	reviewBatchPath := filepath.Join(dir, "authority", "review-queue-draft", "review-p0-heading-00.json")
	if err := os.MkdirAll(filepath.Dir(reviewBatchPath), 0o755); err != nil {
		t.Fatal(err)
	}
	reviewItems := []any{}
	for index, anchorID := range bodyAnchorIDs {
		locator, kind, tag, headingLevel, labelDigest := "document-root", "document-root", "document", any(nil), any(nil)
		contextDigest := lockedDigest
		if index == 1 {
			locator, kind, tag, headingLevel, labelDigest = "#counter", "heading", "h1", 1, "sha256:"+strings.Repeat("6", 64)
			contextDigest = "sha256:" + strings.Repeat("5", 64)
		}
		reviewItems = append(reviewItems, map[string]any{"anchor_id": anchorID, "document_id": "document-reference-atlas-core-counter", "document_url": sourceURL, "source_ids": []any{"reference-atlas-core-v1"}, "locked_source_digest": lockedDigest, "inventory_tool_digest": bodyToolDigest, "review_queue_tool_digest": reviewToolDigest, "locator": locator, "locator_kind": map[bool]string{true: "fragment", false: "document-root"}[index == 1], "semantic_kind": kind, "tag": tag, "heading_level": headingLevel, "parent_anchor_id": map[bool]any{true: bodyAnchorIDs[0], false: nil}[index == 1], "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "context_digest": contextDigest, "label_digest": labelDigest, "existing_reference_edge_ids": []any{}, "priority": 0, "priority_reasons": []any{"fixture-primary-source-review"}, "candidate_cluster_id": nil, "batch_id": "review-p0-heading-00", "state": "pending-human"})
	}
	mustWriteJSON(t, reviewBatchPath, map[string]any{"schema_version": 1, "queue_id": "authority-review-counter", "batch_id": "review-p0-heading-00", "status": "pending-human", "machine_assistance": "ordering-only", "semantic_decisions": "none", "items": reviewItems})
	decisionBindings := func(anchorID, locator, contextDigest string) []any {
		return []any{map[string]any{"anchor_id": anchorID, "document_id": "document-reference-atlas-core-counter", "document_url": sourceURL, "locked_source_digest": lockedDigest, "inventory_tool_digest": bodyToolDigest, "review_queue_tool_digest": reviewToolDigest, "locator": locator, "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "context_digest": contextDigest}}
	}
	if err := os.MkdirAll(filepath.Join(dir, "authority", "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(dir, "authority", "reviews", "decisions.json"), map[string]any{"schema_version": 1, "atlas_id": "counter-reference-atlas", "queue_id": "authority-review-counter", "status": "closed", "decisions": []any{
		map[string]any{"decision_id": "decision.counter.increment", "action": "include", "anchor_ids": []any{bodyAnchorIDs[0]}, "source_bindings": decisionBindings(bodyAnchorIDs[0], "document-root", lockedDigest), "rationale": "固定した一次資料の該当locatorを人が確認し、increment Surfaceへ昇格できる独立した意味境界であると判断した。", "reviewer": "fixture-reviewer", "reviewed_at": "2026-08-28T00:00:00Z", "review_method": "manual-primary-source", "mapping": []any{map[string]any{"old_anchor_id": bodyAnchorIDs[0], "new_item_ids": []any{"counter-protocol.counter.increment"}}}, "result_items": []any{map[string]any{"id": "counter-protocol.counter.increment", "item_type": "surface"}}},
		map[string]any{"decision_id": "decision.counter.reset", "action": "include", "anchor_ids": []any{bodyAnchorIDs[1]}, "source_bindings": decisionBindings(bodyAnchorIDs[1], "#counter", "sha256:"+strings.Repeat("5", 64)), "rationale": "固定した一次資料の該当locatorを人が確認し、reset Surfaceへ昇格できる独立した意味境界であると判断した。", "reviewer": "fixture-reviewer", "reviewed_at": "2026-08-28T00:00:00Z", "review_method": "manual-primary-source", "mapping": []any{map[string]any{"old_anchor_id": bodyAnchorIDs[1], "new_item_ids": []any{"counter-protocol.counter.reset"}}}, "result_items": []any{map[string]any{"id": "counter-protocol.counter.reset", "item_type": "surface"}}},
	}})
	mustWriteJSON(t, filepath.Join(dir, "authority", "review-queue.snapshot.json"), map[string]any{
		"schema_version": 1, "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z", "status": "closed", "queue_id": "authority-review-counter", "input_digest": fileDigest(t, bodyIndexPath), "tool_digest": reviewToolDigest, "decision_ledger": "authority/reviews/decisions.json", "body_storage": "digest-locator-and-offset-only", "machine_assistance": "ordering-only", "semantic_decisions": "human-only",
		"summary": map[string]any{"eligible_documents": 1, "queued_anchors": 2, "pending_human": 0, "human_reviewed": 2, "priority_counts": map[string]any{"0": 2}, "candidate_clusters": 0, "clustered_anchors": 0, "batches": 1, "stale_document_holds": 0, "unavailable_document_holds": 0, "decisions": 2, "included": 2, "excluded": 0, "merged": 0, "split": 0, "deferred": 0, "authority_semantics_exhaustive": true, "queue_counts_as_depth_achievement": false},
		"batches": []any{map[string]any{"id": "review-p0-heading-00", "path": "authority/review-queue-draft/review-p0-heading-00.json", "digest": fileDigest(t, reviewBatchPath), "priority": 0, "semantic_kind": "heading", "bucket": "00", "items": 2}}, "stale_holds": []any{}, "unavailable_holds": []any{},
	})
	for _, relative := range []string{"baselines", "migrations"} {
		if err := os.MkdirAll(filepath.Join(dir, relative), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWriteJSON(t, filepath.Join(dir, "baselines", "authority-body-inventory-v1.json"), map[string]any{
		"schema_version": 1, "id": "authority-body-inventory-v1-fixture", "captured_at": "2026-08-28T00:00:00Z", "source_entries": 1, "unique_documents": 1, "selector_contract": []any{"document-root", "h1"},
		"documents": []any{map[string]any{"id": "document-reference-atlas-core-counter", "path": "authority/body-inventory-draft/document-reference-atlas-core-counter.json", "locked_body_digest": lockedDigest, "source_ids": []any{"reference-atlas-core-v1"}, "anchor_ids": []any{bodyAnchorIDs[0], bodyAnchorIDs[1]}}},
	})
	mustWriteJSON(t, filepath.Join(dir, "migrations", "authority-body-inventory-v1.json"), map[string]any{"schema_version": 1, "baseline_id": "authority-body-inventory-v1-fixture", "replacements": []any{}})
	inventory := fmt.Sprintf("schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\nauthority_lock_digest: %s\nauthority_artifacts:\n  - {id: counter-protocol, source_id: reference-atlas-core-v1, path: authority/counter.authority-surfaces.yaml, digest: %s}\nitems:\n", authorityLockDigest, authorityArtifactDigest)
	for _, behavior := range behaviors {
		inventory += fmt.Sprintf("  - {id: %s, authority_artifact_id: counter-protocol, authority_surface_id: %s, locator: protocol/%s, kind: behavior, capability_id: %s, behavior_id: %s, variant_ids: [%s.default], target_id: %s, title: %s Behavior, surface_ids: [orientation-scope, testing-verification], classification: included, rationale: Authority ArtifactのBehaviorを省略せずInventoryへ分類する。, claim_ids: [%s]}\n", behavior, behavior, strings.TrimPrefix(behavior, "counter."), behavior, behavior, behavior, behavior, behavior, behavior)
	}
	write("surface.inventory.yaml", inventory)
	matrix := "schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\nrows:\n"
	for _, behavior := range behaviors {
		for _, scenario := range definitiveScenarios {
			if scenario == "normal" || scenario == "boundary" || scenario == "refusal" {
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
	forwardEvalPath := filepath.Join(dir, "evidence", "reports", "skill-router-forward-eval.json")
	write("evidence/reports/skill-router-forward-eval.json", "{\"cases\":2,\"passed\":2,\"failed\":0}\n")
	fileBinding := func(id, relative string) map[string]any {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"id": id, "path": relative, "digest": shaDigest(data), "bytes": len(data)}
	}
	routerCells := []any{}
	for outcomeIndex, outcome := range strings.Split(allOutcomes, ", ") {
		for surfaceIndex, surface := range strings.Split(allSurfaces, ", ") {
			behavior := behaviors[(outcomeIndex+surfaceIndex)%len(behaviors)]
			evidenceID := strings.TrimPrefix(behavior, "counter.") + ".normal"
			evidenceRelative := "evidence/" + evidenceID + ".evidence.yaml"
			routerCells = append(routerCells, map[string]any{
				"id": "skill." + outcome + "." + surface, "status": "routed", "outcome": outcome, "surface": surface, "mode": "select", "query": outcome + " " + surface,
				"pattern_id": behavior, "target_id": behavior, "target_set": "foundation", "target_set_allowed": true, "coverage_state": "covered", "coverage_disposition": "covered",
				"required_deliverables": []any{"concept", "evidence"}, "required_output_fields": []any{"result", "evidence"}, "mutation_policy": "authorized", "mutation_status": "completed", "blocked_reasons": []any{},
				"stop_conditions": []any{"coverage-gap", "unverified-evidence", "unauthorized-mutation", "external-human-decision-required", "stale-source-relock-explicit-procedure-required"}, "acceptance_criteria": []any{"固定Contractに対する実行結果とEvidenceが一致する。"},
				"implementation_bindings": []any{fileBinding(strings.TrimPrefix(behavior, "counter.")+"-default", "reference/counter-system.txt")},
				"source_bindings":         []any{map[string]any{"source_id": "reference-atlas-core-v1", "url": sourceURL, "digest": lockedDigest}},
				"evidence_bindings":       []any{fileBinding(evidenceID+"-artifact", "evidence/reports/"+evidenceID+".json")}, "expected_pattern_id": behavior, "result": "pass", "support_status": "routed", "assertions": map[string]any{"permission_boundary": true, "authority_binding": true, "evidence_binding": true},
				"variant_ids": []any{behavior + ".default"}, "authority_item_ids": []any{"counter-protocol." + behavior}, "runtime_evidence_bindings": []any{map[string]any{"evidence_id": evidenceID, "path": evidenceRelative, "digest": fileDigest(t, filepath.Join(dir, evidenceRelative))}},
			})
		}
	}
	boundary := func(id, status, mutationPolicy, mutationStatus string, blocked []any) map[string]any {
		return map[string]any{"id": id, "status": status, "outcome": "understand", "surface": "orientation-scope", "mode": "review", "query": id, "coverage_state": "covered", "coverage_disposition": status, "required_deliverables": []any{"decision"}, "required_output_fields": []any{"status"}, "mutation_policy": mutationPolicy, "mutation_status": mutationStatus, "blocked_reasons": blocked, "stop_conditions": []any{"unauthorized-mutation", "external-human-decision-required", "stale-source-relock-explicit-procedure-required"}, "expected": map[string]any{"status": status}, "result": "pass"}
	}
	routerBoundaries := []any{
		boundary("boundary.ambiguous", "coverage-gap", "read-only", "read-only", []any{}), boundary("boundary.unknown", "coverage-gap", "read-only", "read-only", []any{}),
		boundary("boundary.unauthorized-build", "blocked", "explicit-authorization-required", "blocked", []any{"unauthorized-mutation"}),
		boundary("boundary.human-authority-decision", "blocked", "explicit-authorization-required", "blocked", []any{"external-human-decision-required"}),
		boundary("boundary.stale-relock", "blocked", "explicit-authorization-required", "blocked", []any{"stale-source-relock-explicit-procedure-required"}),
	}
	mustWriteJSON(t, filepath.Join(dir, "evals", "definitive-skill-router.json"), map[string]any{
		"schema_version": 1, "id": "counter.definitive-router-v1", "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z", "status": "subject-skill-ready", "semantic_scope": "router-contract-and-independent-forward-eval",
		"source_bindings": map[string]any{"router": fileBinding("router", "reference/counter-system.txt"), "skill": fileBinding("skill", "go.mod"), "evaluator": fileBinding("evaluator", "harness.txt"), "mastery_contract": fileBinding("mastery-contract", "coverage.yaml")},
		"summary":         map[string]any{"outcomes": 8, "surfaces": 14, "matrix_cells": 112, "passed": 112, "failed": 0, "routed": 112, "mastery_routing_gaps": 0, "partial_coverage_cells": 0, "boundary_cases": 5, "boundary_passed": 5, "boundary_failed": 0},
		"matrix":          routerCells, "boundary_cases": routerBoundaries, "completion_limits": []any{}, "forward_eval": map[string]any{"status": "completed", "cases": 2, "passed": 2, "failed": 0, "artifact_path": "evidence/reports/skill-router-forward-eval.json", "artifact_digest": fileDigest(t, forwardEvalPath)},
	})
	writeCompletionEligibleScenarioFixture(t, dir, behaviors, authorityArtifactDigest, authorityLockDigest, environmentDigest, harnessDigest)
	write("definitive.yaml", fmt.Sprintf("schema_version: 2\natlas_id: counter-reference-atlas\nepoch: \"2026-08-28\"\ncompletion_class: subject-definitive\nauthority_extraction: authority/extraction.snapshot.json\nauthority_body_inventory: authority/body-inventory.snapshot.json\nauthority_body_review: authority/review-queue.snapshot.json\nsurface_inventory: surface.inventory.yaml\nverification_matrix: verification.matrix.yaml\nscenario_proofs: evidence/scenarios/index.json\nscenario_closure_plan: evidence/scenarios/closure-plan.json\nevidence_durability: artifacts/pattern-scenarios/results.json\ndepth_parity: depth.parity.yaml\nskill_eval: evals/counter.definitive-skill-eval.json\nskill_router: evals/definitive-skill-router.json\nnon_regression: non-regression.yaml\ncertificate: evidence/definitive-certificate.json\nhistorical_certificates:\n  - {path: evidence/history/v0.1.0/completion-certificate.json, classification: bounded-complete}\nreference_systems:\n  - {id: counter-system, path: reference/counter-system.txt, digest: %s, behavior_ids: [counter.increment, counter.reset]}\ncomparisons: []\n", referenceDigest))
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

func writeCompletionEligibleScenarioFixture(t *testing.T, dir string, behaviors []string, authorityArtifactDigest, authorityLockDigest, environmentManifestDigest, harnessDigest string) {
	t.Helper()
	writeFile := func(relative string, data []byte) {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestScenarios, resultTests, scenarioRuntimeTests, files := []any{}, []any{}, []any{}, []any{}
	byScenario := map[string]any{}
	referenceData, err := os.ReadFile(filepath.Join(dir, "reference", "counter-system.txt"))
	if err != nil {
		t.Fatal(err)
	}
	referenceDigest := shaDigest(referenceData)
	environment := map[string]any{"profile": "local", "manifest_digest": environmentManifestDigest, "runtime": "counter-v1"}
	environmentIdentity, _ := digestCanonical(environment)
	scenarioRuntimeEnvironment := map[string]any{"runtime": "counter-v1", "platform": "test", "architecture": "test", "version": "1", "worker": "single", "workers": 1, "retries": 0, "viewport": "none", "trace_mode": "on"}
	coverage := mustReadYAML(t, filepath.Join(dir, "coverage.yaml"))
	targets := coverage["targets"].([]any)
	provenance := mustReadYAML(t, filepath.Join(dir, "provenance.yaml"))
	provenanceArtifacts := provenance["artifacts"].([]any)
	for _, scenario := range scenarioTraceScenarios {
		tracePath := "artifacts/reference-system/traces/" + scenario + ".trace.zip"
		screenshotPath := "artifacts/reference-system/screenshots/" + scenario + ".png"
		traceData := []byte("counter integrated trace " + scenario + "\n")
		screenshotData := []byte("counter integrated screenshot " + scenario + "\n")
		writeFile(tracePath, traceData)
		writeFile(screenshotPath, screenshotData)
		trace := map[string]any{"path": tracePath, "digest": shaDigest(traceData), "bytes": len(traceData), "action_stream": true, "network_stream": true, "resource_stream": true}
		screenshot := map[string]any{"path": screenshotPath, "digest": shaDigest(screenshotData), "bytes": len(screenshotData)}
		manifestScenarios = append(manifestScenarios, map[string]any{"id": scenario, "patterns": []any{behaviors[0], behaviors[1]}, "runtime_boundaries": []any{"counter-runtime"}, "assertions": []any{"expected-state", "dedicated-proof"}})
		resultTests = append(resultTests, map[string]any{"id": "reference." + scenario, "scenario": scenario, "title": "counter integrated " + scenario, "file": "counter_test.go", "line": 1, "outcome": "expected", "attempts": 1, "duration_ms": 1, "final_status": "passed", "error": nil, "trace": trace, "screenshot": screenshot})
		byScenario[scenario] = map[string]any{"rows": len(behaviors), "pattern_specific": len(behaviors), "runtime_identity": len(behaviors), "integrated_pattern_mapped": len(behaviors), "gaps": 0}
		for _, behavior := range behaviors {
			evidenceID := strings.TrimPrefix(behavior, "counter.") + ".scenario." + scenario
			artifactPath := "evidence/reports/" + evidenceID + ".json"
			artifactData := []byte(fmt.Sprintf("{\"behavior\":%q,\"scenario\":%q,\"runtime_identity\":\"counter-v1\",\"result\":\"pass\"}\n", behavior, scenario))
			writeFile(artifactPath, artifactData)
			dedicatedTracePath := "artifacts/pattern-scenarios/traces/" + strings.ReplaceAll(behavior, ".", "-") + "__" + scenario + ".trace.zip"
			dedicatedScreenshotPath := "artifacts/pattern-scenarios/screenshots/" + strings.ReplaceAll(behavior, ".", "-") + "__" + scenario + ".png"
			dedicatedTraceData := []byte("counter dedicated scenario trace " + behavior + " " + scenario + "\n")
			dedicatedScreenshotData := []byte("counter dedicated scenario screenshot " + behavior + " " + scenario + "\n")
			writeFile(dedicatedTracePath, dedicatedTraceData)
			writeFile(dedicatedScreenshotPath, dedicatedScreenshotData)
			sourceBindings := []any{map[string]any{"variant_id": behavior + ".default", "path": "reference/counter-system.txt", "digest": referenceDigest}}
			sourceIdentity, _ := digestCanonical(sourceBindings)
			evidenceRelative := "evidence/" + evidenceID + ".evidence.yaml"
			evidenceText := fmt.Sprintf("schema_version: 1\nid: %s\natlas_id: counter-reference-atlas\nclaim_ids: [%s]\nkind: test-report\nproducer: counter-runtime\ncommand: counter-scenario-test --behavior %s --scenario %s\ncreated_at: \"2026-08-28T00:00:00Z\"\nenvironment: {profile: local, manifest_digest: %s, runtime: counter-v1}\nsource_digest: %s\nharness_digest: %s\nharness_path: harness.txt\nexecution_mode: runtime\nruntime_identity: counter-runtime-v1\nartifact: {uri: %s, digest: %s, media_type: application/json, size_bytes: %d}\nverdict: pass\nretention: git\n", evidenceID, behavior, behavior, scenario, environmentManifestDigest, authorityLockDigest, harnessDigest, artifactPath, shaDigest(artifactData), len(artifactData))
			writeFile(evidenceRelative, []byte(evidenceText))
			for _, rawTarget := range targets {
				target := rawTarget.(map[string]any)
				if target["id"] == behavior || target["id"] == "support.execution" {
					target["evidence_ids"] = append(target["evidence_ids"].([]any), evidenceID)
				}
			}
			provenanceArtifacts = append(provenanceArtifacts, map[string]any{"path": artifactPath, "digest": shaDigest(artifactData), "kind": "test-report", "license": "Apache-2.0", "source_ids": []any{"reference-atlas-core-v1"}, "generated_by": "counter-scenario-runtime"})
			scenarioRecord := map[string]any{"id": "runtime." + strings.ReplaceAll(behavior, ".", "-") + "." + scenario, "pattern_id": behavior, "variant_id": behavior + ".default", "scenario": scenario, "title": behavior + " " + scenario + " dedicated runtime", "file": "counter_scenario_test.go", "line": 1, "source_digest": referenceDigest, "outcome": "expected", "attempts": 1, "final_status": "passed", "error": nil, "oracle": map[string]any{"scenario": scenario, "assertion": "counter state matches"}, "trace": map[string]any{"path": dedicatedTracePath, "digest": shaDigest(dedicatedTraceData), "bytes": len(dedicatedTraceData), "action_stream": true, "network_stream": true, "resource_stream": true}, "screenshot": map[string]any{"path": dedicatedScreenshotPath, "digest": shaDigest(dedicatedScreenshotData), "bytes": len(dedicatedScreenshotData)}}
			scenarioRuntimeTests = append(scenarioRuntimeTests, scenarioRecord)
			row := map[string]any{
				"schema_version": 1, "id": "proof." + strings.ReplaceAll(behavior, ".", "-") + "." + scenario, "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z",
				"behavior_scope": "authority-derived-atomic-behavior", "pattern_id": behavior, "behavior_id": behavior, "target_id": behavior, "target_set": "foundation", "scenario": scenario, "applicability": "required", "status": "completion-eligible-runtime-proof",
				"classification": map[string]any{"method": "authority-atomic-runtime-identity", "matcher_digest": referenceDigest, "state_ids": []any{scenario}, "semantic_scope_match": true}, "source_bindings": sourceBindings,
				"pattern_evidence":     map[string]any{"capture_environment_identity": environment, "capture_harness_digest": harnessDigest, "capture_records": []any{}, "benchmark_environment": nil, "benchmark_records": []any{}, "compatibility_environment": nil, "compatibility_records": []any{}, "scenario_runtime_report": "artifacts/pattern-scenarios/results.json", "scenario_runtime_environment": scenarioRuntimeEnvironment, "scenario_runtime_records": []any{scenarioRecord}},
				"integrated_reference": map[string]any{"manifest": "integrations/reference-system/manifest.json", "result": "artifacts/reference-system/results.json", "pattern_mapped": true, "runtime_boundaries": []any{"counter-runtime"}, "assertions": []any{"expected-state"}, "outcome": "expected", "attempts": 1, "trace": trace, "screenshot": screenshot},
				"authority_binding":    map[string]any{"authority_artifact_id": "counter-protocol", "authority_surface_id": behavior, "atomic_behavior_id": behavior, "source_digest": authorityArtifactDigest},
				"runtime_identity":     map[string]any{"evidence_id": evidenceID, "execution_mode": "runtime", "profile": "local", "source_digest": sourceIdentity, "harness_path": "harness.txt", "harness_digest": harnessDigest, "environment": environment, "environment_digest": environmentIdentity, "artifact_path": artifactPath, "artifact_digest": shaDigest(artifactData)},
				"closure":              map[string]any{"dedicated_row": true, "dedicated_artifact": true, "pattern_specific_evidence": true, "real_runtime_identity": true, "integrated_runtime_trace": true, "authority_atomic_behavior": true, "completion_eligible": true}, "gaps": []any{},
			}
			rowPath := "evidence/scenarios/patterns/" + strings.ReplaceAll(behavior, ".", "/") + "/" + scenario + ".proof.json"
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(rowPath))), 0o755); err != nil {
				t.Fatal(err)
			}
			mustWriteJSON(t, filepath.Join(dir, filepath.FromSlash(rowPath)), row)
			files = append(files, map[string]any{"id": row["id"], "pattern_id": behavior, "behavior_id": behavior, "scenario": scenario, "path": rowPath, "digest": fileDigest(t, filepath.Join(dir, filepath.FromSlash(rowPath))), "status": "completion-eligible-runtime-proof"})
		}
	}
	mustWriteYAML(t, filepath.Join(dir, "coverage.yaml"), coverage)
	provenance["artifacts"] = provenanceArtifacts
	mustWriteYAML(t, filepath.Join(dir, "provenance.yaml"), provenance)
	routerPath := filepath.Join(dir, "evals", "definitive-skill-router.json")
	router := mustReadYAML(t, routerPath)
	routerSources := router["source_bindings"].(map[string]any)
	masteryBinding := routerSources["mastery_contract"].(map[string]any)
	masteryData, err := os.ReadFile(filepath.Join(dir, "coverage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	masteryBinding["digest"] = shaDigest(masteryData)
	masteryBinding["bytes"] = len(masteryData)
	mustWriteJSON(t, routerPath, router)
	manifest := map[string]any{"schema_version": 1, "id": "counter-reference-system-v1", "status": "verified-integration-system", "subject": "authority-atomic-counter-behavior-integration", "entry": "reference/counter-system.txt", "runtime": "real-counter-local", "test": "counter_test.go", "evidence": "artifacts/reference-system/results.json", "scenarios": manifestScenarios, "completion_limits": []any{"統合Traceは個別Behavior Runtime Artifactと区別して保持する。"}}
	if err := os.MkdirAll(filepath.Join(dir, "integrations", "reference-system"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(dir, "integrations", "reference-system", "manifest.json"), manifest)
	result := map[string]any{"schema_version": 1, "id": "counter-reference-system-v1", "created_at": "2026-08-28T00:00:00Z", "status": "passed", "command": "counter reference runtime", "profile": "local-real-counter", "counts": map[string]any{"total": 10, "passed": 10, "failed": 0, "flaky": 0, "skipped": 0}, "duration_ms": 10, "source_digest": referenceDigest, "harness_digest": harnessDigest, "environment": map[string]any{"runtime": "counter-v1", "platform": "test", "architecture": "test", "version": "1", "worker": "single", "retries": 0, "viewport": "none", "trace_mode": "on"}, "trace_contract": map[string]any{"per_scenario": true, "required_streams": []any{"action", "network", "resource"}, "console_events": "inside-action-trace-stream"}, "completion_limits": []any{"統合成功は個別Behavior Proofの代替に使用しない。"}, "tests": resultTests}
	mustWriteJSON(t, filepath.Join(dir, "artifacts", "reference-system", "results.json"), result)
	patternScenarioResult := map[string]any{"schema_version": 1, "id": "counter-pattern-scenario-runtime-v1", "created_at": "2026-08-28T00:00:00Z", "status": "passed", "command": "counter scenario runtime", "profile": "local-real-counter", "counts": map[string]any{"rows": len(behaviors) * 10, "variants": len(behaviors), "total": len(scenarioRuntimeTests), "passed": len(scenarioRuntimeTests), "failed": 0, "flaky": 0, "skipped": 0}, "source_digest": referenceDigest, "harness_digest": harnessDigest, "environment": scenarioRuntimeEnvironment, "retention_contract": requiredEvidenceDurabilityProfile(), "tests": scenarioRuntimeTests}
	mustWriteJSON(t, filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json"), patternScenarioResult)
	sourceDigests := map[string]any{"integrations/reference-system/manifest.json": fileDigest(t, filepath.Join(dir, "integrations", "reference-system", "manifest.json")), "artifacts/reference-system/results.json": fileDigest(t, filepath.Join(dir, "artifacts", "reference-system", "results.json")), "artifacts/pattern-scenarios/results.json": fileDigest(t, filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json")), "reference/counter-system.txt": referenceDigest, "harness.txt": harnessDigest}
	rows := len(behaviors) * 10
	index := map[string]any{"schema_version": 1, "id": "counter-scenario-proof-matrix-v1", "atlas_id": "counter-reference-atlas", "generated_at": "2026-08-28T00:00:00Z", "status": "completion-eligible", "denominator": "authority-atomic-behaviors-x-10-scenarios", "tool_digest": referenceDigest, "source_digests": sourceDigests, "summary": map[string]any{"patterns": len(behaviors), "scenarios": 10, "rows": rows, "dedicated_artifacts": rows, "pattern_specific_rows": rows, "pattern_specific_runtime_rows": rows, "pattern_specific_capture_rows": 0, "pattern_specific_gaps": 0, "integrated_trace_rows": rows, "authority_atomic_rows": rows, "completion_eligible_rows": rows}, "by_scenario": byScenario, "files": files, "completion_limits": []any{}}
	indexPath := filepath.Join(dir, "evidence", "scenarios", "index.json")
	mustWriteJSON(t, indexPath, index)
	completedRows := []any{}
	for _, scenario := range scenarioClosureRiskOrder {
		for _, behavior := range behaviors {
			completedRows = append(completedRows, map[string]any{"pattern_id": behavior, "scenario": scenario, "oracle_kinds": []any{"missing"}, "variant_ids": []any{behavior + ".default"}, "all_first_attempt_pass": true, "all_trace_streams": true})
		}
	}
	zeroByScenario := map[string]any{}
	for _, scenario := range scenarioClosureRiskOrder {
		zeroByScenario[scenario] = 0
	}
	closurePlan := map[string]any{
		"schema_version": 1, "id": "counter-pattern-scenario-closure-plan-v1", "generated_at": "2026-08-28T00:00:00Z", "status": "complete", "scope": "authority-atomic-counter-scenario-closure",
		"policy":                 map[string]any{"risk_order": stringSliceToAny(scenarioClosureRiskOrder), "maximum_pattern_rows_per_tranche": 4, "monotonic_addition": true, "mass_closure_forbidden": true},
		"source_digests":         map[string]any{"evidence/scenarios/index.json": fileDigest(t, indexPath), "artifacts/pattern-scenarios/results.json": fileDigest(t, filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json"))},
		"baseline":               map[string]any{"inherited_gap_rows_at_fixture": 0, "matrix_rows": rows, "patterns": len(behaviors), "scenarios": 10},
		"summary":                map[string]any{"completed_dedicated_rows": rows, "remaining_rows": 0, "planned_tranches": 0, "by_scenario": zeroByScenario},
		"independent_incomplete": map[string]any{"authority_atomic_rows": rows, "external_profiles": []any{}, "agent_forward_eval": "completed"},
		"completed_rows":         completedRows, "next_tranche": nil, "tranches": []any{}, "rows": []any{},
	}
	mustWriteJSON(t, filepath.Join(dir, "evidence", "scenarios", "closure-plan.json"), closurePlan)
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
	case "authority-body-review-mapping-forgery":
		path := filepath.Join(dir, "authority", "reviews", "decisions.json")
		doc := mustReadYAML(t, path)
		decisions := doc["decisions"].([]any)
		decisions[0].(map[string]any)["mapping"].([]any)[0].(map[string]any)["new_item_ids"] = []any{"counter-protocol.forged.surface"}
		decisions[0].(map[string]any)["result_items"].([]any)[0].(map[string]any)["id"] = "counter-protocol.forged.surface"
		mustWriteJSON(t, path, doc)
	case "automated-authority-reviewer":
		path := filepath.Join(dir, "authority", "reviews", "decisions.json")
		doc := mustReadYAML(t, path)
		doc["decisions"].([]any)[0].(map[string]any)["reviewer"] = "automated-bot"
		mustWriteJSON(t, path, doc)
	case "zero-decision-semantic-exhaustive":
		ledgerPath := filepath.Join(dir, "authority", "reviews", "decisions.json")
		ledger := mustReadYAML(t, ledgerPath)
		ledger["decisions"] = []any{}
		mustWriteJSON(t, ledgerPath, ledger)
		queuePath := filepath.Join(dir, "authority", "review-queue.snapshot.json")
		queue := mustReadYAML(t, queuePath)
		summary := queue["summary"].(map[string]any)
		summary["pending_human"], summary["human_reviewed"], summary["decisions"], summary["included"] = 2, 0, 0, 0
		mustWriteJSON(t, queuePath, queue)
	case "review-queue-anchor-omission":
		batchPath := filepath.Join(dir, "authority", "review-queue-draft", "review-p0-heading-00.json")
		batch := mustReadYAML(t, batchPath)
		batch["items"] = batch["items"].([]any)[:1]
		mustWriteJSON(t, batchPath, batch)
		queuePath := filepath.Join(dir, "authority", "review-queue.snapshot.json")
		queue := mustReadYAML(t, queuePath)
		record := queue["batches"].([]any)[0].(map[string]any)
		record["digest"], record["items"] = fileDigest(t, batchPath), 1
		mustWriteJSON(t, queuePath, queue)
	case "review-result-id-sharing":
		path := filepath.Join(dir, "authority", "reviews", "decisions.json")
		doc := mustReadYAML(t, path)
		second := doc["decisions"].([]any)[1].(map[string]any)
		second["mapping"].([]any)[0].(map[string]any)["new_item_ids"] = []any{"counter-protocol.counter.increment"}
		second["result_items"].([]any)[0].(map[string]any)["id"] = "counter-protocol.counter.increment"
		mustWriteJSON(t, path, doc)
	case "skill-router-gap":
		path := filepath.Join(dir, "evals", "definitive-skill-router.json")
		doc := mustReadYAML(t, path)
		cell := doc["matrix"].([]any)[0].(map[string]any)
		cell["support_status"], cell["coverage_state"], cell["status"] = "mastery-routing-gap", "partial", "mastery-routing-gap"
		summary := doc["summary"].(map[string]any)
		summary["routed"], summary["mastery_routing_gaps"], summary["partial_coverage_cells"] = 111, 1, 1
		doc["status"], doc["completion_limits"] = "incomplete-mastery-routing-gaps", []any{"Authority由来のrouting gapが残っているためSkill Completionを主張できない。"}
		mustWriteJSON(t, path, doc)
	case "skill-router-forward-eval-missing":
		path := filepath.Join(dir, "evals", "definitive-skill-router.json")
		doc := mustReadYAML(t, path)
		doc["forward_eval"] = map[string]any{"status": "not-run", "cases": 0, "passed": 0, "failed": 0, "artifact_path": nil, "artifact_digest": nil}
		doc["status"], doc["completion_limits"] = "incomplete-forward-eval-required", []any{"独立Agentによる実Project Forward Evalが未実施のためSkill Completionを主張できない。"}
		mustWriteJSON(t, path, doc)
	case "skill-router-human-authority-bypass":
		path := filepath.Join(dir, "evals", "definitive-skill-router.json")
		doc := mustReadYAML(t, path)
		for _, raw := range doc["boundary_cases"].([]any) {
			item := raw.(map[string]any)
			if item["id"] == "boundary.human-authority-decision" {
				item["status"], item["mutation_policy"], item["mutation_status"], item["blocked_reasons"] = "routed", "authorized", "completed", []any{}
			}
		}
		mustWriteJSON(t, path, doc)
	case "scenario-integrated-trace-reuse":
		mutateScenarioProofRow(t, dir, "counter.increment", "normal", func(row map[string]any) {
			trace := row["integrated_reference"].(map[string]any)["trace"].(map[string]any)
			runtime := row["runtime_identity"].(map[string]any)
			runtime["artifact_path"], runtime["artifact_digest"] = trace["path"], trace["digest"]
		})
	case "scenario-authority-binding-forgery":
		mutateScenarioProofRow(t, dir, "counter.increment", "normal", func(row map[string]any) {
			row["authority_binding"].(map[string]any)["atomic_behavior_id"] = "counter.reset"
		})
	case "scenario-capture-identity-substitution":
		mutateScenarioProofRow(t, dir, "counter.increment", "normal", func(row map[string]any) {
			patternEvidence := row["pattern_evidence"].(map[string]any)
			patternEvidence["scenario_runtime_report"] = nil
			patternEvidence["scenario_runtime_environment"] = nil
			patternEvidence["scenario_runtime_records"] = []any{}
		})
	case "scenario-dedicated-integrated-trace-reuse":
		mutateDedicatedScenarioProof(t, dir, "counter.increment", "normal", func(row map[string]any, rowRecord, reportRecord map[string]any, report map[string]any) {
			trace := row["integrated_reference"].(map[string]any)["trace"]
			rowRecord["trace"], reportRecord["trace"] = trace, trace
		})
	case "scenario-runtime-retry":
		mutateDedicatedScenarioProof(t, dir, "counter.increment", "normal", func(row map[string]any, _, _ map[string]any, report map[string]any) {
			row["pattern_evidence"].(map[string]any)["scenario_runtime_environment"].(map[string]any)["retries"] = 1
			report["environment"].(map[string]any)["retries"] = 1
		})
	default:
		t.Fatalf("未知のFixture mutation: %s", mutation)
	}
}

func mutateScenarioProofRow(t *testing.T, dir, behavior, scenario string, mutation func(map[string]any)) {
	t.Helper()
	indexPath := filepath.Join(dir, "evidence", "scenarios", "index.json")
	index := mustReadYAML(t, indexPath)
	for _, raw := range index["files"].([]any) {
		record := raw.(map[string]any)
		if record["behavior_id"] != behavior || record["scenario"] != scenario {
			continue
		}
		rowPath := filepath.Join(dir, filepath.FromSlash(record["path"].(string)))
		row := mustReadYAML(t, rowPath)
		mutation(row)
		mustWriteJSON(t, rowPath, row)
		record["digest"] = fileDigest(t, rowPath)
		mustWriteJSON(t, indexPath, index)
		return
	}
	t.Fatalf("Scenario Proof rowが見つかりません: %s:%s", behavior, scenario)
}

func mutateDedicatedScenarioProof(t *testing.T, dir, behavior, scenario string, mutation func(map[string]any, map[string]any, map[string]any, map[string]any)) {
	t.Helper()
	indexPath := filepath.Join(dir, "evidence", "scenarios", "index.json")
	index := mustReadYAML(t, indexPath)
	for _, raw := range index["files"].([]any) {
		record := raw.(map[string]any)
		if record["behavior_id"] != behavior || record["scenario"] != scenario {
			continue
		}
		rowPath := filepath.Join(dir, filepath.FromSlash(record["path"].(string)))
		row := mustReadYAML(t, rowPath)
		patternEvidence := row["pattern_evidence"].(map[string]any)
		rowRecord := patternEvidence["scenario_runtime_records"].([]any)[0].(map[string]any)
		reportRelative := patternEvidence["scenario_runtime_report"].(string)
		reportPath := filepath.Join(dir, filepath.FromSlash(reportRelative))
		report := mustReadYAML(t, reportPath)
		var reportRecord map[string]any
		for _, rawReportRecord := range report["tests"].([]any) {
			candidate := rawReportRecord.(map[string]any)
			if candidate["pattern_id"] == behavior && candidate["scenario"] == scenario && candidate["variant_id"] == rowRecord["variant_id"] {
				reportRecord = candidate
				break
			}
		}
		if reportRecord == nil {
			t.Fatalf("専用Scenario report recordが見つかりません: %s:%s", behavior, scenario)
		}
		mutation(row, rowRecord, reportRecord, report)
		mustWriteJSON(t, reportPath, report)
		index["source_digests"].(map[string]any)[reportRelative] = fileDigest(t, reportPath)
		mustWriteJSON(t, rowPath, row)
		record["digest"] = fileDigest(t, rowPath)
		mustWriteJSON(t, indexPath, index)
		return
	}
	t.Fatalf("Scenario Proof rowが見つかりません: %s:%s", behavior, scenario)
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
