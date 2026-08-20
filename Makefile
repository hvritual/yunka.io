TOOLCHAIN_LOCK ?= $(CURDIR)/tools/toolchain.env
include $(TOOLCHAIN_LOCK)

GO ?= go
PROTOC ?= protoc
CONTRACT_SOURCES ?= $(CURDIR)/contracts/sources.json
CONTRACT_OUT ?= $(CURDIR)/contracts/generated
VULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
MODULES := pkg framework gateway app app/cmd/rpc

.PHONY: toolchain-check test race vet vuln tidy build contract rpc-contract-check contract-check integration verify verify-production

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
	echo "==> MySQL 8 transactional outbox integration"; \
	(cd framework && $(GO) test -timeout=5m -count=1 -tags=integration ./event/outbox)

verify: toolchain-check contract-check test race vet vuln build

verify-production: verify integration
