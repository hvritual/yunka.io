package add

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
	"github.com/urfave/cli"
)

func validPolicyKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for i, current := range value {
		if current >= 'a' && current <= 'z' {
			continue
		}
		if current >= '0' && current <= '9' && i > 0 {
			continue
		}
		if (current == '.' || current == '_' || current == '-' || current == ':') && i > 0 {
			continue
		}
		return false
	}
	return true
}

func normalizeChoice(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_")
}

func normalizedChoices(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeChoice(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return stableStrings(result)
}

func normalizedKeys(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return stableStrings(result)
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func permissionModeEnum(value string) string {
	if value == "any" {
		return "PERMISSION_ANY"
	}
	return "PERMISSION_ALL"
}

func authenticationEnum(value string) string {
	switch value {
	case "api-key":
		return "AUTHENTICATION_API_KEY"
	case "service-token":
		return "AUTHENTICATION_SERVICE"
	default:
		return "AUTHENTICATION_JWT"
	}
}

func compositionEnum(value string) string {
	if value == "remote_saga" {
		return "COMPOSITION_REMOTE_SAGA"
	}
	return "COMPOSITION_LOCAL"
}

func transactionEnum(value string) string {
	switch value {
	case "read_only":
		return "TRANSACTION_READ_ONLY"
	case "local":
		return "TRANSACTION_LOCAL"
	default:
		return "TRANSACTION_NONE"
	}
}

func idempotencyEnum(value string) string {
	if value == "required" {
		return "IDEMPOTENCY_REQUIRED"
	}
	return "IDEMPOTENCY_NONE"
}

func operationFileStem(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	lastUnderscore := false
	for _, current := range value {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			builder.WriteRune(unicode.ToLower(current))
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && builder.Len() > 0 {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func renderImplementationLanding(domain, application, operationID, rpcName string) string {
	return fmt.Sprintf(`// Scaffolded by yunka add operation. Developer-owned; safe to edit.
//
// Operation: %s
// Application: %s/%s
// Generated interface method: %s
//
// TODO(agent): after running yunka generate, implement the generated Application
// interface in developer-owned code. Do not edit zz_yunka_* generated files.
// Business rules, persistence, authorization decisions, transaction boundaries,
// idempotency behavior, Saga/Outbox behavior, event publication, and external
// effects must follow the explicit contract and application requirements.
package application
`, operationID, domain, application, rpcName)
}

func writeAtomic(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".yunka-add-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func writeNewFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func cleanRelative(value string) string {
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if value == "." || value == "" {
		return ""
	}
	return strings.TrimPrefix(value, "./")
}

func requestFailure(err error) error {
	return &Failure{Kind: FailureRequest, Err: err}
}

func sourceFailure(location string, err error) error {
	return &Failure{Kind: FailureSource, Location: cleanRelative(location), Err: err}
}

func ownershipFailure(location string, err error) error {
	return &Failure{Kind: FailureOwnership, Location: cleanRelative(location), Err: err}
}

func conflictFailure(location string, err error) error {
	return &Failure{Kind: FailureConflict, Location: cleanRelative(location), Err: err}
}

func Diagnose(err error) diagnostic.Diagnostic {
	var failure *Failure
	if !errors.As(err, &failure) {
		item := diagnostic.MustDefinition(diagnostic.CodeDeveloperWorkflowFailed).Diagnostic(diagnostic.SeverityError)
		item.Detail = strings.TrimSpace(err.Error())
		return item
	}
	code := diagnostic.CodeScaffoldRequest
	switch failure.Kind {
	case FailureSource:
		code = diagnostic.CodeScaffoldSource
	case FailureOwnership:
		code = diagnostic.CodeScaffoldOwnership
	case FailureConflict:
		code = diagnostic.CodeScaffoldConflict
	}
	item := diagnostic.MustDefinition(code).Diagnostic(diagnostic.SeverityError)
	item.Detail = strings.TrimSpace(failure.Err.Error())
	if failure.Location != "" {
		item.Location = &diagnostic.Location{Path: failure.Location}
	}
	return item
}

func Render(report Report, format string) (string, error) {
	normalizeReport(&report)
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON, FormatAgentJSON:
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	case "", FormatText:
		var builder strings.Builder
		fmt.Fprintf(&builder, "%s scaffold\n", report.Kind)
		keys := make([]string, 0, len(report.Identity))
		for key := range report.Identity {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "  %-16s %s\n", key, report.Identity[key])
		}
		builder.WriteString("mutations:\n")
		for _, mutation := range report.Mutations {
			fmt.Fprintf(&builder, "  %-8s %-20s %s\n", mutation.Action, mutation.Owner, mutation.Path)
		}
		if len(report.NextActions) > 0 {
			builder.WriteString("next:\n")
			for _, action := range report.NextActions {
				fmt.Fprintf(&builder, "  %s — %s\n", action.Command, action.Purpose)
			}
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("structural scaffold: unsupported format %q", format)
	}
}

func normalizeReport(report *Report) {
	if report.Mutations == nil {
		report.Mutations = []Mutation{}
	}
	if report.Effects == nil {
		report.Effects = []Effect{}
	}
	if report.NextActions == nil {
		report.NextActions = []NextAction{}
	}
	if report.Notes == nil {
		report.Notes = []string{}
	}
	if report.Identity == nil {
		report.Identity = map[string]string{}
	}
	sort.Slice(report.Mutations, func(i, j int) bool {
		if report.Mutations[i].Path != report.Mutations[j].Path {
			return report.Mutations[i].Path < report.Mutations[j].Path
		}
		return report.Mutations[i].Action < report.Mutations[j].Action
	})
	sort.Slice(report.Effects, func(i, j int) bool {
		if report.Effects[i].Stage != report.Effects[j].Stage {
			return report.Effects[i].Stage < report.Effects[j].Stage
		}
		if report.Effects[i].Path != report.Effects[j].Path {
			return report.Effects[i].Path < report.Effects[j].Path
		}
		return report.Effects[i].Scope < report.Effects[j].Scope
	})
}

func printFailure(command, format string, item diagnostic.Diagnostic, exitCode int) error {
	var output string
	var err error
	switch format {
	case FormatAgentJSON:
		var contents []byte
		contents, err = diagnostic.RenderAgentJSON(command, []diagnostic.Diagnostic{item}, false)
		output = string(contents)
	case FormatJSON:
		var contents []byte
		contents, err = diagnostic.RenderJSON(command, []diagnostic.Diagnostic{item})
		output = string(contents)
	default:
		output, err = diagnostic.RenderText([]diagnostic.Diagnostic{item})
	}
	if err != nil {
		return err
	}
	fmt.Print(output)
	return cli.NewExitError("", exitCode)
}

func shellQuote(value string) string {
	return strconv.Quote(strings.TrimSpace(value))
}
