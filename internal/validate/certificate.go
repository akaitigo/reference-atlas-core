package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateCertificate creates the derived completion certificate. The commit is
// the source commit being certified; the certificate itself may be committed or
// attached to a release afterwards, avoiding a self-referential Git hash.
func GenerateCertificate(dir, issuedAt, commit string) (string, error) {
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
	certificate, path, err := buildCertificate(dir, issuedAt, commit)
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

func VerifyCertificate(dir string) error {
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return err
	}
	completion, _ := ctx.documents["atlas"]["completion"].(map[string]any)
	path := filepath.Join(dir, filepath.FromSlash(stringValue(completion["certificate"])))
	if _, err := File(path); err != nil {
		return err
	}
	actual, err := readJSONDocument(path)
	if err != nil {
		return err
	}
	expected, _, err := buildCertificate(dir, stringValue(actual["issued_at"]), stringValue(actual["commit"]))
	if err != nil {
		return err
	}
	actualDigest, digestErr := digestCanonical(actual)
	if digestErr != nil {
		return digestErr
	}
	expectedDigest, digestErr := digestCanonical(expected)
	if digestErr != nil {
		return digestErr
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("Completion Certificateが現在のManifest・Evidence・Artifactと一致しません。再生成してください")
	}
	return nil
}

func buildCertificate(dir, issuedAt, commit string) (map[string]any, string, error) {
	ctx, err := loadAuditContext(dir)
	if err != nil {
		return nil, "", err
	}
	if stringValue(ctx.documents["atlas"]["status"]) != "complete" {
		return nil, "", fmt.Errorf("Certificateはstatus=completeのAtlasにだけ生成できます")
	}
	if countOpenRequired(ctx.targets) != 0 {
		return nil, "", fmt.Errorf("未Closureの必須TargetがあるためCertificateを生成できません")
	}
	if err := ensureMasteryCovered(ctx.documents["mastery"], ctx.targets); err != nil {
		return nil, "", err
	}
	if err := verifyAuthorityDigest(filepath.Join(dir, "sources.lock.yaml"), ctx.documents["coverage"]); err != nil {
		return nil, "", err
	}
	if err := auditClaimEvidenceGraph(ctx); err != nil {
		return nil, "", err
	}
	if err := auditProfiles(ctx); err != nil {
		return nil, "", err
	}
	eval, err := skillEvalSummary(ctx)
	if err != nil {
		return nil, "", err
	}
	atlas := ctx.documents["atlas"]
	license, _ := atlas["license"].(map[string]any)
	sbomPath := filepath.Join(dir, filepath.FromSlash(stringValue(license["sbom"])))
	if err := auditSBOM(sbomPath); err != nil {
		return nil, "", err
	}
	if err := auditSupplyChain(ctx, stringValue(license["sbom"]), stringValue(license["third_party_manifest"])); err != nil {
		return nil, "", err
	}
	if err := auditProvenance(ctx); err != nil {
		return nil, "", err
	}
	if err := validateCommit(commit); err != nil {
		return nil, "", err
	}

	graphDigest, err := digestCanonical(map[string]any{
		"coverage": ctx.documents["coverage"], "claims": orderedEntities(ctx.claims), "evidence": orderedEntities(ctx.evidence),
	})
	if err != nil {
		return nil, "", err
	}
	evidenceDigest, err := digestCanonical(orderedEntities(ctx.evidence))
	if err != nil {
		return nil, "", err
	}
	skillData, err := os.ReadFile(filepath.Join(dir, "skill.package.yaml"))
	if err != nil {
		return nil, "", err
	}
	sbomData, err := os.ReadFile(sbomPath)
	if err != nil {
		return nil, "", err
	}
	provenanceData, err := os.ReadFile(filepath.Join(dir, "provenance.yaml"))
	if err != nil {
		return nil, "", err
	}
	profiles := certificateProfiles(ctx)
	coverageConfig, _ := atlas["coverage"].(map[string]any)
	completion, _ := atlas["completion"].(map[string]any)
	payload := map[string]any{
		"schema_version":        1,
		"atlas_id":              stringValue(atlas["id"]),
		"atlas_release":         stringValue(ctx.documents["skill"]["atlas_release"]),
		"coverage_epoch":        stringValue(coverageConfig["epoch"]),
		"authority_lock_digest": stringValue(ctx.documents["coverage"]["authority_lock_digest"]),
		"core_policy_version":   stringValue(completion["policy_version"]),
		"required_profiles":     profiles,
		"graph_digest":          graphDigest,
		"evidence_set_digest":   evidenceDigest,
		"skill_package_digest":  digestBytes(skillData),
		"skill_eval":            map[string]any{"digest": eval.digest, "pass_rate": eval.passRate},
		"sbom_digest":           digestBytes(sbomData),
		"provenance_digest":     digestBytes(provenanceData),
		"issued_at":             issuedAt,
		"commit":                commit,
	}
	signature, err := digestCanonical(payload)
	if err != nil {
		return nil, "", err
	}
	payload["signature"] = map[string]any{"type": "payload-sha256", "digest": signature}
	return payload, filepath.Join(dir, filepath.FromSlash(stringValue(completion["certificate"]))), nil
}

func validateCommit(commit string) error {
	if len(commit) != 40 || strings.Trim(commit, "0123456789abcdef") != "" {
		return fmt.Errorf("commitは40桁の小文字Git SHAである必要があります")
	}
	return nil
}

// ensureSourceCommitExists rejects transcription mistakes when generating in a
// Git worktree. Verification remains content-addressed and does not require the
// commit object so historical certificates continue to work in shallow clones.
func ensureSourceCommitExists(dir, commit string) error {
	if err := validateCommit(commit); err != nil {
		return err
	}
	if err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return nil
	}
	if err := exec.Command("git", "-C", dir, "cat-file", "-e", commit+"^{commit}").Run(); err != nil {
		return fmt.Errorf("証明対象CommitがGit履歴に存在しません: %s", commit)
	}
	return nil
}

func certificateProfiles(ctx *auditContext) []any {
	byProfile := map[string][]string{}
	for id, evidence := range ctx.evidence {
		if evidence["verdict"] != "pass" {
			continue
		}
		environment, _ := evidence["environment"].(map[string]any)
		profile := stringValue(environment["profile"])
		byProfile[profile] = append(byProfile[profile], id)
	}
	completion, _ := ctx.documents["atlas"]["completion"].(map[string]any)
	profiles := make([]any, 0, len(anySlice(completion["required_profiles"])))
	for _, raw := range anySlice(completion["required_profiles"]) {
		profile := stringValue(raw)
		sort.Strings(byProfile[profile])
		ids := make([]any, len(byProfile[profile]))
		for i, id := range byProfile[profile] {
			ids[i] = id
		}
		profiles = append(profiles, map[string]any{"profile": profile, "result": "pass", "evidence_ids": ids})
	}
	return profiles
}

func orderedEntities(entities map[string]map[string]any) []any {
	keys := make([]string, 0, len(entities))
	for key := range entities {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, entities[key])
	}
	return result
}

func digestCanonical(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func readJSONDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
