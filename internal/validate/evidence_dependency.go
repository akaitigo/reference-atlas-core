package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EvidenceDependencyResult struct {
	AtlasID        string
	Inputs         int
	Outputs        int
	ChangedInputs  int
	AffectedOutput int
	Runs           int
}

// AuditEvidenceDependencyGraph verifies that Evidence is tied to the inputs
// and actual executions which produced it. Re-pinning digests alone cannot
// make descendants of a changed input current.
func AuditEvidenceDependencyGraph(dir, relative string) (EvidenceDependencyResult, error) {
	if err := safeRepositoryRelativePath(relative); err != nil {
		return EvidenceDependencyResult{}, err
	}
	full := filepath.Join(dir, filepath.FromSlash(relative))
	if _, err := File(full); err != nil {
		return EvidenceDependencyResult{}, err
	}
	graph, err := readDocument(full)
	if err != nil {
		return EvidenceDependencyResult{}, err
	}
	if graph["status"] != "current" {
		return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency graphはstaleです。影響Evidenceを実再実行してください")
	}

	inputs := map[string]map[string]any{}
	changed := map[string]time.Time{}
	for _, raw := range anySlice(graph["inputs"]) {
		input, _ := raw.(map[string]any)
		id := stringValue(input["id"])
		if id == "" || inputs[id] != nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency input IDが空または重複しています: %s", id)
		}
		actual, err := aggregateMemberDigest(dir, anySlice(input["members"]))
		if err != nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency input %s: %w", id, err)
		}
		if actual != stringValue(input["current_digest"]) {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency inputのcurrent_digestが実体と一致しません: %s", id)
		}
		observed, err := time.Parse(time.RFC3339, stringValue(input["observed_at"]))
		if err != nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency inputのobserved_atが無効です: %s", id)
		}
		inputs[id] = input
		if input["baseline_digest"] != input["current_digest"] {
			changed[id] = observed
		}
	}

	outputs := map[string]map[string]any{}
	outputByPath := map[string]string{}
	for _, raw := range anySlice(graph["outputs"]) {
		output, _ := raw.(map[string]any)
		id, path := stringValue(output["id"]), filepath.ToSlash(stringValue(output["path"]))
		if id == "" || outputs[id] != nil || path == "" || outputByPath[path] != "" {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency output ID/pathが空または重複しています: id=%s path=%s", id, path)
		}
		if err := safeRepositoryRelativePath(path); err != nil {
			return EvidenceDependencyResult{}, err
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency output %s: %w", id, err)
		}
		if digestBytes(data) != stringValue(output["digest"]) {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency outputのdigestが実体と一致しません: %s", id)
		}
		if output["status"] != "current" {
			return EvidenceDependencyResult{}, fmt.Errorf("影響Evidenceがstaleのままです: %s", id)
		}
		outputs[id], outputByPath[path] = output, id
	}

	for id, output := range outputs {
		for _, raw := range anySlice(output["depends_on"]) {
			dep := stringValue(raw)
			if inputs[dep] == nil && outputs[dep] == nil {
				return EvidenceDependencyResult{}, fmt.Errorf("Evidence dependency edgeが未知nodeを参照しています: %s -> %s", id, dep)
			}
		}
	}
	if err := rejectDependencyCycles(outputs); err != nil {
		return EvidenceDependencyResult{}, err
	}

	required := map[string]bool{}
	for _, raw := range anySlice(graph["required_outputs"]) {
		required[filepath.ToSlash(stringValue(raw))] = true
	}
	discovered, err := discoverRequiredEvidenceOutputs(dir)
	if err != nil {
		return EvidenceDependencyResult{}, err
	}
	for path := range discovered {
		if !required[path] || outputByPath[path] == "" {
			return EvidenceDependencyResult{}, fmt.Errorf("再実行対象がEvidence dependency graphから欠落しています: %s", path)
		}
	}
	for path := range required {
		if outputByPath[path] == "" {
			return EvidenceDependencyResult{}, fmt.Errorf("required_outputに対応するoutputがありません: %s", path)
		}
	}

	runs := map[string]map[string]any{}
	for _, raw := range anySlice(graph["runs"]) {
		run, _ := raw.(map[string]any)
		id := stringValue(run["id"])
		if id == "" || runs[id] != nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerun IDが空または重複しています: %s", id)
		}
		started, err1 := time.Parse(time.RFC3339, stringValue(run["started_at"]))
		completed, err2 := time.Parse(time.RFC3339, stringValue(run["completed_at"]))
		if err1 != nil || err2 != nil || completed.Before(started) {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerun時刻が無効です: %s", id)
		}
		if run["execution_kind"] != "derived" {
			identity, _ := run["runtime_identity"].(map[string]any)
			if len(identity) == 0 {
				return EvidenceDependencyResult{}, fmt.Errorf("実Runtime/Platform rerunにruntime_identityがありません: %s", id)
			}
		}
		boundInputs := map[string]bool{}
		for _, bindingRaw := range anySlice(run["input_bindings"]) {
			binding, _ := bindingRaw.(map[string]any)
			inputID := stringValue(binding["input_id"])
			if inputs[inputID] == nil || boundInputs[inputID] {
				return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerun input bindingが未知または重複しています: run=%s input=%s", id, inputID)
			}
			boundInputs[inputID] = true
		}
		for _, outputRaw := range anySlice(run["output_ids"]) {
			outputID := stringValue(outputRaw)
			if outputs[outputID] == nil {
				return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerun outputが未知nodeを参照しています: run=%s output=%s", id, outputID)
			}
		}
		runs[id] = run
	}

	affected := 0
	for id, output := range outputs {
		ancestors, err := dependencyInputAncestors(id, outputs, inputs, map[string]bool{})
		if err != nil {
			return EvidenceDependencyResult{}, err
		}
		if len(ancestors) == 0 {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence outputがsource/harness/runtime/profileへ到達しません: %s", id)
		}
		run := runs[stringValue(output["run_id"])]
		if run == nil {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidenceは実行Proofへ結ばれていません: %s", id)
		}
		if !stringSliceContains(anySlice(run["output_ids"]), id) {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerun対象からoutputが漏れています: run=%s output=%s", stringValue(run["id"]), id)
		}
		bindings := map[string]string{}
		for _, raw := range anySlice(run["input_bindings"]) {
			binding, _ := raw.(map[string]any)
			bindings[stringValue(binding["input_id"])] = stringValue(binding["digest"])
		}
		for inputID := range ancestors {
			if bindings[inputID] != stringValue(inputs[inputID]["current_digest"]) {
				return EvidenceDependencyResult{}, fmt.Errorf("Evidence rerunが現在の入力digestへ結ばれていません: run=%s input=%s output=%s", stringValue(run["id"]), inputID, id)
			}
		}
		isAffected := false
		started, _ := time.Parse(time.RFC3339, stringValue(run["started_at"]))
		for inputID := range ancestors {
			if observed, ok := changed[inputID]; ok {
				isAffected = true
				if started.Before(observed) {
					return EvidenceDependencyResult{}, fmt.Errorf("digest書換えだけではClosureできません。入力変更後の実再実行が必要です: input=%s output=%s", inputID, id)
				}
			}
		}
		if isAffected {
			affected++
		}
	}

	kinds := map[string]bool{}
	for _, raw := range anySlice(graph["structures"]) {
		structure, _ := raw.(map[string]any)
		kind, path := stringValue(structure["kind"]), stringValue(structure["path"])
		if kinds[kind] {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence structure baselineが重複しています: %s", kind)
		}
		actual, err := evidenceStructureDigest(dir, kind, path)
		if err != nil {
			return EvidenceDependencyResult{}, err
		}
		if actual != stringValue(structure["baseline_digest"]) {
			return EvidenceDependencyResult{}, fmt.Errorf("既存Proof/Closure Planの構造が変化しています: %s", kind)
		}
		kinds[kind] = true
	}
	for _, kind := range []string{"scenario-proof-index", "scenario-closure-plan"} {
		if !kinds[kind] {
			return EvidenceDependencyResult{}, fmt.Errorf("Evidence structure baselineがありません: %s", kind)
		}
	}
	return EvidenceDependencyResult{AtlasID: stringValue(graph["atlas_id"]), Inputs: len(inputs), Outputs: len(outputs), ChangedInputs: len(changed), AffectedOutput: affected, Runs: len(runs)}, nil
}

func safeRepositoryRelativePath(path string) error {
	clean := filepath.Clean(filepath.FromSlash(path))
	if path == "" || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Evidence dependency pathがRepository外を参照しています: %s", path)
	}
	return nil
}

func aggregateMemberDigest(dir string, members []any) (string, error) {
	items := []any{}
	seen := map[string]bool{}
	for _, raw := range members {
		path := filepath.ToSlash(stringValue(raw))
		if seen[path] {
			return "", fmt.Errorf("input memberが重複しています: %s", path)
		}
		seen[path] = true
		if err := safeRepositoryRelativePath(path); err != nil {
			return "", err
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		items = append(items, map[string]any{"path": path, "digest": digestBytes(data)})
	}
	sort.Slice(items, func(i, j int) bool {
		return stringValue(items[i].(map[string]any)["path"]) < stringValue(items[j].(map[string]any)["path"])
	})
	return digestCanonical(items)
}

func rejectDependencyCycles(outputs map[string]map[string]any) error {
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("Evidence dependency graphにcycleがあります: %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, raw := range anySlice(outputs[id]["depends_on"]) {
			dep := stringValue(raw)
			if outputs[dep] != nil {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for id := range outputs {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func dependencyInputAncestors(id string, outputs, inputs map[string]map[string]any, visiting map[string]bool) (map[string]bool, error) {
	if visiting[id] {
		return nil, fmt.Errorf("Evidence dependency graphにcycleがあります: %s", id)
	}
	visiting[id] = true
	result := map[string]bool{}
	for _, raw := range anySlice(outputs[id]["depends_on"]) {
		dep := stringValue(raw)
		if inputs[dep] != nil {
			result[dep] = true
			continue
		}
		child, err := dependencyInputAncestors(dep, outputs, inputs, visiting)
		if err != nil {
			return nil, err
		}
		for inputID := range child {
			result[inputID] = true
		}
	}
	delete(visiting, id)
	return result, nil
}

func stringSliceContains(values []any, want string) bool {
	for _, raw := range values {
		if stringValue(raw) == want {
			return true
		}
	}
	return false
}

func discoverRequiredEvidenceOutputs(dir string) (map[string]bool, error) {
	result := map[string]bool{}
	addIfFile := func(path string) {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path))); err == nil && !info.IsDir() {
			result[path] = true
		}
	}
	for _, path := range []string{
		"artifacts/e2e-results.json", "artifacts/e2e-results.container.json", "artifacts/capture-manifest.json", "artifacts/capture-results.json",
		"artifacts/benchmark-results.json", "artifacts/compatibility-results.json", "artifacts/reference-system/results.json",
		"artifacts/pattern-scenarios/results.json", "evidence/scenarios/index.json", "evidence/scenarios/closure-plan.json",
	} {
		addIfFile(path)
	}
	for _, root := range []string{"artifacts", "evidence/core-v1", "evidence/reports"} {
		fullRoot := filepath.Join(dir, filepath.FromSlash(root))
		if _, err := os.Stat(fullRoot); os.IsNotExist(err) {
			continue
		}
		if err := filepath.WalkDir(fullRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			base := strings.ToLower(entry.Name())
			if root == "artifacts" && !strings.Contains(base, "results") && !strings.Contains(base, "manifest") {
				return nil
			}
			if !strings.HasSuffix(base, ".json") && !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml") {
				return nil
			}
			relative, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			result[filepath.ToSlash(relative)] = true
			return nil
		}); err != nil {
			return nil, err
		}
	}
	for _, pattern := range []string{"evidence/*.evidence.json", "evidence/*.evidence.yaml", "evidence/*.evidence.yml", "evals/*.definitive-skill-eval.json"} {
		matches, err := filepath.Glob(filepath.Join(dir, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			relative, err := filepath.Rel(dir, match)
			if err != nil {
				return nil, err
			}
			result[filepath.ToSlash(relative)] = true
		}
	}
	addIfFile("provenance.yaml")
	indexPath := filepath.Join(dir, "evidence", "scenarios", "index.json")
	if _, err := os.Stat(indexPath); err == nil {
		index, err := readDocument(indexPath)
		if err != nil {
			return nil, err
		}
		for _, raw := range anySlice(index["files"]) {
			item, _ := raw.(map[string]any)
			addIfFile(filepath.ToSlash(stringValue(item["path"])))
		}
	}
	definitivePath := filepath.Join(dir, "definitive.yaml")
	if _, err := os.Stat(definitivePath); err == nil {
		doc, err := readDocument(definitivePath)
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"scenario_proofs", "scenario_closure_plan", "evidence_durability", "skill_eval", "skill_router"} {
			addIfFile(filepath.ToSlash(stringValue(doc[key])))
		}
	}
	return result, nil
}

func evidenceStructureDigest(dir, kind, relative string) (string, error) {
	if err := safeRepositoryRelativePath(relative); err != nil {
		return "", err
	}
	doc, err := readDocument(filepath.Join(dir, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	switch kind {
	case "scenario-proof-index":
		files := []any{}
		for _, raw := range anySlice(doc["files"]) {
			item, _ := raw.(map[string]any)
			proof, err := readDocument(filepath.Join(dir, filepath.FromSlash(stringValue(item["path"]))))
			if err != nil {
				return "", err
			}
			bindings := []any{}
			for _, bindingRaw := range anySlice(proof["source_bindings"]) {
				binding, _ := bindingRaw.(map[string]any)
				bindings = append(bindings, map[string]any{"variant_id": binding["variant_id"], "path": binding["path"]})
			}
			files = append(files, map[string]any{"id": item["id"], "pattern_id": item["pattern_id"], "scenario": item["scenario"], "path": item["path"], "proof_id": proof["id"], "target_id": proof["target_id"], "target_set": proof["target_set"], "behavior_scope": proof["behavior_scope"], "source_bindings": bindings})
		}
		return digestCanonical(map[string]any{"id": doc["id"], "atlas_id": doc["atlas_id"], "denominator": doc["denominator"], "files": files})
	case "scenario-closure-plan":
		tranches := []any{}
		for _, field := range []string{"completed_tranches", "tranches"} {
			for _, raw := range anySlice(doc[field]) {
				item, _ := raw.(map[string]any)
				tranches = append(tranches, map[string]any{"id": item["id"], "risk_rank": item["risk_rank"], "scenario": item["scenario"], "row_ids": item["row_ids"], "pattern_rows": item["pattern_rows"], "variant_runs": item["variant_runs"], "commit_policy": item["commit_policy"]})
			}
		}
		rowIDs := []any{}
		for _, raw := range anySlice(doc["completed_tranches"]) {
			item, _ := raw.(map[string]any)
			rowIDs = append(rowIDs, anySlice(item["row_ids"])...)
		}
		for _, raw := range anySlice(doc["rows"]) {
			item, _ := raw.(map[string]any)
			rowIDs = append(rowIDs, item["id"])
		}
		return digestCanonical(map[string]any{"id": doc["id"], "scope": doc["scope"], "policy": doc["policy"], "baseline": doc["baseline"], "tranches": tranches, "ordered_row_ids": rowIDs})
	default:
		return "", fmt.Errorf("未知のEvidence structure kindです: %s", kind)
	}
}
