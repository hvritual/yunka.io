package boundarycore

import "github.com/hvritual/yunka.io/pkg/contract"

// ManifestDigest names the normalized canonical model used in decisions. Input
// compilation/validation is the caller's responsibility; this is not a signature.
func ManifestDigest(manifest contract.Manifest) string {
	return modelDigest("boundary-manifest/v1", detachedManifest(manifest))
}
