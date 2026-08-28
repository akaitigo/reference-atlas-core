package main

import "testing"

func TestRunRequiresValidateSubcommand(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("引数不足は失敗する必要があります")
	}
	if err := run([]string{"unknown", "atlas.yaml"}); err == nil {
		t.Fatal("未知のSubcommandは失敗する必要があります")
	}
}

func TestVersion(t *testing.T) {
	if version != "1.0.0" {
		t.Fatalf("v1 releaseではversionを固定します: %s", version)
	}
	if err := run([]string{"version"}); err != nil {
		t.Fatal(err)
	}
}
