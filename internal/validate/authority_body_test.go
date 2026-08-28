package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalFEAuthorityBodyDenominatorIsNotDepthOrSurfaceCompletion(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "FE_AUTHORITY_BODY_DENOMINATOR_REFERENCE.json")
	if _, err := File(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := digestBytes(data); got != "sha256:c7a93b40b539f9c6a5207558f1f17e9add0e9ee9788c57f1240c8e6c838070d8" {
		t.Fatalf("FE body denominator reference digestが正本Commitと一致しません: %s", got)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	summary := doc["summary"].(map[string]any)
	if numberValue(summary["unique_documents"]) != 73 || numberValue(summary["matched_documents"]) != 70 || numberValue(summary["stale_documents"]) != 3 ||
		numberValue(summary["anchors"]) != 15963 || numberValue(summary["classified_anchors"]) != 0 || numberValue(summary["unclassified_anchors"]) != 15963 ||
		numberValue(summary["human_reviewed_anchors"]) != 0 || numberValue(summary["core_v2_eligible_artifacts"]) != 0 || summary["authority_semantics_exhaustive"] != false {
		t.Fatalf("FE candidate anchor denominatorの未完状態が変わっています: %v", summary)
	}
}

func TestRawAuthorityBodyAnchorCannotContainPromotedSurface(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "authority", "body-inventory-draft")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	doc := map[string]any{
		"schema_version": 1, "document_id": "document-example", "fetch_url": "https://example.invalid/", "source_ids": []any{"source.one"}, "locked_body_digest": digest,
		"fetch":      map[string]any{"status": "matched", "fetched_digest": digest, "locked_digest_match": true, "http_status": 200, "final_url": "https://example.invalid/", "content_type": "text/html", "fetched_bytes": 1, "error_digest": nil},
		"extraction": map[string]any{"method": "html-semantic-anchor-selector-v1", "tool": "test-tool", "tool_digest": digest, "selector_contract": []any{"document-root"}, "selector_exhaustive_for_locked_body": true, "authority_semantics_exhaustive": false, "review_status": "automated-unreviewed", "body_storage": "digest-locator-and-offset-only"},
		"anchors":    []any{map[string]any{"id": "anchor-root-test", "locator": "document-root", "locator_kind": "document-root", "semantic_kind": "document-root", "tag": "document", "heading_level": nil, "parent_anchor_id": nil, "context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "context_digest": digest, "label_digest": nil, "classification_status": "pending-human", "surface_ids": []any{"forged.surface"}}},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "document-example.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err == nil || !strings.Contains(err.Error(), "surface_ids") {
		t.Fatalf("未Review anchorの直接昇格をSchemaで拒否する必要があります: %v", err)
	}
	anchor := doc["anchors"].([]any)[0].(map[string]any)
	anchor["surface_ids"] = []any{}
	anchor["heading_text"] = "third-party-heading"
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err == nil || !strings.Contains(err.Error(), "additional properties") {
		t.Fatalf("candidate anchorへのheading文字列保存を拒否する必要があります: %v", err)
	}
}

func TestAuthorityReviewDecisionRequiresHumanAttributionAndOldToNewMapping(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "authority", "reviews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	doc := map[string]any{"schema_version": 1, "atlas_id": "sample-atlas", "queue_id": "authority-review-sample", "status": "incomplete-human-review-required",
		"decisions": []any{map[string]any{"decision_id": "decision.one", "action": "include", "anchor_ids": []any{"anchor-one"}, "source_bindings": []any{map[string]any{"anchor_id": "anchor-one", "document_id": "document-one", "document_url": "https://example.invalid/", "locked_source_digest": digest, "inventory_tool_digest": digest, "review_queue_tool_digest": digest, "locator": "document-root", "context_start": 0, "context_end": 1, "context_unit": "byte", "context_digest": digest}}, "rationale": "固定した一次資料を人が確認して分類した具体的理由を十分な長さで記録する。", "reviewed_at": "2026-08-28T00:00:00Z", "review_method": "manual-primary-source", "mapping": []any{map[string]any{"old_anchor_id": "anchor-one", "new_item_ids": []any{"artifact.surface"}}}, "result_items": []any{map[string]any{"id": "artifact.surface", "item_type": "surface"}}}}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "decisions.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := File(path); err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("reviewer欠落を拒否する必要があります: %v", err)
	}
}

func TestAuthorityBodyNonRegressionDoesNotCountRawAnchorsAsCompletion(t *testing.T) {
	old := map[string]any{"source_entries": 84, "unique_documents": 73, "matched_documents": 70, "stale_documents": 3, "failed_documents": 0, "anchors": 15963, "classified_anchors": 0, "unclassified_anchors": 15963, "human_reviewed_anchors": 0, "deferred_anchors": 0, "core_v2_eligible_artifacts": 0, "authority_semantics_exhaustive": false, "baseline_present": true, "stable_documents": map[string]any{"document:a": "sha256:a"}}
	stronger := cloneMap(t, old)
	stronger["matched_documents"] = 73
	stronger["stale_documents"] = 0
	if !authorityBodyStrengthens(old, stronger) {
		t.Fatal("candidate denominatorのSource状態closureを単調強化として許可する必要があります")
	}
	forgedCompletion := cloneMap(t, old)
	forgedCompletion["classified_anchors"], forgedCompletion["unclassified_anchors"], forgedCompletion["human_reviewed_anchors"], forgedCompletion["core_v2_eligible_artifacts"], forgedCompletion["authority_semantics_exhaustive"] = 15963, 0, 15963, 10, true
	if authorityBodyStrengthens(old, forgedCompletion) {
		t.Fatal("candidate denominator内の分類値をHuman ReviewやDepth達成として扱ってはいけません")
	}
	forged := cloneMap(t, old)
	forged["anchors"] = 20000
	forged["unclassified_anchors"] = 20000
	if authorityBodyStrengthens(old, forged) {
		t.Fatal("raw anchor大量生成だけを非退行上の強化として扱ってはいけません")
	}
	deleted := cloneMap(t, old)
	deleted["anchors"] = 15962
	if authorityBodyStrengthens(old, deleted) {
		t.Fatal("candidate anchor denominator縮小を許可してはいけません")
	}
}
