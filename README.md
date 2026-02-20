# helm-values-checker

A Helm plugin that validates your values files against a chart's defaults and optional JSON schema. Catches typos, type mismatches, missing required fields, and deprecated keys before you deploy.

## Why?

Helm's built-in validation only works if the chart ships a `values.schema.json` (most don't), and even then it doesn't detect unknown or misspelled keys. This tool fills that gap.

## Installation

### As a Helm plugin

```bash
helm plugin install --verify=false https://github.com/chrishham/helm-values-checker
```

### From source

```bash
git clone https://github.com/chrishham/helm-values-checker.git
cd helm-values-checker
make build
```

## Usage

```bash
# Validate against a remote chart
helm values-checker validate -f my-values.yaml --chart bitnami/postgresql

# Pin a specific chart version
helm values-checker validate -f my-values.yaml --chart bitnami/postgresql --version 15.2.0

# Validate against a local chart directory
helm values-checker validate -f my-values.yaml --chart ./my-chart/

# JSON output (for CI pipelines)
helm values-checker validate -f my-values.yaml --chart bitnami/postgresql --output json

# Strict mode: treat warnings as errors
helm values-checker validate -f my-values.yaml --chart bitnami/postgresql --strict

# Ignore specific key paths (glob patterns)
helm values-checker validate -f my-values.yaml --chart bitnami/postgresql --ignore-keys "global.**"
```

You must have run `helm repo add` / `helm repo update` beforehand for remote charts.

## Validation Checks

| Check | Severity | Description |
|-------|----------|-------------|
| Unknown keys | Error | Keys in your values that don't exist in chart defaults or schema. Includes "did you mean?" suggestions. |
| Type mismatches | Error | Wrong type (e.g., string where int expected). Null defaults accept any type. Int/float are compatible. |
| Required fields | Error | Missing fields marked as required in `values.schema.json`. |
| Deprecated keys | Warning | Keys marked `deprecated: true` in `values.schema.json`. |

## Example Output

```
Validating my-values.yaml against postgresql (15.2.0)

ERRORS (3)
  line 12: Unknown key "image.regsitry" (did you mean "image.registry"?)
  line 25: Type mismatch at "replicaCount": expected int, got string ("three")
  line 40: Schema validation: auth.postgresPassword is required

WARNINGS (1)
  line 8: Deprecated key "persistence.enabled" - use "primary.persistence.enabled" instead

Summary: 3 errors, 1 warning
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No errors (warnings may exist) |
| 1 | Validation errors found |
| 2 | Warnings found (only with `--strict`) |
| 3 | Tool error (bad flags, chart not found) |

## Releasing

To create a new release, use the included release script:

```bash
./scripts/release.sh 0.4.0
```

This bumps the version in `plugin.yaml`, commits, tags, and pushes. The GitHub Actions release workflow then builds and publishes the binaries automatically.

## Edge Cases

- **Subcharts**: Keys matching a dependency name are validated against that subchart's defaults
- **Arrays of objects**: First element in default list used as structural template
- **Null defaults**: Accepted as "any type allowed"
- **Schema-only keys**: Keys defined in schema but absent from `values.yaml` defaults are considered valid
- **YAML anchors/aliases**: Resolved automatically

## License

Apache License 2.0 — see [LICENSE](LICENSE).
