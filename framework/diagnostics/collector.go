package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/framework/core"
)

const SchemaVersion = 1

type Source interface {
	Name() string
	Snapshot(context.Context) (any, error)
}

type SourceFunc struct {
	SourceName string
	Func       func(context.Context) (any, error)
}

func (source SourceFunc) Name() string { return source.SourceName }

func (source SourceFunc) Snapshot(ctx context.Context) (any, error) {
	if source.Func == nil {
		return nil, errors.New("diagnostics: source function is nil")
	}
	return source.Func(ctx)
}

type ComponentStatus string

const (
	ComponentOK    ComponentStatus = "ok"
	ComponentError ComponentStatus = "error"
)

type Component struct {
	Name   string          `json:"name"`
	Status ComponentStatus `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type Report struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Core          core.DiagnosticsReport `json:"core"`
	Components    []Component            `json:"components,omitempty"`
}

type Collector struct {
	app     *core.App
	sources []Source
}

func New(app *core.App, sources ...Source) (*Collector, error) {
	if app == nil {
		return nil, errors.New("diagnostics: app is required")
	}
	filtered := make([]Source, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		name := strings.TrimSpace(source.Name())
		if name == "" {
			return nil, errors.New("diagnostics: source name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("diagnostics: duplicate source %q", name)
		}
		seen[name] = struct{}{}
		filtered = append(filtered, source)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return strings.TrimSpace(filtered[i].Name()) < strings.TrimSpace(filtered[j].Name())
	})
	return &Collector{app: app, sources: filtered}, nil
}

func (collector *Collector) Snapshot(ctx context.Context) Report {
	if ctx == nil {
		ctx = context.Background()
	}
	report := Report{SchemaVersion: SchemaVersion, Core: collector.app.Diagnostics(ctx)}
	for _, source := range collector.sources {
		component := Component{Name: strings.TrimSpace(source.Name()), Status: ComponentOK}
		value, err := safeSourceSnapshot(ctx, source)
		if err == nil {
			component.Data, err = json.Marshal(value)
		}
		if err != nil {
			component.Status = ComponentError
			component.Error = err.Error()
			component.Data = nil
		}
		report.Components = append(report.Components, component)
	}
	return report
}

func safeSourceSnapshot(ctx context.Context, source Source) (value any, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("diagnostics source %s panicked: %v", source.Name(), recovered)
		}
	}()
	return source.Snapshot(ctx)
}
