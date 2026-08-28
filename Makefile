.PHONY: test validate audit check

test:
	go test ./...

validate:
	go run ./cmd/atlas validate catalog/stage1.yaml
	go run ./cmd/atlas validate examples/company-inventory.yaml
	go run ./cmd/atlas validate examples/frontend-behavior-atlas/atlas.yaml examples/frontend-behavior-atlas/mastery.yaml examples/frontend-behavior-atlas/coverage.yaml examples/frontend-behavior-atlas/sources.lock.yaml examples/frontend-behavior-atlas/skill.package.yaml examples/frontend-behavior-atlas/sample.evidence.yaml

audit:
	go run ./cmd/atlas audit examples/frontend-behavior-atlas

check: test validate audit
