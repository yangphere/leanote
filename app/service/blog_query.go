package service

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yangphere/leanote/app/domain"
)

const (
	DefaultBlogPageSize  = 10
	MaxBlogPage          = 10000
	MaxBlogPageSize      = 100
	MaxBlogKeywordsRunes = 128
	MaxBlogKeywordsBytes = 512
	MaxBlogTagRunes      = 64
	MaxBlogTagBytes      = 256
)

var (
	ErrInvalidBlogQuery = errors.New("invalid blog query")
	ErrInvalidBlogHost  = domain.ErrInvalidBlogHost
)

var blogSortFields = map[string]struct{}{
	"PublicTime":  {},
	"CreatedTime": {},
	"UpdatedTime": {},
	"Title":       {},
}

var jsonpCallbackPart = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

type BlogQueryInput struct {
	Page               string
	PagePresent        bool
	PageSize           string
	PageSizePresent    bool
	Sort               string
	SortPresent        bool
	Keywords           string
	Tag                string
	ConfiguredPageSize int
	ConfiguredSort     string
	IsAsc              bool
}

type BlogQuery struct {
	Page      int
	PageSize  int
	SortField string
	IsAsc     bool
	Keywords  string
	Tag       string
}

func ParseBlogQuery(input BlogQueryInput) (BlogQuery, error) {
	page := 1
	if input.PagePresent {
		parsed, err := parseBoundedBlogInt(input.Page, 1, MaxBlogPage)
		if err != nil {
			return BlogQuery{}, fmt.Errorf("%w: page: %v", ErrInvalidBlogQuery, err)
		}
		page = parsed
	}

	pageSize := input.ConfiguredPageSize
	if !input.PageSizePresent {
		if pageSize < 1 || pageSize > MaxBlogPageSize {
			pageSize = DefaultBlogPageSize
		}
	} else {
		parsed, err := parseBoundedBlogInt(input.PageSize, 1, MaxBlogPageSize)
		if err != nil {
			return BlogQuery{}, fmt.Errorf("%w: pageSize: %v", ErrInvalidBlogQuery, err)
		}
		pageSize = parsed
	}

	sortField := input.ConfiguredSort
	if input.SortPresent {
		if input.Sort == "" {
			return BlogQuery{}, fmt.Errorf("%w: sort is empty", ErrInvalidBlogQuery)
		}
		sortField = input.Sort
	}
	sortField = NormalizeBlogSortField(sortField)
	if input.SortPresent && sortField != input.Sort {
		return BlogQuery{}, fmt.Errorf("%w: unsupported sort", ErrInvalidBlogQuery)
	}

	keywords, err := normalizeBlogText(input.Keywords, MaxBlogKeywordsRunes, MaxBlogKeywordsBytes)
	if err != nil {
		return BlogQuery{}, fmt.Errorf("%w: keywords: %v", ErrInvalidBlogQuery, err)
	}
	tag, err := normalizeBlogText(input.Tag, MaxBlogTagRunes, MaxBlogTagBytes)
	if err != nil {
		return BlogQuery{}, fmt.Errorf("%w: tag: %v", ErrInvalidBlogQuery, err)
	}

	return BlogQuery{
		Page:      page,
		PageSize:  pageSize,
		SortField: sortField,
		IsAsc:     input.IsAsc,
		Keywords:  keywords,
		Tag:       tag,
	}, nil
}

func NormalizeBlogSortField(value string) string {
	if _, ok := blogSortFields[value]; !ok {
		return "PublicTime"
	}
	return value
}

func BlogSortFields(sortField string, isAsc bool) []string {
	sortField = NormalizeBlogSortField(sortField)
	direction := "-"
	if isAsc {
		direction = ""
	}
	return []string{direction + sortField, direction + "_id"}
}

func ValidateBlogSortField(value string) error {
	if _, ok := blogSortFields[value]; !ok {
		return fmt.Errorf("%w: unsupported sort %q", ErrInvalidBlogQuery, value)
	}
	return nil
}

func NormalizeBlogText(value string, maxRunes, maxBytes int) (string, error) {
	return normalizeBlogText(value, maxRunes, maxBytes)
}

func normalizeBlogText(value string, maxRunes, maxBytes int) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("invalid UTF-8")
	}
	value = strings.TrimFunc(value, unicode.IsSpace)
	if utf8.RuneCountInString(value) > maxRunes {
		return "", fmt.Errorf("more than %d code points", maxRunes)
	}
	if len(value) > maxBytes {
		return "", fmt.Errorf("more than %d bytes", maxBytes)
	}
	return value, nil
}

func parseBoundedBlogInt(value string, minimum, maximum int) (int, error) {
	if value == "" {
		return 0, errors.New("empty value")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errors.New("must contain only decimal digits")
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed > uint64(maximum) || parsed < uint64(minimum) {
		return 0, errors.New("out of range")
	}
	return int(parsed), nil
}

func ValidJSONPCallback(callback string) bool {
	if len(callback) == 0 || len(callback) > 128 || strings.Contains(callback, "..") {
		return false
	}
	parts := strings.Split(callback, ".")
	for _, part := range parts {
		if !jsonpCallbackPart.MatchString(part) {
			return false
		}
	}
	return true
}

func CanonicalizeBlogHost(raw string) (string, error) {
	return domain.CanonicalizeBlogHost(raw)
}

func CanonicalizeStoredCustomDomain(raw string) (string, error) {
	return domain.CanonicalizeStoredCustomDomain(raw)
}

func SelectBlogHost(requestHost, forwarded, forwardedHost, remoteAddr, trustedProxyAllowlist string) (string, error) {
	canonicalRequestHost, err := CanonicalizeBlogHost(requestHost)
	if err != nil {
		return "", err
	}
	if !IsTrustedProxy(remoteAddr, trustedProxyAllowlist) {
		return canonicalRequestHost, nil
	}
	forwardedValue, hasForwarded, err := parseForwardedHost(forwarded)
	if err != nil {
		return "", err
	}
	xForwardedValue, hasXForwarded, err := parseSingleForwardedHost(forwardedHost)
	if err != nil {
		return "", err
	}
	if hasForwarded && hasXForwarded && forwardedValue != xForwardedValue {
		return "", fmt.Errorf("%w: forwarded hosts conflict", ErrInvalidBlogHost)
	}
	if hasForwarded {
		return forwardedValue, nil
	}
	if hasXForwarded {
		return xForwardedValue, nil
	}
	return canonicalRequestHost, nil
}

func IsTrustedProxy(remoteAddr, allowlist string) bool {
	if strings.TrimSpace(allowlist) == "" {
		return false
	}
	remoteIP, err := parseRemoteAddr(remoteAddr)
	if err != nil {
		return false
	}
	for _, entry := range strings.Split(allowlist, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return false
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(remoteIP) {
				return true
			}
			continue
		}
		if address, err := netip.ParseAddr(entry); err == nil && address == remoteIP {
			return true
		}
	}
	return false
}

func parseRemoteAddr(value string) (netip.Addr, error) {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return netip.ParseAddr(host)
	}
	return netip.ParseAddr(value)
}

func parseForwardedHost(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}
	if strings.Contains(value, ",") {
		return "", false, fmt.Errorf("%w: multiple Forwarded entries", ErrInvalidBlogHost)
	}
	count := 0
	var host string
	for _, parameter := range strings.Split(value, ";") {
		key, parameterValue, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if !found || !strings.EqualFold(key, "host") {
			continue
		}
		count++
		if count > 1 {
			return "", false, fmt.Errorf("%w: duplicate Forwarded host", ErrInvalidBlogHost)
		}
		host = strings.TrimSpace(parameterValue)
		if len(host) >= 2 && strings.HasPrefix(host, `"`) && strings.HasSuffix(host, `"`) {
			host = host[1 : len(host)-1]
		}
	}
	if count == 0 {
		return "", false, fmt.Errorf("%w: Forwarded lacks host", ErrInvalidBlogHost)
	}
	canonical, err := CanonicalizeBlogHost(host)
	return canonical, true, err
}

func parseSingleForwardedHost(value string) (string, bool, error) {
	if value == "" {
		return "", false, nil
	}
	if strings.Contains(value, ",") {
		return "", false, fmt.Errorf("%w: multiple X-Forwarded-Host values", ErrInvalidBlogHost)
	}
	canonical, err := CanonicalizeBlogHost(strings.TrimSpace(value))
	return canonical, true, err
}
