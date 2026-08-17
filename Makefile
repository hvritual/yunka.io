GO ?= go
VULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.7.0
MODULES := pkg framework gateway app app/cmd/rpc

.PHONY: test race vet vuln tidy build verify

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

build:
	@set -eu; for module in $(MODULES); do \
		echo "==> go build ./$$module/..."; \
		(cd $$module && $(GO) build ./...); \
	done

verify: test race vet vuln build
