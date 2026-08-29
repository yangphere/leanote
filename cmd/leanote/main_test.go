package main

import (
	"strings"
	"testing"
)

const publicSecret = defaultPublicSecret

func TestValidateProdSecretAcceptsRealSecret(t *testing.T) {
	if err := validateProdSecret("prod", "a-real-production-secret"); err != nil {
		t.Fatalf("validateProdSecret(prod, real) = %v, want nil", err)
	}
}

func TestValidateProdSecretRejectsEmpty(t *testing.T) {
	err := validateProdSecret("prod", "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("validateProdSecret(prod, empty) = %v, want empty-secret error", err)
	}
}

func TestValidateProdSecretRejectsPublicDefault(t *testing.T) {
	err := validateProdSecret("prod", publicSecret)
	if err == nil || !strings.Contains(err.Error(), "public repository default") {
		t.Fatalf("validateProdSecret(prod, default) = %v, want public-default error", err)
	}
}

func TestValidateProdSecretSkipsNonProd(t *testing.T) {
	for _, mode := range []string{"dev", "test"} {
		if err := validateProdSecret(mode, ""); err != nil {
			t.Fatalf("validateProdSecret(%s, empty) = %v, want nil (non-prod skips check)", mode, err)
		}
	}
}
