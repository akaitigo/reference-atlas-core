package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akaitigo/reference-atlas-core/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

type Result struct {
	Schema string
}

func File(path string) (Result, error) {
	schemaName, err := schemaFor(path)
	if err != nil {
		return Result{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("%sを読み込めません: %w", path, err)
	}

	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Result{}, fmt.Errorf("%sは有効なYAML/JSONではありません: %w", path, err)
	}
	jsonData, err := json.Marshal(document)
	if err != nil {
		return Result{}, fmt.Errorf("%sをJSONへ正規化できません: %w", path, err)
	}
	if err := json.Unmarshal(jsonData, &document); err != nil {
		return Result{}, fmt.Errorf("%sを検証用Documentへ変換できません: %w", path, err)
	}

	schemaData, err := schemas.FS.ReadFile(schemaName)
	if err != nil {
		return Result{}, fmt.Errorf("組み込みSchema %sを読み込めません: %w", schemaName, err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaData, &schemaDocument); err != nil {
		return Result{}, fmt.Errorf("組み込みSchema %sは有効なJSONではありません: %w", schemaName, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaName, schemaDocument); err != nil {
		return Result{}, fmt.Errorf("Schema %sを登録できません: %w", schemaName, err)
	}
	compiled, err := compiler.Compile(schemaName)
	if err != nil {
		return Result{}, fmt.Errorf("Schema %sをコンパイルできません: %w", schemaName, err)
	}
	if err := compiled.Validate(document); err != nil {
		return Result{}, fmt.Errorf("%sは%sに適合しません: %w", path, schemaName, err)
	}

	if err := crossValidate(schemaName, document); err != nil {
		return Result{}, fmt.Errorf("%sの横断制約に違反しています: %w", path, err)
	}
	return Result{Schema: schemaName}, nil
}

func schemaFor(path string) (string, error) {
	base := filepath.Base(path)
	switch {
	case base == "atlas.yaml" || base == "atlas.yml" || base == "atlas.json":
		return "atlas.schema.json", nil
	case base == "coverage.yaml" || base == "coverage.yml" || base == "coverage.json":
		return "coverage.schema.json", nil
	case base == "sources.lock.yaml" || base == "sources.lock.yml" || base == "sources.lock.json":
		return "sources-lock.schema.json", nil
	case base == "mastery.yaml" || base == "mastery.yml" || base == "mastery.json":
		return "mastery.schema.json", nil
	case base == "definitive.yaml" || base == "definitive.yml" || base == "definitive.json":
		return "definitive.schema.json", nil
	case base == "skill.package.yaml" || base == "skill.package.yml" || base == "skill.package.json":
		return "skill-package.schema.json", nil
	case base == "stage1.yaml" || base == "stage1.yml" || base == "catalog.json":
		return "catalog.schema.json", nil
	case base == "company-inventory.yaml" || base == "company-inventory.yml" || base == "company-inventory.json":
		return "company-inventory.schema.json", nil
	case strings.HasSuffix(base, ".evidence.yaml") || strings.HasSuffix(base, ".evidence.yml") || strings.HasSuffix(base, ".evidence.json"):
		return "evidence.schema.json", nil
	case strings.HasSuffix(base, ".claim.yaml") || strings.HasSuffix(base, ".claim.yml") || strings.HasSuffix(base, ".claim.json"):
		return "claim.schema.json", nil
	case strings.HasSuffix(base, ".skill-eval.yaml") || strings.HasSuffix(base, ".skill-eval.yml") || strings.HasSuffix(base, ".skill-eval.json"):
		return "skill-eval.schema.json", nil
	case base == "provenance.yaml" || base == "provenance.yml" || base == "provenance.json":
		return "provenance.schema.json", nil
	case base == "core-v1.yaml" || base == "core-v1.yml" || base == "core-v1.json":
		return "migration.schema.json", nil
	case base == "completion-certificate.json":
		return "completion-certificate.schema.json", nil
	case base == "manifest.yaml" && filepath.Base(filepath.Dir(path)) == "third_party":
		return "third-party.schema.json", nil
	case base == "surface.inventory.yaml":
		return "surface-inventory.schema.json", nil
	case base == "verification.matrix.yaml":
		return "verification-matrix.schema.json", nil
	case base == "depth.parity.yaml":
		return "depth-parity.schema.json", nil
	case base == "FE_DEPTH_REFERENCE.yaml" || base == "FE_DEPTH_REFERENCE.json":
		return "depth-reference.schema.json", nil
	case strings.HasSuffix(base, ".authority-surfaces.yaml"):
		return "authority-surfaces.schema.json", nil
	case strings.HasSuffix(base, ".definitive-skill-eval.json"):
		return "definitive-skill-eval.schema.json", nil
	case base == "definitive-certificate.json":
		return "definitive-certificate.schema.json", nil
	case base == "definitive-v2.yaml":
		return "definitive-migration.schema.json", nil
	case base == "non-regression.yaml":
		return "non-regression.schema.json", nil
	case strings.HasSuffix(base, ".non-regression-baseline.json"):
		return "non-regression-baseline.schema.json", nil
	default:
		return "", fmt.Errorf("%sに対応するSchemaを判定できません", path)
	}
}
