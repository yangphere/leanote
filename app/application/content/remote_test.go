package content

import (
	"errors"
	"net/netip"
	"testing"
	"time"
)

func TestResolveRemoteFetchProfileUsesConfirmedSafetyBudget(t *testing.T) {
	profile, err := ResolveRemoteFetchProfile(48 * 1024 * 1024)
	if err != nil {
		t.Fatalf("ResolveRemoteFetchProfile() error = %v", err)
	}
	if profile.DNSDialTimeout != 5*time.Second || profile.TLSHandshakeTimeout != 5*time.Second ||
		profile.ResponseHeaderTimeout != 10*time.Second || profile.OverallTimeout != 30*time.Second ||
		profile.MaxRedirects != 5 || profile.MaxCompressedBytes != 32*1024*1024 {
		t.Fatalf("ResolveRemoteFetchProfile() = %#v", profile)
	}

	profile, err = ResolveRemoteFetchProfile(3 * 1024 * 1024)
	if err != nil || profile.MaxCompressedBytes != 3*1024*1024 {
		t.Fatalf("smaller upload limit profile = %#v, %v", profile, err)
	}
}

func TestResolveRemoteFetchProfileFailsClosedForInvalidUploadLimit(t *testing.T) {
	for _, limit := range []int64{-1, 0} {
		_, err := ResolveRemoteFetchProfile(limit)
		var contentErr *Error
		if !errors.As(err, &contentErr) || contentErr.Category != ErrorValidation || contentErr.Code != "invalid_remote_body_limit" {
			t.Fatalf("ResolveRemoteFetchProfile(%d) error = %#v", limit, err)
		}
	}
}

func TestValidateRemoteURLAllowsOnlyCredentialFreeHTTPAndHTTPS(t *testing.T) {
	for _, value := range []string{"https://example.com/image.png", "http://example.com/a"} {
		if _, err := ValidateRemoteURL(value); err != nil {
			t.Fatalf("ValidateRemoteURL(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{"", "ftp://example.com/a", "//example.com/a", "https://user:pass@example.com/a", "https:///a", "https://example.com/a#fragment"} {
		if _, err := ValidateRemoteURL(value); err == nil {
			t.Fatalf("ValidateRemoteURL(%q) succeeded", value)
		}
	}
}

func TestValidatePublicRemoteAddressRejectsSpecialNetworks(t *testing.T) {
	rejected := []string{
		"0.0.0.0", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.169.254",
		"192.0.2.1", "192.88.99.1", "224.0.0.1", "240.0.0.1", "::", "::1", "::127.0.0.1",
		"64:ff9b::7f00:1", "64:ff9b:1::1", "100::1", "2001::1", "2001:10::1", "2001:20::1", "2002:7f00:1::1",
		"fc00::1", "fec0::1", "fe80::1", "2001:db8::1", "2001:4860:4860::8888%public",
	}
	for _, value := range rejected {
		if err := ValidatePublicRemoteAddress(netip.MustParseAddr(value)); err == nil {
			t.Fatalf("ValidatePublicRemoteAddress(%s) succeeded", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "1.1.1.1", "64:ff9b::808:808", "2001:4860:4860::8888"} {
		if err := ValidatePublicRemoteAddress(netip.MustParseAddr(value)); err != nil {
			t.Fatalf("ValidatePublicRemoteAddress(%s) error = %v", value, err)
		}
	}
}
