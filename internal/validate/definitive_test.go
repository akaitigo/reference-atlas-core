package validate

import (
	"strings"
	"testing"
)

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
		map[string]any{"id": "counter.increment.rejection"},
	}
	evidence := map[string]map[string]any{}
	rows := []any{}
	for _, scenario := range definitiveScenarios {
		row := map[string]any{"behavior_id": "counter.increment", "scenario": scenario, "applicability": "not-applicable", "proof_obligation_id": nil, "evidence_ids": []any{}, "execution_requirement": "not-applicable", "profile": nil}
		if scenario == "normal" || scenario == "boundary" || scenario == "rejection" {
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
		base:             &auditContext{evidence: evidence, targets: []any{map[string]any{"id": "counter.increment", "evidence_ids": []any{"ev.normal", "ev.boundary", "ev.rejection"}}}},
		behaviors:        map[string]map[string]any{"counter.increment": behavior},
		claimsByBehavior: map[string]map[string]any{"counter.increment": {"proof_obligations": proofs}}, matrix: map[string]any{"rows": rows},
	}
}
