package kernel

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

func TestBootstrapSequencesBuildRegisterThenExistingAppStart(t *testing.T) {
	var events []string
	component := core.RuntimeComponent{
		Name: "http",
		StartFunc: func(context.Context) error {
			events = append(events, "start:http")
			return nil
		},
		ShutdownFunc: func(context.Context) error {
			events = append(events, "shutdown:http")
			return nil
		},
	}
	result, err := Bootstrap(context.Background(), BootstrapOptions[string]{
		Kernel: Options{Catalog: modulecatalog.New(), RuntimeComponents: []core.RuntimeComponent{component}},
		Build: func() (string, error) {
			events = append(events, "build")
			return "applications", nil
		},
		Register: func(applications string) error {
			events = append(events, "register:"+applications)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.App == nil || result.App.State() != core.AppStateReady || result.Applications != "applications" {
		t.Fatalf("unexpected bootstrap result: %#v", result)
	}
	if want := []string{"build", "register:applications", "start:http"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%#v want=%#v", events, want)
	}
	if err := result.App.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"build", "register:applications", "start:http", "shutdown:http"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%#v want=%#v", events, want)
	}
}

func TestBootstrapRegistrationFailureCleansConstructedAppWithoutStarting(t *testing.T) {
	var events []string
	component := core.RuntimeComponent{
		Name:      "grpc",
		StartFunc: func(context.Context) error { events = append(events, "start:grpc"); return nil },
		ShutdownFunc: func(context.Context) error {
			events = append(events, "shutdown:grpc")
			return nil
		},
	}
	_, err := Bootstrap(context.Background(), BootstrapOptions[string]{
		Kernel: Options{Catalog: modulecatalog.New(), RuntimeComponents: []core.RuntimeComponent{component}},
		Build: func() (string, error) {
			events = append(events, "build")
			return "applications", nil
		},
		Register: func(string) error {
			events = append(events, "register")
			return errors.New("bad transport")
		},
	})
	if err == nil {
		t.Fatal("Bootstrap() error=nil, want registration failure")
	}
	if want := []string{"build", "register", "shutdown:grpc"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%#v want=%#v", events, want)
	}
}

func TestBootstrapRequiresTypedStructuralCallbacks(t *testing.T) {
	if _, err := Bootstrap[string](context.Background(), BootstrapOptions[string]{}); err == nil {
		t.Fatal("missing build callback accepted")
	}
	if _, err := Bootstrap[string](context.Background(), BootstrapOptions[string]{Build: func() (string, error) { return "", nil }}); err == nil {
		t.Fatal("missing register callback accepted")
	}
}
