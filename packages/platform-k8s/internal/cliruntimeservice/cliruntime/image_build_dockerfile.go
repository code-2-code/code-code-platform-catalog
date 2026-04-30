package cliruntime

import (
	"bytes"
	"fmt"
	"text/template"
)

// buildDockerfileContents generates a self-contained Dockerfile for the given
// build target. All agent scripts are inlined via heredoc COPY so BuildKit
// needs no external build context (no git clone).
func buildDockerfileContents(buildTarget string) (string, error) {
	spec, ok := agentBuildSpecs[buildTarget]
	if !ok {
		return "", fmt.Errorf("platformk8s/cliruntime: unknown build target %q", buildTarget)
	}
	var buf bytes.Buffer
	if err := dockerfileTmpl.Execute(&buf, spec); err != nil {
		return "", fmt.Errorf("platformk8s/cliruntime: render dockerfile: %w", err)
	}
	return buf.String(), nil
}

func buildTargetToAgentDir(buildTarget string) string {
	if s, ok := agentBuildSpecs[buildTarget]; ok {
		return s.AgentDir
	}
	return ""
}

func buildTargetToCLIPackage(buildTarget string) string {
	if s, ok := agentBuildSpecs[buildTarget]; ok {
		return s.CLIPackage
	}
	return ""
}

// agentBuildSpec holds per-agent data rendered into the Dockerfile template.
type agentBuildSpec struct {
	AgentDir   string
	CLIPackage string
	// Extra is optional content for an additional COPY heredoc block
	// (e.g. configure-gemini.js). Empty means no extra file.
	ExtraFile    string // destination filename inside /usr/local/lib/<AgentDir>/
	ExtraContent string
	Entrypoint   string
	Prepare      string
}

// -------------------------------------------------------------------
// Shared common scripts (deploy/agents/common/)
// -------------------------------------------------------------------

const commonAuthHelper = `#!/bin/sh
set -eu
CLI_CREDENTIAL_VALUE="${CLI_CREDENTIAL_VALUE:-}"
CLI_CREDENTIAL_FILE_PATH="${CLI_CREDENTIAL_FILE_PATH:-/run/cli-credential/token}"
if [ -n "${CLI_CREDENTIAL_VALUE}" ]; then
  printf '%s' "${CLI_CREDENTIAL_VALUE}"
  exit 0
fi
if [ ! -f "${CLI_CREDENTIAL_FILE_PATH}" ]; then
  echo "missing credential file: ${CLI_CREDENTIAL_FILE_PATH}" >&2
  exit 1
fi
tr -d '\r' <"${CLI_CREDENTIAL_FILE_PATH}"
`

const commonCLIOutputRuntime = `#!/bin/sh
set -eu
CLI_OUTPUT_FIFO_PATH="${CLI_OUTPUT_FIFO_PATH:-/run/cli-output/raw/events.fifo}"
CLI_OUTPUT_READY_PATH="${CLI_OUTPUT_READY_PATH:-/run/cli-output/status/ready}"
CLI_OUTPUT_TERMINAL_PATH="${CLI_OUTPUT_TERMINAL_PATH:-/run/cli-output/raw/terminal.json}"
CLI_OUTPUT_STOP_PATH="${CLI_OUTPUT_STOP_PATH:-/run/cli-output/control/stop.json}"
CLI_OUTPUT_WAIT_TIMEOUT_SECONDS="${CLI_OUTPUT_WAIT_TIMEOUT_SECONDS:-30}"
wait_for_cli_output_ready() {
  deadline=$(( $(date +%s) + CLI_OUTPUT_WAIT_TIMEOUT_SECONDS ))
  while [ ! -p "${CLI_OUTPUT_FIFO_PATH}" ] || [ ! -f "${CLI_OUTPUT_READY_PATH}" ]; do
    if [ "$(date +%s)" -ge "${deadline}" ]; then
      echo "cli output sidecar not ready: missing fifo or ready file" >&2
      exit 1
    fi
    sleep 1
  done
}
start_stop_watcher() {
  child_pid="$1"
  (
    while kill -0 "${child_pid}" 2>/dev/null; do
      if [ -f "${CLI_OUTPUT_STOP_PATH}" ]; then
        if grep -q '"force":[[:space:]]*true' "${CLI_OUTPUT_STOP_PATH}" 2>/dev/null; then
          kill -TERM "${child_pid}" 2>/dev/null || true
        else
          kill -INT "${child_pid}" 2>/dev/null || true
        fi
        exit 0
      fi
      sleep 1
    done
  ) &
  CLI_OUTPUT_WATCHER_PID="$!"
}
run_cli_output_stream() {
  wait_for_cli_output_ready
  "$@" >"${CLI_OUTPUT_FIFO_PATH}" 2>&1 &
  child_pid="$!"
  CLI_OUTPUT_WATCHER_PID=""
  start_stop_watcher "${child_pid}"
  watcher_pid="${CLI_OUTPUT_WATCHER_PID}"
  set +e
  wait "${child_pid}"
  status="$?"
  set -e
  kill "${watcher_pid}" 2>/dev/null || true
  wait "${watcher_pid}" 2>/dev/null || true
  mkdir -p "$(dirname "${CLI_OUTPUT_TERMINAL_PATH}")"
  terminal_tmp="$(mktemp "$(dirname "${CLI_OUTPUT_TERMINAL_PATH}")/.terminal.XXXXXX")"
  printf '{"exit_code":%d}\n' "${status}" >"${terminal_tmp}"
  chmod 660 "${terminal_tmp}"
  mv "${terminal_tmp}" "${CLI_OUTPUT_TERMINAL_PATH}"
  return "${status}"
}
`

// -------------------------------------------------------------------
// Per-agent specs
// -------------------------------------------------------------------

var agentBuildSpecs = map[string]agentBuildSpec{
	"claude-code-agent": {
		AgentDir:   "claude-code",
		CLIPackage: "@anthropic-ai/claude-code",
		Entrypoint: `#!/bin/sh
set -eu
. /usr/local/bin/cli-output-runtime.sh
CLAUDE_PROMPT="${CLAUDE_PROMPT:-${AGENT_RUN_PROMPT:-Reply with exactly OK}}"
CLAUDE_EXPECT_TEXT="${CLAUDE_EXPECT_TEXT:-OK}"
CLAUDE_MODEL="${CLAUDE_MODEL:-${AGENT_RUN_MODEL:-}}"
CLAUDE_BASE_URL="${CLAUDE_BASE_URL:-${AGENT_RUN_RUNTIME_URL:-}}"
CLAUDE_AUTH_MATERIALIZATION_KEY="${CLAUDE_AUTH_MATERIALIZATION_KEY:-${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}}"
CLAUDE_BASE_URL_FILE="${CLAUDE_BASE_URL_FILE:-/run/cli-runtime/base_url}"
CLAUDE_HOME_DIR="${CLAUDE_HOME_DIR:-${HOME}/.claude}"
CLAUDE_PLACEHOLDER_VALUE="${CLAUDE_PLACEHOLDER_VALUE:-PLACEHOLDER}"
if [ -z "${CLAUDE_BASE_URL}" ] && [ -f "${CLAUDE_BASE_URL_FILE}" ]; then
  CLAUDE_BASE_URL="$(tr -d '\r' <"${CLAUDE_BASE_URL_FILE}")"
fi
case "${CLAUDE_AUTH_MATERIALIZATION_KEY}" in
  claude-code.anthropic-api-key) ;;
  "") echo "missing Claude auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Claude auth materialization key: ${CLAUDE_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
if [ -z "${CLAUDE_BASE_URL}" ]; then
  echo "missing Claude base URL: set CLAUDE_BASE_URL or mount ${CLAUDE_BASE_URL_FILE}" >&2; exit 1
fi
if [ -z "${CLAUDE_MODEL}" ]; then
  echo "missing Claude model: set CLAUDE_MODEL" >&2; exit 1
fi
mkdir -p "${CLAUDE_HOME_DIR}"
export CLI_CREDENTIAL_VALUE="${CLAUDE_PLACEHOLDER_VALUE}"
cat >"${CLAUDE_HOME_DIR}/settings.json" <<'SETTINGS'
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "apiKeyHelper": "/usr/local/bin/claude-auth-helper.sh"
}
SETTINGS
export ANTHROPIC_BASE_URL="${CLAUDE_BASE_URL}"
run_cli_output_stream claude \
  -p --output-format stream-json --verbose --include-partial-messages \
  --permission-mode bypassPermissions --model "${CLAUDE_MODEL}" "${CLAUDE_PROMPT}"
`,
		Prepare: `#!/bin/sh
set -eu
case "${AGENT_PREPARE_JOB_TYPE:-}" in
  auth) ;; "") echo "missing prepare job type" >&2; exit 1 ;;
  *) echo "unsupported Claude prepare job type: ${AGENT_PREPARE_JOB_TYPE}" >&2; exit 1 ;;
esac
case "${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}" in
  claude-code.anthropic-api-key) ;; "") echo "missing Claude auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Claude auth materialization key: ${AGENT_RUN_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
if [ -z "${AGENT_RUN_RUNTIME_URL:-}" ]; then echo "missing Claude runtime URL" >&2; exit 1; fi
mkdir -p "${HOME:-/home/node}/.claude"
`,
	},
	"agent-cli-gemini": {
		AgentDir:   "gemini-cli",
		CLIPackage: "@google/gemini-cli",
		ExtraFile:  "configure-gemini.js",
		ExtraContent: `const fs = require("fs");
const path = require("path");
const SELECTED_AUTH_TYPE = "oauth-personal";
const PLACEHOLDER_VALUE = "PLACEHOLDER";
const PLACEHOLDER_TOKEN_TYPE = "Bearer";
const PLACEHOLDER_EXPIRY_DATE = 4102444800000;
if (!process.env.HOME) { throw new Error("missing HOME"); }
const homeDir = path.join(process.env.HOME, ".gemini");
const settingsPath = path.join(homeDir, "settings.json");
const credentialsPath = path.join(homeDir, "oauth_creds.json");
function writeJSONAtomically(filePath, data) {
  const tempPath = ` + "`${filePath}.tmp`" + `;
  fs.writeFileSync(tempPath, JSON.stringify(data, null, 2) + "\n", { mode: 0o600 });
  fs.renameSync(tempPath, filePath);
}
const settings = { security: { auth: { selectedType: SELECTED_AUTH_TYPE } }, privacy: { usageStatisticsEnabled: false } };
const credentials = { access_token: PLACEHOLDER_VALUE, refresh_token: PLACEHOLDER_VALUE, token_type: PLACEHOLDER_TOKEN_TYPE, expiry_date: PLACEHOLDER_EXPIRY_DATE };
fs.mkdirSync(homeDir, { recursive: true, mode: 0o700 });
writeJSONAtomically(settingsPath, settings);
writeJSONAtomically(credentialsPath, credentials);
`,
		Entrypoint: `#!/bin/sh
set -eu
. /usr/local/bin/cli-output-runtime.sh
GEMINI_PROMPT="${GEMINI_PROMPT:-${AGENT_RUN_PROMPT:-Reply with exactly OK}}"
GEMINI_MODEL="${GEMINI_MODEL:-${AGENT_RUN_MODEL:-}}"
GEMINI_MODEL_FILE="${GEMINI_MODEL_FILE:-/run/cli-runtime/model}"
GEMINI_CLI_BIN="${GEMINI_CLI_BIN:-gemini}"
GEMINI_CONFIGURE_SCRIPT="${GEMINI_CONFIGURE_SCRIPT:-/usr/local/lib/gemini-cli/configure-gemini.js}"
GEMINI_AUTH_MATERIALIZATION_KEY="${GEMINI_AUTH_MATERIALIZATION_KEY:-${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}}"
load_optional_file_value() {
  value="$1"; file_path="$2"
  if [ -n "${value}" ] || [ ! -f "${file_path}" ]; then printf '%s' "${value}"; return; fi
  tr -d '\r' <"${file_path}"
}
export GOOGLE_GENAI_USE_GCA=true
case "${GEMINI_AUTH_MATERIALIZATION_KEY}" in
  gemini-cli.google-oauth) ;; "") echo "missing Gemini auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Gemini auth materialization key: ${GEMINI_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
GEMINI_MODEL="$(load_optional_file_value "${GEMINI_MODEL}" "${GEMINI_MODEL_FILE}")"
node "${GEMINI_CONFIGURE_SCRIPT}"
set -- "${GEMINI_CLI_BIN}" --prompt "${GEMINI_PROMPT}" --output-format stream-json
if [ -n "${GEMINI_MODEL}" ]; then set -- "$@" --model "${GEMINI_MODEL}"; fi
run_cli_output_stream "$@"
`,
		Prepare: `#!/bin/sh
set -eu
case "${AGENT_PREPARE_JOB_TYPE:-}" in
  auth) ;; "") echo "missing prepare job type" >&2; exit 1 ;;
  *) echo "unsupported Gemini prepare job type: ${AGENT_PREPARE_JOB_TYPE}" >&2; exit 1 ;;
esac
case "${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}" in
  gemini-cli.google-oauth) ;; "") echo "missing Gemini auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Gemini auth materialization key: ${AGENT_RUN_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
mkdir -p "${HOME:-/home/node}"
`,
	},
	"agent-cli-qwen": {
		AgentDir:   "qwen-cli",
		CLIPackage: "@qwen-code/qwen-code",
		ExtraFile:  "configure-qwen.js",
		ExtraContent: `const fs = require("fs");
const path = require("path");
if (!process.env.HOME) { throw new Error("missing HOME"); }
const homeDir = path.join(process.env.HOME, ".qwen");
const settingsPath = path.join(homeDir, "settings.json");
const credentialsPath = path.join(homeDir, "oauth_creds.json");
const modelName = (process.env.QWEN_MODEL || "").trim();
const authMaterializationKey = (process.env.QWEN_AUTH_MATERIALIZATION_KEY || "").trim();
const placeholderValue = (process.env.QWEN_PLACEHOLDER_VALUE || "").trim();
const baseURLValue = (process.env.QWEN_BASE_URL || "").trim();
const baseURLFile = process.env.QWEN_BASE_URL_FILE || "";
const apiKeyEnvName = (process.env.QWEN_API_KEY_ENV_NAME || "QWEN_PLACEHOLDER_API_KEY").trim();
function readTrimmed(p) { if (!p) return ""; try { return fs.readFileSync(p,"utf8").replace(/\r/g,"").trim(); } catch(e) { if (e&&e.code==="ENOENT") return ""; throw e; } }
function requireString(v,f) { if (!v) throw new Error(` + "`missing ${f}`" + `); return v; }
function parseURL(r) { const t=r.trim(); if (!t) return null; return new URL(t.includes("://")?t:` + "`https://${t}`" + `); }
function normalizeOpenAIBaseURL(r) {
  const p=parseURL(r); if (!p) return "";
  const np=p.pathname.replace(/\/+$/,"");
  if (p.search||p.hash) throw new Error("invalid baseUrl query or fragment");
  if (p.username||p.password) throw new Error("invalid baseUrl credentials");
  const a=p.port?` + "`${p.hostname}:${p.port}`" + `:p.hostname;
  if (!a) throw new Error("invalid baseUrl host");
  return np?` + "`${p.protocol}//${a}${np}`" + `:` + "`${p.protocol}//${a}`" + `;
}
function loadBaseURL() { if (baseURLValue) return baseURLValue; const f=readTrimmed(baseURLFile); if (f) return f; throw new Error(` + "`missing baseUrl: set QWEN_BASE_URL or mount ${baseURLFile}`" + `); }
function writeJSONAtomically(fp, data) { const tp=` + "`${fp}.tmp`" + `; fs.writeFileSync(tp,JSON.stringify(data,null,2)+"\n",{mode:0o600}); fs.renameSync(tp,fp); }
function removeIfExists(fp) { try { fs.unlinkSync(fp); } catch(e) { if (!e||e.code!=="ENOENT") throw e; } }
function buildOpenAICompatibleSettings() {
  if (authMaterializationKey!=="qwen-cli.openai-compatible-api-key") throw new Error(` + "`unsupported Qwen auth materialization key: ${authMaterializationKey}`" + `);
  const modelId=requireString(modelName,"QWEN_MODEL");
  const baseUrl=requireString(normalizeOpenAIBaseURL(loadBaseURL()),"baseUrl");
  return { $version:3, env:{[requireString(apiKeyEnvName,"QWEN_API_KEY_ENV_NAME")]:requireString(placeholderValue,"QWEN_PLACEHOLDER_VALUE")}, modelProviders:{openai:[{id:modelId,name:modelId,baseUrl,envKey:apiKeyEnvName}]}, security:{auth:{selectedType:"openai"}}, model:{name:modelId} };
}
fs.mkdirSync(homeDir, { recursive: true, mode: 0o700 });
writeJSONAtomically(settingsPath, buildOpenAICompatibleSettings());
removeIfExists(credentialsPath);
`,
		Entrypoint: `#!/bin/sh
set -eu
. /usr/local/bin/cli-output-runtime.sh
QWEN_PROMPT="${QWEN_PROMPT:-${AGENT_RUN_PROMPT:-Reply with exactly OK}}"
QWEN_MODEL="${QWEN_MODEL:-${AGENT_RUN_MODEL:-}}"
QWEN_MODEL_FILE="${QWEN_MODEL_FILE:-/run/cli-runtime/model}"
QWEN_BASE_URL="${QWEN_BASE_URL:-${AGENT_RUN_RUNTIME_URL:-}}"
QWEN_AUTH_MATERIALIZATION_KEY="${QWEN_AUTH_MATERIALIZATION_KEY:-${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}}"
QWEN_BASE_URL_FILE="${QWEN_BASE_URL_FILE:-/run/cli-runtime/base_url}"
QWEN_PLACEHOLDER_VALUE="${QWEN_PLACEHOLDER_VALUE:-PLACEHOLDER}"
QWEN_API_KEY_ENV_NAME="${QWEN_API_KEY_ENV_NAME:-QWEN_PLACEHOLDER_API_KEY}"
QWEN_CLI_BIN="${QWEN_CLI_BIN:-qwen}"
QWEN_CONFIGURE_SCRIPT="${QWEN_CONFIGURE_SCRIPT:-/usr/local/lib/qwen-cli/configure-qwen.js}"
if [ -z "${QWEN_MODEL}" ] && [ -f "${QWEN_MODEL_FILE}" ]; then
  QWEN_MODEL="$(tr -d '\r' <"${QWEN_MODEL_FILE}")"
fi
case "${QWEN_AUTH_MATERIALIZATION_KEY}" in
  qwen-cli.openai-compatible-api-key) ;; "") echo "missing Qwen auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Qwen auth materialization key: ${QWEN_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
export QWEN_MODEL QWEN_BASE_URL QWEN_AUTH_MATERIALIZATION_KEY QWEN_BASE_URL_FILE QWEN_PLACEHOLDER_VALUE QWEN_API_KEY_ENV_NAME
node "${QWEN_CONFIGURE_SCRIPT}"
set -- "${QWEN_CLI_BIN}" -o stream-json --include-partial-messages --chat-recording --approval-mode yolo --channel CI "${QWEN_PROMPT}"
run_cli_output_stream "$@"
`,
		Prepare: `#!/bin/sh
set -eu
case "${AGENT_PREPARE_JOB_TYPE:-}" in
  auth) ;; "") echo "missing prepare job type" >&2; exit 1 ;;
  *) echo "unsupported Qwen prepare job type: ${AGENT_PREPARE_JOB_TYPE}" >&2; exit 1 ;;
esac
case "${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}" in
  qwen-cli.openai-compatible-api-key) ;; "") echo "missing Qwen auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Qwen auth materialization key: ${AGENT_RUN_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
if [ -z "${AGENT_RUN_RUNTIME_URL:-}" ]; then echo "missing Qwen runtime URL" >&2; exit 1; fi
mkdir -p "${HOME:-/home/node}"
`,
	},
	"agent-cli-codex": {
		AgentDir:   "codex-cli",
		CLIPackage: "@openai/codex",
		ExtraFile:  "configure-codex.js",
		ExtraContent: `const fs = require("fs");
const path = require("path");
if (!process.env.HOME) { throw new Error("missing HOME"); }
const homeDir = path.join(process.env.HOME, ".codex");
const settingsPath = path.join(homeDir, "settings.json");
const credentialsPath = path.join(homeDir, "oauth_creds.json");
const modelName = (process.env.CODEX_MODEL || "").trim();
const authMaterializationKey = (process.env.CODEX_AUTH_MATERIALIZATION_KEY || "").trim();
const placeholderValue = (process.env.CODEX_PLACEHOLDER_VALUE || "").trim();
const baseURLValue = (process.env.CODEX_BASE_URL || "").trim();
const baseURLFile = process.env.CODEX_BASE_URL_FILE || "";
const apiKeyEnvName = (process.env.CODEX_API_KEY_ENV_NAME || "CODEX_PLACEHOLDER_API_KEY").trim();
function readTrimmed(p) { if (!p) return ""; try { return fs.readFileSync(p,"utf8").replace(/\r/g,"").trim(); } catch(e) { if (e&&e.code==="ENOENT") return ""; throw e; } }
function requireString(v,f) { if (!v) throw new Error(` + "`missing ${f}`" + `); return v; }
function parseURL(r) { const t=r.trim(); if (!t) return null; return new URL(t.includes("://")?t:` + "`https://${t}`" + `); }
function normalizeOpenAIBaseURL(r) {
  const p=parseURL(r); if (!p) return "";
  const np=p.pathname.replace(/\/+$/,"");
  if (p.search||p.hash) throw new Error("invalid baseUrl query or fragment");
  if (p.username||p.password) throw new Error("invalid baseUrl credentials");
  const a=p.port?` + "`${p.hostname}:${p.port}`" + `:p.hostname;
  if (!a) throw new Error("invalid baseUrl host");
  return np?` + "`${p.protocol}//${a}${np}`" + `:` + "`${p.protocol}//${a}`" + `;
}
function loadBaseURL() { if (baseURLValue) return baseURLValue; const f=readTrimmed(baseURLFile); if (f) return f; throw new Error(` + "`missing baseUrl: set CODEX_BASE_URL or mount ${baseURLFile}`" + `); }
function writeJSONAtomically(fp, data) { const tp=` + "`${fp}.tmp`" + `; fs.writeFileSync(tp,JSON.stringify(data,null,2)+"\n",{mode:0o600}); fs.renameSync(tp,fp); }
function removeIfExists(fp) { try { fs.unlinkSync(fp); } catch(e) { if (!e||e.code!=="ENOENT") throw e; } }
function buildOpenAICompatibleSettings() {
  if (authMaterializationKey!=="codex.openai-compatible-api-key" && authMaterializationKey!=="codex.openai-oauth" && authMaterializationKey!=="codex.openai-responses-api-key") throw new Error(` + "`unsupported Codex auth materialization key: ${authMaterializationKey}`" + `);
  const modelId=requireString(modelName,"CODEX_MODEL");
  const baseUrl=requireString(normalizeOpenAIBaseURL(loadBaseURL()),"baseUrl");
  return { $version:3, env:{[requireString(apiKeyEnvName,"CODEX_API_KEY_ENV_NAME")]:requireString(placeholderValue,"CODEX_PLACEHOLDER_VALUE")}, modelProviders:{openai:[{id:modelId,name:modelId,baseUrl,envKey:apiKeyEnvName}]}, security:{auth:{selectedType:"openai"}}, model:{name:modelId} };
}
fs.mkdirSync(homeDir, { recursive: true, mode: 0o700 });
writeJSONAtomically(settingsPath, buildOpenAICompatibleSettings());
removeIfExists(credentialsPath);
`,
		Entrypoint: `#!/bin/sh
set -eu
. /usr/local/bin/cli-output-runtime.sh
CODEX_PROMPT="${CODEX_PROMPT:-${AGENT_RUN_PROMPT:-Reply with exactly OK}}"
CODEX_MODEL="${CODEX_MODEL:-${AGENT_RUN_MODEL:-}}"
CODEX_MODEL_FILE="${CODEX_MODEL_FILE:-/run/cli-runtime/model}"
CODEX_BASE_URL="${CODEX_BASE_URL:-${AGENT_RUN_RUNTIME_URL:-}}"
CODEX_AUTH_MATERIALIZATION_KEY="${CODEX_AUTH_MATERIALIZATION_KEY:-${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}}"
CODEX_BASE_URL_FILE="${CODEX_BASE_URL_FILE:-/run/cli-runtime/base_url}"
CODEX_PLACEHOLDER_VALUE="${CODEX_PLACEHOLDER_VALUE:-PLACEHOLDER}"
CODEX_API_KEY_ENV_NAME="${CODEX_API_KEY_ENV_NAME:-CODEX_PLACEHOLDER_API_KEY}"
CODEX_CLI_BIN="${CODEX_CLI_BIN:-codex}"
CODEX_CONFIGURE_SCRIPT="${CODEX_CONFIGURE_SCRIPT:-/usr/local/lib/codex-cli/configure-codex.js}"
if [ -z "${CODEX_MODEL}" ] && [ -f "${CODEX_MODEL_FILE}" ]; then
  CODEX_MODEL="$(tr -d '\r' <"${CODEX_MODEL_FILE}")"
fi
case "${CODEX_AUTH_MATERIALIZATION_KEY}" in
  codex.openai-compatible-api-key|codex.openai-oauth|codex.openai-responses-api-key) ;; "") echo "missing Codex auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Codex auth materialization key: ${CODEX_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
export CODEX_MODEL CODEX_BASE_URL CODEX_AUTH_MATERIALIZATION_KEY CODEX_BASE_URL_FILE CODEX_PLACEHOLDER_VALUE CODEX_API_KEY_ENV_NAME
node "${CODEX_CONFIGURE_SCRIPT}"
set -- "${CODEX_CLI_BIN}" -o stream-json --include-partial-messages --chat-recording --approval-mode yolo --channel CI "${CODEX_PROMPT}"
run_cli_output_stream "$@"
`,
		Prepare: `#!/bin/sh
set -eu
case "${AGENT_PREPARE_JOB_TYPE:-}" in
  auth) ;; "") echo "missing prepare job type" >&2; exit 1 ;;
  *) echo "unsupported Codex prepare job type: ${AGENT_PREPARE_JOB_TYPE}" >&2; exit 1 ;;
esac
case "${AGENT_RUN_AUTH_MATERIALIZATION_KEY:-}" in
  codex.openai-compatible-api-key|codex.openai-oauth|codex.openai-responses-api-key) ;; "") echo "missing Codex auth materialization key" >&2; exit 1 ;;
  *) echo "unsupported Codex auth materialization key: ${AGENT_RUN_AUTH_MATERIALIZATION_KEY}" >&2; exit 1 ;;
esac
if [ -z "${AGENT_RUN_RUNTIME_URL:-}" ]; then echo "missing Codex runtime URL" >&2; exit 1; fi
mkdir -p "${HOME:-/home/node}"
`,
	},
}

// dockerfileTmpl renders a self-contained Dockerfile that inlines all scripts
// via heredoc COPY. BuildKit 1.4+ is required for the heredoc syntax.
var dockerfileTmpl = template.Must(template.New("dockerfile").Parse(`# syntax=docker/dockerfile:1

FROM node:24-bookworm-slim

ARG NPM_CONFIG_REGISTRY
ARG CLI_PACKAGE
ARG CLI_VERSION
ARG AGENT_DIR

ENV NPM_CONFIG_UPDATE_NOTIFIER=false \
    NPM_CONFIG_FUND=false

RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    test -n "${CLI_PACKAGE}" && test -n "${CLI_VERSION}" && test -n "${AGENT_DIR}"; \
    if [ -n "${NPM_CONFIG_REGISTRY}" ]; then npm config set registry "${NPM_CONFIG_REGISTRY}"; fi; \
    npm install -g "${CLI_PACKAGE}@${CLI_VERSION}"

WORKDIR /workspace

COPY <<'__SCRIPT__' /usr/local/lib/code-code-agent/common/auth-helper.sh
` + commonAuthHelper + `__SCRIPT__

COPY <<'__SCRIPT__' /usr/local/lib/code-code-agent/common/cli-output-runtime.sh
` + commonCLIOutputRuntime + `__SCRIPT__

COPY <<'__SCRIPT__' /usr/local/lib/{{.AgentDir}}/entrypoint.sh
{{.Entrypoint}}__SCRIPT__

COPY <<'__SCRIPT__' /usr/local/lib/{{.AgentDir}}/prepare.sh
{{.Prepare}}__SCRIPT__
{{- if .ExtraFile}}

COPY <<'__SCRIPT__' /usr/local/lib/{{.AgentDir}}/{{.ExtraFile}}
{{.ExtraContent}}__SCRIPT__
{{- end}}

RUN install -m 0755 /usr/local/lib/{{.AgentDir}}/entrypoint.sh /usr/local/bin/agent-entrypoint.sh \
    && install -m 0755 /usr/local/lib/{{.AgentDir}}/prepare.sh /usr/local/bin/agent-prepare.sh \
    && install -m 0755 /usr/local/lib/code-code-agent/common/cli-output-runtime.sh /usr/local/bin/cli-output-runtime.sh \
    && install -m 0755 /usr/local/lib/code-code-agent/common/auth-helper.sh /usr/local/bin/claude-auth-helper.sh \
    && chown -R node:node /workspace

USER node

ENTRYPOINT ["/usr/local/bin/agent-entrypoint.sh"]
`))
