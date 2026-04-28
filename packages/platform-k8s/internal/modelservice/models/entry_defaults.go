package models

import modelv1 "code-code.internal/go-contract/model/v1"

// ApplyCreateDefaults fills in default values for a model definition.
func ApplyCreateDefaults(definition *modelv1.ModelVersion) {
	if definition.PrimaryShape == modelv1.ModelShape_MODEL_SHAPE_UNSPECIFIED {
		definition.PrimaryShape = modelv1.ModelShape_MODEL_SHAPE_CHAT_COMPLETIONS
	}
	if len(definition.SupportedShapes) == 0 {
		definition.SupportedShapes = []modelv1.ModelShape{definition.PrimaryShape}
	}
	if len(definition.InputModalities) == 0 {
		definition.InputModalities = []modelv1.Modality{modelv1.Modality_MODALITY_TEXT}
	}
	if len(definition.OutputModalities) == 0 {
		definition.OutputModalities = []modelv1.Modality{modelv1.Modality_MODALITY_TEXT}
	}
}
