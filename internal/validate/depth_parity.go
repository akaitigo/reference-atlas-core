package validate

import (
	"fmt"
	"path/filepath"
)

const (
	feDepthReferenceDigest = "sha256:2452696f9807b7d4a8ffb22b3ba37f079a25a34ac2370d78423445b96064582a"
	feDepthReferenceCommit = "4a0b2df8e2091a963bd0e0e1bbccef9c84b49a45"
)

var feDepthAxes = []string{
	"authority-body-digestion", "surface-atomic-behavior-variant", "real-runtime-lab",
	"scenario-normal", "scenario-boundary", "scenario-refusal", "scenario-failure", "scenario-recovery",
	"scenario-migration", "scenario-operations", "scenario-security", "scenario-performance", "scenario-compatibility",
	"artifact-trace", "integrated-reference-system", "skill-eval", "rights-provenance", "non-regression-gate",
}

var feDepthRuntimeAxes = map[string]bool{
	"real-runtime-lab": true, "scenario-normal": true, "scenario-boundary": true,
	"scenario-refusal": true, "scenario-failure": true, "scenario-recovery": true,
	"scenario-migration": true, "scenario-operations": true, "scenario-security": true,
	"scenario-performance": true, "scenario-compatibility": true, "artifact-trace": true,
	"integrated-reference-system": true, "non-regression-gate": true,
}

func auditDepthParity(ctx *definitiveContext) (int, error) {
	if ctx.depthParity["completion_status"] != "parity" {
		return 0, fmt.Errorf("Depth Parityはincompleteです。Gap 0になるまでsubject-definitiveにできません")
	}
	reference, _ := ctx.depthParity["reference"].(map[string]any)
	if stringValue(reference["id"]) != "fe-depth-reference-v1" ||
		stringValue(reference["digest"]) != feDepthReferenceDigest ||
		stringValue(reference["repository"]) != "frontend-behavior-atlas" ||
		stringValue(reference["commit"]) != feDepthReferenceCommit ||
		stringValue(reference["status_at_commit"]) != "incomplete" {
		return 0, fmt.Errorf("Depth Parityは固定FE Depth Reference正本と、そのincomplete状態へ接続する必要があります")
	}
	policy, _ := ctx.depthParity["denominator_policy"].(map[string]any)
	if stringValue(policy["source"]) != "authority-derived-subject-surface-inventory" || policy["transplant_absolute_counts"] != false {
		return 0, fmt.Errorf("Depth Parityの母集団はSubject自身のAuthority由来Inventoryでなければならず、FEの絶対件数を転用できません")
	}
	path := stringValue(reference["path"])
	if err := verifyRelativeFileDigest(ctx.base.dir, path, feDepthReferenceDigest, -1); err != nil {
		return 0, fmt.Errorf("FE Depth Reference検証: %w", err)
	}
	full := filepath.Join(ctx.base.dir, filepath.FromSlash(path))
	if _, err := File(full); err != nil {
		return 0, err
	}
	doc, err := readDocumentBytes(full)
	if err != nil {
		return 0, err
	}
	if err := auditFEDepthReferenceContract(doc); err != nil {
		return 0, err
	}

	rows := map[string]map[string]any{}
	usedProofs, usedEvidence := map[string]bool{}, map[string]bool{}
	usedArtifacts, usedTraces := map[string]bool{}, map[string]bool{}
	for _, raw := range anySlice(ctx.depthParity["rows"]) {
		row, _ := raw.(map[string]any)
		behaviorID, variantID, axis := stringValue(row["behavior_id"]), stringValue(row["variant_id"]), stringValue(row["axis"])
		if row["status"] != "satisfied" || numberValue(row["gap_count"]) != 0 {
			return 0, fmt.Errorf("Depth Parity Matrixに未解消Gapがあります: %s:%s:%s", behaviorID, variantID, axis)
		}
		behavior := ctx.behaviors[behaviorID]
		if behavior == nil || !containsString(anySlice(behavior["variant_ids"]), variantID) {
			return 0, fmt.Errorf("Depth Parity Rowが未定義のAuthority由来Behavior/Variantを参照しています: %s:%s", behaviorID, variantID)
		}
		key := behaviorID + ":" + variantID + ":" + axis
		if rows[key] != nil {
			return 0, fmt.Errorf("Depth Parity Rowが重複しています: %s", key)
		}
		rows[key] = row
		proofID := stringValue(row["proof_id"])
		if proofID == "" || usedProofs[proofID] {
			return 0, fmt.Errorf("Depth Parityには軸・Behavior・Variant専用の反証可能Proofが必要です: %s", key)
		}
		usedProofs[proofID] = true
		if len(stringValue(row["oracle"])) < 20 {
			return 0, fmt.Errorf("Depth Parity Proofに専用Oracleがありません: %s", proofID)
		}
		evidenceID := stringValue(anySlice(row["evidence_ids"])[0])
		if usedEvidence[evidenceID] {
			return 0, fmt.Errorf("Depth Parityで集約Evidenceは禁止です: %s", evidenceID)
		}
		usedEvidence[evidenceID] = true
		evidence := ctx.base.evidence[evidenceID]
		if evidence == nil || evidence["verdict"] != "pass" {
			return 0, fmt.Errorf("Depth Parityのpass Evidenceがありません: %s", evidenceID)
		}
		claimID := stringValue(anySlice(behavior["claim_ids"])[0])
		if claims := anySlice(evidence["claim_ids"]); len(claims) != 1 || stringValue(claims[0]) != claimID {
			return 0, fmt.Errorf("Depth Parity EvidenceがAtomic Behavior専用Claimへ接続されていません: %s", evidenceID)
		}
		target := coverageTarget(ctx.base.targets, stringValue(behavior["target_id"]))
		if !containsString(anySlice(target["evidence_ids"]), evidenceID) {
			return 0, fmt.Errorf("Depth Parity Evidenceが専用Targetへ接続されていません: %s", evidenceID)
		}
		mode := evidenceExecutionMode(evidence)
		if feDepthRuntimeAxes[axis] && mode != "runtime" && mode != "platform" {
			return 0, fmt.Errorf("Depth Parity軸%sは実Runtime/Platform Evidenceが必要です: %s mode=%s", axis, evidenceID, mode)
		}
		artifact, _ := evidence["artifact"].(map[string]any)
		uri := stringValue(artifact["uri"])
		if uri != stringValue(row["artifact_uri"]) || usedArtifacts[uri] {
			return 0, fmt.Errorf("Depth Parityには専用Artifactが必要です: %s", key)
		}
		usedArtifacts[uri] = true
		trace := stringValue(row["trace_id"])
		if trace == "" || usedTraces[trace] {
			return 0, fmt.Errorf("Depth Parityには専用Traceが必要です: %s", key)
		}
		usedTraces[trace] = true
	}
	expected := 0
	for behaviorID, behavior := range ctx.behaviors {
		for _, rawVariant := range anySlice(behavior["variant_ids"]) {
			variantID := stringValue(rawVariant)
			for _, axis := range feDepthAxes {
				expected++
				if rows[behaviorID+":"+variantID+":"+axis] == nil {
					return 0, fmt.Errorf("Depth Parity MatrixにGapがあります: %s:%s:%s", behaviorID, variantID, axis)
				}
			}
		}
	}
	if len(rows) != expected {
		return 0, fmt.Errorf("Depth Parity Matrixに余分または不足Rowがあります: expected=%d actual=%d", expected, len(rows))
	}
	covered := map[string]bool{}
	for _, raw := range anySlice(ctx.manifest["reference_systems"]) {
		item, _ := raw.(map[string]any)
		for _, behavior := range anySlice(item["behavior_ids"]) {
			covered[stringValue(behavior)] = true
		}
	}
	if len(covered) < 2 {
		return 0, fmt.Errorf("Depth Parityには複数Atomic Behaviorを統合するReference Systemが必要です")
	}
	for behaviorID := range ctx.behaviors {
		if !covered[behaviorID] {
			return 0, fmt.Errorf("Atomic Behaviorが統合Reference Systemへ接続されていません: %s", behaviorID)
		}
	}
	return len(rows), nil
}

func auditFEDepthReferenceContract(doc map[string]any) error {
	if stringValue(doc["id"]) != "fe-depth-reference-v1" || stringValue(doc["status"]) != "incomplete" ||
		stringValue(doc["completionClaim"]) != "neither-bounded-complete-nor-subject-definitive" {
		return fmt.Errorf("FE Depth ReferenceはFrontendをcomplete扱いしない正本でなければなりません")
	}
	policy, _ := doc["denominatorPolicy"].(map[string]any)
	if stringValue(policy["source"]) != "authority-derived-subject-surface-inventory" || policy["transplantAbsoluteCounts"] != false {
		return fmt.Errorf("FE Depth ReferenceはSubject固有のAuthority由来denominatorと絶対件数非転用を要求する必要があります")
	}
	decision, _ := doc["parityDecision"].(map[string]any)
	if decision["frontendCurrentlyDefinitive"] != false {
		return fmt.Errorf("FE Depth ReferenceがFrontendをsubject-definitive扱いしています")
	}
	summary, _ := doc["summary"].(map[string]any)
	if numberValue(summary["satisfied"]) != 1 || numberValue(summary["partial"]) != 17 || numberValue(summary["missing"]) != 0 {
		return fmt.Errorf("FE Depth Referenceの現状は1 satisfied / 17 partial / 0 missingでなければなりません")
	}
	axisStatus := map[string]string{}
	for _, raw := range anySlice(doc["axes"]) {
		axis, _ := raw.(map[string]any)
		id := stringValue(axis["id"])
		if axisStatus[id] != "" {
			return fmt.Errorf("FE Depth Referenceの軸が重複しています: %s", id)
		}
		if stringValue(axis["portableCriterion"]) == "" || stringValue(axis["denominator"]) == "" {
			return fmt.Errorf("FE Depth Referenceの軸にportable criterionまたはdenominatorがありません: %s", id)
		}
		axisStatus[id] = stringValue(axis["status"])
	}
	if len(axisStatus) != len(feDepthAxes) {
		return fmt.Errorf("FE Depth Referenceの必須18軸が一致しません")
	}
	for _, axis := range feDepthAxes {
		if axisStatus[axis] == "" {
			return fmt.Errorf("FE Depth Referenceの必須軸がありません: %s", axis)
		}
	}
	if axisStatus["non-regression-gate"] != "satisfied" {
		return fmt.Errorf("FE Depth Referenceの唯一のsatisfied軸はnon-regression-gateでなければなりません")
	}
	for axis, status := range axisStatus {
		if axis != "non-regression-gate" && status != "partial" {
			return fmt.Errorf("FE Depth Referenceの軸%sは現状partialでなければなりません", axis)
		}
	}
	return nil
}
