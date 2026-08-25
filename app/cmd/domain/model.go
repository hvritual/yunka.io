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
	SpecVersion  = 2
)

var (
	namePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	tablePrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Column      string `json:"column,omitempty"`
	ProtoNumber int    `json:"protoNumber,omitempty"`
	POOwned     bool   `json:"poOwned,omitempty"`
}

type ObjectSpec struct {
	Name                 string   `json:"name"`
	File                 string   `json:"file"`
	TableName            string   `json:"tableName"`
	Fields               []Field  `json:"fields,omitempty"`
	POEmbedsBase         bool     `json:"poEmbedsBase,omitempty"`
	ReservedProtoNumbers []int    `json:"reservedProtoNumbers,omitempty"`
	ReservedProtoNames   []string `json:"reservedProtoNames,omitempty"`
}

type RESTSpec struct {
	Enabled  bool   `json:"enabled"`
	Prefix   string `json:"prefix,omitempty"`
	BasePath string `json:"basePath,omitempty"` // legacy single-object manifest compatibility
}

type RPCSpec struct {
	Enabled bool   `json:"enabled"`
	Service string `json:"service,omitempty"` // legacy single-object manifest compatibility
}

type Spec struct {
	Version      int          `json:"version"`
	Domain       string       `json:"domain"`
	TablePrefix  string       `json:"tablePrefix"`
	TenantScoped bool         `json:"tenantScoped"`
	Objects      []ObjectSpec `json:"objects,omitempty"`
	REST         RESTSpec     `json:"rest"`
	RPC          RPCSpec      `json:"rpc"`

	// Version-1 single-object manifest compatibility. Regenerate upgrades these
	// fields into Objects and rewrites the manifest deterministically.
	Object    string  `json:"object,omitempty"`
	TableName string  `json:"tableName,omitempty"`
	Fields    []Field `json:"fields,omitempty"`
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
	restPrefix := strings.TrimSpace(options.RESTPrefix)
	if restPrefix == "" {
		restPrefix = "/v1"
	}
	if !strings.HasPrefix(restPrefix, "/") {
		return Spec{}, "", fmt.Errorf("domain: REST prefix %q must start with /", restPrefix)
	}
	restPrefix = strings.TrimRight(restPrefix, "/")
	if restPrefix == "" {
		restPrefix = "/"
	}
	return Spec{
		Version:      SpecVersion,
		Domain:       domainName,
		TablePrefix:  tablePrefix,
		TenantScoped: !options.Global,
		REST:         RESTSpec{Enabled: !options.NoREST, Prefix: restPrefix},
		RPC:          RPCSpec{Enabled: !options.NoRPC},
	}, absoluteRoot, nil
}

// normalizeOptions remains as a compatibility helper for tests/callers that
// construct a single object from flags. New generation treats PO files as the
// authoritative object/field inventory.
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
	fields = assignMissingProtoNumbers(fields, nil)
	spec.Objects = []ObjectSpec{{
		Name:      object,
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
		fields = append(fields, Field{Name: name, Type: kind, Column: name, POOwned: true})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields, nil
}

func validateSpec(spec Spec) error {
	if spec.Version == 1 && len(spec.Objects) == 0 {
		return validateLegacySpec(spec)
	}
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
		reservedNumbers := map[int]struct{}{}
		for _, number := range object.ReservedProtoNumbers {
			if number <= 0 {
				return fmt.Errorf("domain: object %q has invalid reserved proto number %d", object.Name, number)
			}
			if _, duplicate := reservedNumbers[number]; duplicate {
				return fmt.Errorf("domain: object %q duplicates reserved proto number %d", object.Name, number)
			}
			reservedNumbers[number] = struct{}{}
		}
		if err := validateFields(object.Fields); err != nil {
			return fmt.Errorf("domain: object %s: %w", object.Name, err)
		}
		for _, field := range object.Fields {
			if _, collision := reservedNumbers[field.ProtoNumber]; collision {
				return fmt.Errorf("domain: object %q live field %q reuses reserved proto number %d", object.Name, field.Name, field.ProtoNumber)
			}
		}
	}
	if spec.REST.Enabled {
		prefix := spec.REST.Prefix
		if prefix == "" {
			prefix = inferLegacyRESTPrefix(spec)
		}
		if !strings.HasPrefix(prefix, "/") {
			return errors.New("domain: REST prefix must start with /")
		}
	}
	return nil
}

func validateLegacySpec(spec Spec) error {
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
	if err := validateFields(spec.Fields); err != nil {
		return err
	}
	if spec.REST.Enabled && !strings.HasPrefix(spec.REST.BasePath, "/") {
		return errors.New("domain: REST basePath must start with /")
	}
	if spec.RPC.Enabled && strings.TrimSpace(spec.RPC.Service) == "" {
		return errors.New("domain: RPC service name is required")
	}
	return nil
}

func validateFields(fields []Field) error {
	seenNames := map[string]struct{}{}
	seenNumbers := map[int]struct{}{}
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
		if field.ProtoNumber <= 0 {
			return fmt.Errorf("field %q has invalid proto number %d", field.Name, field.ProtoNumber)
		}
		if _, duplicate := seenNumbers[field.ProtoNumber]; duplicate {
			return fmt.Errorf("duplicate proto field number %d", field.ProtoNumber)
		}
		seenNumbers[field.ProtoNumber] = struct{}{}
	}
	return nil
}

func upgradeSpec(spec Spec) Spec {
	if spec.Version != 1 || len(spec.Objects) > 0 {
		return spec
	}
	fields := assignMissingProtoNumbers(spec.Fields, nil)
	for i := range fields {
		if fields[i].Column == "" {
			fields[i].Column = fields[i].Name
		}
	}
	prefix := inferLegacyRESTPrefix(spec)
	return Spec{
		Version:      SpecVersion,
		Domain:       spec.Domain,
		TablePrefix:  spec.TablePrefix,
		TenantScoped: spec.TenantScoped,
		Objects: []ObjectSpec{{
			Name:          spec.Object,
			File:          snakeCase(spec.Object) + ".go",
			TableName:     spec.TableName,
			Fields:        fields,
			POEmbedsBase: true,
		}},
		REST: RESTSpec{Enabled: spec.REST.Enabled, Prefix: prefix},
		RPC:  RPCSpec{Enabled: spec.RPC.Enabled},
	}
}

func inferLegacyRESTPrefix(spec Spec) string {
	if spec.REST.Prefix != "" {
		return spec.REST.Prefix
	}
	base := strings.TrimRight(spec.REST.BasePath, "/")
	if base == "" {
		return "/v1"
	}
	object := spec.Object
	if object == "" && len(spec.Objects) == 1 {
		object = spec.Objects[0].Name
	}
	if object != "" {
		suffix := "/" + pluralize(object)
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
		}
	}
	if base == "" {
		return "/"
	}
	return base
}

func canonicalizeSpec(spec Spec) Spec {
	spec = upgradeSpec(spec)
	spec.Object = ""
	spec.TableName = ""
	spec.Fields = nil
	spec.REST.BasePath = ""
	spec.RPC.Service = ""
	sort.Slice(spec.Objects, func(i, j int) bool { return spec.Objects[i].Name < spec.Objects[j].Name })
	for objectIndex := range spec.Objects {
		fields := spec.Objects[objectIndex].Fields
		sort.Slice(fields, func(i, j int) bool {
			if fields[i].ProtoNumber == fields[j].ProtoNumber {
				return fields[i].Name < fields[j].Name
			}
			return fields[i].ProtoNumber < fields[j].ProtoNumber
		})
		spec.Objects[objectIndex].Fields = fields
		sort.Ints(spec.Objects[objectIndex].ReservedProtoNumbers)
		sort.Strings(spec.Objects[objectIndex].ReservedProtoNames)
	}
	return spec
}

func assignMissingProtoNumbers(fields []Field, prior []Field) []Field {
	priorByName := make(map[string]Field, len(prior))
	maxNumber := 0
	for _, field := range prior {
		priorByName[field.Name] = field
		if field.ProtoNumber > maxNumber {
			maxNumber = field.ProtoNumber
		}
	}
	result := append([]Field(nil), fields...)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	for i := range result {
		if previous, ok := priorByName[result[i].Name]; ok && previous.ProtoNumber > 0 {
			result[i].ProtoNumber = previous.ProtoNumber
			if result[i].Column == "" {
				result[i].Column = previous.Column
			}
			continue
		}
		if result[i].ProtoNumber <= 0 {
			maxNumber++
			result[i].ProtoNumber = maxNumber
		}
		if result[i].Column == "" {
			result[i].Column = result[i].Name
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProtoNumber < result[j].ProtoNumber })
	return result
}

func tableName(prefix, domain, object string) string {
	return prefix + "_" + domain + "_" + object
}

func restBasePath(spec Spec, object ObjectSpec) string {
	prefix := spec.REST.Prefix
	if prefix == "" {
		prefix = inferLegacyRESTPrefix(spec)
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "/" + pluralize(object.Name)
	}
	return prefix + "/" + pluralize(object.Name)
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

func snakeCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	runes := []rune(value)
	for i, current := range runes {
		if current == '-' || current == ' ' || current == '.' {
			if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "_") {
				builder.WriteRune('_')
			}
			continue
		}
		if current == '_' {
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
