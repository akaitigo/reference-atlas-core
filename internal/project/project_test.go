package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akaitigo/reference-atlas-core/internal/project"
	"github.com/akaitigo/reference-atlas-core/internal/validate"
)

func TestScaffoldProducesAuditableIncompleteSubject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sample-reference-atlas")
	if err := project.Scaffold(dir, "sample-reference-atlas", "サンプル技術アトラス", "2026-08-28"); err != nil {
		t.Fatal(err)
	}
	result, err := validate.AuditDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "incomplete" || result.OpenRequired != 1 {
		t.Fatalf("予期しないScaffold監査結果: %+v", result)
	}
	if err := project.Scaffold(dir, "sample-reference-atlas", "上書き", "2026-08-28"); err == nil {
		t.Fatal("空でない出力先は拒否する必要があります")
	}
}

func TestMigrationIsNonDestructiveAndIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sample-reference-atlas")
	if err := project.Scaffold(dir, "sample-reference-atlas", "サンプル技術アトラス", "2026-08-28"); err != nil {
		t.Fatal(err)
	}
	masteryPath := filepath.Join(dir, "mastery.yaml")
	if err := os.Remove(masteryPath); err != nil {
		t.Fatal(err)
	}
	created, err := project.MigrateV1(dir, "2026-08-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("MasteryとMigration Mapの2件を期待しました: %v", created)
	}
	migrationPath := filepath.Join(dir, "migrations", "core-v1.yaml")
	if _, err := validate.File(migrationPath); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	created, err = project.MigrateV1(dir, "2027-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 || string(before) != string(after) {
		t.Fatal("再実行は既存Fileを変更してはいけません")
	}
}

func TestDefinitiveV2MigrationPreservesBoundedCertificate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sample-reference-atlas")
	if err := project.Scaffold(dir, "sample-reference-atlas", "サンプル技術アトラス", "2026-08-28"); err != nil {
		t.Fatal(err)
	}
	certificatePath := filepath.Join(dir, "evidence", "completion-certificate.json")
	if err := os.WriteFile(certificatePath, []byte("{\"historical\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := project.MigrateDefinitiveV2(dir, "2026-08-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("Migration記録が生成されませんでした")
	}
	if _, err := validate.File(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "classification: bounded-complete") || !strings.Contains(string(data), "inventory-required") {
		t.Fatalf("bounded履歴または未完状態が記録されていません:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "evidence", "history", "v0.1.0", "completion-certificate.json")); err != nil {
		t.Fatalf("bounded Certificate履歴が不変Pathへ保存されていません: %v", err)
	}
	second, err := project.MigrateDefinitiveV2(dir, "2027-01-01T00:00:00Z")
	if err != nil || second != "" {
		t.Fatalf("Migration再実行は既存記録を変更してはいけません: path=%s err=%v", second, err)
	}
}
