package validate

import (
	"fmt"
	"path/filepath"
)

const feDepthReferenceDigest = "sha256:5872efa1fb7cd385fe9b9c656e545e74e42334e73b1d77575a943418aae84da0"

var feDepthAxes = []string{
	"authority-consumption", "surface-atomic-behavior-variant", "runtime-lab",
	"scenario-normal", "scenario-boundary", "scenario-refusal", "scenario-failure", "scenario-recovery",
	"scenario-migration", "scenario-operations", "scenario-security", "scenario-performance", "scenario-compatibility",
	"artifact-trace", "integrated-reference-system", "skill-eval", "provenance", "non-regression",
}

func auditDepthParity(ctx *definitiveContext) (int, error) {
	if ctx.depthParity["completion_status"] != "parity" {
		return 0, fmt.Errorf("Depth Parityはincompleteです。Gap 0になるまでsubject-definitiveにできません")
	}
	reference, _ := ctx.depthParity["reference"].(map[string]any)
	if stringValue(reference["id"]) != "FE_DEPTH_REFERENCE" || stringValue(reference["digest"]) != feDepthReferenceDigest {
		return 0, fmt.Errorf("Depth Parityは固定FE_DEPTH_REFERENCEへ接続する必要があります")
	}
	path := stringValue(reference["path"])
	if err := verifyRelativeFileDigest(ctx.base.dir, path, feDepthReferenceDigest, -1); err != nil {
		return 0, fmt.Errorf("FE_DEPTH_REFERENCE検証: %w", err)
	}
	if _, err := File(filepath.Join(ctx.base.dir, filepath.FromSlash(path))); err != nil {
		return 0, err
	}
	doc, err := readDocumentBytes(filepath.Join(ctx.base.dir, filepath.FromSlash(path)))
	if err != nil {
		return 0, err
	}
	axisRuntime := map[string]bool{}
	for _, raw := range anySlice(doc["axes"]) {
		axis, _ := raw.(map[string]any)
		axisRuntime[stringValue(axis["id"])] = axis["runtime_required"] == true
	}
	if len(axisRuntime) != len(feDepthAxes) {
		return 0, fmt.Errorf("FE_DEPTH_REFERENCEの必須軸が一致しません")
	}
	for _, axis := range feDepthAxes {
		if _, ok := axisRuntime[axis]; !ok {
			return 0, fmt.Errorf("FE_DEPTH_REFERENCEの必須軸がありません: %s", axis)
		}
	}
	rows := map[string]map[string]any{}
	usedEvidence, usedArtifacts, usedTraces := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, raw := range anySlice(ctx.depthParity["rows"]) {
		row, _ := raw.(map[string]any)
		behaviorID, variantID, axis := stringValue(row["behavior_id"]), stringValue(row["variant_id"]), stringValue(row["axis"])
		if row["status"] != "parity" || numberValue(row["gap_count"]) != 0 {
			return 0, fmt.Errorf("Depth Parity Matrixに未解消Gapがあります: %s:%s:%s", behaviorID, variantID, axis)
		}
		behavior := ctx.behaviors[behaviorID]
		if behavior == nil || !containsString(anySlice(behavior["variant_ids"]), variantID) {
			return 0, fmt.Errorf("Depth Parity Rowが未定義Behavior/Variantを参照しています: %s:%s", behaviorID, variantID)
		}
		key := behaviorID + ":" + variantID + ":" + axis
		if rows[key] != nil {
			return 0, fmt.Errorf("Depth Parity Rowが重複しています: %s", key)
		}
		rows[key] = row
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
		if axisRuntime[axis] && mode != "runtime" && mode != "platform" {
			return 0, fmt.Errorf("Depth Parity軸%sは実Runtime/Platform Evidenceが必要です: %s mode=%s", axis, evidenceID, mode)
		}
		artifact, _ := evidence["artifact"].(map[string]any)
		uri := stringValue(artifact["uri"])
		if uri != stringValue(row["artifact_uri"]) || usedArtifacts[uri] {
			return 0, fmt.Errorf("Depth Parityには専用Artifactが必要です: %s", key)
		}
		usedArtifacts[uri] = true
		trace := stringValue(row["trace_id"])
		if usedTraces[trace] {
			return 0, fmt.Errorf("Depth Parityには専用Traceが必要です: %s", trace)
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
