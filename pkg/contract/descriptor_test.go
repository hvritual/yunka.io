package contract

import (
	"encoding/binary"
	"testing"
)

func TestParseGoogleHTTPOptionWithoutAnnotationDependency(t *testing.T) {
	rule := appendWireBytes(nil, 4, []byte("/v1/items/{id}")) // google.api.HttpRule.post
	rule = appendWireBytes(rule, 7, []byte("*"))
	options := appendWireBytes(nil, googleHTTPOptionField, rule)
	bindings, err := parseHTTPBindings(options)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].Method != "POST" || bindings[0].Path != "/v1/items/{id}" || bindings[0].Body != "*" {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
}

func appendWireBytes(dst []byte, field int, value []byte) []byte {
	var buffer [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buffer[:], uint64(field<<3|2))
	dst = append(dst, buffer[:n]...)
	n = binary.PutUvarint(buffer[:], uint64(len(value)))
	dst = append(dst, buffer[:n]...)
	return append(dst, value...)
}
