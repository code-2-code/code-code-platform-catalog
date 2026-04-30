package productinfo

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	productinfov1 "code-code.internal/go-contract/product_info/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"sigs.k8s.io/yaml"
)

//go:embed products/*.yaml
var staticProductInfoFS embed.FS

func staticProductInfoIDs() []string {
	entries, err := fs.Glob(staticProductInfoFS, "products/*.yaml")
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSuffix(filepath.Base(entry), filepath.Ext(entry))
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func materializeProductInfoYAML(id string) (*productinfov1.ProductInfo, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("platformk8s/productinfo: product info id is empty")
	}
	raw, err := staticProductInfoFS.ReadFile("products/" + id + ".yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("platformk8s/productinfo: product info %q not found", id)
		}
		return nil, err
	}
	asJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/productinfo: decode product info yaml %q: %w", id, err)
	}
	item := &productinfov1.ProductInfo{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(asJSON, item); err != nil {
		return nil, fmt.Errorf("platformk8s/productinfo: decode product info proto %q: %w", id, err)
	}
	if item.Id == "" {
		item.Id = id
	}
	return item, nil
}
