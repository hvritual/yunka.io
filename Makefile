TOOLCHAIN_LOCK ?= $(CURDIR)/tools/toolchain.env
include $(TOOLCHAIN_LOCK)

GO ?= go
PROTOC ?= protoc
CONTRACT_SOURCES ?= $(CURDIR)/contracts/sources.json
CONTRACT_OUT ?= $(CURDIR)/contracts/generated
DEPENDENCY_POLICY ?= $(CURDIR)/tools/dependency-policy.json
RPC_TOOL_DIR ?= $(CURDIR)/.yunka/bin
GOEXE := $(shell $(GO) env GOEXE)
PROTOC_GEN_GO ?= $(RPC_TOOL_DIR)/protoc-gen-go$(GOEXE)
PROTOC_GEN_GO_GRPC ?= $(RPC_TOOL_DIR)/protoc-gen-go-grpc$(GOEXE)
RPC_GEN_DRIVER ?= $(CURDIR)/tools/rpcgen/generate.py
RPC_ABI_BASELINE ?= $(CURDIR)/contracts/baselines/c5-contract-manifest.json
PYTHON ?= python3
VULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
MODULES := compat/go-kit-kit-log pkg framework gateway app

.PHONY: toolchain-check rpc-tools rpc-toolchain-check rpc-generate rpc-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check dependency-check architecture-check module-check authz-check c7-check domain-check dsl-check test race vet vuln tidy build contract rpc-contract-check contract-check integration verify verify-production

toolchain-check:
	@set -eu; \
	required_go="$$(awk '$$1 == "toolchain" { sub(/^go/, "", $$2); print $$2; exit }' go.work)"; \
	actual_go="$$($(GO) version | awk '{print $$3}' | sed 's/^go//')"; \
	actual_protoc="$$($(PROTOC) --version | awk '{print $$2}')"; \
	test -n "$$required_go" || { echo "toolchain-check: go.work has no toolchain directive" >&2; exit 1; }; \
	test "$$required_go" = "$(GO_VERSION)" || { echo "toolchain-check: go.work=$$required_go lock=$(GO_VERSION)" >&2; exit 1; }; \
	test "$$actual_go" = "$(GO_VERSION)" || { echo "toolchain-check: go=$$actual_go want $(GO_VERSION)" >&2; exit 1; }; \
	test "$$actual_protoc" = "$(PROTOC_VERSION)" || { echo "toolchain-check: protoc=$$actual_protoc want $(PROTOC_VERSION)" >&2; exit 1; }; \
	echo "toolchain-check: go=$(GO_VERSION) protoc=$(PROTOC_VERSION) govulncheck=$(GOVULNCHECK_VERSION)"

rpc-tools:
	@mkdir -p "$(RPC_TOOL_DIR)"
	@GOBIN="$(RPC_TOOL_DIR)" $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@GOBIN="$(RPC_TOOL_DIR)" $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

rpc-toolchain-check:
	@set -eu; \
	actual_go="$$( $(PROTOC_GEN_GO) --version | awk '{print $$NF}' | sed 's/^v//' )"; \
	expected_go="$$(printf '%s' '$(PROTOC_GEN_GO_VERSION)' | sed 's/^v//')"; \
	actual_grpc="$$( $(PROTOC_GEN_GO_GRPC) --version | awk '{print $$NF}' | sed 's/^v//' )"; \
	expected_grpc="$$(printf '%s' '$(PROTOC_GEN_GO_GRPC_VERSION)' | sed 's/^v//')"; \
	test "$$actual_go" = "$$expected_go" || { echo "rpc-toolchain-check: protoc-gen-go=$$actual_go want $$expected_go" >&2; exit 1; }; \
	test "$$actual_grpc" = "$$expected_grpc" || { echo "rpc-toolchain-check: protoc-gen-go-grpc=$$actual_grpc want $$expected_grpc" >&2; exit 1; }; \
	echo "rpc-toolchain-check: protoc-gen-go=$$actual_go protoc-gen-go-grpc=$$actual_grpc"

rpc-generate: toolchain-check rpc-tools
	@$(MAKE) --no-print-directory rpc-toolchain-check
	@$(PYTHON) "$(RPC_GEN_DRIVER)" --repo-root "$(CURDIR)" --protoc "$(PROTOC)" \
		--protoc-gen-go "$(PROTOC_GEN_GO)" --protoc-gen-go-grpc "$(PROTOC_GEN_GO_GRPC)"
	@$(MAKE) contract

rpc-check: toolchain-check rpc-tools rpc-toolchain-check
	@$(PYTHON) "$(RPC_GEN_DRIVER)" --repo-root "$(CURDIR)" --protoc "$(PROTOC)" \
		--protoc-gen-go "$(PROTOC_GEN_GO)" --protoc-gen-go-grpc "$(PROTOC_GEN_GO_GRPC)" --check

rpc-compat-check: toolchain-check
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract check \
		--sources "$(CONTRACT_SOURCES)" --repo-root "$(CURDIR)" \
		--out "$(CONTRACT_OUT)" --title "yunka API" --version "1.0.0" \
		--baseline "$(RPC_ABI_BASELINE)"

rpc-legacy-check:
	@set -eu; \
	for removed in app/cmd/rpc gateway/rpc/gender.sh gateway/rpc/pb gateway/rpc/transport/memory pkg/invoke gateway/rpc/client/legacy_factory.go; do \
		test ! -e "$$removed" || { echo "rpc-legacy-check: stale legacy path $$removed" >&2; exit 1; }; \
	done; \
	stale="$$(find gateway pkg framework app -type f -name '*.xr_*.go' -print)"; \
	test -z "$$stale" || { printf 'rpc-legacy-check: stale XR files\n%s\n' "$$stale" >&2; exit 1; }; \
	if git grep -n -E 'protoc-gen-xr|xr-cluster|--go_out=plugins=grpc' -- go.work app gateway pkg framework tools ':!**/*_test.go'; then \
		echo "rpc-legacy-check: legacy generator reference remains" >&2; exit 1; \
	fi; \
	if git grep -n -E 'yunka\.io/pkg/invoke|invoke\.(Rpc(Client|Server)|Message|SrvHandler|RpcTimeOut)' -- app gateway framework pkg; then \
		echo "rpc-legacy-check: legacy invoke runtime remains" >&2; exit 1; \
	fi; \
	if git grep -n 'github.com/golang/protobuf' -- gateway/rpc framework/core/middleware framework/core/resilience pkg/selector; then \
		echo "rpc-legacy-check: legacy protobuf import remains in RPC code" >&2; exit 1; \
	fi; \
	if grep -R -n -E 'sync\.Pool|reflect\.Value|RegisterServer\(name string|SrvHandler|messageFactories|handlerMap' \
		gateway/rpc/bridge gateway/rpc/client gateway/rpc/handle \
		gateway/rpc/method gateway/rpc/server gateway/rpc/transport pkg/rpcbridge; then \
		echo "rpc-legacy-check: hidden registry, reflection, or pooling remains" >&2; exit 1; \
	fi

rpc-consumer-check:
	@$(PYTHON) tools/check_rpc_consumer_abi.py --repo-root "$(CURDIR)"
	@cd gateway && $(GO) test -count=20 ./dispatcher/intercept/role ./rpc/consumercompat ./rpc/bridge ./rpc/client ./rpc/transport/grpc
	@cd gateway && CGO_ENABLED=1 $(GO) test -race -count=3 ./rpc/consumercompat ./rpc/bridge ./rpc/client ./rpc/transport/grpc

rpc-bridge-check: rpc-legacy-check rpc-consumer-check
	@set -eu; \
	if grep -R -n -E 'sync\.Pool|reflect\.Value|func init\(\)' \
		pkg/rpcbridge gateway/rpc/bridge gateway/rpc/client gateway/rpc/handle \
		gateway/rpc/method gateway/rpc/server gateway/rpc/transport/grpc/server.go; then \
		echo "rpc-bridge-check: typed bridge contains hidden registration, reflection, or pooling" >&2; \
		exit 1; \
	fi
	@cd pkg && $(GO) test -count=10 ./rpcbridge
	@cd gateway && $(GO) test -count=10 ./rpc/bridge ./rpc/client ./rpc/consumercompat ./rpc/transport/grpc


dependency-check:
	@cd app && $(GO) run ./cmd dependency check \
		--repo-root "$(CURDIR)" --policy "$(DEPENDENCY_POLICY)" --go "$(GO)"

architecture-check:
	@cd pkg && $(GO) test -count=1 ./architecturepolicy

module-check: architecture-check
	@cd app && $(GO) test -count=10 ./cmd/module
	@cd app && $(GO) run ./cmd module check --root ../framework/modules

authz-check:
	@cd gateway && $(GO) test -count=20 ./authz ./dispatcher/bridge ./dispatcher/middleware ./rpc/transport/grpc
	@cd gateway && CGO_ENABLED=1 $(GO) test -race -count=3 ./authz ./dispatcher/bridge ./dispatcher/middleware ./rpc/transport/grpc
	@cd pkg && $(GO) test -count=20 ./contract

c7-check: architecture-check
	@cd framework && $(GO) test -count=20 ./core ./core/request ./platform ./requestscope ./kernel ./core/modulecatalog
	@cd framework && CGO_ENABLED=1 $(GO) test -race -count=3 ./core ./core/request ./platform ./requestscope ./kernel ./core/modulecatalog
	@cd gateway && $(GO) test -count=20 ./dispatcher/intercept/role ./dispatcher/middleware ./dispatcher/proxy ./rpc/bridge ./rpc/consumercompat ./rpc/transport/grpc
	@cd gateway && CGO_ENABLED=1 $(GO) test -race -count=3 ./dispatcher/middleware ./dispatcher/proxy ./rpc/bridge ./rpc/consumercompat ./rpc/transport/grpc

domain-check: architecture-check
	@cd app && $(GO) test -count=10 ./cmd/domain

dsl-check: toolchain-check architecture-check
	@cd pkg && $(GO) test -count=10 ./contract ./applicationgraph ./architecturepolicy
	@cd framework && $(GO) test -count=10 ./applicationgraph
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract lint \
		--sources "$(CONTRACT_SOURCES)" --repo-root "$(CURDIR)"

test:
	@set -eu; for module in $(MODULES); do \
		echo "==> go test ./$$module/..."; \
		(cd $$module && $(GO) test ./...); \
	done

race:
	@set -eu; for module in pkg framework gateway; do \
		echo "==> go test -race ./$$module/..."; \
		(cd $$module && CGO_ENABLED=1 $(GO) test -race ./...); \
	done

vet:
	@set -eu; for module in $(MODULES); do \
		echo "==> go vet ./$$module/..."; \
		(cd $$module && $(GO) vet ./...); \
	done

vuln:
	@set -eu; for module in $(MODULES); do \
		echo "==> govulncheck ./$$module/..."; \
		(cd $$module && $(VULNCHECK) ./...); \
	done

tidy:
	@set -eu; for module in $(MODULES); do \
		echo "==> go mod tidy ./$$module"; \
		(cd $$module && $(GO) mod tidy); \
	done
	$(GO) work sync

contract: toolchain-check
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract generate \
		--sources "$(CONTRACT_SOURCES)" --repo-root "$(CURDIR)" \
		--out "$(CONTRACT_OUT)" --title "yunka API" --version "1.0.0"

rpc-contract-check: toolchain-check
	@cd gateway && PROTOC="$(PROTOC)" YUNKA_REPOSITORY_ROOT="$(CURDIR)" \
		$(GO) test -count=1 -tags=contractsync ./rpc/meta

contract-check: toolchain-check rpc-contract-check
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract check \
		--sources "$(CONTRACT_SOURCES)" --repo-root "$(CURDIR)" \
		--out "$(CONTRACT_OUT)" --title "yunka API" --version "1.0.0"

build:
	@set -eu; for module in $(MODULES); do \
		echo "==> go build ./$$module/..."; \
		(cd $$module && $(GO) build ./...); \
	done

integration:
	@set -eu; \
	: "$${YUNKA_TEST_MYSQL_DSN:?YUNKA_TEST_MYSQL_DSN is required for MySQL integration tests}"; \
	echo "==> MySQL 8 transactional outbox and request-scope integration"; \
	(cd framework && $(GO) test -timeout=5m -count=1 -tags=integration ./event/outbox ./requestscope)

verify: toolchain-check dependency-check module-check authz-check c7-check domain-check dsl-check rpc-check contract-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check test race vet vuln build

verify-production: verify integration
