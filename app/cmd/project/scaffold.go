package project

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	bootstrapContractName = "yunka_bootstrap.proto"
	bootstrapCommandPath  = "./cmd/yunka-bootstrap"
)

type ScaffoldReport struct {
	Directories         []string
	BootstrapContract   string
	BootstrapEntrypoint string
	DevManifest         string
	DevSkipped          string
}

// Scaffold materializes the non-business project shape referenced by the
// project profile. It is deliberately idempotent and never overwrites
// developer-owned files. Existing applications are preferred for the local dev
// manifest; a tiny standard-library bootstrap entrypoint is created only when a
// Go module exists but no runnable main package has been declared yet.
func Scaffold(root string, config Config) (ScaffoldReport, error) {
	if err := Validate(config); err != nil {
		return ScaffoldReport{}, err
	}
	absolute, err := absoluteRoot(root)
	if err != nil {
		return ScaffoldReport{}, err
	}
	directories := []string{
		".yunka",
		config.Workflow.Contract.Generated,
		config.Workflow.Modules.Root,
		config.Workflow.GeneratedGo.Root,
		filepath.ToSlash(filepath.Dir(config.Workflow.Dev.Manifest)),
	}
	if config.Workflow.Contract.ProtoRoot != "" {
		directories = append(directories, config.Workflow.Contract.ProtoRoot)
	} else if config.Workflow.Contract.Sources != "" {
		directories = append(directories, filepath.ToSlash(filepath.Dir(config.Workflow.Contract.Sources)))
	}
	directories = stableProjectPaths(directories)
	for _, relative := range directories {
		if relative == "." || relative == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(absolute, filepath.FromSlash(relative)), 0o750); err != nil {
			return ScaffoldReport{}, err
		}
	}
	report := ScaffoldReport{Directories: directories}

	if config.Workflow.Contract.ProtoRoot != "" {
		protoRoot := filepath.Join(absolute, filepath.FromSlash(config.Workflow.Contract.ProtoRoot))
		hasProto, err := containsProtoFile(protoRoot)
		if err != nil {
			return ScaffoldReport{}, err
		}
		if !hasProto {
			relative := filepath.ToSlash(filepath.Join(config.Workflow.Contract.ProtoRoot, bootstrapContractName))
			path := filepath.Join(absolute, filepath.FromSlash(relative))
			if err := writeIfMissing(path, []byte(bootstrapContract)); err != nil {
				return ScaffoldReport{}, err
			}
			report.BootstrapContract = relative
		}
	}

	manifestPath := filepath.Join(absolute, filepath.FromSlash(config.Workflow.Dev.Manifest))
	if _, err := os.Stat(manifestPath); err == nil {
		report.DevManifest = config.Workflow.Dev.Manifest
		return report, nil
	} else if !os.IsNotExist(err) {
		return ScaffoldReport{}, err
	}

	if _, err := os.Stat(filepath.Join(absolute, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			report.DevSkipped = "go.mod is required before a runnable dev manifest can be scaffolded"
			return report, nil
		}
		return ScaffoldReport{}, err
	}
	mains, err := discoverMainPackages(absolute)
	if err != nil {
		return ScaffoldReport{}, err
	}
	bootstrap := false
	if len(mains) == 0 {
		entrypoint := filepath.Join(absolute, "cmd", "yunka-bootstrap", "main.go")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o750); err != nil {
			return ScaffoldReport{}, err
		}
		if err := writeIfMissing(entrypoint, []byte(bootstrapMain)); err != nil {
			return ScaffoldReport{}, err
		}
		report.BootstrapEntrypoint = "cmd/yunka-bootstrap/main.go"
		mains = []string{bootstrapCommandPath}
		bootstrap = true
	}
	if len(mains) != 1 {
		report.DevSkipped = fmt.Sprintf("multiple runnable main packages found: %s; declare .yunka/dev.json explicitly", strings.Join(mains, ", "))
		return report, nil
	}
	contents, err := devManifestBytes(mains[0], bootstrap)
	if err != nil {
		return ScaffoldReport{}, err
	}
	if err := writeIfMissing(manifestPath, contents); err != nil {
		return ScaffoldReport{}, err
	}
	report.DevManifest = config.Workflow.Dev.Manifest
	return report, nil
}

func stableProjectPaths(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
		if value == "" || value == "." {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsProtoFile(root string) (bool, error) {
	found := false
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			found = true
		}
		return nil
	})
	return found, err
}

func discoverMainPackages(root string) ([]string, error) {
	var candidates []string
	if ok, err := directoryHasMainPackage(root); err != nil {
		return nil, err
	} else if ok {
		candidates = append(candidates, ".")
	}
	cmdRoot := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdRoot)
	if os.IsNotExist(err) {
		return candidates, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		directory := filepath.Join(cmdRoot, entry.Name())
		ok, err := directoryHasMainPackage(directory)
		if err != nil {
			return nil, err
		}
		if ok {
			candidates = append(candidates, "./"+filepath.ToSlash(filepath.Join("cmd", entry.Name())))
		}
	}
	sort.Strings(candidates)
	return candidates, nil
}

func directoryHasMainPackage(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			return false, err
		}
		if file.Name != nil && file.Name.Name == "main" {
			return true, nil
		}
	}
	return false, nil
}

func devManifestBytes(target string, bootstrap bool) ([]byte, error) {
	process := map[string]any{
		"name":    "app",
		"command": []string{"go", "run", target},
	}
	if bootstrap {
		process["readiness"] = map[string]any{
			"url":            "http://127.0.0.1:8080/ready",
			"timeout":        "30s",
			"expectedStatus": 200,
		}
	}
	manifest := map[string]any{
		"schemaVersion": 2,
		"processes":     []any{process},
	}
	contents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func writeIfMissing(path string, contents []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

const bootstrapContract = `syntax = "proto3";

package yunka.bootstrap.v1;

// Bootstrap keeps a newly initialized project generatable before its first
// business contract is added. Delete this file once real contracts exist.
message Bootstrap {}
`

const bootstrapMain = `// Code generated as an editable Yunka bootstrap entrypoint.
// Replace this process with the assembled application entrypoint when the first
// business operation is added.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ready\n"))
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errorsCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsCh <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errorsCh:
		log.Fatal(err)
	case <-signals:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("bootstrap shutdown: %v", err)
	}
}
`
