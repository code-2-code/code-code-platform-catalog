package templates

import "strings"

var templateTokenDisplayNames = map[string]string{}

func registerTemplateTokenDisplayName(token string, displayName string) {
	token = strings.TrimSpace(strings.ToLower(token))
	displayName = strings.TrimSpace(displayName)
	if token == "" || displayName == "" {
		return
	}
	templateTokenDisplayNames[token] = displayName
}

func templateTokenDisplayName(token string) string {
	return templateTokenDisplayNames[strings.TrimSpace(strings.ToLower(token))]
}
