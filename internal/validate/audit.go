package validate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
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
	Claims       int
	Evidence     int
	OpenRequired int
}

type auditContext struct {
	dir       string
	documents map[string]map[string]any
	targets   []any
	claims    map[string]map[string]any
	evidence  map[string]map[string]any
	sources   map[string]bool
}

func AuditDir(dir string) (AuditResult, error) {
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return AuditResult{}, err
	}
	atlasID := stringValue(ctx.documents["atlas"]["id"])
	status := stringValue(ctx.documents["atlas"]["status"])
	targetSets := collectIDs(ctx.documents["coverage"]["target_sets"])
	openRequired := countOpenRequired(ctx.targets)
	if status == "complete" && openRequired > 0 {
		return AuditResult{}, fmt.Errorf("complete Atlasに未Closureの必須Targetが%d件あります", openRequired)
	}
	if status == "complete" {
		if err := auditComplete(ctx); err != nil {
			return AuditResult{}, err
		}
	}
	return AuditResult{
		AtlasID: atlasID, Status: status, TargetSets: len(targetSets), Targets: len(ctx.targets),
		MasteryAreas: len(collectIDs(ctx.documents["mastery"]["surfaces"])), Claims: len(ctx.claims),
		Evidence: len(ctx.evidence), OpenRequired: openRequired,
	}, nil
}

func loadAuditContext(dir string) (*auditContext, error) {
	paths := map[string]string{
		"atlas": filepath.Join(dir, "atlas.yaml"), "mastery": filepath.Join(dir, "mastery.yaml"),
		"coverage": filepath.Join(dir, "coverage.yaml"), "sources": filepath.Join(dir, "sources.lock.yaml"),
		"skill": filepath.Join(dir, "skill.package.yaml"),
	}
	documents := map[string]map[string]any{}
	for name, path := range paths {
		if _, err := File(path); err != nil {
			return nil, err
		}
		document, err := readDocument(path)
		if err != nil {
			return nil, err
		}
		documents[name] = document
	}
	atlasID := stringValue(documents["atlas"]["id"])
	for _, name := range []string{"mastery", "coverage", "sources", "skill"} {
		candidate := stringValue(documents[name]["atlas_id"])
		if candidate != atlasID {
			return nil, fmt.Errorf("Atlas IDが一致しません: atlas=%s, %s=%s", atlasID, name, candidate)
		}
	}
	coverageConfig, _ := documents["atlas"]["coverage"].(map[string]any)
	epoch := stringValue(coverageConfig["epoch"])
	for _, name := range []string{"mastery", "coverage", "sources"} {
		candidate := stringValue(documents[name]["epoch"])
		if candidate != epoch {
			return nil, fmt.Errorf("Coverage Epochが一致しません: atlas=%s, %s=%s", epoch, name, candidate)
		}
	}
	targetSets := collectIDs(documents["coverage"]["target_sets"])
	if err := validateMasteryTargetSets(documents["mastery"], targetSets); err != nil {
		return nil, err
	}
	if err := validateTargets(documents["coverage"], targetSets); err != nil {
		return nil, err
	}
	atlasSkills, _ := documents["atlas"]["skills"].(map[string]any)
	atlasRouter, _ := atlasSkills["router"].(map[string]any)
	skillRouter, _ := documents["skill"]["router"].(map[string]any)
	if atlasRouter["id"] != skillRouter["id"] || atlasRouter["path"] != skillRouter["path"] {
		return nil, fmt.Errorf("atlas.yamlとskill.package.yamlのRouterが一致しません")
	}
	claims, err := collectEntities(filepath.Join(dir, "claims"), ".claim.")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	evidence, err := collectEntities(filepath.Join(dir, "evidence"), ".evidence.")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sources := collectIDs(documents["sources"]["sources"])
	targets, _ := documents["coverage"]["targets"].([]any)
	return &auditContext{dir: dir, documents: documents, targets: targets, claims: claims, evidence: evidence, sources: sources}, nil
}

func auditComplete(ctx *auditContext) error {
	if err := ensureMasteryCovered(ctx.documents["mastery"], ctx.targets); err != nil {
		return err
	}
	if err := verifyAuthorityDigest(filepath.Join(ctx.dir, "sources.lock.yaml"), ctx.documents["coverage"]); err != nil {
		return err
	}
	if len(ctx.claims) == 0 || len(ctx.evidence) == 0 {
		return fmt.Errorf("complete AtlasにはClaimとEvidenceの実体が必要です")
	}
	if err := auditClaimEvidenceGraph(ctx); err != nil {
		return err
	}
	if err := auditProfiles(ctx); err != nil {
		return err
	}
	atlas := ctx.documents["atlas"]
	atlasSkills, _ := atlas["skills"].(map[string]any)
	router, _ := atlasSkills["router"].(map[string]any)
	license, _ := atlas["license"].(map[string]any)
	security, _ := atlas["security"].(map[string]any)
	atlasCompletion, _ := atlas["completion"].(map[string]any)
	requiredPaths := []string{
		"README.md", "LICENSE", stringValue(license["notice"]), stringValue(license["third_party_manifest"]),
		stringValue(license["sbom"]), "provenance.yaml", stringValue(security["disclosure"]),
		stringValue(router["path"]), stringValue(atlasSkills["evals"]), stringValue(atlasCompletion["certificate"]),
	}
	for _, relative := range requiredPaths {
		if relative == "" {
			return fmt.Errorf("complete Atlasの必須Pathが空です")
		}
		if _, err := os.Stat(filepath.Join(ctx.dir, filepath.FromSlash(relative))); err != nil {
			return fmt.Errorf("complete Atlasの必須成果物がありません: %s", relative)
		}
	}
	if _, err := skillEvalSummary(ctx); err != nil {
		return err
	}
	if err := auditSBOM(filepath.Join(ctx.dir, filepath.FromSlash(stringValue(license["sbom"])))); err != nil {
		return err
	}
	if err := auditSupplyChain(ctx, stringValue(license["sbom"]), stringValue(license["third_party_manifest"])); err != nil {
		return err
	}
	if err := auditProvenance(ctx); err != nil {
		return err
	}
	return VerifyCertificate(ctx.dir)
}

func auditClaimEvidenceGraph(ctx *auditContext) error {
	referencedClaims := map[string]bool{}
	referencedEvidence := map[string]bool{}
	passedClaims := map[string]bool{}
	for evidenceID, evidence := range ctx.evidence {
		if evidence["atlas_id"] != ctx.documents["atlas"]["id"] {
			return fmt.Errorf("Evidence %sのAtlas IDが一致しません", evidenceID)
		}
		for _, raw := range anySlice(evidence["claim_ids"]) {
			claimID := stringValue(raw)
			if _, ok := ctx.claims[claimID]; !ok {
				return fmt.Errorf("Evidence %sが存在しないClaimを参照しています: %s", evidenceID, claimID)
			}
			if evidence["verdict"] == "pass" {
				passedClaims[claimID] = true
			}
		}
		if err := verifyEvidenceBindings(ctx, evidenceID, evidence); err != nil {
			return err
		}
	}
	for _, rawTarget := range ctx.targets {
		target, _ := rawTarget.(map[string]any)
		if target["state"] != "covered" {
			continue
		}
		targetID := stringValue(target["id"])
		targetClaims := map[string]bool{}
		for _, raw := range anySlice(target["claim_ids"]) {
			claimID := stringValue(raw)
			claim, ok := ctx.claims[claimID]
			if !ok {
				return fmt.Errorf("covered Target %sが存在しないClaimを参照しています: %s", targetID, claimID)
			}
			if claim["status"] != "accepted" || claim["atlas_id"] != ctx.documents["atlas"]["id"] {
				return fmt.Errorf("covered Target %sのClaim %sがacceptedではないかAtlas ID不一致です", targetID, claimID)
			}
			for _, source := range anySlice(claim["source_ids"]) {
				if !ctx.sources[stringValue(source)] {
					return fmt.Errorf("Claim %sが未定義Sourceを参照しています: %s", claimID, source)
				}
			}
			if !passedClaims[claimID] {
				return fmt.Errorf("Claim %sにpass Evidenceがありません", claimID)
			}
			referencedClaims[claimID], targetClaims[claimID] = true, true
		}
		for _, raw := range anySlice(target["evidence_ids"]) {
			evidenceID := stringValue(raw)
			evidence, ok := ctx.evidence[evidenceID]
			if !ok {
				return fmt.Errorf("covered Target %sが存在しないEvidenceを参照しています: %s", targetID, evidenceID)
			}
			if evidence["verdict"] != "pass" {
				return fmt.Errorf("covered Target %sのEvidence %sがpassではありません", targetID, evidenceID)
			}
			connected := false
			for _, rawClaim := range anySlice(evidence["claim_ids"]) {
				connected = connected || targetClaims[stringValue(rawClaim)]
			}
			if !connected {
				return fmt.Errorf("Target %sとEvidence %sが同じClaimへ接続されていません", targetID, evidenceID)
			}
			referencedEvidence[evidenceID] = true
		}
	}
	for id, claim := range ctx.claims {
		if claim["status"] == "accepted" && !referencedClaims[id] {
			return fmt.Errorf("孤立したaccepted Claimがあります: %s", id)
		}
	}
	for id := range ctx.evidence {
		if !referencedEvidence[id] {
			return fmt.Errorf("孤立したEvidenceがあります: %s", id)
		}
	}
	return nil
}

func verifyEvidenceBindings(ctx *auditContext, id string, evidence map[string]any) error {
	expectedSource := stringValue(ctx.documents["coverage"]["authority_lock_digest"])
	if evidence["source_digest"] != expectedSource {
		return fmt.Errorf("Evidence %sのSource DigestがAuthority Lockと一致しません", id)
	}
	harnessPath := stringValue(evidence["harness_path"])
	if harnessPath == "" {
		return fmt.Errorf("complete AtlasのEvidence %sにharness_pathがありません", id)
	}
	if err := verifyRelativeFileDigest(ctx.dir, harnessPath, stringValue(evidence["harness_digest"]), -1); err != nil {
		return fmt.Errorf("Evidence %sのHarness検証に失敗しました: %w", id, err)
	}
	artifact, _ := evidence["artifact"].(map[string]any)
	uri := stringValue(artifact["uri"])
	if parsed, err := url.Parse(uri); err != nil || parsed.IsAbs() {
		return fmt.Errorf("Evidence %sのArtifact URIはRepository相対Pathである必要があります: %s", id, uri)
	}
	size := int64(-1)
	switch raw := artifact["size_bytes"].(type) {
	case int:
		size = int64(raw)
	case int64:
		size = raw
	case float64:
		size = int64(raw)
	}
	if err := verifyRelativeFileDigest(ctx.dir, uri, stringValue(artifact["digest"]), size); err != nil {
		return fmt.Errorf("Evidence %sのArtifact検証に失敗しました: %w", id, err)
	}
	return nil
}

func verifyRelativeFileDigest(root, relative, expected string, expectedSize int64) error {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if relative == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("安全でない相対Pathです: %s", relative)
	}
	data, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return err
	}
	actual := digestBytes(data)
	if actual != expected {
		return fmt.Errorf("Digestが一致しません: path=%s expected=%s actual=%s", relative, expected, actual)
	}
	if expectedSize >= 0 && int64(len(data)) != expectedSize {
		return fmt.Errorf("Sizeが一致しません: path=%s expected=%d actual=%d", relative, expectedSize, len(data))
	}
	return nil
}

func auditProfiles(ctx *auditContext) error {
	passed := map[string]bool{}
	for _, evidence := range ctx.evidence {
		if evidence["verdict"] == "pass" {
			environment, _ := evidence["environment"].(map[string]any)
			passed[stringValue(environment["profile"])] = true
		}
	}
	completion, _ := ctx.documents["atlas"]["completion"].(map[string]any)
	for _, raw := range anySlice(completion["required_profiles"]) {
		profile := stringValue(raw)
		if !passed[profile] {
			return fmt.Errorf("Required Profile %sのpass Evidenceがありません", profile)
		}
	}
	return nil
}

type evalSummary struct {
	path     string
	digest   string
	passRate float64
}

func skillEvalSummary(ctx *auditContext) (evalSummary, error) {
	skill := ctx.documents["skill"]
	evals, _ := skill["evals"].(map[string]any)
	root := filepath.Join(ctx.dir, filepath.FromSlash(stringValue(evals["path"])))
	entities, err := collectEntities(root, ".skill-eval.")
	if err != nil {
		return evalSummary{}, fmt.Errorf("Skill Evalを読み込めません: %w", err)
	}
	if len(entities) != 1 {
		return evalSummary{}, fmt.Errorf("complete AtlasにはSkill Eval実体が1件必要です: actual=%d", len(entities))
	}
	required := map[string]bool{"routing": false, "near-neighbor": false, "coverage-gap": false, "lifecycle": false, "authority": false, "execution": false, "authorization": false, "security": false}
	passed, total := 0, 0
	var evalPath string
	for id, evaluation := range entities {
		if evaluation["atlas_id"] != ctx.documents["atlas"]["id"] {
			return evalSummary{}, fmt.Errorf("Skill EvalのAtlas IDが一致しません")
		}
		router, _ := skill["router"].(map[string]any)
		if evaluation["skill_id"] != router["id"] || evaluation["atlas_release"] != skill["atlas_release"] {
			return evalSummary{}, fmt.Errorf("Skill EvalのSkill IDまたはAtlas Releaseが一致しません")
		}
		for _, rawCase := range anySlice(evaluation["cases"]) {
			item, _ := rawCase.(map[string]any)
			required[stringValue(item["category"])] = true
			total++
			if item["result"] == "pass" {
				passed++
			}
		}
		evalPath, err = findEntityPath(root, id, ".skill-eval.")
		if err != nil {
			return evalSummary{}, err
		}
	}
	for category, present := range required {
		if !present {
			return evalSummary{}, fmt.Errorf("Skill Evalに必須Categoryがありません: %s", category)
		}
	}
	minimum, _ := evals["minimum_pass_rate"].(float64)
	passRate := float64(passed) / float64(total)
	if total == 0 || passRate < minimum {
		return evalSummary{}, fmt.Errorf("Skill Eval合格率が不足しています: pass=%d total=%d minimum=%.3f", passed, total, minimum)
	}
	data, err := os.ReadFile(evalPath)
	if err != nil {
		return evalSummary{}, err
	}
	return evalSummary{path: evalPath, digest: digestBytes(data), passRate: passRate}, nil
}

func findEntityPath(root, id, marker string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.Contains(entry.Name(), marker) {
			return nil
		}
		doc, err := readDocument(path)
		if err != nil {
			return err
		}
		if stringValue(doc["id"]) == id {
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("Entity fileが見つかりません: %s", id)
	}
	return found, nil
}

func auditSBOM(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("SBOMを読み込めません: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("SBOMは有効なJSONではありません: %w", err)
	}
	if doc["spdxVersion"] != "SPDX-2.3" || doc["SPDXID"] != "SPDXRef-DOCUMENT" || doc["dataLicense"] != "CC0-1.0" {
		return fmt.Errorf("SBOMはSPDX 2.3 Document要件を満たしません")
	}
	if parsed, err := url.Parse(stringValue(doc["documentNamespace"])); err != nil || !parsed.IsAbs() {
		return fmt.Errorf("SBOMのdocumentNamespaceが絶対URIではありません")
	}
	packages := anySlice(doc["packages"])
	if len(packages) == 0 {
		return fmt.Errorf("SBOMにPackageがありません")
	}
	for _, raw := range packages {
		pkg, _ := raw.(map[string]any)
		for _, field := range []string{"name", "SPDXID", "versionInfo", "licenseConcluded", "licenseDeclared"} {
			if stringValue(pkg[field]) == "" || stringValue(pkg[field]) == "NOASSERTION" {
				return fmt.Errorf("SBOM Packageの%sが未確定です", field)
			}
		}
	}
	return nil
}

func auditSupplyChain(ctx *auditContext, sbomRelative, thirdPartyRelative string) error {
	thirdPartyPath := filepath.Join(ctx.dir, filepath.FromSlash(thirdPartyRelative))
	if _, err := File(thirdPartyPath); err != nil {
		return err
	}
	thirdParty, err := readDocument(thirdPartyPath)
	if err != nil {
		return err
	}
	type dependency struct{ version, license string }
	declared := map[string]dependency{}
	for _, raw := range anySlice(thirdParty["artifacts"]) {
		item, _ := raw.(map[string]any)
		if item["kind"] != "go-module" {
			continue
		}
		name := stringValue(item["name"])
		if _, duplicate := declared[name]; duplicate {
			return fmt.Errorf("第三者Go Moduleが重複しています: %s", name)
		}
		declared[name] = dependency{version: strings.TrimPrefix(stringValue(item["version"]), "v"), license: stringValue(item["license"])}
	}
	sbomData, err := os.ReadFile(filepath.Join(ctx.dir, filepath.FromSlash(sbomRelative)))
	if err != nil {
		return err
	}
	var sbom map[string]any
	if err := json.Unmarshal(sbomData, &sbom); err != nil {
		return err
	}
	sbomPackages := map[string]dependency{}
	for _, raw := range anySlice(sbom["packages"]) {
		pkg, _ := raw.(map[string]any)
		name := stringValue(pkg["name"])
		if name == stringValue(ctx.documents["atlas"]["id"]) {
			continue
		}
		sbomPackages[name] = dependency{version: strings.TrimPrefix(stringValue(pkg["versionInfo"]), "v"), license: stringValue(pkg["licenseDeclared"])}
	}
	for name, pkg := range sbomPackages {
		entry, ok := declared[name]
		if !ok {
			return fmt.Errorf("SBOM Packageが第三者Manifestにありません: %s", name)
		}
		if entry.version != pkg.version || entry.license != pkg.license {
			return fmt.Errorf("SBOMと第三者Manifestが一致しません: %s", name)
		}
	}
	modules, err := goModDependencies(filepath.Join(ctx.dir, "go.mod"))
	if err != nil {
		return err
	}
	for name, version := range modules {
		pkg, ok := sbomPackages[name]
		if !ok || pkg.version != strings.TrimPrefix(version, "v") {
			return fmt.Errorf("go.mod DependencyがSBOMへ固定されていません: %s %s", name, version)
		}
	}
	return nil
}

func goModDependencies(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dependencies := map[string]string{}
	inRequire := false
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		fields := strings.Fields(line)
		if inRequire && len(fields) >= 2 {
			dependencies[fields[0]] = fields[1]
		} else if len(fields) >= 3 && fields[0] == "require" {
			dependencies[fields[1]] = fields[2]
		}
	}
	return dependencies, nil
}

func auditProvenance(ctx *auditContext) error {
	path := filepath.Join(ctx.dir, "provenance.yaml")
	if _, err := File(path); err != nil {
		return err
	}
	doc, err := readDocument(path)
	if err != nil {
		return err
	}
	if doc["atlas_id"] != ctx.documents["atlas"]["id"] {
		return fmt.Errorf("ProvenanceのAtlas IDが一致しません")
	}
	records := map[string]string{}
	for _, raw := range anySlice(doc["artifacts"]) {
		item, _ := raw.(map[string]any)
		artifactPath := stringValue(item["path"])
		if _, duplicate := records[artifactPath]; duplicate {
			return fmt.Errorf("Provenance Artifact Pathが重複しています: %s", artifactPath)
		}
		records[artifactPath] = stringValue(item["digest"])
		for _, source := range anySlice(item["source_ids"]) {
			if !ctx.sources[stringValue(source)] {
				return fmt.Errorf("Provenance %sが未定義Sourceを参照しています: %s", artifactPath, source)
			}
		}
		if err := verifyRelativeFileDigest(ctx.dir, artifactPath, records[artifactPath], -1); err != nil {
			return fmt.Errorf("Provenance検証に失敗しました: %w", err)
		}
	}
	for _, evidence := range ctx.evidence {
		artifact, _ := evidence["artifact"].(map[string]any)
		artifactPath := stringValue(artifact["uri"])
		if records[artifactPath] != stringValue(artifact["digest"]) {
			return fmt.Errorf("Evidence ArtifactがProvenanceへ正しく登録されていません: %s", artifactPath)
		}
	}
	license, _ := ctx.documents["atlas"]["license"].(map[string]any)
	sbomPath := stringValue(license["sbom"])
	data, err := os.ReadFile(filepath.Join(ctx.dir, filepath.FromSlash(sbomPath)))
	if err != nil || records[sbomPath] != digestBytes(data) {
		return fmt.Errorf("SBOMがProvenanceへ正しく登録されていません: %s", sbomPath)
	}
	return nil
}

func collectEntities(root, marker string) (map[string]map[string]any, error) {
	entities := map[string]map[string]any{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.Contains(entry.Name(), marker) {
			return nil
		}
		if _, err := File(path); err != nil {
			return err
		}
		document, err := readDocument(path)
		if err != nil {
			return err
		}
		id := stringValue(document["id"])
		if _, duplicate := entities[id]; duplicate {
			return fmt.Errorf("Entity IDが重複しています: %s", id)
		}
		entities[id] = document
		return nil
	})
	return entities, err
}

func readDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%sを読み込めません: %w", path, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%sは有効なYAML/JSONではありません: %w", path, err)
	}
	return document, nil
}

func ensureMasteryCovered(mastery map[string]any, targets []any) error {
	coveredSets := map[string]bool{}
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		if target["state"] == "covered" {
			coveredSets[stringValue(target["target_set"])] = true
		}
	}
	for _, collection := range []string{"outcomes", "surfaces"} {
		for _, rawItem := range anySlice(mastery[collection]) {
			item, _ := rawItem.(map[string]any)
			if collection == "surfaces" && item["applicability"] == "not-applicable" {
				continue
			}
			connected := false
			for _, raw := range anySlice(item["target_sets"]) {
				connected = connected || coveredSets[stringValue(raw)]
			}
			if !connected {
				return fmt.Errorf("complete AtlasのMastery %s %sにcovered Targetがありません", collection, item["id"])
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
	actual := digestBytes(data)
	expected := stringValue(coverage["authority_lock_digest"])
	if actual != expected {
		return fmt.Errorf("Authority Lock Digestが一致しません: expected=%s actual=%s", expected, actual)
	}
	return nil
}

func validateTargets(coverage map[string]any, targetSets map[string]bool) error {
	for _, raw := range anySlice(coverage["targets"]) {
		target, _ := raw.(map[string]any)
		if !targetSets[stringValue(target["target_set"])] {
			return fmt.Errorf("Coverage Target %sが未定義Target Setを参照しています: %s", target["id"], target["target_set"])
		}
	}
	return nil
}

func stringValue(raw any) string     { value, _ := raw.(string); return value }
func anySlice(raw any) []any         { value, _ := raw.([]any); return value }
func digestBytes(data []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(data)) }

func collectIDs(raw any) map[string]bool {
	ids := map[string]bool{}
	for _, rawItem := range anySlice(raw) {
		item, _ := rawItem.(map[string]any)
		ids[stringValue(item["id"])] = true
	}
	return ids
}

func validateMasteryTargetSets(mastery map[string]any, targetSets map[string]bool) error {
	for _, collection := range []string{"outcomes", "surfaces"} {
		for _, rawItem := range anySlice(mastery[collection]) {
			item, _ := rawItem.(map[string]any)
			for _, rawRef := range anySlice(item["target_sets"]) {
				ref := stringValue(rawRef)
				if !targetSets[ref] {
					return fmt.Errorf("Mastery %s %sが未定義Target Setを参照しています: %s", collection, item["id"], ref)
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
		state := stringValue(target["state"])
		if target["requirement"] == "required" && state != "covered" && state != "excluded" && state != "infeasible" {
			open++
		}
	}
	return open
}
