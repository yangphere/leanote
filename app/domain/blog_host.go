package domain

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

var ErrInvalidBlogHost = errors.New("invalid blog host")

func CanonicalizeBlogHost(raw string) (string, error) {
	host, err := stripBlogHostPort(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidBlogHost, err)
	}
	if host == "" || strings.ContainsAny(host, "/\\?#@") {
		return "", fmt.Errorf("%w: malformed host", ErrInvalidBlogHost)
	}
	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
	if host == "" || strings.HasSuffix(host, ".") {
		return "", fmt.Errorf("%w: empty host label", ErrInvalidBlogHost)
	}
	for _, character := range host {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return "", fmt.Errorf("%w: whitespace or control character", ErrInvalidBlogHost)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		return strings.ToLower(ip.String()), nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", fmt.Errorf("%w: invalid IDN", ErrInvalidBlogHost)
	}
	ascii = strings.ToLower(strings.TrimSuffix(ascii, "."))
	labels := strings.Split(ascii, ".")
	if len(labels) == 0 || len(ascii) > 253 {
		return "", fmt.Errorf("%w: invalid label count", ErrInvalidBlogHost)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("%w: invalid label", ErrInvalidBlogHost)
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
				return "", fmt.Errorf("%w: invalid host character", ErrInvalidBlogHost)
			}
		}
	}
	return ascii, nil
}

func CanonicalizeStoredCustomDomain(raw string) (string, error) {
	host, err := CanonicalizeBlogHost(raw)
	if err != nil {
		return "", err
	}
	if net.ParseIP(host) != nil {
		return "", fmt.Errorf("%w: IP addresses are not custom domains", ErrInvalidBlogHost)
	}
	return host, nil
}

func stripBlogHostPort(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("empty host")
	}
	if strings.HasPrefix(raw, "[") {
		if host, port, err := net.SplitHostPort(raw); err == nil {
			if err := validateBlogHostPort(port); err != nil {
				return "", err
			}
			return host, nil
		}
		if strings.HasSuffix(raw, "]") {
			return strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"), nil
		}
		return "", errors.New("invalid bracketed host")
	}
	if strings.Count(raw, ":") == 1 {
		host, port, _ := strings.Cut(raw, ":")
		if err := validateBlogHostPort(port); err != nil {
			return "", err
		}
		return host, nil
	}
	if strings.Contains(raw, ":") {
		return "", errors.New("IPv6 host must be bracketed")
	}
	return raw, nil
}

func validateBlogHostPort(port string) error {
	if port == "" {
		return errors.New("empty port")
	}
	parsed, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsed == 0 {
		return errors.New("invalid port")
	}
	return nil
}
