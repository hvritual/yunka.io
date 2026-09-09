package implementation

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAG061CanonicalDottedKeyIsAccepted(t *testing.T) {
	root, options := compilerProject(t)
	proto := filepath.Join(root, "contracts/proto/shelf.proto")
	original, err := os.ReadFile(proto)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(original), `name:"catalog"`, `name:"catalog.v2"`, 1)
	if updated == string(original) {
		t.Fatal("fixture anchor drift")
	}
	if err = os.WriteFile(proto, []byte(updated), 0640); err != nil {
		t.Fatal(err)
	}
	options.Application = "shelf/catalog.v2"
	before := snapshot(t, root)
	plan, err := Run(context.Background(), options, false)
	if err != nil {
		t.Fatalf("canonical dotted Application was rejected: %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("dotted-key planning changed source")
	}
	for _, f := range plan.Files {
		if !strings.HasPrefix(f.Path, "internal/shelf/application/catalog.v2/") {
			t.Fatalf("key was silently remapped: %s", f.Path)
		}
	}
	applied, err := Run(context.Background(), options, true)
	if err != nil || applied.Application != options.Application {
		t.Fatalf("canonical dotted-key apply: %v %+v", err, applied)
	}
	for _, key := range []string{"shelf/a:b", "shelf/..", "shelf/CON", "shelf/a.", "shelf/vendor"} {
		if _, _, err := applicationParts(key); err == nil {
			t.Fatalf("Go path constraint not enforced for %q", key)
		}
	}
}

func TestAG061PackageConflictsPrecedeEveryWrite(t *testing.T) {
	for _, tc := range []struct {
		name, path, content string
		allowed             bool
	}{
		{"owner_other", "existing.go", "package different\n", false},
		{"hidden_other", "internal/usecase/existing.go", "package different\n", false},
		{"malformed", "existing.go", "not a package clause\n", false},
		{"incompatible_tagged", "existing.go", "//go:build alternate\n\npackage different\n", false},
		{"owner_same", "existing.go", "package owner\n", true},
		{"hidden_same", "internal/usecase/existing.go", "package usecase\n", true},
		{"external_test", "existing_test.go", "package owner_test\n", true},
		{"external_non_test", "existing.go", "package owner_test\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, options := compilerProject(t)
			put(t, root, "internal/shelf/application/catalog/"+tc.path, tc.content)
			before := snapshot(t, root)
			_, err := Run(context.Background(), options, true)
			if tc.allowed {
				if err != nil {
					t.Fatal(err)
				}
				for name, hash := range before {
					if snapshot(t, root)[name] != hash {
						t.Fatalf("existing file changed: %s", name)
					}
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "existing Go package") {
				t.Fatalf("wrong package-conflict rejection: %v", err)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("discoverable package conflict left partially created starter")
			}
		})
	}
}
