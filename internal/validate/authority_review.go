package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type AuthorityReviewResult struct {
	AtlasID                      string
	Status                       string
	QueuedAnchors                int
	PendingHuman                 int
	HumanReviewed                int
	Deferred                     int
	StaleHolds                   int
	UnavailableHolds             int
	Decisions                    int
	AuthoritySemanticsExhaustive bool
	SurfaceIDs                   map[string]bool
	AtomicBehaviorIDs            map[string]bool
}

type authorityReviewAnchor struct {
	documentID          string
	documentURL         string
	authorityURL        string
	documentLocator     string
	sourceIDs           []any
	lockedSourceDigest  string
	inventoryToolDigest string
	locator             string
	contextStart        int
	contextEnd          int
	contextUnit         string
	contextDigest       string
}

type authorityStaleDocument struct {
	documentID          string
	documentURL         string
	sourceIDs           []any
	lockedSourceDigest  string
	inventoryToolDigest string
	fetchedDigest       string
}

type authorityUnavailableDocument struct {
	documentID          string
	authorityURL        string
	documentLocator     string
	sourceIDs           []any
	lockedSourceDigest  string
	inventoryToolDigest string
	errorDigest         string
}

var automatedReviewer = regexp.MustCompile(`(?i)^(auto(?:mated)?|agent|bot|system|machine)(?:$|[-_. ])`)

// AuditAuthorityReviewQueue verifies the deterministic work queue and the
// human decision ledger. Queue/batch/cluster counts are operational metadata,
// never Completion or Depth credit.
func AuditAuthorityReviewQueue(dir string, requireClosed bool) (AuthorityReviewResult, error) {
	body, err := AuditAuthorityBodyInventory(dir, false)
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	anchors, staleDocuments, unavailableDocuments, matchedDocuments, bodyToolDigest, bodyStorage, err := loadAuthorityReviewInputs(dir)
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	indexPath := filepath.Join(dir, "authority", "review-queue.snapshot.json")
	if _, err := File(indexPath); err != nil {
		return AuthorityReviewResult{}, err
	}
	index, err := readDocument(indexPath)
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	queueID, queueToolDigest := stringValue(index["queue_id"]), stringValue(index["tool_digest"])
	if stringValue(index["atlas_id"]) != body.AtlasID || stringValue(index["body_storage"]) != bodyStorage || index["semantic_decisions"] != "human-only" {
		return AuthorityReviewResult{}, fmt.Errorf("Authority Review QueueがBody denominator境界と一致しません")
	}
	if value, ok := index["summary"].(map[string]any)["queue_counts_as_depth_achievement"]; ok && value != false {
		return AuthorityReviewResult{}, fmt.Errorf("Review Queue件数をDepth達成へ算入できません")
	}

	seenAnchors, seenBatches := map[string]bool{}, map[string]bool{}
	expectedFiles := []string{}
	priorityCounts := map[string]int{}
	clusterIDs := map[string]bool{}
	clusteredAnchors := 0
	for _, raw := range anySlice(index["batches"]) {
		record, _ := raw.(map[string]any)
		batchID, relative := stringValue(record["id"]), stringValue(record["path"])
		if seenBatches[batchID] || relative != "authority/review-queue-draft/"+batchID+".json" {
			return AuthorityReviewResult{}, fmt.Errorf("Authority Review batch identityが不正です: %s", batchID)
		}
		seenBatches[batchID] = true
		if err := verifyRelativeFileDigest(dir, relative, stringValue(record["digest"]), -1); err != nil {
			return AuthorityReviewResult{}, fmt.Errorf("Authority Review batch %s: %w", batchID, err)
		}
		expectedFiles = append(expectedFiles, filepath.Base(relative))
		full := filepath.Join(dir, filepath.FromSlash(relative))
		if _, err := File(full); err != nil {
			return AuthorityReviewResult{}, err
		}
		batch, err := readDocument(full)
		if err != nil {
			return AuthorityReviewResult{}, err
		}
		if stringValue(batch["queue_id"]) != queueID || stringValue(batch["batch_id"]) != batchID || batch["semantic_decisions"] != "none" {
			return AuthorityReviewResult{}, fmt.Errorf("Authority Review batchがQueue境界と一致しません: %s", batchID)
		}
		items := anySlice(batch["items"])
		if len(items) != int(numberValue(record["items"])) {
			return AuthorityReviewResult{}, fmt.Errorf("Authority Review batch item数がIndexと一致しません: %s", batchID)
		}
		for _, rawItem := range items {
			item, _ := rawItem.(map[string]any)
			anchorID := stringValue(item["anchor_id"])
			expected, ok := anchors[anchorID]
			if !ok || seenAnchors[anchorID] || stringValue(item["batch_id"]) != batchID || item["state"] != "pending-human" {
				return AuthorityReviewResult{}, fmt.Errorf("Review Queueのstable anchor完全包含が不正です: %s", anchorID)
			}
			seenAnchors[anchorID] = true
			if err := compareAuthorityReviewBinding(item, expected, bodyToolDigest, queueToolDigest); err != nil {
				return AuthorityReviewResult{}, err
			}
			priorityCounts[fmt.Sprintf("%d", int(numberValue(item["priority"])))]++
			if clusterID := stringValue(item["candidate_cluster_id"]); clusterID != "" {
				clusterIDs[clusterID] = true
				clusteredAnchors++
			}
		}
	}
	if len(seenAnchors) != len(anchors) {
		return AuthorityReviewResult{}, fmt.Errorf("Review Queueがstable anchorを完全包含していません: expected=%d queued=%d", len(anchors), len(seenAnchors))
	}
	actualFiles, err := authorityDraftFiles(filepath.Join(dir, "authority", "review-queue-draft"))
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	sort.Strings(expectedFiles)
	if !sameStrings(expectedFiles, actualFiles) {
		return AuthorityReviewResult{}, fmt.Errorf("Authority Review batch file集合がIndexと一致しません")
	}

	if err := auditAuthorityStaleHolds(index, staleDocuments, queueToolDigest); err != nil {
		return AuthorityReviewResult{}, err
	}
	if err := auditAuthorityUnavailableHolds(index, unavailableDocuments, queueToolDigest); err != nil {
		return AuthorityReviewResult{}, err
	}
	ledgerPath := filepath.Join(dir, filepath.FromSlash(stringValue(index["decision_ledger"])))
	if _, err := File(ledgerPath); err != nil {
		return AuthorityReviewResult{}, err
	}
	ledger, err := readDocument(ledgerPath)
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	if stringValue(ledger["atlas_id"]) != body.AtlasID || stringValue(ledger["queue_id"]) != queueID {
		return AuthorityReviewResult{}, fmt.Errorf("Authority Review decision ledger identityがQueueと一致しません")
	}
	result, actionCounts, err := auditAuthorityReviewDecisions(anySlice(ledger["decisions"]), anchors, bodyToolDigest, queueToolDigest)
	if err != nil {
		return AuthorityReviewResult{}, err
	}
	result.AtlasID = body.AtlasID
	result.Status = stringValue(index["status"])
	result.QueuedAnchors = len(anchors)
	result.PendingHuman = len(anchors) - result.HumanReviewed
	result.StaleHolds = len(staleDocuments)
	result.UnavailableHolds = len(unavailableDocuments)
	result.Decisions = len(anySlice(ledger["decisions"]))

	summary, _ := index["summary"].(map[string]any)
	indexedPriority, _ := summary["priority_counts"].(map[string]any)
	if int(numberValue(summary["eligible_documents"])) != matchedDocuments || int(numberValue(summary["queued_anchors"])) != len(anchors) ||
		int(numberValue(summary["pending_human"])) != result.PendingHuman || int(numberValue(summary["human_reviewed"])) != result.HumanReviewed ||
		int(numberValue(summary["batches"])) != len(seenBatches) || int(numberValue(summary["stale_document_holds"])) != len(staleDocuments) || int(numberValue(summary["unavailable_document_holds"])) != len(unavailableDocuments) ||
		int(numberValue(summary["candidate_clusters"])) != len(clusterIDs) || int(numberValue(summary["clustered_anchors"])) != clusteredAnchors || !sameCounts(priorityCounts, indexedPriority) ||
		int(numberValue(summary["decisions"])) != result.Decisions || int(numberValue(summary["included"])) != actionCounts["include"] || int(numberValue(summary["excluded"])) != actionCounts["exclude"] ||
		int(numberValue(summary["merged"])) != actionCounts["merge"] || int(numberValue(summary["split"])) != actionCounts["split"] || int(numberValue(summary["deferred"])) != actionCounts["defer"] {
		return AuthorityReviewResult{}, fmt.Errorf("Authority Review Queue summaryがBatch/decision実体と一致しません")
	}
	closed := result.PendingHuman == 0 && result.Deferred == 0 && result.StaleHolds == 0 && result.UnavailableHolds == 0 && body.FailedDocuments == 0 && result.Decisions > 0
	result.AuthoritySemanticsExhaustive = summary["authority_semantics_exhaustive"] == true
	if result.AuthoritySemanticsExhaustive != closed || (stringValue(index["status"]) == "closed") != closed || (stringValue(ledger["status"]) == "closed") != closed {
		return AuthorityReviewResult{}, fmt.Errorf("semantic exhaustive/statusは全stable anchorの人手Review、stale hold 0、decision 1件以上を必要とします")
	}
	if requireClosed && !closed {
		return AuthorityReviewResult{}, fmt.Errorf("Authority Human Review Queueが未完です: pending=%d deferred=%d stale_holds=%d unavailable_holds=%d failed=%d decisions=%d", result.PendingHuman, result.Deferred, result.StaleHolds, result.UnavailableHolds, body.FailedDocuments, result.Decisions)
	}
	return result, nil
}

func loadAuthorityReviewInputs(dir string) (map[string]authorityReviewAnchor, map[string]authorityStaleDocument, map[string]authorityUnavailableDocument, int, string, string, error) {
	index, err := readDocument(filepath.Join(dir, "authority", "body-inventory.snapshot.json"))
	if err != nil {
		return nil, nil, nil, 0, "", "", err
	}
	anchors := map[string]authorityReviewAnchor{}
	stale := map[string]authorityStaleDocument{}
	unavailable := map[string]authorityUnavailableDocument{}
	matched := 0
	for _, raw := range anySlice(index["documents"]) {
		record, _ := raw.(map[string]any)
		doc, err := readDocument(filepath.Join(dir, filepath.FromSlash(stringValue(record["path"]))))
		if err != nil {
			return nil, nil, nil, 0, "", "", err
		}
		extraction, _ := doc["extraction"].(map[string]any)
		fetch, _ := doc["fetch"].(map[string]any)
		documentURL := stringValue(doc["fetch_url"])
		authorityURL := stringValue(doc["authority_url"])
		documentLocator := stringValue(doc["document_locator"])
		sourceDigest := stringValue(doc["locked_body_digest"])
		if documentURL == "" {
			documentURL = authorityURL
		}
		if fetch["status"] == "matched" {
			matched++
		}
		if fetch["status"] == "stale" {
			stale[stringValue(doc["document_id"])] = authorityStaleDocument{documentID: stringValue(doc["document_id"]), documentURL: documentURL, sourceIDs: anySlice(doc["source_ids"]), lockedSourceDigest: sourceDigest, inventoryToolDigest: stringValue(extraction["tool_digest"]), fetchedDigest: stringValue(fetch["fetched_digest"])}
		}
		if fetch["status"] == "failed" {
			unavailable[stringValue(doc["document_id"])] = authorityUnavailableDocument{documentID: stringValue(doc["document_id"]), authorityURL: authorityURL, documentLocator: documentLocator, sourceIDs: anySlice(doc["source_ids"]), lockedSourceDigest: stringValue(doc["locked_source_digest"]), inventoryToolDigest: stringValue(extraction["tool_digest"]), errorDigest: stringValue(fetch["error_digest"])}
		}
		for _, rawAnchor := range anySlice(doc["anchors"]) {
			anchor, _ := rawAnchor.(map[string]any)
			anchors[stringValue(anchor["id"])] = authorityReviewAnchor{documentID: stringValue(doc["document_id"]), documentURL: documentURL, authorityURL: authorityURL, documentLocator: documentLocator, sourceIDs: anySlice(doc["source_ids"]), lockedSourceDigest: sourceDigest, inventoryToolDigest: stringValue(extraction["tool_digest"]), locator: stringValue(anchor["locator"]), contextStart: int(numberValue(anchor["context_start"])), contextEnd: int(numberValue(anchor["context_end"])), contextUnit: stringValue(anchor["context_unit"]), contextDigest: stringValue(anchor["context_digest"])}
		}
	}
	return anchors, stale, unavailable, matched, stringValue(index["tool_digest"]), stringValue(index["body_storage"]), nil
}

func compareAuthorityReviewBinding(item map[string]any, expected authorityReviewAnchor, bodyToolDigest, queueToolDigest string) error {
	if stringValue(item["document_id"]) != expected.documentID || !reviewDocumentBindingMatches(item, expected) || !sameStringSet(anySlice(item["source_ids"]), expected.sourceIDs) ||
		stringValue(item["locked_source_digest"]) != expected.lockedSourceDigest || stringValue(item["inventory_tool_digest"]) != expected.inventoryToolDigest || expected.inventoryToolDigest != bodyToolDigest ||
		stringValue(item["review_queue_tool_digest"]) != queueToolDigest || stringValue(item["locator"]) != expected.locator || int(numberValue(item["context_start"])) != expected.contextStart ||
		int(numberValue(item["context_end"])) != expected.contextEnd || stringValue(item["context_unit"]) != expected.contextUnit || stringValue(item["context_digest"]) != expected.contextDigest {
		return fmt.Errorf("Review Queue itemのsource/tool/locator bindingがBody anchorと一致しません: %s", item["anchor_id"])
	}
	return nil
}

func reviewDocumentBindingMatches(value map[string]any, expected authorityReviewAnchor) bool {
	if documentURL := stringValue(value["document_url"]); documentURL != "" {
		return documentURL == expected.documentURL && stringValue(value["authority_url"]) == "" && stringValue(value["document_locator"]) == ""
	}
	return stringValue(value["authority_url"]) == expected.authorityURL && stringValue(value["document_locator"]) == expected.documentLocator
}

func auditAuthorityStaleHolds(index map[string]any, expected map[string]authorityStaleDocument, queueToolDigest string) error {
	seen := map[string]bool{}
	for _, raw := range anySlice(index["stale_holds"]) {
		hold, _ := raw.(map[string]any)
		id := stringValue(hold["document_id"])
		doc, ok := expected[id]
		if !ok || seen[id] || stringValue(hold["document_url"]) != doc.documentURL || !sameStringSet(anySlice(hold["source_ids"]), doc.sourceIDs) ||
			stringValue(hold["locked_source_digest"]) != doc.lockedSourceDigest || stringValue(hold["inventory_tool_digest"]) != doc.inventoryToolDigest || stringValue(hold["review_queue_tool_digest"]) != queueToolDigest || stringValue(hold["fetched_digest"]) != doc.fetchedDigest {
			return fmt.Errorf("stale holdがBody stale documentと一致しません: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("stale documentをReview Queueから隔離したholdが不足しています")
	}
	return nil
}

func auditAuthorityUnavailableHolds(index map[string]any, expected map[string]authorityUnavailableDocument, queueToolDigest string) error {
	seen := map[string]bool{}
	for _, raw := range anySlice(index["unavailable_holds"]) {
		hold, _ := raw.(map[string]any)
		id := stringValue(hold["document_id"])
		doc, ok := expected[id]
		if !ok || seen[id] || stringValue(hold["authority_url"]) != doc.authorityURL || stringValue(hold["document_locator"]) != doc.documentLocator || !sameStringSet(anySlice(hold["source_ids"]), doc.sourceIDs) ||
			stringValue(hold["locked_source_digest"]) != doc.lockedSourceDigest || stringValue(hold["inventory_tool_digest"]) != doc.inventoryToolDigest || stringValue(hold["review_queue_tool_digest"]) != queueToolDigest || stringValue(hold["error_digest"]) != doc.errorDigest {
			return fmt.Errorf("unavailable holdがBody failed documentと一致しません: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("failed documentをReview Queueから隔離したunavailable holdが不足しています")
	}
	return nil
}

func auditAuthorityReviewDecisions(decisions []any, anchors map[string]authorityReviewAnchor, bodyToolDigest, queueToolDigest string) (AuthorityReviewResult, map[string]int, error) {
	result := AuthorityReviewResult{SurfaceIDs: map[string]bool{}, AtomicBehaviorIDs: map[string]bool{}}
	actionCounts := map[string]int{}
	seenDecisions, seenAnchors, newOwners := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, raw := range decisions {
		decision, _ := raw.(map[string]any)
		decisionID, action := stringValue(decision["decision_id"]), stringValue(decision["action"])
		if seenDecisions[decisionID] {
			return result, actionCounts, fmt.Errorf("Authority Review decision IDが重複しています: %s", decisionID)
		}
		seenDecisions[decisionID] = true
		reviewer := strings.TrimSpace(stringValue(decision["reviewer"]))
		if automatedReviewer.MatchString(reviewer) || decision["review_method"] != "manual-primary-source" {
			return result, actionCounts, fmt.Errorf("自動reviewerや一次資料以外のReview methodは禁止です: %s", decisionID)
		}
		anchorIDs, bindings, mappings := anySlice(decision["anchor_ids"]), anySlice(decision["source_bindings"]), anySlice(decision["mapping"])
		if len(bindings) != len(anchorIDs) || len(mappings) != len(anchorIDs) {
			return result, actionCounts, fmt.Errorf("Decision anchor/binding/mapping cardinalityが一致しません: %s", decisionID)
		}
		bindingByAnchor, mappingByAnchor := map[string]map[string]any{}, map[string]map[string]any{}
		for _, rawBinding := range bindings {
			binding, _ := rawBinding.(map[string]any)
			id := stringValue(binding["anchor_id"])
			if bindingByAnchor[id] != nil {
				return result, actionCounts, fmt.Errorf("Decision source bindingが重複しています: %s", id)
			}
			bindingByAnchor[id] = binding
		}
		for _, rawMapping := range mappings {
			mapping, _ := rawMapping.(map[string]any)
			id := stringValue(mapping["old_anchor_id"])
			if mappingByAnchor[id] != nil {
				return result, actionCounts, fmt.Errorf("Decision old→new mappingが重複しています: %s", id)
			}
			mappingByAnchor[id] = mapping
		}
		mappingSets := map[string]bool{}
		mappedIDs := map[string]bool{}
		for _, rawID := range anchorIDs {
			anchorID := stringValue(rawID)
			expected, ok := anchors[anchorID]
			if !ok || seenAnchors[anchorID] {
				return result, actionCounts, fmt.Errorf("Queue外anchorまたは複数decisionです: %s", anchorID)
			}
			seenAnchors[anchorID] = true
			binding, mapping := bindingByAnchor[anchorID], mappingByAnchor[anchorID]
			if binding == nil || mapping == nil || stringValue(binding["document_id"]) != expected.documentID || !reviewDocumentBindingMatches(binding, expected) ||
				stringValue(binding["locked_source_digest"]) != expected.lockedSourceDigest || stringValue(binding["inventory_tool_digest"]) != bodyToolDigest || stringValue(binding["review_queue_tool_digest"]) != queueToolDigest ||
				stringValue(binding["locator"]) != expected.locator || int(numberValue(binding["context_start"])) != expected.contextStart || int(numberValue(binding["context_end"])) != expected.contextEnd || stringValue(binding["context_unit"]) != expected.contextUnit || stringValue(binding["context_digest"]) != expected.contextDigest {
				return result, actionCounts, fmt.Errorf("Decision source/tool bindingがQueue itemと一致しません: %s", anchorID)
			}
			newIDs := sortedAnyStrings(anySlice(mapping["new_item_ids"]))
			mappingSets[strings.Join(newIDs, "\x00")] = true
			for _, id := range newIDs {
				mappedIDs[id] = true
			}
		}
		resultItems := map[string]string{}
		for _, rawItem := range anySlice(decision["result_items"]) {
			item, _ := rawItem.(map[string]any)
			id := stringValue(item["id"])
			if resultItems[id] != "" {
				return result, actionCounts, fmt.Errorf("Decision result itemが重複しています: %s", id)
			}
			resultItems[id] = stringValue(item["item_type"])
		}
		if len(mappedIDs) != len(resultItems) {
			return result, actionCounts, fmt.Errorf("Decision mappingとSurface/Atomic resultが一致しません: %s", decisionID)
		}
		for id := range mappedIDs {
			if resultItems[id] == "" {
				return result, actionCounts, fmt.Errorf("Decision mappingとSurface/Atomic resultが一致しません: %s", decisionID)
			}
			if owner := newOwners[id]; owner != "" && owner != decisionID {
				return result, actionCounts, fmt.Errorf("new item IDが複数decisionで不正共有されています: %s", id)
			}
			newOwners[id] = decisionID
			if resultItems[id] == "surface" {
				result.SurfaceIDs[id] = true
			} else {
				result.AtomicBehaviorIDs[id] = true
			}
		}
		switch action {
		case "exclude", "defer":
			if len(mappedIDs) != 0 || len(resultItems) != 0 {
				return result, actionCounts, fmt.Errorf("%sは新itemを生成できません: %s", action, decisionID)
			}
		case "include":
			if len(mappedIDs) == 0 || len(mappedIDs) != totalMappingIDs(mappings) {
				return result, actionCounts, fmt.Errorf("includeのnew ID共有にはmerge decisionが必要です: %s", decisionID)
			}
		case "merge":
			if len(anchorIDs) < 2 || len(mappedIDs) == 0 || len(mappingSets) != 1 {
				return result, actionCounts, fmt.Errorf("merge mappingが不正です: %s", decisionID)
			}
		case "split":
			if len(anchorIDs) != 1 || len(mappedIDs) < 2 {
				return result, actionCounts, fmt.Errorf("split mappingが不正です: %s", decisionID)
			}
		}
		result.HumanReviewed += len(anchorIDs)
		if action == "defer" {
			result.Deferred += len(anchorIDs)
		}
		actionCounts[action]++
	}
	return result, actionCounts, nil
}

func sortedAnyStrings(values []any) []string {
	result := make([]string, 0, len(values))
	for _, raw := range values {
		result = append(result, stringValue(raw))
	}
	sort.Strings(result)
	return result
}

func totalMappingIDs(mappings []any) int {
	total := 0
	for _, raw := range mappings {
		mapping, _ := raw.(map[string]any)
		total += len(anySlice(mapping["new_item_ids"]))
	}
	return total
}

func authorityReviewFingerprint(result AuthorityReviewResult) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%d:%d:%t", result.Status, result.QueuedAnchors, result.PendingHuman, result.HumanReviewed, result.Deferred, result.StaleHolds+result.UnavailableHolds, result.Decisions, result.AuthoritySemanticsExhaustive)
}

func reviewQueueFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil, fmt.Errorf("Authority Review Queue directoryに許可されないentryがあります: %s", entry.Name())
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	return files, nil
}
