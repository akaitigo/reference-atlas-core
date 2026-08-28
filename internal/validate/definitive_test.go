package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalFEDepthReferencePreservesIncompleteStatus(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "FE_DEPTH_REFERENCE.json")
	if _, err := File(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := shaDigest(data); got != feDepthReferenceDigest {
		t.Fatalf("FE Depth Reference digestが正本Commitと一致しません: %s", got)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if err := auditFEDepthReferenceContract(doc); err != nil {
		t.Fatal(err)
	}
	if doc["status"] != "incomplete" {
		t.Fatalf("FEをcomplete扱いしてはいけません: %v", doc["status"])
	}
}

func TestFEDepthParityIgnoresPortableRawCounts(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "FE_DEPTH_REFERENCE.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	density := doc["observedDensity"].(map[string]any)
	density["targets"] = float64(1)
	density["variants"] = float64(1_000_000)
	density["lockedE2ETests"] = float64(0)
	if err := auditFEDepthReferenceContract(doc); err != nil {
		t.Fatalf("FE固有の観測件数を他Subjectの合否閾値として読んではいけません: %v", err)
	}
}

func TestDepthParitySchemaAllowsHonestIncompleteStaging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "depth.parity.yaml")
	content := "schema_version: 2\natlas_id: sample-reference-atlas\nepoch: \"2026-08-28\"\ncompletion_status: incomplete\nreference: {id: fe-depth-reference-v1, path: authority/FE_DEPTH_REFERENCE.json, digest: " + feDepthReferenceDigest + ", repository: frontend-behavior-atlas, commit: " + feDepthReferenceCommit + ", status_at_commit: incomplete}\ndenominator_policy: {source: authority-derived-subject-surface-inventory, transplant_absolute_counts: false}\nrows: []\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err != nil {
		t.Fatalf("移行中のGapを捏造Parityなしで保存できる必要があります: %v", err)
	}
}

func TestCertificateGenerationRejectsUnknownGitCommit(t *testing.T) {
	err := ensureSourceCommitExists(".", strings.Repeat("a", 40))
	if err == nil || !strings.Contains(err.Error(), "Git履歴に存在しません") {
		t.Fatalf("存在しないsource commitをCertificate生成に使えてはいけません: %v", err)
	}
}

func TestDefinitiveRejectsRequiredExcluded(t *testing.T) {
	ctx := &definitiveContext{base: &auditContext{targets: []any{map[string]any{"id": "runtime.native", "requirement": "required", "state": "infeasible"}}}}
	err := auditDefinitiveRequiredTargets(ctx)
	if err == nil || !strings.Contains(err.Error(), "covered") {
		t.Fatalf("required infeasibleを決定版として受理してはいけません: %v", err)
	}
}

func TestDefinitiveRejectsStaticFixtureForRuntimeProof(t *testing.T) {
	ctx := proofMatrixFixture()
	ctx.base.evidence["ev.normal"]["execution_mode"] = "fixture"
	_, _, _, err := auditDefinitiveProofMatrix(ctx)
	if err == nil || !strings.Contains(err.Error(), "代替できません") {
		t.Fatalf("static fixtureをRuntime Evidenceとして拒否する必要があります: %v", err)
	}
}

func TestDefinitiveRejectsAggregatedEvidence(t *testing.T) {
	ctx := proofMatrixFixture()
	rows := ctx.matrix["rows"].([]any)
	boundary := rows[1].(map[string]any)
	boundary["evidence_ids"] = []any{"ev.normal"}
	_, _, _, err := auditDefinitiveProofMatrix(ctx)
	if err == nil || !strings.Contains(err.Error(), "集約Evidence") {
		t.Fatalf("1件のEvidenceを複数Behavior/Scenarioへ集約できてはいけません: %v", err)
	}
}

func TestDefinitiveProofMatrixAcceptsExplicitBehaviorProofs(t *testing.T) {
	ctx := proofMatrixFixture()
	proofs, rows, runtime, err := auditDefinitiveProofMatrix(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proofs != 3 || rows != 3 || runtime != 3 {
		t.Fatalf("予期しない決定版Proof結果: proofs=%d rows=%d runtime=%d", proofs, rows, runtime)
	}
}

func TestDefinitiveProofMatrixAcceptsLegacyRejectionSpelling(t *testing.T) {
	ctx := proofMatrixFixture()
	for _, raw := range ctx.matrix["rows"].([]any) {
		row := raw.(map[string]any)
		if row["scenario"] == "refusal" {
			row["scenario"] = "rejection"
		}
	}
	if _, _, _, err := auditDefinitiveProofMatrix(ctx); err != nil {
		t.Fatalf("v2 pre-releaseのrejection識別子はrefusalへ互換正規化する必要があります: %v", err)
	}
}

func TestDefinitiveRequiresIntegratedReferenceSystemWhenApplicable(t *testing.T) {
	ctx := &definitiveContext{base: &auditContext{}, manifest: map[string]any{"reference_systems": []any{}, "comparisons": []any{}}, behaviors: map[string]map[string]any{
		"counter.increment": {"surface_ids": []any{"architecture-design"}},
		"counter.reset":     {"surface_ids": []any{"testing-verification"}},
	}}
	err := auditReferenceSystemsAndComparisons(ctx)
	if err == nil || !strings.Contains(err.Error(), "統合Reference System") {
		t.Fatalf("適用分野の統合Reference System欠落を拒否する必要があります: %v", err)
	}
}

func TestDefinitiveRequiresComparisonWhenApplicable(t *testing.T) {
	ctx := &definitiveContext{base: &auditContext{}, manifest: map[string]any{"reference_systems": []any{}, "comparisons": []any{}}, behaviors: map[string]map[string]any{
		"counter.increment": {"surface_ids": []any{"decision-comparison"}},
	}}
	err := auditReferenceSystemsAndComparisons(ctx)
	if err == nil || !strings.Contains(err.Error(), "複数方式Comparison") {
		t.Fatalf("適用分野のComparison欠落を拒否する必要があります: %v", err)
	}
}

func proofMatrixFixture() *definitiveContext {
	behavior := map[string]any{"behavior_id": "counter.increment", "target_id": "counter.increment", "surface_ids": []any{"testing-verification"}, "claim_ids": []any{"counter.increment"}}
	proofs := []any{
		map[string]any{"id": "counter.increment.normal"},
		map[string]any{"id": "counter.increment.boundary"},
		map[string]any{"id": "counter.increment.refusal"},
	}
	evidence := map[string]map[string]any{}
	rows := []any{}
	for _, scenario := range definitiveScenarios {
		row := map[string]any{"behavior_id": "counter.increment", "scenario": scenario, "applicability": "not-applicable", "proof_obligation_id": nil, "evidence_ids": []any{}, "execution_requirement": "not-applicable", "profile": nil}
		if scenario == "normal" || scenario == "boundary" || scenario == "refusal" {
			evidenceID := "ev." + scenario
			row["applicability"] = "required"
			row["proof_obligation_id"] = "counter.increment." + scenario
			row["evidence_ids"] = []any{evidenceID}
			row["execution_requirement"] = "runtime"
			row["profile"] = "local"
			evidence[evidenceID] = map[string]any{
				"verdict": "pass", "claim_ids": []any{"counter.increment"}, "execution_mode": "runtime", "runtime_identity": "counter-runtime-v1",
				"environment": map[string]any{"profile": "local"}, "artifact": map[string]any{"uri": "reports/" + scenario + ".json"},
			}
		}
		rows = append(rows, row)
	}
	return &definitiveContext{
		base:             &auditContext{evidence: evidence, targets: []any{map[string]any{"id": "counter.increment", "evidence_ids": []any{"ev.normal", "ev.boundary", "ev.refusal"}}}},
		behaviors:        map[string]map[string]any{"counter.increment": behavior},
		claimsByBehavior: map[string]map[string]any{"counter.increment": {"proof_obligations": proofs}}, matrix: map[string]any{"rows": rows},
	}
}
