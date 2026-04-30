package productinfo

import (
	"context"
	"slices"
	"strings"

	productinfov1 "code-code.internal/go-contract/product_info/v1"
)

// Service provides read-only access to product info entries.
type Service struct {
	byID map[string]*productinfov1.ProductInfo
}

// NewService creates one product info service from embedded YAML.
func NewService() (*Service, error) {
	ids := staticProductInfoIDs()
	slices.Sort(ids)
	byID := make(map[string]*productinfov1.ProductInfo, len(ids))
	for _, id := range ids {
		item, err := materializeProductInfoYAML(id)
		if err != nil {
			return nil, err
		}
		byID[strings.TrimSpace(item.GetId())] = item
	}
	return &Service{byID: byID}, nil
}

// Get returns one ProductInfo by id, or nil if not found.
func (s *Service) Get(_ context.Context, id string) *productinfov1.ProductInfo {
	return s.byID[strings.TrimSpace(id)]
}

// List returns all product info entries sorted by display name.
func (s *Service) List(_ context.Context) []*productinfov1.ProductInfo {
	items := make([]*productinfov1.ProductInfo, 0, len(s.byID))
	for _, item := range s.byID {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b *productinfov1.ProductInfo) int {
		return strings.Compare(a.GetDisplayName(), b.GetDisplayName())
	})
	return items
}
