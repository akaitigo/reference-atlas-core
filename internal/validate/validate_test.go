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

func TestSupplyChainAcceptsMavenAndNpmWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "third_party"), 0o700); err != nil {
		t.Fatal(err)
	}
	thirdParty := `
schema_version: 1
artifacts:
  - {id: kotlin-stdlib, name: org.jetbrains.kotlin:kotlin-stdlib, kind: maven-package, version: 2.4.10, source: "pkg:maven/org.jetbrains.kotlin/kotlin-stdlib@2.4.10", license: Apache-2.0, redistribution: metadata-only}
  - {id: lru-cache, name: lru-cache, kind: npm-package, version: 10.4.3, source: "pkg:npm/lru-cache@10.4.3", license: ISC, redistribution: metadata-only}
`
	sbom := `{
  "spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT",
  "name":"test","documentNamespace":"https://example.invalid/sbom",
  "packages":[
    {"name":"test-atlas","SPDXID":"SPDXRef-Root","versionInfo":"1.0.0","licenseConcluded":"Apache-2.0","licenseDeclared":"Apache-2.0"},
    {"name":"org.jetbrains.kotlin:kotlin-stdlib","SPDXID":"SPDXRef-Kotlin","versionInfo":"2.4.10","licenseConcluded":"Apache-2.0","licenseDeclared":"Apache-2.0"},
    {"name":"lru-cache","SPDXID":"SPDXRef-Lru","versionInfo":"10.4.3","licenseConcluded":"ISC","licenseDeclared":"ISC"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "third_party", "manifest.yaml"), []byte(thirdParty), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sbom.spdx.json"), []byte(sbom), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := &auditContext{dir: dir, documents: map[string]map[string]any{"atlas": {"id": "test-atlas"}}}
	if err := auditSupplyChain(ctx, "sbom.spdx.json", "third_party/manifest.yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestSupplyChainRequiresGoModOnlyForGoModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "third_party"), 0o700); err != nil {
		t.Fatal(err)
	}
	thirdParty := `
schema_version: 1
artifacts:
  - {id: yaml-v3, name: gopkg.in/yaml.v3, kind: go-module, version: v3.0.1, source: "https://gopkg.in/yaml.v3", license: MIT, redistribution: allowed}
`
	sbom := `{
  "packages":[
    {"name":"test-atlas","versionInfo":"1.0.0","licenseDeclared":"Apache-2.0"},
    {"name":"gopkg.in/yaml.v3","versionInfo":"3.0.1","licenseDeclared":"MIT"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "third_party", "manifest.yaml"), []byte(thirdParty), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sbom.spdx.json"), []byte(sbom), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := &auditContext{dir: dir, documents: map[string]map[string]any{"atlas": {"id": "test-atlas"}}}
	err := auditSupplyChain(ctx, "sbom.spdx.json", "third_party/manifest.yaml")
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("go-module宣言にはgo.mod拒否を期待しました: %v", err)
	}
}
