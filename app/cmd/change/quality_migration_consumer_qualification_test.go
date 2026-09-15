package change

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	domaincmd "yunka.io/app/cmd/domain"
	projectcmd "yunka.io/app/cmd/project"
	"yunka.io/app/cmd/projectflow"
)

func TestQualityMigrationRealConsumersUseSameContract(t *testing.T) {
	frameworkRoot := strings.TrimSpace(os.Getenv("YUNKA_MIGRATION_FRAMEWORK_ROOT"))
	bizSource := strings.TrimSpace(os.Getenv("YUNKA_MIGRATION_CONSUMER_BIZ"))
	bizRuntime := strings.TrimSpace(os.Getenv("YUNKA_MIGRATION_BIZ_RUNTIME"))
	iotSource := strings.TrimSpace(os.Getenv("YUNKA_MIGRATION_CONSUMER_IOT"))
	iotRuntime := strings.TrimSpace(os.Getenv("YUNKA_MIGRATION_IOT_RUNTIME"))
	values := []string{frameworkRoot, bizSource, bizRuntime, iotSource, iotRuntime}
	allEmpty := true
	for _, value := range values {
		if value != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		t.Skip("real consumer migration qualification is not configured")
	}
	for index, value := range values {
		if value == "" {
			t.Fatalf("real consumer migration qualification input %d is missing", index)
		}
	}

	t.Run("biz-generic-container", func(t *testing.T) {
		workspace, projectRoot := cloneMigrationConsumerWorkspace(t, bizSource, bizRuntime, false)
		installMigrationConsumerBaseline(t, frameworkRoot, projectRoot, "biz.json")

		touched := []string{
			"internal/access/domain/model.go",
			"internal/access/domain/data_scope.go",
			"internal/access/domain/errors.go",
			"internal/access/domain/tenant.go",
			"internal/access/domain/membership.go",
			"internal/access/domain/role.go",
			"internal/access/domain/credential.go",
		}
		plan, root, err := BuildQualityMigrationPlanWithCoverage(context.Background(), projectflow.Options{Root: projectRoot}, workspace, "HEAD", []string{MigrationRecipeGenericContainerSplit}, touched, ReviewNarrative{
			Problem: "The access domain model file aggregates tenant, membership, role, permission scope, credential, and error responsibilities.",
			CurrentConcepts: []string{"single generic access model container"},
			DesiredOwnership: []string{"tenant lifecycle source", "membership lifecycle source", "role and grant source", "credential source", "data-scope source", "domain error source"},
			Why: "Make durable domain responsibility visible without changing the access model contract.",
			What: "Split internal/access/domain/model.go into responsibility-named files inside the same Go package.",
			Boundary: "No behavior, exported API, persistence path, generated ownership, or package boundary changes.",
			AffectedInvariants: []string{"access domain behavior", "public access-domain API"},
			Risks: []string{"declaration loss while moving source between files"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !migrationPlanHasRule(plan, "AUDIT-NAME-002") {
			t.Fatalf("Biz model.go did not expose the generic-container baseline: %#v", plan.BaselineFindings)
		}
		if _, err := WriteQualityMigrationPlan(root, DefaultQualityMigrationPlanPath, plan); err != nil {
			t.Fatal(err)
		}
		splitBizAccessModel(t, projectRoot)
		commitMigrationCandidate(t, projectRoot, "split access domain concepts")

		packet, err := CheckQualityMigration(context.Background(), projectflow.Options{Root: projectRoot}, DefaultQualityMigrationPlanPath)
		if err != nil {
			t.Fatal(err)
		}
		assertStructuralMigrationPacket(t, packet)
		runMigrationConsumerTest(t, projectRoot, "./internal/access/domain")
	})

	t.Run("iot-durable-test-identity", func(t *testing.T) {
		workspace, projectRoot := cloneMigrationConsumerWorkspace(t, iotSource, iotRuntime, true)
		installMigrationConsumerBaseline(t, frameworkRoot, projectRoot, "iot-current.json")

		oldPath := "backend-yunka/internal/delivery/ag03_sqlite_startup_test.go"
		newPath := "backend-yunka/internal/delivery/sqlite_startup_lock_test.go"
		plan, root, err := BuildQualityMigrationPlanWithCoverage(context.Background(), projectflow.Options{Root: projectRoot}, workspace, "HEAD", []string{MigrationRecipeDurableTestRename}, []string{oldPath, newPath}, ReviewNarrative{
			Problem: "A durable SQLite startup regression is named after historical delivery identifier AG03 instead of the invariant it protects.",
			CurrentConcepts: []string{"task-named SQLite startup regression"},
			DesiredOwnership: []string{"SQLite startup external-lock invariant regression"},
			Why: "Keep regression evidence understandable after the delivery task identifier loses context.",
			What: "Rename the SQLite startup lock regression file and test function by the invariant they prove.",
			Boundary: "No production behavior, exported API, persistence implementation, or generated ownership changes.",
			AffectedInvariants: []string{"SQLite startup waits for an external file lock within the existing busy budget"},
			Risks: []string{"accidental test weakening during rename"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !migrationPlanHasRule(plan, "AUDIT-NAME-001") {
			t.Fatalf("IoT durable test did not expose the historical-identity baseline: %#v", plan.BaselineFindings)
		}
		if _, err := WriteQualityMigrationPlan(root, DefaultQualityMigrationPlanPath, plan); err != nil {
			t.Fatal(err)
		}
		renameIoTSQLiteRegression(t, projectRoot, oldPath, newPath)
		commitMigrationCandidate(t, projectRoot, "rename SQLite startup lock regression")

		packet, err := CheckQualityMigration(context.Background(), projectflow.Options{Root: projectRoot}, DefaultQualityMigrationPlanPath)
		if err != nil {
			t.Fatal(err)
		}
		assertStructuralMigrationPacket(t, packet)
		if packet.QualityDebt == nil || packet.QualityDebt.DeterministicFixed == 0 {
			t.Fatalf("expected historical naming debt to be fixed: %#v", packet.QualityDebt)
		}
		runMigrationConsumerTest(t, filepath.Join(projectRoot, "backend-yunka"), "./internal/delivery", "-run", "^TestSQLiteStartupWaitsForExternalLock$")
	})
}

func cloneMigrationConsumerWorkspace(t *testing.T, sourceProject, sourceRuntime string, runtimeInsideProject bool) (string, string) {
	t.Helper()
	workspace := t.TempDir()
	projectRoot := filepath.Join(workspace, "project")
	cloneExactMigrationCheckout(t, sourceProject, projectRoot)
	runtimeRoot := filepath.Join(workspace, "yunka.io")
	if runtimeInsideProject {
		runtimeRoot = filepath.Join(projectRoot, "third_party", "yunka")
		if err := os.RemoveAll(runtimeRoot); err != nil {
			t.Fatal(err)
		}
	}
	cloneExactMigrationCheckout(t, sourceRuntime, runtimeRoot)
	if status := migrationGit(t, projectRoot, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("consumer clone is dirty before governance baseline:\n%s", status)
	}
	return workspace, projectRoot
}

func cloneExactMigrationCheckout(t *testing.T, source, destination string) {
	t.Helper()
	source = strings.TrimSpace(source)
	sourceRepo := migrationGit(t, source, "rev-parse", "--show-toplevel")
	head := migrationGit(t, source, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, "", "git", "init", destination)
	migrationExec(t, destination, "git", "config", "user.email", "quality-migration@example.invalid")
	migrationExec(t, destination, "git", "config", "user.name", "Yunka Quality Migration")
	migrationExec(t, destination, "git", "fetch", "--no-tags", "--depth=1", sourceRepo, head)
	migrationExec(t, destination, "git", "checkout", "--detach", "FETCH_HEAD")
	if got := migrationGit(t, destination, "rev-parse", "HEAD"); got != head {
		t.Fatalf("local clone head=%s want=%s", got, head)
	}
}

func installMigrationConsumerBaseline(t *testing.T, frameworkRoot, projectRoot, policyFixture string) {
	t.Helper()
	policy, err := os.ReadFile(filepath.Join(frameworkRoot, "tools", "source-policy", policyFixture))
	if err != nil {
		t.Fatal(err)
	}
	writeMigrationQualificationFile(t, filepath.Join(projectRoot, ".yunka", "source-policy.json"), policy)
	establishExplicitMigrationDomainCoverage(t, projectRoot)
	report, err := projectcmd.EnsureEngineeringQualityBaseline(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if report.Skipped != "" || report.Policy == "" || report.Baseline == "" {
		t.Fatalf("engineering-quality baseline was not installed: %#v", report)
	}
	migrationExec(t, projectRoot, "git", "add", "-A")
	migrationExec(t, projectRoot, "git", "commit", "-m", "establish bounded migration governance baseline")
}

func establishExplicitMigrationDomainCoverage(t *testing.T, projectRoot string) {
	t.Helper()
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	domainRoot := projectflow.ResolveDescriptorPath(descriptor, descriptor.GeneratedGoRoot)
	entries, err := domaincmd.InspectCoverage(domainRoot)
	if err != nil {
		t.Fatal(err)
	}
	var exemptions []domaincmd.CoverageExemption
	for _, entry := range entries {
		if entry.State != domaincmd.CoverageUnknown {
			continue
		}
		exemptions = append(exemptions, domaincmd.CoverageExemption{
			Domain: entry.Domain,
			Reason: "real-consumer migration qualification explicitly establishes developer ownership before organization repair",
			Owner:  "engineering-quality-migration",
		})
	}
	if len(exemptions) == 0 {
		return
	}
	coveragePath := filepath.Join(projectRoot, filepath.FromSlash(domaincmd.DomainCoverageRelativePath))
	if _, err := os.Stat(coveragePath); err == nil {
		t.Fatalf("existing domain coverage still contains UNKNOWN entries; qualification will not overwrite %s", coveragePath)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	contents, err := json.MarshalIndent(domaincmd.CoverageContract{SchemaVersion: domaincmd.DomainCoverageSchemaVersion, Exemptions: exemptions}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeMigrationQualificationFile(t, coveragePath, append(contents, '\n'))
	if _, err := domaincmd.ValidateCoverage(domainRoot); err != nil {
		t.Fatalf("explicit migration domain coverage: %v", err)
	}
}

func splitBizAccessModel(t *testing.T, projectRoot string) {
	t.Helper()
	modelPath := filepath.Join(projectRoot, "internal", "access", "domain", "model.go")
	contents, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, modelPath, contents, 0)
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]ast.Decl{}
	for _, declaration := range file.Decls {
		if generated, ok := declaration.(*ast.GenDecl); ok && generated.Tok == token.IMPORT {
			continue
		}
		name, ok := bizAccessDeclarationFile(declaration)
		if !ok {
			t.Fatalf("unclassified access model declaration %T", declaration)
		}
		groups[name] = append(groups[name], declaration)
	}
	expected := []string{"credential.go", "data_scope.go", "errors.go", "membership.go", "role.go", "tenant.go"}
	for _, name := range expected {
		declarations := groups[name]
		if len(declarations) == 0 {
			t.Fatalf("access split produced no declarations for %s", name)
		}
		var body strings.Builder
		for _, declaration := range declarations {
			if err := printer.Fprint(&body, set, declaration); err != nil {
				t.Fatal(err)
			}
			body.WriteString("\n\n")
		}
		imports := []string{}
		for _, candidate := range []string{"errors", "sort", "time"} {
			if strings.Contains(body.String(), candidate+".") {
				imports = append(imports, candidate)
			}
		}
		sort.Strings(imports)
		var source strings.Builder
		source.WriteString("package domain\n\n")
		if len(imports) == 1 {
			source.WriteString("import \"")
			source.WriteString(imports[0])
			source.WriteString("\"\n\n")
		} else if len(imports) > 1 {
			source.WriteString("import (\n")
			for _, imported := range imports {
				source.WriteString("\t\"")
				source.WriteString(imported)
				source.WriteString("\"\n")
			}
			source.WriteString(")\n\n")
		}
		source.WriteString(body.String())
		formatted, err := format.Source([]byte(source.String()))
		if err != nil {
			t.Fatalf("format %s: %v\n%s", name, err, source.String())
		}
		writeMigrationQualificationFile(t, filepath.Join(filepath.Dir(modelPath), name), formatted)
	}
	if len(groups) != len(expected) {
		t.Fatalf("unexpected access split groups: %#v", groups)
	}
	if err := os.Remove(modelPath); err != nil {
		t.Fatal(err)
	}
}

func bizAccessDeclarationFile(declaration ast.Decl) (string, bool) {
	if function, ok := declaration.(*ast.FuncDecl); ok {
		if function.Recv != nil && len(function.Recv.List) > 0 {
			switch migrationReceiverName(function.Recv.List[0].Type) {
			case "Tenant":
				return "tenant.go", true
			case "Membership":
				return "membership.go", true
			case "Role":
				return "role.go", true
			}
		}
		switch function.Name.Name {
		case "NewTenant":
			return "tenant.go", true
		case "NewInvitedMembership", "NewActiveMembership":
			return "membership.go", true
		case "NewRole", "NewOwnerRole", "containsOwnerRequiredPermissions":
			return "role.go", true
		}
		return "", false
	}
	group, ok := declaration.(*ast.GenDecl)
	if !ok {
		return "", false
	}
	selected := ""
	for _, spec := range group.Specs {
		var names []string
		switch typed := spec.(type) {
		case *ast.TypeSpec:
			names = []string{typed.Name.Name}
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				names = append(names, name.Name)
			}
		default:
			return "", false
		}
		for _, name := range names {
			candidate := bizAccessNameFile(name)
			if candidate == "" || (selected != "" && selected != candidate) {
				return "", false
			}
			selected = candidate
		}
	}
	return selected, selected != ""
}

func bizAccessNameFile(name string) string {
	switch {
	case name == "DataScope" || strings.HasPrefix(name, "DataScope"):
		return "data_scope.go"
	case strings.HasPrefix(name, "Err"):
		return "errors.go"
	case name == "Tenant" || strings.HasPrefix(name, "TenantStatus"):
		return "tenant.go"
	case name == "User" || name == "Membership" || strings.HasPrefix(name, "TenantMemberStatus"):
		return "membership.go"
	case name == "Role" || name == "MemberRole" || name == "PermissionGrant" || name == "MemberSite" || name == "OwnerRequiredPermissions" || strings.HasPrefix(name, "TenantRoleStatus") || name == "TenantOwnerRoleName":
		return "role.go"
	case name == "Credential":
		return "credential.go"
	default:
		return ""
	}
}

func migrationReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return migrationReceiverName(typed.X)
	default:
		return ""
	}
}

func renameIoTSQLiteRegression(t *testing.T, projectRoot, oldPath, newPath string) {
	t.Helper()
	oldAbsolute := filepath.Join(projectRoot, filepath.FromSlash(oldPath))
	contents, err := os.ReadFile(oldAbsolute)
	if err != nil {
		t.Fatal(err)
	}
	before := "TestAG03SQLiteStartupWaitsForExternalLock"
	after := "TestSQLiteStartupWaitsForExternalLock"
	if strings.Count(string(contents), before) != 1 {
		t.Fatalf("expected exactly one durable test identity %s", before)
	}
	contents = []byte(strings.Replace(string(contents), before, after, 1))
	writeMigrationQualificationFile(t, filepath.Join(projectRoot, filepath.FromSlash(newPath)), contents)
	if err := os.Remove(oldAbsolute); err != nil {
		t.Fatal(err)
	}
}

func assertStructuralMigrationPacket(t *testing.T, packet QualityMigrationReviewPacket) {
	t.Helper()
	if !packet.Conformant {
		t.Fatalf("migration packet violations=%#v", packet.Violations)
	}
	for name, delta := range map[string]ReviewDelta{
		"behavior": packet.BehaviorChange,
		"public-api": packet.PublicAPIChange,
		"persistence": packet.PersistenceChange,
		"generated": packet.GeneratedCodeChange,
	} {
		if delta.State != ReviewDeltaNone {
			t.Fatalf("%s delta=%#v", name, delta)
		}
	}
	if packet.QualityDebt == nil || packet.QualityDebt.BlockingNew != 0 {
		t.Fatalf("quality debt=%#v", packet.QualityDebt)
	}
}

func commitMigrationCandidate(t *testing.T, projectRoot, message string) {
	t.Helper()
	migrationExec(t, projectRoot, "git", "add", "-A")
	migrationExec(t, projectRoot, "git", "commit", "-m", message)
}

func runMigrationConsumerTest(t *testing.T, root string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	commandArgs := append([]string{"test"}, args...)
	commandArgs = append(commandArgs, "-count=1")
	command := exec.CommandContext(ctx, "go", commandArgs...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("consumer go %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
}

func migrationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s: %v\n%s", root, strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func migrationExec(t *testing.T, root, commandName string, args ...string) {
	t.Helper()
	command := exec.Command(commandName, args...)
	if root != "" {
		command.Dir = root
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", commandName, strings.Join(args, " "), err, output)
	}
}

func writeMigrationQualificationFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
}
