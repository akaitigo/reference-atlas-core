package validate

import (
	"path/filepath"
	"testing"
)

func TestCorePassesItsOwnCompletionGate(t *testing.T) {
	root := filepath.Join("..", "..")
	result, err := AuditDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.CompletionClass != "bounded-complete" || result.OpenRequired != 0 || result.Claims == 0 || result.Evidence == 0 {
		t.Fatalf("Coreをcompleteとして受理できません: %+v", result)
	}
}
