package content

import (
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const hardMaxRemoteCompressedBytes int64 = 32 * 1024 * 1024

type RemoteFetchProfile struct {
	DNSDialTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	OverallTimeout        time.Duration
	MaxRedirects          int
	MaxCompressedBytes    int64
}

func ResolveRemoteFetchProfile(uploadImageBytes int64) (RemoteFetchProfile, error) {
	if uploadImageBytes <= 0 {
		return RemoteFetchProfile{}, validationError("invalid_remote_body_limit", nil)
	}
	bodyLimit := uploadImageBytes
	if bodyLimit > hardMaxRemoteCompressedBytes {
		bodyLimit = hardMaxRemoteCompressedBytes
	}
	return RemoteFetchProfile{
		DNSDialTimeout:        5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		OverallTimeout:        30 * time.Second,
		MaxRedirects:          5,
		MaxCompressedBytes:    bodyLimit,
	}, nil
}

func ValidateRemoteURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, validationError("invalid_remote_url", nil)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, validationError("remote_scheme_not_allowed", nil)
	}
	return parsed, nil
}

var nonPublicRemotePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("ff00::/8"),
}

var nat64WellKnownPrefix = netip.MustParsePrefix("64:ff9b::/96")

func ValidatePublicRemoteAddress(address netip.Addr) error {
	if !address.IsValid() || address.Zone() != "" {
		return validationError("invalid_remote_address", nil)
	}
	address = address.Unmap()
	if nat64WellKnownPrefix.Contains(address) {
		bytes := address.As16()
		if err := ValidatePublicRemoteAddress(netip.AddrFrom4([4]byte{bytes[12], bytes[13], bytes[14], bytes[15]})); err != nil {
			return validationError("remote_address_not_public", nil)
		}
	}
	if !address.IsGlobalUnicast() {
		return validationError("remote_address_not_public", nil)
	}
	for _, prefix := range nonPublicRemotePrefixes {
		if prefix.Contains(address) {
			return validationError("remote_address_not_public", nil)
		}
	}
	return nil
}
