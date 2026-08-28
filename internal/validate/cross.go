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
	default:
		return nil
	}
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
