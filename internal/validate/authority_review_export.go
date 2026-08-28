package validate

import (
	"fmt"
	"os"
	"path/filepath"
)

type AuthorityReviewExportResult struct {
	AtlasID           string
	Packets           int
	UniqueAnchors     int
	DomainProjections int
	MachineProposals  int
	HumanDecisions    int
	StaleHolds        int
}

// AuditAuthorityReviewExport verifies a read-only projection of the human
// review queue. Packets and proposals are navigation aids, never decisions or
// Completion/Depth credit.
func AuditAuthorityReviewExport(dir string) (AuthorityReviewExportResult, error) {
	if _, err := AuditAuthorityRelock(dir); err != nil {
		return AuthorityReviewExportResult{}, err
	}
	exportPath := filepath.Join(dir, "authority", "portal", "review-export.v1.json")
	if _, err := File(exportPath); err != nil {
		return AuthorityReviewExportResult{}, err
	}
	export, err := readDocument(exportPath)
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	review, err := AuditAuthorityReviewQueue(dir, false)
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	queuePath := filepath.Join(dir, "authority", "review-queue.snapshot.json")
	queueData, err := os.ReadFile(queuePath)
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	integrity, _ := export["integrity"].(map[string]any)
	queue, err := readDocument(queuePath)
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	if stringValue(export["atlas_id"]) != review.AtlasID || stringValue(integrity["queue_id"]) != stringValue(queue["queue_id"]) || stringValue(integrity["queue_digest"]) != digestBytes(queueData) {
		return AuthorityReviewExportResult{}, fmt.Errorf("read-only Review ExportのQueue integrityが一致しません")
	}
	for _, pair := range [][2]string{{stringValue(export["schema_path"]), stringValue(export["schema_digest"])}, {stringValue(export["packet_schema_path"]), stringValue(export["packet_schema_digest"])}} {
		if err := verifyRelativeFileDigest(dir, pair[0], pair[1], -1); err != nil {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review Export schema integrity: %w", err)
		}
	}
	indexRelative := stringValue(export["packet_index_path"])
	if err := verifyRelativeFileDigest(dir, indexRelative, stringValue(integrity["packet_index_digest"]), -1); err != nil {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review packet index: %w", err)
	}
	packetIndex, err := readDocument(filepath.Join(dir, filepath.FromSlash(indexRelative)))
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	if stringValue(packetIndex["atlas_id"]) != review.AtlasID || stringValue(packetIndex["queue_id"]) != stringValue(queue["queue_id"]) || stringValue(packetIndex["queue_digest"]) != digestBytes(queueData) {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review packet indexがQueueへ固定されていません")
	}

	anchors, _, _, _, bodyToolDigest, _, err := loadAuthorityReviewInputs(dir)
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	queueToolDigest := stringValue(queue["tool_digest"])
	ledger, err := readDocument(filepath.Join(dir, "authority", "reviews", "decisions.json"))
	if err != nil {
		return AuthorityReviewExportResult{}, err
	}
	decisionByAnchor := map[string]string{}
	for _, raw := range anySlice(ledger["decisions"]) {
		decision, _ := raw.(map[string]any)
		for _, rawID := range anySlice(decision["anchor_ids"]) {
			decisionByAnchor[stringValue(rawID)] = stringValue(decision["decision_id"])
		}
	}

	exportRecords := anySlice(export["packets"])
	indexRecords := anySlice(packetIndex["packets"])
	if !sameCanonicalLists(exportRecords, indexRecords) {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review Exportとpacket indexのpacket集合が一致しません")
	}
	seenPackets, seenAnchors := map[string]bool{}, map[string]bool{}
	projectionCount, deepLinks, pending, human := 0, 0, 0, 0
	referencedProposals := map[string]bool{}
	for _, raw := range exportRecords {
		record, _ := raw.(map[string]any)
		packetID, relative := stringValue(record["id"]), stringValue(record["path"])
		if seenPackets[packetID] || filepath.Base(relative) != packetID+".json" {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review packet identityが不正です: %s", packetID)
		}
		seenPackets[packetID] = true
		if err := verifyRelativeFileDigest(dir, relative, stringValue(record["digest"]), -1); err != nil {
			return AuthorityReviewExportResult{}, err
		}
		if _, err := File(filepath.Join(dir, filepath.FromSlash(relative))); err != nil {
			return AuthorityReviewExportResult{}, err
		}
		packet, err := readDocument(filepath.Join(dir, filepath.FromSlash(relative)))
		if err != nil {
			return AuthorityReviewExportResult{}, err
		}
		binding, _ := packet["source_binding"].(map[string]any)
		anchorID := stringValue(binding["anchor_id"])
		expected, ok := anchors[anchorID]
		if !ok || seenAnchors[anchorID] || stringValue(packet["packet_id"]) != packetID || stringValue(packet["queue_id"]) != stringValue(queue["queue_id"]) || stringValue(record["anchor_id"]) != anchorID {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review packetがQueue anchorへ一意に固定されていません: %s", packetID)
		}
		seenAnchors[anchorID] = true
		if err := compareAuthorityReviewBinding(binding, expected, bodyToolDigest, queueToolDigest); err != nil {
			return AuthorityReviewExportResult{}, err
		}
		projections := len(anySlice(packet["candidate_domain_projections"]))
		if projections != int(numberValue(record["candidate_edges"])) {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review packet projection数がIndexと一致しません: %s", packetID)
		}
		projectionCount += projections
		if stringValue(record["deep_link"]) != stringValue(packet["deep_link"].(map[string]any)["url"]) {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review packet deep-linkがIndexと一致しません: %s", packetID)
		}
		deepLinks++
		decisionBoundary, _ := packet["decision_boundary"].(map[string]any)
		decisionIDs := sortedAnyStrings(anySlice(decisionBoundary["decision_ids"]))
		if decisionID := decisionByAnchor[anchorID]; decisionID == "" {
			if packet["status"] != "pending-human" || decisionBoundary["human_decision_recorded"] != false || len(decisionIDs) != 0 {
				return AuthorityReviewExportResult{}, fmt.Errorf("machine packetをHuman decisionとして扱えません: %s", packetID)
			}
			pending++
		} else {
			if packet["status"] != "human-reviewed" || decisionBoundary["human_decision_recorded"] != true || len(decisionIDs) != 1 || decisionIDs[0] != decisionID {
				return AuthorityReviewExportResult{}, fmt.Errorf("Review packetのHuman decision表示がLedgerと一致しません: %s", packetID)
			}
			human++
		}
		for _, rawID := range anySlice(packet["proposed_cluster_ids"]) {
			referencedProposals[stringValue(rawID)] = true
		}
	}
	proposals := anySlice(export["proposed_clusters"])
	if !sameCanonicalLists(proposals, anySlice(packetIndex["proposed_clusters"])) {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review Exportとpacket indexのmachine proposal集合が一致しません")
	}
	proposalIDs := map[string]bool{}
	for _, raw := range proposals {
		proposal, _ := raw.(map[string]any)
		id := stringValue(proposal["id"])
		if proposalIDs[id] || proposal["semantic_decision"] != "none-machine-proposal-only" || proposal["human_reviewed"] != false {
			return AuthorityReviewExportResult{}, fmt.Errorf("machine proposalとHuman decisionの型境界が不正です: %s", id)
		}
		proposalIDs[id] = true
	}
	for id := range referencedProposals {
		if !proposalIDs[id] {
			return AuthorityReviewExportResult{}, fmt.Errorf("Review packetが未定義machine proposalを参照しています: %s", id)
		}
	}
	if !sameCanonicalLists(anySlice(export["stale_holds"]), anySlice(queue["stale_holds"])) {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review Exportのstale holdがQueueと一致しません")
	}
	staleReport, _ := export["stale_candidate_report"].(map[string]any)
	if err := verifyRelativeFileDigest(dir, stringValue(staleReport["path"]), stringValue(staleReport["digest"]), -1); err != nil {
		return AuthorityReviewExportResult{}, err
	}
	if _, err := File(filepath.Join(dir, filepath.FromSlash(stringValue(staleReport["path"])))); err != nil {
		return AuthorityReviewExportResult{}, err
	}
	summary, _ := export["summary"].(map[string]any)
	if int(numberValue(summary["packets"])) != len(seenPackets) || int(numberValue(summary["unique_anchors"])) != len(seenAnchors) || int(numberValue(summary["candidate_domain_projections"])) != projectionCount || int(numberValue(summary["deep_links"])) != deepLinks || int(numberValue(summary["pending_human"])) != pending || int(numberValue(summary["human_reviewed"])) != human || int(numberValue(summary["proposed_clusters"])) != len(proposalIDs) || int(numberValue(summary["stale_document_holds"])) != review.StaleHolds {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review Export summaryがpacket/proposal/ledger実体と一致しません")
	}
	boundary, _ := export["decision_boundary"].(map[string]any)
	if int(numberValue(boundary["decisions_observed"])) != review.Decisions {
		return AuthorityReviewExportResult{}, fmt.Errorf("Review Exportのdecision表示がLedgerと一致しません")
	}
	return AuthorityReviewExportResult{AtlasID: review.AtlasID, Packets: len(seenPackets), UniqueAnchors: len(seenAnchors), DomainProjections: projectionCount, MachineProposals: len(proposalIDs), HumanDecisions: review.Decisions, StaleHolds: review.StaleHolds}, nil
}

func sameCanonicalLists(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	leftDigest, leftErr := digestCanonical(left)
	rightDigest, rightErr := digestCanonical(right)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}
