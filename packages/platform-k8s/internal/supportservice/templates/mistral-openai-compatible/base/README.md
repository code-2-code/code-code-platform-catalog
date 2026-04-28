# Mistral OpenAI-Compatible Base

这个目录是 `Mistral` 的 `Kustomize base`。

作用：

- 提供稳定的 `baseUrl`
- 提供稳定的资源骨架
- 提供一组可直接覆盖的初始模型位点

建议：

- 在自己的 overlay 里替换：
  - namespace
  - secret name
  - model ids
  - priority
