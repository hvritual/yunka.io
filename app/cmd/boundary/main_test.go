package boundary

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T, inventory bool) (string, []string) {
	t.Helper()
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		var err error
		protoc, err = exec.LookPath("protoc")
		if err != nil {
			t.Skip("real protoc is required")
		}
	}
	root := t.TempDir()
	support, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "contracts", "proto", "yunka", "dsl", "v1", "options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "support,explicit/yunka/dsl/v1/options.proto"), string(support))
	for _, domain := range []string{"alpha", "beta"} {
		write(t, filepath.Join(root, "contracts/proto", domain, "dto.proto"), fmt.Sprintf(`syntax="proto3";package %s.v1;import "yunka/dsl/v1/options.proto";
message Request { option(yunka.dsl.v1.dto)={kind:DTO_INPUT};string id=1; }
message Reply { option(yunka.dsl.v1.dto)={kind:DTO_OUTPUT};string id=1; }`, domain))
		write(t, filepath.Join(root, "contracts/proto", domain, "service.proto"), fmt.Sprintf(`syntax="proto3";package %s.v1;import "yunka/dsl/v1/options.proto";import %q;
option(yunka.dsl.v1.domain)={name:%q};service API {
 option(yunka.dsl.v1.application)={name:"management" operations:{id:%q use_case:"inspect" public:true request_type:%q response_type:%q application_method:"Inspect"}};
 rpc Get(Request) returns(Reply){option(yunka.dsl.v1.operation)={id:%q use_case:"get" public:true boundary:{context:%q aggregate:"item"}};}
}`, domain, func() string {
			if inventory {
				return "dto.proto"
			}
			return domain + "/dto.proto"
		}(), domain, domain+".inspect", domain+".v1.Request", domain+".v1.Reply", domain+".get", domain+".items"))
	}
	args := []string{"--root", root, "--protoc", protoc, "--format", "json"}
	if inventory {
		inv := contract.SourceInventory{SchemaVersion: 1, SourceSets: []contract.SourceSet{
			{Name: "alpha", Root: "contracts/proto/alpha", Files: []string{"service.proto", "dto.proto"}, ProtoPaths: []string{"support,explicit"}},
			{Name: "beta", Root: "contracts/proto/beta", Files: []string{"dto.proto", "service.proto"}, ProtoPaths: []string{"support,explicit"}},
		}}
		data, _ := json.Marshal(inv)
		write(t, filepath.Join(root, "contracts/sources.json"), string(data))
	} else {
		args = append(args, "--proto-path", "support,explicit")
	}
	return root, args
}
func run(args ...string) (string, error) {
	app := cli.NewApp()
	app.Name = "yunka"
	app.Commands = []cli.Command{Command()}
	var out bytes.Buffer
	app.Writer = &out
	app.ErrWriter = &out
	err := app.Run(append([]string{"yunka", "boundary", "inspect"}, args...))
	return out.String(), err
}
func decode(t *testing.T, s string) Report {
	t.Helper()
	var r Report
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("decode: %v\n%s", err, s)
	}
	return r
}
func digestTree(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	err := filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		result[rel] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestIssue161BoundaryCLIRealProtocNamespaces(t *testing.T) {
	for _, inventory := range []bool{false, true} {
		t.Run(fmt.Sprintf("inventory=%t", inventory), func(t *testing.T) {
			root, args := fixture(t, inventory)
			before := digestTree(t, root)
			first, err := run(append(args, "alpha/management")...)
			if err != nil {
				t.Fatalf("%v\n%s", err, first)
			}
			second, err := run(append(args, "alpha/management")...)
			if err != nil || first != second {
				t.Fatalf("unstable report %v", err)
			}
			r := decode(t, first)
			if r.Authority != "read_only" || r.IntentCoverage.State != "partial" || !reflect.DeepEqual(r.IntentCoverage.UnknownOperations, []string{"alpha.inspect"}) {
				t.Fatalf("wrong coverage %#v", r.IntentCoverage)
			}
			if len(r.Fingerprint.Operations) != 2 || r.Fingerprint.Service != "alpha.v1.API" {
				t.Fatal("wrong Service projection")
			}
			for _, p := range r.Sources {
				if strings.Contains(p.ProjectPath, "beta") || filepath.IsAbs(p.ProjectPath) {
					t.Fatal("scope leak")
				}
				if _, err := os.Stat(filepath.Join(root, p.ProjectPath)); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(first, root) || strings.Contains(first, "safe_to_merge") || strings.Contains(first, "boundaryDecision") {
				t.Fatal("absolute-path leak or invented authority")
			}
			textArgs := append([]string(nil), args...)
			for i := range textArgs {
				if textArgs[i] == "json" {
					textArgs[i] = "text"
				}
			}
			text, err := run(append(textArgs, "alpha/management")...)
			if err != nil || !strings.Contains(text, "No boundary decision or mutation authorization") || !strings.Contains(text, "alpha.inspect boundary=unknown") {
				t.Fatalf("text=%s %v", text, err)
			}
			if !reflect.DeepEqual(before, digestTree(t, root)) {
				t.Fatal("read-only command wrote project state")
			}
		})
	}
}

func TestIssue161BoundaryCLIRecompilesAndSurvivesMoves(t *testing.T) {
	root, args := fixture(t, false)
	first, err := run(append(args, "alpha/management")...)
	if err != nil {
		t.Fatal(err)
	}
	before := decode(t, first)
	path := filepath.Join(root, "contracts/proto/alpha/service.proto")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.ReplaceAll(string(source), `aggregate:"item"`, `aggregate:"order"`))
	next, err := run(append(args, "alpha/management")...)
	if err != nil {
		t.Fatal(err)
	}
	after := decode(t, next)
	if before.FingerprintDigest == after.FingerprintDigest || before.OperationPlansDigest != after.OperationPlansDigest {
		t.Fatal("intent mutation was lost or altered runtime IR")
	}
	oldDTO := filepath.Join(root, "contracts/proto/alpha/dto.proto")
	newDTO := filepath.Join(root, "contracts/proto/alpha/model/dto.proto")
	if err := os.MkdirAll(filepath.Dir(newDTO), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldDTO, newDTO); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.ReplaceAll(string(current), `"alpha/dto.proto"`, `"alpha/model/dto.proto"`))
	moved, err := run(append(args, "alpha/management")...)
	if err != nil {
		t.Fatal(err)
	}
	mr := decode(t, moved)
	if mr.FingerprintDigest == after.FingerprintDigest || mr.OperationPlansDigest != after.OperationPlansDigest || !strings.Contains(moved, "alpha/model/dto.proto") {
		t.Fatal("source move did not preserve semantic identity")
	}
	// A stale generated artifact cannot hide broken canonical input.
	write(t, filepath.Join(root, "contracts/generated/manifest.json"), `{"schemaVersion":5,"services":[]}`)
	write(t, path, "broken protobuf")
	snapshot := digestTree(t, root)
	output, err := run(append(args, "alpha/management")...)
	if err == nil || strings.Contains(output, `"fingerprintDigest"`) {
		t.Fatal("broken source fell back to stale/partial evidence")
	}
	if !reflect.DeepEqual(snapshot, digestTree(t, root)) {
		t.Fatal("failure path changed project")
	}
}

func TestIssue161BoundaryCLIRejectsInvalidInputsAndCancellation(t *testing.T) {
	root, args := fixture(t, false)
	cases := [][]string{args, append(append([]string{}, args...), "alpha/management", "extra"), append(append([]string{}, args...), "alpha/absent"), append(append([]string{}, args...), "--proto-path", "", "alpha/management"), append(append([]string{}, args...), "--format", "yaml", "alpha/management"), append(append([]string{}, args...), "--protoc", "", "alpha/management")}
	before := digestTree(t, root)
	for _, input := range cases {
		out, err := run(input...)
		if err == nil || strings.Contains(out, `"fingerprintDigest"`) {
			t.Fatalf("accepted invalid query %v: %s %v", input, out, err)
		}
	}
	if !reflect.DeepEqual(before, digestTree(t, root)) {
		t.Fatal("invalid queries wrote project")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, projectflow.Options{Root: root}, "alpha/management"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	_, inventoryArgs := fixture(t, true)
	output, err := run(append(inventoryArgs, "--proto-path", "ignored", "alpha/management")...)
	if err == nil || !strings.Contains(err.Error(), "inventory includes") || strings.Contains(output, `"fingerprintDigest"`) {
		t.Fatalf("silently ignored inventory override: %s %v", output, err)
	}
}
