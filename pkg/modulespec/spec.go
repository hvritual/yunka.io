package modulespec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	Filename       = "module.yunka.json"
	SchemaVersion  = 1
	EvidenceSource = "module-spec"
)

var (
	namePattern      = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	configKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,256}$`)
)

type Requirements struct {
	ConfigKey string   `json:"configKey,omitempty"`
	Logger    bool     `json:"logger,omitempty"`
	Databases []string `json:"databases,omitempty"`
	EventBus  bool     `json:"eventBus,omitempty"`
	RPC       []string `json:"rpc,omitempty"`
}

type Spec struct {
	SchemaVersion int          `json:"schemaVersion"`
	Version       string       `json:"version,omitempty"`
	DependsOn     []string     `json:"dependsOn,omitempty"`
	Requirements  Requirements `json:"requirements,omitempty"`
}

func Default() Spec {
	return Spec{SchemaVersion: SchemaVersion, Version: "v0.1.0"}
}

func Normalize(spec Spec) Spec {
	spec.Version = strings.TrimSpace(spec.Version)
	if spec.Version == "" {
		spec.Version = "v0.1.0"
	}
	spec.DependsOn = normalizeNames(spec.DependsOn)
	spec.Requirements.ConfigKey = strings.TrimSpace(spec.Requirements.ConfigKey)
	spec.Requirements.Databases = normalizeNames(spec.Requirements.Databases)
	spec.Requirements.RPC = normalizeNames(spec.Requirements.RPC)
	return spec
}

func Validate(spec Spec) error {
	if spec.SchemaVersion != SchemaVersion {
		return fmt.Errorf("module spec: unsupported schemaVersion %d; expected %d", spec.SchemaVersion, SchemaVersion)
	}
	if strings.TrimSpace(spec.Version) == "" {
		return fmt.Errorf("module spec: version is required")
	}
	if spec.Requirements.ConfigKey != "" && !configKeyPattern.MatchString(spec.Requirements.ConfigKey) {
		return fmt.Errorf("module spec: invalid configKey %q", spec.Requirements.ConfigKey)
	}
	for _, dependency := range spec.DependsOn {
		if !namePattern.MatchString(dependency) {
			return fmt.Errorf("module spec: invalid dependency %q", dependency)
		}
	}
	for _, database := range spec.Requirements.Databases {
		if !namePattern.MatchString(database) {
			return fmt.Errorf("module spec: invalid database requirement %q", database)
		}
	}
	for _, rpc := range spec.Requirements.RPC {
		if !namePattern.MatchString(rpc) {
			return fmt.Errorf("module spec: invalid RPC requirement %q", rpc)
		}
	}
	return nil
}

func ValidateForModule(name string, spec Spec) error {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return fmt.Errorf("module spec: invalid module name %q", name)
	}
	if err := Validate(spec); err != nil {
		return err
	}
	for _, dependency := range spec.DependsOn {
		if dependency == name {
			return fmt.Errorf("module spec: module %q depends on itself", name)
		}
	}
	return nil
}

func Load(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("module spec: decode %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Spec{}, fmt.Errorf("module spec: decode %s: trailing JSON value", path)
		}
		return Spec{}, fmt.Errorf("module spec: decode %s: %w", path, err)
	}
	spec = Normalize(spec)
	if err := Validate(spec); err != nil {
		return Spec{}, fmt.Errorf("module spec: %s: %w", path, err)
	}
	return spec, nil
}

func Marshal(spec Spec) ([]byte, error) {
	if spec.SchemaVersion == 0 {
		spec.SchemaVersion = SchemaVersion
	}
	spec = Normalize(spec)
	if err := Validate(spec); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func normalizeNames(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
