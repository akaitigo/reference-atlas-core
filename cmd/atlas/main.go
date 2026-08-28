package main

import (
	"fmt"
	"os"
	"time"

	"github.com/akaitigo/reference-atlas-core/internal/project"
	"github.com/akaitigo/reference-atlas-core/internal/validate"
)

const version = "1.1.0"

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
		if len(args) == 4 && args[2] == "--gate" && args[3] == "evidence-durability" {
			result, err := validate.AuditEvidenceDurability(args[1], "artifacts/pattern-scenarios/results.json")
			if err != nil {
				return err
			}
			fmt.Printf("Evidence durability監査済み: %s artifacts=%d rows=%d variants=%d publish_on=full-run-passed failed_run=retain-prior-success swap=staged-directory-rename-with-rollback\n", result.ReportID, result.Artifacts, result.Rows, result.Variants)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "scenario-plan" {
			result, err := validate.AuditScenarioClosurePlan(args[1], "evidence/scenarios/closure-plan.json")
			if err != nil {
				return err
			}
			fmt.Printf("Scenario Closure Plan監査済み: %s remaining_rows=%d completed_rows=%d planned_tranches=%d maximum_rows_per_tranche=%d next_tranche=%s\n", result.AtlasID, result.RemainingRows, result.CompletedRows, result.PlannedTranches, result.MaximumRowsPerTranche, result.NextTranche)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "scenario-trace" {
			result, err := validate.AuditScenarioTrace(args[1], "evidence/scenarios/index.json", false)
			if err != nil {
				return err
			}
			fmt.Printf("Integrated Scenario/Trace監査済み: %s patterns=%d rows=%d dedicated_artifacts=%d pattern_specific=%d runtime_identity=%d gaps=%d integrated_trace_rows=%d dedicated_scenario_runtime_rows=%d scenario_closure_gaps=%d authority_atomic=%d completion_eligible=%d integrated_scenarios=%d completion_limited=%t\n", result.AtlasID, result.Patterns, result.Rows, result.DedicatedArtifactRows, result.PatternSpecificRows, result.RuntimeIdentityRows, result.PatternSpecificGaps, result.IntegratedTraceRows, result.DedicatedScenarioRows, result.ScenarioClosureGaps, result.AuthorityAtomicRows, result.CompletionEligibleRows, result.IntegratedScenarioTests, result.CompletionLimited)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "skill-router" {
			result, err := validate.AuditDefinitiveSkillRouter(args[1], "evals/definitive-skill-router.json", false)
			if err != nil {
				return err
			}
			fmt.Printf("Definitive Skill Router監査済み: %s cells=%d routed=%d routing_gaps=%d partial=%d boundaries=%d forward_eval=%t completion_limited=%t\n", result.AtlasID, result.Cells, result.Routed, result.RoutingGaps, result.PartialCoverageCells, result.BoundaryCases, result.ForwardEvalCompleted, result.CompletionLimited)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "authority-relock" {
			result, err := validate.AuditAuthorityRelock(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Authority stale relock監査済み: %s candidates=%d unchanged=%d authorized_updates=%d automatic_updates=false\n", result.AtlasID, result.Candidates, result.Unchanged, result.AuthorizedUpdates)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "authority-review-export" {
			result, err := validate.AuditAuthorityReviewExport(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Authority Review read-only export監査済み: %s packets=%d anchors=%d candidate_projections=%d machine_proposals=%d human_decisions=%d stale_holds=%d write_decisions=false promote_human_review=false depth_credit=false\n", result.AtlasID, result.Packets, result.UniqueAnchors, result.DomainProjections, result.MachineProposals, result.HumanDecisions, result.StaleHolds)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "authority-review" {
			result, err := validate.AuditAuthorityReviewQueue(args[1], false)
			if err != nil {
				return err
			}
			fmt.Printf("Authority Human Review監査済み: %s status=%s queued=%d pending_human=%d human_reviewed=%d deferred=%d stale_holds=%d unavailable_holds=%d decisions=%d semantic_exhaustive=%t depth_credit=false\n",
				result.AtlasID, result.Status, result.QueuedAnchors, result.PendingHuman, result.HumanReviewed, result.Deferred, result.StaleHolds, result.UnavailableHolds, result.Decisions, result.AuthoritySemanticsExhaustive)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "authority-body" {
			result, err := validate.AuditAuthorityBodyInventory(args[1], false)
			if err != nil {
				return err
			}
			fmt.Printf("Authority Body Denominator監査済み: %s status=%s sources=%d documents=%d matched=%d stale=%d failed=%d candidate_anchors=%d classified=%d unclassified=%d human_reviewed=%d deferred=%d eligible_artifacts=%d semantic_exhaustive=%t baseline=%t\n",
				result.AtlasID, result.Status, result.SourceEntries, result.UniqueDocuments, result.MatchedDocuments, result.StaleDocuments, result.FailedDocuments, result.Anchors, result.ClassifiedAnchors, result.UnclassifiedAnchors, result.HumanReviewedAnchors, result.DeferredAnchors, result.CoreV2EligibleArtifacts, result.AuthoritySemanticsExhaustive, result.BaselinePresent)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "authority-extraction" {
			result, err := validate.AuditAuthorityExtraction(args[1], false)
			if err != nil {
				return err
			}
			fmt.Printf("Authority Extraction監査済み: %s status=%s locked=%d matched=%d stale=%d failed=%d candidate_edges=%d classified_edges=%d unclassified_edges=%d missing_locators=%d deferred_locators=%d text_exhaustive=%t human_reviewed=%d core_v2_eligible=%d\n",
				result.AtlasID, result.Status, result.LockedSources, result.MatchedSources, result.StaleSources, result.FailedSources, result.CandidateEdges, result.ClassifiedReferenceEdges, result.UnclassifiedReferenceEdges, result.MissingLocators, result.DeferredLocators, result.AuthorityTextSurfacesExhaustive, result.HumanReviewedSurfaces, result.CoreV2EligibleSurfaces)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "non-regression" {
			result, err := validate.AuditNonRegression(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("非退行監査済み: %s baseline_items=%d current_items=%d replacements=%d\n", result.AtlasID, result.BaselineItems, result.CurrentItems, result.Replacements)
			return nil
		}
		if len(args) == 4 && args[2] == "--gate" && args[3] == "definitive" {
			result, err := validate.AuditDefinitive(args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Subject Definitive監査済み: %s completion_class=%s authority_surfaces=%d behaviors=%d proofs=%d required_matrix_rows=%d runtime_evidence=%d scenario_proof_rows=%d depth_parity_rows=%d reference_systems=%d comparisons=%d\n",
				result.AtlasID, result.CompletionClass, result.AuthoritySurfaces, result.IncludedBehaviors, result.ProofObligations, result.RequiredMatrixRows, result.RuntimeEvidence, result.ScenarioProofRows, result.DepthParityRows, result.ReferenceSystems, result.Comparisons)
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
	case "baseline":
		if len(args) < 4 || args[1] != "generate" {
			return usageError()
		}
		capturedAt, commit, err := baselineOptions(args[4:])
		if err != nil {
			return err
		}
		if commit == "" {
			return fmt.Errorf("baseline generateには--commitが必要です")
		}
		path, err := validate.GenerateNonRegressionBaseline(args[2], args[3], commit, capturedAt)
		if err != nil {
			return err
		}
		fmt.Printf("Non-regression baselineを生成しました: %s\n", path)
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

func baselineOptions(args []string) (string, string, error) {
	capturedAt, commit := "", ""
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", usageError()
		}
		switch args[0] {
		case "--captured-at":
			capturedAt = args[1]
		case "--commit":
			commit = args[1]
		default:
			return "", "", fmt.Errorf("未知のOptionです: %s", args[0])
		}
		args = args[2:]
	}
	return capturedAt, commit, nil
}

func usageError() error {
	return fmt.Errorf("使い方: atlas validate <manifest...> | audit <atlas-directory> [--gate definitive|non-regression|authority-extraction|authority-body|authority-review|authority-review-export|authority-relock|skill-router|scenario-trace|scenario-plan|evidence-durability] | baseline generate <directory> <output> --commit SHA [--captured-at RFC3339] | scaffold <directory> <atlas-id> <日本語title> [epoch] | generate skill-reference <directory> | migrate v1|definitive-v2 <directory> | certificate generate|verify|generate-definitive|verify-definitive <directory> | version")
}
