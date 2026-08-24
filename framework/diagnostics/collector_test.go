package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunka.io/framework/core"
)

func TestCollectorSortsAndIsolatesSourceFailure(t *testing.T) {
	app, appErr := core.NewApp(core.AppOptions{})
	if appErr != nil {
		t.Fatal(appErr)
	}
	collector, err := New(app,
		SourceFunc{SourceName: "z", Func: func(context.Context) (any, error) { panic("boom") }},
		SourceFunc{SourceName: "a", Func: func(context.Context) (any, error) { return map[string]string{"ok": "yes"}, nil }},
		SourceFunc{SourceName: "m", Func: func(context.Context) (any, error) { return nil, errors.New("unavailable") }},
	)
	if err != nil {
		t.Fatal(err)
	}
	report := collector.Snapshot(context.Background())
	if len(report.Components) != 3 || report.Components[0].Name != "a" || report.Components[1].Name != "m" || report.Components[2].Name != "z" {
		t.Fatalf("unexpected components: %#v", report.Components)
	}
	if report.Components[0].Status != ComponentOK || report.Components[1].Status != ComponentError || report.Components[2].Status != ComponentError {
		t.Fatalf("unexpected statuses: %#v", report.Components)
	}
}

func TestHTTPHandlerSecurityDefaults(t *testing.T) {
	app, appErr := core.NewApp(core.AppOptions{})
	if appErr != nil {
		t.Fatal(appErr)
	}
	collector, err := New(app)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewHTTPHandler(collector, HTTPOptions{AllowRemote: true}); err == nil {
		t.Fatal("remote handler without token must fail")
	}
	handler, err := NewHTTPHandler(collector, HTTPOptions{Token: "01234567890123456789012345678901"})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://localhost/_yunka/diagnostics", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/_yunka/diagnostics", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Authorization", "Bearer 01234567890123456789012345678901")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var report Report
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersion || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected response: %#v headers=%v", report, response.Header())
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/_yunka/diagnostics", nil)
	request.RemoteAddr = "198.51.100.10:12345"
	request.Header.Set("Authorization", "Bearer 01234567890123456789012345678901")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "loopback") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
