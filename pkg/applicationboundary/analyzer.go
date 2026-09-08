package applicationboundary

import (
	"golang.org/x/tools/go/analysis"
	"reflect"
)

// NewAnalyzer exposes the same core checks to go/analysis and analysistest.
// A pass only supplies its own package body: constructor bodies in other packages
// remain incomplete here. Check uses go/packages to supply the complete selected
// module; callers must not mistake a single pass for module-wide qualification.
func NewAnalyzer(policy Policy) *analysis.Analyzer {
	return &analysis.Analyzer{Name: "applicationboundary", ResultType: reflect.TypeOf(Report{}), Doc: "checks explicit factory and actual capability type boundaries", Run: func(pass *analysis.Pass) (any, error) {
		if err := policy.Validate(); err != nil {
			return nil, err
		}
		report := Analyze(Program{Fset: pass.Fset, Packages: []SourcePackage{{Types: pass.Pkg, Info: pass.TypesInfo, Files: pass.Files}}}, policy)
		for _, f := range report.Findings {
			pos := f.Position
			if !pos.IsValid() && len(pass.Files) > 0 {
				pos = pass.Files[0].Package
			}
			pass.Reportf(pos, "%s [%s]: %s", f.Rule, f.Class, f.Message)
		}
		return report, nil
	}}
}
