GO ?= go
PROTOC ?= protoc
CONTRACT_PROTO_DIR ?= $(CURDIR)/app/cmd/rpc/pb
CONTRACT_OUT ?= $(CURDIR)/contracts/generated
VULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.7.0
MODULES := pkg framework gateway app app/cmd/rpc

.PHONY: test race vet vuln tidy build contract contract-check integration verify verify-production

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

contract:
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract generate \
		--proto-dir "$(CONTRACT_PROTO_DIR)" --out "$(CONTRACT_OUT)" --title "yunka API" --version "1.0.0"

contract-check:
	@cd app && PROTOC="$(PROTOC)" $(GO) run ./cmd contract check \
		--proto-dir "$(CONTRACT_PROTO_DIR)" --out "$(CONTRACT_OUT)" --title "yunka API" --version "1.0.0"

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

verify: contract-check test race vet vuln build

verify-production: verify integration
