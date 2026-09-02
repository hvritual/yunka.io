package devruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
)

const (
	LegacyDevSchemaVersion      = 1
	DevSchemaVersion            = 2
	RuntimeClosureSchemaVersion = 3

	DefaultReadinessTimeout  = 30 * time.Second
	DefaultReadinessInterval = 250 * time.Millisecond
	MinReadinessInterval     = 10 * time.Millisecond
	MaxReadinessInterval     = 30 * time.Second
	MaxReadinessTimeout      = 5 * time.Minute
	MaxReadinessURLBytes     = 2048

	DefaultRuntimeApplication      = "yunka"
	DefaultRuntimeStatePath        = ".yunka/dev-runtime.json"
	DefaultRuntimeGraphPath        = ".yunka/runtime-graph.json"
	DefaultRuntimeContractManifest = "contracts/generated/manifest.json"
	DefaultRuntimeShutdownTimeout  = 10 * time.Second
	MinRuntimeShutdownTimeout      = 100 * time.Millisecond
	MaxRuntimeShutdownTimeout      = 5 * time.Minute
)

type DevManifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Runtime       *RuntimeConfig `json:"runtime,omitempty"`
	Processes     []Process      `json:"processes"`
}

type RuntimeConfig struct {
	Application      string `json:"application,omitempty"`
	StatePath        string `json:"statePath,omitempty"`
	GraphPath        string `json:"graphPath,omitempty"`
	ContractManifest string `json:"contractManifest,omitempty"`
	ShutdownTimeout  string `json:"shutdownTimeout,omitempty"`
	Closure          bool   `json:"closure,omitempty"`
}

type Process struct {
	Name       string     `json:"name"`
	Command    []string   `json:"command"`
	WorkingDir string     `json:"workingDir,omitempty"`
	DependsOn  []string   `json:"dependsOn,omitempty"`
	GraphNode  string     `json:"graphNode,omitempty"`
	GraphNodes []string   `json:"graphNodes,omitempty"`
	InheritEnv []string   `json:"inheritEnv,omitempty"`
	Readiness  *Readiness `json:"readiness,omitempty"`
}

// Readiness is an explicit HTTP barrier for a process. Dependents are not
// started until the configured endpoint succeeds. DiagnosticsReady additionally
// requires the W01/W07 response field core.health.ready to be true.
type Readiness struct {
	URL                string `json:"url"`
	Timeout            string `json:"timeout,omitempty"`
	Interval           string `json:"interval,omitempty"`
	ExpectedStatus     int    `json:"expectedStatus,omitempty"`
	DiagnosticsReady   bool   `json:"diagnosticsReady,omitempty"`
	CaptureDiagnostics bool   `json:"captureDiagnostics,omitempty"`
	TokenEnv           string `json:"tokenEnv,omitempty"`
}

func LoadDevManifest(path string) (DevManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DevManifest{}, err
	}
	var manifest DevManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return DevManifest{}, err
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = LegacyDevSchemaVersion
	}
	if !supportedDevSchema(manifest.SchemaVersion) {
		return DevManifest{}, fmt.Errorf("devruntime: unsupported manifest schema version %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func (manifest DevManifest) Validate(root string, graph applicationgraph.Graph) error {
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = LegacyDevSchemaVersion
	}
	if !supportedDevSchema(manifest.SchemaVersion) {
		return fmt.Errorf("devruntime: unsupported manifest schema version %d", manifest.SchemaVersion)
	}
	if manifest.Runtime != nil && manifest.SchemaVersion < RuntimeClosureSchemaVersion {
		return fmt.Errorf("devruntime: runtime closure configuration requires schemaVersion %d", RuntimeClosureSchemaVersion)
	}
	if manifest.SchemaVersion >= RuntimeClosureSchemaVersion {
		runtime := normalizeRuntimeConfig(manifest.Runtime)
		if _, err := runtimeShutdownDuration(runtime); err != nil {
			return fmt.Errorf("devruntime: runtime shutdownTimeout: %w", err)
		}
		statePath, err := resolveArtifactPath(root, runtime.StatePath)
		if err != nil {
			return fmt.Errorf("devruntime: runtime statePath: %w", err)
		}
		graphPath, err := resolveArtifactPath(root, runtime.GraphPath)
		if err != nil {
			return fmt.Errorf("devruntime: runtime graphPath: %w", err)
		}
		if statePath == graphPath {
			return errors.New("devruntime: runtime statePath and graphPath must differ")
		}
		if _, err := resolveArtifactPath(root, runtime.ContractManifest); err != nil {
			return fmt.Errorf("devruntime: runtime contractManifest: %w", err)
		}
	}

	names := make(map[string]Process, len(manifest.Processes))
	rawDependencies := make(map[string][]string, len(manifest.Processes))
	graphIndex := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		graphIndex[node.ID] = struct{}{}
	}
	for _, original := range manifest.Processes {
		if err := validateProcessGraphOwnership(original); err != nil {
			return fmt.Errorf("devruntime: process %q graph ownership: %w", strings.TrimSpace(original.Name), err)
		}
		if len(original.GraphNodes) != 0 && manifest.SchemaVersion < RuntimeClosureSchemaVersion {
			return fmt.Errorf("devruntime: process %q graphNodes requires schemaVersion %d", strings.TrimSpace(original.Name), RuntimeClosureSchemaVersion)
		}
		rawProcessDependencies := append([]string(nil), original.DependsOn...)
		process := normalizeProcess(original)
		name := process.Name
		if name == "" {
			return errors.New("devruntime: process name is required")
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("devruntime: duplicate process %q", name)
		}
		if len(process.Command) == 0 || strings.TrimSpace(process.Command[0]) == "" {
			return fmt.Errorf("devruntime: process %q command is required", name)
		}
		for index, argument := range process.Command {
			if strings.IndexByte(argument, 0) >= 0 {
				return fmt.Errorf("devruntime: process %q command argument %d contains NUL", name, index)
			}
		}
		if _, err := resolveWorkingDir(root, process.WorkingDir); err != nil {
			return fmt.Errorf("devruntime: process %q: %w", name, err)
		}
		if len(graphIndex) > 0 {
			for _, graphNode := range processOwnedGraphNodes(process) {
				if _, ok := graphIndex[graphNode]; !ok {
					return fmt.Errorf("devruntime: process %q graph node %q not found", name, graphNode)
				}
			}
		}
		for _, environmentName := range process.InheritEnv {
			if !validEnvironmentName(environmentName) {
				return fmt.Errorf("devruntime: process %q invalid environment name %q", name, environmentName)
			}
		}
		if process.Readiness != nil && manifest.SchemaVersion < DevSchemaVersion {
			return fmt.Errorf("devruntime: process %q readiness requires schemaVersion %d", name, DevSchemaVersion)
		}
		if process.Readiness != nil && process.Readiness.CaptureDiagnostics && manifest.SchemaVersion < RuntimeClosureSchemaVersion {
			return fmt.Errorf("devruntime: process %q captureDiagnostics requires schemaVersion %d", name, RuntimeClosureSchemaVersion)
		}
		if err := validateReadiness(process.Readiness); err != nil {
			return fmt.Errorf("devruntime: process %q readiness: %w", name, err)
		}
		names[name] = process
		rawDependencies[name] = rawProcessDependencies
	}

	for _, process := range names {
		seen := make(map[string]struct{}, len(rawDependencies[process.Name]))
		for _, dependency := range rawDependencies[process.Name] {
			dependency = strings.TrimSpace(dependency)
			if dependency == "" {
				continue
			}
			if dependency == process.Name {
				return fmt.Errorf("devruntime: process %q depends on itself", process.Name)
			}
			if _, ok := names[dependency]; !ok {
				return fmt.Errorf("devruntime: process %q dependency %q not found", process.Name, dependency)
			}
			if _, duplicate := seen[dependency]; duplicate {
				return fmt.Errorf("devruntime: process %q duplicate dependency %q", process.Name, dependency)
			}
			seen[dependency] = struct{}{}
		}
	}
	return nil
}

func supportedDevSchema(version int) bool {
	return version == LegacyDevSchemaVersion || version == DevSchemaVersion || version == RuntimeClosureSchemaVersion
}

func normalizeRuntimeConfig(input *RuntimeConfig) RuntimeConfig {
	var runtime RuntimeConfig
	if input != nil {
		runtime = *input
	}
	runtime.Application = strings.TrimSpace(runtime.Application)
	if runtime.Application == "" {
		runtime.Application = DefaultRuntimeApplication
	}
	runtime.StatePath = strings.TrimSpace(runtime.StatePath)
	if runtime.StatePath == "" {
		runtime.StatePath = DefaultRuntimeStatePath
	}
	runtime.GraphPath = strings.TrimSpace(runtime.GraphPath)
	if runtime.GraphPath == "" {
		runtime.GraphPath = DefaultRuntimeGraphPath
	}
	runtime.ContractManifest = strings.TrimSpace(runtime.ContractManifest)
	if runtime.ContractManifest == "" {
		runtime.ContractManifest = DefaultRuntimeContractManifest
	}
	runtime.ShutdownTimeout = strings.TrimSpace(runtime.ShutdownTimeout)
	if runtime.ShutdownTimeout == "" {
		runtime.ShutdownTimeout = DefaultRuntimeShutdownTimeout.String()
	}
	return runtime
}

func runtimeShutdownDuration(runtime RuntimeConfig) (time.Duration, error) {
	value := strings.TrimSpace(runtime.ShutdownTimeout)
	if value == "" {
		return DefaultRuntimeShutdownTimeout, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < MinRuntimeShutdownTimeout || duration > MaxRuntimeShutdownTimeout {
		return 0, fmt.Errorf("must be between %s and %s", MinRuntimeShutdownTimeout, MaxRuntimeShutdownTimeout)
	}
	return duration, nil
}

func normalizeProcess(process Process) Process {
	process.Name = strings.TrimSpace(process.Name)
	process.WorkingDir = strings.TrimSpace(process.WorkingDir)
	process.GraphNode = strings.TrimSpace(process.GraphNode)
	process.GraphNodes = stableStrings(process.GraphNodes)
	process.Command = append([]string(nil), process.Command...)
	if len(process.Command) > 0 {
		process.Command[0] = strings.TrimSpace(process.Command[0])
	}
	process.DependsOn = stableStrings(process.DependsOn)
	process.InheritEnv = stableStrings(process.InheritEnv)
	if process.Readiness != nil {
		clone := *process.Readiness
		clone.URL = strings.TrimSpace(clone.URL)
		clone.Timeout = strings.TrimSpace(clone.Timeout)
		clone.Interval = strings.TrimSpace(clone.Interval)
		clone.TokenEnv = strings.TrimSpace(clone.TokenEnv)
		process.Readiness = &clone
	}
	return process
}

func validateReadiness(readiness *Readiness) error {
	if readiness == nil {
		return nil
	}
	if len(readiness.URL) == 0 {
		return errors.New("URL is required")
	}
	if len(readiness.URL) > MaxReadinessURLBytes {
		return fmt.Errorf("URL must not exceed %d bytes", MaxReadinessURLBytes)
	}
	parsed, err := url.Parse(readiness.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return errors.New("URL host is required")
	}
	if parsed.User != nil {
		return errors.New("URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return errors.New("URL must not contain a fragment")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return errors.New("plain HTTP readiness is restricted to loopback IP addresses; use HTTPS for remote probes")
	}
	if _, _, err := readinessDurations(readiness); err != nil {
		return err
	}
	if readiness.ExpectedStatus != 0 && (readiness.ExpectedStatus < 200 || readiness.ExpectedStatus > 299) {
		return errors.New("expectedStatus must be between 200 and 299")
	}
	if readiness.TokenEnv != "" && !validEnvironmentName(readiness.TokenEnv) {
		return fmt.Errorf("invalid token environment name %q", readiness.TokenEnv)
	}
	return nil
}

func readinessDurations(readiness *Readiness) (time.Duration, time.Duration, error) {
	timeout := DefaultReadinessTimeout
	interval := DefaultReadinessInterval
	if readiness == nil {
		return timeout, interval, nil
	}
	var err error
	if readiness.Timeout != "" {
		timeout, err = time.ParseDuration(readiness.Timeout)
		if err != nil || timeout <= 0 || timeout > MaxReadinessTimeout {
			return 0, 0, fmt.Errorf("timeout must be greater than zero and at most %s", MaxReadinessTimeout)
		}
	}
	if readiness.Interval != "" {
		interval, err = time.ParseDuration(readiness.Interval)
		if err != nil || interval < MinReadinessInterval || interval > MaxReadinessInterval {
			return 0, 0, fmt.Errorf("interval must be between %s and %s", MinReadinessInterval, MaxReadinessInterval)
		}
	}
	if interval > timeout {
		return 0, 0, errors.New("interval must not exceed timeout")
	}
	return timeout, interval, nil
}

func isLoopbackHost(host string) bool {
	if zone := strings.LastIndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validEnvironmentName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for index, current := range value {
		if (current >= 'A' && current <= 'Z') || (current >= 'a' && current <= 'z') || current == '_' || (index > 0 && current >= '0' && current <= '9') {
			continue
		}
		return false
	}
	return true
}

func resolveWorkingDir(root, value string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal := root
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		rootReal = resolved
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return root, nil
	}
	if filepath.IsAbs(value) {
		return "", errors.New("workingDir must be relative to project root")
	}
	candidate := filepath.Clean(filepath.Join(root, value))
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("workingDir escapes project root")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
		realRelative, relErr := filepath.Rel(rootReal, resolved)
		if relErr != nil {
			return "", relErr
		}
		if realRelative == ".." || strings.HasPrefix(realRelative, ".."+string(filepath.Separator)) {
			return "", errors.New("workingDir resolves outside project root")
		}
	}
	return candidate, nil
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
