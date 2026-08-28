package validate

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type AuditResult struct {
	AtlasID      string
	Status       string
	TargetSets   int
	Targets      int
	MasteryAreas int
	OpenRequired int
}

func AuditDir(dir string) (AuditResult, error) {
	paths := map[string]string{
		"atlas":    filepath.Join(dir, "atlas.yaml"),
		"mastery":  filepath.Join(dir, "mastery.yaml"),
		"coverage": filepath.Join(dir, "coverage.yaml"),
		"sources":  filepath.Join(dir, "sources.lock.yaml"),
		"skill":    filepath.Join(dir, "skill.package.yaml"),
	}
	documents := map[string]map[string]any{}
	for name, path := range paths {
		if _, err := File(path); err != nil {
			return AuditResult{}, err
		}
		document, err := readDocument(path)
		if err != nil {
			return AuditResult{}, err
		}
		documents[name] = document
	}

	atlasID, _ := documents["atlas"]["id"].(string)
	for _, name := range []string{"mastery", "coverage", "sources", "skill"} {
		candidate, _ := documents[name]["atlas_id"].(string)
		if candidate != atlasID {
			return AuditResult{}, fmt.Errorf("Atlas IDが一致しません: atlas=%s, %s=%s", atlasID, name, candidate)
		}
	}

	coverageConfig, _ := documents["atlas"]["coverage"].(map[string]any)
	epoch, _ := coverageConfig["epoch"].(string)
	for _, name := range []string{"mastery", "coverage", "sources"} {
		candidate, _ := documents[name]["epoch"].(string)
		if candidate != epoch {
			return AuditResult{}, fmt.Errorf("Coverage Epochが一致しません: atlas=%s, %s=%s", epoch, name, candidate)
		}
	}

	targetSets := collectIDs(documents["coverage"]["target_sets"])
	if err := validateMasteryTargetSets(documents["mastery"], targetSets); err != nil {
		return AuditResult{}, err
	}

	atlasSkills, _ := documents["atlas"]["skills"].(map[string]any)
	atlasRouter, _ := atlasSkills["router"].(map[string]any)
	skillRouter, _ := documents["skill"]["router"].(map[string]any)
	if atlasRouter["id"] != skillRouter["id"] || atlasRouter["path"] != skillRouter["path"] {
		return AuditResult{}, fmt.Errorf("atlas.yamlとskill.package.yamlのRouterが一致しません")
	}

	status, _ := documents["atlas"]["status"].(string)
	targets, _ := documents["coverage"]["targets"].([]any)
	openRequired := countOpenRequired(targets)
	if status == "complete" && openRequired > 0 {
		return AuditResult{}, fmt.Errorf("complete Atlasに未Closureの必須Targetが%d件あります", openRequired)
	}
	if status == "complete" {
		if err := auditComplete(dir, documents, targets); err != nil {
			return AuditResult{}, err
		}
	}

	return AuditResult{
		AtlasID:      atlasID,
		Status:       status,
		TargetSets:   len(targetSets),
		Targets:      len(targets),
		MasteryAreas: len(collectIDs(documents["mastery"]["surfaces"])),
		OpenRequired: openRequired,
	}, nil
}

func auditComplete(dir string, documents map[string]map[string]any, targets []any) error {
	if err := ensureMasteryCovered(documents["mastery"], targets); err != nil {
		return err
	}
	if err := verifyAuthorityDigest(filepath.Join(dir, "sources.lock.yaml"), documents["coverage"]); err != nil {
		return err
	}
	evidenceIDs, passedProfiles, err := collectEvidence(dir)
	if err != nil {
		return err
	}
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		if target["state"] != "covered" {
			continue
		}
		id, _ := target["id"].(string)
		for _, rawEvidenceID := range target["evidence_ids"].([]any) {
			evidenceID, _ := rawEvidenceID.(string)
			if !evidenceIDs[evidenceID] {
				return fmt.Errorf("covered Target %sが存在しないEvidenceを参照しています: %s", id, evidenceID)
			}
		}
	}
	atlasCompletion, _ := documents["atlas"]["completion"].(map[string]any)
	for _, rawProfile := range atlasCompletion["required_profiles"].([]any) {
		profile, _ := rawProfile.(string)
		if !passedProfiles[profile] {
			return fmt.Errorf("Required Profile %sのpass Evidenceがありません", profile)
		}
	}

	atlas := documents["atlas"]
	atlasSkills, _ := atlas["skills"].(map[string]any)
	router, _ := atlasSkills["router"].(map[string]any)
	license, _ := atlas["license"].(map[string]any)
	security, _ := atlas["security"].(map[string]any)
	requiredPaths := []string{
		"README.md", "LICENSE",
		stringValue(license["notice"]), stringValue(license["third_party_manifest"]), stringValue(license["sbom"]),
		stringValue(security["disclosure"]), stringValue(router["path"]), stringValue(atlasSkills["evals"]),
		stringValue(atlasCompletion["certificate"]),
	}
	for _, relative := range requiredPaths {
		if relative == "" {
			return fmt.Errorf("complete Atlasの必須Pathが空です")
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relative))); err != nil {
			return fmt.Errorf("complete Atlasの必須成果物がありません: %s", relative)
		}
	}
	return nil
}

func ensureMasteryCovered(mastery map[string]any, targets []any) error {
	coveredSets := map[string]bool{}
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		if target["state"] == "covered" {
			targetSet, _ := target["target_set"].(string)
			coveredSets[targetSet] = true
		}
	}
	for _, collection := range []string{"outcomes", "surfaces"} {
		items, _ := mastery[collection].([]any)
		for _, rawItem := range items {
			item, _ := rawItem.(map[string]any)
			if collection == "surfaces" && item["applicability"] == "not-applicable" {
				continue
			}
			id, _ := item["id"].(string)
			connected := false
			for _, rawTargetSet := range item["target_sets"].([]any) {
				targetSet, _ := rawTargetSet.(string)
				connected = connected || coveredSets[targetSet]
			}
			if !connected {
				return fmt.Errorf("complete AtlasのMastery %s %sにcovered Targetがありません", collection, id)
			}
		}
	}
	return nil
}

func verifyAuthorityDigest(path string, coverage map[string]any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	expected, _ := coverage["authority_lock_digest"].(string)
	if actual != expected {
		return fmt.Errorf("Authority Lock Digestが一致しません: expected=%s actual=%s", expected, actual)
	}
	return nil
}

func collectEvidence(dir string) (map[string]bool, map[string]bool, error) {
	ids := map[string]bool{}
	passedProfiles := map[string]bool{}
	evidenceRoot := filepath.Join(dir, "evidence")
	if _, err := os.Stat(evidenceRoot); err != nil {
		return ids, passedProfiles, fmt.Errorf("Evidence Directoryがありません: evidence/")
	}
	err := filepath.WalkDir(evidenceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".evidence.yaml") || strings.HasSuffix(entry.Name(), ".evidence.yml") || strings.HasSuffix(entry.Name(), ".evidence.json")) {
			return nil
		}
		if _, err := File(path); err != nil {
			return err
		}
		document, err := readDocument(path)
		if err != nil {
			return err
		}
		id, _ := document["id"].(string)
		if ids[id] {
			return fmt.Errorf("Evidence IDが重複しています: %s", id)
		}
		ids[id] = true
		if document["verdict"] == "pass" {
			environment, _ := document["environment"].(map[string]any)
			profile, _ := environment["profile"].(string)
			passedProfiles[profile] = true
		}
		return nil
	})
	return ids, passedProfiles, err
}

func stringValue(raw any) string {
	value, _ := raw.(string)
	return value
}

func readDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%sを読み込めません: %w", path, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%sは有効なYAMLではありません: %w", path, err)
	}
	return document, nil
}

func collectIDs(raw any) map[string]bool {
	items, _ := raw.([]any)
	ids := map[string]bool{}
	for _, rawItem := range items {
		item, _ := rawItem.(map[string]any)
		id, _ := item["id"].(string)
		ids[id] = true
	}
	return ids
}

func validateMasteryTargetSets(mastery map[string]any, targetSets map[string]bool) error {
	for _, collection := range []string{"outcomes", "surfaces"} {
		items, _ := mastery[collection].([]any)
		for _, rawItem := range items {
			item, _ := rawItem.(map[string]any)
			id, _ := item["id"].(string)
			refs, _ := item["target_sets"].([]any)
			for _, rawRef := range refs {
				ref, _ := rawRef.(string)
				if !targetSets[ref] {
					return fmt.Errorf("Mastery %s %sが未定義Target Setを参照しています: %s", collection, id, ref)
				}
			}
		}
	}
	return nil
}

func countOpenRequired(targets []any) int {
	open := 0
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		requirement, _ := target["requirement"].(string)
		state, _ := target["state"].(string)
		if requirement == "required" && state != "covered" && state != "excluded" && state != "infeasible" {
			open++
		}
	}
	return open
}
