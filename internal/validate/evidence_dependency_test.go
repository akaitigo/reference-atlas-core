package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceDependencyGraphFixture(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	if _, err := File(filepath.Join(dir, "evidence", "dependency-graph.json")); err != nil {
		t.Fatal(err)
	}
	result, err := AuditEvidenceDependencyGraph(dir, "evidence/dependency-graph.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Inputs != 4 || result.Outputs == 0 || result.Runs != 1 {
		t.Fatalf("Evidence dependency fixture countが不正です: %+v", result)
	}
}

func TestEvidenceDependencyRejectsDigestOnlyClosureForEveryInputKind(t *testing.T) {
	for _, inputID := range []string{"source-fixture", "harness-fixture", "runtime-fixture", "profile-fixture"} {
		inputID := inputID
		t.Run(inputID, func(t *testing.T) {
			dir := createDefinitiveRepositoryFixture(t)
			graphPath := filepath.Join(dir, "evidence", "dependency-graph.json")
			graph := mustReadYAML(t, graphPath)
			var changed map[string]any
			for _, raw := range graph["inputs"].([]any) {
				input := raw.(map[string]any)
				if input["id"] == inputID {
					changed = input
					break
				}
			}
			member := changed["members"].([]any)[0].(string)
			full := filepath.Join(dir, filepath.FromSlash(member))
			data, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, append(data, []byte("\nchanged input\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
			newDigest, err := aggregateMemberDigest(dir, changed["members"].([]any))
			if err != nil {
				t.Fatal(err)
			}
			changed["current_digest"] = newDigest
			changed["observed_at"] = "2026-08-28T02:00:00Z"
			run := graph["runs"].([]any)[0].(map[string]any)
			for _, raw := range run["input_bindings"].([]any) {
				binding := raw.(map[string]any)
				if binding["input_id"] == inputID {
					binding["digest"] = newDigest
				}
			}
			mustWriteJSON(t, graphPath, graph)
			_, err = AuditEvidenceDependencyGraph(dir, "evidence/dependency-graph.json")
			if err == nil || !strings.Contains(err.Error(), "digest書換えだけではClosureできません") {
				t.Fatalf("%s変更後の実再実行欠落を拒否できません: %v", inputID, err)
			}
		})
	}
}

func TestEvidenceDependencyRejectsMissingRerunTarget(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	graphPath := filepath.Join(dir, "evidence", "dependency-graph.json")
	graph := mustReadYAML(t, graphPath)
	run := graph["runs"].([]any)[0].(map[string]any)
	ids := run["output_ids"].([]any)
	run["output_ids"] = ids[1:]
	mustWriteJSON(t, graphPath, graph)
	_, err := AuditEvidenceDependencyGraph(dir, "evidence/dependency-graph.json")
	if err == nil || !strings.Contains(err.Error(), "rerun対象からoutputが漏れています") {
		t.Fatalf("再実行対象漏れを拒否できません: %v", err)
	}
}

func TestEvidenceDependencyRejectsOmittedKnownEvidence(t *testing.T) {
	dir := createDefinitiveRepositoryFixture(t)
	graphPath := filepath.Join(dir, "evidence", "dependency-graph.json")
	graph := mustReadYAML(t, graphPath)
	outputs := graph["outputs"].([]any)
	var removedID string
	kept := []any{}
	for _, raw := range outputs {
		output := raw.(map[string]any)
		if removedID == "" && strings.HasSuffix(output["path"].(string), ".proof.json") {
			removedID = output["id"].(string)
			continue
		}
		kept = append(kept, output)
	}
	graph["outputs"] = kept
	// Remove the exact path represented by removedID from both self-declared lists.
	removedPath := ""
	for _, raw := range outputs {
		output := raw.(map[string]any)
		if output["id"] == removedID {
			removedPath = output["path"].(string)
		}
	}
	required := []any{}
	for _, raw := range graph["required_outputs"].([]any) {
		if raw.(string) != removedPath {
			required = append(required, raw)
		}
	}
	graph["required_outputs"] = required
	run := graph["runs"].([]any)[0].(map[string]any)
	runIDs := []any{}
	for _, raw := range run["output_ids"].([]any) {
		if raw.(string) != removedID {
			runIDs = append(runIDs, raw)
		}
	}
	run["output_ids"] = runIDs
	mustWriteJSON(t, graphPath, graph)
	_, err := AuditEvidenceDependencyGraph(dir, "evidence/dependency-graph.json")
	if err == nil || !strings.Contains(err.Error(), "graphから欠落") {
		t.Fatalf("既知EvidenceのGraph外退避を拒否できません: %v", err)
	}
}

func TestEvidenceDependencyRejectsProofAndPlanStructureChanges(t *testing.T) {
	for _, tc := range []struct{ name, path, field string }{
		{"proof", "evidence/scenarios/index.json", "denominator"},
		{"closure-plan", "evidence/scenarios/closure-plan.json", "scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := createDefinitiveRepositoryFixture(t)
			path := filepath.Join(dir, filepath.FromSlash(tc.path))
			doc := mustReadYAML(t, path)
			doc[tc.field] = "structure was narrowed after rerun"
			mustWriteJSON(t, path, doc)
			graphPath := filepath.Join(dir, "evidence", "dependency-graph.json")
			graph := mustReadYAML(t, graphPath)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, raw := range graph["outputs"].([]any) {
				output := raw.(map[string]any)
				if output["path"] == tc.path {
					output["digest"] = shaDigest(data)
				}
			}
			mustWriteJSON(t, graphPath, graph)
			_, err = AuditEvidenceDependencyGraph(dir, "evidence/dependency-graph.json")
			if err == nil || !strings.Contains(err.Error(), "構造が変化") {
				t.Fatalf("%s構造変更を拒否できません: %v", tc.name, err)
			}
		})
	}
}
