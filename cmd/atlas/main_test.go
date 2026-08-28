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
