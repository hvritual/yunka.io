// Package sourceaudit inventories repository source independently of Go's package
// wildcards and checks explicit package boundaries over a declared build matrix.
// It is a CLI-internal static tool, not an application runtime or a type auditor.
package sourceaudit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	SchemaVersion = 1
	Pass          = "PASS"
	Fail          = "FAIL"
	Incomplete    = "INCOMPLETE"
	Proven        = "PROVEN_VIOLATION"
	Unknown       = "INCOMPLETE"
)

type Profile struct {
	Name   string   `json:"name"`
	GOOS   string   `json:"goos"`
	GOARCH string   `json:"goarch"`
	CGO    bool     `json:"cgo"`
	Tags   []string `json:"tags"`
}

type ModulePolicy struct {
	Kind      string   `json:"kind,omitempty"` // source (default) or explicit manifest-only aggregator
	Path      string   `json:"path"`
	Workspace string   `json:"workspace"` // "off" or an exact root-relative go.work
	Profiles  []string `json:"profiles"`
}

type Component struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Kind          string   `json:"kind"`  // production or test-support; never guessed from a name
	Allow         []string `json:"allow"` // same-component imports are always allowed
	AllowExternal bool     `json:"allowExternal"`
	DenyImports   []string `json:"denyImports,omitempty"` // exact package or package subtree
}

type Exclusion struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // fixture or dependency
	Reason string `json:"reason"`
}

type Policy struct {
	SchemaVersion      int            `json:"schemaVersion"`
	Profiles           []Profile      `json:"profiles"`
	Modules            []ModulePolicy `json:"modules"`
	Components         []Component    `json:"components"`
	Exclusions         []Exclusion    `json:"exclusions,omitempty"`
	TestSupportImports []string       `json:"testSupportImports,omitempty"`
}

type File struct {
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Kind        string   `json:"kind"`             // production, test, manifest, fixture, dependency
	Module      string   `json:"module,omitempty"` // physical root-relative module directory
	Component   string   `json:"component,omitempty"`
	Generated   bool     `json:"generated,omitempty"` // observation, never an exemption
	Profiles    []string `json:"profiles,omitempty"`
	Disposition string   `json:"disposition"`
	Reason      string   `json:"reason,omitempty"`
}

type Module struct {
	Path        string `json:"path"`
	Identity    string `json:"identity,omitempty"`
	Disposition string `json:"disposition"`
}

type LocalReference struct {
	File          string `json:"file"`
	Kind          string `json:"kind"`
	Module        string `json:"module,omitempty"`
	Version       string `json:"version,omitempty"`
	Target        string `json:"target"`
	TargetVersion string `json:"targetVersion,omitempty"`
	Disposition   string `json:"disposition"`
}

type Finding struct {
	Rule    string `json:"rule"`
	Class   string `json:"class"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Profile string `json:"profile,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Message string `json:"message"`
}

type ProfileResult struct {
	Module   string   `json:"module"`
	Profile  string   `json:"profile"`
	Status   string   `json:"status"`
	Packages int      `json:"packages"`
	Files    int      `json:"files"`
	Rules    []string `json:"rules"`
}

type Inventory struct {
	Complete  bool   `json:"complete"`
	Files     int    `json:"files"` // all regular files except VCS metadata
	GoFiles   int    `json:"goFiles"`
	Manifests int    `json:"manifests"`
	Bytes     int64  `json:"bytes"`
	Digest    string `json:"digest"` // all copied input paths, modes and bytes
}

type Analysis struct {
	Complete          bool `json:"complete"`
	RequiredProfiles  int  `json:"requiredProfiles"`
	CompletedProfiles int  `json:"completedProfiles"`
	CheckedGoFiles    int  `json:"checkedGoFiles"`
	ExcludedGoFiles   int  `json:"excludedGoFiles"`
	UncoveredGoFiles  int  `json:"uncoveredGoFiles"`
}

type Report struct {
	SchemaVersion   int              `json:"schemaVersion"`
	Status          string           `json:"status"`
	Backend         string           `json:"backend"`
	Toolchain       string           `json:"toolchain,omitempty"`
	PolicySHA256    string           `json:"policySHA256"`
	Inventory       Inventory        `json:"inventory"`
	Analysis        Analysis         `json:"analysis"`
	SourceUnchanged bool             `json:"sourceUnchanged"`
	Files           []File           `json:"files"`
	Modules         []Module         `json:"modules"`
	References      []LocalReference `json:"references"`
	Profiles        []ProfileResult  `json:"profiles"`
	Findings        []Finding        `json:"findings"`
}

func (r Report) Error() error {
	if r.Status == Pass {
		return nil
	}
	return fmt.Errorf("source audit %s: %d findings", r.Status, len(r.Findings))
}

func (r Report) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	return append(b, '\n'), err
}

func (r *Report) add(rule, class, file, profile, from, to, message string) {
	r.Findings = append(r.Findings, Finding{Rule: rule, Class: class, File: file, Profile: profile, From: from, To: to, Message: message})
}

func (r *Report) finish() {
	r.Status = Pass
	r.Analysis.Complete = r.Inventory.Complete && r.SourceUnchanged
	r.Analysis.CompletedProfiles = 0
	for _, p := range r.Profiles {
		if p.Status != Incomplete {
			r.Analysis.CompletedProfiles++
		}
	}
	if r.Analysis.CompletedProfiles != r.Analysis.RequiredProfiles {
		r.Analysis.Complete = false
	}
	r.Analysis.CheckedGoFiles, r.Analysis.ExcludedGoFiles, r.Analysis.UncoveredGoFiles = 0, 0, 0
	for i := range r.Files {
		f := &r.Files[i]
		sort.Strings(f.Profiles)
		switch f.Disposition {
		case "checked":
			r.Analysis.CheckedGoFiles++
		case "excluded":
			if strings.HasSuffix(f.Path, ".go") {
				r.Analysis.ExcludedGoFiles++
			}
		case "uncovered":
			r.Analysis.UncoveredGoFiles++
		}
	}
	for _, f := range r.Findings {
		if f.Class == Unknown {
			r.Analysis.Complete = false
		}
		if f.Class == Proven {
			r.Status = Fail
		}
	}
	if !r.Analysis.Complete {
		r.Status = Incomplete
	}
	sort.Slice(r.Files, func(i, j int) bool { return r.Files[i].Path < r.Files[j].Path })
	sort.Slice(r.Modules, func(i, j int) bool { return r.Modules[i].Path < r.Modules[j].Path })
	sort.Slice(r.Profiles, func(i, j int) bool {
		a, b := r.Profiles[i], r.Profiles[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		return a.Profile < b.Profile
	})
	sort.Slice(r.References, func(i, j int) bool {
		a, _ := json.Marshal(r.References[i])
		b, _ := json.Marshal(r.References[j])
		return string(a) < string(b)
	})
	sort.Slice(r.Findings, func(i, j int) bool {
		a, _ := json.Marshal(r.Findings[i])
		b, _ := json.Marshal(r.Findings[j])
		return string(a) < string(b)
	})
	// Repeated dependency metadata must not create repeated diagnostics.
	out := r.Findings[:0]
	last := ""
	for _, f := range r.Findings {
		b, _ := json.Marshal(f)
		if string(b) != last {
			out = append(out, f)
			last = string(b)
		}
	}
	r.Findings = out
}
