package validate

import "fmt"

func crossValidate(schemaName string, document any) error {
	root, ok := document.(map[string]any)
	if !ok {
		return fmt.Errorf("Document RootはObjectである必要があります")
	}

	switch schemaName {
	case "catalog.schema.json":
		return validateCatalog(root)
	case "coverage.schema.json":
		return validateCoverage(root)
	case "company-inventory.schema.json":
		return validateCompanyInventory(root)
	case "mastery.schema.json":
		return validateMastery(root)
	default:
		return nil
	}
}

func validateMastery(root map[string]any) error {
	requiredOutcomes := []string{"understand", "choose", "build", "verify", "operate", "troubleshoot", "evolve", "delegate"}
	requiredSurfaces := []string{
		"orientation-scope", "foundations-mechanics", "architecture-design", "implementation-construction",
		"testing-verification", "failure-recovery", "operations-observability", "security-privacy-safety",
		"performance-capacity-cost", "compatibility-integration", "migration-evolution-deprecation",
		"decision-comparison", "provenance-rights", "agent-skill",
	}
	if err := requireExactIDs(root["outcomes"], requiredOutcomes, "Mastery Outcome"); err != nil {
		return err
	}
	return requireExactIDs(root["surfaces"], requiredSurfaces, "Mastery Surface")
}

func requireExactIDs(raw any, required []string, label string) error {
	items, _ := raw.([]any)
	found := map[string]bool{}
	for _, rawItem := range items {
		item, _ := rawItem.(map[string]any)
		id, _ := item["id"].(string)
		if found[id] {
			return fmt.Errorf("%s IDが重複しています: %s", label, id)
		}
		found[id] = true
	}
	for _, id := range required {
		if !found[id] {
			return fmt.Errorf("必須%sがありません: %s", label, id)
		}
	}
	return nil
}

func validateCompanyInventory(root map[string]any) error {
	technologies, _ := root["technologies"].([]any)
	ids := map[string]bool{}
	for _, rawTechnology := range technologies {
		technology, _ := rawTechnology.(map[string]any)
		id, _ := technology["id"].(string)
		if ids[id] {
			return fmt.Errorf("Technology IDが重複しています: %s", id)
		}
		ids[id] = true
	}
	for _, rawIntegration := range root["integrations"].([]any) {
		integration, _ := rawIntegration.(map[string]any)
		from, _ := integration["from"].(string)
		to, _ := integration["to"].(string)
		if !ids[from] || !ids[to] {
			return fmt.Errorf("Integrationが未定義Technologyを参照しています: %s -> %s", from, to)
		}
	}
	return nil
}

func validateCatalog(root map[string]any) error {
	domains, _ := root["domains"].([]any)
	ids := map[string]bool{}
	repositories := map[string]bool{}
	for _, rawDomain := range domains {
		domain, _ := rawDomain.(map[string]any)
		subjects, _ := domain["subjects"].([]any)
		for _, rawSubject := range subjects {
			subject, _ := rawSubject.(map[string]any)
			id, _ := subject["id"].(string)
			repository, _ := subject["repository"].(string)
			if ids[id] {
				return fmt.Errorf("Subject IDが重複しています: %s", id)
			}
			if repositories[repository] {
				return fmt.Errorf("Repository名が重複しています: %s", repository)
			}
			ids[id] = true
			repositories[repository] = true
		}
	}
	return nil
}

func validateCoverage(root map[string]any) error {
	targets, _ := root["targets"].([]any)
	ids := map[string]bool{}
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		id, _ := target["id"].(string)
		if ids[id] {
			return fmt.Errorf("Coverage Target IDが重複しています: %s", id)
		}
		ids[id] = true
		state, _ := target["state"].(string)
		claims, _ := target["claim_ids"].([]any)
		evidence, _ := target["evidence_ids"].([]any)
		if state == "covered" && (len(claims) == 0 || len(evidence) == 0) {
			return fmt.Errorf("covered Target %sにはClaimとEvidenceが必要です", id)
		}
	}
	return nil
}
