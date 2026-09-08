// Package applicationboundary checks explicitly selected Go capability boundaries.
// It is a read-only development tool, not an execution/security boundary.
package applicationboundary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

const SchemaVersion = 1
const (
	Pass            = "PASS"
	Fail            = "FAIL"
	Incomplete      = "INCOMPLETE"
	ProvenViolation = "proven_violation"
	Unknown         = "incomplete"
)

// Symbol is a Go package-level identity, never a variable spelling or path glob.
// Receiver methods are not selectable factories in this version.
type Symbol struct {
	Package string `json:"package"`
	Name    string `json:"name"`
}

func (s Symbol) String() string { return s.Package + "." + s.Name }

type Slot struct {
	Index    int    `json:"index"`
	Contract Symbol `json:"contract"`
}
type Factory struct {
	Symbol         Symbol   `json:"symbol"`
	AllowedCallers []string `json:"allowedCallerPackages"`
	Results        []Slot   `json:"results,omitempty"`
	Arguments      []Slot   `json:"arguments,omitempty"`
}
type Policy struct {
	SchemaVersion int       `json:"schemaVersion"`
	Factories     []Factory `json:"factories"`
}

// SourcePackage must contain complete type information for the active build.
// Imported constructor bodies can be supplied as additional SourcePackages.
type SourcePackage struct {
	Types *types.Package
	Info  *types.Info
	Files []*ast.File
}
type Program struct {
	Fset     *token.FileSet
	Packages []SourcePackage
	Root     string
}

type Finding struct {
	Rule     string    `json:"rule"`
	Class    string    `json:"class"`
	Subject  string    `json:"subject"`
	File     string    `json:"file,omitempty"`
	Line     int       `json:"line,omitempty"`
	Column   int       `json:"column,omitempty"`
	Message  string    `json:"message"`
	Position token.Pos `json:"-"`
}
type Source struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type ExcludedPackage struct {
	Package string `json:"package"`
	Reason  string `json:"reason"`
}
type Report struct {
	ExcludedPackages  []ExcludedPackage `json:"excludedPackages,omitempty"`
	SchemaVersion     int               `json:"schemaVersion"`
	Status            string            `json:"status"`
	PolicySHA256      string            `json:"policySha256"`
	Packages          []string          `json:"packages"`
	Sources           []Source          `json:"sources"`
	Build             map[string]string `json:"build,omitempty"`
	CheckedFactories  int               `json:"checkedFactories"`
	CheckedReferences int               `json:"checkedReferences"`
	Findings          []Finding         `json:"findings"`
}

func newReport(policy Policy) Report {
	b, _ := json.Marshal(policy)
	h := sha256.Sum256(b)
	return Report{SchemaVersion: SchemaVersion, Status: Pass, PolicySHA256: hex.EncodeToString(h[:]), Packages: []string{}, Sources: []Source{}, Findings: []Finding{}}
}
func (r *Report) finish() {
	sort.Strings(r.Packages)
	sort.Slice(r.ExcludedPackages, func(i, j int) bool { return r.ExcludedPackages[i].Package < r.ExcludedPackages[j].Package })
	sort.Slice(r.Sources, func(i, j int) bool { return r.Sources[i].Path < r.Sources[j].Path })
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
	unique := r.Findings[:0]
	for _, f := range r.Findings {
		if len(unique) > 0 {
			p := unique[len(unique)-1]
			if p.Rule == f.Rule && p.Class == f.Class && p.Subject == f.Subject && p.File == f.File && p.Line == f.Line && p.Column == f.Column && p.Message == f.Message {
				continue
			}
		}
		unique = append(unique, f)
	}
	r.Findings = unique
	r.Status = Pass
	for _, f := range r.Findings {
		if f.Class == Unknown {
			r.Status = Incomplete
			return
		}
		r.Status = Fail
	}
}
func (r Report) Marshal() ([]byte, error) {
	b, e := json.MarshalIndent(r, "", "  ")
	return append(b, '\n'), e
}
func (r Report) Error() error {
	if r.Status != Pass {
		return fmt.Errorf("typed audit: %s (%d findings)", r.Status, len(r.Findings))
	}
	return nil
}
