package implementation

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"sort"
	"strings"
)

// Planned paths alone cannot reveal a conflicting package in another file. Check
// every existing Go package clause in each planned package directory before any
// creation. Build-tagged files are also checked: unconditional starter files must
// coexist with each of them. External tests with the canonical _test package are
// valid. We neither execute nor rewrite any existing source.
func preflightPackages(root *os.Root, files []File) error {
	directories := map[string]string{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, parser.PackageClauseOnly)
		if err != nil {
			return err
		}
		dir := path.Dir(f.Path)
		if previous := directories[dir]; previous != "" && previous != parsed.Name.Name {
			return fmt.Errorf("add implementation: planned package conflict in %s", dir)
		}
		directories[dir] = parsed.Name.Name
	}
	var names []string
	for dir := range directories {
		names = append(names, dir)
	}
	sort.Strings(names)
	for _, dir := range names {
		opened, err := root.Open(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		entries, readErr := opened.ReadDir(-1)
		closeErr := opened.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			name := path.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("add implementation: non-regular existing Go file %s", name)
			}
			data, err := root.ReadFile(name)
			if err != nil {
				return err
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), name, data, parser.PackageClauseOnly)
			if err != nil {
				return fmt.Errorf("add implementation: existing Go package cannot be parsed at %s: %w", name, err)
			}
			want := directories[dir]
			actual := parsed.Name.Name
			if actual != want && !(strings.HasSuffix(entry.Name(), "_test.go") && actual == want+"_test") {
				return fmt.Errorf("add implementation: existing Go package %s in %s conflicts with planned package %s", actual, name, want)
			}
		}
	}
	return nil
}
