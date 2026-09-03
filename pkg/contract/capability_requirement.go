package contract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	applicationCapabilityNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	applicationCapabilityTypePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
)

func normalizeCapabilityRequirement(value CapabilityRequirement) CapabilityRequirement {
	value.Name = strings.TrimSpace(value.Name)
	value.Package = strings.TrimSpace(value.Package)
	value.Type = strings.TrimSpace(value.Type)
	return value
}

func normalizeCapabilityRequirements(values []CapabilityRequirement) []CapabilityRequirement {
	result := make([]CapabilityRequirement, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = normalizeCapabilityRequirement(value)
		if value.Name == "" && value.Package == "" && value.Type == "" {
			continue
		}
		key := value.Name + "\x00" + value.Package + "\x00" + value.Type
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
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

func validateCapabilityRequirement(application string, value CapabilityRequirement) error {
	value = normalizeCapabilityRequirement(value)
	if !applicationCapabilityNamePattern.MatchString(value.Name) {
		return fmt.Errorf("contract: application %s has invalid capability name %q", application, value.Name)
	}
	if value.Package == "" || strings.ContainsAny(value.Package, " \t\r\n") || strings.HasPrefix(value.Package, ".") || strings.HasSuffix(value.Package, "/") {
		return fmt.Errorf("contract: application %s capability %q has invalid Go package %q", application, value.Name, value.Package)
	}
	if !applicationCapabilityTypePattern.MatchString(value.Type) {
		return fmt.Errorf("contract: application %s capability %q has invalid Go type %q", application, value.Name, value.Type)
	}
	return nil
}
