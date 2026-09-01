package providerplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunka.io/pkg/assemblyplan"
)

const SchemaVersion = 1

var (
	providerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	environmentPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type HostCapabilities struct {
	Config   bool `json:"config,omitempty"`
	Logger   bool `json:"logger,omitempty"`
	EventBus bool `json:"eventBus,omitempty"`
}

type DatabaseBinding struct {
	Name            string `json:"name"`
	Driver          string `json:"driver"`
	DSNEnv          string `json:"dsnEnv"`
	MaxOpenConns    int    `json:"maxOpenConns,omitempty"`
	MaxIdleConns    int    `json:"maxIdleConns,omitempty"`
	ConnMaxLifetime string `json:"connMaxLifetime,omitempty"`
	ConnMaxIdleTime string `json:"connMaxIdleTime,omitempty"`
}

type TLSBinding struct {
	ServerNameEnv string `json:"serverNameEnv,omitempty"`
	CAFileEnv     string `json:"caFileEnv,omitempty"`
	CertFileEnv   string `json:"certFileEnv,omitempty"`
	KeyFileEnv    string `json:"keyFileEnv,omitempty"`
}

type RPCBinding struct {
	Name      string      `json:"name"`
	Driver    string      `json:"driver"`
	TargetEnv string      `json:"targetEnv"`
	Insecure  bool        `json:"insecure,omitempty"`
	TLS       *TLSBinding `json:"tls,omitempty"`
}

type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	Host          HostCapabilities  `json:"host,omitempty"`
	Databases     []DatabaseBinding `json:"databases,omitempty"`
	RPC           []RPCBinding      `json:"rpc,omitempty"`
}

func Empty() Manifest { return Manifest{SchemaVersion: SchemaVersion} }

func Load(path string) (Manifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("providerplan: decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return Manifest{}, fmt.Errorf("providerplan: %s contains multiple JSON values", path)
		}
		return Manifest{}, fmt.Errorf("providerplan: decode %s: %w", path, err)
	}
	manifest = Canonicalize(manifest)
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Marshal(manifest Manifest) ([]byte, error) {
	manifest = Canonicalize(manifest)
	if err := Validate(manifest); err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func Canonicalize(manifest Manifest) Manifest {
	for index := range manifest.Databases {
		binding := &manifest.Databases[index]
		binding.Name = strings.TrimSpace(binding.Name)
		binding.Driver = strings.ToLower(strings.TrimSpace(binding.Driver))
		binding.DSNEnv = strings.TrimSpace(binding.DSNEnv)
		binding.ConnMaxLifetime = strings.TrimSpace(binding.ConnMaxLifetime)
		binding.ConnMaxIdleTime = strings.TrimSpace(binding.ConnMaxIdleTime)
	}
	for index := range manifest.RPC {
		binding := &manifest.RPC[index]
		binding.Name = strings.TrimSpace(binding.Name)
		binding.Driver = strings.ToLower(strings.TrimSpace(binding.Driver))
		binding.TargetEnv = strings.TrimSpace(binding.TargetEnv)
		if binding.TLS != nil {
			binding.TLS.ServerNameEnv = strings.TrimSpace(binding.TLS.ServerNameEnv)
			binding.TLS.CAFileEnv = strings.TrimSpace(binding.TLS.CAFileEnv)
			binding.TLS.CertFileEnv = strings.TrimSpace(binding.TLS.CertFileEnv)
			binding.TLS.KeyFileEnv = strings.TrimSpace(binding.TLS.KeyFileEnv)
		}
	}
	sort.Slice(manifest.Databases, func(i, j int) bool { return manifest.Databases[i].Name < manifest.Databases[j].Name })
	sort.Slice(manifest.RPC, func(i, j int) bool { return manifest.RPC[i].Name < manifest.RPC[j].Name })
	return manifest
}

func Validate(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("providerplan: unsupported schemaVersion %d", manifest.SchemaVersion)
	}
	seenDatabase := make(map[string]struct{}, len(manifest.Databases))
	for _, binding := range manifest.Databases {
		if !providerNamePattern.MatchString(binding.Name) {
			return fmt.Errorf("providerplan: invalid database provider name %q", binding.Name)
		}
		if _, duplicate := seenDatabase[binding.Name]; duplicate {
			return fmt.Errorf("providerplan: duplicate database provider %q", binding.Name)
		}
		seenDatabase[binding.Name] = struct{}{}
		if binding.Driver != "mysql" {
			return fmt.Errorf("providerplan: database %q driver %q is unsupported; supported: mysql", binding.Name, binding.Driver)
		}
		if err := validateEnvironment(binding.DSNEnv, "database "+binding.Name+" dsnEnv", true); err != nil {
			return err
		}
		if binding.MaxOpenConns < 0 || binding.MaxIdleConns < 0 {
			return fmt.Errorf("providerplan: database %q pool sizes cannot be negative", binding.Name)
		}
		if err := validateDuration(binding.ConnMaxLifetime, "database "+binding.Name+" connMaxLifetime"); err != nil {
			return err
		}
		if err := validateDuration(binding.ConnMaxIdleTime, "database "+binding.Name+" connMaxIdleTime"); err != nil {
			return err
		}
	}
	seenRPC := make(map[string]struct{}, len(manifest.RPC))
	for _, binding := range manifest.RPC {
		if !providerNamePattern.MatchString(binding.Name) {
			return fmt.Errorf("providerplan: invalid RPC provider name %q", binding.Name)
		}
		if _, duplicate := seenRPC[binding.Name]; duplicate {
			return fmt.Errorf("providerplan: duplicate RPC provider %q", binding.Name)
		}
		seenRPC[binding.Name] = struct{}{}
		if binding.Driver != "grpc" {
			return fmt.Errorf("providerplan: RPC %q driver %q is unsupported; supported: grpc", binding.Name, binding.Driver)
		}
		if err := validateEnvironment(binding.TargetEnv, "RPC "+binding.Name+" targetEnv", true); err != nil {
			return err
		}
		if binding.Insecure && binding.TLS != nil {
			return fmt.Errorf("providerplan: RPC %q cannot configure both insecure and TLS transport", binding.Name)
		}
		if !binding.Insecure && binding.TLS == nil {
			return fmt.Errorf("providerplan: RPC %q must configure TLS or explicitly set insecure=true", binding.Name)
		}
		if binding.TLS != nil {
			if err := validateEnvironment(binding.TLS.ServerNameEnv, "RPC "+binding.Name+" tls.serverNameEnv", false); err != nil {
				return err
			}
			if err := validateEnvironment(binding.TLS.CAFileEnv, "RPC "+binding.Name+" tls.caFileEnv", false); err != nil {
				return err
			}
			if err := validateEnvironment(binding.TLS.CertFileEnv, "RPC "+binding.Name+" tls.certFileEnv", false); err != nil {
				return err
			}
			if err := validateEnvironment(binding.TLS.KeyFileEnv, "RPC "+binding.Name+" tls.keyFileEnv", false); err != nil {
				return err
			}
			if (binding.TLS.CertFileEnv == "") != (binding.TLS.KeyFileEnv == "") {
				return fmt.Errorf("providerplan: RPC %q TLS certFileEnv and keyFileEnv must be configured together", binding.Name)
			}
		}
	}
	return nil
}

func ValidateModules(manifest Manifest, modules []assemblyplan.ModuleInput) error {
	manifest = Canonicalize(manifest)
	if err := Validate(manifest); err != nil {
		return err
	}
	databases := make(map[string]struct{}, len(manifest.Databases))
	for _, binding := range manifest.Databases {
		databases[binding.Name] = struct{}{}
	}
	rpc := make(map[string]struct{}, len(manifest.RPC))
	for _, binding := range manifest.RPC {
		rpc[binding.Name] = struct{}{}
	}
	ordered := append([]assemblyplan.ModuleInput(nil), modules...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	var failures []error
	for _, module := range ordered {
		requirements := module.Requirements
		if strings.TrimSpace(requirements.ConfigKey) != "" && !manifest.Host.Config {
			failures = append(failures, fmt.Errorf("providerplan: module %q requires host config capability", module.Name))
		}
		if requirements.Logger && !manifest.Host.Logger {
			failures = append(failures, fmt.Errorf("providerplan: module %q requires host logger capability", module.Name))
		}
		if requirements.EventBus && !manifest.Host.EventBus {
			failures = append(failures, fmt.Errorf("providerplan: module %q requires host eventBus capability", module.Name))
		}
		for _, name := range requirements.Databases {
			if _, ok := databases[name]; !ok {
				failures = append(failures, fmt.Errorf("providerplan: module %q requires database provider %q", module.Name, name))
			}
		}
		for _, name := range requirements.RPC {
			if _, ok := rpc[name]; !ok {
				failures = append(failures, fmt.Errorf("providerplan: module %q requires RPC provider %q", module.Name, name))
			}
		}
	}
	return errors.Join(failures...)
}

func HasRequirements(modules []assemblyplan.ModuleInput) bool {
	for _, module := range modules {
		requirements := module.Requirements
		if strings.TrimSpace(requirements.ConfigKey) != "" || requirements.Logger || requirements.EventBus || len(requirements.Databases) > 0 || len(requirements.RPC) > 0 {
			return true
		}
	}
	return false
}

func BindingCount(manifest Manifest) int {
	count := len(manifest.Databases) + len(manifest.RPC)
	if manifest.Host.Config {
		count++
	}
	if manifest.Host.Logger {
		count++
	}
	if manifest.Host.EventBus {
		count++
	}
	return count
}

func validateEnvironment(value, label string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("providerplan: %s is required", label)
		}
		return nil
	}
	if !environmentPattern.MatchString(value) {
		return fmt.Errorf("providerplan: %s %q is not a valid environment variable name", label, value)
	}
	return nil
}

func validateDuration(value, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("providerplan: %s: %w", label, err)
	}
	if duration < 0 {
		return fmt.Errorf("providerplan: %s cannot be negative", label)
	}
	return nil
}
