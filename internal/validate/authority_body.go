package validate

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AuthorityBodyResult struct {
	AtlasID                      string
	Status                       string
	SourceEntries                int
	UniqueDocuments              int
	MatchedDocuments             int
	StaleDocuments               int
	FailedDocuments              int
	Anchors                      int
	ClassifiedAnchors            int
	UnclassifiedAnchors          int
	HumanReviewedAnchors         int
	DeferredAnchors              int
	CoreV2EligibleArtifacts      int
	AuthoritySemanticsExhaustive bool
	BaselinePresent              bool
	EligibleSurfaceIDs           map[string]bool
}

type authorityBodyAnchorRecord struct {
	documentID string
}

// AuditAuthorityBodyInventory verifies the candidate-anchor denominator. Raw
// anchors are review inputs and never count as Authority Surfaces or Depth.
func AuditAuthorityBodyInventory(dir string, requireClosed bool) (AuthorityBodyResult, error) {
	indexPath := filepath.Join(dir, "authority", "body-inventory.snapshot.json")
	if _, err := File(indexPath); err != nil {
		return AuthorityBodyResult{}, err
	}
	index, err := readDocument(indexPath)
	if err != nil {
		return AuthorityBodyResult{}, err
	}
	sourcesDoc, err := readDocument(filepath.Join(dir, "sources.lock.yaml"))
	if err != nil {
		return AuthorityBodyResult{}, err
	}
	if stringValue(index["atlas_id"]) != stringValue(sourcesDoc["atlas_id"]) {
		return AuthorityBodyResult{}, fmt.Errorf("Authority body denominatorのAtlas IDがSource Lockと一致しません")
	}
	locked := map[string]map[string]any{}
	for _, raw := range anySlice(sourcesDoc["sources"]) {
		source, _ := raw.(map[string]any)
		locked[stringValue(source["id"])] = source
	}

	selectorContract := anySlice(index["selector_contract"])
	toolDigest := stringValue(index["tool_digest"])
	bodyStorage := stringValue(index["body_storage"])
	documents := anySlice(index["documents"])
	seenDocs := map[string]bool{}
	seenSources := map[string]bool{}
	anchors := map[string]authorityBodyAnchorRecord{}
	expectedFiles := []string{}
	matched, stale, failed, selectorExhaustive := 0, 0, 0, 0
	anchorCount := 0
	kindCounts := map[string]int{}
	contextUnitByMethod := map[string]string{}
	for _, raw := range documents {
		record, _ := raw.(map[string]any)
		documentID := stringValue(record["id"])
		if seenDocs[documentID] {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body documentが重複しています: %s", documentID)
		}
		seenDocs[documentID] = true
		relative := stringValue(record["path"])
		if relative != "authority/body-inventory-draft/"+documentID+".json" {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body document pathがIDと一致しません: %s", documentID)
		}
		if err := verifyRelativeFileDigest(dir, relative, stringValue(record["digest"]), -1); err != nil {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body document %s: %w", documentID, err)
		}
		expectedFiles = append(expectedFiles, filepath.Base(relative))
		doc, err := readDocument(filepath.Join(dir, filepath.FromSlash(relative)))
		if err != nil {
			return AuthorityBodyResult{}, err
		}
		if stringValue(doc["document_id"]) != documentID {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body document identityが一致しません: %s", documentID)
		}
		lockedDigest := stringValue(doc["locked_body_digest"])
		directURL := withoutFragment(stringValue(doc["fetch_url"]))
		authorityURL := withoutFragment(stringValue(doc["authority_url"]))
		containerDigest := stringValue(doc["locked_source_digest"])
		for _, rawSourceID := range anySlice(doc["source_ids"]) {
			sourceID := stringValue(rawSourceID)
			source := locked[sourceID]
			validDirect := directURL != "" && withoutFragment(stringValue(source["url"])) == directURL && stringValue(source["digest"]) == lockedDigest
			validContainer := authorityURL != "" && withoutFragment(stringValue(source["url"])) == authorityURL && stringValue(source["digest"]) == containerDigest && stringValue(doc["document_locator"]) != ""
			if source == nil || (!validDirect && !validContainer) {
				return AuthorityBodyResult{}, fmt.Errorf("Authority body documentのSource Lock closureが不正です: %s", sourceID)
			}
			seenSources[sourceID] = true
		}
		if len(anySlice(doc["source_ids"])) != int(numberValue(record["source_entries"])) {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body documentのSource数がIndexと一致しません: %s", documentID)
		}
		extraction, _ := doc["extraction"].(map[string]any)
		method := stringValue(extraction["method"])
		if stringValue(extraction["tool_digest"]) != toolDigest || !sameStringSet(anySlice(extraction["selector_contract"]), selectorContract) ||
			extraction["review_status"] != "automated-unreviewed" || extraction["authority_semantics_exhaustive"] != false || stringValue(extraction["body_storage"]) != bodyStorage {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body documentの抽出・Review境界が不正です: %s", documentID)
		}
		fetch, _ := doc["fetch"].(map[string]any)
		status := stringValue(fetch["status"])
		docAnchors := anySlice(doc["anchors"])
		switch status {
		case "matched":
			matched++
			if fetch["locked_digest_match"] != true || stringValue(fetch["fetched_digest"]) != lockedDigest || extraction["selector_exhaustive_for_locked_body"] != true || len(docAnchors) == 0 || fetch["error_digest"] != nil {
				return AuthorityBodyResult{}, fmt.Errorf("matched Authority body documentが不正です: %s", documentID)
			}
			selectorExhaustive++
		case "stale":
			stale++
			if fetch["locked_digest_match"] != false || stringValue(fetch["fetched_digest"]) == "" || stringValue(fetch["fetched_digest"]) == lockedDigest || extraction["selector_exhaustive_for_locked_body"] != false || len(docAnchors) != 0 || fetch["error_digest"] != nil {
				return AuthorityBodyResult{}, fmt.Errorf("stale Authority body documentが不正です: %s", documentID)
			}
		case "failed":
			failed++
			if fetch["locked_digest_match"] != false || fetch["fetched_digest"] != nil || stringValue(fetch["error_digest"]) == "" || extraction["selector_exhaustive_for_locked_body"] != false || len(docAnchors) != 0 {
				return AuthorityBodyResult{}, fmt.Errorf("failed Authority body documentが不正です: %s", documentID)
			}
		}
		local := map[string]bool{}
		localKinds := map[string]int{}
		for position, rawAnchor := range docAnchors {
			anchor, _ := rawAnchor.(map[string]any)
			anchorID := stringValue(anchor["id"])
			if local[anchorID] || anchors[anchorID].documentID != "" {
				return AuthorityBodyResult{}, fmt.Errorf("Authority body anchor IDが重複しています: %s", anchorID)
			}
			if anchor["classification_status"] != "pending-human" || len(anySlice(anchor["surface_ids"])) != 0 || numberValue(anchor["context_end"]) <= numberValue(anchor["context_start"]) {
				return AuthorityBodyResult{}, fmt.Errorf("未Review candidate anchorをSurfaceへ昇格できません: %s", anchorID)
			}
			if len(anySlice(anchor["behavior_ids"])) != 0 {
				return AuthorityBodyResult{}, fmt.Errorf("未Review candidate anchorをBehaviorへ昇格できません: %s", anchorID)
			}
			unit := stringValue(anchor["context_unit"])
			if existing := contextUnitByMethod[method]; existing != "" && existing != unit {
				return AuthorityBodyResult{}, fmt.Errorf("同じAuthority selector methodでoffset unitを混在できません: method=%s", method)
			}
			contextUnitByMethod[method] = unit
			parent := stringValue(anchor["parent_anchor_id"])
			if position == 0 {
				rootKind := stringValue(anchor["semantic_kind"])
				if rootKind == "" {
					rootKind = stringValue(anchor["raw_selector"])
				}
				if rootKind != "document-root" || anchor["locator"] != "document-root" || parent != "" || stringValue(anchor["context_digest"]) != lockedDigest {
					return AuthorityBodyResult{}, fmt.Errorf("Authority body root anchorが不正です: %s", documentID)
				}
			} else if !local[parent] {
				return AuthorityBodyResult{}, fmt.Errorf("Authority body anchor parentが先行定義されていません: %s", anchorID)
			}
			local[anchorID] = true
			anchors[anchorID] = authorityBodyAnchorRecord{documentID: documentID}
			selector := stringValue(anchor["raw_selector"])
			kind := selector
			if selector == "" {
				kind = stringValue(anchor["semantic_kind"])
				if position == 0 {
					selector = "document-root"
				} else {
					selector = strings.ToLower(stringValue(anchor["tag"]))
				}
			}
			if !containsString(selectorContract, selector) {
				return AuthorityBodyResult{}, fmt.Errorf("Authority anchor selectorがSubject contract外です: %s", selector)
			}
			localKinds[kind]++
			kindCounts[kind]++
			anchorCount++
		}
		indexedKinds, _ := record["anchors_by_kind"].(map[string]any)
		if indexedKinds == nil {
			indexedKinds, _ = record["anchors_by_selector"].(map[string]any)
		}
		if int(numberValue(record["anchors"])) != len(docAnchors) || !sameCounts(localKinds, indexedKinds) || stringValue(record["fetch_status"]) != status {
			return AuthorityBodyResult{}, fmt.Errorf("Authority body document Indexが実体と一致しません: %s", documentID)
		}
	}
	if len(seenSources) != len(locked) {
		return AuthorityBodyResult{}, fmt.Errorf("Authority body denominatorが全Source Lock entryを閉じていません: locked=%d used=%d", len(locked), len(seenSources))
	}
	actualFiles, err := authorityDraftFiles(filepath.Join(dir, "authority", "body-inventory-draft"))
	if err != nil {
		return AuthorityBodyResult{}, err
	}
	sort.Strings(expectedFiles)
	if !sameStrings(expectedFiles, actualFiles) {
		return AuthorityBodyResult{}, fmt.Errorf("Authority body document file集合がIndexと一致しません")
	}

	summary, _ := index["summary"].(map[string]any)
	indexedKinds, _ := summary["anchors_by_kind"].(map[string]any)
	if indexedKinds == nil {
		indexedKinds, _ = summary["anchors_by_selector"].(map[string]any)
	}
	summaryAnchors := int(numberValue(summary["anchors"]))
	if summary["anchors"] == nil {
		summaryAnchors = int(numberValue(summary["raw_anchor_candidates"]))
	}
	if int(numberValue(summary["source_entries"])) != len(locked) || int(numberValue(summary["unique_documents"])) != len(documents) ||
		int(numberValue(summary["matched_documents"])) != matched || int(numberValue(summary["stale_documents"])) != stale || int(numberValue(summary["failed_documents"])) != failed ||
		int(numberValue(summary["selector_exhaustive_documents"])) != selectorExhaustive || summaryAnchors != anchorCount || !sameCounts(kindCounts, indexedKinds) {
		return AuthorityBodyResult{}, fmt.Errorf("Authority body denominator summaryがArtifact実体と一致しません")
	}
	classified := int(numberValue(summary["classified_anchors"]))
	unclassified := int(numberValue(summary["unclassified_anchors"]))
	if summary["pending_human_anchors"] != nil {
		unclassified = int(numberValue(summary["pending_human_anchors"]))
		classified = anchorCount - unclassified
	}
	eligible := int(numberValue(summary["core_v2_eligible_artifacts"]))
	if summary["promoted_surface_artifacts"] != nil {
		eligible = int(numberValue(summary["promoted_surface_artifacts"]))
	}
	result := AuthorityBodyResult{
		AtlasID: stringValue(index["atlas_id"]), Status: stringValue(index["status"]), SourceEntries: len(locked), UniqueDocuments: len(documents),
		MatchedDocuments: matched, StaleDocuments: stale, FailedDocuments: failed, Anchors: anchorCount,
		ClassifiedAnchors: classified, UnclassifiedAnchors: unclassified,
		HumanReviewedAnchors: int(numberValue(summary["human_reviewed_anchors"])), CoreV2EligibleArtifacts: eligible,
		AuthoritySemanticsExhaustive: summary["authority_semantics_exhaustive"] == true, EligibleSurfaceIDs: map[string]bool{},
	}
	if result.ClassifiedAnchors != 0 || result.UnclassifiedAnchors != anchorCount || result.HumanReviewedAnchors != 0 || result.CoreV2EligibleArtifacts != 0 || result.AuthoritySemanticsExhaustive {
		return AuthorityBodyResult{}, fmt.Errorf("candidate anchor denominatorをHuman Review結果やAuthority Surface/Depth達成として扱えません")
	}
	present, err := auditAuthorityBodyBaseline(dir, index, anchors)
	if err != nil {
		return AuthorityBodyResult{}, err
	}
	result.BaselinePresent = present
	if requireClosed {
		if result.StaleDocuments > 0 || result.FailedDocuments > 0 {
			return AuthorityBodyResult{}, fmt.Errorf("candidate anchor denominatorに未解決Source stateがあります: stale=%d failed=%d", result.StaleDocuments, result.FailedDocuments)
		}
		if !present {
			return AuthorityBodyResult{}, fmt.Errorf("subject-definitiveには専用Authority body non-regression baselineが必要です")
		}
	}
	return result, nil
}

func auditAuthorityBodyBaseline(dir string, index map[string]any, anchors map[string]authorityBodyAnchorRecord) (bool, error) {
	baselinePath := filepath.Join(dir, "baselines", "authority-body-inventory-v1.json")
	migrationPath := filepath.Join(dir, "migrations", "authority-body-inventory-v1.json")
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, err := File(baselinePath); err != nil {
		return false, err
	}
	if _, err := File(migrationPath); err != nil {
		return false, err
	}
	baseline, err := readDocument(baselinePath)
	if err != nil {
		return false, err
	}
	migration, err := readDocument(migrationPath)
	if err != nil {
		return false, err
	}
	if stringValue(migration["baseline_id"]) != stringValue(baseline["id"]) || int(numberValue(index["summary"].(map[string]any)["source_entries"])) < int(numberValue(baseline["source_entries"])) ||
		len(anySlice(index["documents"])) < int(numberValue(baseline["unique_documents"])) || !sameStringSet(anySlice(index["selector_contract"]), anySlice(baseline["selector_contract"])) {
		return false, fmt.Errorf("Authority body non-regression baselineのSource/document/selector floorが縮小しています")
	}
	baselineAnchors := map[string]bool{}
	baselineAnchorDigests := map[string]string{}
	baselineAnchorDocuments := map[string]string{}
	currentDocuments := map[string]map[string]any{}
	for _, raw := range anySlice(index["documents"]) {
		record, _ := raw.(map[string]any)
		doc, err := readDocument(filepath.Join(dir, filepath.FromSlash(stringValue(record["path"]))))
		if err != nil {
			return false, err
		}
		currentDocuments[stringValue(record["id"])] = doc
	}
	for _, raw := range anySlice(baseline["documents"]) {
		doc, _ := raw.(map[string]any)
		current := currentDocuments[stringValue(doc["id"])]
		if current == nil || stringValue(doc["path"]) != "authority/body-inventory-draft/"+stringValue(doc["id"])+".json" ||
			stringValue(current["locked_body_digest"]) != stringValue(doc["locked_body_digest"]) || !sameStringSet(anySlice(current["source_ids"]), anySlice(doc["source_ids"])) {
			return false, fmt.Errorf("Authority body baseline documentが削除または置換されています: %s", doc["id"])
		}
		for _, rawID := range anySlice(doc["anchor_ids"]) {
			id := stringValue(rawID)
			if baselineAnchors[id] {
				return false, fmt.Errorf("Authority body baseline anchorが重複しています: %s", id)
			}
			baselineAnchors[id] = true
			baselineAnchorDigests[id] = stringValue(doc["locked_body_digest"])
			baselineAnchorDocuments[id] = stringValue(doc["id"])
		}
	}
	replaced, replacementNew := map[string]bool{}, map[string]bool{}
	for _, raw := range anySlice(migration["replacements"]) {
		item, _ := raw.(map[string]any)
		oldID := stringValue(item["old_anchor_id"])
		if !baselineAnchors[oldID] || replaced[oldID] || anchors[oldID].documentID != "" || stringValue(item["source_digest"]) != baselineAnchorDigests[oldID] || stringValue(item["tool_digest"]) != stringValue(index["tool_digest"]) || stringValue(item["execution_proof"]) == stringValue(item["migration_evidence"]) {
			return false, fmt.Errorf("Authority body anchor replacement mappingが不正です: %s", oldID)
		}
		for _, rawNew := range anySlice(item["new_anchor_ids"]) {
			newID := stringValue(rawNew)
			if anchors[newID].documentID == "" || replacementNew[newID] {
				return false, fmt.Errorf("Authority body replacement先が現行stable anchorでないか共有されています: %s", newID)
			}
			replacementNew[newID] = true
		}
		for _, proof := range []string{stringValue(item["execution_proof"]), stringValue(item["migration_evidence"])} {
			clean := filepath.Clean(filepath.FromSlash(proof))
			if proof == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return false, fmt.Errorf("Authority body replacement Evidence pathが安全ではありません: %s", proof)
			}
			if _, err := os.Stat(filepath.Join(dir, clean)); err != nil {
				return false, fmt.Errorf("Authority body replacement Evidenceがありません: %s", proof)
			}
		}
		replaced[oldID] = true
	}
	for id := range baselineAnchors {
		if anchors[id].documentID == "" && !replaced[id] {
			return false, fmt.Errorf("Authority body anchorがMappingなしで削除されています: %s", id)
		}
		if anchors[id].documentID != "" && anchors[id].documentID != baselineAnchorDocuments[id] {
			return false, fmt.Errorf("Authority body anchorがdocument間で移動しています: %s", id)
		}
	}
	return true, nil
}

func withoutFragment(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return parsed.String()
}

func authorityBodyFingerprint(result AuthorityBodyResult) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%d:%t", result.Status, result.UniqueDocuments, result.Anchors, result.ClassifiedAnchors, result.UnclassifiedAnchors, result.DeferredAnchors, result.AuthoritySemanticsExhaustive)
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func qualifiedAuthoritySurfaceID(artifactID, surfaceID string) string {
	return strings.TrimSuffix(artifactID, ".") + "." + strings.TrimPrefix(surfaceID, ".")
}
