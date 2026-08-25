package module

import (
	"bufio"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const AppName = "module"

var (
	generatedModuleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	capabilityNamePattern      = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	configKeyPattern           = regexp.MustCompile(`^[A-Za-z0-9._-]{1,256}$`)
)

type Options struct {
	Name      string
	Root      string
	ConfigKey string
	NoConfig  bool
	Logger    bool
	Databases []string
	EventBus  bool
	RPC       []string
	DependsOn []string
}

func DefaultOptions(name string) Options {
	name = strings.TrimSpace(name)
	return Options{
		Name:      name,
		Root:      "modules",
		ConfigKey: "modules." + name,
		Logger:    true,
	}
}

func Generate(moduleName string) error {
	return GenerateWithOptions(DefaultOptions(moduleName))
}

func GenerateWithOptions(options Options) error {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return err
	}
	_, _, target, packageImport, err := resolveTarget(normalized.Root, normalized.Name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("module: target %s already exists", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(parent, ".yunka-module-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.MkdirAll(filepath.Join(temporary, "autoload"), 0o750); err != nil {
		return err
	}
	files := map[string]string{
		"config.go":                              configTemplate(normalized.Name),
		"dependencies.go":                        dependenciesTemplate(normalized),
		"module.go":                              moduleTemplate(normalized),
		"zz_yunka_module_gen.go":                 generatedWiringTemplate(normalized),
		filepath.Join("autoload", "register.go"): autoloadTemplate(packageImport),
	}
	order := []string{
		"config.go",
		"dependencies.go",
		"module.go",
		"zz_yunka_module_gen.go",
		filepath.Join("autoload", "register.go"),
	}
	for _, relative := range order {
		formatted, err := format.Source([]byte(files[relative]))
		if err != nil {
			return fmt.Errorf("module: format %s: %w", relative, err)
		}
		path := filepath.Join(temporary, relative)
		if err := os.WriteFile(path, formatted, 0o640); err != nil {
			return err
		}
	}
	return os.Rename(temporary, target)
}

func normalizeOptions(options Options) (Options, error) {
	options.Name = strings.TrimSpace(options.Name)
	if !generatedModuleNamePattern.MatchString(options.Name) {
		return Options{}, fmt.Errorf("module: name %q must match %s", options.Name, generatedModuleNamePattern)
	}
	options.Root = strings.TrimSpace(options.Root)
	if options.Root == "" {
		options.Root = "modules"
	}
	if options.NoConfig {
		options.ConfigKey = ""
	} else {
		options.ConfigKey = strings.TrimSpace(options.ConfigKey)
		if options.ConfigKey == "" {
			options.ConfigKey = "modules." + options.Name
		}
		if !configKeyPattern.MatchString(options.ConfigKey) {
			return Options{}, fmt.Errorf("module: config key %q must match %s", options.ConfigKey, configKeyPattern)
		}
	}
	var err error
	if options.Databases, err = normalizeCapabilityList("database", options.Databases); err != nil {
		return Options{}, err
	}
	if options.RPC, err = normalizeCapabilityList("rpc", options.RPC); err != nil {
		return Options{}, err
	}
	if options.DependsOn, err = normalizeCapabilityList("dependency", options.DependsOn); err != nil {
		return Options{}, err
	}
	if err := validateGeneratedFields(options); err != nil {
		return Options{}, err
	}
	return options, nil
}

func normalizeCapabilityList(kind string, values []string) ([]string, error) {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !capabilityNamePattern.MatchString(value) {
			return nil, fmt.Errorf("module: %s name %q must match %s", kind, value, capabilityNamePattern)
		}
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validateGeneratedFields(options Options) error {
	fields := map[string]string{}
	claim := func(field, source string) error {
		if previous, exists := fields[field]; exists {
			return fmt.Errorf("module: generated field %s collides between %s and %s", field, previous, source)
		}
		fields[field] = source
		return nil
	}
	if options.ConfigKey != "" {
		if err := claim("Config", "configuration"); err != nil {
			return err
		}
	}
	if options.Logger {
		if err := claim("Logger", "logger"); err != nil {
			return err
		}
	}
	if options.EventBus {
		if err := claim("EventBus", "event bus"); err != nil {
			return err
		}
	}
	for _, name := range options.Databases {
		if err := claim(databaseField(name), "database "+name); err != nil {
			return err
		}
	}
	for _, name := range options.RPC {
		if err := claim(rpcField(name), "rpc "+name); err != nil {
			return err
		}
	}
	return nil
}

func resolveTarget(root, moduleName string) (moduleDirectory, goModule, target, packageImport string, err error) {
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return "", "", "", "", err
	}
	moduleDirectory, goModule, err = findOwningGoModule(rootAbsolute)
	if err != nil {
		return "", "", "", "", err
	}
	target = filepath.Join(rootAbsolute, moduleName)
	relative, err := filepath.Rel(moduleDirectory, target)
	if err != nil {
		return "", "", "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", "", "", fmt.Errorf("module: target %s escapes Go module %s", target, moduleDirectory)
	}
	packageImport = strings.TrimSuffix(goModule, "/") + "/" + filepath.ToSlash(relative)
	return moduleDirectory, goModule, target, packageImport, nil
}

func findOwningGoModule(start string) (string, string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	for {
		if info, statErr := os.Stat(current); statErr == nil && !info.IsDir() {
			current = filepath.Dir(current)
		} else if os.IsNotExist(statErr) {
			parent := filepath.Dir(current)
			if parent == current {
				return "", "", fmt.Errorf("module: go.mod not found for %s", start)
			}
			current = parent
			continue
		} else if statErr != nil {
			return "", "", statErr
		}
		path := filepath.Join(current, "go.mod")
		if file, openErr := os.Open(path); openErr == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "module ") {
					value := strings.TrimSpace(strings.TrimPrefix(line, "module "))
					if value == "" {
						return "", "", fmt.Errorf("module: empty module directive in %s", path)
					}
					return current, value, nil
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				return "", "", scanErr
			}
			return "", "", fmt.Errorf("module: module directive not found in %s", path)
		} else if !os.IsNotExist(openErr) {
			return "", "", openErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", fmt.Errorf("module: go.mod not found for %s", start)
		}
		current = parent
	}
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
	if builder.Len() == 0 {
		return "Capability"
	}
	return builder.String()
}

func databaseField(name string) string { return exportedIdentifier(name) + "Database" }
func rpcField(name string) string      { return exportedIdentifier(name) + "RPC" }

func configTemplate(packageName string) string {
	return fmt.Sprintf(`package %s

// Config is populated through the descriptor-declared configuration key.
type Config struct{}

func DefaultConfig() Config    { return Config{} }
func (Config) Validate() error { return nil }
`, packageName)
}

func dependenciesTemplate(options Options) string {
	imports := make([]string, 0, 4)
	if len(options.RPC) > 0 {
		imports = append(imports, `"google.golang.org/grpc"`)
	}
	if len(options.Databases) > 0 {
		imports = append(imports, `"gorm.io/gorm"`)
	}
	if options.EventBus {
		imports = append(imports, `"yunka.io/framework/core/eventBus"`)
	}
	if options.Logger {
		imports = append(imports, `"yunka.io/pkg/logExt"`)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "package %s\n\n", options.Name)
	if len(imports) > 0 {
		builder.WriteString("import (\n")
		for _, imported := range imports {
			fmt.Fprintf(&builder, "\t%s\n", imported)
		}
		builder.WriteString(")\n\n")
	}
	builder.WriteString("// Dependencies is the complete compiler-checked capability view for this module.\n")
	builder.WriteString("// It contains no lookup, connection construction, or global runtime access.\n")
	builder.WriteString("type Dependencies struct {\n")
	if options.ConfigKey != "" {
		builder.WriteString("\tConfig Config\n")
	}
	if options.Logger {
		builder.WriteString("\tLogger logExt.Logger\n")
	}
	for _, name := range options.Databases {
		fmt.Fprintf(&builder, "\t%s *gorm.DB\n", databaseField(name))
	}
	if options.EventBus {
		builder.WriteString("\tEventBus eventBus.EventBus\n")
	}
	for _, name := range options.RPC {
		fmt.Fprintf(&builder, "\t%s grpc.ClientConnInterface\n", rpcField(name))
	}
	builder.WriteString("}\n")
	return builder.String()
}

func moduleTemplate(options Options) string {
	var checks []string
	if options.Logger {
		checks = append(checks, `if dependencies.Logger == nil {
		return nil, fmt.Errorf("%s: logger is required", ModuleName)
	}`)
	}
	for _, name := range options.Databases {
		checks = append(checks, fmt.Sprintf(`if dependencies.%s == nil {
		return nil, fmt.Errorf("%%s: database %s is required", ModuleName)
	}`, databaseField(name), name))
	}
	if options.EventBus {
		checks = append(checks, `if dependencies.EventBus == nil {
		return nil, fmt.Errorf("%s: event bus is required", ModuleName)
	}`)
	}
	for _, name := range options.RPC {
		checks = append(checks, fmt.Sprintf(`if dependencies.%s == nil {
		return nil, fmt.Errorf("%%s: RPC target %s is required", ModuleName)
	}`, rpcField(name), name))
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "package %s\n\n", options.Name)
	if len(checks) > 0 {
		builder.WriteString(`import "fmt"

`)
	}
	fmt.Fprintf(&builder, "const ModuleName = %q\n\n", options.Name)
	builder.WriteString("type Module struct {\n\tdependencies Dependencies\n}\n\n")
	builder.WriteString("func NewModule(dependencies Dependencies) (*Module, error) {\n")
	for _, check := range checks {
		builder.WriteString("\t")
		builder.WriteString(strings.ReplaceAll(check, "\n", "\n\t"))
		builder.WriteString("\n")
	}
	builder.WriteString("\treturn &Module{dependencies: dependencies}, nil\n}\n\n")
	builder.WriteString("func (*Module) Name() string { return ModuleName }\n")
	return builder.String()
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func generatedWiringTemplate(options Options) string {
	usesFmt := options.ConfigKey != "" || len(options.Databases) > 0 || len(options.RPC) > 0
	var builder strings.Builder
	builder.WriteString("// Code generated by yunka module; DO NOT EDIT.\n\n")
	fmt.Fprintf(&builder, "package %s\n\n", options.Name)
	builder.WriteString("import (\n")
	if usesFmt {
		builder.WriteString("\t\"fmt\"\n")
	}
	builder.WriteString("\t\"yunka.io/framework/core/modulecatalog\"\n")
	builder.WriteString(")\n\n")
	builder.WriteString("func GeneratedDescriptor() modulecatalog.Descriptor {\n")
	builder.WriteString("\treturn modulecatalog.Descriptor{\n")
	builder.WriteString("\t\tName: ModuleName,\n")
	builder.WriteString("\t\tVersion: \"v0.1.0\",\n")
	if len(options.DependsOn) > 0 {
		builder.WriteString("\t\tDependsOn: []string{")
		for index, dependency := range options.DependsOn {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(&builder, "%q", dependency)
		}
		builder.WriteString("},\n")
	}
	builder.WriteString("\t\tRequirements: modulecatalog.Requirements{\n")
	if options.ConfigKey != "" {
		fmt.Fprintf(&builder, "\t\t\tConfigKey: %q,\n", options.ConfigKey)
	}
	if options.Logger {
		builder.WriteString("\t\t\tLogger: true,\n")
	}
	if len(options.Databases) > 0 {
		builder.WriteString("\t\t\tDatabases: []modulecatalog.DatabaseRequirement{")
		for index, name := range options.Databases {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(&builder, "{Name: %q}", name)
		}
		builder.WriteString("},\n")
	}
	if options.EventBus {
		builder.WriteString("\t\t\tEventBus: true,\n")
	}
	if len(options.RPC) > 0 {
		builder.WriteString("\t\t\tRPC: []modulecatalog.RPCRequirement{")
		for index, name := range options.RPC {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(&builder, "{Name: %q}", name)
		}
		builder.WriteString("},\n")
	}
	builder.WriteString("\t\t},\n")
	builder.WriteString("\t\tBuild: generatedBuild,\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func generatedBuild(context modulecatalog.BuildContext) (modulecatalog.Instance, error) {\n")
	builder.WriteString("\tdependencies := Dependencies{}\n")
	if options.ConfigKey != "" {
		builder.WriteString("\tconfig := DefaultConfig()\n")
		fmt.Fprintf(&builder, "\tif err := context.Config().Decode(ModuleName, %q, &config); err != nil {\n", options.ConfigKey)
		builder.WriteString("\t\treturn nil, fmt.Errorf(\"%s config: %w\", ModuleName, err)\n\t}\n")
		builder.WriteString("\tif err := config.Validate(); err != nil {\n")
		builder.WriteString("\t\treturn nil, fmt.Errorf(\"%s config validation: %w\", ModuleName, err)\n\t}\n")
		builder.WriteString("\tdependencies.Config = config\n")
	}
	if options.Logger {
		builder.WriteString("\tdependencies.Logger = context.Logger()\n")
	}
	for _, name := range options.Databases {
		local := lowerFirst(databaseField(name))
		fmt.Fprintf(&builder, "\t%s, err := context.Databases().GORM(%q)\n", local, name)
		fmt.Fprintf(&builder, "\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"%%s database %s: %%w\", ModuleName, err)\n\t}\n", name)
		fmt.Fprintf(&builder, "\tdependencies.%s = %s\n", databaseField(name), local)
	}
	if options.EventBus {
		builder.WriteString("\tdependencies.EventBus = context.EventBus()\n")
	}
	for _, name := range options.RPC {
		local := lowerFirst(rpcField(name))
		fmt.Fprintf(&builder, "\t%s, err := context.RPC().Connection(%q)\n", local, name)
		fmt.Fprintf(&builder, "\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"%%s RPC target %s: %%w\", ModuleName, err)\n\t}\n", name)
		fmt.Fprintf(&builder, "\tdependencies.%s = %s\n", rpcField(name), local)
	}
	builder.WriteString("\treturn NewModule(dependencies)\n")
	builder.WriteString("}\n")
	return builder.String()
}

func autoloadTemplate(packageImport string) string {
	return fmt.Sprintf(`package autoload

import (
	"yunka.io/framework/core/modulecatalog"
	module %q
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
`, packageImport)
}
