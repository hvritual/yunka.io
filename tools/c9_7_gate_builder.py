from pathlib import Path

root = Path(__file__).resolve().parents[1]
makefile = root / "Makefile"
text = makefile.read_text()
text = text.replace(
    ".PHONY: toolchain-check rpc-tools rpc-toolchain-check rpc-generate rpc-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check dependency-check architecture-check module-check authz-check operation-check c7-check domain-check dsl-check test race vet vuln tidy build contract rpc-contract-check contract-check integration verify verify-production",
    ".PHONY: toolchain-check rpc-tools rpc-toolchain-check rpc-generate rpc-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check dependency-check architecture-check module-check authz-check operation-check c9-7-check c7-check domain-check dsl-check test race vet vuln tidy build contract rpc-contract-check contract-check integration verify verify-production",
    1,
)
anchor = """operation-check: architecture-check
	@cd pkg && $(GO) test -count=20 ./operationplan ./contract ./applicationgraph
	@cd framework && $(GO) test -count=20 ./operation ./diagnostics
	@cd gateway && $(GO) test -count=20 ./authz ./rpc/transport/grpc
	@cd framework && CGO_ENABLED=1 $(GO) test -race -count=3 ./operation ./diagnostics
	@cd gateway && CGO_ENABLED=1 $(GO) test -race -count=3 ./authz ./rpc/transport/grpc

"""
replacement = anchor + """c9-7-check: architecture-check
	@cd pkg && $(GO) test -count=10 ./operationplan ./contract ./applicationgraph ./architecturepolicy
	@cd framework && $(GO) test -count=10 ./execution ./operation ./requestscope ./workflow/saga ./diagnostics
	@cd framework && CGO_ENABLED=1 $(GO) test -race -count=3 ./execution ./operation ./requestscope ./workflow/saga ./diagnostics
	@cd gateway && $(GO) test -count=10 ./authz ./rpc/transport/grpc

"""
if anchor not in text:
    raise SystemExit("operation-check anchor missing")
text = text.replace(anchor, replacement, 1)
old_verify = "verify: toolchain-check dependency-check module-check authz-check operation-check c7-check domain-check dsl-check rpc-check contract-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check test race vet vuln build"
new_verify = "verify: toolchain-check dependency-check module-check authz-check operation-check c9-7-check c7-check domain-check dsl-check rpc-check contract-check rpc-compat-check rpc-legacy-check rpc-consumer-check rpc-bridge-check test race vet vuln build"
if old_verify not in text:
    raise SystemExit("verify anchor missing")
text = text.replace(old_verify, new_verify, 1)
makefile.write_text(text)
