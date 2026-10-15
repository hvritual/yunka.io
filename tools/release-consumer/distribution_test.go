// This package is copied into a disposable independent Go module. It tests
// distributed public contracts, never producer workspace imports or real IAM.
package releaseconsumer

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	canonical "github.com/hvritual/yunka.io/framework/modules/outboxruntime"
	"github.com/hvritual/yunka.io/gateway/authz"
	"github.com/hvritual/yunka.io/infras/modules/outboxruntime"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

// EntryReader is a fixture-owned public constructor contract, independent of
// both framework internals and any consumer's business interfaces.
type EntryReader interface{ ID() string }

type readerModule struct{ events *[]string }

func (*readerModule) Name() string { return "entry-reader" }
func (*readerModule) ID() string   { return "distributed-reader" }
func (m *readerModule) Start(context.Context) error {
	*m.events = append(*m.events, "start")
	return nil
}
func (m *readerModule) Health(context.Context) error {
	*m.events = append(*m.events, "health")
	return nil
}
func (m *readerModule) Shutdown(context.Context) error {
	*m.events = append(*m.events, "shutdown")
	return nil
}

var readerKey = modulecatalog.MustCapabilityKey[EntryReader]("entry.reader", "example.com/releaseconsumer", "EntryReader")

func (m *readerModule) ExportCapabilities() []modulecatalog.CapabilityExport {
	return []modulecatalog.CapabilityExport{readerKey.Export(m)}
}

func readerDescriptor(events *[]string) modulecatalog.Descriptor {
	return modulecatalog.Descriptor{
		Name:     "entry-reader",
		Provides: []modulecatalog.CapabilityContract{readerKey.Contract()},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return &readerModule{events: events}, nil
		},
	}
}

func TestDistributedCapabilityUsesAppLifecycle(t *testing.T) {
	var events []string
	catalog := modulecatalog.New()
	catalog.MustRegister(readerDescriptor(&events))
	app, exports, err := core.NewAppWithCapabilities(core.AppOptions{Catalog: catalog})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Cleanup(func() { _ = app.Shutdown(ctx) })
	reader, err := modulecatalog.ResolveCapability(exports, readerKey)
	if err != nil || reader.ID() != "distributed-reader" {
		t.Fatalf("resolve: %v, %v", reader, err)
	}
	if err := app.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if health := app.Health(ctx); !health.Live || !health.Ready {
		t.Fatalf("health: %+v", health)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"start", "health", "shutdown"}) {
		t.Fatalf("lifecycle: %v", events)
	}
}

func TestDistributedDuplicateProviderFailsClosed(t *testing.T) {
	var events []string
	catalog := modulecatalog.New()
	a := readerDescriptor(&events)
	b := readerDescriptor(&events)
	b.Name = "other-reader"
	catalog.MustRegister(a)
	catalog.MustRegister(b)
	if _, err := catalog.Seal(); err == nil || !strings.Contains(err.Error(), "multiple providers") {
		t.Fatalf("duplicate: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("invalid catalog started modules: %v", events)
	}
}

func TestDistributedMissingExportFailsClosed(t *testing.T) {
	var events []string
	d := readerDescriptor(&events)
	d.Build = func(modulecatalog.BuildContext) (modulecatalog.Instance, error) { return missingExport{}, nil }
	catalog := modulecatalog.New()
	catalog.MustRegister(d)
	if _, _, err := core.NewAppWithCapabilities(core.AppOptions{Catalog: catalog}); err == nil || !strings.Contains(err.Error(), "did not export") {
		t.Fatalf("missing export: %v", err)
	}
}

type missingExport struct{}

func (missingExport) Name() string                                         { return "entry-reader" }
func (missingExport) ExportCapabilities() []modulecatalog.CapabilityExport { return nil }

func TestDistributedOutboxRemainsCanonicalFacade(t *testing.T) {
	want, got := canonical.GeneratedDescriptor(), outboxruntime.GeneratedDescriptor()
	if want.Name != got.Name || !reflect.DeepEqual(want.Requirements, got.Requirements) {
		t.Fatalf("facade changed: %+v %+v", want, got)
	}
	// These independent imports force all four locked modules into the consumer graph.
	_ = authz.Policy{}
	_ = operationplan.Set{}
}
