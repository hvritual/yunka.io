package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/applicationboundary"
)

func TestTypedAuditPublicCLI(t *testing.T) {
	for _, tc := range []struct{ name, source, status string }{
		{"positive", `return &small{}`, applicationboundary.Pass},
		{"wide", `return &big{}`, applicationboundary.Fail},
		{"unknown", `var r Reader;return r`, applicationboundary.Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := `package sample
type Reader interface{ Read() }
type small struct{}
func(*small)Read(){}
type big struct{}
func(*big)Read(){}
func(*big)Delete(){}
func Build()Reader{` + tc.source + `}
`
			policy := `{"schemaVersion":1,"factories":[{"symbol":{"package":"example.test/sample","name":"Build"},"allowedCallerPackages":["example.test/sample"],"results":[{"index":0,"contract":{"package":"example.test/sample","name":"Reader"}}]}]}`
			for name, s := range map[string]string{"go.mod": "module example.test/sample\n\ngo 1.23.0\n", "sample.go": source, "policy.json": policy} {
				if e := os.WriteFile(filepath.Join(root, name), []byte(s), 0600); e != nil {
					t.Fatal(e)
				}
			}
			var out bytes.Buffer
			app := cli.NewApp()
			app.Writer = &out
			app.ErrWriter = &out
			app.Commands = []cli.Command{Command()}
			err := app.Run([]string{"yunka", "audit", "types", "--root", root, "--policy", "policy.json", "--format", "agent-json"})
			if (err == nil) != (tc.status == applicationboundary.Pass) {
				t.Fatalf("exit error=%v output=%s", err, out.String())
			}
			var report applicationboundary.Report
			if e := json.Unmarshal(out.Bytes(), &report); e != nil {
				t.Fatalf("not machine JSON: %v %s", e, out.String())
			}
			if report.Status != tc.status || report.CheckedFactories != 1 {
				t.Fatalf("report: %+v", report)
			}
			if strings.Contains(out.String(), root) {
				t.Fatal("absolute path leaked")
			}
		})
	}
}
func TestTypedRenderAndMissingPolicy(t *testing.T) {
	app := cli.NewApp()
	app.Commands = []cli.Command{Command()}
	if err := app.Run([]string{"yunka", "audit", "types"}); err == nil {
		t.Fatal("missing policy passed")
	}
	if _, err := RenderTypes(applicationboundary.Report{}, "invented"); err == nil {
		t.Fatal("invalid format accepted")
	}
}
