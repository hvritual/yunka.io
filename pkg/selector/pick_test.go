package selector

import (
	"errors"
	"testing"

	"yunka.io/pkg/registry"
)

func TestServiceFromFullMethod(t *testing.T) {
	service, err := ServiceFromFullMethod("/io.yunka.Device/Get")
	if err != nil || service != "io.yunka.Device" {
		t.Fatalf("service=%q err=%v", service, err)
	}
	for _, invalid := range []string{"invalid", "/svc", "/svc/"} {
		if _, err := ServiceFromFullMethod(invalid); !errors.Is(err, ErrInvalidMethod) {
			t.Fatalf("method=%q err=%v", invalid, err)
		}
	}
}

func TestPickFallsBackToSelectMark(t *testing.T) {
	legacy := &legacySelector{node: &registry.Node{Id: "a", Address: "127.0.0.1:9000"}}
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

func (selector *legacySelector) Init(...Option) error { return nil }
func (selector *legacySelector) Options() Options     { return Options{} }
func (selector *legacySelector) Select(string, ...SelectOption) (Next, error) {
	return func() (*registry.Node, error) { return selector.node, nil }, nil
}
func (selector *legacySelector) Mark(string, *registry.Node, error) { selector.marked++ }
func (selector *legacySelector) Reset(string)                       {}
func (selector *legacySelector) Close() error                       { return nil }
func (selector *legacySelector) String() string                     { return "legacy" }
