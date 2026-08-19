package selector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"yunka.io/pkg/registry"
)

type fakeRPCClient struct {
	node        string
	err         error
	invokeCalls int
}

func (client *fakeRPCClient) Invoke(context.Context, string, proto.Message, proto.Message, ...interface{}) error {
	client.invokeCalls++
	return client.err
}
func (client *fakeRPCClient) InvokeNode(_ context.Context, node, _ string, _ proto.Message, _ proto.Message, _ ...interface{}) error {
	client.node = node
	return client.err
}

func TestRPCWrapperClosesFeedbackLoop(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 8, EWMAAlpha: 1})
	transportErr := errors.New("transport")
	transport := &fakeRPCClient{err: transportErr}
	client := WrapRPCClient(transport, selector)
	if err := client.Invoke(context.Background(), "/svc/Get", nil, nil); !errors.Is(err, transportErr) {
		t.Fatalf("err=%v", err)
	}
	if transport.node == "" {
		t.Fatal("InvokeNode was not used")
	}
	var matched NodeSnapshot
	for _, snapshot := range selector.Snapshot("svc") {
		if snapshot.Address == transport.node {
			matched = snapshot
			break
		}
	}
	if matched.LastOutcome != OutcomeFailure {
		t.Fatalf("outcome=%v", matched.LastOutcome)
	}
	if matched.InFlight != 0 {
		t.Fatalf("inflight=%d", matched.InFlight)
	}
	if matched.EWMA <= 0 || matched.EWMA > time.Second {
		t.Fatalf("ewma=%v", matched.EWMA)
	}
}

func TestRPCWrapperBypass(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 9})
	transport := &fakeRPCClient{}
	client := WrapRPCClient(transport, selector, WithRPCBypass(func(method string) bool { return method == "/svc/Local" }))
	if err := client.Invoke(context.Background(), "/svc/Local", nil, nil); err != nil {
		t.Fatal(err)
	}
	if transport.invokeCalls != 1 {
		t.Fatalf("invoke calls=%d", transport.invokeCalls)
	}
	if transport.node != "" {
		t.Fatalf("unexpected remote node %q", transport.node)
	}
}

func TestServiceFromRPCMethod(t *testing.T) {
	service, err := serviceFromRPCMethod("/io.yunka.Device/Get")
	if err != nil || service != "io.yunka.Device" {
		t.Fatalf("service=%q err=%v", service, err)
	}
	if _, err := serviceFromRPCMethod("invalid"); !errors.Is(err, ErrInvalidMethod) {
		t.Fatalf("err=%v", err)
	}
}

func TestLegacyPickFallsBackToSelectMark(t *testing.T) {
	legacy := &legacySelector{node: &registry.Node{Id: "a", Address: "1"}}
	selection, err := Pick(legacy, "svc")
	if err != nil {
		t.Fatal(err)
	}
	selection.Done(errors.New("bad"))
	if legacy.marked != 1 {
		t.Fatalf("marked=%d", legacy.marked)
	}
}

type legacySelector struct {
	node   *registry.Node
	marked int
}

func (l *legacySelector) Init(...Option) error { return nil }
func (l *legacySelector) Options() Options     { return Options{} }
func (l *legacySelector) Select(string, ...SelectOption) (Next, error) {
	return func() (*registry.Node, error) { return l.node, nil }, nil
}
func (l *legacySelector) Mark(string, *registry.Node, error) { l.marked++ }
func (l *legacySelector) Reset(string)                       {}
func (l *legacySelector) Close() error                       { return nil }
func (l *legacySelector) String() string                     { return "legacy" }
