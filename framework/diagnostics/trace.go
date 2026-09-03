package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const TraceSchemaVersion = 1

type TraceEvidenceKind string

const (
	TraceEvidenceSpan      TraceEvidenceKind = "span"
	TraceEvidenceLog       TraceEvidenceKind = "log"
	TraceEvidenceOperation TraceEvidenceKind = "operation"
	TraceEvidenceEvent     TraceEvidenceKind = "event"
)

type TraceEvidence struct {
	Kind          TraceEvidenceKind `json:"kind"`
	Source        string            `json:"source,omitempty"`
	Service       string            `json:"service,omitempty"`
	Name          string            `json:"name,omitempty"`
	TraceID       string            `json:"traceId"`
	SpanID        string            `json:"spanId,omitempty"`
	ParentSpanID  string            `json:"parentSpanId,omitempty"`
	OperationID   string            `json:"operationId,omitempty"`
	EventID       string            `json:"eventId,omitempty"`
	CorrelationID string            `json:"correlationId,omitempty"`
	CausationID   string            `json:"causationId,omitempty"`
	Status        string            `json:"status,omitempty"`
	Timestamp     time.Time         `json:"timestamp,omitempty"`
	DurationMS    float64           `json:"durationMs,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type TraceSource interface {
	Name() string
	LookupTrace(context.Context, string) ([]TraceEvidence, error)
}

type TraceSourceFunc struct {
	SourceName string
	Func       func(context.Context, string) ([]TraceEvidence, error)
}

func (source TraceSourceFunc) Name() string { return source.SourceName }
func (source TraceSourceFunc) LookupTrace(ctx context.Context, traceID string) ([]TraceEvidence, error) {
	if source.Func == nil {
		return nil, errors.New("diagnostics: trace source function is nil")
	}
	return source.Func(ctx, traceID)
}

type TraceSourceStatus struct {
	Name   string          `json:"name"`
	Status ComponentStatus `json:"status"`
	Count  int             `json:"count"`
	Error  string          `json:"error,omitempty"`
}

type TraceReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	TraceID       string              `json:"traceId"`
	Evidence      []TraceEvidence     `json:"evidence,omitempty"`
	Sources       []TraceSourceStatus `json:"sources,omitempty"`
}

type TraceAnalyzer struct {
	sources []TraceSource
}

func NewTraceAnalyzer(sources ...TraceSource) (*TraceAnalyzer, error) {
	filtered := make([]TraceSource, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		name := strings.TrimSpace(source.Name())
		if name == "" {
			return nil, errors.New("diagnostics: trace source name is required")
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("diagnostics: duplicate trace source %q", name)
		}
		seen[name] = struct{}{}
		filtered = append(filtered, source)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name() < filtered[j].Name() })
	return &TraceAnalyzer{sources: filtered}, nil
}

// Analyze performs one read-only aggregation for a canonical trace identifier.
// A source failure is isolated into Sources so healthy telemetry backends can
// still return partial evidence; malformed cross-trace evidence from a source
// is discarded rather than contaminating the requested execution chain.
func (analyzer *TraceAnalyzer) Analyze(ctx context.Context, traceID string) (TraceReport, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" || len(traceID) > 128 {
		return TraceReport{}, errors.New("diagnostics: trace id is required and must be at most 128 bytes")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return TraceReport{}, err
	}
	report := TraceReport{SchemaVersion: TraceSchemaVersion, TraceID: traceID}
	if analyzer == nil {
		return report, errors.New("diagnostics: trace analyzer is nil")
	}
	for _, source := range analyzer.sources {
		status := TraceSourceStatus{Name: strings.TrimSpace(source.Name()), Status: ComponentOK}
		records, err := safeTraceLookup(ctx, source, traceID)
		prepared := make([]TraceEvidence, 0, len(records))
		if err == nil {
			for _, record := range records {
				record = cloneTraceEvidence(record)
				if record.TraceID == "" {
					record.TraceID = traceID
				}
				if record.TraceID != traceID {
					err = fmt.Errorf("trace source %s returned evidence for trace %s", status.Name, record.TraceID)
					break
				}
				if record.Source == "" {
					record.Source = status.Name
				}
				prepared = append(prepared, record)
			}
		}
		if err != nil {
			status.Status = ComponentError
			status.Error = err.Error()
		} else {
			status.Count = len(prepared)
			report.Evidence = append(report.Evidence, prepared...)
		}
		report.Sources = append(report.Sources, status)
	}
	sortTraceEvidence(report.Evidence)
	return report, nil
}

func safeTraceLookup(ctx context.Context, source TraceSource, traceID string) (records []TraceEvidence, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("diagnostics trace source %s panicked: %v", source.Name(), recovered)
		}
	}()
	return source.LookupTrace(ctx, traceID)
}

func cloneTraceEvidence(record TraceEvidence) TraceEvidence {
	clone := record
	if record.Attributes != nil {
		clone.Attributes = make(map[string]string, len(record.Attributes))
		for key, value := range record.Attributes {
			clone.Attributes[key] = value
		}
	}
	return clone
}

func sortTraceEvidence(records []TraceEvidence) {
	sort.SliceStable(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if !left.Timestamp.Equal(right.Timestamp) {
			if left.Timestamp.IsZero() {
				return false
			}
			if right.Timestamp.IsZero() {
				return true
			}
			return left.Timestamp.Before(right.Timestamp)
		}
		if left.Service != right.Service {
			return left.Service < right.Service
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.SpanID != right.SpanID {
			return left.SpanID < right.SpanID
		}
		return left.EventID < right.EventID
	})
}
