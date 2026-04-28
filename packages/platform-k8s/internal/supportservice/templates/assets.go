package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed *.yaml
var templateAssetsFS embed.FS

// TemplateAsset describes one embedded quick template manifest.
type TemplateAsset struct {
	ID       string
	Manifest []byte
}

// TemplateAssets returns the embedded quick template manifests shipped with
// platform-k8s.
func TemplateAssets() ([]TemplateAsset, error) {
	entries, err := fs.Glob(templateAssetsFS, "*.yaml")
	if err != nil {
		return nil, fmt.Errorf("platformk8s: list template assets: %w", err)
	}
	assets := make([]TemplateAsset, 0, len(entries))
	for _, entry := range entries {
		raw, err := templateAssetsFS.ReadFile(entry)
		if err != nil {
			return nil, fmt.Errorf("platformk8s: read template asset %q: %w", entry, err)
		}
		assets = append(assets, TemplateAsset{
			ID:       strings.TrimSuffix(path.Base(entry), path.Ext(entry)),
			Manifest: raw,
		})
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].ID < assets[j].ID
	})
	return assets, nil
}
