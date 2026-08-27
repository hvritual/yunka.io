#!/usr/bin/env python3
from pathlib import Path

path = Path("pkg/contract/application_codegen.go")
text = path.read_text()

old = 'b.WriteString(GeneratedApplicationMarker + "\\n\\npackage rest\\n\\nimport (\\n\\t\\\"errors\\\"\\n\\t\\\"net/http\\\"\\n\\t\\\"strconv\\\"\\n")'
new = 'b.WriteString(GeneratedApplicationMarker + "\\n\\npackage rest\\n\\nimport (\\n\\t\\\"errors\\\"\\n\\t\\\"net/http\\\"\\n")'
if old not in text:
    raise SystemExit("REST import block not found")
text = text.replace(old, new, 1)

query_old = '''\t\t\t\t\tqueryExpr := "request.URL.Query().Get(" + strconv.Quote(field.Name) + ")"\n\t\t\t\t\tfmt.Fprintf(&handlers, "\\tif raw := %s; raw != \\\"\\\" {\\n", queryExpr)\n\t\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "raw", false); err != nil {'''
query_new = '''\t\t\t\t\tqueryExpr := "request.URL.Query().Get(" + strconv.Quote(field.Name) + ")"\n\t\t\t\t\tfmt.Fprintf(&handlers, "\\tif raw := %s; raw != \\\"\\\" {\\n", queryExpr)\n\t\t\t\t\tif scalarAssignmentNeedsStrconv(field) {\n\t\t\t\t\t\timports.add("strconv", "strconv")\n\t\t\t\t\t}\n\t\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "raw", false); err != nil {'''
if query_old not in text:
    raise SystemExit("query assignment block not found")
text = text.replace(query_old, query_new, 1)

path_old = '''\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "request.PathValue("+strconv.Quote(fieldName)+")", true); err != nil {'''
path_new = '''\t\t\t\tif scalarAssignmentNeedsStrconv(field) {\n\t\t\t\t\timports.add("strconv", "strconv")\n\t\t\t\t}\n\t\t\t\tif err := writeScalarAssignment(&handlers, "wire", field, "request.PathValue("+strconv.Quote(fieldName)+")", true); err != nil {'''
if path_old not in text:
    raise SystemExit("path assignment block not found")
text = text.replace(path_old, path_new, 1)

helper_anchor = '''func writeScalarAssignment(builder *strings.Builder, receiver string, field Field, rawExpression string, path bool) error {'''
helper = '''func scalarAssignmentNeedsStrconv(field Field) bool {\n\tswitch field.Type {\n\tcase "bool", "int32", "int64", "sint32", "sint64", "sfixed32", "sfixed64", "uint32", "uint64", "fixed32", "fixed64", "float", "double":\n\t\treturn true\n\tdefault:\n\t\treturn false\n\t}\n}\n\n'''
if helper_anchor not in text:
    raise SystemExit("writeScalarAssignment anchor not found")
text = text.replace(helper_anchor, helper + helper_anchor, 1)

path.write_text(text)
