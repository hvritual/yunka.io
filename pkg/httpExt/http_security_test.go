package httpExt

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectsUntrustedTLSCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":"0"}`))
	}))
	defer server.Close()
	if _, err := Post(server.URL, nil, nil); err == nil {
		t.Fatal("request with an untrusted certificate succeeded")
	}
}

func TestRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = bytes.NewBufferString(strings.Repeat("x", int(maxResponseBytes)+1)).WriteTo(w)
	}))
	defer server.Close()
	if _, err := Post(server.URL, nil, nil); err == nil {
		t.Fatal("oversized response was accepted")
	}
}
