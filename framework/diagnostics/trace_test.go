package diagnostics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTraceAnalyzerAggregatesSortsAndIsolatesSourceFailures(t *testing.T) {
	base := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	analyzer, err := NewTraceAnalyzer(
		TraceSourceFunc{SourceName: "logs", Func: func(context.Context, string) ([]TraceEvidence, error) {
			return []TraceEvidence{{Kind: TraceEvidenceLog, Service: "gateway", Name: "request.done", Timestamp: base.Add(time.Second)}}, nil
		}},
		TraceSourceFunc{SourceName: "spans", Func: func(context.Context, string) ([]TraceEvidence, error) {
			return []TraceEvidence{{Kind: TraceEvidenceSpan, Service: "gateway", Name: "POST /device", Timestamp: base}}, nil
		}},
		TraceSourceFunc{SourceName: "broken", Func: func(context.Context, string) ([]TraceEvidence, error) {
			return nil, errors.New("backend unavailable")
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := analyzer.Analyze(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Evidence) != 2 || report.Evidence[0].Kind != TraceEvidenceSpan || report.Evidence[1].Kind != TraceEvidenceLog {
		t.Fatalf("evidence=%+v", report.Evidence)
	}
	for _, evidence := range report.Evidence {
		if evidence.TraceID != "trace-1" {
			t.Fatalf("trace id=%q", evidence.TraceID)
		}
	}
	if len(report.Sources) != 3 || report.Sources[0].Name != "broken" || report.Sources[0].Status != ComponentError {
		t.Fatalf("sources=%+v", report.Sources)
	}
}

func TestTraceAnalyzerRejectsCrossTraceEvidenceFromSource(t *testing.T) {
	analyzer, err := NewTraceAnalyzer(TraceSourceFunc{SourceName: "bad", Func: func(context.Context, string) ([]TraceEvidence, error) {
		return []TraceEvidence{{Kind: TraceEvidenceSpan, TraceID: "other-trace"}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := analyzer.Analyze(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Evidence) != 0 || len(report.Sources) != 1 || report.Sources[0].Status != ComponentError {
		t.Fatalf("report=%+v", report)
	}
}

func TestTraceHTTPHandlerReturnsOneTraceReport(t *testing.T) {
	analyzer, err := NewTraceAnalyzer(TraceSourceFunc{SourceName: "otel", Func: func(_ context.Context, traceID string) ([]TraceEvidence, error) {
		return []TraceEvidence{{Kind: TraceEvidenceOperation, TraceID: traceID, OperationID: "device.update"}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewTraceHTTPHandler(analyzer, HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/_yunka/trace?trace_id=trace-1", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if body := response.Body.String(); body == "" || !containsAll(body, `"traceId":"trace-1"`, `"operationId":"device.update"`) {
		t.Fatalf("body=%s", body)
	}
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if len(needle) > 0 && !contains(value, needle) {
			return false
		}
	}
	return true
}

func contains(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return needle == ""
}
