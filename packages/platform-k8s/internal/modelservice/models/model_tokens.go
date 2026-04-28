package models

import (
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

func hasModelToken(value string, tokens ...string) bool {
	return source.HasModelToken(value, tokens...)
}
