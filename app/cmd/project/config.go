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
	ConfigVersion       = 1
	DefaultTablePrefix  = "yk"
	ConfigRelativePath  = ".yunka/project.json"
)

var tablePrefixPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

type DatabaseConfig struct {
	TablePrefix string `json:"tablePrefix"`
}

type Config struct {
	Version  int            `json:"version"`
	Database DatabaseConfig `json:"database"`
}

func DefaultConfig() Config {
	return Config{
		Version: ConfigVersion,
		Database: DatabaseConfig{TablePrefix: DefaultTablePrefix},
	}
}

func Initialize(root, requestedPrefix string) (Config, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(root) == "" {
		absolute, err = filepath.Abs(".")
		if err != nil {
			return Config{}, err
		}
	}
	path := filepath.Join(absolute, filepath.FromSlash(ConfigRelativePath))
	if current, err := Load(absolute); err == nil {
		requestedPrefix = strings.TrimSpace(requestedPrefix)
		if requestedPrefix != "" && requestedPrefix != current.Database.TablePrefix {
			return Config{}, fmt.Errorf("project: database prefix is already %q; refusing to change it to %q", current.Database.TablePrefix, requestedPrefix)
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
	config := DefaultConfig()
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
		return DefaultConfig(), nil
	}
	return Config{}, err
}

func Load(root string) (Config, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(root) == "" {
		absolute, err = filepath.Abs(".")
		if err != nil {
			return Config{}, err
		}
	}
	path := filepath.Join(absolute, filepath.FromSlash(ConfigRelativePath))
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(contents, &config); err != nil {
		return Config{}, fmt.Errorf("project: decode %s: %w", ConfigRelativePath, err)
	}
	if err := Validate(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func Validate(config Config) error {
	if config.Version != ConfigVersion {
		return fmt.Errorf("project: config version %d is unsupported", config.Version)
	}
	return ValidateTablePrefix(config.Database.TablePrefix)
}

func ValidateTablePrefix(prefix string) error {
	if !tablePrefixPattern.MatchString(strings.TrimSpace(prefix)) {
		return fmt.Errorf("project: database table prefix %q must match %s", prefix, tablePrefixPattern)
	}
	return nil
}

func write(path string, config Config) error {
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
