package main

import (
	"fmt"
	"os"

	"github.com/akaitigo/reference-atlas-core/internal/validate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 || args[0] != "validate" {
		return fmt.Errorf("使い方: atlas validate <manifest.yaml> [manifest.yaml ...]")
	}

	for _, path := range args[1:] {
		result, err := validate.File(path)
		if err != nil {
			return err
		}
		fmt.Printf("検証済み: %s (%s)\n", path, result.Schema)
	}
	return nil
}
