package validate

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

var scenarioClosureRiskOrder = []string{"security", "refusal", "failure", "recovery", "migration", "operations", "boundary", "performance", "compatibility", "normal"}

type ScenarioClosurePlanResult struct {
	AtlasID               string
	RemainingRows         int
	CompletedRows         int
	PlannedTranches       int
	MaximumRowsPerTranche int
	NextTranche           string
}

type closurePlanRow struct {
	id         string
	patternID  string
	scenario   string
	riskRank   int
	variantIDs []string
	document   map[string]any
}

func AuditScenarioClosurePlan(dir, relative string) (ScenarioClosurePlanResult, error) {
	if _, err := AuditEvidenceDurability(dir, "artifacts/pattern-scenarios/results.json"); err != nil {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Evidence durability: %w", err)
	}
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if _, err := File(path); err != nil {
		return ScenarioClosurePlanResult{}, err
	}
	plan, err := readDocument(path)
	if err != nil {
		return ScenarioClosurePlanResult{}, err
	}
	for sourcePath, rawDigest := range plan["source_digests"].(map[string]any) {
		if err := verifyRelativeFileDigest(dir, sourcePath, stringValue(rawDigest), -1); err != nil {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan source: %w", err)
		}
	}
	indexPath := "evidence/scenarios/index.json"
	if _, err := AuditScenarioTrace(dir, indexPath, false); err != nil {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure PlanのScenario Indexが無効です: %w", err)
	}
	index, err := readDocument(filepath.Join(dir, indexPath))
	if err != nil {
		return ScenarioClosurePlanResult{}, err
	}
	report, err := readDocument(filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json"))
	if err != nil {
		return ScenarioClosurePlanResult{}, err
	}
	riskRank := map[string]int{}
	for position, scenario := range scenarioClosureRiskOrder {
		riskRank[scenario] = position + 1
	}
	expectedRows := []closurePlanRow{}
	for _, rawRecord := range anySlice(index["files"]) {
		record, _ := rawRecord.(map[string]any)
		if record["status"] != "pattern-specific-gap" {
			continue
		}
		proofPath := stringValue(record["path"])
		proof, err := readDocument(filepath.Join(dir, filepath.FromSlash(proofPath)))
		if err != nil {
			return ScenarioClosurePlanResult{}, err
		}
		patternID, scenario := stringValue(proof["pattern_id"]), stringValue(proof["scenario"])
		variants := []string{}
		for _, rawBinding := range anySlice(proof["source_bindings"]) {
			binding, _ := rawBinding.(map[string]any)
			variants = append(variants, stringValue(binding["variant_id"]))
		}
		row := map[string]any{
			"id": "closure." + strings.ReplaceAll(patternID, "/", ".") + "." + scenario, "pattern_id": patternID, "target_id": proof["target_id"], "scenario": scenario, "risk_rank": riskRank[scenario],
			"proof": map[string]any{"path": proofPath, "digest": record["digest"]}, "variant_ids": stringSliceToAny(variants), "required_closure": requiredScenarioClosureContract(), "gaps": proof["gaps"],
		}
		expectedRows = append(expectedRows, closurePlanRow{id: stringValue(row["id"]), patternID: patternID, scenario: scenario, riskRank: riskRank[scenario], variantIDs: variants, document: row})
	}
	sort.Slice(expectedRows, func(i, j int) bool {
		if expectedRows[i].riskRank != expectedRows[j].riskRank {
			return expectedRows[i].riskRank < expectedRows[j].riskRank
		}
		return expectedRows[i].patternID < expectedRows[j].patternID
	})
	actualRows := anySlice(plan["rows"])
	if len(actualRows) != len(expectedRows) {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Planが全Gap rowを完全包含していません: expected=%d actual=%d", len(expectedRows), len(actualRows))
	}
	for index, expected := range expectedRows {
		if !sameCanonical(actualRows[index], expected.document) {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan rowがrisk順またはProof実体と一致しません: ordinal=%d expected=%s", index+1, expected.id)
		}
	}
	completed := buildCompletedClosureRows(report, riskRank)
	if !sameCanonical(plan["completed_rows"], completed) {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan completed_rowsが専用Runtime reportと一致しません")
	}
	completedTrancheCounts, closedPlannedRows, err := auditCompletedClosureTranches(plan, completed, expectedRows, riskRank)
	if err != nil {
		return ScenarioClosurePlanResult{}, err
	}
	expectedTranches := buildExpectedClosureTranches(expectedRows, completedTrancheCounts)
	actualTranches := anySlice(plan["tranches"])
	if len(actualTranches) != len(expectedTranches) {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan tranche数が4 row上限の実体と一致しません: expected=%d actual=%d", len(expectedTranches), len(actualTranches))
	}
	for index := range expectedTranches {
		if !sameCanonical(actualTranches[index], expectedTranches[index]) {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan trancheがrisk順、4 row上限、またはVariant実行数と一致しません: ordinal=%d", index+1)
		}
	}
	if len(expectedTranches) == 0 {
		if plan["next_tranche"] != nil || plan["status"] != "complete" {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Gap 0のClosure Planはstatus=completeかつnext_tranche=nullである必要があります")
		}
	} else if !sameCanonical(plan["next_tranche"], expectedTranches[0]) || plan["status"] != "incomplete" {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Planのnext_trancheまたはstatusがrisk順の先頭と一致しません")
	}
	completedKeys := map[string]bool{}
	for _, raw := range completed {
		item, _ := raw.(map[string]any)
		completedKeys[stringValue(item["pattern_id"])+":"+stringValue(item["scenario"])] = true
	}
	for _, row := range expectedRows {
		if completedKeys[row.patternID+":"+row.scenario] {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan rowを専用Runtime completed rowと同時に未完扱いできません: %s", row.id)
		}
	}
	summary, _ := plan["summary"].(map[string]any)
	byScenario, _ := summary["by_scenario"].(map[string]any)
	actualByScenario := map[string]int{}
	for _, row := range expectedRows {
		actualByScenario[row.scenario]++
	}
	for _, scenario := range scenarioClosureRiskOrder {
		if int(numberValue(byScenario[scenario])) != actualByScenario[scenario] {
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan by_scenarioが実体と一致しません: %s", scenario)
		}
	}
	if int(numberValue(summary["completed_dedicated_rows"])) != len(completed) || int(numberValue(summary["remaining_rows"])) != len(expectedRows) || int(numberValue(summary["planned_tranches"])) != len(expectedTranches) {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan summaryが実体と一致しません")
	}
	completedTrancheTotal := len(anySlice(plan["completed_tranches"]))
	if _, present := summary["completed_planned_tranches"]; present && int(numberValue(summary["completed_planned_tranches"])) != completedTrancheTotal {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan completed planned tranche集計が実体と一致しません")
	} else if completedTrancheTotal > 0 && !present {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan completed planned tranche集計がありません")
	}
	closedSummaryFound := false
	for key, value := range summary {
		if strings.HasPrefix(key, "closed_since_") {
			closedSummaryFound = true
			if int(numberValue(value)) != closedPlannedRows {
				return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan closed-since集計が実体と一致しません: %s", key)
			}
		}
	}
	if completedTrancheTotal > 0 && !closedSummaryFound {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan completed trancheにclosed-since集計がありません")
	}
	indexSummary, _ := index["summary"].(map[string]any)
	baseline, _ := plan["baseline"].(map[string]any)
	if int(numberValue(baseline["matrix_rows"])) != int(numberValue(indexSummary["rows"])) || int(numberValue(baseline["patterns"])) != int(numberValue(indexSummary["patterns"])) || int(numberValue(baseline["scenarios"])) != 10 {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan baseline denominatorがScenario Indexと一致しません")
	}
	inheritedFound, plannedBaselineFound := false, false
	for key, raw := range baseline {
		if strings.HasPrefix(key, "inherited_gap_rows_at_") {
			inheritedFound = true
			if int(numberValue(raw)) < len(expectedRows) {
				return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan inherited gap baselineを縮小できません")
			}
		}
		if strings.HasPrefix(key, "planned_gap_rows_at_") && int(numberValue(raw))-len(expectedRows) != closedPlannedRows {
			plannedBaselineFound = true
			return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan planned gap baselineと完了履歴が一致しません: %s", key)
		} else if strings.HasPrefix(key, "planned_gap_rows_at_") {
			plannedBaselineFound = true
		}
	}
	if !inheritedFound {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Planに固定commit由来のinherited gap baselineがありません")
	}
	if completedTrancheTotal > 0 && !plannedBaselineFound {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Plan completed trancheに固定planned gap baselineがありません")
	}
	independent, _ := plan["independent_incomplete"].(map[string]any)
	if int(numberValue(independent["authority_atomic_rows"])) != int(numberValue(indexSummary["authority_atomic_rows"])) {
		return ScenarioClosurePlanResult{}, fmt.Errorf("Scenario Closure Planの独立Authority未完軸がIndexと一致しません")
	}
	next := ""
	if item, _ := plan["next_tranche"].(map[string]any); item != nil {
		next = stringValue(item["id"])
	}
	return ScenarioClosurePlanResult{AtlasID: stringValue(index["atlas_id"]), RemainingRows: len(expectedRows), CompletedRows: len(completed), PlannedTranches: len(expectedTranches), MaximumRowsPerTranche: 4, NextTranche: next}, nil
}

func requiredScenarioClosureContract() map[string]any {
	return map[string]any{"drive_pattern_scenario_and_every_variant": true, "first_attempt_only": true, "retries": 0, "dedicated_runtime_identity": true, "dedicated_oracle": true, "separate_trace_per_variant": true, "required_trace_streams": []any{"action", "network", "resource"}, "separate_screenshot_per_variant": true, "source_and_harness_digests": true, "forbidden_substitutions": []any{"metadata-only", "capture-reuse", "integrated-trace-reuse", "mock-or-static-runtime"}}
}

func auditCompletedClosureTranches(plan map[string]any, completed []any, remaining []closurePlanRow, riskRank map[string]int) (map[string]int, int, error) {
	completedByID := map[string]map[string]any{}
	for _, raw := range completed {
		row, _ := raw.(map[string]any)
		id := "closure." + strings.ReplaceAll(stringValue(row["pattern_id"]), "/", ".") + "." + stringValue(row["scenario"])
		completedByID[id] = row
	}
	counts, used, closedByScenario := map[string]int{}, map[string]bool{}, map[string][]string{}
	closedRows, lastRisk := 0, 0
	for _, raw := range anySlice(plan["completed_tranches"]) {
		tranche, _ := raw.(map[string]any)
		scenario := stringValue(tranche["scenario"])
		rank := riskRank[scenario]
		if rank == 0 || rank < lastRisk {
			return nil, 0, fmt.Errorf("Scenario Closure Plan completed trancheがrisk順ではありません")
		}
		lastRisk = rank
		counts[scenario]++
		rowIDs := anySlice(tranche["row_ids"])
		expectedRowIDs, oracleKinds, variantRuns := []any{}, []string{}, 0
		seenOracle := map[string]bool{}
		previousID := ""
		for _, rawID := range rowIDs {
			id := stringValue(rawID)
			row := completedByID[id]
			if row == nil || stringValue(row["scenario"]) != scenario || used[id] || (previousID != "" && id < previousID) {
				return nil, 0, fmt.Errorf("Scenario Closure Plan completed trancheが専用Runtime完了rowの安定順と一致しません: %s", id)
			}
			used[id], previousID = true, id
			expectedRowIDs = append(expectedRowIDs, id)
			closedByScenario[scenario] = append(closedByScenario[scenario], id)
			variantRuns += len(anySlice(row["variant_ids"]))
			for _, rawKind := range anySlice(row["oracle_kinds"]) {
				kind := stringValue(rawKind)
				if !seenOracle[kind] {
					seenOracle[kind], oracleKinds = true, append(oracleKinds, kind)
				}
			}
		}
		expected := map[string]any{
			"id": fmt.Sprintf("%s-%03d", scenario, counts[scenario]), "risk_rank": rank, "scenario": scenario, "status": "completed",
			"row_ids": expectedRowIDs, "pattern_rows": len(expectedRowIDs), "variant_runs": variantRuns, "oracle_kinds": stringSliceToAny(oracleKinds),
			"commit_policy": "one-reviewed-tranche-with-non-regression-runtime-identity-and-oracle-validation",
		}
		if !sameCanonical(tranche, expected) {
			return nil, 0, fmt.Errorf("Scenario Closure Plan completed trancheのrow、Variant、Oracle実体が一致しません: %s", tranche["id"])
		}
		closedRows += len(rowIDs)
	}
	if len(remaining) > 0 && lastRisk > remaining[0].riskRank {
		return nil, 0, fmt.Errorf("Scenario Closure Plan completed trancheが未完risk rowを飛び越えています")
	}
	for _, scenario := range scenarioClosureRiskOrder {
		ordered := append([]string{}, closedByScenario[scenario]...)
		for _, row := range remaining {
			if row.scenario == scenario {
				ordered = append(ordered, row.id)
			}
		}
		sorted := append([]string{}, ordered...)
		sort.Strings(sorted)
		if !sameStringSlice(ordered, sorted) {
			return nil, 0, fmt.Errorf("Scenario Closure Plan completed trancheが同一Scenarioの安定Pattern順を退避しています: %s", scenario)
		}
	}
	return counts, closedRows, nil
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func buildExpectedClosureTranches(rows []closurePlanRow, completedCounts map[string]int) []any {
	tranches := []any{}
	for _, scenario := range scenarioClosureRiskOrder {
		scenarioRows := []closurePlanRow{}
		for _, row := range rows {
			if row.scenario == scenario {
				scenarioRows = append(scenarioRows, row)
			}
		}
		for start, sequence := 0, completedCounts[scenario]+1; start < len(scenarioRows); start, sequence = start+4, sequence+1 {
			end := start + 4
			if end > len(scenarioRows) {
				end = len(scenarioRows)
			}
			selected, rowIDs, variantRuns := scenarioRows[start:end], []any{}, 0
			for _, row := range selected {
				rowIDs = append(rowIDs, row.id)
				variantRuns += len(row.variantIDs)
			}
			tranches = append(tranches, map[string]any{"id": fmt.Sprintf("%s-%03d", scenario, sequence), "risk_rank": selected[0].riskRank, "scenario": scenario, "status": "planned", "row_ids": rowIDs, "pattern_rows": len(selected), "variant_runs": variantRuns, "commit_policy": "one-reviewed-tranche-with-non-regression-runtime-identity-and-oracle-validation"})
		}
	}
	return tranches
}

func buildCompletedClosureRows(report map[string]any, riskRank map[string]int) []any {
	type group struct {
		patternID string
		scenario  string
		records   []map[string]any
	}
	groups := map[string]*group{}
	for _, raw := range anySlice(report["tests"]) {
		record, _ := raw.(map[string]any)
		patternID, scenario := stringValue(record["pattern_id"]), stringValue(record["scenario"])
		key := patternID + "\x00" + scenario
		if groups[key] == nil {
			groups[key] = &group{patternID: patternID, scenario: scenario}
		}
		groups[key].records = append(groups[key].records, record)
	}
	ordered := []*group{}
	for _, item := range groups {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if riskRank[ordered[i].scenario] != riskRank[ordered[j].scenario] {
			return riskRank[ordered[i].scenario] < riskRank[ordered[j].scenario]
		}
		return ordered[i].patternID < ordered[j].patternID
	})
	result := []any{}
	for _, item := range ordered {
		oracleKinds, variants := []string{}, []string{}
		seenOracle := map[string]bool{}
		firstAttempt, streams := true, true
		for _, record := range item.records {
			oracle, _ := record["oracle"].(map[string]any)
			kind := stringValue(oracle["kind"])
			if kind == "" {
				kind = "missing"
			}
			if !seenOracle[kind] {
				oracleKinds, seenOracle[kind] = append(oracleKinds, kind), true
			}
			variants = append(variants, stringValue(record["variant_id"]))
			firstAttempt = firstAttempt && record["outcome"] == "expected" && int(numberValue(record["attempts"])) == 1 && record["final_status"] == "passed"
			trace, _ := record["trace"].(map[string]any)
			streams = streams && trace["action_stream"] == true && trace["network_stream"] == true && trace["resource_stream"] == true
		}
		sort.Strings(variants)
		result = append(result, map[string]any{"pattern_id": item.patternID, "scenario": item.scenario, "oracle_kinds": stringSliceToAny(oracleKinds), "variant_ids": stringSliceToAny(variants), "all_first_attempt_pass": firstAttempt, "all_trace_streams": streams})
	}
	return result
}

func sameCanonical(left, right any) bool {
	leftDigest, leftErr := digestCanonical(left)
	rightDigest, rightErr := digestCanonical(right)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func stringSliceToAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
