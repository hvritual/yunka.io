package projectflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

// ContractEdit is disposable evidence from two compilations of one captured input
// set. Only the requested source differs. Neither compilation writes the project.
type ContractEdit struct {
	Before       contractcore.Manifest
	After        contractcore.Manifest
	InputsDigest string
}

type contractInputs struct {
	project     resolvedProject
	files       map[string][]byte
	dirs        map[string]bool
	includeKeys []string
	owned       map[string]bool
}

// PreviewContractEdit compiles real protobuf, not a synthetic Manifest or stale
// generated output. Source uses the existing project-relative ownership namespace.
func PreviewContractEdit(ctx context.Context, options Options, source string, original, replacement []byte) (ContractEdit, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	input, err := captureContractInputs(ctx, options)
	if err != nil {
		return ContractEdit{}, err
	}
	key := "project/" + source
	if !input.owned[source] || !bytes.Equal(input.files[key], original) {
		return ContractEdit{}, fmt.Errorf("contract edit: source is not owned or changed during preparation: %s", source)
	}
	digest := input.digest()
	directory, err := os.MkdirTemp("", "yunka-contract-edit-*")
	if err != nil {
		return ContractEdit{}, err
	}
	defer os.RemoveAll(directory)
	for dir := range input.dirs {
		if err := os.MkdirAll(filepath.Join(directory, filepath.FromSlash(dir)), 0700); err != nil {
			return ContractEdit{}, err
		}
	}
	for name, data := range input.files {
		if err := ctx.Err(); err != nil {
			return ContractEdit{}, err
		}
		destination := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return ContractEdit{}, err
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			return ContractEdit{}, err
		}
	}
	p := input.project
	p.Root = filepath.Join(directory, "project")
	if p.InventoryPath != "" {
		p.InventoryPath = filepath.Join(p.Root, relative(input.project.Root, p.InventoryPath))
	}
	if p.ProtoDir != "" {
		p.ProtoDir = filepath.Join(p.Root, relative(input.project.Root, p.ProtoDir))
	}
	p.AdditionalProtoPaths = nil
	for _, name := range input.includeKeys {
		p.AdditionalProtoPaths = append(p.AdditionalProtoPaths, filepath.Join(directory, filepath.FromSlash(name)))
	}
	before, err := compileContract(ctx, p)
	if err != nil {
		return ContractEdit{}, fmt.Errorf("contract edit before: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, filepath.FromSlash(key)), replacement, 0600); err != nil {
		return ContractEdit{}, err
	}
	after, err := compileContract(ctx, p)
	if err != nil {
		return ContractEdit{}, fmt.Errorf("contract edit after: %w", err)
	}
	current, err := ContractInputsDigest(ctx, options)
	if err != nil {
		return ContractEdit{}, err
	}
	if current != digest {
		return ContractEdit{}, fmt.Errorf("contract edit: compiler inputs changed during preview")
	}
	return ContractEdit{Before: before.Manifest, After: after.Manifest, InputsDigest: digest}, nil
}

// ContractInputsDigest binds source bytes, explicit include order/content and
// project resolution metadata, without persisting another source of truth.
func ContractInputsDigest(ctx context.Context, options Options) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	input, err := captureContractInputs(ctx, options)
	if err != nil {
		return "", err
	}
	return input.digest(), nil
}

func captureContractInputs(ctx context.Context, options Options) (contractInputs, error) {
	input := contractInputs{files: map[string][]byte{}, dirs: map[string]bool{"project": true}, owned: map[string]bool{}}
	if err := ctx.Err(); err != nil {
		return input, err
	}
	project, err := resolveProject(options)
	if err != nil {
		return input, err
	}
	input.project = project
	if project.InventoryPath != "" && len(options.ProtoPaths) > 0 {
		return input, fmt.Errorf("contract edit: inventory includes belong in sourceSets[].protoPaths")
	}
	for _, path := range options.ProtoPaths {
		if strings.TrimSpace(path) == "" {
			return input, fmt.Errorf("contract edit: --proto-path must not be blank")
		}
	}
	// Resolution metadata is part of the bound input even when it is ignored by Git.
	metadata := []string{filepath.Join(project.Root, "go.mod"), filepath.Join(project.Root, ".yunka/project.json")}
	if project.InventoryPath != "" {
		metadata = append(metadata, project.InventoryPath)
	}
	for _, path := range metadata {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return input, err
		}
		if !info.Mode().IsRegular() {
			return input, fmt.Errorf("contract edit: metadata must be a regular file: %s", path)
		}
		physical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return input, err
		}
		if physical != filepath.Clean(path) || !editRelativePath(relative(project.Root, path)) {
			return input, fmt.Errorf("contract edit: aliased or escaping metadata: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return input, err
		}
		input.files["project/"+relative(project.Root, path)] = data
	}
	sets, err := protobufSourceSets(project)
	if err != nil {
		return input, err
	}
	for _, set := range sets {
		for _, file := range set.Files {
			name := relative(project.Root, filepath.Join(set.Root, filepath.FromSlash(file)))
			if !editRelativePath(name) {
				return input, fmt.Errorf("contract edit: source escapes project: %s", name)
			}
			input.owned[name] = true
		}
		roots := append([]string{set.Root}, set.ProtoPaths...)
		for _, root := range roots {
			rel := relative(project.Root, root)
			key := ""
			if rel == "." || editRelativePath(rel) {
				key = "project"
				if rel != "." {
					key += "/" + rel
				}
			} else {
				// Only ordinary proto-root projects accept explicitly external include roots.
				if project.InventoryPath != "" {
					return input, fmt.Errorf("contract edit: inventory root escapes project")
				}
				index := -1
				for i, p := range project.AdditionalProtoPaths {
					if p == root {
						index = i
						break
					}
				}
				if index < 0 {
					return input, fmt.Errorf("contract edit: source root escapes project")
				}
				key = fmt.Sprintf("includes/%04d", index)
			}
			if err := input.captureRoot(ctx, root, key); err != nil {
				return input, err
			}
		}
	}
	for i, root := range project.AdditionalProtoPaths {
		rel := relative(project.Root, root)
		index := i
		for j, previous := range project.AdditionalProtoPaths[:i] {
			if previous == root {
				index = j
				break
			}
		}
		key := fmt.Sprintf("includes/%04d", index)
		if rel == "." {
			key = "project"
		} else if editRelativePath(rel) {
			key = "project/" + rel
		}
		input.includeKeys = append(input.includeKeys, key)
	}
	return input, nil
}
func (input *contractInputs) captureRoot(ctx context.Context, root, key string) error {
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	// Reject aliased source roots rather than silently weakening byte identity.
	if physical != filepath.Clean(root) {
		return fmt.Errorf("contract edit: symlinked compiler root is unsupported: %s", root)
	}
	input.dirs[key] = true
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("contract edit: symlink in compiler input root: %s", path)
		}
		if !strings.EqualFold(filepath.Ext(path), ".proto") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("contract edit: non-regular protobuf source: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := key + "/" + filepath.ToSlash(rel)
		if !editRelativePath(name) {
			return fmt.Errorf("contract edit: unsafe source path %s", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if previous, ok := input.files[name]; ok && !bytes.Equal(previous, data) {
			return fmt.Errorf("contract edit: input changed while capturing %s", name)
		}
		input.files[name] = data
		return nil
	})
}
func (input contractInputs) digest() string {
	type fileDigest struct {
		Name   string
		SHA256 string
	}
	names := make([]string, 0, len(input.files))
	for name := range input.files {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]fileDigest, 0, len(names))
	for _, name := range names {
		sum := sha256.Sum256(input.files[name])
		files = append(files, fileDigest{name, hex.EncodeToString(sum[:])})
	}
	data, _ := json.Marshal(struct {
		Project  ProjectDescriptor
		Includes []string
		Files    []fileDigest
	}{describeResolvedProject(input.project), input.includeKeys, files})
	sum := sha256.Sum256(append([]byte("operation-authoring-inputs/v1\n"), data...))
	return hex.EncodeToString(sum[:])
}
func editRelativePath(path string) bool {
	return path != "" && path != "." && path != ".." && !strings.HasPrefix(path, "../") && !filepath.IsAbs(path) && !strings.ContainsAny(path, "\\:\x00") && filepath.ToSlash(filepath.Clean(path)) == path
}
