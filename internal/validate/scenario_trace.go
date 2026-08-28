package validate

import (
	"fmt"
	"path/filepath"
	"strings"
)

var scenarioTraceScenarios = []string{"normal", "boundary", "refusal", "failure", "recovery", "migration", "operations", "security", "performance", "compatibility"}

type ScenarioTraceResult struct {
	AtlasID                 string
	Patterns                int
	Rows                    int
	DedicatedArtifactRows   int
	PatternSpecificRows     int
	RuntimeIdentityRows     int
	PatternSpecificGaps     int
	IntegratedTraceRows     int
	DedicatedScenarioRows   int
	ScenarioClosureGaps     int
	AuthorityAtomicRows     int
	CompletionEligibleRows  int
	IntegratedScenarioTests int
	CompletionLimited       bool
}

type scenarioTraceAudit struct {
	result                ScenarioTraceResult
	rows                  map[string]map[string]any
	dedicatedScenarioRows map[string]bool
}

type patternScenarioReport struct {
	document          map[string]any
	environmentDigest string
	records           map[string]map[string]any
}

func AuditScenarioTrace(dir, relative string, requireComplete bool) (ScenarioTraceResult, error) {
	audit, err := auditScenarioTraceDocument(dir, relative)
	if err != nil {
		return ScenarioTraceResult{}, err
	}
	if requireComplete {
		if audit.result.CompletionLimited {
			return ScenarioTraceResult{}, fmt.Errorf("Scenario/TraceにCompletion limitまたは未Closure rowがあります")
		}
		if err := auditCompletionEligibleScenarioRows(dir, audit.rows, audit.dedicatedScenarioRows); err != nil {
			return ScenarioTraceResult{}, err
		}
	}
	return audit.result, nil
}

func auditScenarioTraceDocument(dir, relative string) (*scenarioTraceAudit, error) {
	indexPath := filepath.Join(dir, filepath.FromSlash(relative))
	if _, err := File(indexPath); err != nil {
		return nil, err
	}
	index, err := readDocument(indexPath)
	if err != nil {
		return nil, err
	}
	atlasID := stringValue(index["atlas_id"])
	verifiedSourceDocuments := []map[string]any{}
	sourceDigests := index["source_digests"].(map[string]any)
	for path, rawDigest := range sourceDigests {
		if err := verifyRelativeFileDigest(dir, path, stringValue(rawDigest), -1); err != nil {
			return nil, fmt.Errorf("Scenario Proof source digest: %w", err)
		}
		if document, err := readDocument(filepath.Join(dir, filepath.FromSlash(path))); err == nil {
			verifiedSourceDocuments = append(verifiedSourceDocuments, document)
		}
	}
	manifestPath := filepath.Join(dir, "integrations", "reference-system", "manifest.json")
	resultPath := filepath.Join(dir, "artifacts", "reference-system", "results.json")
	if _, err := File(manifestPath); err != nil {
		return nil, err
	}
	if _, err := File(resultPath); err != nil {
		return nil, err
	}
	manifest, err := readDocument(manifestPath)
	if err != nil {
		return nil, err
	}
	referenceResult, err := readDocument(resultPath)
	if err != nil {
		return nil, err
	}
	manifestPatterns, referenceTests, err := auditIntegratedReferenceSystem(dir, manifest, referenceResult)
	if err != nil {
		return nil, err
	}
	rows := map[string]map[string]any{}
	dedicatedScenarioRows := map[string]bool{}
	patternScenarioReports := map[string]*patternScenarioReport{}
	usedDedicatedScenarioArtifacts := map[string]string{}
	patterns := map[string]bool{}
	byScenario := map[string]map[string]int{}
	for _, scenario := range scenarioTraceScenarios {
		byScenario[scenario] = map[string]int{"rows": 0, "pattern_specific": 0, "runtime_identity": 0, "integrated_pattern_mapped": 0, "gaps": 0}
	}
	dedicated, patternSpecific, runtimeRows, captureRows, gaps, integratedRows, authorityRows, eligibleRows := 0, 0, 0, 0, 0, 0, 0, 0
	for _, raw := range anySlice(index["files"]) {
		record, _ := raw.(map[string]any)
		relativePath := stringValue(record["path"])
		if err := verifyRelativeFileDigest(dir, relativePath, stringValue(record["digest"]), -1); err != nil {
			return nil, fmt.Errorf("Scenario Proof row: %w", err)
		}
		full := filepath.Join(dir, filepath.FromSlash(relativePath))
		if _, err := File(full); err != nil {
			return nil, err
		}
		row, err := readDocument(full)
		if err != nil {
			return nil, err
		}
		if row["atlas_id"] != atlasID || row["id"] != record["id"] || row["pattern_id"] != record["pattern_id"] || row["scenario"] != record["scenario"] || row["status"] != record["status"] {
			return nil, fmt.Errorf("Scenario Proof indexとrow実体が一致しません: %s", relativePath)
		}
		scenario, patternID := stringValue(row["scenario"]), stringValue(row["pattern_id"])
		if !containsStringValue(scenarioTraceScenarios, scenario) {
			return nil, fmt.Errorf("Scenario Proof scenarioが不正です: %s", scenario)
		}
		subjectID := stringValue(row["behavior_id"])
		if subjectID == "" {
			subjectID = patternID
		}
		key := subjectID + ":" + scenario
		if rows[key] != nil {
			return nil, fmt.Errorf("Scenario Proof rowが重複しています: %s", key)
		}
		rows[key] = row
		patterns[subjectID] = true
		patternEvidence, _ := row["pattern_evidence"].(map[string]any)
		digestEvidence := append([]map[string]any{patternEvidence}, verifiedSourceDocuments...)
		for _, rawBinding := range anySlice(row["source_bindings"]) {
			binding, _ := rawBinding.(map[string]any)
			implementationBinding := map[string]any{"id": binding["variant_id"], "path": binding["path"], "digest": binding["digest"]}
			if err := verifyRouterImplementationBinding(dir, implementationBinding, digestEvidence); err != nil {
				return nil, fmt.Errorf("Scenario Proof %s source binding: %w", key, err)
			}
		}
		integrated, _ := row["integrated_reference"].(map[string]any)
		trace, _ := integrated["trace"].(map[string]any)
		screenshot, _ := integrated["screenshot"].(map[string]any)
		for _, artifact := range []map[string]any{trace, screenshot} {
			if err := verifyRelativeFileDigest(dir, stringValue(artifact["path"]), stringValue(artifact["digest"]), int64(numberValue(artifact["bytes"]))); err != nil {
				return nil, fmt.Errorf("Scenario Proof %s integrated artifact: %w", key, err)
			}
		}
		if trace["action_stream"] != true || trace["network_stream"] != true || trace["resource_stream"] != true {
			return nil, fmt.Errorf("Scenario Proof Traceにaction/network/resource streamがありません: %s", key)
		}
		test := referenceTests[scenario]
		testTrace, _ := test["trace"].(map[string]any)
		if test == nil || testTrace["path"] != trace["path"] || testTrace["digest"] != trace["digest"] {
			return nil, fmt.Errorf("Scenario Proof Traceが統合Runtime結果と一致しません: %s", key)
		}
		expectedMapped := manifestPatterns[scenario][patternID]
		if integrated["pattern_mapped"] != expectedMapped {
			return nil, fmt.Errorf("Scenario Proofの統合Pattern mappingがManifestと一致しません: %s", key)
		}
		closure, _ := row["closure"].(map[string]any)
		dedicatedScenario, artifacts, err := auditDedicatedScenarioRuntime(dir, row, sourceDigests, patternScenarioReports)
		if err != nil {
			return nil, fmt.Errorf("Scenario Proof %s dedicated runtime: %w", key, err)
		}
		if dedicatedScenario {
			dedicatedScenarioRows[key] = true
			for _, artifactPath := range artifacts {
				if owner := usedDedicatedScenarioArtifacts[artifactPath]; owner != "" && owner != key {
					return nil, fmt.Errorf("専用Scenario Runtime Artifactを複数rowへ流用できません: path=%s rows=%s,%s", artifactPath, owner, key)
				}
				usedDedicatedScenarioArtifacts[artifactPath] = key
			}
		}
		stats := byScenario[scenario]
		stats["rows"]++
		if closure["dedicated_row"] == true && closure["dedicated_artifact"] == true {
			dedicated++
		}
		if closure["pattern_specific_evidence"] == true {
			patternSpecific++
			stats["pattern_specific"]++
			if row["status"] == "bounded-capture-proof" {
				captureRows++
			}
		}
		if closure["real_runtime_identity"] == true {
			runtimeRows++
			stats["runtime_identity"]++
		}
		if closure["integrated_runtime_trace"] == true {
			integratedRows++
		}
		if integrated["pattern_mapped"] == true {
			stats["integrated_pattern_mapped"]++
		}
		if closure["pattern_specific_evidence"] != true {
			gaps++
			stats["gaps"]++
		}
		if closure["authority_atomic_behavior"] == true {
			authorityRows++
		}
		if closure["completion_eligible"] == true {
			eligibleRows++
		}
	}
	for subjectID := range patterns {
		for _, scenario := range scenarioTraceScenarios {
			if rows[subjectID+":"+scenario] == nil {
				return nil, fmt.Errorf("Scenario Proof denominatorに未分類rowがあります: %s:%s", subjectID, scenario)
			}
		}
	}
	if len(rows) != len(patterns)*len(scenarioTraceScenarios) {
		return nil, fmt.Errorf("Scenario Proof denominatorに余分なrowがあります")
	}
	summary, _ := index["summary"].(map[string]any)
	expectedSummary := map[string]int{"patterns": len(patterns), "scenarios": 10, "rows": len(rows), "dedicated_artifacts": dedicated, "pattern_specific_rows": patternSpecific, "pattern_specific_runtime_rows": runtimeRows, "pattern_specific_capture_rows": captureRows, "pattern_specific_gaps": gaps, "integrated_trace_rows": integratedRows, "authority_atomic_rows": authorityRows, "completion_eligible_rows": eligibleRows}
	for name, expected := range expectedSummary {
		if int(numberValue(summary[name])) != expected {
			return nil, fmt.Errorf("Scenario Proof summaryが実体と一致しません: %s expected=%d actual=%v", name, expected, summary[name])
		}
	}
	for scenario, expected := range byScenario {
		actual, _ := index["by_scenario"].(map[string]any)[scenario].(map[string]any)
		for name, count := range expected {
			if int(numberValue(actual[name])) != count {
				return nil, fmt.Errorf("Scenario Proof by_scenarioが実体と一致しません: %s:%s", scenario, name)
			}
		}
	}
	limited := gaps > 0 || len(dedicatedScenarioRows) != len(rows) || authorityRows != len(rows) || eligibleRows != len(rows) || len(anySlice(index["completion_limits"])) > 0 || index["status"] != "completion-eligible"
	if index["status"] == "completion-eligible" && (gaps > 0 || len(dedicatedScenarioRows) != len(rows) || authorityRows != len(rows) || eligibleRows != len(rows) || len(anySlice(index["completion_limits"])) > 0) {
		return nil, fmt.Errorf("Scenario Proof indexが未Closure実体をcompletion-eligibleと宣言しています")
	}
	return &scenarioTraceAudit{result: ScenarioTraceResult{AtlasID: atlasID, Patterns: len(patterns), Rows: len(rows), DedicatedArtifactRows: dedicated, PatternSpecificRows: patternSpecific, RuntimeIdentityRows: runtimeRows, PatternSpecificGaps: gaps, IntegratedTraceRows: integratedRows, DedicatedScenarioRows: len(dedicatedScenarioRows), ScenarioClosureGaps: len(rows) - len(dedicatedScenarioRows), AuthorityAtomicRows: authorityRows, CompletionEligibleRows: eligibleRows, IntegratedScenarioTests: len(referenceTests), CompletionLimited: limited}, rows: rows, dedicatedScenarioRows: dedicatedScenarioRows}, nil
}

func auditDedicatedScenarioRuntime(dir string, row map[string]any, sourceDigests map[string]any, cache map[string]*patternScenarioReport) (bool, []string, error) {
	patternEvidence, _ := row["pattern_evidence"].(map[string]any)
	reportPath := stringValue(patternEvidence["scenario_runtime_report"])
	records := anySlice(patternEvidence["scenario_runtime_records"])
	environment := patternEvidence["scenario_runtime_environment"]
	if reportPath == "" && len(records) == 0 && environment == nil {
		return false, nil, nil
	}
	if reportPath == "" || len(records) == 0 || environment == nil {
		return false, nil, fmt.Errorf("report、environment、recordsを部分的に設定できません")
	}
	if stringValue(sourceDigests[reportPath]) == "" {
		return false, nil, fmt.Errorf("専用Scenario Runtime reportがindex.source_digestsにありません: %s", reportPath)
	}
	report := cache[reportPath]
	if report == nil {
		document, err := readDocument(filepath.Join(dir, filepath.FromSlash(reportPath)))
		if err != nil {
			return false, nil, err
		}
		if document["status"] != "passed" || isNonRuntimeProfile(stringValue(document["profile"])) {
			return false, nil, fmt.Errorf("専用Scenario suiteがpassした実Runtime Profileではありません")
		}
		reportEnvironment, _ := document["environment"].(map[string]any)
		if int(numberValue(reportEnvironment["retries"])) != 0 || stringValue(reportEnvironment["trace_mode"]) != "on" {
			return false, nil, fmt.Errorf("専用Scenario suiteはretry=0かつtrace=onである必要があります")
		}
		tests := anySlice(document["tests"])
		counts, _ := document["counts"].(map[string]any)
		if int(numberValue(counts["total"])) != len(tests) || int(numberValue(counts["passed"])) != len(tests) || numberValue(counts["failed"])+numberValue(counts["flaky"])+numberValue(counts["skipped"]) != 0 {
			return false, nil, fmt.Errorf("専用Scenario suiteのcountsがfirst-attempt pass実体と一致しません")
		}
		report = &patternScenarioReport{document: document, records: map[string]map[string]any{}}
		report.environmentDigest, _ = digestCanonical(reportEnvironment)
		rowKeys, variantKeys := map[string]bool{}, map[string]bool{}
		for _, rawRecord := range tests {
			record, _ := rawRecord.(map[string]any)
			key := scenarioRuntimeRecordKey(record)
			if report.records[key] != nil {
				return false, nil, fmt.Errorf("専用Scenario suite recordが重複しています: %s", key)
			}
			if err := auditScenarioRuntimeRecordArtifacts(dir, record); err != nil {
				return false, nil, err
			}
			report.records[key] = record
			rowKeys[stringValue(record["pattern_id"])+"\x00"+stringValue(record["scenario"])] = true
			variantKeys[stringValue(record["pattern_id"])+"\x00"+stringValue(record["variant_id"])] = true
		}
		if int(numberValue(counts["rows"])) != len(rowKeys) || int(numberValue(counts["variants"])) != len(variantKeys) {
			return false, nil, fmt.Errorf("専用Scenario suiteのrow/variant countsが実体と一致しません")
		}
		cache[reportPath] = report
	}
	environmentDigest, _ := digestCanonical(environment)
	if environmentDigest != report.environmentDigest {
		return false, nil, fmt.Errorf("rowのScenario Runtime environmentがreportと一致しません")
	}
	bindings := map[string]string{}
	for _, rawBinding := range anySlice(row["source_bindings"]) {
		binding, _ := rawBinding.(map[string]any)
		bindings[stringValue(binding["variant_id"])] = stringValue(binding["digest"])
	}
	seen, artifacts := map[string]bool{}, []string{}
	patternID, scenario := stringValue(row["pattern_id"]), stringValue(row["scenario"])
	integrated, _ := row["integrated_reference"].(map[string]any)
	integratedTrace, _ := integrated["trace"].(map[string]any)
	for _, rawRecord := range records {
		record, _ := rawRecord.(map[string]any)
		variantID := stringValue(record["variant_id"])
		if stringValue(record["pattern_id"]) != patternID || stringValue(record["scenario"]) != scenario || bindings[variantID] == "" || stringValue(record["source_digest"]) != bindings[variantID] || seen[variantID] {
			return false, nil, fmt.Errorf("exact Pattern+Scenario+全Variant bindingではありません: pattern=%s scenario=%s variant=%s", patternID, scenario, variantID)
		}
		seen[variantID] = true
		reportRecord := report.records[scenarioRuntimeRecordKey(record)]
		rowDigest, _ := digestCanonical(record)
		reportDigest, _ := digestCanonical(reportRecord)
		if reportRecord == nil || rowDigest != reportDigest {
			return false, nil, fmt.Errorf("row recordが固定report実体と一致しません: %s", scenarioRuntimeRecordKey(record))
		}
		if record["outcome"] != "expected" || int(numberValue(record["attempts"])) != 1 || record["final_status"] != "passed" || record["error"] != nil {
			return false, nil, fmt.Errorf("専用Scenario recordがretryなしのpassではありません: %s", scenarioRuntimeRecordKey(record))
		}
		oracle, _ := record["oracle"].(map[string]any)
		if len(oracle) == 0 || (oracle["scenario"] != nil && oracle["scenario"] != scenario) {
			return false, nil, fmt.Errorf("専用Scenario recordにScenario固有Oracleがありません: %s", scenarioRuntimeRecordKey(record))
		}
		trace, _ := record["trace"].(map[string]any)
		if trace["path"] == integratedTrace["path"] || trace["digest"] == integratedTrace["digest"] {
			return false, nil, fmt.Errorf("統合Traceを専用Scenario Runtime Proofへ流用できません")
		}
		artifacts = append(artifacts, stringValue(trace["path"]))
	}
	if len(seen) != len(bindings) {
		return false, nil, fmt.Errorf("専用Scenario suiteが全Variantを実行していません: expected=%d actual=%d", len(bindings), len(seen))
	}
	return true, artifacts, nil
}

func scenarioRuntimeRecordKey(record map[string]any) string {
	return stringValue(record["pattern_id"]) + "\x00" + stringValue(record["scenario"]) + "\x00" + stringValue(record["variant_id"])
}

func auditScenarioRuntimeRecordArtifacts(dir string, record map[string]any) error {
	trace, _ := record["trace"].(map[string]any)
	screenshot, _ := record["screenshot"].(map[string]any)
	for _, artifact := range []map[string]any{trace, screenshot} {
		if err := verifyRelativeFileDigest(dir, stringValue(artifact["path"]), stringValue(artifact["digest"]), int64(numberValue(artifact["bytes"]))); err != nil {
			return err
		}
	}
	if trace["action_stream"] != true || trace["network_stream"] != true || trace["resource_stream"] != true {
		return fmt.Errorf("専用Scenario Traceにaction/network/resource streamがありません")
	}
	return nil
}

func isNonRuntimeProfile(profile string) bool {
	lower := strings.ToLower(profile)
	return profile == "" || strings.Contains(lower, "static") || strings.Contains(lower, "mock") || strings.Contains(lower, "fixture") || strings.Contains(lower, "compile")
}

func auditIntegratedReferenceSystem(dir string, manifest, result map[string]any) (map[string]map[string]bool, map[string]map[string]any, error) {
	patterns := map[string]map[string]bool{}
	for _, raw := range anySlice(manifest["scenarios"]) {
		item, _ := raw.(map[string]any)
		scenario := stringValue(item["id"])
		if patterns[scenario] != nil || !containsStringValue(scenarioTraceScenarios, scenario) {
			return nil, nil, fmt.Errorf("統合Reference System scenarioが重複または不正です: %s", scenario)
		}
		patterns[scenario] = stringSet(anySlice(item["patterns"]))
	}
	if len(patterns) != 10 {
		return nil, nil, fmt.Errorf("統合Reference Systemに10 Scenarioがありません")
	}
	counts, _ := result["counts"].(map[string]any)
	if result["status"] != "passed" || int(numberValue(counts["passed"])) != 10 || numberValue(counts["failed"])+numberValue(counts["flaky"])+numberValue(counts["skipped"]) != 0 {
		return nil, nil, fmt.Errorf("統合Reference Systemの10 Scenario Runtimeがpassではありません")
	}
	tests := map[string]map[string]any{}
	for _, raw := range anySlice(result["tests"]) {
		test, _ := raw.(map[string]any)
		scenario := stringValue(test["scenario"])
		if tests[scenario] != nil || patterns[scenario] == nil || test["final_status"] != "passed" {
			return nil, nil, fmt.Errorf("統合Reference System testが重複、不明、またはpass以外です: %s", scenario)
		}
		trace, _ := test["trace"].(map[string]any)
		screenshot, _ := test["screenshot"].(map[string]any)
		for _, artifact := range []map[string]any{trace, screenshot} {
			if err := verifyRelativeFileDigest(dir, stringValue(artifact["path"]), stringValue(artifact["digest"]), int64(numberValue(artifact["bytes"]))); err != nil {
				return nil, nil, err
			}
		}
		if trace["action_stream"] != true || trace["network_stream"] != true || trace["resource_stream"] != true {
			return nil, nil, fmt.Errorf("統合Reference System Trace streamが不足しています: %s", scenario)
		}
		tests[scenario] = test
	}
	if len(tests) != 10 {
		return nil, nil, fmt.Errorf("統合Reference System Runtime結果に10 Scenarioがありません")
	}
	return patterns, tests, nil
}

func auditCompletionEligibleScenarioRows(dir string, rows map[string]map[string]any, dedicatedScenarioRows map[string]bool) error {
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return err
	}
	inventory, err := readDocument(filepath.Join(dir, "surface.inventory.yaml"))
	if err != nil {
		return err
	}
	behaviors := map[string]map[string]any{}
	artifactDigests := map[string]string{}
	for _, raw := range anySlice(inventory["authority_artifacts"]) {
		item, _ := raw.(map[string]any)
		artifactDigests[stringValue(item["id"])] = stringValue(item["digest"])
	}
	for _, raw := range anySlice(inventory["items"]) {
		item, _ := raw.(map[string]any)
		behaviors[stringValue(item["behavior_id"])] = item
	}
	usedArtifacts := map[string]bool{}
	for behaviorID, item := range behaviors {
		for _, scenario := range scenarioTraceScenarios {
			key := behaviorID + ":" + scenario
			row := rows[key]
			if row == nil {
				return fmt.Errorf("Authority Atomic BehaviorのScenario Proof rowがありません: %s", key)
			}
			if !dedicatedScenarioRows[key] {
				return fmt.Errorf("Scenario Proof rowにexact Pattern+Scenario+全Variantの専用Runtime suiteがありません: %s", key)
			}
			closure, _ := row["closure"].(map[string]any)
			if row["behavior_scope"] != "authority-derived-atomic-behavior" || row["status"] != "completion-eligible-runtime-proof" || closure["dedicated_row"] != true || closure["dedicated_artifact"] != true || closure["pattern_specific_evidence"] != true || closure["real_runtime_identity"] != true || closure["authority_atomic_behavior"] != true || closure["completion_eligible"] != true || len(anySlice(row["gaps"])) != 0 {
				return fmt.Errorf("Scenario Proof rowにGapまたは統合Trace流用があります: %s", key)
			}
			authority, _ := row["authority_binding"].(map[string]any)
			if authority["authority_artifact_id"] != item["authority_artifact_id"] || authority["authority_surface_id"] != item["authority_surface_id"] || authority["atomic_behavior_id"] != item["behavior_id"] || stringValue(authority["source_digest"]) != artifactDigests[stringValue(item["authority_artifact_id"])] {
				return fmt.Errorf("Scenario Proof rowのAtomic Authority bindingがInventoryと一致しません: %s", key)
			}
			runtime, _ := row["runtime_identity"].(map[string]any)
			sourceDigest, _ := digestCanonical(row["source_bindings"])
			environmentDigest, _ := digestCanonical(runtime["environment"])
			if runtime["source_digest"] != sourceDigest || runtime["environment_digest"] != environmentDigest {
				return fmt.Errorf("Scenario Proof rowのsource/environment identity digestが一致しません: %s", key)
			}
			if err := verifyRelativeFileDigest(dir, stringValue(runtime["harness_path"]), stringValue(runtime["harness_digest"]), -1); err != nil {
				return fmt.Errorf("Scenario Proof harness %s: %w", key, err)
			}
			artifactPath := stringValue(runtime["artifact_path"])
			if usedArtifacts[artifactPath] {
				return fmt.Errorf("統合TraceまたはRuntime Artifactを複数Behavior/Scenarioへ流用できません: %s", artifactPath)
			}
			usedArtifacts[artifactPath] = true
			if err := verifyRelativeFileDigest(dir, artifactPath, stringValue(runtime["artifact_digest"]), -1); err != nil {
				return fmt.Errorf("Scenario Proof runtime artifact %s: %w", key, err)
			}
			integrated, _ := row["integrated_reference"].(map[string]any)
			trace, _ := integrated["trace"].(map[string]any)
			if artifactPath == stringValue(trace["path"]) {
				return fmt.Errorf("統合Reference System Traceを個別Behavior Proofへ流用できません: %s", key)
			}
			evidenceID := stringValue(runtime["evidence_id"])
			evidence := ctx.evidence[evidenceID]
			evidenceArtifact, _ := evidence["artifact"].(map[string]any)
			environment, _ := evidence["environment"].(map[string]any)
			evidenceEnvironmentDigest, _ := digestCanonical(environment)
			if evidence == nil || evidence["verdict"] != "pass" || evidenceExecutionMode(evidence) != stringValue(runtime["execution_mode"]) || stringValue(environment["profile"]) != stringValue(runtime["profile"]) || evidenceEnvironmentDigest != environmentDigest || stringValue(evidence["runtime_identity"]) == "" || stringValue(evidence["harness_digest"]) != stringValue(runtime["harness_digest"]) || stringValue(evidenceArtifact["uri"]) != artifactPath || stringValue(evidenceArtifact["digest"]) != stringValue(runtime["artifact_digest"]) {
				return fmt.Errorf("Scenario Proof rowのRuntime Evidence実体が一致しません: %s evidence=%s", key, evidenceID)
			}
		}
	}
	if len(rows) != len(behaviors)*10 {
		return fmt.Errorf("Scenario Proof row denominatorがAuthority Atomic Behavior × 10ではありません")
	}
	return nil
}
