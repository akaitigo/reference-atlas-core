package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalFEAuthorityExtractionReferenceIsFailClosed(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "FE_AUTHORITY_EXTRACTION_REFERENCE.json")
	if _, err := File(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := shaDigest(data); got != "sha256:4708796bd7c05c3ae750c87a9d01291f66ea2a7cc1f54731172638bfffd9a629" {
		t.Fatalf("FE Authority Extraction reference digestが正本Commitと一致しません: %s", got)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	summary := doc["summary"].(map[string]any)
	if doc["status"] != "incomplete-human-review-required" ||
		numberValue(summary["locked_sources"]) != 84 ||
		numberValue(summary["reference_edges_classified"]) != 235 ||
		numberValue(summary["unclassified_reference_edges"]) != 0 ||
		summary["authority_text_surfaces_exhaustive"] != false ||
		numberValue(summary["human_reviewed_surfaces"]) != 0 ||
		numberValue(summary["core_v2_eligible_surfaces"]) != 0 {
		t.Fatalf("FE参照の正直な未完状態が変わっています: status=%v summary=%v", doc["status"], summary)
	}
}

func TestAuthorityEligibilityKeepsOpenStatesIndependent(t *testing.T) {
	base := AuthorityExtractionResult{
		Status: "incomplete-human-review-required", LockedSources: 84, MatchedSources: 81,
		StaleSources: 3, CandidateEdges: 235, ClassifiedReferenceEdges: 235,
		DeferredLocators: 4, AuthorityTextSurfacesExhaustive: false,
	}
	cases := []struct {
		name string
		edit func(*AuthorityExtractionResult)
		want string
	}{
		{"stale", func(*AuthorityExtractionResult) {}, "stale body"},
		{"fetch-failed", func(r *AuthorityExtractionResult) { r.StaleSources = 0; r.FailedSources = 1 }, "fetch failed"},
		{"locator-missing", func(r *AuthorityExtractionResult) { r.StaleSources = 0; r.MissingLocators = 1 }, "fragment-not-found"},
		{"locator-deferred", func(r *AuthorityExtractionResult) { r.StaleSources = 0 }, "locator evaluation deferred"},
		{"unclassified-edge", func(r *AuthorityExtractionResult) {
			r.StaleSources = 0
			r.DeferredLocators = 0
			r.UnclassifiedReferenceEdges = 1
		}, "未分類reference edge"},
		{"not-exhaustive", func(r *AuthorityExtractionResult) { r.StaleSources = 0; r.DeferredLocators = 0 }, "本文全体"},
		{"human-review-zero", func(r *AuthorityExtractionResult) {
			r.StaleSources = 0
			r.DeferredLocators = 0
			r.AuthorityTextSurfacesExhaustive = true
		}, "Human reviewが0"},
		{"eligible-zero", func(r *AuthorityExtractionResult) {
			r.StaleSources = 0
			r.DeferredLocators = 0
			r.AuthorityTextSurfacesExhaustive = true
			r.HumanReviewedSurfaces = 1
		}, "eligible Surfaceが0"},
		{"status-incomplete", func(r *AuthorityExtractionResult) {
			r.StaleSources = 0
			r.DeferredLocators = 0
			r.AuthorityTextSurfacesExhaustive = true
			r.HumanReviewedSurfaces = 1
			r.CoreV2EligibleSurfaces = 1
		}, "statusがCore v2 eligible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.edit(&candidate)
			err := requireEligibleAuthorityExtraction(candidate)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("独立した未完状態をfail-closedにできていません: want=%q err=%v", tc.want, err)
			}
		})
	}
	passing := base
	passing.StaleSources = 0
	passing.DeferredLocators = 0
	passing.AuthorityTextSurfacesExhaustive = true
	passing.HumanReviewedSurfaces = 1
	passing.CoreV2EligibleSurfaces = 1
	passing.Status = "eligible-for-core-v2"
	if err := requireEligibleAuthorityExtraction(passing); err != nil {
		t.Fatalf("全独立状態を閉じたExtractionを受理できません: %v", err)
	}
}

func TestAuthorityDraftSchemaRejectsThirdPartyTextFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "authority", "surfaces-draft")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{
		"schema_version":       1,
		"source_id":            "example-source",
		"source_url":           "https://example.invalid/spec",
		"locked_source_digest": "sha256:" + strings.Repeat("0", 64),
		"fetch": map[string]any{
			"status": "matched", "fetched_digest": "sha256:" + strings.Repeat("0", 64), "locked_digest_match": true,
			"http_status": 200, "final_url": "https://example.invalid/spec", "content_type": "text/html", "fetched_bytes": 1, "error_digest": nil,
		},
		"extraction": map[string]any{
			"method": "locked-body-locator-context-digest", "tool": "test-extractor",
			"review_status": "automated-unreviewed", "body_storage": "digest-and-locator-context-digest-only",
		},
		"candidate_surfaces": []any{},
	}
	for _, forbidden := range []string{"body", "text", "excerpt", "heading", "content", "html"} {
		t.Run(forbidden, func(t *testing.T) {
			doc := make(map[string]any, len(base)+1)
			for key, value := range base {
				doc[key] = value
			}
			doc[forbidden] = "third-party-text"
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, forbidden+".json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := File(path); err == nil || !strings.Contains(err.Error(), "additional properties") {
				t.Fatalf("本文field %qを拒否する必要があります: %v", forbidden, err)
			}
		})
	}
}

func TestAuthorityDraftSchemaRejectsNestedTextAndUnknownKeys(t *testing.T) {
	base := map[string]any{
		"schema_version":       1,
		"source_id":            "example-source",
		"source_url":           "https://example.invalid/spec#section",
		"locked_source_digest": "sha256:" + strings.Repeat("0", 64),
		"fetch": map[string]any{
			"status": "matched", "fetched_digest": "sha256:" + strings.Repeat("0", 64), "locked_digest_match": true,
			"http_status": 200, "final_url": "https://example.invalid/spec", "content_type": "text/html", "fetched_bytes": 1, "error_digest": nil,
		},
		"extraction": map[string]any{
			"method": "locked-body-locator-context-digest", "tool": "test-extractor",
			"review_status": "automated-unreviewed", "body_storage": "digest-and-locator-context-digest-only",
		},
		"candidate_surfaces": []any{map[string]any{
			"edge_id": "edge.one", "source_id": "example-source", "reference_url": "https://example.invalid/spec#section",
			"locator": "#section", "pattern_id": "group/pattern", "pattern_kind": "atomic",
			"candidate_behavior_id": "behavior.one", "capability_id": "capability.one", "target_id": "target.one", "claim_id": "claim.one",
			"variant_ids": []any{"variant.one"}, "surface_ids": []any{"foundations-mechanics"},
			"classification_basis": "domain-contract-projection-unreviewed", "domain_reference_metadata_digest": "sha256:" + strings.Repeat("1", 64),
			"locator_status": "fragment-found", "context_digest": "sha256:" + strings.Repeat("2", 64),
			"context_start": 0, "context_end": 1, "context_unit": "utf16-code-unit", "heading_digest": "sha256:" + strings.Repeat("3", 64),
			"classification": "candidate-included-unreviewed",
		}},
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"fetch-body", func(doc map[string]any) { doc["fetch"].(map[string]any)["response_body"] = "third-party-text" }},
		{"extraction-excerpt", func(doc map[string]any) { doc["extraction"].(map[string]any)["excerpt"] = "third-party-text" }},
		{"candidate-heading", func(doc map[string]any) {
			doc["candidate_surfaces"].([]any)[0].(map[string]any)["heading_text"] = "third-party-text"
		}},
		{"candidate-unknown", func(doc map[string]any) {
			doc["candidate_surfaces"].([]any)[0].(map[string]any)["unreviewed_note"] = "unknown"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := cloneMap(t, base)
			tc.mutate(doc)
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(t.TempDir(), "authority", "surfaces-draft")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := File(path); err == nil || !strings.Contains(err.Error(), "additional properties") {
				t.Fatalf("入れ子の本文またはallowlist外keyを拒否する必要があります: %v", err)
			}
		})
	}
}

func TestAuthorityExtractionNonRegressionAllowsOnlyMonotonicState(t *testing.T) {
	old := map[string]any{
		"locked_sources": 1, "candidate_surfaces": 1, "reference_edges_classified": 1,
		"fetched_digest_matched": 0, "fetched_digest_stale": 1, "fetch_failed": 0,
		"fragments_not_found": 0, "locator_evaluations_deferred": 1, "unclassified_reference_edges": 0,
		"human_reviewed_surfaces": 0, "core_v2_eligible_surfaces": 0, "status_rank": 1,
		"authority_text_surfaces_exhaustive": false,
		"stable_contracts":                   map[string]any{"source:s": "sha256:stable", "edge:e": "sha256:edge"},
		"fetch_ranks":                        map[string]any{"s": 1}, "locator_ranks": map[string]any{"e": 0},
		"fetch_state_fingerprints": map[string]any{"s": "sha256:stale"}, "locator_state_fingerprints": map[string]any{"e": "sha256:deferred"},
	}
	stronger := cloneMap(t, old)
	stronger["fetched_digest_matched"] = 1
	stronger["fetched_digest_stale"] = 0
	stronger["locator_evaluations_deferred"] = 0
	stronger["human_reviewed_surfaces"] = 1
	stronger["core_v2_eligible_surfaces"] = 1
	stronger["status_rank"] = 2
	stronger["authority_text_surfaces_exhaustive"] = true
	stronger["fetch_ranks"].(map[string]any)["s"] = 2
	stronger["locator_ranks"].(map[string]any)["e"] = 2
	stronger["fetch_state_fingerprints"].(map[string]any)["s"] = "sha256:matched"
	stronger["locator_state_fingerprints"].(map[string]any)["e"] = "sha256:located"
	if !authorityExtractionStrengthens(old, stronger) {
		t.Fatal("stale/deferredからmatched/located/reviewed/eligibleへの単調強化を許可する必要があります")
	}

	worsened := cloneMap(t, stronger)
	worsened["authority_text_surfaces_exhaustive"] = false
	if authorityExtractionStrengthens(stronger, worsened) {
		t.Fatal("Authority本文exhaustiveをtrueからfalseへ後退できてはいけません")
	}
	overwrittenFailure := cloneMap(t, old)
	overwrittenFailure["fetch_state_fingerprints"].(map[string]any)["s"] = "sha256:another-stale"
	if authorityExtractionStrengthens(old, overwrittenFailure) {
		t.Fatal("同じstale rankの記録を上書きできてはいけません")
	}
	contractRemoved := cloneMap(t, stronger)
	delete(contractRemoved["stable_contracts"].(map[string]any), "edge:e")
	if authorityExtractionStrengthens(stronger, contractRemoved) {
		t.Fatal("Authority candidate edge contractを削除できてはいけません")
	}
}

func cloneMap(t *testing.T, source map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
