package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRouterImplementationBindingAcceptsVerifiedCompositeVariantDigest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entry.ts"), []byte("export const value = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binding := map[string]any{
		"id":     "variant-a",
		"path":   "entry.ts",
		"digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	evidence := []map[string]any{{
		"records": []any{map[string]any{
			"id":         "subject.target::variant-a::runtime",
			"sourceHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}}
	if err := verifyRouterImplementationBinding(dir, binding, evidence); err != nil {
		t.Fatalf("検証済みEvidence recordに束縛されたcomposite Variant digestを拒否しました: %v", err)
	}
}

func TestRouterImplementationBindingRejectsUnrelatedCompositeDigest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "entry.ts"), []byte("export const value = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binding := map[string]any{
		"id":     "variant-a",
		"path":   "entry.ts",
		"digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	evidence := []map[string]any{{
		"records": []any{map[string]any{
			"id":         "subject.target::variant-b::runtime",
			"sourceHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}}
	if err := verifyRouterImplementationBinding(dir, binding, evidence); err == nil {
		t.Fatal("別VariantのEvidence digestで実装bindingを代替できました")
	}
}
