# code-code-platform-catalog

Catalog services split from the Code Code platform.

This repository owns:

- `packages/platform-k8s/internal/modelservice`: model registry, model cards, and model definition sync.
- `packages/platform-k8s/internal/supportservice`: vendor, provider surface, CLI definition, product-info, and support reference data.
- `packages/platform-k8s/internal/cliruntimeservice`: CLI runtime image records and version sync.
- Runtime entrypoints for model, support, and CLI runtime services.

Contracts are consumed through the `code-code-contracts` submodule.

Useful checks:

```bash
git submodule update --init --recursive
cd packages/platform-k8s
go test ./internal/modelservice/... ./internal/supportservice/... ./internal/cliruntimeservice/... ./cmd/platform-model-service ./cmd/platform-support-service ./cmd/platform-cli-runtime-service
```
