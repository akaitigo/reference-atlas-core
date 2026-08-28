package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AuthorityRelockResult struct {
	AtlasID           string
	Candidates        int
	Unchanged         int
	AuthorizedUpdates int
}

// AuditAuthorityRelock verifies that an observed stale-body candidate cannot
// update Source Lock automatically. Every actual update needs a human choice,
// old-to-new mapping, execution proof, migration evidence, and non-regression
// evidence.
func AuditAuthorityRelock(dir string) (AuthorityRelockResult, error) {
	path := filepath.Join(dir, "authority", "stale-relock-candidates.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return AuthorityRelockResult{}, nil
	} else if err != nil {
		return AuthorityRelockResult{}, err
	}
	if _, err := File(path); err != nil {
		return AuthorityRelockResult{}, err
	}
	report, err := readDocument(path)
	if err != nil {
		return AuthorityRelockResult{}, err
	}
	sources, err := readDocument(filepath.Join(dir, "sources.lock.yaml"))
	if err != nil {
		return AuthorityRelockResult{}, err
	}
	locks := map[string]map[string]any{}
	for _, raw := range anySlice(sources["sources"]) {
		item, _ := raw.(map[string]any)
		locks[stringValue(item["id"])] = item
	}
	result := AuthorityRelockResult{AtlasID: stringValue(report["atlas_id"]), Candidates: len(anySlice(report["candidates"]))}
	expectedDecisionFiles := []string{}
	ctx, contextErr := loadAuditContext(dir)
	for _, raw := range anySlice(report["candidates"]) {
		candidate, _ := raw.(map[string]any)
		oldLock, _ := candidate["current_lock"].(map[string]any)
		selectedLocks := []map[string]any{}
		sameURL, _ := candidate["same_url_observation"].(map[string]any)
		alternative, _ := candidate["immutable_alternative"].(map[string]any)
		selectedLocks = append(selectedLocks, map[string]any{"choice": "same-url-updated-body", "url": sameURL["final_url"], "digest": sameURL["digest"]}, map[string]any{"choice": "immutable-artifact-alternative", "url": alternative["url"], "digest": alternative["digest"]})
		changed := false
		selected := map[string]any(nil)
		for _, rawSourceID := range anySlice(candidate["source_ids"]) {
			sourceID := stringValue(rawSourceID)
			lock := locks[sourceID]
			if lock == nil {
				return AuthorityRelockResult{}, fmt.Errorf("stale relock candidateのSourceがLockにありません: %s", sourceID)
			}
			if lock["url"] == oldLock["url"] && lock["digest"] == oldLock["digest"] {
				continue
			}
			changed = true
			for _, option := range selectedLocks {
				if lock["url"] == option["url"] && lock["digest"] == option["digest"] {
					selected = option
				}
			}
			if selected == nil {
				return AuthorityRelockResult{}, fmt.Errorf("Source Lock更新が明示候補のいずれにも一致しません: %s", sourceID)
			}
		}
		if !changed {
			result.Unchanged++
			continue
		}
		decisionName := "relock-decision." + stringValue(candidate["document_id"]) + ".json"
		expectedDecisionFiles = append(expectedDecisionFiles, decisionName)
		decisionPath := filepath.Join(dir, "authority", "relock-decisions", decisionName)
		if _, err := File(decisionPath); err != nil {
			return AuthorityRelockResult{}, fmt.Errorf("stale Lock更新に明示Human decisionがありません: %w", err)
		}
		decision, err := readDocument(decisionPath)
		if err != nil {
			return AuthorityRelockResult{}, err
		}
		decisionOld, _ := decision["old_lock"].(map[string]any)
		decisionNew, _ := decision["new_lock"].(map[string]any)
		if decision["atlas_id"] != report["atlas_id"] || decision["document_id"] != candidate["document_id"] || decision["choice"] != selected["choice"] || decisionOld["url"] != oldLock["url"] || decisionOld["digest"] != oldLock["digest"] || decisionNew["url"] != selected["url"] || decisionNew["digest"] != selected["digest"] {
			return AuthorityRelockResult{}, fmt.Errorf("stale relock decisionの選択/旧/new Lockが候補と一致しません: %s", candidate["document_id"])
		}
		mappings := map[string]bool{}
		for _, rawMapping := range anySlice(decision["old_to_new_mappings"]) {
			mapping, _ := rawMapping.(map[string]any)
			mappings[stringValue(mapping["old_id"])] = true
		}
		for _, rawSourceID := range anySlice(candidate["source_ids"]) {
			if !mappings[stringValue(rawSourceID)] {
				return AuthorityRelockResult{}, fmt.Errorf("stale relock decisionが全旧Source IDをmappingしていません")
			}
		}
		if contextErr != nil {
			return AuthorityRelockResult{}, contextErr
		}
		if err := validateReplacementEvidence(ctx, anySlice(decision["execution_proof_ids"]), false); err != nil {
			return AuthorityRelockResult{}, fmt.Errorf("stale relock execution Proof: %w", err)
		}
		if err := validateReplacementEvidence(ctx, anySlice(decision["migration_evidence_ids"]), true); err != nil {
			return AuthorityRelockResult{}, fmt.Errorf("stale relock Migration Evidence: %w", err)
		}
		if err := validateReplacementEvidence(ctx, []any{decision["non_regression_evidence_id"]}, false); err != nil {
			return AuthorityRelockResult{}, fmt.Errorf("stale relock Non-regression Evidence: %w", err)
		}
		result.AuthorizedUpdates++
	}
	decisionDir := filepath.Join(dir, "authority", "relock-decisions")
	actualDecisionFiles := []string{}
	if entries, err := os.ReadDir(decisionDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				return AuthorityRelockResult{}, fmt.Errorf("relock decision directoryに許可されないentryがあります: %s", entry.Name())
			}
			actualDecisionFiles = append(actualDecisionFiles, entry.Name())
		}
	} else if !os.IsNotExist(err) {
		return AuthorityRelockResult{}, err
	}
	sort.Strings(expectedDecisionFiles)
	sort.Strings(actualDecisionFiles)
	if !sameStrings(expectedDecisionFiles, actualDecisionFiles) {
		return AuthorityRelockResult{}, fmt.Errorf("stale relock decision集合が実際のLock更新と一致しません")
	}
	return result, nil
}
