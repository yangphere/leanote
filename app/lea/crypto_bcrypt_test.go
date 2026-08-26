package lea

import (
	"encoding/base64"
	"testing"
)

// Pinned bcrypt digest generated with x/crypto 2020-04-29 pseudo-version
// (cost 10) on 2026-08-26. The upgrade to v0.55.0 must keep this hash valid.
const pinnedBcryptHash = "JDJhJDEwJDEwL3M4ekk5cThwTy9GcUNUREE4MU9IZnVtSjNObndsajkzOUpkeUc4elh3ODVIUXFpVFZP"

func TestCompareHashPinnedDigest(t *testing.T) {
	digest, err := base64.StdEncoding.DecodeString(pinnedBcryptHash)
	if err != nil {
		t.Fatalf("decode pinned hash: %v", err)
	}
	if !CompareHash(digest, "V7nR2xW9q") {
		t.Error("pinned hash must verify against its plaintext")
	}
	if CompareHash(digest, "wrong-password") {
		t.Error("pinned hash must reject a wrong password")
	}
}

func TestGenerateHashRoundTrip(t *testing.T) {
	const password = "another-secret-123"
	hash, err := GenerateHash(password)
	if err != nil {
		t.Fatalf("GenerateHash: %v", err)
	}
	if len(hash) < 59 || string(hash[:4]) != "$2a$" {
		t.Fatalf("unexpected hash format: %q", hash[:8])
	}
	if !CompareHash(hash, password) {
		t.Error("generated hash must verify against its plaintext")
	}
	if CompareHash(hash, password+"x") {
		t.Error("generated hash must reject a wrong password")
	}
}
