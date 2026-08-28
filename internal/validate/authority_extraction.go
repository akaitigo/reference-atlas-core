package validate

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
)

type AuthorityExtractionResult struct {
	AtlasID                         string
	Status                          string
	LockedSources                   int
	MatchedSources                  int
	StaleSources                    int
	FailedSources                   int
	CandidateEdges                  int
	ClassifiedReferenceEdges        int
	UnclassifiedReferenceEdges      int
	MissingLocators                 int
	DeferredLocators                int
	AuthorityTextSurfacesExhaustive bool
	HumanReviewedSurfaces           int
	CoreV2EligibleSurfaces          int
}

// AuditAuthorityExtraction validates the metadata-only extraction artifacts.
// It deliberately does not treat classified Domain reference edges as an
// exhaustive inventory of surfaces found in the Authority text.
func AuditAuthorityExtraction(dir string, requireEligible bool) (AuthorityExtractionResult, error) {
	sourcesPath := filepath.Join(dir, "sources.lock.yaml")
	if _, err := File(sourcesPath); err != nil {
		return AuthorityExtractionResult{}, err
	}
	sourcesDoc, err := readDocument(sourcesPath)
	if err != nil {
		return AuthorityExtractionResult{}, err
	}
	locked := map[string]map[string]any{}
	for _, raw := range anySlice(sourcesDoc["sources"]) {
		source, _ := raw.(map[string]any)
		locked[stringValue(source["id"])] = source
	}

	snapshotPath := filepath.Join(dir, "authority", "extraction.snapshot.json")
	if _, err := File(snapshotPath); err != nil {
		return AuthorityExtractionResult{}, err
	}
	snapshot, err := readDocument(snapshotPath)
	if err != nil {
		return AuthorityExtractionResult{}, err
	}
	if stringValue(snapshot["atlas_id"]) != stringValue(sourcesDoc["atlas_id"]) {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority Extraction SnapshotのAtlas IDがsources.lockと一致しません")
	}
	if snapshot["body_storage"] != "digest-and-locator-context-digest-only" {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority本文はDigestとLocator metadata以外保存できません")
	}

	indexSources := anySlice(snapshot["sources"])
	if len(indexSources) != len(locked) {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority ExtractionのSource集合がLockと一致しません: locked=%d snapshot=%d", len(locked), len(indexSources))
	}
	expectedFiles := make([]string, 0, len(indexSources))
	seenSources := map[string]bool{}
	seenEdges := map[string]bool{}
	matched, stale, failed := 0, 0, 0
	rootLocators, foundLocators, missingLocators, deferredLocators, edges := 0, 0, 0, 0, 0
	for _, raw := range indexSources {
		index, _ := raw.(map[string]any)
		sourceID := stringValue(index["id"])
		if seenSources[sourceID] {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Extraction Sourceが重複しています: %s", sourceID)
		}
		seenSources[sourceID] = true
		lockedSource := locked[sourceID]
		if lockedSource == nil {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority ExtractionがLock外Sourceを参照しています: %s", sourceID)
		}
		relative := stringValue(index["path"])
		expectedRelative := "authority/surfaces-draft/" + sourceID + ".json"
		if relative != expectedRelative {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Draft pathがSource IDと一致しません: %s", relative)
		}
		expectedFiles = append(expectedFiles, filepath.Base(relative))
		full := filepath.Join(dir, filepath.FromSlash(relative))
		if err := verifyRelativeFileDigest(dir, relative, stringValue(index["digest"]), -1); err != nil {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Draft %s: %w", sourceID, err)
		}
		if _, err := File(full); err != nil {
			return AuthorityExtractionResult{}, err
		}
		draft, err := readDocument(full)
		if err != nil {
			return AuthorityExtractionResult{}, err
		}
		if stringValue(draft["source_id"]) != sourceID ||
			stringValue(draft["source_url"]) != stringValue(lockedSource["url"]) ||
			stringValue(draft["locked_source_digest"]) != stringValue(lockedSource["digest"]) {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority DraftがSource Lock identityと一致しません: %s", sourceID)
		}
		extraction, _ := draft["extraction"].(map[string]any)
		if extraction["body_storage"] != "digest-and-locator-context-digest-only" || extraction["review_status"] != "automated-unreviewed" {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Draftの本文保存またはReview境界が不正です: %s", sourceID)
		}
		if snapshot["tool_digest"] != nil && extraction["tool_digest"] != snapshot["tool_digest"] {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Draftのtool digestがSnapshotと一致しません: %s", sourceID)
		}
		fetch, _ := draft["fetch"].(map[string]any)
		fetchStatus := stringValue(fetch["status"])
		switch fetchStatus {
		case "matched":
			matched++
			if fetch["locked_digest_match"] != true || stringValue(fetch["fetched_digest"]) != stringValue(lockedSource["digest"]) || fetch["error_digest"] != nil {
				return AuthorityExtractionResult{}, fmt.Errorf("matched Authority fetchのDigest境界が不正です: %s", sourceID)
			}
		case "stale":
			stale++
			if fetch["locked_digest_match"] != false || stringValue(fetch["fetched_digest"]) == "" || stringValue(fetch["fetched_digest"]) == stringValue(lockedSource["digest"]) || fetch["error_digest"] != nil {
				return AuthorityExtractionResult{}, fmt.Errorf("stale Authority fetchのDigest境界が不正です: %s", sourceID)
			}
		case "failed":
			failed++
			if fetch["locked_digest_match"] != false || fetch["fetched_digest"] != nil || stringValue(fetch["error_digest"]) == "" {
				return AuthorityExtractionResult{}, fmt.Errorf("failed Authority fetchの記録境界が不正です: %s", sourceID)
			}
		default:
			return AuthorityExtractionResult{}, fmt.Errorf("未知のAuthority fetch状態です: %s", fetchStatus)
		}
		candidates := anySlice(draft["candidate_surfaces"])
		if len(candidates) != int(numberValue(index["candidate_surfaces"])) {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Draftのcandidate数がIndexと一致しません: %s", sourceID)
		}
		locatorCounts := map[string]int{}
		for _, rawCandidate := range candidates {
			candidate, _ := rawCandidate.(map[string]any)
			edgeID := stringValue(candidate["edge_id"])
			if seenEdges[edgeID] {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidate edgeが重複しています: %s", edgeID)
			}
			seenEdges[edgeID] = true
			if stringValue(candidate["source_id"]) != sourceID || stringValue(candidate["reference_url"]) != stringValue(lockedSource["url"]) {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidateがSource Lockと一致しません: %s", edgeID)
			}
			parsed, err := url.Parse(stringValue(candidate["reference_url"]))
			if err != nil {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidate URLが不正です: %s", edgeID)
			}
			expectedLocator := parsed.Fragment
			if expectedLocator == "" {
				expectedLocator = "document-root"
			} else {
				expectedLocator = "#" + expectedLocator
			}
			if stringValue(candidate["locator"]) != expectedLocator {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidate locatorがURLと一致しません: %s", edgeID)
			}
			status := stringValue(candidate["locator_status"])
			locatorCounts[status]++
			located := status == "root-document" || status == "fragment-found"
			hasContext := stringValue(candidate["context_digest"]) != "" &&
				candidate["context_start"] != nil && candidate["context_end"] != nil &&
				candidate["context_unit"] == "utf16-code-unit"
			if located != hasContext {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidateのLocator context境界が不正です: %s", edgeID)
			}
			if located && numberValue(candidate["context_end"]) <= numberValue(candidate["context_start"]) {
				return AuthorityExtractionResult{}, fmt.Errorf("Authority candidateのLocator offsetが不正です: %s", edgeID)
			}
			switch status {
			case "root-document":
				rootLocators++
				if expectedLocator != "document-root" || fetchStatus != "matched" || stringValue(candidate["context_digest"]) != stringValue(fetch["fetched_digest"]) {
					return AuthorityExtractionResult{}, fmt.Errorf("Authority root locator境界が不正です: %s", edgeID)
				}
			case "fragment-found":
				foundLocators++
				if expectedLocator == "document-root" || fetchStatus != "matched" {
					return AuthorityExtractionResult{}, fmt.Errorf("Authority fragment locator境界が不正です: %s", edgeID)
				}
			case "fragment-not-found":
				missingLocators++
				if fetchStatus != "matched" {
					return AuthorityExtractionResult{}, fmt.Errorf("Authority missing locatorのfetch状態が不正です: %s", edgeID)
				}
			case "not-evaluated-stale-body":
				deferredLocators++
				if fetchStatus != "stale" {
					return AuthorityExtractionResult{}, fmt.Errorf("Authority stale locator deferred状態が不正です: %s", edgeID)
				}
			case "not-evaluated-fetch-failed":
				deferredLocators++
				if fetchStatus != "failed" {
					return AuthorityExtractionResult{}, fmt.Errorf("Authority failed locator deferred状態が不正です: %s", edgeID)
				}
			}
			edges++
		}
		indexedCounts, _ := index["locator_status"].(map[string]any)
		if !sameCounts(locatorCounts, indexedCounts) || index["locked_digest_match"] != fetch["locked_digest_match"] {
			return AuthorityExtractionResult{}, fmt.Errorf("Authority Extraction index recordがDraft実体と一致しません: %s", sourceID)
		}
	}
	if len(seenSources) != len(locked) {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority Source LockにDraftがないSourceがあります")
	}
	actualFiles, err := authorityDraftFiles(filepath.Join(dir, "authority", "surfaces-draft"))
	if err != nil {
		return AuthorityExtractionResult{}, err
	}
	sort.Strings(expectedFiles)
	if !sameStrings(actualFiles, expectedFiles) {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority Draft file集合がSource Lockと一致しません")
	}

	summary, _ := snapshot["summary"].(map[string]any)
	result := AuthorityExtractionResult{
		AtlasID: stringValue(snapshot["atlas_id"]), Status: stringValue(snapshot["status"]),
		LockedSources: len(locked), MatchedSources: matched, StaleSources: stale, FailedSources: failed,
		CandidateEdges: edges, ClassifiedReferenceEdges: int(numberValue(summary["reference_edges_classified"])),
		UnclassifiedReferenceEdges: int(numberValue(summary["unclassified_reference_edges"])),
		MissingLocators:            missingLocators, DeferredLocators: deferredLocators,
		AuthorityTextSurfacesExhaustive: summary["authority_text_surfaces_exhaustive"] == true,
		HumanReviewedSurfaces:           int(numberValue(summary["human_reviewed_surfaces"])),
		CoreV2EligibleSurfaces:          int(numberValue(summary["core_v2_eligible_surfaces"])),
	}
	if int(numberValue(summary["locked_sources"])) != len(locked) ||
		int(numberValue(summary["fetched_digest_matched"])) != matched ||
		int(numberValue(summary["fetched_digest_stale"])) != stale ||
		int(numberValue(summary["fetch_failed"])) != failed ||
		int(numberValue(summary["candidate_surfaces"])) != edges ||
		int(numberValue(summary["root_locators"])) != rootLocators ||
		int(numberValue(summary["fragments_found"])) != foundLocators ||
		int(numberValue(summary["fragments_not_found"])) != missingLocators ||
		int(numberValue(summary["locator_evaluations_deferred"])) != deferredLocators {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority Extraction summaryがDraft実体と一致しません")
	}
	if result.ClassifiedReferenceEdges+result.UnclassifiedReferenceEdges != edges {
		return AuthorityExtractionResult{}, fmt.Errorf("Authority reference edge分類数がcandidate実体と一致しません")
	}
	if requireEligible {
		if err := requireEligibleAuthorityExtraction(result); err != nil {
			return AuthorityExtractionResult{}, err
		}
	}
	return result, nil
}

func requireEligibleAuthorityExtraction(result AuthorityExtractionResult) error {
	if result.StaleSources > 0 {
		return fmt.Errorf("Authority Extractionにstale bodyがあります: %d", result.StaleSources)
	}
	if result.FailedSources > 0 {
		return fmt.Errorf("Authority Extractionにfetch failedがあります: %d", result.FailedSources)
	}
	if result.MissingLocators > 0 {
		return fmt.Errorf("Authority Extractionにfragment-not-foundがあります: %d", result.MissingLocators)
	}
	if result.DeferredLocators > 0 {
		return fmt.Errorf("Authority Extractionにlocator evaluation deferredがあります: %d", result.DeferredLocators)
	}
	if result.UnclassifiedReferenceEdges > 0 {
		return fmt.Errorf("Authority Extractionに未分類reference edgeがあります: %d", result.UnclassifiedReferenceEdges)
	}
	if !result.AuthorityTextSurfacesExhaustive {
		return fmt.Errorf("candidate reference edgeの分類完了はAuthority本文全体のSurface exhaustiveを意味しません")
	}
	if result.HumanReviewedSurfaces == 0 {
		return fmt.Errorf("Authority ExtractionのHuman reviewが0件です")
	}
	if result.CoreV2EligibleSurfaces == 0 {
		return fmt.Errorf("Authority ExtractionのCore v2 eligible Surfaceが0件です")
	}
	if result.HumanReviewedSurfaces < result.CoreV2EligibleSurfaces {
		return fmt.Errorf("Core v2 eligible SurfaceがHuman reviewed Surfaceを超えています")
	}
	if result.Status != "eligible-for-core-v2" {
		return fmt.Errorf("Authority Extraction statusがCore v2 eligibleではありません: %s", result.Status)
	}
	return nil
}

func sameCounts(actual map[string]int, expected map[string]any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range actual {
		if int(numberValue(expected[key])) != value {
			return false
		}
	}
	return true
}

func authorityDraftFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			return nil, fmt.Errorf("Authority Draft directoryに許可されないentryがあります: %s", entry.Name())
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	return files, nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
