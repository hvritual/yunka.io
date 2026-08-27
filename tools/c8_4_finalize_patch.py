#!/usr/bin/env python3
from pathlib import Path


def patch_application_codegen() -> None:
    path = Path("pkg/contract/application_codegen.go")
    text = path.read_text()
    start_marker = "\t\t\tpathFields, _ := simplePathFields(binding.Path)\n"
    end_marker = "\t\t\tfmt.Fprintf(&handlers, \"\\toutput, err := handler.application.%s(request.Context(), wire)\\n\", method.Name)\n"
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    old = text[start:end]
    if 'for _, fieldName := range pathFields' not in old or 'binding.Body == "*"' not in old:
        raise SystemExit("unexpected REST adapter source block")
    replacement = '''\t\t\tpathFields, _ := simplePathFields(binding.Path)\n\t\t\tif binding.Body == "*" {\n\t\t\t\timports.add("io", "io")\n\t\t\t\thandlers.WriteString("\\tbody, err := io.ReadAll(request.Body)\\n\\tif err != nil { http.Error(writer, \\\"invalid request body\\\", http.StatusBadRequest); return }\\n\\tif len(body) > 0 { if err := protojson.Unmarshal(body, wire); err != nil { http.Error(writer, \\\"invalid request body\\\", http.StatusBadRequest); return } }\\n")\n\t\t\t} else {\n\t\t\t\tpathSet := make(map[string]struct{}, len(pathFields))\n\t\t\t\tfor _, value := range pathFields {\n\t\t\t\t\tpathSet[value] = struct{}{}\n\t\t\t\t}\n\t\t\t\tfor _, field := range requestMessage.Fields {\n\t\t\t\t\tif _, pathField := pathSet[field.Name]; pathField || field.Repeated || field.Map || field.Kind == "message" || field.Kind == "enum" {\n\t\t\t\t\t\tcontinue\n\t\t\t\t\t}\n\t\t\t\t\tqueryExpr := "request.URL.Query().Get(" + strconv.Quote(field.Name) + ")"\n\t\t\t\t\tfmt.Fprintf(&handlers, "\\tif raw := %s; raw != \\\"\\\" {\\n", queryExpr)\n\t\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "raw", false); err != nil {\n\t\t\t\t\t\treturn "", err\n\t\t\t\t\t}\n\t\t\t\t\thandlers.WriteString("\\t}\\n")\n\t\t\t\t}\n\t\t\t}\n\t\t\t// HTTP path bindings are authoritative over body/query values. Apply them last.\n\t\t\tfor _, fieldName := range pathFields {\n\t\t\t\tfield, ok := findMessageField(requestMessage, fieldName)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn "", fmt.Errorf("contract application codegen: %s path field %q not found in %s", method.FullName, fieldName, method.Request)\n\t\t\t\t}\n\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "request.PathValue("+strconv.Quote(fieldName)+")", true); err != nil {\n\t\t\t\t\treturn "", fmt.Errorf("contract application codegen: %s: %w", method.FullName, err)\n\t\t\t\t}\n\t\t\t}\n'''
    path.write_text(text[:start] + replacement + text[end:])


def patch_makefile() -> None:
    path = Path("Makefile")
    text = path.read_text()
    old = "dsl-check: toolchain-check architecture-check\n\t@cd pkg && $(GO) test -count=10 ./contract ./applicationgraph ./architecturepolicy\n"
    new = "dsl-check: toolchain-check architecture-check rpc-tools rpc-toolchain-check\n\t@cd pkg && YUNKA_REQUIRE_C84_RUNTIME=1 PROTOC=\"$(PROTOC)\" PROTOC_GEN_GO=\"$(PROTOC_GEN_GO)\" PROTOC_GEN_GO_GRPC=\"$(PROTOC_GEN_GO_GRPC)\" $(GO) test -count=1 ./contract -run '^TestC84GeneratedApplicationRuntime$$'\n\t@cd pkg && $(GO) test -count=10 ./contract ./applicationgraph ./architecturepolicy\n"
    if old not in text:
        raise SystemExit("unexpected dsl-check target")
    path.write_text(text.replace(old, new, 1))


if __name__ == "__main__":
    patch_application_codegen()
    patch_makefile()
