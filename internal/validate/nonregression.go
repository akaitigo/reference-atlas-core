package validate

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var nonRegressionCollections = []string{"tests_labs", "target_sets", "targets", "claims", "proof_obligations", "evidence", "sources", "authority_extraction", "skill_eval_cases", "required_profiles", "matrix_rows", "depth_parity_rows", "ci_jobs"}

type NonRegressionResult struct {
	AtlasID       string
	BaselineItems int
	CurrentItems  int
	Replacements  int
}

func GenerateNonRegressionBaseline(dir, output, commit, capturedAt string) (string, error) {
	if err := validateCommit(commit); err != nil {
		return "", err
	}
	if capturedAt == "" {
		capturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, capturedAt); err != nil {
		return "", fmt.Errorf("captured-atがRFC3339ではありません: %w", err)
	}
	baseline, err := captureNonRegressionState(dir, commit, capturedAt)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return "", err
	}
	if _, err := File(output); err != nil {
		return "", err
	}
	return output, nil
}

func AuditNonRegression(dir string) (NonRegressionResult, error) {
	policyPath := filepath.Join(dir, "non-regression.yaml")
	if _, err := File(policyPath); err != nil {
		return NonRegressionResult{}, err
	}
	policy, err := readDocument(policyPath)
	if err != nil {
		return NonRegressionResult{}, err
	}
	baselineRef, _ := policy["baseline"].(map[string]any)
	baselinePath := stringValue(baselineRef["path"])
	if err := verifyRelativeFileDigest(dir, baselinePath, stringValue(baselineRef["digest"]), -1); err != nil {
		return NonRegressionResult{}, fmt.Errorf("Non-regression baseline: %w", err)
	}
	fullBaselinePath := filepath.Join(dir, filepath.FromSlash(baselinePath))
	if _, err := File(fullBaselinePath); err != nil {
		return NonRegressionResult{}, err
	}
	baseline, err := readJSONDocument(fullBaselinePath)
	if err != nil {
		return NonRegressionResult{}, err
	}
	if err := verifyBaselineAgainstGit(dir, baseline); err != nil {
		return NonRegressionResult{}, err
	}
	if err := verifyBaselineAnchor(dir, baselineRef); err != nil {
		return NonRegressionResult{}, err
	}
	current, err := captureNonRegressionState(dir, stringValue(baseline["baseline_commit"]), stringValue(baseline["captured_at"]))
	if err != nil {
		return NonRegressionResult{}, err
	}
	if baseline["atlas_id"] != current["atlas_id"] || policy["atlas_id"] != current["atlas_id"] {
		return NonRegressionResult{}, fmt.Errorf("Non-regression Atlas IDが一致しません")
	}
	baselineCollections, _ := baseline["collections"].(map[string]any)
	currentCollections, _ := current["collections"].(map[string]any)
	replacements := indexReplacements(anySlice(policy["replacements"]))
	replacementOwners := map[string]string{}
	for key, mapping := range replacements {
		for _, raw := range anySlice(mapping["new_ids"]) {
			newKey := stringValue(mapping["collection"]) + ":" + stringValue(raw)
			if owner := replacementOwners[newKey]; owner != "" && owner != key {
				return NonRegressionResult{}, fmt.Errorf("複数Baselineを粗いReplacementへ集約できません: %s (%s, %s)", newKey, owner, key)
			}
			replacementOwners[newKey] = key
		}
	}
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return NonRegressionResult{}, err
	}
	baselineCount, currentCount := 0, 0
	for _, collection := range nonRegressionCollections {
		oldItems := indexBaselineItems(anySlice(baselineCollections[collection]))
		newItems := indexBaselineItems(anySlice(currentCollections[collection]))
		baselineCount += len(oldItems)
		currentCount += len(newItems)
		for id, oldItem := range oldItems {
			newItem, exists := newItems[id]
			if collection == "evidence" && oldItem["verdict"] != "pass" && !exists {
				return NonRegressionResult{}, fmt.Errorf("失敗Evidenceを削除・上書きできません: %s", id)
			}
			if exists {
				if err := rejectDirectWeakening(collection, oldItem, newItem); err != nil {
					return NonRegressionResult{}, err
				}
				if oldItem["fingerprint"] == newItem["fingerprint"] {
					continue
				}
				if collection == "authority_extraction" && authorityExtractionStrengthens(oldItem, newItem) {
					continue
				}
			}
			mapping := replacements[collection+":"+id]
			if mapping == nil {
				if !exists {
					return NonRegressionResult{}, fmt.Errorf("Baseline %sを削除・Scope外移動できません: %s", collection, id)
				}
				return NonRegressionResult{}, fmt.Errorf("Baseline %sのAssertion・閾値・予算・Platformを変更できません: %s", collection, id)
			}
			if err := validateReplacement(ctx, collection, oldItem, mapping, newItems); err != nil {
				return NonRegressionResult{}, err
			}
		}
		for _, item := range newItems {
			if collection == "tests_labs" && item["enabled"] != true {
				return NonRegressionResult{}, fmt.Errorf("Test/Labをskip・xfail・disabled化できません: %s", item["id"])
			}
		}
	}
	oldExclusions := stringSet(anySlice(baseline["scope_exclusions"]))
	if baseline["scope_statement_fingerprint"] != current["scope_statement_fingerprint"] {
		return NonRegressionResult{}, fmt.Errorf("Baseline Scope statementを縮小・置換できません")
	}
	for _, raw := range anySlice(current["scope_exclusions"]) {
		exclusion := stringValue(raw)
		if !oldExclusions[exclusion] {
			return NonRegressionResult{}, fmt.Errorf("Baseline能力を新しいScope exclusionへ移動できません: %s", exclusion)
		}
	}
	if numberValue(current["minimum_skill_pass_rate"]) < numberValue(baseline["minimum_skill_pass_rate"]) {
		return NonRegressionResult{}, fmt.Errorf("Skill Eval minimum_pass_rateを緩和できません: baseline=%v current=%v", baseline["minimum_skill_pass_rate"], current["minimum_skill_pass_rate"])
	}
	return NonRegressionResult{AtlasID: stringValue(current["atlas_id"]), BaselineItems: baselineCount, CurrentItems: currentCount, Replacements: len(replacements)}, nil
}

func verifyBaselineAnchor(dir string, current map[string]any) error {
	if err := exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", "non-regression.yaml").Run(); err != nil {
		return nil
	}
	output, err := exec.Command("git", "-C", dir, "log", "--diff-filter=A", "--format=%H", "--", "non-regression.yaml").Output()
	if err != nil {
		return fmt.Errorf("Non-regression baseline anchor履歴を取得できません: %w", err)
	}
	commits := strings.Fields(string(output))
	if len(commits) == 0 {
		return fmt.Errorf("Non-regression baseline anchorの追加Commitがありません")
	}
	anchorCommit := commits[len(commits)-1]
	data, err := exec.Command("git", "-C", dir, "show", anchorCommit+":non-regression.yaml").Output()
	if err != nil {
		return fmt.Errorf("Non-regression baseline anchorを読めません: %w", err)
	}
	var anchorPolicy map[string]any
	if err := yaml.Unmarshal(data, &anchorPolicy); err != nil {
		return err
	}
	anchor, _ := anchorPolicy["baseline"].(map[string]any)
	anchorDigest, _ := digestCanonical(anchor)
	currentDigest, _ := digestCanonical(current)
	if anchorDigest != currentDigest {
		return fmt.Errorf("ユーザー明示承認なしに初回公開Baseline anchorを変更できません: anchor_commit=%s", anchorCommit)
	}
	return nil
}

func verifyBaselineAgainstGit(dir string, baseline map[string]any) error {
	if err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return nil
	}
	commit := stringValue(baseline["baseline_commit"])
	archive, err := exec.Command("git", "-C", dir, "archive", "--format=tar", commit).Output()
	if err != nil {
		return fmt.Errorf("baseline_commitをGit履歴から取得できません: %s: %w", commit, err)
	}
	temporary, err := os.MkdirTemp("", "atlas-non-regression-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nextErr
		}
		clean := filepath.Clean(header.Name)
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Git archiveに不正Pathがあります: %s", header.Name)
		}
		path := filepath.Join(temporary, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if openErr != nil {
				return openErr
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	expected, err := captureNonRegressionState(temporary, commit, stringValue(baseline["captured_at"]))
	if err != nil {
		return err
	}
	expectedDigest, err := digestCanonical(expected)
	if err != nil {
		return err
	}
	actualDigest, err := digestCanonical(baseline)
	if err != nil {
		return err
	}
	if expectedDigest != actualDigest {
		return fmt.Errorf("Non-regression baselineがbaseline_commitの公開treeと一致しません: %s", commit)
	}
	return nil
}

func captureNonRegressionState(dir, commit, capturedAt string) (map[string]any, error) {
	atlas, err := readDocument(filepath.Join(dir, "atlas.yaml"))
	if err != nil {
		return nil, err
	}
	coverage, err := readDocument(filepath.Join(dir, "coverage.yaml"))
	if err != nil {
		return nil, err
	}
	sources, err := readDocument(filepath.Join(dir, "sources.lock.yaml"))
	if err != nil {
		return nil, err
	}
	skill, err := readDocument(filepath.Join(dir, "skill.package.yaml"))
	if err != nil {
		return nil, err
	}
	collections := map[string]any{}
	tests, err := captureTestsAndLabs(dir)
	if err != nil {
		return nil, err
	}
	collections["tests_labs"] = tests
	collections["target_sets"] = snapshotEntities(anySlice(coverage["target_sets"]), func(item map[string]any) string { return stringValue(item["id"]) }, nil)
	collections["targets"] = snapshotEntities(anySlice(coverage["targets"]), func(item map[string]any) string { return stringValue(item["id"]) }, func(item map[string]any, snapshot map[string]any) {
		snapshot["requirement"], snapshot["state"] = item["requirement"], item["state"]
		snapshot["claim_count"], snapshot["evidence_count"] = len(anySlice(item["claim_ids"])), len(anySlice(item["evidence_ids"]))
	})
	claims, err := collectEntities(filepath.Join(dir, "claims"), ".claim.")
	if err != nil {
		return nil, err
	}
	claimItems, proofItems := []any{}, []any{}
	for _, id := range sortedEntityIDs(claims) {
		claim := claims[id]
		claimItems = append(claimItems, baselineItem(id, claim, nil))
		for _, rawProof := range anySlice(claim["proof_obligations"]) {
			proof, _ := rawProof.(map[string]any)
			proofItems = append(proofItems, baselineItem(stringValue(proof["id"]), map[string]any{"claim_id": id, "proof": proof}, nil))
		}
	}
	collections["claims"], collections["proof_obligations"] = claimItems, proofItems
	evidence, err := collectEntities(filepath.Join(dir, "evidence"), ".evidence.")
	if err != nil {
		return nil, err
	}
	evidenceItems := []any{}
	for _, id := range sortedEntityIDs(evidence) {
		entity := evidence[id]
		environment, _ := entity["environment"].(map[string]any)
		mode := evidenceExecutionMode(entity)
		semantic := map[string]any{"id": id, "claim_ids": entity["claim_ids"], "kind": entity["kind"], "producer": entity["producer"], "command": entity["command"], "profile": environment["profile"], "verdict": entity["verdict"], "execution_mode": mode}
		if entity["verdict"] != "pass" {
			semantic = entity
		}
		evidenceItems = append(evidenceItems, baselineItem(id, semantic, func(snapshot map[string]any) {
			snapshot["verdict"], snapshot["execution_mode"], snapshot["profile"] = entity["verdict"], mode, environment["profile"]
		}))
	}
	collections["evidence"] = evidenceItems
	collections["sources"] = snapshotEntities(anySlice(sources["sources"]), func(item map[string]any) string { return stringValue(item["id"]) }, nil)
	authorityExtraction, err := captureAuthorityExtractionState(dir)
	if err != nil {
		return nil, err
	}
	if len(authorityExtraction) > 0 {
		collections["authority_extraction"] = authorityExtraction
	}
	evalItems, minimum, err := captureSkillEval(dir, skill)
	if err != nil {
		return nil, err
	}
	collections["skill_eval_cases"] = evalItems
	completion, _ := atlas["completion"].(map[string]any)
	profileItems := []any{}
	for _, raw := range anySlice(completion["required_profiles"]) {
		profile := stringValue(raw)
		profileItems = append(profileItems, baselineItem(profile, map[string]any{"profile": profile}, nil))
	}
	collections["required_profiles"] = profileItems
	collections["matrix_rows"] = captureMatrix(dir)
	collections["depth_parity_rows"] = captureRowsIfPresent(filepath.Join(dir, "depth.parity.yaml"), "rows", func(item map[string]any) string {
		return stringValue(item["behavior_id"]) + ":" + stringValue(item["variant_id"]) + ":" + stringValue(item["axis"])
	})
	ciItems, err := captureCI(dir)
	if err != nil {
		return nil, err
	}
	collections["ci_jobs"] = ciItems
	for _, name := range nonRegressionCollections {
		sortBaselineItems(anySlice(collections[name]))
	}
	scope, _ := atlas["scope"].(map[string]any)
	scopeFingerprint, _ := digestCanonical(map[string]any{"statement": scope["statement"]})
	return map[string]any{
		"schema_version": 2, "atlas_id": atlas["id"], "baseline_commit": commit, "captured_at": capturedAt,
		"scope_statement_fingerprint": scopeFingerprint, "scope_exclusions": anySlice(scope["exclusions"]), "minimum_skill_pass_rate": minimum, "collections": collections,
	}, nil
}

func captureAuthorityExtractionState(dir string) ([]any, error) {
	snapshotPath := filepath.Join(dir, "authority", "extraction.snapshot.json")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return []any{}, nil
	} else if err != nil {
		return nil, err
	}
	if _, err := File(snapshotPath); err != nil {
		return nil, err
	}
	snapshot, err := readDocument(snapshotPath)
	if err != nil {
		return nil, err
	}
	stableContracts := map[string]any{}
	locatorRanks := map[string]any{}
	fetchRanks := map[string]any{}
	locatorStates := map[string]any{}
	fetchStates := map[string]any{}
	drafts := map[string]any{}
	for _, raw := range anySlice(snapshot["sources"]) {
		index, _ := raw.(map[string]any)
		relative := stringValue(index["path"])
		full := filepath.Join(dir, filepath.FromSlash(relative))
		if _, err := File(full); err != nil {
			return nil, err
		}
		draft, err := readDocument(full)
		if err != nil {
			return nil, err
		}
		sourceID := stringValue(index["id"])
		drafts[sourceID] = draft
		fetch, _ := draft["fetch"].(map[string]any)
		fetchRanks[sourceID] = authorityFetchRank(stringValue(fetch["status"]))
		fetchStates[sourceID], _ = digestCanonical(fetch)
		stableContracts["source:"+sourceID], _ = digestCanonical(map[string]any{
			"source_id": draft["source_id"], "source_url": draft["source_url"], "locked_source_digest": draft["locked_source_digest"],
		})
		for _, rawCandidate := range anySlice(draft["candidate_surfaces"]) {
			candidate, _ := rawCandidate.(map[string]any)
			edgeID := stringValue(candidate["edge_id"])
			stableContracts["edge:"+edgeID], _ = digestCanonical(map[string]any{
				"edge_id": candidate["edge_id"], "source_id": candidate["source_id"], "reference_url": candidate["reference_url"],
				"locator": candidate["locator"], "pattern_id": candidate["pattern_id"], "pattern_kind": candidate["pattern_kind"],
				"candidate_behavior_id": candidate["candidate_behavior_id"], "capability_id": candidate["capability_id"],
				"target_id": candidate["target_id"], "claim_id": candidate["claim_id"], "variant_ids": candidate["variant_ids"],
				"surface_ids": candidate["surface_ids"], "classification_basis": candidate["classification_basis"],
				"domain_reference_metadata_digest": candidate["domain_reference_metadata_digest"],
			})
			locatorRanks[edgeID] = authorityLocatorRank(stringValue(candidate["locator_status"]))
			locatorStates[edgeID], _ = digestCanonical(map[string]any{
				"locator_status": candidate["locator_status"], "context_digest": candidate["context_digest"],
				"context_start": candidate["context_start"], "context_end": candidate["context_end"],
				"context_unit": candidate["context_unit"], "heading_digest": candidate["heading_digest"],
			})
		}
	}
	item := baselineItem("snapshot", map[string]any{"snapshot": snapshot, "drafts": drafts}, func(item map[string]any) {
		item["path"] = "authority/extraction.snapshot.json"
		item["stable_contracts"] = stableContracts
		item["fetch_ranks"] = fetchRanks
		item["locator_ranks"] = locatorRanks
		item["fetch_state_fingerprints"] = fetchStates
		item["locator_state_fingerprints"] = locatorStates
		summary, _ := snapshot["summary"].(map[string]any)
		for _, key := range []string{"locked_sources", "fetched_digest_matched", "fetched_digest_stale", "fetch_failed", "candidate_surfaces", "fragments_not_found", "locator_evaluations_deferred", "reference_edges_classified", "unclassified_reference_edges", "human_reviewed_surfaces", "core_v2_eligible_surfaces"} {
			item[key] = summary[key]
		}
		item["authority_text_surfaces_exhaustive"] = summary["authority_text_surfaces_exhaustive"]
		item["status_rank"] = authorityExtractionStatusRank(stringValue(snapshot["status"]))
	})
	return []any{item}, nil
}

func authorityExtractionStrengthens(oldItem, newItem map[string]any) bool {
	for _, key := range []string{"locked_sources", "candidate_surfaces", "reference_edges_classified", "fetched_digest_matched", "human_reviewed_surfaces", "core_v2_eligible_surfaces", "status_rank"} {
		if numberValue(newItem[key]) < numberValue(oldItem[key]) {
			return false
		}
	}
	for _, key := range []string{"fetched_digest_stale", "fetch_failed", "fragments_not_found", "locator_evaluations_deferred", "unclassified_reference_edges"} {
		if numberValue(newItem[key]) > numberValue(oldItem[key]) {
			return false
		}
	}
	if oldItem["authority_text_surfaces_exhaustive"] == true && newItem["authority_text_surfaces_exhaustive"] != true {
		return false
	}
	if !mapSubsetEqual(oldItem["stable_contracts"], newItem["stable_contracts"]) {
		return false
	}
	if !rankedStateNonRegressive(oldItem["fetch_ranks"], newItem["fetch_ranks"], oldItem["fetch_state_fingerprints"], newItem["fetch_state_fingerprints"]) ||
		!rankedStateNonRegressive(oldItem["locator_ranks"], newItem["locator_ranks"], oldItem["locator_state_fingerprints"], newItem["locator_state_fingerprints"]) {
		return false
	}
	return true
}

func mapSubsetEqual(oldRaw, newRaw any) bool {
	oldValues, _ := oldRaw.(map[string]any)
	newValues, _ := newRaw.(map[string]any)
	for key, oldValue := range oldValues {
		if newValues[key] != oldValue {
			return false
		}
	}
	return true
}

func rankedStateNonRegressive(oldRankRaw, newRankRaw, oldStateRaw, newStateRaw any) bool {
	oldRanks, _ := oldRankRaw.(map[string]any)
	newRanks, _ := newRankRaw.(map[string]any)
	oldStates, _ := oldStateRaw.(map[string]any)
	newStates, _ := newStateRaw.(map[string]any)
	for key, oldValue := range oldRanks {
		oldRank, newRank := numberValue(oldValue), numberValue(newRanks[key])
		if newRank < oldRank {
			return false
		}
		if newRank == oldRank && newStates[key] != oldStates[key] {
			return false
		}
	}
	return true
}

func authorityFetchRank(status string) int {
	switch status {
	case "matched":
		return 2
	case "stale":
		return 1
	default:
		return 0
	}
}

func authorityLocatorRank(status string) int {
	switch status {
	case "root-document", "fragment-found":
		return 2
	case "fragment-not-found":
		return 1
	default:
		return 0
	}
}

func authorityExtractionStatusRank(status string) int {
	switch status {
	case "eligible-for-core-v2":
		return 2
	case "incomplete-human-review-required":
		return 1
	default:
		return 0
	}
}

func captureTestsAndLabs(dir string) ([]any, error) {
	items := []any{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".git" || rel == ".cache" || rel == "bin" || strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, ".cache/") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			fileset := token.NewFileSet()
			parsed, parseErr := parser.ParseFile(fileset, path, data, 0)
			if parseErr != nil {
				return parseErr
			}
			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || !strings.HasPrefix(function.Name.Name, "Test") {
					continue
				}
				start := fileset.Position(function.Pos()).Offset
				end := fileset.Position(function.End()).Offset
				body := data[start:end]
				items = append(items, baselineItem("test:"+rel+":"+function.Name.Name, map[string]any{"source": string(body)}, func(snapshot map[string]any) {
					snapshot["path"], snapshot["kind"], snapshot["enabled"] = rel, "test", goTestEnabled(function)
				}))
			}
		} else if baselineHarnessKind(rel) != "" {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			kind := baselineHarnessKind(rel)
			items = append(items, baselineItem(kind+":"+rel, map[string]any{"content": string(data)}, func(snapshot map[string]any) {
				enabled := true
				if kind != "test-fixture" {
					enabled = testEnabled(data)
				}
				snapshot["path"], snapshot["kind"], snapshot["enabled"] = rel, kind, enabled
			}))
		}
		return nil
	})
	return items, err
}

func baselineHarnessKind(relative string) string {
	if strings.HasPrefix(relative, "examples/") || strings.HasPrefix(relative, "labs/") {
		return "lab"
	}
	if strings.HasPrefix(relative, "testdata/") {
		return "test-fixture"
	}
	base := filepath.Base(relative)
	if strings.HasPrefix(relative, "evals/run.") || (strings.HasPrefix(relative, "scripts/") && (strings.Contains(base, "test") || strings.Contains(base, "gate") || strings.Contains(base, "verify") || strings.Contains(base, "check"))) {
		return "test-harness"
	}
	return ""
}

func captureSkillEval(dir string, skill map[string]any) ([]any, float64, error) {
	evals, _ := skill["evals"].(map[string]any)
	minimum := numberValue(evals["minimum_pass_rate"])
	paths, err := filepath.Glob(filepath.Join(dir, "evals", "*skill-eval.json"))
	if err != nil || len(paths) == 0 {
		return nil, minimum, fmt.Errorf("Skill Eval baselineを取得できません: %w", err)
	}
	items := []any{}
	for _, path := range paths {
		doc, readErr := readDocument(path)
		if readErr != nil {
			return nil, minimum, readErr
		}
		for _, raw := range anySlice(doc["cases"]) {
			item, _ := raw.(map[string]any)
			items = append(items, baselineItem(stringValue(item["id"]), item, nil))
		}
	}
	return items, minimum, nil
}

func captureMatrix(dir string) []any {
	return captureRowsIfPresent(filepath.Join(dir, "verification.matrix.yaml"), "rows", func(item map[string]any) string {
		return stringValue(item["behavior_id"]) + ":" + stringValue(item["scenario"])
	})
}

func captureRowsIfPresent(path, field string, id func(map[string]any) string) []any {
	doc, err := readDocument(path)
	if err != nil {
		return []any{}
	}
	return snapshotEntities(anySlice(doc[field]), id, nil)
}

func captureCI(dir string) ([]any, error) {
	paths, err := filepath.Glob(filepath.Join(dir, ".github", "workflows", "*.y*ml"))
	if err != nil {
		return nil, err
	}
	items := []any{}
	for _, path := range paths {
		doc, readErr := readYAMLWithoutSchema(path)
		if readErr != nil {
			return nil, readErr
		}
		jobs, _ := doc["jobs"].(map[string]any)
		for id, raw := range jobs {
			job, _ := raw.(map[string]any)
			rel, _ := filepath.Rel(dir, path)
			items = append(items, baselineItem(filepath.ToSlash(rel)+":"+id, job, func(snapshot map[string]any) {
				snapshot["path"], snapshot["kind"], snapshot["enabled"], snapshot["platform"] = filepath.ToSlash(rel), "ci-job", true, job["runs-on"]
			}))
		}
	}
	return items, nil
}

func readYAMLWithoutSchema(path string) (map[string]any, error) { return readDocumentBytes(path) }

func readDocumentBytes(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func snapshotEntities(values []any, id func(map[string]any) string, decorate func(map[string]any, map[string]any)) []any {
	items := make([]any, 0, len(values))
	for _, raw := range values {
		entity, _ := raw.(map[string]any)
		items = append(items, baselineItem(id(entity), entity, func(snapshot map[string]any) {
			if decorate != nil {
				decorate(entity, snapshot)
			}
		}))
	}
	return items
}

func baselineItem(id string, semantic any, decorate func(map[string]any)) map[string]any {
	fingerprint, _ := digestCanonical(semantic)
	item := map[string]any{"id": id, "fingerprint": fingerprint}
	if decorate != nil {
		decorate(item)
	}
	return item
}

func sortBaselineItems(items []any) {
	sort.Slice(items, func(i, j int) bool {
		return stringValue(items[i].(map[string]any)["id"]) < stringValue(items[j].(map[string]any)["id"])
	})
}

func sortedEntityIDs(entities map[string]map[string]any) []string {
	ids := make([]string, 0, len(entities))
	for id := range entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func testEnabled(data []byte) bool {
	lower := strings.ToLower(string(data))
	for _, marker := range []string{"@disabled", "@ignore", "disabled: true", "enabled: false", "pytest.mark.skip", "pytest.mark.xfail", "test.skip(", "describe.skip(", "xdescribe(", "skip=1", "skip=true", "xfail=1", "xfail=true"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func goTestEnabled(function *ast.FuncDecl) bool {
	enabled := true
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && (selector.Sel.Name == "Skip" || selector.Sel.Name == "Skipf" || selector.Sel.Name == "SkipNow") {
			enabled = false
		}
		return true
	})
	return enabled
}

func evidenceExecutionMode(evidence map[string]any) string {
	if mode := stringValue(evidence["execution_mode"]); mode != "" {
		return mode
	}
	command := strings.ToLower(stringValue(evidence["command"]))
	if strings.Contains(command, "go test") || strings.Contains(command, "atlas audit") || strings.Contains(command, "make ") {
		return "runtime"
	}
	return "static"
}

func indexBaselineItems(values []any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, raw := range values {
		item, _ := raw.(map[string]any)
		result[stringValue(item["id"])] = item
	}
	return result
}

func indexReplacements(values []any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, raw := range values {
		item, _ := raw.(map[string]any)
		result[stringValue(item["collection"])+":"+stringValue(item["old_id"])] = item
	}
	return result
}

func rejectDirectWeakening(collection string, oldItem, newItem map[string]any) error {
	id := stringValue(oldItem["id"])
	if collection == "tests_labs" && newItem["enabled"] != true {
		return fmt.Errorf("Test/Labをskip・xfail・disabled化できません: %s", id)
	}
	if collection == "targets" {
		if requirementRank(stringValue(newItem["requirement"])) < requirementRank(stringValue(oldItem["requirement"])) {
			return fmt.Errorf("Target requirementを弱化できません: %s %s→%s", id, oldItem["requirement"], newItem["requirement"])
		}
		if (oldItem["state"] == "covered" || oldItem["state"] == "partial" || oldItem["state"] == "planned") && (newItem["state"] == "excluded" || newItem["state"] == "infeasible") {
			return fmt.Errorf("covered/partial/planned Targetをexcluded/infeasibleへ退避できません: %s", id)
		}
	}
	if collection == "evidence" {
		if oldItem["verdict"] != "pass" && oldItem["fingerprint"] != newItem["fingerprint"] {
			return fmt.Errorf("失敗Evidenceを削除・上書きできません: %s", id)
		}
		if (oldItem["execution_mode"] == "runtime" || oldItem["execution_mode"] == "platform") && (newItem["execution_mode"] == "static" || newItem["execution_mode"] == "fixture") {
			return fmt.Errorf("実Runtime/Platform Evidenceをstatic/mock/fixtureへ置換できません: %s", id)
		}
	}
	return nil
}

func validateReplacement(ctx *auditContext, collection string, oldItem, mapping map[string]any, current map[string]map[string]any) error {
	oldID := stringValue(oldItem["id"])
	claimCount, evidenceCount := 0, 0
	for _, raw := range anySlice(mapping["new_ids"]) {
		id := stringValue(raw)
		candidate := current[id]
		if candidate == nil {
			return fmt.Errorf("Replacement Mappingの新IDが存在しません: %s:%s", collection, id)
		}
		if err := rejectDirectWeakening(collection, oldItem, candidate); err != nil {
			return err
		}
		claimCount += int(numberValue(candidate["claim_count"]))
		evidenceCount += int(numberValue(candidate["evidence_count"]))
	}
	if collection == "targets" && (claimCount < int(numberValue(oldItem["claim_count"])) || evidenceCount < int(numberValue(oldItem["evidence_count"]))) {
		return fmt.Errorf("Replacement TargetのClaim/Evidence能力がBaselineより減少しています: %s", oldID)
	}
	if err := validateReplacementEvidence(ctx, anySlice(mapping["execution_proof_evidence_ids"]), false); err != nil {
		return fmt.Errorf("Replacement %s:%sの実行Proofが無効です: %w", collection, oldID, err)
	}
	if err := validateReplacementEvidence(ctx, anySlice(mapping["migration_evidence_ids"]), true); err != nil {
		return fmt.Errorf("Replacement %s:%sのMigration Evidenceが無効です: %w", collection, oldID, err)
	}
	return nil
}

func validateReplacementEvidence(ctx *auditContext, ids []any, migration bool) error {
	for _, raw := range ids {
		id := stringValue(raw)
		evidence := ctx.evidence[id]
		if evidence == nil || evidence["verdict"] != "pass" {
			return fmt.Errorf("pass Evidenceがありません: %s", id)
		}
		mode := evidenceExecutionMode(evidence)
		if mode != "runtime" && mode != "platform" {
			return fmt.Errorf("Runtime/Platform Evidenceではありません: %s", id)
		}
		if migration {
			matched := false
			for _, claim := range anySlice(evidence["claim_ids"]) {
				matched = matched || strings.Contains(stringValue(claim), "migration")
			}
			if !matched {
				return fmt.Errorf("Migration Claimへ接続されていません: %s", id)
			}
		}
	}
	return nil
}

func requirementRank(value string) int {
	switch value {
	case "required":
		return 3
	case "recommended":
		return 2
	case "optional":
		return 1
	default:
		return 0
	}
}

func stringSet(values []any) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[stringValue(value)] = true
	}
	return result
}

func numberValue(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int:
		return float64(number)
	default:
		return 0
	}
}
