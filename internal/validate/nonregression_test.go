package validate

import (
	"strings"
	"testing"
)

func TestMonotonicBaselineRecapturePreservesDuplicateIDFingerprintMultiset(t *testing.T) {
	oldBaseline := recaptureBaselineWithProofs(
		baselineProof("foundation.authority-digests", "a"),
		baselineProof("foundation.authority-digests", "b"),
	)
	newBaseline := recaptureBaselineWithProofs(
		baselineProof("foundation.authority-digests", "b"),
		baselineProof("foundation.authority-digests", "a"),
		baselineProof("foundation.new-proof", "c"),
	)
	if err := verifyMonotonicBaselineRecapture(oldBaseline, newBaseline); err != nil {
		t.Fatalf("同一ID・異なるfingerprintのProof multisetを保持した再採取を拒否しました: %v", err)
	}
}

func TestMonotonicBaselineRecaptureRejectsMissingDuplicateIDFingerprint(t *testing.T) {
	oldBaseline := recaptureBaselineWithProofs(
		baselineProof("foundation.authority-digests", "a"),
		baselineProof("foundation.authority-digests", "b"),
	)
	newBaseline := recaptureBaselineWithProofs(baselineProof("foundation.authority-digests", "a"))
	err := verifyMonotonicBaselineRecapture(oldBaseline, newBaseline)
	if err == nil || !strings.Contains(err.Error(), "既存項目を削除・変更") {
		t.Fatalf("重複IDの片方を削除した再採取を拒否できません: %v", err)
	}
}

func TestMonotonicBaselineRecapturePreservesDuplicateMultiplicity(t *testing.T) {
	oldBaseline := recaptureBaselineWithProofs(
		baselineProof("foundation.authority-digests", "a"),
		baselineProof("foundation.authority-digests", "a"),
	)
	newBaseline := recaptureBaselineWithProofs(baselineProof("foundation.authority-digests", "a"))
	if err := verifyMonotonicBaselineRecapture(oldBaseline, newBaseline); err == nil {
		t.Fatal("同一(id, fingerprint)の重複数削減を受理しました")
	}
}

func recaptureBaselineWithProofs(proofs ...any) map[string]any {
	return map[string]any{
		"atlas_id":                    "duplicate-proof-atlas",
		"baseline_commit":             strings.Repeat("a", 40),
		"scope_statement_fingerprint": "sha256:" + strings.Repeat("1", 64),
		"scope_exclusions":            []any{"fixed exclusion"},
		"minimum_skill_pass_rate":     float64(1),
		"collections":                 map[string]any{"proof_obligations": proofs},
	}
}

func baselineProof(id, marker string) map[string]any {
	return map[string]any{"id": id, "fingerprint": "sha256:" + strings.Repeat(marker, 64)}
}
