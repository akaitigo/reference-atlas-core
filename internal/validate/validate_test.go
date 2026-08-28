package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamplesValidate(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{
		filepath.Join(root, "catalog", "stage1.yaml"),
		filepath.Join(root, "examples", "company-inventory.yaml"),
		filepath.Join(root, "examples", "frontend-behavior-atlas", "atlas.yaml"),
		filepath.Join(root, "examples", "frontend-behavior-atlas", "coverage.yaml"),
		filepath.Join(root, "examples", "frontend-behavior-atlas", "sources.lock.yaml"),
		filepath.Join(root, "examples", "frontend-behavior-atlas", "skill.package.yaml"),
		filepath.Join(root, "examples", "frontend-behavior-atlas", "sample.evidence.yaml"),
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := File(path); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		})
	}
}

func TestCoveredTargetRequiresEvidence(t *testing.T) {
	document := `
schema_version: 1
atlas_id: test-atlas
epoch: "2026-08-28"
authority_lock_digest: sha256:0000000000000000000000000000000000000000000000000000000000000000
target_sets:
  - id: core
    title: Core
    sequence: 1
    completion_required: true
    exit_criteria: ["first criterion is long enough", "second criterion is long enough"]
targets:
  - id: core.example
    title: Example
    target_set: core
    kind: capability
    requirement: required
    state: covered
    rationale: This rationale is sufficiently long.
    claim_ids: []
    evidence_ids: []
`
	path := filepath.Join(t.TempDir(), "coverage.yaml")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := File(path)
	if err == nil || !strings.Contains(err.Error(), "claim_ids") {
		t.Fatalf("covered制約違反を期待しました: %v", err)
	}
}
