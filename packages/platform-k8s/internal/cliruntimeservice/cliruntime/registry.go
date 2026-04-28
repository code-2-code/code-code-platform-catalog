package cliruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type RegistryTagLister interface {
	ListTags(context.Context, string) ([]string, error)
}

type RemoteRegistryTagLister struct {
	Insecure bool
}

func (l RemoteRegistryTagLister) ListTags(ctx context.Context, repository string) ([]string, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("platformk8s/cliruntime: image repository is empty")
	}
	options := []name.Option{}
	if l.Insecure {
		options = append(options, name.Insecure)
	}
	ref, err := name.NewRepository(repository, options...)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: parse image repository %q: %w", repository, err)
	}
	tags, err := remote.List(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		var registryErr *transport.Error
		if errors.As(err, &registryErr) && registryErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("platformk8s/cliruntime: list registry tags for %q: %w", repository, err)
	}
	return tags, nil
}
