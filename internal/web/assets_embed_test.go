//go:build !dev

package web

import "testing"

// The fingerprint has to follow the content, or the token freezes and the cache problem
// comes back.
func TestAssetVersionFollowsContent(t *testing.T) {
	if fingerprintStatic() != assetVersion {
		t.Error("fingerprint deveria ser estável para o mesmo conteúdo")
	}
}
