package main

import (
	"fmt"
	"os"
	"time"

	"github.com/akaitigo/reference-atlas-core/internal/project"
	"github.com/akaitigo/reference-atlas-core/internal/validate"
)

const version = "1.0.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "validate":
		if len(args) < 2 {
			return usageError()
		}
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
		fmt.Printf("監査済み: %s status=%s mastery=%d target_sets=%d targets=%d claims=%d evidence=%d open_required=%d\n",
			result.AtlasID, result.Status, result.MasteryAreas, result.TargetSets, result.Targets, result.Claims, result.Evidence, result.OpenRequired)
		return nil
	case "certificate":
		if len(args) < 3 {
			return usageError()
		}
		switch args[1] {
		case "generate":
			issuedAt, commit, err := certificateOptions(args[3:])
			if err != nil {
				return err
			}
			path, err := validate.GenerateCertificate(args[2], issuedAt, commit)
			if err != nil {
				return err
			}
			fmt.Printf("Completion Certificateを生成しました: %s\n", path)
			return nil
		case "verify":
			if len(args) != 3 {
				return usageError()
			}
			if err := validate.VerifyCertificate(args[2]); err != nil {
				return err
			}
			fmt.Printf("Completion Certificateを検証しました: %s\n", args[2])
			return nil
		default:
			return usageError()
		}
	case "scaffold":
		if len(args) < 4 || len(args) > 5 {
			return usageError()
		}
		epoch := time.Now().UTC().Format("2006-01-02")
		if len(args) == 5 {
			epoch = args[4]
		}
		if err := project.Scaffold(args[1], args[2], args[3], epoch); err != nil {
			return err
		}
		fmt.Printf("Subject Atlas Scaffoldを生成しました: %s\n", args[1])
		return nil
	case "generate":
		if len(args) != 3 || args[1] != "skill-reference" {
			return usageError()
		}
		if err := project.GenerateSkillReference(args[2]); err != nil {
			return err
		}
		fmt.Printf("Skill Coverage Referenceを再生成しました: %s\n", args[2])
		return nil
	case "migrate":
		if len(args) != 3 || args[1] != "v1" {
			return usageError()
		}
		created, err := project.MigrateV1(args[2], "")
		if err != nil {
			return err
		}
		for _, path := range created {
			fmt.Printf("移行Draftを生成しました: %s\n", path)
		}
		if len(created) == 0 {
			fmt.Println("移行済みです。既存Fileは変更していません。")
		}
		return nil
	case "version":
		if len(args) != 1 {
			return usageError()
		}
		fmt.Printf("atlas %s\n", version)
		return nil
	default:
		return fmt.Errorf("未知のCommandです: %s", args[0])
	}
}

func certificateOptions(args []string) (string, string, error) {
	issuedAt, commit := "", ""
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", usageError()
		}
		switch args[0] {
		case "--issued-at":
			issuedAt = args[1]
		case "--commit":
			commit = args[1]
		default:
			return "", "", fmt.Errorf("未知のOptionです: %s", args[0])
		}
		args = args[2:]
	}
	return issuedAt, commit, nil
}

func usageError() error {
	return fmt.Errorf("使い方: atlas validate <manifest...> | audit <atlas-directory> | scaffold <directory> <atlas-id> <日本語title> [epoch] | generate skill-reference <directory> | migrate v1 <directory> | certificate generate|verify <directory> | version")
}
