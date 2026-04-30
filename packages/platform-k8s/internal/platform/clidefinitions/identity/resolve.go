package identity

import (
	"context"
	"strings"

	clidefinitionv1 "code-code.internal/go-contract/cli_definition/v1"
	"code-code.internal/go-contract/domainerror"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveContainerImage(ctx context.Context, reader ctrlclient.Reader, namespace, cliID, executionClass string) (*clidefinitionv1.CLIContainerImage, error) {
	if reader == nil {
		return nil, domainerror.NewValidation("platformk8s/clidefinitions: reader is nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, domainerror.NewValidation("platformk8s/clidefinitions: namespace is empty")
	}
	_ = ctx
	definition, err := registeredCLIDefinition(cliID)
	if err != nil {
		return nil, domainerror.NewNotFound("platformk8s/clidefinitions: cli definition %q not found", cliID)
	}
	for _, image := range definition.GetContainerImages() {
		if image != nil && strings.TrimSpace(image.GetExecutionClass()) == strings.TrimSpace(executionClass) {
			return image, nil
		}
	}
	return nil, domainerror.NewValidation("platformk8s/clidefinitions: execution class %q is not declared by cli definition %q", executionClass, cliID)
}
