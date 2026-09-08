package applicationboundary

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestImportedWildcardOmittedPackagesAreAnalyzed(t *testing.T) {
	for _, dir := range []string{"_hidden", ".private", "testdata/helper", "_hidden/nested"} {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			module := "example.test/coverage"
			hidden := module + "/" + dir
			files := map[string]string{
				"go.mod":           "module " + module + "\n\ngo 1.23.0\n",
				"entry.go":         fmt.Sprintf("package entry\nimport h %q\nfunc Entry(){h.Wire()}\n", hidden),
				"owner/owner.go":   "package owner\ntype Reader interface{Read()}\ntype narrow struct{}\nfunc(*narrow)Read(){}\nfunc Build()Reader{return &narrow{}}\n",
				dir + "/hidden.go": fmt.Sprintf("package hidden\nimport renamed %q\nfunc Wire(){_ = renamed.Build()}\n", module+"/owner"),
			}
			for name, source := range files {
				p := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := Policy{SchemaVersion: 1, Factories: []Factory{{Symbol: Symbol{module + "/owner", "Build"}, AllowedCallers: []string{module}, Results: []Slot{{0, Symbol{module + "/owner", "Reader"}}}}}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			r := Check(ctx, root, p, Options{})
			if r.Status != Fail || r.CheckedReferences != 1 || len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-001" || r.Findings[0].File != dir+"/hidden.go" {
				t.Fatalf("active imported package omitted: %+v", r)
			}
			if !slices.Contains(r.Packages, hidden) {
				t.Fatal("imported package not recorded as analyzed")
			}
			p.Factories[0].AllowedCallers = []string{hidden}
			if r := Check(ctx, root, p, Options{}); r.Status != Pass || r.CheckedReferences != 1 {
				t.Fatalf("legal imported package not fully analyzed: %+v", r)
			}
		})
	}
}

func TestLoaderRejectsParentPATHWrapperWithoutExecution(t *testing.T) {
	fakeRoot := t.TempDir()
	marker := filepath.Join(fakeRoot, "invoked")
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	fakeGo := filepath.Join(fakeRoot, name)
	source := "package main\nimport(\"os\";\"os/exec\")\nfunc main(){_ = os.WriteFile(" + strconv.Quote(marker) + ",[]byte(\"called\"),0600);cmd:=exec.Command(" + strconv.Quote(loaderGoPath()) + ",os.Args[1:]...);cmd.Stdout=os.Stdout;cmd.Stderr=os.Stderr;cmd.Env=os.Environ();if cmd.Run()!=nil{os.Exit(1)}}\n"
	file := filepath.Join(fakeRoot, "wrapper.go")
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, loaderGoPath(), "build", "-o", fakeGo, file)
	cmd.Env = loadEnvironment()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build controlled Go wrapper: %v %s", err, out)
	}
	t.Setenv("PATH", fakeRoot+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := Check(ctx, writeModule(t, `func Build()Reader{return &narrow{}}`), policyFor("Build"), Options{})
	if r.Status != Incomplete || len(r.Findings) != 1 || !strings.Contains(r.Findings[0].Message, "process PATH") {
		t.Fatalf("wrong loader executable accepted: %+v", r)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("parent PATH wrapper was executed before rejection")
	}
}

func TestLoaderAcceptsCanonicalGoFirst(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(loaderGoPath())+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := Check(context.Background(), writeModule(t, `func Build()Reader{return &narrow{}}`), policyFor("Build"), Options{})
	if r.Status != Pass {
		t.Fatalf("canonical loader rejected: %+v", r)
	}
}
