package validator

import (
	"fmt"
	"os"

	"github.com/chrishham/helm-values-checker/internal/chart"
	"github.com/chrishham/helm-values-checker/internal/model"
	"gopkg.in/yaml.v3"
)

// maxValuesFileSize is the maximum allowed size for a values file (10 MB).
const maxValuesFileSize = 10 * 1024 * 1024

// Validate runs all validation checks on a values file against the resolved chart.
func Validate(valuesFile string, resolved *chart.ResolvedChart, ignoreKeys []string) (*model.ValidationResult, error) {
	fi, err := os.Stat(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("reading values file %s: %w", valuesFile, err)
	}
	if fi.Size() > maxValuesFileSize {
		return nil, fmt.Errorf("values file %s is too large (%d bytes, max %d)", valuesFile, fi.Size(), maxValuesFileSize)
	}

	data, err := os.ReadFile(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("reading values file %s: %w", valuesFile, err)
	}

	userDoc := &yaml.Node{}
	if err := yaml.Unmarshal(data, userDoc); err != nil {
		return nil, fmt.Errorf("parsing values file %s: %w", valuesFile, err)
	}

	var userNode *yaml.Node
	if userDoc.Kind == yaml.DocumentNode && len(userDoc.Content) > 0 {
		userNode = userDoc.Content[0]
	} else {
		userNode = userDoc
	}

	if userNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("values file %s: expected a YAML mapping at top level", valuesFile)
	}

	result := &model.ValidationResult{
		ValuesFile:   valuesFile,
		ChartName:    resolved.Chart.Metadata.Name,
		ChartVersion: resolved.Chart.Metadata.Version,
	}

	// Extract schema-defined keys if schema is available
	schemaKeys := extractSchemaKeys(resolved.SchemaBytes)

	// Extract schema type definitions for type checking fallback
	schemaTypes := extractSchemaTypes(resolved.SchemaBytes)

	// Pre-compute all paths from defaults tree for deep suggestions
	allPaths := collectAllPaths(resolved.DefaultsNode, "")

	// 1. Unknown key detection
	result.Findings = append(result.Findings,
		detectUnknownKeys(userNode, resolved.DefaultsNode, schemaKeys, resolved.SubchartDefaults, ignoreKeys, "", allPaths)...)

	// 2. Type mismatch detection (uses schema types as fallback for null/absent defaults)
	result.Findings = append(result.Findings,
		detectTypeMismatches(userNode, resolved.DefaultsNode, ignoreKeys, "", schemaTypes)...)

	// 3. Schema validation (required fields + deprecated keys; type errors filtered when custom checker handles them)
	schemaFindings, err := validateSchema(userNode, resolved.SchemaBytes, ignoreKeys, schemaTypes)
	if err != nil {
		return nil, fmt.Errorf("schema validation for %s: %w", valuesFile, err)
	}
	result.Findings = append(result.Findings, schemaFindings...)

	return result, nil
}
