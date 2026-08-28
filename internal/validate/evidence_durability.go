package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvidenceDurabilityResult describes the one complete generation currently
// published under an Artifact directory. Failed attempts are not publishable
// generations and therefore must not be visible in this set.
type EvidenceDurabilityResult struct {
	ReportID  string
	Artifacts int
	Rows      int
	Variants  int
}

func requiredEvidenceDurabilityProfile() map[string]any {
	return map[string]any{
		"publish_on": "full-run-passed",
		"failed_run": "retain-prior-success",
		"swap":       "staged-directory-rename-with-rollback",
	}
}

// AuditEvidenceDurability verifies the durable publication profile used by a
// dedicated Runtime reporter. The report is the manifest for one generation:
// every referenced Artifact must exist and no unlisted file from an older or
// partial generation may remain below the publication root.
func AuditEvidenceDurability(dir, relative string) (EvidenceDurabilityResult, error) {
	if filepath.IsAbs(relative) || filepath.Clean(relative) == "." || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return EvidenceDurabilityResult{}, fmt.Errorf("Evidence durability Report pathがRepository外を参照しています: %s", relative)
	}
	fullReport := filepath.Join(dir, filepath.FromSlash(relative))
	if filepath.Base(fullReport) != "results.json" {
		return EvidenceDurabilityResult{}, fmt.Errorf("Evidence durability Profileはresults.jsonを正本にする必要があります: %s", relative)
	}
	if _, err := File(fullReport); err != nil {
		return EvidenceDurabilityResult{}, err
	}
	report, err := readDocument(fullReport)
	if err != nil {
		return EvidenceDurabilityResult{}, err
	}
	contract, _ := report["retention_contract"].(map[string]any)
	if contract["publish_on"] != "full-run-passed" || contract["failed_run"] != "retain-prior-success" || contract["swap"] != "staged-directory-rename-with-rollback" {
		return EvidenceDurabilityResult{}, fmt.Errorf("Evidence durability Profileを弱めることはできません")
	}
	if report["status"] != "passed" {
		return EvidenceDurabilityResult{}, fmt.Errorf("failed runを成功Evidence directoryへ公開できません")
	}
	tests := anySlice(report["tests"])
	counts, _ := report["counts"].(map[string]any)
	if len(tests) == 0 || int(numberValue(counts["total"])) != len(tests) || int(numberValue(counts["passed"])) != len(tests) || numberValue(counts["failed"])+numberValue(counts["flaky"])+numberValue(counts["skipped"]) != 0 {
		return EvidenceDurabilityResult{}, fmt.Errorf("full-run pass以外をEvidence generationとして公開できません")
	}

	reportRoot := filepath.Dir(relative)
	expected := map[string]bool{filepath.ToSlash(relative): true}
	rowKeys, variantKeys, recordIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, raw := range tests {
		record, _ := raw.(map[string]any)
		id := stringValue(record["id"])
		if recordIDs[id] {
			return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generationに重複recordがあります: %s", id)
		}
		recordIDs[id] = true
		if record["final_status"] != "passed" || int(numberValue(record["attempts"])) != 1 || record["error"] != nil {
			return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generationにfirst-attempt pass以外のrecordがあります: %s", id)
		}
		rowKeys[stringValue(record["pattern_id"])+"\x00"+stringValue(record["scenario"])] = true
		variantKeys[stringValue(record["pattern_id"])+"\x00"+stringValue(record["variant_id"])] = true
		for _, field := range []string{"trace", "screenshot"} {
			artifact, _ := record[field].(map[string]any)
			path := filepath.ToSlash(stringValue(artifact["path"]))
			if path == "" || !pathWithinArtifactRoot(reportRoot, path) {
				return EvidenceDurabilityResult{}, fmt.Errorf("Evidence Artifactが公開directory外を参照しています: record=%s path=%s", id, path)
			}
			if expected[path] {
				return EvidenceDurabilityResult{}, fmt.Errorf("新旧または複数recordでArtifactを共有できません: %s", path)
			}
			expected[path] = true
			if err := verifyRelativeFileDigest(dir, path, stringValue(artifact["digest"]), int64(numberValue(artifact["bytes"]))); err != nil {
				return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generation Artifact: %w", err)
			}
		}
		trace, _ := record["trace"].(map[string]any)
		if trace["action_stream"] != true || trace["network_stream"] != true || trace["resource_stream"] != true {
			return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generation Trace streamが不足しています: %s", id)
		}
	}
	if int(numberValue(counts["rows"])) != len(rowKeys) || int(numberValue(counts["variants"])) != len(variantKeys) {
		return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generationのrow/variant countが実体と一致しません")
	}

	actual := map[string]bool{}
	root := filepath.Join(dir, filepath.FromSlash(reportRoot))
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Evidence generationにsymlinkを含められません: %s", path)
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		actual[filepath.ToSlash(rel)] = true
		return nil
	}); err != nil {
		return EvidenceDurabilityResult{}, err
	}
	if len(actual) != len(expected) {
		return EvidenceDurabilityResult{}, fmt.Errorf("Evidence directoryが1世代の完全なArtifact集合ではありません: expected=%d actual=%d", len(expected), len(actual))
	}
	for path := range expected {
		if !actual[path] {
			return EvidenceDurabilityResult{}, fmt.Errorf("Evidence generation Artifactが欠落しています: %s", path)
		}
	}
	return EvidenceDurabilityResult{ReportID: stringValue(report["id"]), Artifacts: len(actual), Rows: len(rowKeys), Variants: len(variantKeys)}, nil
}

func pathWithinArtifactRoot(root, candidate string) bool {
	root = filepath.ToSlash(filepath.Clean(filepath.FromSlash(root)))
	candidate = filepath.ToSlash(filepath.Clean(filepath.FromSlash(candidate)))
	return candidate != root && strings.HasPrefix(candidate, root+"/")
}
