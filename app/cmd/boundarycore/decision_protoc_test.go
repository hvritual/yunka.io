package boundarycore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
)

func TestIssue161DecisionRealProtocAddition(t *testing.T) {
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		var err error
		protoc, err = exec.LookPath("protoc")
		if err != nil {
			t.Skip("real protoc required")
		}
	}
	support, err := filepath.Abs(filepath.Join("..", "..", "..", "contracts", "proto"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dto := `syntax="proto3";package sales.v1;import "yunka/dsl/v1/options.proto";
message Request {option(yunka.dsl.v1.dto)={kind:DTO_INPUT}; string id=1;}
message Reply {option(yunka.dsl.v1.dto)={kind:DTO_OUTPUT}; string id=1;}`
	service := `syntax="proto3";package sales.v1;import "dto.proto";import "yunka/dsl/v1/options.proto";
option(yunka.dsl.v1.domain)={name:"sales"};
service Orders {option(yunka.dsl.v1.application)={name:"orders"};
rpc Get(Request) returns(Reply){option(yunka.dsl.v1.operation)={id:"sales.get" use_case:"get" public:true boundary:{context:"sales.orders" aggregate:"order"}};}
%s
}`
	compile := func(name, member string) contract.Manifest {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		template := service
		if strings.HasPrefix(member, "option") {
			template = strings.Replace(template, `option(yunka.dsl.v1.application)={name:"orders"};`, member, 1)
			member = ""
		}
		for file, value := range map[string]string{"dto.proto": dto, "service.proto": fmt.Sprintf(template, member)} {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
		r, err := contract.Compile(context.Background(), contract.CompileOptions{Dir: dir, Protoc: protoc, ProtoPaths: []string{support}})
		if err != nil {
			t.Fatal(err)
		}
		return r.Manifest
	}
	before := compile("base", "")
	request := AdditionRequest{strings.Repeat("c", 40), "sales/orders", "sales.next"}
	for _, internal := range []bool{false, true} {
		t.Run(fmt.Sprintf("internal=%t", internal), func(t *testing.T) {
			for _, contextName := range []string{"sales.orders", "shipping.jobs"} {
				member := fmt.Sprintf(`rpc Next(Request) returns(Reply){option(yunka.dsl.v1.operation)={id:"sales.next" use_case:"next" public:true boundary:{context:%q aggregate:"order"}};}`, contextName)
				if internal {
					// Internal candidates are canonical Operations, not fabricated RPCs.
					member = fmt.Sprintf(`option(yunka.dsl.v1.application)={name:"orders" operations:{id:"sales.next" use_case:"next" public:true request_type:"sales.v1.Request" response_type:"sales.v1.Reply" application_method:"Next" boundary:{context:%q aggregate:"order"}}};`, contextName)
					// Replace the one Application option with its internal declaration.
				}
				after := compile(fmt.Sprintf("%t-%s", internal, contextName), member)
				b, _ := json.Marshal(before)
				a, _ := json.Marshal(after)
				d := evaluate(t, request, before, after)
				want := ReuseExistingApplication
				if internal {
					want = ArchitectureReviewRequired
				} // RPC-only peers do not witness an internal transport shape.
				if contextName != "sales.orders" {
					want = CreateNewApplication
				}
				if d.Outcome != want {
					t.Fatalf("want %s got %+v", want, d)
				}
				if !reflect.DeepEqual(d, evaluate(t, request, before, after)) {
					t.Fatal("nondeterministic decision")
				}
				bb, _ := json.Marshal(before)
				aa, _ := json.Marshal(after)
				if sha256.Sum256(b) != sha256.Sum256(bb) || sha256.Sum256(a) != sha256.Sum256(aa) {
					t.Fatal("decision modified compiler evidence")
				}
			}
		})
	}
}
