package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	LegacyConfigVersion = 1
	ConfigVersion       = 2
	DefaultTablePrefix  = "yk"
	ConfigRelativePath  = ".yunka/project.json"
)

var tablePrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

type DatabaseConfig struct {
	TablePrefix string `json:"tablePrefix"`
}

type ContractProfile struct {
	Sources   string `json:"sources,omitempty"`
	ProtoRoot string `json:"protoRoot,omitempty"`
	Generated string `json:"generated"`
}

type ModuleProfile struct {
	Root string `json:"root"`
}

type GeneratedGoProfile struct {
	Root   string `json:"root"`
	Import string `json:"import,omitempty"`
}

type DevProfile struct {
	Manifest string `json:"manifest"`
}

type WorkflowProfile struct {
	Contract    ContractProfile    `json:"contract"`
	Modules     ModuleProfile      `json:"modules"`
	GeneratedGo GeneratedGoProfile `json:"generatedGo"`
	Dev         DevProfile         `json:"dev"`
}

type Config struct {
	Version  int             `json:"version"`
	Database DatabaseConfig  `json:"database"`
	Workflow WorkflowProfile `json:"workflow"`
}

type legacyConfig struct {
	Version  int            `json:"version"`
	Database DatabaseConfig `json:"database"`
}

func DefaultConfig() Config {
	return Config{
		Version:  ConfigVersion,
		Database: DatabaseConfig{TablePrefix: DefaultTablePrefix},
		Workflow: WorkflowProfile{
			Contract: ContractProfile{
				ProtoRoot: "contracts/proto",
				Generated: "contracts/generated",
			},
			Modules:     ModuleProfile{Root: "modules"},
			GeneratedGo: GeneratedGoProfile{Root: "internal"},
			Dev:         DevProfile{Manifest: ".yunka/dev.json"},
		},
	}
}

func Initialize(root, requestedPrefix string) (Config, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(absolute, filepath.FromSlash(ConfigRelativePath))
	if current, sourceVersion, err := loadWithVersion(absolute); err == nil {
		requestedPrefix = strings.TrimSpace(requestedPrefix)
		if requestedPrefix != "" && requestedPrefix != current.Database.TablePrefix {
			return Config{}, fmt.Errorf("project: database prefix is already %q; refusing to change it to %q", current.Database.TablePrefix, requestedPrefix)
		}
		if sourceVersion != ConfigVersion {
			if err := write(path, current); err != nil {
				return Config{}, err
			}
		}
		return current, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	prefix := strings.TrimSpace(requestedPrefix)
	if prefix == "" {
		prefix = DefaultTablePrefix
	}
	if err := ValidateTablePrefix(prefix); err != nil {
		return Config{}, err
	}
	config := defaultsForRoot(absolute)
	config.Database.TablePrefix = prefix
	if err := write(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func Ensure(root string) (Config, error) {
	config, err := Load(root)
	if err == nil {
		return config, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	return Initialize(root, "")
}

func LoadOrDefault(root string) (Config, error) {
	config, err := Load(root)
	if err == nil {
		return config, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		absolute, absErr := absoluteRoot(root)
		if absErr != nil {
			return Config{}, absErr
		}
		return defaultsForRoot(absolute), nil
	}
	return Config{}, err
}

// Load accepts both the legacy database-only v1 file and the C11.2 v2 profile.
// Legacy files are upgraded in memory only; callers that merely inspect/check a
// project never mutate it. Initialize is the explicit persistence/migration path.
func Load(root string) (Config, error) {
	config, _, err := loadWithVersion(root)
	return config, err
}

func loadWithVersion(root string) (Config, int, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return Config{}, 0, err
	}
	path := filepath.Join(absolute, filepath.FromSlash(ConfigRelativePath))
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, 0, err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(contents, &header); err != nil {
		return Config{}, 0, fmt.Errorf("project: decode %s: %w", ConfigRelativePath, err)
	}
	switch header.Version {
	case LegacyConfigVersion:
		var legacy legacyConfig
		if err := json.Unmarshal(contents, &legacy); err != nil {
			return Config{}, 0, fmt.Errorf("project: decode legacy %s: %w", ConfigRelativePath, err)
		}
		config := defaultsForRoot(absolute)
		config.Database = legacy.Database
		normalize(&config)
		if err := Validate(config); err != nil {
			return Config{}, 0, err
		}
		return config, LegacyConfigVersion, nil
	case ConfigVersion:
		var config Config
		if err := json.Unmarshal(contents, &config); err != nil {
			return Config{}, 0, fmt.Errorf("project: decode %s: %w", ConfigRelativePath, err)
		}
		normalize(&config)
		if err := Validate(config); err != nil {
			return Config{}, 0, err
		}
		return config, ConfigVersion, nil
	default:
		return Config{}, 0, fmt.Errorf("project: config version %d is unsupported", header.Version)
	}
}

func Validate(config Config) error {
	if config.Version != ConfigVersion {
		return fmt.Errorf("project: config version %d is unsupported", config.Version)
	}
	if err := ValidateTablePrefix(config.Database.TablePrefix); err != nil {
		return err
	}
	if (config.Workflow.Contract.Sources == "") == (config.Workflow.Contract.ProtoRoot == "") {
		return errors.New("project: workflow.contract must set exactly one of sources or protoRoot")
	}
	for name, value := range map[string]string{
		"workflow.contract.generated": config.Workflow.Contract.Generated,
		"workflow.modules.root":       config.Workflow.Modules.Root,
		"workflow.generatedGo.root":   config.Workflow.GeneratedGo.Root,
		"workflow.dev.manifest":        config.Workflow.Dev.Manifest,
	} {
		if err := validateProjectPath(name, value); err != nil {
			return err
		}
	}
	if config.Workflow.Contract.Sources != "" {
		if err := validateProjectPath("workflow.contract.sources", config.Workflow.Contract.Sources); err != nil {
			return err
		}
	}
	if config.Workflow.Contract.ProtoRoot != "" {
		if err := validateProjectPath("workflow.contract.protoRoot", config.Workflow.Contract.ProtoRoot); err != nil {
			return err
		}
	}
	if value := strings.TrimSpace(config.Workflow.GeneratedGo.Import); value != "" {
		if strings.ContainsAny(value, " \t\r\n") || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
			return fmt.Errorf("project: workflow.generatedGo.import %q is not a valid Go import root", value)
		}
	}
	return nil
}

func ValidateTablePrefix(prefix string) error {
	if !tablePrefixPattern.MatchString(strings.TrimSpace(prefix)) {
		return fmt.Errorf("project: database table prefix %q must match %s", prefix, tablePrefixPattern)
	}
	return nil
}

func absoluteRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	return filepath.Abs(root)
}

func defaultsForRoot(root string) Config {
	config := DefaultConfig()
	if info, err := os.Stat(filepath.Join(root, "contracts", "sources.json")); err == nil && !info.IsDir() {
		config.Workflow.Contract.Sources = "contracts/sources.json"
		config.Workflow.Contract.ProtoRoot = ""
	}
	return config
}

func normalize(config *Config) {
	if config == nil {
		return
	}
	config.Version = ConfigVersion
	config.Database.TablePrefix = strings.TrimSpace(config.Database.TablePrefix)
	config.Workflow.Contract.Sources = normalizeProjectPath(config.Workflow.Contract.Sources)
	config.Workflow.Contract.ProtoRoot = normalizeProjectPath(config.Workflow.Contract.ProtoRoot)
	config.Workflow.Contract.Generated = normalizeProjectPath(config.Workflow.Contract.Generated)
	config.Workflow.Modules.Root = normalizeProjectPath(config.Workflow.Modules.Root)
	config.Workflow.GeneratedGo.Root = normalizeProjectPath(config.Workflow.GeneratedGo.Root)
	config.Workflow.GeneratedGo.Import = strings.Trim(strings.TrimSpace(config.Workflow.GeneratedGo.Import), "/")
	config.Workflow.Dev.Manifest = normalizeProjectPath(config.Workflow.Dev.Manifest)
}

func normalizeProjectPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
}

func validateProjectPath(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("project: %s is required", name)
	}
	if filepath.IsAbs(filepath.FromSlash(value)) {
		return fmt.Errorf("project: %s must be project-relative, got %q", name, value)
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("project: %s must stay inside the project root, got %q", name, value)
	}
	return nil
}

func write(path string, config Config) error {
	normalize(&config)
	if err := Validate(config); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".project-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
