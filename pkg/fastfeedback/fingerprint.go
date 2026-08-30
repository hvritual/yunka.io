package fastfeedback

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

const (
	SchemaVersion     = 1
	Algorithm         = "sha256-tree-v1"
	CacheRelativePath = ".yunka/cache/fast-feedback.json"
)

type EngineIdentity struct {
	ID       string `json:"id"`
	Verified bool   `json:"verified"`
}

type Fingerprint struct {
	Digest string `json:"digest"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

type Metadata struct {
	SchemaVersion int            `json:"schemaVersion"`
	Algorithm     string         `json:"algorithm"`
	Engine        EngineIdentity `json:"engine"`
	Toolchain     string         `json:"toolchain"`
	Inputs        Fingerprint    `json:"inputs"`
	Outputs       Fingerprint    `json:"outputs"`
}

type Root struct {
	Label    string
	Path     string
	Optional bool
}

func CurrentEngineIdentity() EngineIdentity {
	info, ok := debug.ReadBuildInfo()
	if ok {
		settings := make(map[string]string, len(info.Settings))
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
		if revision := strings.TrimSpace(settings["vcs.revision"]); revision != "" {
			modified := strings.EqualFold(strings.TrimSpace(settings["vcs.modified"]), "true")
			return EngineIdentity{ID: "vcs:" + revision, Verified: !modified}
		}
		version := strings.TrimSpace(info.Main.Version)
		if version != "" && version != "(devel)" {
			path := strings.TrimSpace(info.Main.Path)
			if path == "" {
				path = "yunka"
			}
			return EngineIdentity{ID: "module:" + path + "@" + version, Verified: true}
		}
	}
	return EngineIdentity{ID: fmt.Sprintf("unverified:fast-feedback-schema-%d", SchemaVersion), Verified: false}
}

func FingerprintRoots(roots []Root) (Fingerprint, error) {
	ordered := append([]Root(nil), roots...)
	for index := range ordered {
		ordered[index].Label = strings.TrimSpace(ordered[index].Label)
		ordered[index].Path = strings.TrimSpace(ordered[index].Path)
		if ordered[index].Label == "" {
			return Fingerprint{}, fmt.Errorf("fastfeedback: root[%d] label is required", index)
		}
		if ordered[index].Path == "" {
			return Fingerprint{}, fmt.Errorf("fastfeedback: root %q path is required", ordered[index].Label)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Label < ordered[j].Label })
	for index := 1; index < len(ordered); index++ {
		if ordered[index-1].Label == ordered[index].Label {
			return Fingerprint{}, fmt.Errorf("fastfeedback: duplicate root label %q", ordered[index].Label)
		}
	}

	hash := sha256.New()
	result := Fingerprint{}
	writeToken := func(value string) {
		_, _ = io.WriteString(hash, value)
		_, _ = hash.Write([]byte{0})
	}
	writeToken(Algorithm)

	for _, root := range ordered {
		writeToken("root")
		writeToken(root.Label)
		info, err := os.Lstat(root.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && root.Optional {
				writeToken("missing")
				continue
			}
			return Fingerprint{}, fmt.Errorf("fastfeedback: inspect root %q: %w", root.Label, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := fingerprintSymlinkFile(hash, root.Label, root.Path, "", &result); err != nil {
				return Fingerprint{}, err
			}
			continue
		}
		if info.Mode().IsRegular() {
			if err := fingerprintFile(hash, root.Label, root.Path, "", &result, "file"); err != nil {
				return Fingerprint{}, err
			}
			continue
		}
		if !info.IsDir() {
			return Fingerprint{}, fmt.Errorf("fastfeedback: root %q has unsupported type %s", root.Label, info.Mode())
		}
		writeToken("directory")
		paths := make([]string, 0)
		err = filepath.WalkDir(root.Path, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == root.Path {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root.Path, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return Fingerprint{}, fmt.Errorf("fastfeedback: walk root %q: %w", root.Label, err)
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			writeToken("empty")
		}
		for _, relative := range paths {
			absolute := filepath.Join(root.Path, filepath.FromSlash(relative))
			entryInfo, err := os.Lstat(absolute)
			if err != nil {
				return Fingerprint{}, fmt.Errorf("fastfeedback: stat %q/%s: %w", root.Label, relative, err)
			}
			if entryInfo.Mode()&os.ModeSymlink != 0 {
				if err := fingerprintSymlinkFile(hash, root.Label, absolute, relative, &result); err != nil {
					return Fingerprint{}, err
				}
				continue
			}
			if !entryInfo.Mode().IsRegular() {
				return Fingerprint{}, fmt.Errorf("fastfeedback: root %q entry %q has unsupported type %s", root.Label, relative, entryInfo.Mode())
			}
			if err := fingerprintFile(hash, root.Label, absolute, relative, &result, "file"); err != nil {
				return Fingerprint{}, err
			}
		}
	}
	result.Digest = hex.EncodeToString(hash.Sum(nil))
	return result, nil
}

func NewMetadata(engine EngineIdentity, toolchain string, inputs, outputs Fingerprint) (Metadata, error) {
	metadata := Metadata{
		SchemaVersion: SchemaVersion,
		Algorithm:     Algorithm,
		Engine:        EngineIdentity{ID: strings.TrimSpace(engine.ID), Verified: engine.Verified},
		Toolchain:     strings.TrimSpace(toolchain),
		Inputs:        inputs,
		Outputs:       outputs,
	}
	if err := Validate(metadata); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func Validate(metadata Metadata) error {
	if metadata.SchemaVersion != SchemaVersion {
		return fmt.Errorf("fastfeedback: unsupported schemaVersion %d", metadata.SchemaVersion)
	}
	if metadata.Algorithm != Algorithm {
		return fmt.Errorf("fastfeedback: unsupported algorithm %q", metadata.Algorithm)
	}
	if strings.TrimSpace(metadata.Engine.ID) == "" {
		return errors.New("fastfeedback: engine identity is required")
	}
	if strings.TrimSpace(metadata.Toolchain) == "" {
		return errors.New("fastfeedback: toolchain identity is required")
	}
	if err := validateFingerprint("inputs", metadata.Inputs); err != nil {
		return err
	}
	if err := validateFingerprint("outputs", metadata.Outputs); err != nil {
		return err
	}
	return nil
}

func Reusable(cached, current Metadata) bool {
	if Validate(cached) != nil || Validate(current) != nil {
		return false
	}
	if !cached.Engine.Verified || !current.Engine.Verified {
		return false
	}
	return cached.SchemaVersion == current.SchemaVersion &&
		cached.Algorithm == current.Algorithm &&
		cached.Engine.ID == current.Engine.ID &&
		cached.Toolchain == current.Toolchain &&
		cached.Inputs == current.Inputs &&
		cached.Outputs == current.Outputs
}

func Load(path string) (Metadata, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	var metadata Metadata
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("fastfeedback: decode cache: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return Metadata{}, errors.New("fastfeedback: cache contains multiple JSON values")
		}
		return Metadata{}, fmt.Errorf("fastfeedback: decode cache: %w", err)
	}
	if err := Validate(metadata); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func Write(path string, metadata Metadata) error {
	if err := Validate(metadata); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".fast-feedback-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	writer := bufio.NewWriter(temporary)
	if _, err := writer.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := writer.Flush(); err != nil {
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

func fingerprintSymlinkFile(hash io.Writer, label, path, relative string, result *Fingerprint) error {
	targetInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("fastfeedback: resolve symlink %q/%s: %w", label, relative, err)
	}
	if targetInfo.IsDir() {
		return fmt.Errorf("fastfeedback: root %q entry %q is a directory symlink and cannot provide reusable evidence", label, relative)
	}
	if !targetInfo.Mode().IsRegular() {
		return fmt.Errorf("fastfeedback: root %q entry %q symlink target has unsupported type %s", label, relative, targetInfo.Mode())
	}
	return fingerprintFile(hash, label, path, relative, result, "symlink-file")
}

func fingerprintFile(hash io.Writer, label, path, relative string, result *Fingerprint, kind string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("fastfeedback: read %q/%s: %w", label, relative, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("fastfeedback: stat %q/%s: %w", label, relative, err)
	}
	_, _ = io.WriteString(hash, kind)
	_, _ = hash.Write([]byte{0})
	_, _ = io.WriteString(hash, label)
	_, _ = hash.Write([]byte{0})
	_, _ = io.WriteString(hash, filepath.ToSlash(relative))
	_, _ = hash.Write([]byte{0})
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("fastfeedback: hash %q/%s: %w", label, relative, err)
	}
	_, _ = hash.Write([]byte{0})
	result.Files++
	result.Bytes += info.Size()
	return nil
}

func validateFingerprint(name string, fingerprint Fingerprint) error {
	if len(fingerprint.Digest) != sha256.Size*2 {
		return fmt.Errorf("fastfeedback: %s digest must be a sha256 hex string", name)
	}
	if _, err := hex.DecodeString(fingerprint.Digest); err != nil {
		return fmt.Errorf("fastfeedback: %s digest is invalid: %w", name, err)
	}
	if fingerprint.Files < 0 || fingerprint.Bytes < 0 {
		return fmt.Errorf("fastfeedback: %s file/byte counts cannot be negative", name)
	}
	return nil
}
