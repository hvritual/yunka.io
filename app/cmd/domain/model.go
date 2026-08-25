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
	SpecVersion  = 1
)

var (
	namePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	tablePrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type RESTSpec struct {
	Enabled  bool   `json:"enabled"`
	BasePath string `json:"basePath,omitempty"`
}
type RPCSpec struct {
	Enabled bool   `json:"enabled"`
	Service string `json:"service,omitempty"`
}
type Spec struct {
	Version      int      `json:"version"`
	Domain       string   `json:"domain"`
	Object       string   `json:"object"`
	TablePrefix  string   `json:"tablePrefix"`
	TableName    string   `json:"tableName"`
	TenantScoped bool     `json:"tenantScoped"`
	Fields       []Field  `json:"fields"`
	REST         RESTSpec `json:"rest"`
	RPC          RPCSpec  `json:"rpc"`
}
type Options struct {
	Name        string
	Object      string
	Root        string
	TablePrefix string
	RESTPrefix  string
	Fields      []string
	Global      bool
	NoREST      bool
	NoRPC       bool
}

var reservedFields = map[string]struct{}{
	"id": {}, "tenant_id": {}, "version": {}, "created_at": {}, "updated_at": {}, "deleted_at": {},
}

func normalizeOptions(options Options) (Spec, string, error) {
	domainName := strings.TrimSpace(options.Name)
	if !namePattern.MatchString(domainName) {
		return Spec{}, "", fmt.Errorf("domain: name %q must match %s", domainName, namePattern)
	}
	object := strings.TrimSpace(options.Object)
	if object == "" {
		object = domainName
	}
	if !namePattern.MatchString(object) {
		return Spec{}, "", fmt.Errorf("domain: object %q must match %s", object, namePattern)
	}
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "internal"
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Spec{}, "", err
	}
	tablePrefix := strings.TrimSpace(options.TablePrefix)
	if tablePrefix == "" {
		tablePrefix = "biz"
	}
	if !tablePrefixPattern.MatchString(tablePrefix) {
		return Spec{}, "", fmt.Errorf("domain: table prefix %q must match %s", tablePrefix, tablePrefixPattern)
	}
	fields, err := parseFields(options.Fields)
	if err != nil {
		return Spec{}, "", err
	}
	if len(fields) == 0 {
		fields = []Field{{Name: "name", Type: "string"}}
	}
	restPrefix := strings.TrimSpace(options.RESTPrefix)
	if restPrefix == "" {
		restPrefix = "/v1"
	}
	if !strings.HasPrefix(restPrefix, "/") {
		return Spec{}, "", fmt.Errorf("domain: REST prefix %q must start with /", restPrefix)
	}
	restPrefix = strings.TrimRight(restPrefix, "/")
	plural := pluralize(object)
	spec := Spec{
		Version:      SpecVersion,
		Domain:       domainName,
		Object:       object,
		TablePrefix:  tablePrefix,
		TableName:    tableName(tablePrefix, domainName, object),
		TenantScoped: !options.Global,
		Fields:       fields,
		REST: RESTSpec{
			Enabled:  !options.NoREST,
			BasePath: restPrefix + "/" + plural,
		},
		RPC: RPCSpec{
			Enabled: !options.NoRPC,
			Service: exportedIdentifier(object) + "Service",
		},
	}
	if err := validateSpec(spec); err != nil {
		return Spec{}, "", err
	}
	return spec, absoluteRoot, nil
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
		fields = append(fields, Field{Name: name, Type: kind})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields, nil
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("domain: spec version %d is unsupported", spec.Version)
	}
	if !namePattern.MatchString(spec.Domain) || !namePattern.MatchString(spec.Object) {
		return errors.New("domain: invalid domain or object name")
	}
	if !tablePrefixPattern.MatchString(spec.TablePrefix) {
		return errors.New("domain: invalid table prefix")
	}
	expectedTable := tableName(spec.TablePrefix, spec.Domain, spec.Object)
	if spec.TableName != expectedTable {
		return fmt.Errorf("domain: tableName=%q want %q (prefix_domain_object)", spec.TableName, expectedTable)
	}
	if len(spec.Fields) == 0 {
		return errors.New("domain: at least one business field is required")
	}
	seen := map[string]struct{}{}
	for _, field := range spec.Fields {
		if !namePattern.MatchString(field.Name) {
			return fmt.Errorf("domain: invalid field name %q", field.Name)
		}
		if _, reserved := reservedFields[field.Name]; reserved {
			return fmt.Errorf("domain: field %q is reserved", field.Name)
		}
		if _, ok := goType(field.Type); !ok {
			return fmt.Errorf("domain: unsupported field type %q", field.Type)
		}
		if _, duplicate := seen[field.Name]; duplicate {
			return fmt.Errorf("domain: duplicate field %q", field.Name)
		}
		seen[field.Name] = struct{}{}
	}
	if spec.REST.Enabled && !strings.HasPrefix(spec.REST.BasePath, "/") {
		return errors.New("domain: REST basePath must start with /")
	}
	if spec.RPC.Enabled && strings.TrimSpace(spec.RPC.Service) == "" {
		return errors.New("domain: RPC service name is required")
	}
	return nil
}

func tableName(prefix, domain, object string) string {
	return prefix + "_" + domain + "_" + object
}

func pluralize(value string) string {
	if strings.HasSuffix(value, "s") {
		return value
	}
	if strings.HasSuffix(value, "y") && len(value) > 1 {
		return value[:len(value)-1] + "ies"
	}
	return value + "s"
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

func protoType(kind string) string {
	switch kind {
	case "int64":
		return "int64"
	case "uint64":
		return "uint64"
	case "bool":
		return "bool"
	case "float64":
		return "double"
	case "time":
		return "google.protobuf.Timestamp"
	default:
		return "string"
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
