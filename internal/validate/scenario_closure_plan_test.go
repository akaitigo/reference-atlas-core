package validate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type scenarioClosurePlanCase struct {
	Mutation           string `yaml:"mutation"`
	PlanError          string `yaml:"plan_error"`
	NonRegressionError string `yaml:"non_regression_error"`
}

func TestScenarioClosurePlanFixture(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	makeStagingScenarioClosurePlan(t, dir)
	result, err := AuditScenarioClosurePlan(dir, "evidence/scenarios/closure-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingRows != 5 || result.CompletedRows != 15 || result.PlannedTranches != 3 || result.MaximumRowsPerTranche != 4 || result.NextTranche != "security-001" {
		t.Fatalf("段階的Closure Plan集計が実体と一致しません: %+v", result)
	}
}

func TestScenarioClosurePlanNegativeFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "scenario-closure-plan", "cases", "*.yaml"))
	if err != nil || len(paths) != 3 {
		t.Fatalf("Scenario Closure Plan negative fixtureが不足しています: paths=%d err=%v", len(paths), err)
	}
	for _, path := range paths {
		path := path
		t.Run(strings.TrimSuffix(filepath.Base(path), ".yaml"), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var tc scenarioClosurePlanCase
			if err := yaml.Unmarshal(data, &tc); err != nil {
				t.Fatal(err)
			}
			dir := createDefinitiveRepositoryFixture(t)
			makeStagingScenarioClosurePlan(t, dir)
			baselinePath := filepath.Join(dir, "baselines", "scenario-plan.non-regression-baseline.json")
			if _, err := GenerateNonRegressionBaseline(dir, baselinePath, strings.Repeat("c", 40), "2026-08-28T00:00:00Z"); err != nil {
				t.Fatal(err)
			}
			mustWriteYAML(t, filepath.Join(dir, "non-regression.yaml"), map[string]any{"schema_version": 2, "atlas_id": "counter-reference-atlas", "baseline": map[string]any{"path": "baselines/scenario-plan.non-regression-baseline.json", "digest": fileDigest(t, baselinePath)}, "replacements": []any{}})
			mutateScenarioClosurePlan(t, dir, tc.Mutation)
			if _, err := AuditScenarioClosurePlan(dir, "evidence/scenarios/closure-plan.json"); err == nil || !strings.Contains(err.Error(), tc.PlanError) {
				t.Fatalf("Closure Plan回避を拒否できません: want=%q err=%v", tc.PlanError, err)
			}
			if _, err := AuditNonRegression(dir); err == nil || !strings.Contains(err.Error(), tc.NonRegressionError) {
				t.Fatalf("Non-regressionがClosure Plan回避を拒否できません: want=%q err=%v", tc.NonRegressionError, err)
			}
		})
	}
}

func makeStagingScenarioClosurePlan(t *testing.T, dir string) {
	t.Helper()
	selected := map[string]bool{"counter.increment:security": true, "counter.reset:security": true, "counter.increment:refusal": true, "counter.reset:refusal": true, "counter.increment:failure": true}
	reportPath := filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json")
	report := mustReadYAML(t, reportPath)
	keptTests := []any{}
	for _, raw := range report["tests"].([]any) {
		record := raw.(map[string]any)
		if !selected[stringValue(record["pattern_id"])+":"+stringValue(record["scenario"])] {
			keptTests = append(keptTests, record)
			continue
		}
		for _, field := range []string{"trace", "screenshot"} {
			artifact := record[field].(map[string]any)
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(stringValue(artifact["path"])))); err != nil {
				t.Fatal(err)
			}
		}
	}
	report["tests"] = keptTests
	report["counts"] = map[string]any{"rows": 15, "variants": 2, "total": 15, "passed": 15, "failed": 0, "flaky": 0, "skipped": 0}
	mustWriteJSON(t, reportPath, report)
	indexPath := filepath.Join(dir, "evidence", "scenarios", "index.json")
	index := mustReadYAML(t, indexPath)
	index["source_digests"].(map[string]any)["artifacts/pattern-scenarios/results.json"] = fileDigest(t, reportPath)
	for _, raw := range index["files"].([]any) {
		record := raw.(map[string]any)
		key := stringValue(record["behavior_id"]) + ":" + stringValue(record["scenario"])
		if !selected[key] {
			continue
		}
		rowPath := filepath.Join(dir, filepath.FromSlash(stringValue(record["path"])))
		row := mustReadYAML(t, rowPath)
		row["status"] = "pattern-specific-gap"
		patternEvidence := row["pattern_evidence"].(map[string]any)
		patternEvidence["scenario_runtime_report"], patternEvidence["scenario_runtime_environment"], patternEvidence["scenario_runtime_records"] = nil, nil, []any{}
		closure := row["closure"].(map[string]any)
		closure["pattern_specific_evidence"], closure["real_runtime_identity"], closure["completion_eligible"] = false, false, false
		row["gaps"] = []any{"専用Scenario Runtime suiteが未実行であるためClosure対象外。"}
		mustWriteJSON(t, rowPath, row)
		record["status"], record["digest"] = "pattern-specific-gap", fileDigest(t, rowPath)
		stats := index["by_scenario"].(map[string]any)[stringValue(record["scenario"])].(map[string]any)
		stats["pattern_specific"] = int(numberValue(stats["pattern_specific"])) - 1
		stats["runtime_identity"] = int(numberValue(stats["runtime_identity"])) - 1
		stats["gaps"] = int(numberValue(stats["gaps"])) + 1
	}
	index["status"] = "incomplete-scenario-gaps"
	index["completion_limits"] = []any{"5件の専用Scenario Runtime suiteが未実行であるためCompletion対象外。"}
	summary := index["summary"].(map[string]any)
	summary["pattern_specific_rows"], summary["pattern_specific_runtime_rows"], summary["pattern_specific_gaps"], summary["completion_eligible_rows"] = 15, 15, 5, 15
	mustWriteJSON(t, indexPath, index)

	rows := []closurePlanRow{}
	for _, raw := range index["files"].([]any) {
		record := raw.(map[string]any)
		if record["status"] != "pattern-specific-gap" {
			continue
		}
		proof := mustReadYAML(t, filepath.Join(dir, filepath.FromSlash(stringValue(record["path"]))))
		variants := []string{}
		for _, rawBinding := range proof["source_bindings"].([]any) {
			variants = append(variants, stringValue(rawBinding.(map[string]any)["variant_id"]))
		}
		scenario, patternID := stringValue(proof["scenario"]), stringValue(proof["pattern_id"])
		rank := 0
		for position, candidate := range scenarioClosureRiskOrder {
			if candidate == scenario {
				rank = position + 1
			}
		}
		document := map[string]any{"id": "closure." + strings.ReplaceAll(patternID, "/", ".") + "." + scenario, "pattern_id": patternID, "target_id": proof["target_id"], "scenario": scenario, "risk_rank": rank, "proof": map[string]any{"path": record["path"], "digest": record["digest"]}, "variant_ids": stringSliceToAny(variants), "required_closure": requiredScenarioClosureContract(), "gaps": proof["gaps"]}
		rows = append(rows, closurePlanRow{id: stringValue(document["id"]), patternID: patternID, scenario: scenario, riskRank: rank, variantIDs: variants, document: document})
	}
	sortClosurePlanRows(rows)
	tranches := buildExpectedClosureTranches(rows, map[string]int{})
	rowDocuments := []any{}
	byScenario := map[string]any{}
	for _, scenario := range scenarioClosureRiskOrder {
		byScenario[scenario] = 0
	}
	for _, row := range rows {
		rowDocuments = append(rowDocuments, row.document)
		byScenario[row.scenario] = int(numberValue(byScenario[row.scenario])) + 1
	}
	completed := buildCompletedClosureRows(report, func() map[string]int {
		result := map[string]int{}
		for position, scenario := range scenarioClosureRiskOrder {
			result[scenario] = position + 1
		}
		return result
	}())
	plan := map[string]any{"schema_version": 1, "id": "counter-pattern-scenario-closure-plan-v1", "generated_at": "2026-08-28T00:00:00Z", "status": "incomplete", "scope": "counter-pattern-scenario-gap-plan", "policy": map[string]any{"risk_order": stringSliceToAny(scenarioClosureRiskOrder), "maximum_pattern_rows_per_tranche": 4, "monotonic_addition": true, "mass_closure_forbidden": true}, "source_digests": map[string]any{"evidence/scenarios/index.json": fileDigest(t, indexPath), "artifacts/pattern-scenarios/results.json": fileDigest(t, reportPath)}, "baseline": map[string]any{"inherited_gap_rows_at_fixture": 5, "matrix_rows": 20, "patterns": 2, "scenarios": 10}, "summary": map[string]any{"completed_dedicated_rows": 15, "remaining_rows": 5, "planned_tranches": 3, "by_scenario": byScenario}, "independent_incomplete": map[string]any{"authority_atomic_rows": 20, "external_profiles": []any{}, "agent_forward_eval": "completed"}, "completed_rows": completed, "next_tranche": tranches[0], "tranches": tranches, "rows": rowDocuments}
	mustWriteJSON(t, filepath.Join(dir, "evidence", "scenarios", "closure-plan.json"), plan)
}

func sortClosurePlanRows(rows []closurePlanRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].riskRank != rows[j].riskRank {
			return rows[i].riskRank < rows[j].riskRank
		}
		return rows[i].patternID < rows[j].patternID
	})
}

func mutateScenarioClosurePlan(t *testing.T, dir, mutation string) {
	t.Helper()
	path := filepath.Join(dir, "evidence", "scenarios", "closure-plan.json")
	plan := mustReadYAML(t, path)
	switch mutation {
	case "delete-row":
		plan["rows"] = plan["rows"].([]any)[1:]
	case "retreat-order":
		rows := plan["rows"].([]any)
		rows[0], rows[2] = rows[2], rows[0]
	case "inflate-tranche":
		tranches := plan["tranches"].([]any)
		first, second := tranches[0].(map[string]any), tranches[1].(map[string]any)
		first["row_ids"] = append(first["row_ids"].([]any), second["row_ids"].([]any)[0])
		first["pattern_rows"] = 5
		first["variant_runs"] = int(numberValue(first["variant_runs"])) + 1
	default:
		t.Fatalf("未知のScenario Closure Plan mutation: %s", mutation)
	}
	mustWriteJSON(t, path, plan)
}
