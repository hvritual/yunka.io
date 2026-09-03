package httpbinding

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterDispatchesFinalVariableCustomVerbs(t *testing.T) {
	mux := http.NewServeMux()
	seen := ""
	if err := Register(mux, http.MethodPost, "/v1/resources/{id}:revoke", func(writer http.ResponseWriter, request *http.Request) {
		seen = "revoke:" + request.PathValue("id")
		writer.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}
	if err := Register(mux, http.MethodPost, "/v1/resources/{id}:approve", func(writer http.ResponseWriter, request *http.Request) {
		seen = "approve:" + request.PathValue("id")
		writer.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/v1/resources/r-1:revoke", want: "revoke:r-1"},
		{path: "/v1/resources/r-2:approve", want: "approve:r-2"},
	} {
		seen = ""
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, test.path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("%s status=%d", test.path, recorder.Code)
		}
		if seen != test.want {
			t.Fatalf("%s seen=%q want=%q", test.path, seen, test.want)
		}
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/resources/r-3:unknown", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown custom verb status=%d", recorder.Code)
	}
}

func TestRegisterCustomVerbCoexistsWithPlainWildcard(t *testing.T) {
	mux := http.NewServeMux()
	seen := ""
	if err := Register(mux, http.MethodPost, "/v1/resources/{id}", func(writer http.ResponseWriter, request *http.Request) {
		seen = "plain:" + request.PathValue("id")
		writer.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}
	if err := Register(mux, http.MethodPost, "/v1/resources/{id}:revoke", func(writer http.ResponseWriter, request *http.Request) {
		seen = "revoke:" + request.PathValue("id")
		writer.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/resources/r-1:revoke", nil))
	if recorder.Code != http.StatusNoContent || seen != "revoke:r-1" {
		t.Fatalf("custom route status=%d seen=%q", recorder.Code, seen)
	}

	seen = ""
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/resources/r-1", nil))
	if recorder.Code != http.StatusNoContent || seen != "plain:r-1" {
		t.Fatalf("plain route status=%d seen=%q", recorder.Code, seen)
	}
}

func TestRegisterPreservesEarlierPathValuesWithLiteralCustomVerb(t *testing.T) {
	mux := http.NewServeMux()
	seen := ""
	if err := Register(mux, http.MethodPost, "/v1/parents/{parent}/actions:revoke", func(writer http.ResponseWriter, request *http.Request) {
		seen = request.PathValue("parent")
		writer.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/parents/p-1/actions:revoke", nil))
	if recorder.Code != http.StatusNoContent || seen != "p-1" {
		t.Fatalf("status=%d seen=%q", recorder.Code, seen)
	}
}

func TestRegisterRejectsDuplicateAndUnsupportedBindings(t *testing.T) {
	mux := http.NewServeMux()
	handler := func(http.ResponseWriter, *http.Request) {}
	if err := Register(mux, http.MethodPost, "/v1/resources/{id}:revoke", handler); err != nil {
		t.Fatal(err)
	}
	if err := Register(mux, http.MethodPost, "/v1/resources/{name}:revoke", handler); err == nil {
		t.Fatal("expected structurally duplicate custom verb binding to fail")
	}
	if err := Register(http.NewServeMux(), http.MethodGet, "/v1/{name=projects/*}", handler); err == nil {
		t.Fatal("expected complex path variable to fail closed")
	}
}

func TestRegisterConvertsServeMuxPanicToError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/resources/{id}", func(http.ResponseWriter, *http.Request) {})
	if err := Register(mux, http.MethodGet, "/v1/resources/{name}", func(http.ResponseWriter, *http.Request) {}); err == nil {
		t.Fatal("expected registration conflict to return an error")
	}
}
