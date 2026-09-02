#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-report}"
case "${MODE}" in
  report|check) ;;
  *)
    echo "usage: $0 [report|check]" >&2
    exit 2
    ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO:-go}"
OUT_DIR="${DEPENDENCY_RESOLUTION_OUT:-${ROOT}/.artifacts/dependency-resolution}"
MODULES=(pkg framework gateway app app/cmd/rpc)

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

normalize_graph() {
  local module="$1"
  local gowork="$2"

  (
    cd "${ROOT}/${module}"
    GOWORK="${gowork}" "${GO_BIN}" list -m -f '{{if not .Main}}{{.Path}} {{if .Version}}{{.Version}}{{else}}(none){{end}}{{if .Replace}} => {{.Replace.Path}} {{if .Replace.Version}}{{.Replace.Version}}{{else}}(local){{end}}{{end}}{{end}}' all
  ) | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

workspace_file="${ROOT}/go.work"
if [[ ! -f "${workspace_file}" ]]; then
  echo "dependency-resolution: missing ${workspace_file}" >&2
  exit 2
fi

drift_modules=0
summary="${OUT_DIR}/summary.txt"
: > "${summary}"

printf 'Dependency resolution comparison\n' | tee -a "${summary}"
printf 'workspace: %s\n' "${workspace_file}" | tee -a "${summary}"
printf 'mode: %s\n\n' "${MODE}" | tee -a "${summary}"

for module in "${MODULES[@]}"; do
  safe_name="${module//\//_}"
  workspace_graph="${OUT_DIR}/${safe_name}.workspace.txt"
  isolated_graph="${OUT_DIR}/${safe_name}.isolated.txt"
  diff_file="${OUT_DIR}/${safe_name}.diff"

  normalize_graph "${module}" "${workspace_file}" > "${workspace_graph}"
  normalize_graph "${module}" "off" > "${isolated_graph}"

  if diff -u "${isolated_graph}" "${workspace_graph}" > "${diff_file}"; then
    rm -f "${diff_file}"
    printf '[OK] %s: workspace and GOWORK=off select the same dependency graph\n' "${module}" | tee -a "${summary}"
  else
    drift_modules=$((drift_modules + 1))
    isolated_count="$(wc -l < "${isolated_graph}" | tr -d ' ')"
    workspace_count="$(wc -l < "${workspace_graph}" | tr -d ' ')"
    diff_lines="$(wc -l < "${diff_file}" | tr -d ' ')"
    printf '[DRIFT] %s: isolated=%s workspace=%s diff-lines=%s\n' \
      "${module}" "${isolated_count}" "${workspace_count}" "${diff_lines}" | tee -a "${summary}"
  fi
done

printf '\nmodules-with-drift: %s/%s\n' "${drift_modules}" "${#MODULES[@]}" | tee -a "${summary}"
printf 'artifacts: %s\n' "${OUT_DIR}" | tee -a "${summary}"

if [[ "${MODE}" == "check" && "${drift_modules}" -ne 0 ]]; then
  echo "dependency-resolution: drift detected; see ${OUT_DIR}/*.diff" >&2
  exit 1
fi
