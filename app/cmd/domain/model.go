package domain

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	AppName      = "domain"
	ManifestName = "domain.json"
	SpecVersion  = 3
)

var (
	namePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	tablePrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

// Field is persistence-derived domain metadata. Protobuf numbering and
// transport metadata intentionally do not belong to the domain manifest.
type Field struct {
	Name    string `json:"name"`
	GoName  string `json:"-"`
	Type    string `json:"type"`
	Column  string `json:"column,omitempty"`
	POOwned bool   `json:"poOwned,omitempty"`
}

type ObjectSpec struct {
	Name         string  `json:"name"`
	GoName       string  `json:"-"`
	File         string  `json:"file"`
	TableName    string  `json:"tableName"`
	Fields       []Field `json:"fields,omitempty"`
	POEmbedsBase bool    `json:"poEmbedsBase,omitempty"`
}

// Legacy transport fields exist only so V1/V2 manifests can be read and
// deterministically upgraded. Canonical V3 manifests never write them.
type legacyRESTSpec struct {
	Enabled  bool   `json:"enabled"`
	Prefix   string `json:"prefix,omitempty"`
	BasePath string `json:"basePath,omitempty"`
}

type legacyRPCSpec struct {
	Enabled bool   `json:"enabled"`
	Service string `json:"service,omitempty"`
}

type Spec struct {
	Version      int          `json:"version"`
	Domain       string       `json:"domain"`
	TablePrefix  string       `json:"tablePrefix"`
	TenantScoped bool         `json:"tenantScoped"`
	Objects      []ObjectSpec `json:"objects,omitempty"`

	// Version-1/2 compatibility inputs. canonicalizeSpec always clears them.
	REST      *legacyRESTSpec `json:"rest,omitempty"`
	RPC       *legacyRPCSpec  `json:"rpc,omitempty"`
	Object    string          `json:"object,omitempty"`
	TableName string          `json:"tableName,omitempty"`
	Fields    []Field         `json:"fields,omitempty"`
}

type Options struct {
	Name        string
	Object      string
	Root        string
	TablePrefix string
	Fields      []string
	Global      bool
}

var reservedFields = map[string]struct{}{
	"id": {}, "tenant_id": {}, "version": {}, "created_at": {}, "updated_at": {}, "deleted_at": {},
}

func newDomainSpec(options Options, tablePrefix string) (Spec, string, error) {
	domainName := strings.TrimSpace(options.Name)
	if !namePattern.MatchString(domainName) {
		return Spec{}, "", fmt.Errorf("domain: name %q must match %s", domainName, namePattern)
	}
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "internal"
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Spec{}, "", err
	}
	if !tablePrefixPattern.MatchString(tablePrefix) {
		return Spec{}, "", fmt.Errorf("domain: table prefix %q must match %s", tablePrefix, tablePrefixPattern)
	}
	return Spec{
		Version:      SpecVersion,
		Domain:       domainName,
		TablePrefix:  tablePrefix,
		TenantScoped: !options.Global,
	}, absoluteRoot, nil
}

// normalizeOptions remains as a compatibility helper for tests/callers that
// construct a single object from flags. New generation scans developer POs.
func normalizeOptions(options Options) (Spec, string, error) {
	tablePrefix := strings.TrimSpace(options.TablePrefix)
	if tablePrefix == "" {
		tablePrefix = "yk"
	}
	spec, root, err := newDomainSpec(options, tablePrefix)
	if err != nil {
		return Spec{}, "", err
	}
	object := strings.TrimSpace(options.Object)
	if object == "" {
		object = spec.Domain
	}
	if !namePattern.MatchString(object) {
		return Spec{}, "", fmt.Errorf("domain: object %q must match %s", object, namePattern)
	}
	fields, err := parseFields(options.Fields)
	if err != nil {
		return Spec{}, "", err
	}
	spec.Objects = []ObjectSpec{{
		Name:      object,
		GoName:    exportedIdentifier(object),
		File:      snakeCase(object) + ".go",
		TableName: tableName(tablePrefix, spec.Domain, object),
		Fields:    fields,
	}}
	if err := validateSpec(spec); err != nil {
		return Spec{}, "", err
	}
	return spec, root, nil
}

func parseFields(values []string) ([]Field, error) {
	seen := make(map[string]struct{}, len(values))
	fields := make([]Field, 0, len(values))
	for _, raw := range values {
		parts := strings.Split(strings.TrimSpace(raw), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("domain: field %q must use name:type", raw)
		}
		name := strings.TrimSpace(parts[0])
		kind := strings.TrimSpace(parts[1])
		if !namePattern.MatchString(name) {
			return nil, fmt.Errorf("domain: field name %q must match %s", name, namePattern)
		}
		if _, reserved := reservedFields[name]; reserved {
			return nil, fmt.Errorf("domain: field %q is reserved by the PO/domain contract", name)
		}
		if _, ok := goType(kind); !ok {
			return nil, fmt.Errorf("domain: unsupported field type %q", kind)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("domain: duplicate field %q", name)
		}
		seen[name] = struct{}{}
		fields = append(fields, Field{Name: name, GoName: exportedIdentifier(name), Type: kind, Column: name, POOwned: true})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields, nil
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("domain: spec version %d is unsupported", spec.Version)
	}
	if !namePattern.MatchString(spec.Domain) {
		return errors.New("domain: invalid domain name")
	}
	if !tablePrefixPattern.MatchString(spec.TablePrefix) {
		return errors.New("domain: invalid table prefix")
	}
	if len(spec.Objects) == 0 {
		return errors.New("domain: at least one PO object is required")
	}
	seenObjects := map[string]struct{}{}
	seenTables := map[string]struct{}{}
	for _, object := range spec.Objects {
		if !namePattern.MatchString(object.Name) {
			return fmt.Errorf("domain: invalid object name %q", object.Name)
		}
		if _, duplicate := seenObjects[object.Name]; duplicate {
			return fmt.Errorf("domain: duplicate object %q", object.Name)
		}
		seenObjects[object.Name] = struct{}{}
		expectedFile := snakeCase(object.Name) + ".go"
		if object.File != expectedFile {
			return fmt.Errorf("domain: object %q PO file=%q want %q (snake_case)", object.Name, object.File, expectedFile)
		}
		expectedTable := tableName(spec.TablePrefix, spec.Domain, object.Name)
		if object.TableName != expectedTable {
			return fmt.Errorf("domain: object %q tableName=%q want %q", object.Name, object.TableName, expectedTable)
		}
		if _, duplicate := seenTables[object.TableName]; duplicate {
			return fmt.Errorf("domain: duplicate table %q", object.TableName)
		}
		seenTables[object.TableName] = struct{}{}
		if err := validateFields(object.Fields); err != nil {
			return fmt.Errorf("domain: object %s: %w", object.Name, err)
		}
	}
	return nil
}

func validateFields(fields []Field) error {
	seenNames := map[string]struct{}{}
	for _, field := range fields {
		if !namePattern.MatchString(field.Name) {
			return fmt.Errorf("invalid field name %q", field.Name)
		}
		if _, reserved := reservedFields[field.Name]; reserved {
			return fmt.Errorf("field %q is reserved", field.Name)
		}
		if _, ok := goType(field.Type); !ok {
			return fmt.Errorf("unsupported field type %q", field.Type)
		}
		if _, duplicate := seenNames[field.Name]; duplicate {
			return fmt.Errorf("duplicate field %q", field.Name)
		}
		seenNames[field.Name] = struct{}{}
		if field.Column == "" {
			return fmt.Errorf("field %q has no persistence column", field.Name)
		}
	}
	return nil
}

// upgradeSpec accepts historical V1/V2 manifests as migration inputs and
// projects them onto the persistence-only V3 model. New generation never
// writes transport or protobuf metadata back into domain.json.
func upgradeSpec(spec Spec) Spec {
	if spec.Version == SpecVersion {
		return spec
	}
	if spec.Version == 1 && len(spec.Objects) == 0 && spec.Object != "" {
		fields := append([]Field(nil), spec.Fields...)
		for i := range fields {
			if fields[i].Column == "" {
				fields[i].Column = fields[i].Name
			}
			if fields[i].GoName == "" {
				fields[i].GoName = exportedIdentifier(fields[i].Name)
			}
		}
		spec.Objects = []ObjectSpec{{
			Name:         spec.Object,
			GoName:       exportedIdentifier(spec.Object),
			File:         snakeCase(spec.Object) + ".go",
			TableName:    spec.TableName,
			Fields:       fields,
			POEmbedsBase: true,
		}}
	}
	if spec.Version == 1 || spec.Version == 2 {
		spec.Version = SpecVersion
	}
	return spec
}

func canonicalizeSpec(spec Spec) Spec {
	spec = upgradeSpec(spec)
	spec.REST = nil
	spec.RPC = nil
	spec.Object = ""
	spec.TableName = ""
	spec.Fields = nil
	sort.Slice(spec.Objects, func(i, j int) bool { return spec.Objects[i].Name < spec.Objects[j].Name })
	for objectIndex := range spec.Objects {
		if spec.Objects[objectIndex].GoName == "" {
			spec.Objects[objectIndex].GoName = exportedIdentifier(spec.Objects[objectIndex].Name)
		}
		fields := spec.Objects[objectIndex].Fields
		for i := range fields {
			if fields[i].GoName == "" {
				fields[i].GoName = exportedIdentifier(fields[i].Name)
			}
			if fields[i].Column == "" {
				fields[i].Column = fields[i].Name
			}
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		spec.Objects[objectIndex].Fields = fields
	}
	return spec
}

func tableName(prefix, domain, object string) string {
	return prefix + "_" + domain + "_" + object
}

func exportedIdentifier(value string) string {
	parts := strings.FieldsFunc(value, func(current rune) bool {
		return !unicode.IsLetter(current) && !unicode.IsDigit(current)
	})
	var builder strings.Builder
	for _, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		builder.WriteString(string(runes))
	}
	return builder.String()
}

func snakeCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	runes := []rune(value)
	for i, current := range runes {
		if current == '-' || current == ' ' || current == '.' || current == '_' {
			if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "_") {
				builder.WriteRune('_')
			}
			continue
		}
		if unicode.IsUpper(current) {
			previousLowerOrDigit := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			previousUpper := i > 0 && unicode.IsUpper(runes[i-1])
			if builder.Len() > 0 && (previousLowerOrDigit || (previousUpper && nextLower)) && !strings.HasSuffix(builder.String(), "_") {
				builder.WriteRune('_')
			}
			builder.WriteRune(unicode.ToLower(current))
			continue
		}
		builder.WriteRune(unicode.ToLower(current))
	}
	return strings.Trim(builder.String(), "_")
}

func goType(kind string) (string, bool) {
	switch kind {
	case "string":
		return "string", true
	case "int64":
		return "int64", true
	case "uint64":
		return "uint64", true
	case "bool":
		return "bool", true
	case "float64":
		return "float64", true
	case "time":
		return "time.Time", true
	default:
		return "", false
	}
}

func jsonName(value string) string {
	parts := strings.Split(value, "_")
	if len(parts) == 1 {
		return value
	}
	for i := 1; i < len(parts); i++ {
		parts[i] = exportedIdentifier(parts[i])
	}
	return strings.Join(parts, "")
}

func columnType(field Field) string {
	switch field.Type {
	case "string":
		return "type:varchar(255)"
	case "int64":
		return "type:bigint"
	case "uint64":
		return "type:bigint unsigned"
	case "bool":
		return "type:tinyint(1)"
	case "float64":
		return "type:double"
	case "time":
		return "type:datetime(3)"
	default:
		return ""
	}
}

func fieldNeedsTime(fields []Field) bool {
	for _, field := range fields {
		if field.Type == "time" {
			return true
		}
	}
	return false
}
