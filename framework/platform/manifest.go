package platform

import "github.com/hvritual/yunka.io/pkg/providerplan"

// NewFromManifest is the canonical runtime entrypoint for declarative
// infrastructure providers. The manifest selects typed factories; host-owned
// capabilities remain explicit in base Options and are never discovered from a
// global container.
func NewFromManifest(path string, base Options) (*Provider, error) {
	manifest, err := providerplan.Load(path)
	if err != nil {
		return nil, err
	}
	bound, err := BindManifest(manifest, base)
	if err != nil {
		return nil, err
	}
	return New(bound)
}
