package supportservice

import (
	"embed"
	"fmt"
	"slices"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	observabilityv1 "code-code.internal/go-contract/observability/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/yaml"
)

//go:embed runtime_http_telemetry_profiles.yaml
var runtimeTelemetryFS embed.FS

func loadRuntimeTelemetryProfiles() (*observabilityv1.ObservabilityCapability, error) {
	raw, err := runtimeTelemetryFS.ReadFile("runtime_http_telemetry_profiles.yaml")
	if err != nil {
		return nil, err
	}
	asJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/supportservice: decode runtime telemetry profiles: %w", err)
	}
	capability := &observabilityv1.ObservabilityCapability{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(asJSON, capability); err != nil {
		return nil, fmt.Errorf("platformk8s/supportservice: parse runtime telemetry profiles: %w", err)
	}
	if err := observabilityv1.ValidateCapability(capability); err != nil {
		return nil, fmt.Errorf("platformk8s/supportservice: invalid runtime telemetry profiles: %w", err)
	}
	return passiveHTTPObservability(capability), nil
}

func passiveHTTPObservability(capability *observabilityv1.ObservabilityCapability) *observabilityv1.ObservabilityCapability {
	if capability == nil {
		return nil
	}
	profiles := make([]*observabilityv1.ObservabilityProfile, 0, len(capability.GetProfiles()))
	seen := map[string]struct{}{}
	for _, profile := range capability.GetProfiles() {
		if profile == nil || profile.GetPassiveHttp() == nil {
			continue
		}
		key := strings.TrimSpace(profile.GetProfileId())
		if key == "" {
			key = strings.TrimSpace(profile.GetDisplayName())
		}
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		profiles = append(profiles, proto.Clone(profile).(*observabilityv1.ObservabilityProfile))
	}
	if len(profiles) == 0 {
		return nil
	}
	return &observabilityv1.ObservabilityCapability{Profiles: profiles}
}

func (s *Server) RuntimeTelemetryProfiles() *observabilityv1.ObservabilityCapability {
	if s == nil || s.runtimeTelemetry == nil {
		return nil
	}
	return proto.Clone(s.runtimeTelemetry).(*observabilityv1.ObservabilityCapability)
}

func (s *Server) protocolRuntimeTelemetry(protocol apiprotocolv1.Protocol) *observabilityv1.ObservabilityCapability {
	if s == nil || s.runtimeTelemetry == nil {
		return nil
	}
	ids := protocolRuntimeTelemetryProfileIDs(protocol)
	if len(ids) == 0 {
		return nil
	}
	profiles := make([]*observabilityv1.ObservabilityProfile, 0, len(ids))
	for _, profile := range s.runtimeTelemetry.GetProfiles() {
		if profile == nil || !slices.Contains(ids, strings.TrimSpace(profile.GetProfileId())) {
			continue
		}
		profiles = append(profiles, proto.Clone(profile).(*observabilityv1.ObservabilityProfile))
	}
	if len(profiles) == 0 {
		return nil
	}
	return &observabilityv1.ObservabilityCapability{Profiles: profiles}
}

func protocolRuntimeTelemetryProfileIDs(protocol apiprotocolv1.Protocol) []string {
	switch protocol {
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE:
		return []string{"protocol.openai-compatible.runtime-http-telemetry"}
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_RESPONSES:
		return []string{"protocol.openai-responses.runtime-http-telemetry"}
	default:
		return nil
	}
}
