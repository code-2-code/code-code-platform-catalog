# MiniMax Anthropic Base

这个目录是 `MiniMax` 的 `Anthropic-compatible` `Kustomize base`。

作用：

- 提供稳定的 `baseUrl`
- 提供 endpoint 级固定 `modelCatalog`
- 提供一组可直接覆盖的资源骨架
- 这组 base 按 endpoint fallback path 设计，默认不要求 `ProviderCredentialRef`
- 默认 `modelCatalog` 固定包含 `MiniMax-M2.7`、`MiniMax-M2.7-highspeed`、`MiniMax-M2.5`、`MiniMax-M2.5-highspeed`

建议：

- 在自己的 overlay 里替换：
  - namespace
  - secret name
  - model ids
- 默认 `baseUrl` 使用 `https://api.minimaxi.com/anthropic`
- 如果你的 MiniMax key 属于国际站，再在 overlay 中改成 `https://api.minimax.io/anthropic`
