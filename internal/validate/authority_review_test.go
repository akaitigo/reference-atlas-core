package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalFEAuthorityReviewQueueRemainsIncomplete(t *testing.T) {
	queuePath := filepath.Join("..", "..", "profiles", "FE_AUTHORITY_REVIEW_QUEUE_REFERENCE.json")
	decisionPath := filepath.Join("..", "..", "profiles", "FE_AUTHORITY_REVIEW_DECISIONS_REFERENCE.json")
	for path, expected := range map[string]string{queuePath: "sha256:7288e28e7c30b2c5a5fe3b80f7b96c465e105ee6d0b34513a4ea7230ec9a46fd", decisionPath: "sha256:c65f73790aceeaa2069cf988cb53c50d82520d5e3edbce8fc11d9e4c610b4b89"} {
		if _, err := File(path); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if digestBytes(data) != expected {
			t.Fatalf("FE Human Review reference digestが正本Commitと一致しません: %s", path)
		}
	}
	data, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	var queue map[string]any
	if err := json.Unmarshal(data, &queue); err != nil {
		t.Fatal(err)
	}
	summary := queue["summary"].(map[string]any)
	if numberValue(summary["queued_anchors"]) != 15963 || numberValue(summary["pending_human"]) != 15963 || numberValue(summary["human_reviewed"]) != 0 || numberValue(summary["decisions"]) != 0 || summary["authority_semantics_exhaustive"] != false {
		t.Fatalf("FE Review Queueの正直な未完状態が変わっています: %v", summary)
	}
}

func TestReadOnlyReviewExportCannotWriteOrPromote(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "authority", "portal")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	doc := map[string]any{
		"schema_version": 1, "contract_id": "reference-atlas-review-portal-export-v1", "atlas_id": "sample-atlas", "generated_at": "2026-08-28T00:00:00Z", "status": "incomplete-human-review-required", "mode": "read-only", "locale": "ja",
		"schema_path": "authority/portal/review-export.v1.schema.json", "schema_digest": digest, "packet_schema_path": "authority/portal/review-packet.v1.schema.json", "packet_schema_digest": digest,
		"integrity":         map[string]any{"queue_id": "authority-review-sample", "queue_digest": digest, "packet_index_digest": digest, "producer_tool_digest": digest},
		"capabilities":      map[string]any{"open_primary_source_deep_link": true, "show_machine_proposed_clusters": true, "show_decision_state": true, "write_decisions": false, "promote_human_review": false},
		"summary":           map[string]any{"packets": 0, "unique_anchors": 0, "candidate_domain_projections": 0, "deep_links": 0, "pending_human": 0, "human_reviewed": 0, "proposed_clusters": 0, "semantic_decisions_by_export": 0, "stale_document_holds": 0},
		"packet_index_path": "authority/review-packets/priority-0.snapshot.json", "packets": []any{}, "proposed_clusters": []any{}, "stale_holds": []any{},
		"stale_candidate_report": map[string]any{"path": "authority/stale-relock-candidates.json", "digest": digest, "status": "proposal-only-explicit-relock-required", "locked_digests_updated": 0, "human_choices": 0},
		"decision_boundary":      map[string]any{"ledger_path": "authority/reviews/decisions.json", "decisions_observed": 0, "input_owner": "human-reviewer", "export_accepts_writes": false},
	}
	path := filepath.Join(dir, "review-export.v1.json")
	write := func() error {
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	}
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err != nil {
		t.Fatal(err)
	}
	capabilities := doc["capabilities"].(map[string]any)
	capabilities["write_decisions"] = true
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err == nil || !strings.Contains(err.Error(), "write_decisions") {
		t.Fatalf("read-only exportからdecisionを書けてはいけません: %v", err)
	}
	capabilities["write_decisions"], capabilities["promote_human_review"] = false, true
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err == nil || !strings.Contains(err.Error(), "promote_human_review") {
		t.Fatalf("machine exportからHuman Reviewを昇格できてはいけません: %v", err)
	}
}

func TestAuthorityReviewNonRegressionDoesNotCountQueueVolume(t *testing.T) {
	old := map[string]any{"queued_anchors": 10, "pending_human": 10, "human_reviewed": 0, "deferred": 0, "stale_holds": 1, "unavailable_holds": 0, "decisions": 0, "authority_semantics_exhaustive": false, "depth_credit": false, "stable_anchors": map[string]any{"anchor-one": "sha256:one"}, "surface_ids": []any{}, "atomic_behavior_ids": []any{}}
	volume := cloneMap(t, old)
	volume["queued_anchors"] = 1000
	volume["pending_human"] = 1000
	if authorityReviewStrengthens(old, volume) {
		t.Fatal("Queue件数の増加だけを完成度の強化として扱ってはいけません")
	}
	closed := cloneMap(t, old)
	closed["pending_human"], closed["human_reviewed"], closed["stale_holds"], closed["decisions"], closed["authority_semantics_exhaustive"] = 0, 10, 0, 10, true
	closed["surface_ids"] = []any{"artifact.surface"}
	if !authorityReviewStrengthens(old, closed) {
		t.Fatal("人手Review closureと結果IDの単調追加を強化として扱う必要があります")
	}
}
