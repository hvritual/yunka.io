package applicationboundary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

type Options struct{ Tags string }

// Check loads one module and one active build configuration. It deliberately
// ignores inherited workspaces, GOFLAGS and package-driver hooks, disallows module
// and toolchain downloads, and never runs business code. Dependencies must already
// be available. It does not certify nested modules or the AG-05 build matrix.
func Check(ctx context.Context, root string, policy Policy, options Options) Report {
	r := newReport(policy)
	incomplete := func(message string) Report {
		r.Findings = append(r.Findings, Finding{Rule: "AG-TYPE-000", Class: Unknown, Subject: "source", Message: message})
		r.finish()
		return r
	}
	if err := policy.Validate(); err != nil {
		return incomplete(err.Error())
	}
	if ctx == nil {
		return incomplete("a context is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return incomplete("cannot resolve module root")
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return incomplete("cannot resolve module root")
	}
	if strings.ContainsAny(options.Tags, "\n\r") {
		return incomplete("invalid build tags")
	}
	before := map[string]string{}
	record := func(name string) error {
		actual, e := filepath.EvalSymlinks(name)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(abs, actual)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("source escapes module root")
		}
		data, e := os.ReadFile(actual)
		if e != nil {
			return e
		}
		h := sha256.Sum256(data)
		before[actual] = hex.EncodeToString(h[:])
		return nil
	}
	if err := record(filepath.Join(abs, "go.mod")); err != nil {
		return incomplete("module root must contain a regular readable go.mod")
	}
	if _, err := os.Stat(filepath.Join(abs, "go.sum")); err == nil {
		if err := record(filepath.Join(abs, "go.sum")); err != nil {
			return incomplete("cannot capture go.sum")
		}
	}
	flags := []string{"-mod=readonly"}
	if options.Tags != "" {
		flags = append(flags, "-tags="+options.Tags)
	}
	cfg := &packages.Config{Context: ctx, Dir: abs, Fset: token.NewFileSet(), Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule, Env: loadEnvironment(), BuildFlags: flags, Tests: false}
	// Capture exactly the bytes parsed, not a second read taken only after loading.
	var captureMu sync.Mutex
	cfg.ParseFile = func(fset *token.FileSet, name string, src []byte) (*ast.File, error) {
		rel, e := filepath.Rel(abs, name)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("compiled source outside module")
		}
		actual, e := filepath.EvalSymlinks(name)
		if e != nil {
			return nil, e
		}
		if actual != name {
			return nil, fmt.Errorf("symlinked compiled source is unsupported")
		}
		if src == nil {
			var e error
			src, e = os.ReadFile(name)
			if e != nil {
				return nil, e
			}
		}
		h := sha256.Sum256(src)
		captureMu.Lock()
		before[name] = hex.EncodeToString(h[:])
		captureMu.Unlock()
		return parser.ParseFile(fset, name, src, parser.ParseComments|parser.AllErrors)
	}
	loaded, err := packages.Load(cfg, "./...")
	if err != nil || ctx.Err() != nil {
		return incomplete("package loading failed or was cancelled; verify toolchain and cached dependencies")
	}
	if len(loaded) == 0 {
		return incomplete("module has no packages for the active build")
	}
	var errorsFound bool
	packages.Visit(loaded, nil, func(p *packages.Package) {
		if len(p.Errors) > 0 || p.IllTyped {
			errorsFound = true
		}
	})
	if errorsFound {
		return incomplete("package loading/type checking is incomplete; run the canonical build with approved dependencies")
	}
	program := Program{Fset: cfg.Fset, Root: abs}
	module := ""
	for _, p := range loaded {
		if p.Module == nil || filepath.Clean(p.Module.Dir) != abs {
			return incomplete("loaded package is outside the selected module")
		}
		if module == "" {
			module = p.Module.Path
		} else if module != p.Module.Path {
			return incomplete("multiple source modules require separate typed checks")
		}
		if len(p.Syntax) == 0 || p.TypesInfo == nil {
			return incomplete("loaded package has no complete syntax/types")
		}
		program.Packages = append(program.Packages, SourcePackage{Types: p.Types, Info: p.TypesInfo, Files: p.Syntax})
	}
	for _, f := range policy.Factories {
		if f.Symbol.Package != module && !strings.HasPrefix(f.Symbol.Package, module+"/") {
			return incomplete("factory policy must select source within the checked module")
		}
	}
	r = Analyze(program, policy)
	if ctx.Err() != nil {
		return incomplete("analysis cancelled")
	}
	for name, hash := range before {
		data, err := os.ReadFile(name)
		if err != nil {
			return incomplete("source disappeared during analysis")
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != hash {
			return incomplete("source changed during analysis")
		}
		rel, _ := filepath.Rel(abs, name)
		r.Sources = append(r.Sources, Source{Path: filepath.ToSlash(rel), SHA256: hash})
	}
	r.Build = map[string]string{"module": module, "go": runtime.Version(), "goos": envOr("GOOS", runtime.GOOS), "goarch": envOr("GOARCH", runtime.GOARCH), "tags": options.Tags, "workspace": "off", "scope": "one-module-active-build", "dependencies": "cached-readonly"}
	r.finish()
	return r
}
func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func loadEnvironment() []string {
	vars := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			vars[key] = value
		}
	}
	for _, k := range []string{"GOFLAGS", "GOWORK", "GOENV", "GOTOOLCHAIN", "GOPACKAGESDRIVER", "GOPROXY", "GOSUMDB", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOVCS"} {
		delete(vars, k)
	}
	for k, v := range map[string]string{"GOENV": "off", "GOTOOLCHAIN": "local", "GOWORK": "off", "GOFLAGS": "", "GO111MODULE": "on", "GOPACKAGESDRIVER": "off", "GOPROXY": "off", "GOSUMDB": "off", "GOVCS": "*:off", "GOROOT": runtime.GOROOT()} {
		vars[k] = v
	}
	vars["PATH"] = filepath.Join(runtime.GOROOT(), "bin") + string(os.PathListSeparator) + vars["PATH"]
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		result = append(result, k+"="+vars[k])
	}
	return result
}
