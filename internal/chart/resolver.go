package chart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"gopkg.in/yaml.v3"
)

// ResolvedChart holds a loaded chart with its parsed default values tree.
type ResolvedChart struct {
	Chart         *chart.Chart
	DefaultsNode  *yaml.Node       // yaml.Node tree of values.yaml
	SchemaBytes   []byte           // raw values.schema.json, nil if absent
	SubchartDefaults map[string]*yaml.Node // dependency name -> defaults node
	tempDir       string           // set if we pulled a remote chart
}

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

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), "", "", func(format string, v ...interface{}) {}); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("initializing helm config: %w", err)
	}

	pull := action.NewPullWithOpts(action.WithConfig(cfg))
	pull.Settings = settings
	pull.DestDir = tmpDir
	pull.Untar = true
	pull.UntarDir = tmpDir
	if version != "" {
		pull.Version = version
	}

	output, err := pull.Run(chartRef)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("pulling chart %s: %w\n%s", chartRef, err, output)
	}

	// Find the extracted chart directory
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("reading temp dir: %w", err)
	}

	var chartDir string
	for _, e := range entries {
		if e.IsDir() {
			chartDir = filepath.Join(tmpDir, e.Name())
			break
		}
	}
	if chartDir == "" {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("no chart directory found after pulling %s", chartRef)
	}

	ch, err := loader.Load(chartDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("loading pulled chart: %w", err)
	}

	return buildResolved(ch, tmpDir)
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
