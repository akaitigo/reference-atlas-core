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
		filepath.Join(root, "examples", "frontend-behavior-atlas", "mastery.yaml"),
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

func TestExampleAudit(t *testing.T) {
	root := filepath.Join("..", "..")
	result, err := AuditDir(filepath.Join(root, "examples", "frontend-behavior-atlas"))
	if err != nil {
		t.Fatal(err)
	}
	if result.MasteryAreas != 14 || result.OpenRequired == 0 {
		t.Fatalf("予期しない監査結果: %+v", result)
	}
}

func TestV1CompatibilityFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "frontend-behavior-atlas")
	for _, name := range []string{"atlas.yaml", "mastery.yaml", "coverage.yaml", "sources.lock.yaml", "skill.package.yaml", "sample.evidence.yaml"} {
		if _, err := File(filepath.Join(root, name)); err != nil {
			t.Fatalf("既存v1 fixtureとの互換性が壊れました (%s): %v", name, err)
		}
	}
}

func TestRelativeDigestRejectsTraversalAndTampering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.txt")
	if err := os.WriteFile(path, []byte("verified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelativeFileDigest(dir, "artifact.txt", digestBytes([]byte("verified\n")), 9); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelativeFileDigest(dir, "../artifact.txt", digestBytes([]byte("verified\n")), 9); err == nil {
		t.Fatal("Path traversalは拒否する必要があります")
	}
	if err := verifyRelativeFileDigest(dir, "artifact.txt", digestBytes([]byte("tampered\n")), 9); err == nil {
		t.Fatal("改変Digestは拒否する必要があります")
	}
}
