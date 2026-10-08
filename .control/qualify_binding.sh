#!/usr/bin/env bash
set -euo pipefail
out="$RUNNER_TEMP/http-binding"
mkdir -p "$out"
test "$(git rev-parse HEAD)" = "$BASE_SHA"
test "$(git ls-remote origin refs/heads/main | cut -f1)" = "$BASE_SHA"
git show "$CONTROL_SHA:.control/install_binding.py" > "$RUNNER_TEMP/install_binding.py"
python3 "$RUNNER_TEMP/install_binding.py" runtime
export PROTOC="$(command -v protoc)" PROTOC_GEN_GO="$PWD/.yunka/bin/protoc-gen-go" PROTOC_GEN_GO_GRPC="$PWD/.yunka/bin/protoc-gen-go-grpc" YUNKA_REQUIRE_C9_RUNTIME=1
set +e
(cd pkg && go test -count=1 -v ./contract -run '^TestGeneratedHTTPBindingTransportParity$') > "$out/runtime-red.log" 2>&1
code=$?
set -e
tail -n 70 "$out/runtime-red.log"
test "$code" -ne 0
grep -q 'HTTP_BINDING_PARAMETER_LOSS' "$out/runtime-red.log"
git diff --exit-code
sha256sum pkg/contract/http_binding_runtime_test.go > "$out/test-source.sha256"
python3 "$RUNNER_TEMP/install_binding.py" implementation
git diff --check
(cd pkg && go test -count=1 -v ./contract -run '^(TestHTTPBinding|TestGeneratedHTTPBindingTransportParity)') > "$out/runtime-green.log" 2>&1 || { tail -n 180 "$out/runtime-green.log"; exit 1; }
tail -n 120 "$out/runtime-green.log"
sha256sum -c "$out/test-source.sha256"
(cd pkg && YUNKA_REQUIRE_C9_RUNTIME=0 go test -count=1 ./contract) > "$out/contract-regression.log" 2>&1 || { tail -n 180 "$out/contract-regression.log"; exit 1; }
make contract contract-check > "$out/artifact-reproducibility.log" 2>&1 || { tail -n 120 "$out/artifact-reproducibility.log"; exit 1; }
git diff --exit-code -- contracts/generated
python3 - <<'PY'
import subprocess
expected = {'Makefile','docs/STATUS.md','docs/architecture/HTTP-BINDING-SEMANTICS.md','pkg/contract/source_provenance.go','pkg/contract/model.go','pkg/contract/compiler.go','pkg/contract/c9_application_codegen.go','pkg/contract/application_codegen.go','pkg/contract/lint.go','pkg/contract/openapi.go','pkg/contract/typescript.go','pkg/contract/http_binding_plan.go','pkg/contract/http_binding_codegen.go','pkg/contract/http_binding_openapi.go','pkg/contract/http_binding_plan_test.go','pkg/contract/http_binding_runtime_test.go'}
actual = set(subprocess.check_output(['git','diff','--name-only'],text=True).splitlines()) | set(subprocess.check_output(['git','ls-files','--others','--exclude-standard'],text=True).splitlines())
assert actual == expected, (actual-expected,expected-actual)
subprocess.run(['git','add','--',*sorted(expected)],check=True)
PY
git diff --cached --check
git config user.name 'Yunka Delivery'
git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
test -z "$(git ls-remote --heads origin "$OUTPUT_BRANCH")"
git switch -c "$OUTPUT_BRANCH"
git commit -m 'fix(contract): preserve HTTP query semantics across generated transports'
git rev-parse HEAD HEAD^{tree} > "$out/candidate.txt"
python3 -B tools/qualify_branch_baseline.py --mode pr --base-ref main --base-sha "$BASE_SHA" --checked-sha "$(git rev-parse HEAD)" > "$out/baseline.json"
git diff "$BASE_SHA" HEAD > "$out/product.patch"
git archive --format=tar HEAD > "$out/candidate-source.tar"
test -z "$(git status --porcelain --untracked-files=all)"
test "$(git ls-remote origin refs/heads/main | cut -f1)" = "$BASE_SHA"
git push origin "HEAD:refs/heads/$OUTPUT_BRANCH"
