package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func GenerateDefinitiveCertificate(dir, issuedAt, commit string) (string, error) {
	if issuedAt == "" {
		issuedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, issuedAt); err != nil {
		return "", fmt.Errorf("issued-atがRFC3339ではありません: %w", err)
	}
	if commit == "" {
		output, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			return "", fmt.Errorf("証明対象Commitを取得できません。--commitで指定してください: %w", err)
		}
		commit = strings.TrimSpace(string(output))
	}
	if err := ensureSourceCommitExists(dir, commit); err != nil {
		return "", err
	}
	certificate, path, err := buildDefinitiveCertificate(dir, issuedAt, commit)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(certificate, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, path); err != nil {
		return "", err
	}
	return path, nil
}

func VerifyDefinitiveCertificate(dir string) error {
	ctx, err := loadDefinitiveContext(dir)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filepath.FromSlash(stringValue(ctx.manifest["certificate"])))
	if _, err := File(path); err != nil {
		return err
	}
	actual, err := readJSONDocument(path)
	if err != nil {
		return err
	}
	expected, _, err := buildDefinitiveCertificate(dir, stringValue(actual["issued_at"]), stringValue(actual["commit"]))
	if err != nil {
		return err
	}
	actualDigest, err := digestCanonical(actual)
	if err != nil {
		return err
	}
	expectedDigest, err := digestCanonical(expected)
	if err != nil {
		return err
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("Subject Definitive Certificateが現在のAuthority Inventory・Proof Matrix・Evidenceと一致しません")
	}
	return nil
}

func buildDefinitiveCertificate(dir, issuedAt, commit string) (map[string]any, string, error) {
	ctx, err := loadDefinitiveBase(dir)
	if err != nil {
		return nil, "", err
	}
	if stringValue(ctx.base.documents["atlas"]["status"]) != "complete" {
		return nil, "", fmt.Errorf("historical bounded-complete基盤は検証済みですが、subject-definitive Certificate生成にはatlas.status=completeが必要です")
	}
	if err := auditDefinitiveRequiredTargets(ctx); err != nil {
		return nil, "", err
	}
	if _, err := auditSurfaceInventory(ctx); err != nil {
		return nil, "", err
	}
	if _, _, _, err := auditDefinitiveProofMatrix(ctx); err != nil {
		return nil, "", err
	}
	if err := auditReferenceSystemsAndComparisons(ctx); err != nil {
		return nil, "", err
	}
	if err := auditDefinitiveSkill(ctx); err != nil {
		return nil, "", err
	}
	if _, err := AuditNonRegression(dir); err != nil {
		return nil, "", fmt.Errorf("Definitive non-regression Gate: %w", err)
	}
	if _, err := auditDepthParity(ctx); err != nil {
		return nil, "", err
	}
	if err := auditDefinitivePromotionFoundation(ctx); err != nil {
		return nil, "", err
	}
	if err := validateCommit(commit); err != nil {
		return nil, "", err
	}
	inventoryData, err := os.ReadFile(filepath.Join(dir, "surface.inventory.yaml"))
	if err != nil {
		return nil, "", err
	}
	matrixData, err := os.ReadFile(filepath.Join(dir, "verification.matrix.yaml"))
	if err != nil {
		return nil, "", err
	}
	depthParityData, err := os.ReadFile(filepath.Join(dir, "depth.parity.yaml"))
	if err != nil {
		return nil, "", err
	}
	skillData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(stringValue(ctx.manifest["skill_eval"]))))
	if err != nil {
		return nil, "", err
	}
	historical := []any{}
	for _, raw := range anySlice(ctx.manifest["historical_certificates"]) {
		item, _ := raw.(map[string]any)
		path := stringValue(item["path"])
		full := filepath.Join(dir, filepath.FromSlash(path))
		if _, err := File(full); err != nil {
			return nil, "", fmt.Errorf("bounded historical Certificateが無効です: %w", err)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, "", err
		}
		historical = append(historical, map[string]any{"path": path, "classification": "bounded-complete", "digest": digestBytes(data)})
	}
	proofGraphDigest, err := digestCanonical(map[string]any{"inventory": ctx.inventory["items"], "claims": orderedEntities(ctx.base.claims), "matrix": ctx.matrix["rows"], "evidence": orderedEntities(ctx.base.evidence)})
	if err != nil {
		return nil, "", err
	}
	referenceDigest, err := digestCanonical(map[string]any{"reference_systems": ctx.manifest["reference_systems"], "comparisons": ctx.manifest["comparisons"]})
	if err != nil {
		return nil, "", err
	}
	coverageConfig, _ := ctx.base.documents["atlas"]["coverage"].(map[string]any)
	nonRegressionData, err := os.ReadFile(filepath.Join(dir, "non-regression.yaml"))
	if err != nil {
		return nil, "", err
	}
	payload := map[string]any{
		"schema_version": 2, "completion_class": "subject-definitive",
		"atlas_id": stringValue(ctx.base.documents["atlas"]["id"]), "atlas_release": stringValue(ctx.base.documents["skill"]["atlas_release"]),
		"coverage_epoch": stringValue(coverageConfig["epoch"]), "authority_lock_digest": stringValue(ctx.base.documents["coverage"]["authority_lock_digest"]),
		"surface_inventory_digest": digestBytes(inventoryData), "verification_matrix_digest": digestBytes(matrixData),
		"depth_parity_digest": digestBytes(depthParityData),
		"proof_graph_digest":  proofGraphDigest, "skill_eval_digest": digestBytes(skillData), "reference_system_digest": referenceDigest,
		"non_regression_digest":   digestBytes(nonRegressionData),
		"historical_certificates": historical, "issued_at": issuedAt, "commit": commit,
	}
	signature, err := digestCanonical(payload)
	if err != nil {
		return nil, "", err
	}
	payload["signature"] = map[string]any{"type": "payload-sha256", "digest": signature}
	return payload, filepath.Join(dir, filepath.FromSlash(stringValue(ctx.manifest["certificate"]))), nil
}
