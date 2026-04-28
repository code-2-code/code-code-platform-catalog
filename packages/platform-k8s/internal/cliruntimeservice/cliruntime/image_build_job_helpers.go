package cliruntime

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

const buildAndPushScript = `
case "${BUILD_TARGET}" in
  claude-code-agent) agent_dir="claude-code"; cli_package="@anthropic-ai/claude-code" ;;
  agent-cli-qwen) agent_dir="qwen-cli"; cli_package="@qwen-code/qwen-code" ;;
  agent-cli-gemini) agent_dir="gemini-cli"; cli_package="@google/gemini-cli" ;;
  *) echo "unknown build target ${BUILD_TARGET}" >&2; exit 2 ;;
esac
if [ -z "${SOURCE_CONTEXT}" ]; then
  echo "source-context is required" >&2
  exit 2
fi
build_context="${SOURCE_CONTEXT}"
if [ -n "${SOURCE_REVISION}" ]; then
  case "${SOURCE_CONTEXT}" in
    *'#'*|*'?'*) ;;
    *) build_context="${SOURCE_CONTEXT}#${SOURCE_REVISION}" ;;
  esac
fi
cache_ref="${IMAGE_REPOSITORY}:buildcache"
buildctl-daemonless.sh build \
  --frontend=gateway.v0 \
  --opt source=docker/dockerfile:1 \
  --opt context="${build_context}" \
  --opt filename="deploy/images/release/node-cli-agent.Dockerfile" \
  --opt build-arg:AGENT_DIR="${agent_dir}" \
  --opt build-arg:CLI_PACKAGE="${cli_package}" \
  --opt build-arg:CLI_VERSION="${CLI_VERSION}" \
  --opt build-arg:NPM_CONFIG_REGISTRY="${BUILD_NPM_REGISTRY}" \
  --import-cache type=registry,ref="${cache_ref}" \
  --export-cache type=registry,ref="${cache_ref}",mode=max \
  --output type=image,name="${IMAGE}",push=true
`

const pruneOldTagsScript = `
keep="${RETENTION_KEEP_TAGS}"
case "${keep}" in
  ''|*[!0-9]*|0) echo "retention disabled"; exit 0 ;;
esac
repo="${IMAGE_REPOSITORY}"
if [ -f /registry-auth/config.json ]; then
  mkdir -p "${HOME}/.docker"
  cp /registry-auth/config.json "${HOME}/.docker/config.json"
fi
tmp_file="$(mktemp)"
format='{''{ .Created }''}'
regctl tag ls "${repo}" | awk '/^cli-/ {print}' | while read -r tag; do
  created="$(regctl image inspect --format "${format}" "${repo}:${tag}" 2>/dev/null || true)"
  if [ -z "${created}" ]; then created="0001-01-01T00:00:00Z"; fi
  printf '%s %s\n' "${created}" "${tag}" >> "${tmp_file}"
done
tag_count="$(wc -l < "${tmp_file}" | tr -d ' ')"
delete_count=$((tag_count - keep))
if [ "${delete_count}" -le 0 ]; then
  echo "retention already satisfied for ${repo}: ${tag_count}/${keep}"
  exit 0
fi
sort "${tmp_file}" | awk -v n="${delete_count}" 'NR <= n {print $2}' | while read -r tag; do
  echo "delete old CLI image tag ${repo}:${tag}"
  regctl tag delete --ignore-missing "${repo}:${tag}"
done
`

func imageBuildJobName(request ImageBuildRequest) string {
	sum := sha1.Sum([]byte(request.RequestID))
	return "cli-image-build-" + dnsLabel(request.CLIID) + "-" + hex.EncodeToString(sum[:])[:10]
}

func jobActive(job *batchv1.Job) bool {
	return job != nil && job.Status.Succeeded == 0 && !jobFailed(job)
}

func jobFailed(job *batchv1.Job) bool {
	if job == nil {
		return false
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func imageBuildSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		ReadOnlyRootFilesystem:   boolPtr(false),
		RunAsNonRoot:             boolPtr(true),
		RunAsUser:                int64Ptr(1000),
		RunAsGroup:               int64Ptr(1000),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

func dnsLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "unknown"
	}
	if len(out) > 40 {
		out = strings.TrimRight(out[:40], "-")
	}
	return out
}

func boolPtr(value bool) *bool    { return &value }
func int32Ptr(value int32) *int32 { return &value }
func int64Ptr(value int64) *int64 { return &value }

func fsGroupChangePolicyPtr(value corev1.PodFSGroupChangePolicy) *corev1.PodFSGroupChangePolicy {
	return &value
}
