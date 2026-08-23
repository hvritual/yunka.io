package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC7GatewayRoleVerticalSliceDoesNotRegressToLegacyRuntime(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	paths := []string{
		"gateway/dispatcher/intercept/role/composition.go",
		"gateway/dispatcher/intercept/role/role_intercept.go",
		"gateway/rpc/transport/grpc/typed_bridge_integration_test.go",
	}
	forbidden := []string{
		"yunka.io/framework/core/request",
		"NewModuleGatewayProvider",
		"GetService(",
		"PutService(",
		"SetRuntime(",
		"sync.Pool",
		"reflect.Value",
	}

	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		text := string(content)
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				t.Errorf("%s reintroduced legacy C7 runtime token %q", relative, token)
			}
		}
	}

	roleContent, err := os.ReadFile(filepath.Join(repoRoot, "gateway", "dispatcher", "intercept", "role", "role_intercept.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(roleContent), "requestscope.Execute") {
		t.Error("Gateway Role handler must keep requestscope-owned execution")
	}

	compositionContent, err := os.ReadFile(filepath.Join(repoRoot, "gateway", "dispatcher", "intercept", "role", "composition.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compositionContent), "build.Databases().GORM") {
		t.Error("Gateway Role composition must resolve its database through restricted module BuildContext capabilities")
	}

	bridgeTestContent, err := os.ReadFile(filepath.Join(repoRoot, "gateway", "rpc", "transport", "grpc", "typed_bridge_integration_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bridgeTestContent), "rpcbridge.Static") || !strings.Contains(string(bridgeTestContent), "requestscope.ExecuteValue") {
		t.Error("typed Gateway RPC integration must prove stateless service ownership with handler-owned Request Scope")
	}
}
