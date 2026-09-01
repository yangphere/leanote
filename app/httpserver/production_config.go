package httpserver

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

const canonicalProductionConfig = "/etc/leanote/app.conf"

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
	for _, key := range sortedKeys(prod) {
		if forbiddenProductionKey(key) {
			return nil, configError("CONFIG_KEY_INVALID", key)
		}
	}
	for _, key := range sortedKeys(sections["DEFAULT"]) {
		if forbiddenProductionKey(key) {
			return nil, configError("CONFIG_KEY_INVALID", key)
		}
	}
	for _, value := range prod {
		if strings.Contains(value, "${MONGO_URL}") || strings.Contains(value, "${MONGODB_URI}") {
			return nil, configError("CONFIG_SOURCE_CONFLICT", "MONGODB_URL")
		}
	}
	// Sensitive production keys cannot be inherited from DEFAULT. Keeping a
	// second source there would make the effective config depend on override
	// order and could silently retain a literal or undeclared environment key.
	for _, key := range []string{"app.secret", "db.urlEnv"} {
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
	cfg, err := ParseConfig(data, "prod")
	if err != nil {
		return nil, configError("CONFIG_KEY_INVALID", "prod")
	}
	return cfg, nil
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
	for _, forbidden := range []string{"db.url", "db.host", "db.port", "db.username", "db.password"} {
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
