package contract

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

func edgeOwnedCapabilityFixture() Manifest {
	return Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{
			Name:      "tenant.proto",
			Package:   "tenant.v1",
			GoPackage: "example.com/biz/contracts/tenant/v1;tenantv1",
			Domain:    &DomainDeclaration{Name: "tenant", Version: "v1"},
		}},
		Messages: []Message{
			{Name: "AssignRoleRequest", FullName: "tenant.v1.AssignRoleRequest"},
			{Name: "AssignRoleResponse", FullName: "tenant.v1.AssignRoleResponse"},
			{Name: "RevokeRoleRequest", FullName: "tenant.v1.RevokeRoleRequest"},
			{Name: "RevokeRoleResponse", FullName: "tenant.v1.RevokeRoleResponse"},
			{Name: "ActivateTenantRequest", FullName: "tenant.v1.ActivateTenantRequest"},
			{Name: "ActivateTenantResponse", FullName: "tenant.v1.ActivateTenantResponse"},
			{Name: "RemoveMemberRequest", FullName: "tenant.v1.RemoveMemberRequest"},
			{Name: "RemoveMemberResponse", FullName: "tenant.v1.RemoveMemberResponse"},
		},
		Services: []Service{
			{
				Name: "TenantRolePermissionApplication", FullName: "tenant.v1.TenantRolePermissionApplication", Domain: "tenant",
				Application: &ApplicationDeclaration{Name: "role_permission"},
				Methods: []Method{
					{Name: "AssignRole", FullName: "tenant.v1.TenantRolePermissionApplication.AssignRole", Request: "tenant.v1.AssignRoleRequest", Response: "tenant.v1.AssignRoleResponse", Operation: &OperationDeclaration{ID: "tenant.role.assign", UseCase: "assign_role", Permissions: []string{"tenant.role.assign"}, PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}},
					{Name: "RevokeRole", FullName: "tenant.v1.TenantRolePermissionApplication.RevokeRole", Request: "tenant.v1.RevokeRoleRequest", Response: "tenant.v1.RevokeRoleResponse", Operation: &OperationDeclaration{ID: "tenant.role.revoke", UseCase: "revoke_role", Permissions: []string{"tenant.role.revoke"}, PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}},
				},
			},
			{
				Name: "TenantLifecycleApplication", FullName: "tenant.v1.TenantLifecycleApplication", Domain: "tenant",
				Application: &ApplicationDeclaration{Name: "tenant_lifecycle", Requires: []string{"tenant/role_permission"}},
				Methods: []Method{{Name: "ActivateTenant", FullName: "tenant.v1.TenantLifecycleApplication.ActivateTenant", Request: "tenant.v1.ActivateTenantRequest", Response: "tenant.v1.ActivateTenantResponse", Operation: &OperationDeclaration{ID: "tenant.lifecycle.activate", UseCase: "activate_tenant", Permissions: []string{"tenant.lifecycle.activate", "tenant.role.assign"}, PermissionMode: "all", RequiresOperations: []string{"tenant.role.assign"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}},
			},
			{
				Name: "TenantMemberLifecycleApplication", FullName: "tenant.v1.TenantMemberLifecycleApplication", Domain: "tenant",
				Application: &ApplicationDeclaration{Name: "tenant_member_lifecycle", Requires: []string{"tenant/role_permission"}},
				Methods: []Method{{Name: "RemoveMember", FullName: "tenant.v1.TenantMemberLifecycleApplication.RemoveMember", Request: "tenant.v1.RemoveMemberRequest", Response: "tenant.v1.RemoveMemberResponse", Operation: &OperationDeclaration{ID: "tenant.member.remove", UseCase: "remove_member", Permissions: []string{"tenant.member.remove", "tenant.role.revoke"}, PermissionMode: "all", RequiresOperations: []string{"tenant.role.revoke"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}},
			},
		},
	}
}

func TestRenderC9CapabilityPortsAreOwnedBySourceEdgeAndOperationSubset(t *testing.T) {
	manifest := edgeOwnedCapabilityFixture()
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}

	lifecycle := byPath["tenant/application/zz_yunka_tenant_lifecycle_capability_ports_gen.go"]
	member := byPath["tenant/application/zz_yunka_tenant_member_lifecycle_capability_ports_gen.go"]
	if lifecycle == "" || member == "" {
		t.Fatalf("expected both source-owned capability files, got paths=%v", generatedApplicationPaths(files))
	}

	if !strings.Contains(lifecycle, "type TenantLifecycleToTenantRolePermissionChildCapability interface") ||
		!strings.Contains(lifecycle, "func NewTenantLifecycleToTenantRolePermissionChildCapability(") ||
		!strings.Contains(lifecycle, "AssignRole(context.Context") {
		t.Fatalf("tenant lifecycle capability is not source-edge owned:\n%s", lifecycle)
	}
	if strings.Contains(lifecycle, "RevokeRole(context.Context") {
		t.Fatalf("tenant lifecycle capability leaked undeclared target operation RevokeRole:\n%s", lifecycle)
	}

	if !strings.Contains(member, "type TenantMemberLifecycleToTenantRolePermissionChildCapability interface") ||
		!strings.Contains(member, "func NewTenantMemberLifecycleToTenantRolePermissionChildCapability(") ||
		!strings.Contains(member, "RevokeRole(context.Context") {
		t.Fatalf("tenant member lifecycle capability is not source-edge owned:\n%s", member)
	}
	if strings.Contains(member, "AssignRole(context.Context") {
		t.Fatalf("tenant member lifecycle capability leaked undeclared target operation AssignRole:\n%s", member)
	}

	if strings.Contains(lifecycle, "type TenantRolePermissionChildCapability interface") || strings.Contains(member, "type TenantRolePermissionChildCapability interface") {
		t.Fatalf("target-owned child capability symbol still exists and can collide across source applications:\nlifecycle=%s\nmember=%s", lifecycle, member)
	}
}

func TestRenderAssemblyCodeUsesSourceEdgeCapabilityIdentity(t *testing.T) {
	manifest := edgeOwnedCapabilityFixture()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := CompileAssemblyPlan(manifest, operations, []assemblyplan.ModuleInput{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	source := string(files[0].Content)
	for _, required := range []string{
		"TenantRolePermission tenantapplication.TenantLifecycleToTenantRolePermissionChildCapability",
		"TenantRolePermission tenantapplication.TenantMemberLifecycleToTenantRolePermissionChildCapability",
		"tenantapplication.NewTenantLifecycleToTenantRolePermissionChildCapability(applications.TenantRolePermission, executor)",
		"tenantapplication.NewTenantMemberLifecycleToTenantRolePermissionChildCapability(applications.TenantRolePermission, executor)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("generated assembly missing edge-owned capability %q:\n%s", required, source)
		}
	}
}

func TestRenderC9EdgeOwnedCapabilitiesAreDeterministic(t *testing.T) {
	manifest := edgeOwnedCapabilityFixture()
	first, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	manifest.Services[0], manifest.Services[2] = manifest.Services[2], manifest.Services[0]
	second, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("generated file count drifted: first=%d second=%d", len(first), len(second))
	}
	for index := range first {
		if first[index].Path != second[index].Path || !bytes.Equal(first[index].Content, second[index].Content) {
			t.Fatalf("edge-owned capability generation is not deterministic at index %d:\nfirst=%s\nsecond=%s", index, first[index].Content, second[index].Content)
		}
	}
}

func generatedApplicationPaths(files []GeneratedApplicationFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}
