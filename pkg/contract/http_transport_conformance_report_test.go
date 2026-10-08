package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const conformanceCaseMarker = "CONFORMANCE_CASE:"

// A successful result denotes equivalence of the declared Operation semantic
// dimension, not equality of transport-specific status codes or wire bytes.
// Nil comparison fields explicitly mean "not applicable": for example, a
// malformed URL query cannot be encoded as a typed gRPC request.
type transportConformanceCase struct {
	Case               string `json:"case"`
	Supported          bool   `json:"supported"`
	RequestEquivalent  *bool  `json:"request-equivalent"`
	ResponseEquivalent *bool  `json:"response-equivalent"`
	HTTPStatus         int    `json:"httpStatus,omitempty"`
	GRPCStatus         string `json:"grpcStatus"`
	Diagnostic         string `json:"diagnostic"`
}

type transportConformanceReport struct {
	SchemaVersion        int                        `json:"schemaVersion"`
	CandidateSHA         string                     `json:"candidateSHA"`
	CandidateTree        string                     `json:"candidateTree"`
	ProtobufSHA256       string                     `json:"protobufSHA256"`
	DescriptorSHA256     string                     `json:"descriptorSHA256"`
	GeneratedSHA256      string                     `json:"generatedSHA256"`
	GeneratedFileCount   int                        `json:"generatedFileCount"`
	CaseCount            int                        `json:"caseCount"`
	Cases                []transportConformanceCase `json:"cases"`
	Summary              string                     `json:"summary"`
}

// Required identities are generic behavioral classes, not Biz-specific
// Operation IDs. A missing or renamed test case blocks artifact acceptance.
var requiredConformanceCases = map[string]bool{
	"get/camel-repeated-duplicate-empty": true,
	"get/snake-repeated-two":            true,
	"get/omitted-values":                true,
	"get/scalars-and-base64":            true,
	"get/path-owns-resource":            true,
	"get/empty-singular":                true,
	"post/body-with-path-override":      true,
	"invalid/mixed-alias":               true,
	"invalid/singular-duplicate":        true,
	"invalid/overflow":                  true,
	"invalid/illegal-encoding":          true,
	"invalid/empty-numeric":             true,
	"invalid/invalid-bool":              true,
	"validation/unknown-code":           true,
	"validation/129-values":             true,
	"status/not-found":                  true,
	"status/aborted":                    true,
	"status/already-exists":             true,
	"status/internal":                   true,
	"status/permission-denied":          true,
	"status/ordinary-error":             true,
	"auth/unauthenticated":              true,
	"auth/foreign-tenant":               true,
	"idempotency/missing-key":           true,
	"idempotency/in-progress":           true,
	"idempotency/completed":             true,
	"framework/executor-unavailable":    true,
	"unsupported/enum":                  false,
	"unsupported/message":               false,
	"unsupported/map":                   false,
	"unsupported/oneof":                 false,
	"unsupported/repeated-path":         false,
	"unsupported/named-body":            false,
}

func readConformanceRows(runLog string) ([]transportConformanceCase, error) {
	var cases []transportConformanceCase
	for _, line := range strings.Split(runLog, "\n") {
		start := strings.Index(line, conformanceCaseMarker)
		if start < 0 {
			continue
		}
		var record transportConformanceCase
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[start+len(conformanceCaseMarker):])), &record); err != nil {
			return nil, fmt.Errorf("decode generated transport case: %w", err)
		}
		cases = append(cases, record)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("generated protocol runtime produced no machine-readable conformance cases")
	}
	return cases, nil
}

func expectedSHA(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func validateConformanceReport(report transportConformanceReport) error {
	if report.SchemaVersion != 1 ||
		!expectedSHA(report.CandidateSHA, 40) ||
		!expectedSHA(report.CandidateTree, 40) ||
		!expectedSHA(report.ProtobufSHA256, 64) ||
		!expectedSHA(report.DescriptorSHA256, 64) ||
		!expectedSHA(report.GeneratedSHA256, 64) ||
		report.GeneratedFileCount < 1 || report.CaseCount != len(report.Cases) ||
		report.CaseCount != len(requiredConformanceCases) ||
		strings.TrimSpace(report.Summary) == "" {
		return fmt.Errorf("conformance: incomplete or unbound report identity/count")
	}
	seen := make(map[string]bool, len(report.Cases))
	for _, item := range report.Cases {
		supported, ok := requiredConformanceCases[item.Case]
		if !ok || seen[item.Case] || item.Supported != supported {
			return fmt.Errorf("conformance: unexpected, duplicate or reclassified case %q", item.Case)
		}
		seen[item.Case] = true
		if item.Diagnostic == "" || strings.Contains(item.Diagnostic, "private-") {
			return fmt.Errorf("conformance: case %q has missing/unsafe diagnostic", item.Case)
		}
		if !supported {
			if item.HTTPStatus != 0 || item.GRPCStatus != "not-applicable" ||
				item.RequestEquivalent != nil || item.ResponseEquivalent != nil ||
				!strings.Contains(item.Diagnostic, "UNSUPPORTED_HTTP_BINDING") {
				return fmt.Errorf("conformance: unsupported case %q was promoted to runtime support", item.Case)
			}
			continue
		}
		if item.HTTPStatus < 100 || item.HTTPStatus > 599 || item.GRPCStatus == "" {
			return fmt.Errorf("conformance: supported case %q has no observed transport result", item.Case)
		}
		if item.RequestEquivalent != nil && !*item.RequestEquivalent ||
			item.ResponseEquivalent != nil && !*item.ResponseEquivalent {
			return fmt.Errorf("conformance: semantic mismatch in %q", item.Case)
		}
		if strings.HasPrefix(item.Case, "invalid/") {
			if item.HTTPStatus != 400 || item.RequestEquivalent != nil ||
				item.ResponseEquivalent != nil || item.GRPCStatus != "not-applicable" {
				return fmt.Errorf("conformance: malformed URL case %q must be rejected before Application", item.Case)
			}
		} else if item.ResponseEquivalent == nil || !*item.ResponseEquivalent {
			return fmt.Errorf("conformance: case %q has no proven normalized response equivalence", item.Case)
		}
	}
	return nil
}

func conformanceGitIdentity(root, revision string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", revision)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("conformance: exact Git revision %s: %w: %s", revision, err, out)
	}
	value := strings.TrimSpace(string(out))
	if !expectedSHA(value, 40) {
		return "", fmt.Errorf("conformance: invalid exact revision %q", value)
	}
	return value, nil
}

func conformanceGeneratedFingerprint(files []GeneratedApplicationFile) string {
	digest := sha256.New()
	for _, file := range files {
		fmt.Fprintf(digest, "%s\x00%d\x00", file.Path, len(file.Content))
		_, _ = digest.Write(file.Content)
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// Compiler rejection is recorded as unsupported, not as a false claim of
// HTTP/gRPC runtime parity. Every diagnostic is derived by exercising the
// actual URL compiler, not from an independently maintained capability table.
func unsupportedConformanceCases() ([]transportConformanceCase, error) {
	tests := []struct {
		name    string
		field   Field
		binding HTTPBinding
	}{
		{"enum", Field{Name: "state", JSONName: "state", Kind: "enum", Type: "echo.v1.State"}, HTTPBinding{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
		{"message", Field{Name: "filter", JSONName: "filter", Kind: "message", Type: "echo.v1.Response"}, HTTPBinding{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
		{"map", Field{Name: "lookup", JSONName: "lookup", Kind: "map", Map: true, MapKeyType: "string", MapValueKind: "scalar", MapValueType: "string"}, HTTPBinding{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
		{"oneof", Field{Name: "choice", JSONName: "choice", Kind: "scalar", Type: "string", Oneof: true}, HTTPBinding{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
		{"repeated-path", Field{Name: "tenant_id", JSONName: "tenantId", Kind: "scalar", Type: "string", Repeated: true}, HTTPBinding{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
		{"named-body", Field{Name: "value", JSONName: "value", Kind: "scalar", Type: "string"}, HTTPBinding{Method: "POST", Path: "/v1/echo/{tenant_id}", Body: "value"}},
	}
	records := make([]transportConformanceCase, 0, len(tests))
	for _, sample := range tests {
		fields := []Field{httpTenantField()}
		if sample.name == "repeated-path" {
			fields = []Field{sample.field}
		} else {
			sample.field.Number = 2
			fields = append(fields, sample.field)
		}
		m := httpBindingManifest(fields...)
		method := m.Services[0].Methods[0]
		_, err := compileHTTPBindingPlan(method, sample.binding, messageIndex(m))
		if err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_HTTP_BINDING") ||
			!strings.Contains(err.Error(), "echo.read") || !strings.Contains(err.Error(), sample.binding.Path) {
			return nil, fmt.Errorf("conformance: unsupported compiler negative %s not accurately detected: %v", sample.name, err)
		}
		records = append(records, transportConformanceCase{
			Case:       "unsupported/" + sample.name,
			Supported:  false,
			GRPCStatus: "not-applicable",
			Diagnostic: err.Error(),
		})
	}
	return records, nil
}

func conformanceReportBytes(report transportConformanceReport) ([]byte, error) {
	sort.Slice(report.Cases, func(i, j int) bool { return report.Cases[i].Case < report.Cases[j].Case })
	if err := validateConformanceReport(report); err != nil {
		return nil, err
	}
	return json.MarshalIndent(report, "", "  ")
}

func saveConformanceReport(t *testing.T, report transportConformanceReport) {
	t.Helper()
	data, err := conformanceReportBytes(report)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("canonical generated transport conformance: %s\n%s", report.Summary, data)
	dir := os.Getenv("YUNKA_CONFORMANCE_EVIDENCE_DIR")
	if dir == "" && os.Getenv("RUNNER_TEMP") != "" {
		dir = filepath.Join(os.Getenv("RUNNER_TEMP"), "yunka-protocol-conformance")
	}
	if dir == "" {
		return // local dev: -v exposes the exact report in the test log
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "matrix.json")
	if err := os.WriteFile(output, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("machine-readable protocol qualification: %s", output)
}

func TestConformanceReportRejectsIncompleteOrForgedEvidence(t *testing.T) {
	truth := true
	hash40 := strings.Repeat("a", 40)
	hash64 := strings.Repeat("b", 64)
	report := transportConformanceReport{
		SchemaVersion: 1, CandidateSHA: hash40, CandidateTree: hash40,
		ProtobufSHA256: hash64, DescriptorSHA256: hash64,
		GeneratedSHA256: hash64, GeneratedFileCount: 1,
		Summary: "unit report validator", CaseCount: len(requiredConformanceCases),
	}
	for name, supported := range requiredConformanceCases {
		item := transportConformanceCase{Case: name, Supported: supported,
			Diagnostic: "semantic check completed", GRPCStatus: "OK", HTTPStatus: 200,
			ResponseEquivalent: &truth, RequestEquivalent: &truth}
		if !supported {
			item.HTTPStatus = 0
			item.GRPCStatus = "not-applicable"
			item.ResponseEquivalent = nil
			item.RequestEquivalent = nil
			item.Diagnostic = "UNSUPPORTED_HTTP_BINDING: echo.read /v1/test"
		} else if strings.HasPrefix(name, "invalid/") {
			item.HTTPStatus = 400
			item.GRPCStatus = "not-applicable"
			item.ResponseEquivalent = nil
			item.RequestEquivalent = nil
		}
		report.Cases = append(report.Cases, item)
	}
	if err := validateConformanceReport(report); err != nil {
		t.Fatalf("valid reference evidence rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*transportConformanceReport)
	}{
		{"missing case", func(r *transportConformanceReport) { r.Cases = r.Cases[:len(r.Cases)-1]; r.CaseCount-- }},
		{"duplicate case", func(r *transportConformanceReport) { r.Cases[1] = r.Cases[0] }},
		{"falsified SHA", func(r *transportConformanceReport) { r.CandidateSHA = "main" }},
		{"falsified generated digest", func(r *transportConformanceReport) { r.GeneratedSHA256 = "unknown" }},
		{"false equivalence", func(r *transportConformanceReport) {
			for i := range r.Cases { if r.Cases[i].ResponseEquivalent != nil { bad := false; r.Cases[i].ResponseEquivalent = &bad; break } }
		}},
		{"unsupported promoted", func(r *transportConformanceReport) {
			for i := range r.Cases { if !r.Cases[i].Supported { r.Cases[i].Supported = true; break } }
		}},
		{"unsafe diagnostics", func(r *transportConformanceReport) { r.Cases[0].Diagnostic = "private-resource-must-not-leak" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := report
			candidate.Cases = append([]transportConformanceCase(nil), report.Cases...)
			test.mutate(&candidate)
			if err := validateConformanceReport(candidate); err == nil {
				t.Fatal("tampered/missing conformance evidence was accepted")
			}
		})
	}
}

func TestConformanceLogParserRejectsMissingOrMalformedCases(t *testing.T) {
	for _, source := range []string{"plain Go test output", "CONFORMANCE_CASE:{not-json}"} {
		if _, err := readConformanceRows(source); err == nil {
			t.Fatalf("malformed fixture log %q was accepted", source)
		}
	}
	payload := `CONFORMANCE_CASE:{"case":"get/omitted-values","supported":true,"request-equivalent":true,"response-equivalent":true,"httpStatus":200,"grpcStatus":"OK","diagnostic":"decoded canonical values"}`
	rows, err := readConformanceRows("    transport_test.go:21: " + payload + "\n")
	if err != nil || len(rows) != 1 || rows[0].Case != "get/omitted-values" {
		t.Fatalf("actual -v output parsing failed: %v %#v", err, rows)
	}
}
