package projectflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

// ContractInputSnapshot is a detached canonical model and its captured input
// identity. ContentDigest excludes the absolute checkout root only; explicit
// source/include order, bytes and project resolution metadata remain bound.
// Neither digest signs the evidence or certifies the compiler executable.
type ContractInputSnapshot struct {
	Manifest      contractcore.Manifest
	InputsDigest  string
	ContentDigest string
}

// CaptureContractSnapshot compiles a private captured input tree, not generated
// artifacts or a live tree that can change between compiler reads.
func CaptureContractSnapshot(ctx context.Context, options Options) (ContractInputSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	input, err := captureContractInputs(ctx, options)
	if err != nil {
		return ContractInputSnapshot{}, err
	}
	p, cleanup, err := materializeContractInputs(ctx, input)
	if err != nil {
		return ContractInputSnapshot{}, err
	}
	defer cleanup()
	result, err := compileContract(ctx, p)
	if err != nil {
		return ContractInputSnapshot{}, fmt.Errorf("captured contract: %w", err)
	}
	digest, err := ContractInputsDigest(ctx, options)
	if err != nil {
		return ContractInputSnapshot{}, err
	}
	if digest != input.digest() {
		return ContractInputSnapshot{}, fmt.Errorf("captured contract: inputs changed during compilation")
	}
	return ContractInputSnapshot{Manifest: result.Manifest, InputsDigest: digest, ContentDigest: input.digestForRoot("")}, nil
}

// materializeContractInputs is shared with Operation preview. It preserves
// independent source-set namespaces and explicit external include ordering.
func materializeContractInputs(ctx context.Context, input contractInputs) (resolvedProject, func(), error) {
	directory, err := os.MkdirTemp("", "yunka-contract-input-*")
	if err != nil {
		return resolvedProject{}, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	failed := true
	defer func() {
		if failed {
			cleanup()
		}
	}()
	for dir := range input.dirs {
		if err := os.MkdirAll(filepath.Join(directory, filepath.FromSlash(dir)), 0700); err != nil {
			return resolvedProject{}, nil, err
		}
	}
	for name, data := range input.files {
		if err := ctx.Err(); err != nil {
			return resolvedProject{}, nil, err
		}
		destination := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return resolvedProject{}, nil, err
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			return resolvedProject{}, nil, err
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
	failed = false
	return p, cleanup, nil
}
