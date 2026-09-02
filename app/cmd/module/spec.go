package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/pkg/modulespec"
)

type SpecOptions struct {
	Name      string
	Root      string
	Version   string
	ConfigKey string
	Logger    bool
	Databases []string
	EventBus  bool
	RPC       []string
	DependsOn []string
}

func AddSpec(options SpecOptions) error {
	name := strings.TrimSpace(options.Name)
	if !generatedModuleNamePattern.MatchString(name) {
		return fmt.Errorf("module: name %q must match %s", name, generatedModuleNamePattern)
	}
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "modules"
	}
	var err error
	options.Databases, err = normalizeCapabilityList("database", options.Databases)
	if err != nil {
		return err
	}
	options.RPC, err = normalizeCapabilityList("rpc", options.RPC)
	if err != nil {
		return err
	}
	options.DependsOn, err = normalizeCapabilityList("dependency", options.DependsOn)
	if err != nil {
		return err
	}
	for _, dependency := range options.DependsOn {
		if dependency == name {
			return fmt.Errorf("module: module %q depends on itself", name)
		}
	}
	configKey := strings.TrimSpace(options.ConfigKey)
	if configKey != "" && !configKeyPattern.MatchString(configKey) {
		return fmt.Errorf("module: config key %q must match %s", configKey, configKeyPattern)
	}

	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	moduleRoot := filepath.Join(rootAbsolute, name)
	if info, statErr := os.Stat(moduleRoot); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("module: target %s is not a directory", moduleRoot)
		}
		if legacy, legacyErr := hasLegacyModuleSource(moduleRoot); legacyErr != nil {
			return legacyErr
		} else if legacy {
			return fmt.Errorf("module: %s contains legacy generated module source; migrate it before adding %s", name, modulespec.Filename)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	} else if err := os.MkdirAll(moduleRoot, 0o750); err != nil {
		return err
	}

	path := filepath.Join(moduleRoot, modulespec.Filename)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("module: declarative spec %s already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	spec := modulespec.Default()
	if strings.TrimSpace(options.Version) != "" {
		spec.Version = strings.TrimSpace(options.Version)
	}
	spec.DependsOn = append([]string(nil), options.DependsOn...)
	spec.Requirements = modulespec.Requirements{
		ConfigKey: configKey,
		Logger:    options.Logger,
		Databases: append([]string(nil), options.Databases...),
		EventBus:  options.EventBus,
		RPC:       append([]string(nil), options.RPC...),
	}
	if err := modulespec.ValidateForModule(name, modulespec.Normalize(spec)); err != nil {
		return err
	}
	return writeSpecAtomic(path, spec)
}

func RequireSpec(root, name, kind, value string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("module: module name is required")
	}
	path := specPath(root, name)
	spec, err := modulespec.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("module: declarative spec for %s not found; run `yunka module add --name %s` first", name, name)
		}
		return err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	value = strings.TrimSpace(value)
	switch kind {
	case "database":
		if value == "" {
			return fmt.Errorf("module: database name is required")
		}
		spec.Requirements.Databases = append(spec.Requirements.Databases, value)
	case "rpc":
		if value == "" {
			return fmt.Errorf("module: RPC target name is required")
		}
		spec.Requirements.RPC = append(spec.Requirements.RPC, value)
	case "dependency", "depends-on":
		if value == "" {
			return fmt.Errorf("module: dependency name is required")
		}
		spec.DependsOn = append(spec.DependsOn, value)
	case "logger":
		if value != "" {
			return fmt.Errorf("module: logger does not accept a value")
		}
		spec.Requirements.Logger = true
	case "event-bus":
		if value != "" {
			return fmt.Errorf("module: event-bus does not accept a value")
		}
		spec.Requirements.EventBus = true
	case "config":
		if value == "" {
			return fmt.Errorf("module: config key is required")
		}
		spec.Requirements.ConfigKey = value
	default:
		return fmt.Errorf("module: unsupported requirement %q; use database, rpc, dependency, logger, event-bus, or config", kind)
	}
	spec = modulespec.Normalize(spec)
	if err := modulespec.ValidateForModule(name, spec); err != nil {
		return err
	}
	return writeSpecAtomic(path, spec)
}

func ShowSpec(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("module: module name is required")
	}
	path := specPath(root, name)
	spec, err := modulespec.Load(path)
	if err != nil {
		return "", err
	}
	if err := modulespec.ValidateForModule(name, spec); err != nil {
		return "", err
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "Module: %s\n", name)
	fmt.Fprintf(&builder, "Source: %s\n", filepath.ToSlash(path))
	fmt.Fprintf(&builder, "Version: %s\n", spec.Version)
	builder.WriteString("Requires:\n")
	requirements := make([]string, 0)
	if spec.Requirements.ConfigKey != "" {
		requirements = append(requirements, "config."+spec.Requirements.ConfigKey)
	}
	if spec.Requirements.Logger {
		requirements = append(requirements, "logger")
	}
	for _, database := range spec.Requirements.Databases {
		requirements = append(requirements, "database."+database)
	}
	if spec.Requirements.EventBus {
		requirements = append(requirements, "event-bus")
	}
	for _, rpc := range spec.Requirements.RPC {
		requirements = append(requirements, "rpc."+rpc)
	}
	if len(requirements) == 0 {
		builder.WriteString("  none\n")
	} else {
		for _, requirement := range requirements {
			fmt.Fprintf(&builder, "  - %s\n", requirement)
		}
	}
	builder.WriteString("Depends on:\n")
	if len(spec.DependsOn) == 0 {
		builder.WriteString("  none\n")
	} else {
		for _, dependency := range spec.DependsOn {
			fmt.Fprintf(&builder, "  - %s\n", dependency)
		}
	}
	builder.WriteString("Runtime build: none\n")
	return builder.String(), nil
}

func specPath(root, name string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "modules"
	}
	return filepath.Join(root, strings.TrimSpace(name), modulespec.Filename)
}

func writeSpecAtomic(path string, spec modulespec.Spec) error {
	data, err := modulespec.Marshal(spec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".module-yunka-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func hasLegacyModuleSource(root string) (bool, error) {
	for _, relative := range requiredModuleFiles {
		_, err := os.Stat(filepath.Join(root, relative))
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}
