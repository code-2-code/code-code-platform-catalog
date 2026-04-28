package cerebras

// ModelsResponse is the API response from the Cerebras /v1/models endpoint.
type ModelsResponse struct {
	Data []Model `json:"data"`
}

// Model represents one entry from the Cerebras API.
type Model struct {
	ID            string `json:"id"`
	OwnedBy       string `json:"owned_by"`
	Name          string `json:"name"`
	HuggingFaceID string `json:"hugging_face_id"`
	Pricing       struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
	Capabilities struct {
		FunctionCalling   bool `json:"function_calling"`
		StructuredOutputs bool `json:"structured_outputs"`
		Vision            bool `json:"vision"`
		Reasoning         bool `json:"reasoning"`
	} `json:"capabilities"`
	Limits struct {
		MaxContextLength    int64 `json:"max_context_length"`
		MaxCompletionTokens int64 `json:"max_completion_tokens"`
	} `json:"limits"`
}
