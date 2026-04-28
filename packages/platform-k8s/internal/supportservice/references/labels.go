package references

import (
	"strings"

	providerv1 "code-code.internal/go-contract/provider/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProviderCredentialReferenceLabel = "platform.code-code.internal/provider-credential-ref"
)

func SetProviderRuntimeReferenceLabels(object metav1.Object, instance *providerv1.ProviderSurfaceBinding) {
	if object == nil {
		return
	}
	setReferenceLabel(object, ProviderCredentialReferenceLabel, instance.GetProviderCredentialRef().GetProviderCredentialId())
}

func setReferenceLabel(object metav1.Object, key, value string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		delete(labels, key)
	} else {
		labels[key] = value
	}
	if len(labels) == 0 {
		object.SetLabels(nil)
		return
	}
	object.SetLabels(labels)
}
