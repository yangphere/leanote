package httpserver

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

const canonicalProductionConfig = "/etc/leanote/app.conf"

var productionContentRootKeys = []string{
	"content.private.data",
	"content.private.quarantine",
	"content.public.data",
	"content.public.quarantine",
	"content.temporary",
	"admin.backup.root",
}

// ConfigError is a stable, redacted production configuration failure.
type ConfigError struct {
	Code string
	Key  string
}

func (e *ConfigError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("configuration error code=%s run_mode=prod", e.Code)
	}
	return fmt.Sprintf("configuration error code=%s key=%s run_mode=prod", e.Code, e.Key)
}

func configError(code, key string) error { return &ConfigError{Code: code, Key: key} }

// ValidateProductionConfig validates the sole C-b v1 production interface and
// returns the expanded config only after all fail-closed checks pass.
func ValidateProductionConfig(path string) (*Config, error) {
	if path != canonicalProductionConfig {
		return nil, configError("CONFIG_PATH_INVALID", "conf")
	}
	// Lstat is intentional: the canonical path must itself be a regular file,
	// not a symlink whose target can be swapped outside the deployment root.
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, configError("CONFIG_FILE_MISSING", "conf")
		}
		return nil, configError("CONFIG_FILE_UNREADABLE", "conf")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0440 {
		return nil, configError("CONFIG_FILE_UNREADABLE", "conf")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, configError("CONFIG_FILE_UNREADABLE", "conf")
	}
	sections, err := parseProductionSections(data)
	if err != nil {
		return nil, err
	}
	prod, ok := sections["prod"]
	if !ok {
		return nil, configError("CONFIG_SECTION_MISSING", "prod")
	}
	if err := validateProductionKeys(sections); err != nil {
		return nil, err
	}
	for _, value := range prod {
		if strings.Contains(value, "${MONGO_URL}") || strings.Contains(value, "${MONGODB_URI}") {
			return nil, configError("CONFIG_SOURCE_CONFLICT", "MONGODB_URL")
		}
	}
	// Sensitive production keys cannot be inherited from DEFAULT. Keeping a
	// second source there would make the effective config depend on override
	// order and could silently retain a literal or undeclared environment key.
	for _, key := range []string{"app.secret", "db.urlEnv", "site.url"} {
		if _, inherited := sections["DEFAULT"][key]; inherited {
			if _, overridden := prod[key]; !overridden {
				continue // the required-key check below reports the missing prod key
			}
			return nil, configError("CONFIG_SOURCE_CONFLICT", key)
		}
	}
	for _, required := range []string{"app.secret", "db.dbname", "db.urlEnv"} {
		if _, ok := prod[required]; !ok {
			return nil, configError("CONFIG_KEY_INVALID", required)
		}
	}
	if err := validateProductionContentRootKeys(prod); err != nil {
		return nil, err
	}
	if prod["db.urlEnv"] != "${MONGODB_URL}" || prod["app.secret"] != "${LEANOTE_APP_SECRET}" {
		return nil, configError("CONFIG_SOURCE_CONFLICT", "db.urlEnv/app.secret")
	}
	dbName := strings.TrimSpace(stripQuotes(prod["db.dbname"]))
	if dbName == "" {
		return nil, configError("CONFIG_KEY_INVALID", "db.dbname")
	}
	mongoRaw, mongoPresent := os.LookupEnv("MONGODB_URL")
	if !mongoPresent {
		return nil, configError("CONFIG_VALUE_MISSING", "MONGODB_URL")
	}
	mongoURL := strings.TrimSpace(mongoRaw)
	if mongoURL == "" {
		return nil, configError("CONFIG_VALUE_EMPTY", "MONGODB_URL")
	}
	secretRaw, secretPresent := os.LookupEnv("LEANOTE_APP_SECRET")
	if !secretPresent {
		return nil, configError("CONFIG_VALUE_MISSING", "LEANOTE_APP_SECRET")
	}
	secret := strings.TrimSpace(secretRaw)
	if secret == "" {
		return nil, configError("CONFIG_VALUE_EMPTY", "LEANOTE_APP_SECRET")
	}
	if err := validateProductionSecret(secret); err != nil {
		return nil, err
	}
	if err := validateMongoURL(mongoURL, dbName); err != nil {
		return nil, err
	}
	if err := validateProductionSiteURL(prod); err != nil {
		return nil, err
	}
	if err := validateProductionSessionExpires(sections); err != nil {
		return nil, err
	}
	cfg, err := ParseConfig(data, "prod")
	if err != nil {
		return nil, configError("CONFIG_KEY_INVALID", "prod")
	}
	return cfg, nil
}

func validateProductionKeys(sections map[string]map[string]string) error {
	for _, section := range []string{"prod", "DEFAULT"} {
		for _, key := range sortedKeys(sections[section]) {
			if forbiddenProductionKey(key) {
				return configError("CONFIG_KEY_INVALID", key)
			}
		}
	}
	return nil
}

const defaultProductionSessionTTL = 168 * time.Hour

func validateProductionSessionExpires(sections map[string]map[string]string) error {
	_, err := parseProductionSessionExpires(sections)
	return err
}

func productionSessionTTL(cfg *Config) (time.Duration, error) {
	// Keep source validation and the typed handoff on the same parser, including
	// optional environment values and the error key identifying their source.
	return parseProductionSessionExpires(cfg.data)
}

func parseProductionSessionExpires(sections map[string]map[string]string) (time.Duration, error) {
	raw, present := sections["prod"]["session.expires"]
	if !present {
		raw, present = sections["DEFAULT"]["session.expires"]
	}
	if !present {
		return defaultProductionSessionTTL, nil
	}
	candidate := stripQuotes(raw)
	key := "session.expires"
	if candidate == "${LEANOTE_SESSION_EXPIRES}" {
		key = "LEANOTE_SESSION_EXPIRES"
		candidate = os.Getenv(key)
		if strings.TrimSpace(candidate) == "" {
			return defaultProductionSessionTTL, nil
		}
	} else if strings.Contains(candidate, "${") {
		return 0, configError("CONFIG_SOURCE_CONFLICT", key)
	}
	ttl, err := time.ParseDuration(strings.TrimSpace(candidate))
	if err != nil || ttl < 5*time.Minute || ttl > 8760*time.Hour {
		return 0, configError("CONFIG_SESSION_EXPIRES_INVALID", key)
	}
	return ttl, nil
}

func validateProductionSiteURL(prod map[string]string) error {
	rawSiteURL, siteURLPresent := prod["site.url"]
	if !siteURLPresent {
		return configError("CONFIG_KEY_INVALID", "site.url")
	}
	siteURL := stripQuotes(rawSiteURL)
	siteURLKey := "site.url"
	candidate := siteURL
	if siteURL == "${LEANOTE_SITE_URL}" {
		siteURLKey = "LEANOTE_SITE_URL"
		envSiteURL, present := os.LookupEnv("LEANOTE_SITE_URL")
		if !present {
			return configError("CONFIG_VALUE_MISSING", "LEANOTE_SITE_URL")
		}
		if strings.TrimSpace(envSiteURL) == "" {
			return configError("CONFIG_VALUE_EMPTY", "LEANOTE_SITE_URL")
		}
		candidate = envSiteURL
	} else if strings.Contains(siteURL, "${") {
		return configError("CONFIG_SOURCE_CONFLICT", "site.url")
	}
	if err := validateSiteURL(candidate); err != nil {
		return configError("CONFIG_SITE_URL_INVALID", siteURLKey)
	}
	return nil
}

// validateSiteURL accepts only the origin URL used to build absolute links and
// derive the default blog host. It deliberately rejects path/query/fragment
// components because those values cannot be represented by the existing URL
// consumers without changing their behavior.
func validateSiteURL(candidate string) error {
	if candidate == "" || candidate != strings.TrimSpace(candidate) {
		return fmt.Errorf("site url must not contain surrounding whitespace")
	}
	if strings.HasSuffix(candidate, "?") || strings.HasSuffix(candidate, "#") {
		return fmt.Errorf("site url must not end with query or fragment marker")
	}
	colon := strings.IndexByte(candidate, ':')
	if colon <= 0 {
		return fmt.Errorf("site url must include a scheme")
	}
	scheme := candidate[:colon]
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("site url scheme must be http or https")
	}
	u, err := url.Parse(candidate)
	if err != nil || u.Opaque != "" {
		return fmt.Errorf("site url is not an absolute URL")
	}
	if u.Scheme != scheme || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("site url scheme must be lowercase http or https")
	}
	if u.User != nil || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return fmt.Errorf("site url must contain only a host")
	}
	if u.Host == "" || u.Hostname() == "" {
		return fmt.Errorf("site url host is required")
	}
	if port := u.Port(); port != "" {
		parsed, parseErr := strconv.Atoi(port)
		if parseErr != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("site url port is invalid")
		}
	}
	if _, err := domain.CanonicalizeBlogHost(u.Host); err != nil {
		return fmt.Errorf("site url host is invalid")
	}
	return nil
}

func validateProductionContentRootKeys(prod map[string]string) error {
	for _, key := range productionContentRootKeys {
		if value, ok := prod[key]; !ok || strings.TrimSpace(stripQuotes(value)) == "" {
			return configError("CONFIG_CONTENT_ROOT_MISSING", key)
		}
	}
	return nil
}

func parseProductionSections(data []byte) (map[string]map[string]string, error) {
	sections := map[string]map[string]string{"DEFAULT": {}}
	section := "DEFAULT"
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
			return nil, configError("CONFIG_KEY_INVALID", fmt.Sprintf("line_%d", lineNumber+1))
		}
		line := strings.TrimSpace(stripInlineComment(raw))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section == "" {
				return nil, configError("CONFIG_SECTION_MISSING", "prod")
			}
			if _, exists := sections[section]; exists {
				return nil, configError("CONFIG_SOURCE_CONFLICT", section)
			}
			sections[section] = map[string]string{}
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep <= 0 {
			return nil, configError("CONFIG_KEY_INVALID", fmt.Sprintf("line_%d", lineNumber+1))
		}
		key := strings.TrimSpace(line[:sep])
		value := strings.TrimSpace(line[sep+1:])
		if _, exists := sections[section][key]; exists {
			return nil, configError("CONFIG_SOURCE_CONFLICT", key)
		}
		sections[section][key] = value
	}
	return sections, nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func forbiddenProductionKey(key string) bool {
	for _, forbidden := range []string{"db.url", "db.host", "db.port", "db.username", "db.password", "cookie.secure"} {
		if key == forbidden {
			return true
		}
	}
	return false
}

func validateProductionSecret(secret string) error {
	if secret == "V85ZzBeTnzpsHyjQX4zukbQ8qqtju9y2aDM55VWxAH9Qop19poekx3xkcDVvrD0y" {
		return configError("CONFIG_PUBLIC_DEFAULT", "LEANOTE_APP_SECRET")
	}
	if len(secret) < 32 {
		return configError("CONFIG_SECRET_INVALID", "LEANOTE_APP_SECRET")
	}
	for _, r := range secret {
		if r > unicode.MaxASCII || unicode.IsControl(r) {
			return configError("CONFIG_SECRET_INVALID", "LEANOTE_APP_SECRET")
		}
	}
	return nil
}

func validateMongoURL(raw, dbName string) error {
	cs, err := connstring.ParseAndValidate(raw)
	if err != nil || (cs.Scheme != "mongodb" && cs.Scheme != "mongodb+srv") || len(cs.Hosts) == 0 {
		return configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
	}
	for _, rawHost := range cs.Hosts {
		host := rawHost
		if parsedHost, _, splitErr := net.SplitHostPort(rawHost); splitErr == nil {
			host = parsedHost
		} else {
			host = strings.Trim(host, "[]")
		}
		host = strings.TrimSuffix(strings.ToLower(host), ".")
		if host == "localhost" {
			return configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
		}
	}
	if cs.Database == "" || cs.Database != dbName || cs.Database == "leanote_test" {
		return configError("CONFIG_MONGO_INVALID", "db.dbname")
	}
	return nil
}

// CanonicalProductionConfigPath returns the only accepted production path.
func CanonicalProductionConfigPath() string { return filepath.Clean(canonicalProductionConfig) }
