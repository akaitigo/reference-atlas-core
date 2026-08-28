package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DefinitiveSkillRouterResult struct {
	AtlasID              string
	Cells                int
	Routed               int
	RoutingGaps          int
	PartialCoverageCells int
	BoundaryCases        int
	ForwardEvalCompleted bool
	CompletionLimited    bool
}

var routerOutcomes = []string{"understand", "choose", "build", "verify", "operate", "troubleshoot", "evolve", "delegate"}
var routerSurfaces = []string{"orientation-scope", "foundations-mechanics", "architecture-design", "implementation-construction", "testing-verification", "failure-recovery", "operations-observability", "security-privacy-safety", "performance-capacity-cost", "compatibility-integration", "migration-evolution-deprecation", "decision-comparison", "provenance-rights", "agent-skill"}

func AuditDefinitiveSkillRouter(dir, relative string, requireComplete bool) (DefinitiveSkillRouterResult, error) {
	path := filepath.Join(dir, filepath.FromSlash(relative))
	if filepath.Base(path) == "definitive-skill-router.json" {
		if _, err := File(path); err != nil {
			return DefinitiveSkillRouterResult{}, err
		}
	}
	doc, err := readDocument(path)
	if err != nil {
		return DefinitiveSkillRouterResult{}, err
	}
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return DefinitiveSkillRouterResult{}, err
	}
	if stringValue(doc["atlas_id"]) != stringValue(ctx.documents["atlas"]["id"]) {
		return DefinitiveSkillRouterResult{}, fmt.Errorf("Definitive Skill RouterのAtlas IDが一致しません")
	}
	for _, raw := range doc["source_bindings"].(map[string]any) {
		binding, _ := raw.(map[string]any)
		if err := verifyRelativeFileDigest(dir, stringValue(binding["path"]), stringValue(binding["digest"]), optionalBindingBytes(binding)); err != nil {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router source binding: %w", err)
		}
	}
	lockedSources := map[string]map[string]any{}
	for _, raw := range anySlice(ctx.documents["sources"]["sources"]) {
		source, _ := raw.(map[string]any)
		lockedSources[stringValue(source["id"])] = source
	}
	targets := map[string]map[string]any{}
	for _, raw := range ctx.targets {
		target, _ := raw.(map[string]any)
		targets[stringValue(target["id"])] = target
	}
	seenCells := map[string]bool{}
	evidenceDocumentCache := map[string]map[string]any{}
	routed, gaps, partial, passed, failed := 0, 0, 0, 0, 0
	for _, raw := range anySlice(doc["matrix"]) {
		cell, _ := raw.(map[string]any)
		outcome, surface := stringValue(cell["outcome"]), stringValue(cell["surface"])
		key := outcome + ":" + surface
		if seenCells[key] || !containsStringValue(routerOutcomes, outcome) || !containsStringValue(routerSurfaces, surface) {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerの8 Outcome × 14 Surface cellが不正です: %s", key)
		}
		seenCells[key] = true
		targetID := stringValue(cell["target_id"])
		if targetID != "" && targets[targetID] == nil {
			targetID = stringValue(cell["target_set"]) + "." + targetID
		}
		if targetID == "" || targets[targetID] == nil {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router cellが実Targetへ接続されていません: %s", key)
		}
		cellEvidenceDocuments := []map[string]any{}
		for _, rawBinding := range anySlice(cell["evidence_bindings"]) {
			binding, _ := rawBinding.(map[string]any)
			relative := stringValue(binding["path"])
			if err := verifyRelativeFileDigest(dir, relative, stringValue(binding["digest"]), optionalBindingBytes(binding)); err != nil {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router %s evidence_bindings: %w", key, err)
			}
			if document, ok := evidenceDocumentCache[relative]; ok {
				cellEvidenceDocuments = append(cellEvidenceDocuments, document)
			} else if document, err := readDocument(filepath.Join(dir, filepath.FromSlash(relative))); err == nil {
				evidenceDocumentCache[relative] = document
				cellEvidenceDocuments = append(cellEvidenceDocuments, document)
			}
		}
		for _, rawBinding := range anySlice(cell["implementation_bindings"]) {
			binding, _ := rawBinding.(map[string]any)
			if err := verifyRouterImplementationBinding(dir, binding, cellEvidenceDocuments); err != nil {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router %s implementation_bindings: %w", key, err)
			}
		}
		if len(anySlice(cell["implementation_bindings"])) == 0 || len(anySlice(cell["source_bindings"])) == 0 || len(anySlice(cell["evidence_bindings"])) == 0 {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router cellにVariant/Authority/Evidence digest bindingがありません: %s", key)
		}
		switch cell["mutation_policy"] {
		case "read-only":
			if cell["mutation_status"] != "read-only" {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router read-only mutation境界が不正です: %s", key)
			}
		case "explicit-authorization-required":
			if cell["mutation_status"] != "blocked" && cell["mutation_status"] != "authorized-for-request-scope" {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerが明示許可なしのMutationを停止しません: %s", key)
			}
		case "authorized":
			if cell["mutation_status"] != "authorized" && cell["mutation_status"] != "completed" {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router mutation authorizationと実行状態が一致しません: %s", key)
			}
		}
		for _, rawBinding := range anySlice(cell["source_bindings"]) {
			binding, _ := rawBinding.(map[string]any)
			matched := false
			for id, source := range lockedSources {
				if (stringValue(binding["source_id"]) == "" || stringValue(binding["source_id"]) == id) && binding["url"] == source["url"] && binding["digest"] == source["digest"] {
					matched = true
				}
			}
			if !matched {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router Authority source bindingがSource Lockと一致しません: %s", key)
			}
		}
		if cell["result"] == "pass" {
			passed++
		} else {
			failed++
		}
		if cell["support_status"] == "routed" {
			routed++
		} else {
			gaps++
		}
		if cell["coverage_state"] != "covered" {
			partial++
		}
		if requireComplete {
			if targets[targetID]["state"] != "covered" || cell["support_status"] != "routed" || cell["coverage_state"] != "covered" || cell["result"] != "pass" {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router cellにrouting gap/partial/failureがあります: %s", key)
			}
			if err := auditCompleteRouterBindings(dir, ctx, cell, key); err != nil {
				return DefinitiveSkillRouterResult{}, err
			}
		}
	}
	if len(seenCells) != len(routerOutcomes)*len(routerSurfaces) {
		return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerの8 Outcome × 14 Surface matrixが不完全です: %d/112", len(seenCells))
	}
	boundaryExpected := map[string]string{"boundary.ambiguous": "coverage-gap", "boundary.unknown": "coverage-gap", "boundary.unauthorized-build": "unauthorized-mutation", "boundary.human-authority-decision": "external-human-decision-required", "boundary.stale-relock": "stale-source-relock-explicit-procedure-required"}
	seenBoundaries, boundaryPassed := map[string]bool{}, 0
	for _, raw := range anySlice(doc["boundary_cases"]) {
		cell, _ := raw.(map[string]any)
		id := stringValue(cell["id"])
		expected, required := boundaryExpected[id]
		if !required || seenBoundaries[id] || cell["result"] != "pass" {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router fail-closed境界Caseが不正です: %s", id)
		}
		seenBoundaries[id] = true
		if id == "boundary.ambiguous" || id == "boundary.unknown" {
			if cell["status"] != expected || cell["mutation_status"] != "read-only" {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("曖昧/未知Queryがfail-closedではありません: %s", id)
			}
		} else if cell["status"] != "blocked" || cell["mutation_status"] != "blocked" || !stringSet(anySlice(cell["blocked_reasons"]))[expected] {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("mutation/Authority/relock境界が独立停止しません: %s", id)
		}
		boundaryPassed++
	}
	if len(seenBoundaries) != len(boundaryExpected) {
		return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerの5 fail-closed境界Caseが不足しています")
	}
	summary, _ := doc["summary"].(map[string]any)
	if int(numberValue(summary["matrix_cells"])) != len(seenCells) || int(numberValue(summary["passed"])) != passed || int(numberValue(summary["failed"])) != failed || int(numberValue(summary["routed"])) != routed || int(numberValue(summary["mastery_routing_gaps"])) != gaps || int(numberValue(summary["partial_coverage_cells"])) != partial || int(numberValue(summary["boundary_cases"])) != len(anySlice(doc["boundary_cases"])) || int(numberValue(summary["boundary_passed"])) != boundaryPassed {
		return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router summaryがMatrix/Boundary実体と一致しません")
	}
	forwardComplete := false
	if forward, ok := doc["forward_eval"].(map[string]any); ok {
		forwardComplete = forward["status"] == "completed" && numberValue(forward["cases"]) > 0 && numberValue(forward["failed"]) == 0 && numberValue(forward["passed"]) == numberValue(forward["cases"])
		if forwardComplete {
			if err := verifyRelativeFileDigest(dir, stringValue(forward["artifact_path"]), stringValue(forward["artifact_digest"]), -1); err != nil {
				return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Router Forward Eval: %w", err)
			}
		}
	}
	limited := gaps > 0 || partial > 0 || !forwardComplete || failed > 0 || len(anySlice(doc["completion_limits"])) > 0
	if requireComplete {
		if limited || doc["status"] != "subject-skill-ready" {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerはrouting gap/partial/Forward Eval未実施をCompletion limitとして残しています: gaps=%d partial=%d forward=%t", gaps, partial, forwardComplete)
		}
		if _, err := AuditAuthorityReviewQueue(dir, true); err != nil {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill RouterはHuman Authority decision closureを代替できません: %w", err)
		}
		if _, err := AuditAuthorityRelock(dir); err != nil {
			return DefinitiveSkillRouterResult{}, fmt.Errorf("Skill Routerはstale relock procedureを代替できません: %w", err)
		}
	}
	return DefinitiveSkillRouterResult{AtlasID: stringValue(doc["atlas_id"]), Cells: len(seenCells), Routed: routed, RoutingGaps: gaps, PartialCoverageCells: partial, BoundaryCases: len(seenBoundaries), ForwardEvalCompleted: forwardComplete, CompletionLimited: limited}, nil
}

func containsStringValue(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func optionalBindingBytes(binding map[string]any) int64 {
	if binding["bytes"] == nil {
		return -1
	}
	return int64(numberValue(binding["bytes"]))
}

// A Subject may use either a byte digest for a single implementation file or a
// composite Variant digest (source graph, contract, and local assets). Composite
// digests must be present in an independently digest-verified Evidence record for
// the same Variant ID; merely declaring a different digest beside a path is not a
// binding.
func verifyRouterImplementationBinding(root string, binding map[string]any, evidenceDocuments []map[string]any) error {
	relative, expected, id := stringValue(binding["path"]), stringValue(binding["digest"]), stringValue(binding["id"])
	clean := filepath.Clean(filepath.FromSlash(relative))
	if relative == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("安全でない相対Pathです: %s", relative)
	}
	data, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return err
	}
	if optionalBindingBytes(binding) >= 0 && int64(len(data)) != optionalBindingBytes(binding) {
		return fmt.Errorf("Sizeが一致しません: path=%s", relative)
	}
	if digestBytes(data) == expected {
		return nil
	}
	if id == "" {
		return fmt.Errorf("composite Variant digestにはVariant IDが必要です: path=%s", relative)
	}
	for _, document := range evidenceDocuments {
		if containsRouterDigestRecord(document, id, expected) {
			return nil
		}
	}
	return fmt.Errorf("Variant digestが実Fileまたは検証済みEvidence recordに一致しません: id=%s path=%s", id, relative)
}

func containsRouterDigestRecord(value any, id, digest string) bool {
	wantDigest := strings.TrimPrefix(digest, "sha256:")
	switch typed := value.(type) {
	case map[string]any:
		hasID, hasDigest := false, false
		for key, raw := range typed {
			if key == id || strings.Contains(key, "::"+id+"::") || strings.HasSuffix(key, "::"+id) {
				hasID = true
			}
			if text, ok := raw.(string); ok {
				if text == id || strings.Contains(text, "::"+id+"::") || strings.HasSuffix(text, "::"+id) {
					hasID = true
				}
				if strings.TrimPrefix(text, "sha256:") == wantDigest {
					hasDigest = true
				}
			}
		}
		if hasID && hasDigest {
			return true
		}
		for _, raw := range typed {
			if containsRouterDigestRecord(raw, id, digest) {
				return true
			}
		}
	case []any:
		for _, raw := range typed {
			if containsRouterDigestRecord(raw, id, digest) {
				return true
			}
		}
	}
	return false
}

func auditCompleteRouterBindings(dir string, ctx *auditContext, cell map[string]any, key string) error {
	inventory, err := readDocument(filepath.Join(dir, "surface.inventory.yaml"))
	if err != nil {
		return err
	}
	variants, authorityItems := map[string]bool{}, map[string]bool{}
	for _, raw := range anySlice(inventory["items"]) {
		item, _ := raw.(map[string]any)
		for _, rawID := range anySlice(item["variant_ids"]) {
			variants[stringValue(rawID)] = true
		}
		authorityItems[qualifiedAuthoritySurfaceID(stringValue(item["authority_artifact_id"]), stringValue(item["authority_surface_id"]))] = true
	}
	if len(anySlice(cell["variant_ids"])) == 0 || len(anySlice(cell["authority_item_ids"])) == 0 || len(anySlice(cell["runtime_evidence_bindings"])) == 0 {
		return fmt.Errorf("Skill Router complete cellにVariant/Authority/Runtime Evidence ID bindingがありません: %s", key)
	}
	for _, raw := range anySlice(cell["variant_ids"]) {
		if !variants[stringValue(raw)] {
			return fmt.Errorf("Skill Router VariantがInventoryにありません: %s", raw)
		}
	}
	for _, raw := range anySlice(cell["authority_item_ids"]) {
		if !authorityItems[stringValue(raw)] {
			return fmt.Errorf("Skill Router Authority itemがInventoryにありません: %s", raw)
		}
	}
	for _, raw := range anySlice(cell["runtime_evidence_bindings"]) {
		binding, _ := raw.(map[string]any)
		id := stringValue(binding["evidence_id"])
		evidence := ctx.evidence[id]
		if evidence == nil || evidence["verdict"] != "pass" || (evidenceExecutionMode(evidence) != "runtime" && evidenceExecutionMode(evidence) != "platform") {
			return fmt.Errorf("Skill Router Runtime Evidenceがpass実体ではありません: %s", id)
		}
		if err := verifyRelativeFileDigest(dir, stringValue(binding["path"]), stringValue(binding["digest"]), -1); err != nil {
			return err
		}
	}
	return nil
}
