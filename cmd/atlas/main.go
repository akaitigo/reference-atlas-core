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
	if len(args) < 2 {
		return fmt.Errorf("使い方: atlas validate <manifest...> | atlas audit <atlas-directory>")
	}
	switch args[0] {
	case "validate":
		for _, path := range args[1:] {
			result, err := validate.File(path)
			if err != nil {
				return err
			}
			fmt.Printf("検証済み: %s (%s)\n", path, result.Schema)
		}
		return nil
	case "audit":
		if len(args) != 2 {
			return fmt.Errorf("使い方: atlas audit <atlas-directory>")
		}
		result, err := validate.AuditDir(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("監査済み: %s status=%s mastery=%d target_sets=%d targets=%d open_required=%d\n",
			result.AtlasID, result.Status, result.MasteryAreas, result.TargetSets, result.Targets, result.OpenRequired)
		return nil
	default:
		return fmt.Errorf("未知のCommandです: %s", args[0])
	}
}
