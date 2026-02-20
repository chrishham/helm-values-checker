package chart

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
)

// ResolvedChart holds a loaded chart with its parsed default values tree.
type ResolvedChart struct {
	Chart            *chart.Chart
	DefaultsNode     *yaml.Node            // yaml.Node tree of values.yaml
	SchemaBytes      []byte                // raw values.schema.json, nil if absent
	SubchartDefaults map[string]*yaml.Node // dependency name -> defaults node
	tempDir          string                // set if we pulled a remote chart
}

var (
	urlCredsRE = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s@]+)@`)
	secretQSRE = regexp.MustCompile(`(?i)(\b(access_token|token|password|passwd|pwd|secret|apikey|api_key)\b=)([^&\s]+)`)
)

// Cleanup removes any temporary files created during chart resolution.
func (r *ResolvedChart) Cleanup() {
	if r.tempDir != "" {
		os.RemoveAll(r.tempDir)
	}
}

// Resolve loads a chart from a local path or pulls it from a remote repository.
func Resolve(chartRef, version string) (*ResolvedChart, error) {
	if isLocalPath(chartRef) {
		return resolveLocal(chartRef)
	}
	return resolveRemote(chartRef, version)
}

func isLocalPath(ref string) bool {
	// Treat as local if it starts with ., /, or ~ or exists on disk
	if strings.HasPrefix(ref, ".") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") {
		return true
	}
	info, err := os.Stat(ref)
	return err == nil && info.IsDir()
}

func resolveLocal(path string) (*ResolvedChart, error) {
	// Expand ~ if needed
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("expanding home dir: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	ch, err := loader.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading chart from %s: %w", path, err)
	}

	return buildResolved(ch, "")
}

func resolveRemote(chartRef, version string) (*ResolvedChart, error) {
	settings := cli.New()

	tmpDir, err := os.MkdirTemp("", "helm-values-checker-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	regClient, err := registry.NewClient(registry.ClientOptDebug(debugEnabled()), registry.ClientOptWriter(io.Discard))
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("initializing helm registry client: %w", err)
	}

	var out strings.Builder
	opts := []getter.Option{}
	if registry.IsOCI(chartRef) {
		opts = append(opts, getter.WithRegistryClient(regClient))
	}

	dl := downloader.ChartDownloader{
		Out:              &out,
		Verify:           downloader.VerifyNever,
		Getters:          getter.All(settings),
		Options:          opts,
		RegistryClient:   regClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	saved, _, err := dl.DownloadTo(chartRef, version, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		// Downloader output can contain URLs and other user-specific details (including credentials in rare cases).
		// Only include a redacted version when explicitly debugging.
		if debugEnabled() && strings.TrimSpace(out.String()) != "" {
			return nil, fmt.Errorf("pulling chart %s: %w\n%s", chartRef, err, redactSensitive(out.String()))
		}
		return nil, fmt.Errorf("pulling chart %s: %w", chartRef, err)
	}

	ch, err := loader.Load(saved)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("loading pulled chart: %w", err)
	}

	return buildResolved(ch, tmpDir)
}

func debugEnabled() bool {
	v := strings.TrimSpace(os.Getenv("HELM_VALUES_CHECKER_DEBUG"))
	return v != "" && v != "0" && strings.ToLower(v) != "false"
}

func redactSensitive(s string) string {
	const maxLen = 2000
	redacted := urlCredsRE.ReplaceAllString(s, "${1}REDACTED@")
	redacted = secretQSRE.ReplaceAllString(redacted, "${1}REDACTED")
	if len(redacted) > maxLen {
		return redacted[:maxLen] + "\n... (truncated)"
	}
	return redacted
}

func buildResolved(ch *chart.Chart, tempDir string) (*ResolvedChart, error) {
	resolved := &ResolvedChart{
		Chart:            ch,
		SubchartDefaults: make(map[string]*yaml.Node),
		tempDir:          tempDir,
	}

	// Parse main values.yaml into yaml.Node tree
	for _, f := range ch.Raw {
		if f.Name == "values.yaml" || f.Name == "values.yml" {
			node := &yaml.Node{}
			if err := yaml.Unmarshal(f.Data, node); err != nil {
				return nil, fmt.Errorf("parsing values.yaml: %w", err)
			}
			// yaml.Unmarshal wraps in a Document node
			if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
				resolved.DefaultsNode = node.Content[0]
			} else {
				resolved.DefaultsNode = node
			}
			break
		}
	}

	if resolved.DefaultsNode == nil {
		// Create empty mapping if no values.yaml
		resolved.DefaultsNode = &yaml.Node{Kind: yaml.MappingNode}
	}

	// Load schema if present
	if ch.Schema != nil {
		resolved.SchemaBytes = ch.Schema
	}

	// Parse subchart defaults
	for _, dep := range ch.Dependencies() {
		for _, f := range dep.Raw {
			if f.Name == "values.yaml" || f.Name == "values.yml" {
				node := &yaml.Node{}
				if err := yaml.Unmarshal(f.Data, node); err != nil {
					continue
				}
				if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
					resolved.SubchartDefaults[dep.Name()] = node.Content[0]
				} else {
					resolved.SubchartDefaults[dep.Name()] = node
				}
				break
			}
		}
	}

	return resolved, nil
}
