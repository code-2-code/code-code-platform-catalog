package modelcatalogsources

import (
	"context"
	"net/http"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	modelcatalogdiscoveryv1 "code-code.internal/go-contract/model_catalog_discovery/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelcatalogdiscovery"
)

type ProbeRequest struct {
	ProbeID                  string
	Protocol                 apiprotocolv1.Protocol
	BaseURL                  string
	Headers                  http.Header
	ProviderSurfaceBindingID string
	Operation                *modelcatalogdiscoveryv1.ModelCatalogDiscoveryOperation
	DynamicValues            modelcatalogdiscovery.DynamicValues
	ConcurrencyKey           string
}

type ModelIDProbe interface {
	ProbeModelIDs(context.Context, ProbeRequest) ([]string, error)
}
