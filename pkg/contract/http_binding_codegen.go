package contract

import (
	"fmt"
	"strconv"
	"strings"
)

// writeHTTPBinding emits ordinary typed assignments shared by the canonical
// Executor adapter and retained compatibility projection. Runtime never parses
// a descriptor or consults a second field registry to discover this mapping.
func writeHTTPBinding(out *strings.Builder, imports *importSet, plan httpBindingPlan) {
	if plan.Body == "*" {
		imports.add("io", "io")
		out.WriteString("\tbody, err := io.ReadAll(request.Body)\n\tif err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return }\n\tif len(body) > 0 { if err := protojson.Unmarshal(body, wire); err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return } }\n")
	}
	if len(plan.Query) > 0 {
		imports.add("net/url", "url")
		out.WriteString("\tquery, err := url.ParseQuery(request.URL.RawQuery)\n\tif err != nil { http.Error(writer, \"invalid query encoding\", http.StatusBadRequest); return }\n")
		for _, field := range plan.Query {
			name := httpJSONName(field)
			out.WriteString("\t{\n")
			fmt.Fprintf(out, "\tvalues, present := query[%q]\n", name)
			if name != field.Name {
				fmt.Fprintf(out, "\tif alias, exists := query[%q]; exists {\n", field.Name)
				out.WriteString("\tif present { http.Error(writer, \"ambiguous request parameter\", http.StatusBadRequest); return }; values, present = alias, true\n\t}\n")
			}
			out.WriteString("\tif present {\n")
			if field.Repeated {
				out.WriteString("\tfor _, raw := range values {\n")
			} else {
				out.WriteString("\tif len(values) != 1 { http.Error(writer, \"duplicate request parameter\", http.StatusBadRequest); return }\n\traw := values[0]\n")
			}
			writeHTTPScalarValue(out, imports, field, "raw")
			if field.Repeated {
				out.WriteString("\t}\n")
			}
			out.WriteString("\t}\n\t}\n")
		}
	}
	// URL path ownership is applied last: neither the JSON body nor a query
	// alias can substitute a different resource identity for the matched route.
	for _, path := range plan.Path {
		out.WriteString("\t{\n")
		writeHTTPScalarValue(out, imports, path.Field, "request.PathValue("+strconv.Quote(path.Name)+")")
		out.WriteString("\t}\n")
	}
}

func writeHTTPScalarValue(out *strings.Builder, imports *importSet, field Field, raw string) {
	if field.Type == "string" && !field.Repeated && !field.Optional {
		fmt.Fprintf(out, "\twire.%s = %s\n", protoGoFieldName(field.Name), raw)
		return
	}
	badRequest := "if err != nil { http.Error(writer, \"invalid request parameter\", http.StatusBadRequest); return }\n"
	switch field.Type {
	case "string":
		fmt.Fprintf(out, "\tvalue := %s\n", raw)
	case "bytes":
		imports.add("encoding/base64", "base64")
		fmt.Fprintf(out, "\tvalue, err := base64.StdEncoding.DecodeString(%s)\n", raw)
		// ProtoJSON admits padded/unpadded standard and URL-safe base64.
		for _, encoding := range []string{"RawStdEncoding", "URLEncoding", "RawURLEncoding"} {
			fmt.Fprintf(out, "\tif err != nil { value, err = base64.%s.DecodeString(%s) }\n", encoding, raw)
		}
		out.WriteString(badRequest)
	case "bool":
		imports.add("strconv", "strconv")
		fmt.Fprintf(out, "\tvalue, err := strconv.ParseBool(%s)\n", raw)
		out.WriteString(badRequest)
	default:
		imports.add("strconv", "strconv")
		bits, cast, parser := 64, "int64", "ParseInt"
		switch field.Type {
		case "uint32", "uint64", "fixed32", "fixed64":
			cast, parser = "uint64", "ParseUint"
		case "float", "double":
			cast, parser = "float64", "ParseFloat"
		}
		if strings.Contains(field.Type, "32") || field.Type == "float" {
			bits = 32
			cast = strings.Replace(cast, "64", "32", 1)
		}
		if parser == "ParseFloat" {
			fmt.Fprintf(out, "\tparsed, err := strconv.%s(%s, %d)\n", parser, raw, bits)
		} else {
			fmt.Fprintf(out, "\tparsed, err := strconv.%s(%s, 10, %d)\n", parser, raw, bits)
		}
		out.WriteString(badRequest)
		fmt.Fprintf(out, "\tvalue := %s(parsed)\n", cast)
	}
	target := "wire." + protoGoFieldName(field.Name)
	if field.Repeated {
		fmt.Fprintf(out, "\t%s = append(%s, value)\n", target, target)
	} else if field.Optional && field.Type != "bytes" {
		fmt.Fprintf(out, "\t%s = &value\n", target)
	} else {
		fmt.Fprintf(out, "\t%s = value\n", target)
	}
}
