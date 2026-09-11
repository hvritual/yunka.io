package contract

import (
	"strings"
	"testing"

	contractdslv1 "github.com/hvritual/yunka.io/pkg/contractdsl/v1"
)

func TestWebSessionAuthenticationProvenance(t *testing.T) {
	if got := authenticationName(uint64(contractdslv1.Authentication_AUTHENTICATION_WEB_SESSION)); got != "web-session" {
		t.Fatalf("web session authentication name=%q want web-session", got)
	}
	if got := authenticationName(999); got != "" {
		t.Fatalf("unknown authentication must fail closed, got %q", got)
	}
}

func TestGeneratedPolicyPreservesAPIKeyAndWebSession(t *testing.T) {
	service := Service{
		Name:        "ExampleApplication",
		FullName:    "example.v1.ExampleApplication",
		Domain:      "example",
		Application: &ApplicationDeclaration{Name: "example"},
		Methods: []Method{{
			Name:     "Read",
			FullName: "example.v1.ExampleApplication.Read",
			Operation: &OperationDeclaration{
				ID:             "example.read",
				Authentication: []string{"api-key", "web-session"},
			},
		}},
	}
	source, err := renderOperationPolicy(service, namingForService(service, false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, `Authentication: []string{"api-key","web-session",}`) {
		t.Fatalf("generated policy did not preserve both authentication methods:\n%s", source)
	}
}
