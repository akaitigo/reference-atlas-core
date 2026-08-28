package validate

import (
	"fmt"
	"path/filepath"
	"sort"
)

var definitiveScenarios = []string{"normal", "boundary", "rejection", "failure", "recovery", "migration", "operations", "security", "performance", "compatibility"}

var scenarioBySurface = map[string][]string{
	"failure-recovery":                {"failure", "recovery"},
	"operations-observability":        {"operations"},
	"security-privacy-safety":         {"security"},
	"performance-capacity-cost":       {"performance"},
	"compatibility-integration":       {"compatibility"},
	"migration-evolution-deprecation": {"migration"},
}

type DefinitiveAuditResult struct {
	AtlasID            string
	CompletionClass    string
	AuthoritySurfaces  int
	IncludedBehaviors  int
	ProofObligations   int
	RequiredMatrixRows int
	RuntimeEvidence    int
	ReferenceSystems   int
	Comparisons        int
	DepthParityRows    int
}

type definitiveContext struct {
	base             *auditContext
	manifest         map[string]any
	inventory        map[string]any
	matrix           map[string]any
	depthParity      map[string]any
	skillEval        map[string]any
	behaviors        map[string]map[string]any
	claimsByBehavior map[string]map[string]any
}

func AuditDefinitive(dir string) (DefinitiveAuditResult, error) {
	ctx, err := loadDefinitiveBase(dir)
	if err != nil {
		return DefinitiveAuditResult{}, err
	}
	if err := auditDefinitiveRequiredTargets(ctx); err != nil {
		return DefinitiveAuditResult{}, err
	}
	authorityCount, err := auditSurfaceInventory(ctx)
	if err != nil {
		return DefinitiveAuditResult{}, err
	}
	proofCount, requiredRows, runtimeCount, err := auditDefinitiveProofMatrix(ctx)
	if err != nil {
		return DefinitiveAuditResult{}, err
	}
	if err := auditReferenceSystemsAndComparisons(ctx); err != nil {
		return DefinitiveAuditResult{}, err
	}
	if err := auditDefinitiveSkill(ctx); err != nil {
		return DefinitiveAuditResult{}, err
	}
	if _, err := AuditDefinitiveSkillRouter(dir, stringValue(ctx.manifest["skill_router"]), true); err != nil {
		return DefinitiveAuditResult{}, fmt.Errorf("Definitive Skill Router Gate: %w", err)
	}
	if _, err := AuditNonRegression(dir); err != nil {
		return DefinitiveAuditResult{}, fmt.Errorf("Definitive non-regression Gate: %w", err)
	}
	depthRows, err := auditDepthParity(ctx)
	if err != nil {
		return DefinitiveAuditResult{}, err
	}
	if err := auditDefinitivePromotionFoundation(ctx); err != nil {
		return DefinitiveAuditResult{}, err
	}
	if stringValue(ctx.base.documents["atlas"]["status"]) != "complete" {
		return DefinitiveAuditResult{}, fmt.Errorf("historical bounded-complete基盤は検証済みですが、subject-definitive昇格にはatlas.status=completeが必要です")
	}
	if err := VerifyDefinitiveCertificate(dir); err != nil {
		return DefinitiveAuditResult{}, err
	}
	return DefinitiveAuditResult{
		AtlasID: stringValue(ctx.base.documents["atlas"]["id"]), CompletionClass: "subject-definitive",
		AuthoritySurfaces: authorityCount, IncludedBehaviors: len(ctx.behaviors), ProofObligations: proofCount,
		RequiredMatrixRows: requiredRows, RuntimeEvidence: runtimeCount,
		ReferenceSystems: len(anySlice(ctx.manifest["reference_systems"])), Comparisons: len(anySlice(ctx.manifest["comparisons"])),
		DepthParityRows: depthRows,
	}, nil
}

// loadDefinitiveBase accepts either the current bounded-complete state or an
// immutable, self-signed v1 certificate retained as bounded-complete history.
// The latter lets an honest status=incomplete migration reach its actual v2
// promotion gap without rewriting or deleting the historical certificate.
func loadDefinitiveBase(dir string) (*definitiveContext, error) {
	baseResult, baseErr := AuditDir(dir)
	ctx, ctxErr := loadDefinitiveContext(dir)
	if ctxErr != nil {
		return nil, ctxErr
	}
	if baseErr == nil && baseResult.CompletionClass == "bounded-complete" {
		return ctx, nil
	}
	if err := verifyHistoricalBoundedBase(ctx); err != nil {
		if baseErr != nil {
			return nil, fmt.Errorf("bounded-complete基盤が無効で、historical bounded Certificateも検証できません: current=%v historical=%w", baseErr, err)
		}
		return nil, fmt.Errorf("subject-definitiveには現在またはhistoricalの有効なbounded-complete基盤が必要です: %w", err)
	}
	return ctx, nil
}

func verifyHistoricalBoundedBase(ctx *definitiveContext) error {
	items := anySlice(ctx.manifest["historical_certificates"])
	if len(items) == 0 {
		return fmt.Errorf("historical_certificatesがありません")
	}
	verified := 0
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["classification"] != "bounded-complete" {
			continue
		}
		path := stringValue(item["path"])
		full := filepath.Join(ctx.base.dir, filepath.FromSlash(path))
		if _, err := File(full); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		certificate, err := readJSONDocument(full)
		if err != nil {
			return err
		}
		if stringValue(certificate["atlas_id"]) != stringValue(ctx.base.documents["atlas"]["id"]) {
			return fmt.Errorf("%sのAtlas IDが現在のRepositoryと一致しません", path)
		}
		signature, _ := certificate["signature"].(map[string]any)
		if stringValue(signature["type"]) != "payload-sha256" {
			return fmt.Errorf("%sの署名方式が無効です", path)
		}
		payload := make(map[string]any, len(certificate)-1)
		for key, value := range certificate {
			if key != "signature" {
				payload[key] = value
			}
		}
		digest, err := digestCanonical(payload)
		if err != nil {
			return err
		}
		if digest != stringValue(signature["digest"]) {
			return fmt.Errorf("%sのpayload署名が一致しません", path)
		}
		verified++
	}
	if verified == 0 {
		return fmt.Errorf("bounded-complete分類のhistorical Certificateがありません")
	}
	return nil
}

// auditDefinitivePromotionFoundation enforces the v1 foundation without
// requiring the historical certificate to match the migrated working tree.
func auditDefinitivePromotionFoundation(ctx *definitiveContext) error {
	if err := ensureMasteryCovered(ctx.base.documents["mastery"], ctx.base.targets); err != nil {
		return fmt.Errorf("subject-definitive昇格のMastery基盤: %w", err)
	}
	if err := verifyAuthorityDigest(filepath.Join(ctx.base.dir, "sources.lock.yaml"), ctx.base.documents["coverage"]); err != nil {
		return err
	}
	if err := auditClaimEvidenceGraph(ctx.base); err != nil {
		return err
	}
	if err := auditProfiles(ctx.base); err != nil {
		return err
	}
	if _, err := skillEvalSummary(ctx.base); err != nil {
		return err
	}
	atlas := ctx.base.documents["atlas"]
	license, _ := atlas["license"].(map[string]any)
	if err := auditSBOM(filepath.Join(ctx.base.dir, filepath.FromSlash(stringValue(license["sbom"])))); err != nil {
		return err
	}
	if err := auditSupplyChain(ctx.base, stringValue(license["sbom"]), stringValue(license["third_party_manifest"])); err != nil {
		return err
	}
	return auditProvenance(ctx.base)
}

func loadDefinitiveContext(dir string) (*definitiveContext, error) {
	base, err := loadAuditContext(dir)
	if err != nil {
		return nil, err
	}
	docs := map[string]map[string]any{}
	for name, relative := range map[string]string{"manifest": "definitive.yaml", "inventory": "surface.inventory.yaml", "matrix": "verification.matrix.yaml", "depth-parity": "depth.parity.yaml"} {
		path := filepath.Join(dir, relative)
		if _, err := File(path); err != nil {
			return nil, err
		}
		docs[name], err = readDocument(path)
		if err != nil {
			return nil, err
		}
	}
	skillPath := filepath.Join(dir, filepath.FromSlash(stringValue(docs["manifest"]["skill_eval"])))
	if _, err := File(skillPath); err != nil {
		return nil, err
	}
	skillEval, err := readDocument(skillPath)
	if err != nil {
		return nil, err
	}
	atlasID := stringValue(base.documents["atlas"]["id"])
	coverageConfig, _ := base.documents["atlas"]["coverage"].(map[string]any)
	epoch := stringValue(coverageConfig["epoch"])
	for name, doc := range map[string]map[string]any{"definitive": docs["manifest"], "inventory": docs["inventory"], "matrix": docs["matrix"], "depth-parity": docs["depth-parity"], "skill-eval": skillEval} {
		if stringValue(doc["atlas_id"]) != atlasID {
			return nil, fmt.Errorf("%sのAtlas IDが一致しません", name)
		}
		if candidate := stringValue(doc["epoch"]); candidate != "" && candidate != epoch {
			return nil, fmt.Errorf("%sのCoverage Epochが一致しません", name)
		}
	}
	if docs["inventory"]["authority_lock_digest"] != base.documents["coverage"]["authority_lock_digest"] {
		return nil, fmt.Errorf("Surface InventoryのAuthority Lock Digestが一致しません")
	}
	return &definitiveContext{base: base, manifest: docs["manifest"], inventory: docs["inventory"], matrix: docs["matrix"], depthParity: docs["depth-parity"], skillEval: skillEval, behaviors: map[string]map[string]any{}, claimsByBehavior: map[string]map[string]any{}}, nil
}

func auditDefinitiveRequiredTargets(ctx *definitiveContext) error {
	for _, raw := range ctx.base.targets {
		target, _ := raw.(map[string]any)
		if target["requirement"] == "required" && target["state"] != "covered" {
			return fmt.Errorf("subject-definitiveではrequired Targetをcoveredにする必要があります: %s state=%s", target["id"], target["state"])
		}
	}
	return nil
}

func auditSurfaceInventory(ctx *definitiveContext) (int, error) {
	if stringValue(ctx.manifest["authority_extraction"]) != "authority/extraction.snapshot.json" {
		return 0, fmt.Errorf("subject-definitiveにはauthority/extraction.snapshot.jsonが必要です")
	}
	extractionResult, err := AuditAuthorityExtraction(ctx.base.dir, true)
	if err != nil {
		return 0, fmt.Errorf("Authority Extraction Gate: %w", err)
	}
	if stringValue(ctx.manifest["authority_body_inventory"]) != "authority/body-inventory.snapshot.json" || stringValue(ctx.manifest["authority_body_review"]) != "authority/review-queue.snapshot.json" {
		return 0, fmt.Errorf("subject-definitiveにはAuthority body denominatorとReview Queueが必要です")
	}
	_, err = AuditAuthorityBodyInventory(ctx.base.dir, true)
	if err != nil {
		return 0, fmt.Errorf("Authority Body Denominator Gate: %w", err)
	}
	reviewResult, err := AuditAuthorityReviewQueue(ctx.base.dir, true)
	if err != nil {
		return 0, fmt.Errorf("Authority Human Review Gate: %w", err)
	}
	if _, err := AuditAuthorityRelock(ctx.base.dir); err != nil {
		return 0, fmt.Errorf("Authority stale relock Gate: %w", err)
	}
	type extractedSurface struct{ item map[string]any }
	extracted := map[string]extractedSurface{}
	qualifiedExtracted := map[string]bool{}
	artifactIDs := map[string]bool{}
	sourceArtifacts := map[string]bool{}
	for _, raw := range anySlice(ctx.inventory["authority_artifacts"]) {
		artifact, _ := raw.(map[string]any)
		artifactID := stringValue(artifact["id"])
		if artifactIDs[artifactID] {
			return 0, fmt.Errorf("Authority Artifact IDが重複しています: %s", artifactID)
		}
		artifactIDs[artifactID] = true
		path := stringValue(artifact["path"])
		if err := verifyRelativeFileDigest(ctx.base.dir, path, stringValue(artifact["digest"]), -1); err != nil {
			return 0, fmt.Errorf("Authority Artifact %s: %w", artifactID, err)
		}
		full := filepath.Join(ctx.base.dir, filepath.FromSlash(path))
		if _, err := File(full); err != nil {
			return 0, err
		}
		doc, err := readDocument(full)
		if err != nil {
			return 0, err
		}
		sourceID := stringValue(artifact["source_id"])
		if stringValue(doc["source_id"]) != sourceID || !ctx.base.sources[sourceID] {
			return 0, fmt.Errorf("Authority Artifact %sのSourceがLockと一致しません", artifactID)
		}
		if stringValue(doc["source_digest"]) != lockedSourceDigest(ctx.base.documents["sources"], sourceID) {
			return 0, fmt.Errorf("Authority Artifact %sのSource DigestがLock Entryと一致しません", artifactID)
		}
		sourceArtifacts[sourceID] = true
		for _, rawSurface := range anySlice(doc["surfaces"]) {
			surface, _ := rawSurface.(map[string]any)
			key := artifactID + ":" + stringValue(surface["id"])
			if _, duplicate := extracted[key]; duplicate {
				return 0, fmt.Errorf("Authority Surfaceが重複しています: %s", key)
			}
			extracted[key] = extractedSurface{item: surface}
			qualified := qualifiedAuthoritySurfaceID(artifactID, stringValue(surface["id"]))
			if qualifiedExtracted[qualified] {
				return 0, fmt.Errorf("qualified Authority Surface IDが重複しています: %s", qualified)
			}
			qualifiedExtracted[qualified] = true
		}
	}
	for sourceID := range ctx.base.sources {
		if !sourceArtifacts[sourceID] {
			return 0, fmt.Errorf("Authority Lock Sourceに固定Surface Artifactがありません: %s", sourceID)
		}
	}
	seen := map[string]bool{}
	claimOwner := map[string]string{}
	targetOwner := map[string]string{}
	for _, raw := range anySlice(ctx.inventory["items"]) {
		item, _ := raw.(map[string]any)
		key := stringValue(item["authority_artifact_id"]) + ":" + stringValue(item["authority_surface_id"])
		if seen[key] {
			return 0, fmt.Errorf("Surface Inventory項目が重複しています: %s", key)
		}
		seen[key] = true
		source, exists := extracted[key]
		if !exists {
			return 0, fmt.Errorf("Surface InventoryがAuthority Artifactにない項目を参照しています: %s", key)
		}
		if item["classification"] != "included" {
			return 0, fmt.Errorf("subject-definitiveではAuthority由来Surfaceを除外できません: %s", key)
		}
		for _, field := range []string{"locator", "kind", "capability_id", "behavior_id", "title"} {
			if item[field] != source.item[field] {
				return 0, fmt.Errorf("Surface Inventory %sの%sがAuthority Artifactと一致しません", key, field)
			}
		}
		if !sameStringSet(anySlice(item["surface_ids"]), anySlice(source.item["surface_ids"])) {
			return 0, fmt.Errorf("Surface Inventory %sのsurface_idsがAuthority Artifactと一致しません", key)
		}
		if !sameStringSet(anySlice(item["variant_ids"]), anySlice(source.item["variant_ids"])) {
			return 0, fmt.Errorf("Surface Inventory %sのvariant_idsがAuthority Artifactと一致しません", key)
		}
		behaviorID := stringValue(item["behavior_id"])
		if len(anySlice(item["variant_ids"])) == 0 {
			return 0, fmt.Errorf("subject-definitiveではBehaviorにVariantが1件以上必要です: %s", behaviorID)
		}
		if _, duplicate := ctx.behaviors[behaviorID]; duplicate {
			return 0, fmt.Errorf("Behavior IDはAuthority Surface粒度で一意である必要があります: %s", behaviorID)
		}
		ctx.behaviors[behaviorID] = item
		claimIDs := anySlice(item["claim_ids"])
		if len(claimIDs) != 1 {
			return 0, fmt.Errorf("Behavior %sには専用Claimが1件必要です", behaviorID)
		}
		claimID := stringValue(claimIDs[0])
		targetID := stringValue(item["target_id"])
		if owner, used := targetOwner[targetID]; used {
			return 0, fmt.Errorf("集約Targetは禁止です: %sが%sと%sで共有されています", targetID, owner, behaviorID)
		}
		targetOwner[targetID] = behaviorID
		target := coverageTarget(ctx.base.targets, targetID)
		if target == nil || target["requirement"] != "required" || target["state"] != "covered" {
			return 0, fmt.Errorf("Behavior %sには専用のcovered required Targetが必要です: %s", behaviorID, targetID)
		}
		if claimRefs := anySlice(target["claim_ids"]); len(claimRefs) != 1 || stringValue(claimRefs[0]) != claimID {
			return 0, fmt.Errorf("Behavior %sのTargetが専用Claimへ接続されていません", behaviorID)
		}
		claim, ok := ctx.base.claims[claimID]
		if !ok || claim["status"] != "accepted" {
			return 0, fmt.Errorf("Behavior %sのaccepted Claimがありません: %s", behaviorID, claimID)
		}
		if owner, used := claimOwner[claimID]; used {
			return 0, fmt.Errorf("集約Claimは禁止です: %sが%sと%sで共有されています", claimID, owner, behaviorID)
		}
		claimOwner[claimID] = behaviorID
		if stringValue(claim["capability_id"]) != stringValue(item["capability_id"]) {
			return 0, fmt.Errorf("Claim %sのCapabilityがInventoryと一致しません", claimID)
		}
		ctx.claimsByBehavior[behaviorID] = claim
	}
	if len(seen) != len(extracted) {
		return 0, fmt.Errorf("Authority Surface Inventoryに未分類があります: extracted=%d classified=%d", len(extracted), len(seen))
	}
	if extractionResult.CoreV2EligibleSurfaces != len(extracted) {
		return 0, fmt.Errorf("Authority Extractionのeligible Surface数がreview済みArtifact実体と一致しません: eligible=%d artifacts=%d", extractionResult.CoreV2EligibleSurfaces, len(extracted))
	}
	if !sameBoolKeys(qualifiedExtracted, reviewResult.SurfaceIDs) {
		return 0, fmt.Errorf("Core v2 Authority ArtifactはHuman Review Queueのold→new mappingからだけ昇格できます: reviewed_surfaces=%d artifacts=%d", len(reviewResult.SurfaceIDs), len(qualifiedExtracted))
	}
	for behaviorID := range reviewResult.AtomicBehaviorIDs {
		if ctx.behaviors[behaviorID] == nil {
			return 0, fmt.Errorf("Human Review QueueのAtomic Behavior resultがSurface Inventoryにありません: %s", behaviorID)
		}
	}
	return len(extracted), nil
}

func sameBoolKeys(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

func auditDefinitiveProofMatrix(ctx *definitiveContext) (int, int, int, error) {
	proofOwner := map[string]string{}
	for behaviorID, claim := range ctx.claimsByBehavior {
		for _, raw := range anySlice(claim["proof_obligations"]) {
			proof, _ := raw.(map[string]any)
			id := stringValue(proof["id"])
			if _, duplicate := proofOwner[id]; duplicate {
				return 0, 0, 0, fmt.Errorf("Proof Obligation IDが重複しています: %s", id)
			}
			proofOwner[id] = behaviorID
		}
	}
	rows := map[string]map[string]any{}
	usedProof := map[string]bool{}
	usedEvidence := map[string]bool{}
	usedArtifacts := map[string]bool{}
	requiredRows, runtimeEvidence := 0, 0
	for _, raw := range anySlice(ctx.matrix["rows"]) {
		row, _ := raw.(map[string]any)
		behaviorID, scenario := stringValue(row["behavior_id"]), stringValue(row["scenario"])
		if _, ok := ctx.behaviors[behaviorID]; !ok {
			return 0, 0, 0, fmt.Errorf("Verification Matrixが未定義Behaviorを参照しています: %s", behaviorID)
		}
		key := behaviorID + ":" + scenario
		if _, duplicate := rows[key]; duplicate {
			return 0, 0, 0, fmt.Errorf("Verification Matrix Rowが重複しています: %s", key)
		}
		rows[key] = row
		if row["applicability"] != "required" {
			continue
		}
		requiredRows++
		proofID := stringValue(row["proof_obligation_id"])
		if proofOwner[proofID] != behaviorID {
			return 0, 0, 0, fmt.Errorf("Matrix %sのProofがBehavior専用ではありません: %s", key, proofID)
		}
		if usedProof[proofID] {
			return 0, 0, 0, fmt.Errorf("Proof Obligationを複数Scenarioへ集約できません: %s", proofID)
		}
		usedProof[proofID] = true
		evidenceIDs := anySlice(row["evidence_ids"])
		if len(evidenceIDs) != 1 {
			return 0, 0, 0, fmt.Errorf("Matrix %sには専用Evidenceが1件必要です", key)
		}
		evidenceID := stringValue(evidenceIDs[0])
		if usedEvidence[evidenceID] {
			return 0, 0, 0, fmt.Errorf("集約Evidenceは禁止です: %s", evidenceID)
		}
		usedEvidence[evidenceID] = true
		evidence, ok := ctx.base.evidence[evidenceID]
		if !ok || evidence["verdict"] != "pass" {
			return 0, 0, 0, fmt.Errorf("Matrix %sのpass Evidenceがありません: %s", key, evidenceID)
		}
		claimIDs := anySlice(evidence["claim_ids"])
		behaviorClaimID := stringValue(anySlice(ctx.behaviors[behaviorID]["claim_ids"])[0])
		if len(claimIDs) != 1 || stringValue(claimIDs[0]) != behaviorClaimID {
			return 0, 0, 0, fmt.Errorf("Evidence %sがBehavior専用Claimへ接続されていません", evidenceID)
		}
		target := coverageTarget(ctx.base.targets, stringValue(ctx.behaviors[behaviorID]["target_id"]))
		if !containsString(anySlice(target["evidence_ids"]), evidenceID) {
			return 0, 0, 0, fmt.Errorf("Evidence %sがBehavior専用Targetへ接続されていません", evidenceID)
		}
		mode := stringValue(evidence["execution_mode"])
		requirement := stringValue(row["execution_requirement"])
		if requirement == "static-allowed" || (requirement == "runtime" && mode != "runtime") || (requirement == "platform" && mode != "platform") || mode == "fixture" || mode == "static" || mode == "" {
			return 0, 0, 0, fmt.Errorf("Evidence %sのProfile/Execution Modeは%s Proofを代替できません: mode=%s", evidenceID, requirement, mode)
		}
		environment, _ := evidence["environment"].(map[string]any)
		if stringValue(environment["profile"]) != stringValue(row["profile"]) {
			return 0, 0, 0, fmt.Errorf("Evidence %sのProfileがMatrix要件と一致しません: expected=%s actual=%s", evidenceID, row["profile"], environment["profile"])
		}
		if (mode == "runtime" || mode == "platform") && stringValue(evidence["runtime_identity"]) == "" {
			return 0, 0, 0, fmt.Errorf("Evidence %sにRuntime/Platform identityがありません", evidenceID)
		}
		artifact, _ := evidence["artifact"].(map[string]any)
		artifactURI := stringValue(artifact["uri"])
		if usedArtifacts[artifactURI] {
			return 0, 0, 0, fmt.Errorf("Behavior/Scenario専用Artifactが必要です: %s", artifactURI)
		}
		usedArtifacts[artifactURI] = true
		if mode == "runtime" || mode == "platform" {
			runtimeEvidence++
		}
	}
	for behaviorID, item := range ctx.behaviors {
		required := map[string]bool{"normal": true, "boundary": true, "rejection": true}
		for _, rawSurface := range anySlice(item["surface_ids"]) {
			for _, scenario := range scenarioBySurface[stringValue(rawSurface)] {
				required[scenario] = true
			}
		}
		for _, scenario := range definitiveScenarios {
			row, ok := rows[behaviorID+":"+scenario]
			if !ok {
				return 0, 0, 0, fmt.Errorf("Verification Matrixに未分類Scenarioがあります: %s:%s", behaviorID, scenario)
			}
			if required[scenario] && row["applicability"] != "required" {
				return 0, 0, 0, fmt.Errorf("Behavior Surfaceから必須となるScenarioをnot-applicableにできません: %s:%s", behaviorID, scenario)
			}
		}
	}
	if len(rows) != len(ctx.behaviors)*len(definitiveScenarios) {
		return 0, 0, 0, fmt.Errorf("Verification Matrixに余分または不足Rowがあります")
	}
	for proofID := range proofOwner {
		if !usedProof[proofID] {
			return 0, 0, 0, fmt.Errorf("Matrixへ接続されていないProof Obligationがあります: %s", proofID)
		}
	}
	return len(proofOwner), requiredRows, runtimeEvidence, nil
}

func auditReferenceSystemsAndComparisons(ctx *definitiveContext) error {
	integrationBehaviors, comparisonBehaviors := map[string]bool{}, map[string]bool{}
	for behaviorID, item := range ctx.behaviors {
		for _, raw := range anySlice(item["surface_ids"]) {
			surface := stringValue(raw)
			if surface == "architecture-design" || surface == "compatibility-integration" {
				integrationBehaviors[behaviorID] = true
			}
			if surface == "decision-comparison" {
				comparisonBehaviors[behaviorID] = true
			}
		}
	}
	refSystems := anySlice(ctx.manifest["reference_systems"])
	if len(integrationBehaviors) > 0 && len(refSystems) == 0 {
		return fmt.Errorf("適用Surfaceには統合Reference Systemが必要です")
	}
	coveredBehaviors := map[string]bool{}
	for _, raw := range refSystems {
		item, _ := raw.(map[string]any)
		if err := verifyRelativeFileDigest(ctx.base.dir, stringValue(item["path"]), stringValue(item["digest"]), -1); err != nil {
			return fmt.Errorf("Reference System検証: %w", err)
		}
		for _, behavior := range anySlice(item["behavior_ids"]) {
			id := stringValue(behavior)
			if _, ok := ctx.behaviors[id]; !ok {
				return fmt.Errorf("Reference Systemが未定義Behaviorを参照しています: %s", id)
			}
			coveredBehaviors[id] = true
		}
	}
	if len(integrationBehaviors) > 0 && len(coveredBehaviors) < 2 {
		return fmt.Errorf("統合Reference Systemは複数Behaviorを接続する必要があります")
	}
	for behaviorID := range integrationBehaviors {
		if !coveredBehaviors[behaviorID] {
			return fmt.Errorf("適用Behaviorが統合Reference Systemへ接続されていません: %s", behaviorID)
		}
	}
	coveredComparison := map[string]bool{}
	for _, raw := range anySlice(ctx.manifest["comparisons"]) {
		item, _ := raw.(map[string]any)
		for _, behavior := range anySlice(item["behavior_ids"]) {
			id := stringValue(behavior)
			if !comparisonBehaviors[id] {
				return fmt.Errorf("Comparisonが適用外Behaviorを参照しています: %s", id)
			}
			coveredComparison[id] = true
		}
		for _, evidence := range anySlice(item["evidence_ids"]) {
			entity, ok := ctx.base.evidence[stringValue(evidence)]
			if !ok || entity["verdict"] != "pass" {
				return fmt.Errorf("Comparison Evidenceがありません: %s", evidence)
			}
		}
	}
	for behaviorID := range comparisonBehaviors {
		if !coveredComparison[behaviorID] {
			return fmt.Errorf("比較Surfaceに複数方式Comparisonがありません: %s", behaviorID)
		}
	}
	return nil
}

func auditDefinitiveSkill(ctx *definitiveContext) error {
	requiredOutcomes := map[string]bool{"understand": false, "choose": false, "build": false, "verify": false, "operate": false, "troubleshoot": false, "evolve": false, "delegate": false}
	requiredSurfaces := map[string]bool{"orientation-scope": false, "foundations-mechanics": false, "architecture-design": false, "implementation-construction": false, "testing-verification": false, "failure-recovery": false, "operations-observability": false, "security-privacy-safety": false, "performance-capacity-cost": false, "compatibility-integration": false, "migration-evolution-deprecation": false, "decision-comparison": false, "provenance-rights": false, "agent-skill": false}
	gap, authorization := false, false
	for _, raw := range anySlice(ctx.skillEval["cases"]) {
		item, _ := raw.(map[string]any)
		if item["result"] != "pass" {
			continue
		}
		for _, id := range anySlice(item["outcome_ids"]) {
			requiredOutcomes[stringValue(id)] = true
		}
		for _, id := range anySlice(item["surface_ids"]) {
			requiredSurfaces[stringValue(id)] = true
		}
		gap = gap || item["gap_behavior"] == true
		authorization = authorization || item["authorization_boundary"] == true
	}
	for id, ok := range requiredOutcomes {
		if !ok {
			return fmt.Errorf("Definitive Skill EvalがOutcomeをCoverageしていません: %s", id)
		}
	}
	for id, ok := range requiredSurfaces {
		if !ok {
			return fmt.Errorf("Definitive Skill EvalがSurfaceをCoverageしていません: %s", id)
		}
	}
	if !gap || !authorization {
		return fmt.Errorf("Definitive Skill EvalにGap応答と権限境界のpass Caseが必要です")
	}
	return nil
}

func lockedSourceDigest(sources map[string]any, id string) string {
	for _, raw := range anySlice(sources["sources"]) {
		item, _ := raw.(map[string]any)
		if stringValue(item["id"]) == id {
			return stringValue(item["digest"])
		}
	}
	return ""
}

func coverageTarget(targets []any, id string) map[string]any {
	for _, raw := range targets {
		target, _ := raw.(map[string]any)
		if stringValue(target["id"]) == id {
			return target
		}
	}
	return nil
}

func containsString(values []any, expected string) bool {
	for _, value := range values {
		if stringValue(value) == expected {
			return true
		}
	}
	return false
}

func sameStringSet(left, right []any) bool {
	a, b := make([]string, len(left)), make([]string, len(right))
	for i, value := range left {
		a[i] = stringValue(value)
	}
	for i, value := range right {
		b[i] = stringValue(value)
	}
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
