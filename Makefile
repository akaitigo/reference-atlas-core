.PHONY: fmt-check vet test compatibility validate audit check build release-check

GOCACHE ?= $(CURDIR)/.cache/go-build
export GOCACHE

fmt-check:
	test -z "$$(gofmt -l cmd internal schemas)"

vet:
	go vet ./...

test:
	go test ./...

compatibility:
	go test ./internal/validate -run 'TestV1CompatibilityFixtures|TestExamplesValidate'

validate:
	go run ./cmd/atlas validate catalog/stage1.yaml
	go run ./cmd/atlas validate examples/company-inventory.yaml
	go run ./cmd/atlas validate examples/frontend-behavior-atlas/atlas.yaml examples/frontend-behavior-atlas/mastery.yaml examples/frontend-behavior-atlas/coverage.yaml examples/frontend-behavior-atlas/sources.lock.yaml examples/frontend-behavior-atlas/skill.package.yaml examples/frontend-behavior-atlas/sample.evidence.yaml
	go run ./cmd/atlas validate atlas.yaml mastery.yaml coverage.yaml sources.lock.yaml skill.package.yaml provenance.yaml non-regression.yaml baselines/main-b32f6af.non-regression-baseline.json baselines/main-b32f6af.definitive-v2.non-regression-baseline.json baselines/main-b32f6af.scenario-trace-v1.non-regression-baseline.json baselines/main-b32f6af.scenario-closure-plan-v1.non-regression-baseline.json baselines/main-b32f6af.evidence-durability-v1.non-regression-baseline.json profiles/FE_DEPTH_REFERENCE.json profiles/FE_AUTHORITY_EXTRACTION_REFERENCE.json profiles/FE_AUTHORITY_BODY_DENOMINATOR_REFERENCE.json profiles/FE_AUTHORITY_REVIEW_QUEUE_REFERENCE.json profiles/FE_AUTHORITY_REVIEW_DECISIONS_REFERENCE.json migrations/core-v1.yaml migrations/definitive-v2.yaml third_party/manifest.yaml
	go run ./cmd/atlas validate claims/*.claim.yaml evidence/*.evidence.yaml evals/*.skill-eval.json evidence/completion-certificate.json

audit:
	go run ./cmd/atlas audit examples/frontend-behavior-atlas
	go run ./cmd/atlas audit .
	go run ./cmd/atlas audit . --gate non-regression

build:
	mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o bin/atlas ./cmd/atlas

check: fmt-check vet test compatibility validate audit

release-check: check build
	./bin/atlas version
	./bin/atlas certificate verify .
