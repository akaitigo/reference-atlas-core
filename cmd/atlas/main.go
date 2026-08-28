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
		if len(args) == 4 && args[2] == "--gate" && args[3] == "definitive" {
			result, err := validate.AuditDefinitive(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("決定版監査済み: %s completion_class=%s authority_surfaces=%d behaviors=%d proofs=%d required_matrix_rows=%d runtime_evidence=%d reference_systems=%d comparisons=%d\n",
				result.AtlasID, result.CompletionClass, result.AuthoritySurfaces, result.IncludedBehaviors, result.ProofObligations, result.RequiredMatrixRows, result.RuntimeEvidence, result.ReferenceSystems, result.Comparisons)
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("使い方: atlas audit <atlas-directory>")
		}
		result, err := validate.AuditDir(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("監査済み: %s completion_class=%s manifest_status=%s mastery=%d target_sets=%d targets=%d claims=%d evidence=%d open_required=%d\n",
			result.AtlasID, result.CompletionClass, result.Status, result.MasteryAreas, result.TargetSets, result.Targets, result.Claims, result.Evidence, result.OpenRequired)
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
			fmt.Printf("Bounded Completion Certificateを生成しました: %s completion_class=bounded-complete\n", path)
			return nil
		case "verify":
			if len(args) != 3 {
				return usageError()
			}
			if err := validate.VerifyCertificate(args[2]); err != nil {
				return err
			}
			fmt.Printf("Bounded Completion Certificateを検証しました: %s completion_class=bounded-complete\n", args[2])
			return nil
		case "generate-definitive":
			issuedAt, commit, err := certificateOptions(args[3:])
			if err != nil {
				return err
			}
			path, err := validate.GenerateDefinitiveCertificate(args[2], issuedAt, commit)
			if err != nil {
				return err
			}
			fmt.Printf("Subject Definitive Certificateを生成しました: %s completion_class=subject-definitive\n", path)
			return nil
		case "verify-definitive":
			if len(args) != 3 {
				return usageError()
			}
			if err := validate.VerifyDefinitiveCertificate(args[2]); err != nil {
				return err
			}
			fmt.Printf("Subject Definitive Certificateを検証しました: %s completion_class=subject-definitive\n", args[2])
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
		if len(args) != 3 {
			return usageError()
		}
		switch args[1] {
		case "v1":
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
		case "definitive-v2":
			path, err := project.MigrateDefinitiveV2(args[2], "")
			if err != nil {
				return err
			}
			if path == "" {
				fmt.Println("Definitive v2 Migration記録は既に存在します。")
			} else {
				fmt.Printf("Bounded Certificateを保持したDefinitive v2 Migration記録を生成しました: %s\n", path)
			}
			return nil
		default:
			return usageError()
		}
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
	return fmt.Errorf("使い方: atlas validate <manifest...> | audit <atlas-directory> [--gate definitive] | scaffold <directory> <atlas-id> <日本語title> [epoch] | generate skill-reference <directory> | migrate v1|definitive-v2 <directory> | certificate generate|verify|generate-definitive|verify-definitive <directory> | version")
}
