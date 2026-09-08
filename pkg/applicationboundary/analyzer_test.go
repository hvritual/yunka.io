package applicationboundary

import (
	"golang.org/x/tools/go/analysis/analysistest"
	"testing"
)

func TestAnalysisExactDiagnostics(t *testing.T) {
	p := Policy{SchemaVersion: 1}
	for _, name := range []string{"Good", "Wide", "Concrete", "Dynamic", "Embedded", "State", "Accept"} {
		f := Factory{Symbol: Symbol{"shapes", name}, AllowedCallers: []string{"shapes"}, Results: []Slot{{0, Symbol{"shapes", "Reader"}}}}
		if name == "Accept" {
			f.Arguments = []Slot{{0, Symbol{"shapes", "Reader"}}}
		}
		p.Factories = append(p.Factories, f)
	}
	results := analysistest.Run(t, analysistest.TestData(), NewAnalyzer(p), "shapes")
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("analysis did not execute: %+v", results)
	}
	report, ok := results[0].Result.(Report)
	if !ok || report.Status != Incomplete || len(report.Findings) != 6 {
		t.Fatalf("unexpected exact diagnostic inventory: %+v", results[0].Result)
	}
}
