package projectflow

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
)

// ImplementationLayout is a derived physical projection of one canonical
// Application. It is not persisted and does not own Application identity.
type ImplementationLayout struct {
	Root       string `json:"root"`
	Build      string `json:"build"`
	TypePolicy string `json:"typePolicy"`
	Handler    string `json:"handler,omitempty"`
}

// DescribeImplementationLayout derives the sealed-v1 owner paths from the
// canonical project descriptor and Application/Go method identities. The
// starter and change planner share this function so path conventions cannot
// drift into separate registries.
func DescribeImplementationLayout(project ProjectDescriptor, domain, application, method string) (ImplementationLayout, error) {
	domain = strings.TrimSpace(domain)
	application = strings.TrimSpace(application)
	method = strings.TrimSpace(method)
	for _, value := range []string{domain, application} {
		if value == "" || value == "internal" || value == "vendor" || strings.ContainsAny(value, "/\\") || module.CheckImportPath(value) != nil || !filepath.IsLocal(value) {
			return ImplementationLayout{}, fmt.Errorf("implementation layout: canonical Application segment %q cannot form a contained Go path", value)
		}
	}
	if strings.TrimSpace(project.GeneratedGoRoot) == "" || project.GeneratedGoRoot == "." || !filepath.IsLocal(project.GeneratedGoRoot) {
		return ImplementationLayout{}, fmt.Errorf("implementation layout: canonical generated Go root is not contained")
	}
	root := filepath.ToSlash(filepath.Join(project.GeneratedGoRoot, domain, "application", application))
	layout := ImplementationLayout{
		Root:       root,
		Build:      filepath.ToSlash(filepath.Join(root, "build.go")),
		TypePolicy: filepath.ToSlash(filepath.Join(root, "architecture.types.json")),
	}
	if method != "" {
		filename, err := ImplementationHandlerFilename(method)
		if err != nil {
			return ImplementationLayout{}, err
		}
		layout.Handler = filepath.ToSlash(filepath.Join(root, "internal", "usecase", filename))
	}
	return layout, nil
}

// ImplementationHandlerFilename is the one physical naming rule shared by the
// starter and change planner. Canonical method identity remains owned by the
// generated Application interface.
func ImplementationHandlerFilename(method string) (string, error) {
	method = strings.TrimSpace(method)
	if method == "" || strings.ContainsAny(method, "/\\") {
		return "", fmt.Errorf("implementation layout: canonical Application method %q is not a Go identifier", method)
	}
	return strings.ToLower(method) + "_handler.go", nil
}
