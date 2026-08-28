package validate

import "testing"

func TestScenarioTraceCompletionEligibleFixture(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	result, err := AuditScenarioTrace(dir, "evidence/scenarios/index.json", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Patterns != 2 || result.Rows != 20 || result.DedicatedArtifactRows != 20 || result.PatternSpecificRows != 20 || result.RuntimeIdentityRows != 20 || result.PatternSpecificGaps != 0 || result.IntegratedTraceRows != 20 || result.DedicatedScenarioRows != 20 || result.ScenarioClosureGaps != 0 || result.AuthorityAtomicRows != 20 || result.CompletionEligibleRows != 20 || result.IntegratedScenarioTests != 10 || result.CompletionLimited {
		t.Fatalf("Scenario/Trace集計がFixture実体と一致しません: %+v", result)
	}
}

func TestScenarioTraceRejectsIntegratedTraceAsAtomicRuntimeArtifact(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	applyDefinitiveMutation(t, dir, "scenario-integrated-trace-reuse")
	if _, err := AuditScenarioTrace(dir, "evidence/scenarios/index.json", true); err == nil {
		t.Fatal("統合Traceを個別Atomic Behavior Runtime Artifactへ流用できてはいけません")
	}
}
