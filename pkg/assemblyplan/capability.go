package assemblyplan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	capabilityNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	capabilityTypePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
)

// CapabilityRequirement is a compiler-visible Application constructor
// dependency. It carries only the stable logical key and Go contract identity;
// runtime provider values are resolved later by the App-scoped module catalog.
type CapabilityRequirement struct {
	Name     string   `json:"name"`
	Package  string   `json:"package"`
	Type     string   `json:"type"`
	Evidence Evidence `json:"evidence"`
}

func normalizeCapabilityRequirements(values []CapabilityRequirement) []CapabilityRequirement {
	result := append([]CapabilityRequirement(nil), values...)
	for index := range result {
		result[index].Name = strings.TrimSpace(result[index].Name)
		result[index].Package = strings.TrimSpace(result[index].Package)
		result[index].Type = strings.TrimSpace(result[index].Type)
		result[index].Evidence = normalizeEvidence(result[index].Evidence)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		return result[i].Type < result[j].Type
	})
	return result
}

func validateCapabilityRequirements(application string, values []CapabilityRequirement) error {
	seen := make(map[string]CapabilityRequirement, len(values))
	for _, value := range normalizeCapabilityRequirements(values) {
		if !capabilityNamePattern.MatchString(value.Name) {
			return fmt.Errorf("assemblyplan: application %s has invalid capability name %q", application, value.Name)
		}
		if value.Package == "" || strings.ContainsAny(value.Package, " \t\r\n") || strings.HasPrefix(value.Package, ".") || strings.HasSuffix(value.Package, "/") {
			return fmt.Errorf("assemblyplan: application %s capability %q has invalid package %q", application, value.Name, value.Package)
		}
		if !capabilityTypePattern.MatchString(value.Type) {
			return fmt.Errorf("assemblyplan: application %s capability %q has invalid type %q", application, value.Name, value.Type)
		}
		if previous, duplicate := seen[value.Name]; duplicate {
			if previous.Package == value.Package && previous.Type == value.Type {
				return fmt.Errorf("assemblyplan: application %s has duplicate capability %q", application, value.Name)
			}
			return fmt.Errorf("assemblyplan: application %s capability %q has conflicting contracts", application, value.Name)
		}
		seen[value.Name] = value
		if err := validateEvidence(value.Evidence, "application capability "+application+"/"+value.Name); err != nil {
			return err
		}
	}
	return nil
}
