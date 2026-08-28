package validate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEvidenceDurabilityReferenceFixtureRetainsThirtyThreeArtifacts(t *testing.T) {
	dir := createEvidenceDurabilityFixture(t)
	result, err := AuditEvidenceDurability(dir, "artifacts/pattern-scenarios/results.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Artifacts != 33 || result.Rows != 8 || result.Variants != 16 {
		t.Fatalf("FE参照と同じ1 Report + 16 Trace + 16 Screenshot集合ではありません: %+v", result)
	}
	before := snapshotPublishedArtifacts(t, dir)
	if len(before) != 33 {
		t.Fatalf("negative fixtureの事前Artifact数が33ではありません: %d", len(before))
	}
	for _, attempt := range []string{"failed-run", "no-match-run"} {
		staging := filepath.Join(dir, "artifacts", ".pattern-scenarios-next")
		if err := os.MkdirAll(staging, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staging, attempt+".partial"), []byte("not publishable\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(staging); err != nil {
			t.Fatal(err)
		}
		after := snapshotPublishedArtifacts(t, dir)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%sが直前成功Evidenceを変更しました", attempt)
		}
	}
}

func TestEvidenceDurabilityRejectsPartialOverwriteSuccessErasureAndMixedGeneration(t *testing.T) {
	cases := []struct {
		name   string
		want   string
		mutate func(*testing.T, string)
	}{
		{name: "partial-overwrite", want: "Digest", mutate: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "artifacts", "pattern-scenarios", "traces", "pattern-00__security__variant-00.trace.zip"), []byte("partial new generation\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "erase-prior-success", want: "Evidence generation Artifact", mutate: func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "artifacts", "pattern-scenarios", "screenshots", "pattern-00__security__variant-00.png")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "mixed-generations", want: "1世代の完全なArtifact集合", mutate: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "artifacts", "pattern-scenarios", "traces", "stale-old-generation.trace.zip"), []byte("stale\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "publish-failed-run", want: "failed run", mutate: func(t *testing.T, dir string) {
			path := filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json")
			doc := mustReadYAML(t, path)
			doc["status"] = "failed"
			mustWriteJSON(t, path, doc)
		}},
		{name: "weaken-swap", want: "pattern-scenario-results.schema.json", mutate: func(t *testing.T, dir string) {
			path := filepath.Join(dir, "artifacts", "pattern-scenarios", "results.json")
			doc := mustReadYAML(t, path)
			doc["retention_contract"].(map[string]any)["swap"] = "direct-file-replacement"
			mustWriteJSON(t, path, doc)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := createEvidenceDurabilityFixture(t)
			tc.mutate(t, dir)
			if _, err := AuditEvidenceDurability(dir, "artifacts/pattern-scenarios/results.json"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Evidence durability違反を拒否できません: want=%q err=%v", tc.want, err)
			}
		})
	}
}

func createEvidenceDurabilityFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "artifacts", "pattern-scenarios")
	if err := os.MkdirAll(filepath.Join(root, "traces"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "screenshots"), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []any{}
	for index := 0; index < 16; index++ {
		pattern := "pattern-" + twoDigits(index/2)
		variant := "variant-" + twoDigits(index)
		base := pattern + "__security__" + variant
		tracePath := "artifacts/pattern-scenarios/traces/" + base + ".trace.zip"
		screenshotPath := "artifacts/pattern-scenarios/screenshots/" + base + ".png"
		traceData, screenshotData := []byte("trace "+base+"\n"), []byte("screenshot "+base+"\n")
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(tracePath)), traceData, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(screenshotPath)), screenshotData, 0o600); err != nil {
			t.Fatal(err)
		}
		tests = append(tests, map[string]any{
			"id": base, "pattern_id": pattern, "variant_id": variant, "scenario": "security", "title": "dedicated security runtime " + base, "file": "scenario_test.go", "line": 1,
			"source_digest": "sha256:" + strings.Repeat("a", 64), "outcome": "expected", "attempts": 1, "final_status": "passed", "error": nil, "oracle": map[string]any{"kind": "security-boundary"},
			"trace":      map[string]any{"path": tracePath, "digest": shaDigest(traceData), "bytes": len(traceData), "action_stream": true, "network_stream": true, "resource_stream": true},
			"screenshot": map[string]any{"path": screenshotPath, "digest": shaDigest(screenshotData), "bytes": len(screenshotData)},
		})
	}
	report := map[string]any{
		"schema_version": 1, "id": "durability-reference-runtime-v1", "created_at": "2026-08-28T00:00:00Z", "status": "passed", "command": "subject runtime full run", "profile": "local-real-runtime",
		"counts":        map[string]any{"rows": 8, "variants": 16, "total": 16, "passed": 16, "failed": 0, "flaky": 0, "skipped": 0},
		"source_digest": "sha256:" + strings.Repeat("b", 64), "harness_digest": "sha256:" + strings.Repeat("c", 64),
		"environment":        map[string]any{"runtime": "runtime-v1", "platform": "test", "architecture": "test", "version": "1", "worker": "single", "workers": 1, "retries": 0, "trace_mode": "on"},
		"retention_contract": requiredEvidenceDurabilityProfile(), "tests": tests,
	}
	mustWriteJSON(t, filepath.Join(root, "results.json"), report)
	return dir
}

func twoDigits(value int) string {
	return string([]byte{'0' + byte(value/10), '0' + byte(value%10)})
}

func snapshotPublishedArtifacts(t *testing.T, dir string) map[string]string {
	t.Helper()
	result := map[string]string{}
	root := filepath.Join(dir, "artifacts", "pattern-scenarios")
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		result[filepath.ToSlash(rel)] = shaDigest(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
