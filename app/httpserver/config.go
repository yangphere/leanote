// Package httpserver hosts the first-party HTTP stack that replaces Revel:
// configuration, routing registry, sessions, request binding, responses and
// middleware. Services must not import this package.
package httpserver

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const defaultSection = "DEFAULT"

var (
	varRegExp    = regexp.MustCompile(`%\(([a-zA-Z0-9_.\-]+)\)s`)
	envVarRegExp = regexp.MustCompile(`\$\{([a-zA-Z0-9_.\-]+)}`)
)

// boolString reproduces revel/config's accepted boolean vocabulary
// (config.go boolString map), matched case-insensitively.
var boolString = map[string]bool{
	"t": true, "true": true, "y": true, "yes": true, "on": true, "1": true,
	"f": false, "false": false, "n": false, "no": false, "off": false, "0": false,
}

// Config reproduces the revel/config semantics the app relies on: a default
// section plus run-mode sections, ${VAR} expansion from the environment and
// %(key)s interpolation from other keys, both applied lazily at read time.
type Config struct {
	data    map[string]map[string]string
	section string
}

// maxExpansionDepth mirrors revel/config's DepthValues recursion bound.
const maxExpansionDepth = 200

// ParseConfig parses app.conf content and activates the run-mode section.
// Lookups consult the active section first, then the default section. An
// empty runMode activates only the default section; a named section that is
// absent is a load error (Revel fails the same way).
func ParseConfig(data []byte, runMode string) (*Config, error) {
	c := &Config{data: map[string]map[string]string{defaultSection: {}}}
	section := defaultSection
	option := ""
	for lineNumber, rawLine := range strings.Split(string(data), "\n") {
		rawLine = strings.TrimSuffix(rawLine, "\r")
		indented := len(rawLine) > 0 && (rawLine[0] == ' ' || rawLine[0] == '\t')
		line := strings.TrimSpace(stripInlineComment(rawLine))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if indented {
			if option == "" {
				return nil, fmt.Errorf("conf line %d: continuation without a preceding option", lineNumber+1)
			}
			c.data[section][option] += "\n" + line
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" {
				return nil, fmt.Errorf("conf line %d: empty section name", lineNumber+1)
			}
			section = name
			if _, ok := c.data[section]; !ok {
				c.data[section] = map[string]string{}
			}
			option = ""
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep <= 0 {
			return nil, fmt.Errorf("conf line %d: could not parse line %q", lineNumber+1, line)
		}
		key := strings.TrimSpace(line[:sep])
		value := strings.TrimSpace(stripInlineComment(line[sep+1:]))
		if key == "" {
			continue
		}
		c.data[section][key] = value
		option = key
	}
	if runMode != "" {
		if _, ok := c.data[runMode]; !ok {
			return nil, fmt.Errorf("app.conf: run mode not found: %s", runMode)
		}
		c.section = runMode
	}
	return c, nil
}

// LoadConfigFile reads and parses the configuration file at path.
func LoadConfigFile(path, runMode string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(data, runMode)
}

// Section returns the active run-mode section name.
func (c *Config) Section() string {
	return c.section
}

func (c *Config) lookup(key string) (string, bool) {
	var raw string
	var found bool
	if v, ok := c.data[c.section][key]; ok {
		raw, found = v, true
	} else if v, ok := c.data[defaultSection][key]; ok {
		raw, found = v, true
	}
	if !found {
		return "", false
	}
	value, err := c.expand(strings.TrimSpace(raw), c.section)
	if err != nil {
		// Undefined %(...)s references read as absent, exactly like
		// revel/config's OptionError collapse in Context.String.
		return "", false
	}
	return value, true
}

// expand resolves %(key)s against the config (section overrides default)
// recursively up to maxExpansionDepth, then ${VAR} from the environment.
// Matching revel/config's computeVar: an empty resolution — an undefined
// %(key)s, or an unset/empty ${VAR} — is an error ("Option not found"), and
// exhausting the depth is a cycle error. Both surface as found=false via
// lookup.
func (c *Config) expand(value, section string) (string, error) {
	for depth := 0; depth < maxExpansionDepth; depth++ {
		matches := varRegExp.FindAllStringSubmatchIndex(value, -1)
		if len(matches) == 0 {
			return envExpand(value)
		}
		var b strings.Builder
		cursor := 0
		undefined := ""
		for _, m := range matches {
			name := value[m[2]:m[3]]
			replacement, ok := c.data[section][name]
			if !ok {
				replacement, ok = c.data[defaultSection][name]
			}
			b.WriteString(value[cursor:m[0]])
			if !ok || replacement == "" {
				if undefined == "" {
					undefined = name
				}
				b.WriteString(value[m[0]:m[1]])
			} else {
				b.WriteString(replacement)
			}
			cursor = m[1]
		}
		b.WriteString(value[cursor:])
		if undefined != "" {
			return "", fmt.Errorf("option not found: %s", undefined)
		}
		value = b.String()
	}
	return "", fmt.Errorf("possible cycle while unfolding variables: max depth of %d reached", maxExpansionDepth)
}

// envExpand substitutes ${VAR} from the environment; an unset or empty
// variable is an error, matching revel/config's empty-resolution rule.
func envExpand(value string) (string, error) {
	for _, m := range envVarRegExp.FindAllStringSubmatchIndex(value, -1) {
		name := value[m[2]:m[3]]
		if strings.TrimSpace(os.Getenv(name)) == "" {
			return "", fmt.Errorf("option not found: %s", name)
		}
	}
	return envVarRegExp.ReplaceAllStringFunc(value, func(match string) string {
		return strings.TrimSpace(os.Getenv(envVarRegExp.FindStringSubmatch(match)[1]))
	}), nil
}

// String returns the expanded value and whether the key was found.
func (c *Config) String(key string) (string, bool) {
	raw, ok := c.lookup(key)
	if !ok {
		return "", false
	}
	return stripQuotes(raw), true
}

// StringDefault returns the value or def when the key is missing.
func (c *Config) StringDefault(key, def string) string {
	if v, ok := c.String(key); ok {
		return v
	}
	return def
}

// Int returns the integer value and whether the key was found. Parse
// failures report found=false (revel/config OptionError semantics).
func (c *Config) Int(key string) (int, bool) {
	raw, ok := c.lookup(key)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return n, true
}

// IntDefault returns the integer value or def when missing/unparseable.
func (c *Config) IntDefault(key string, def int) int {
	if v, ok := c.Int(key); ok {
		return v
	}
	return def
}

// Bool returns the boolean value and whether the key was found. The
// accepted vocabulary matches revel/config: t/true/y/yes/on/1 and
// f/false/n/no/off/0, case-insensitively.
func (c *Config) Bool(key string) (bool, bool) {
	raw, ok := c.lookup(key)
	if !ok {
		return false, false
	}
	v, present := boolString[strings.ToLower(strings.TrimSpace(raw))]
	if !present {
		return false, false
	}
	return v, true
}

// BoolDefault returns the boolean value or def when missing/unparseable.
func (c *Config) BoolDefault(key string, def bool) bool {
	if v, ok := c.Bool(key); ok {
		return v
	}
	return def
}

// stripInlineComment removes an inline "#" or " ;" comment that is either
// at the start of the value or preceded by whitespace, honouring quotes and
// backslash escapes. This mirrors how revel/config reads app.conf values.
func stripInlineComment(value string) string {
	var quote rune
	escaped := false
	previousWhitespace := false
	for index, character := range value {
		if escaped {
			escaped = false
			previousWhitespace = false
			continue
		}
		if character == '\\' {
			escaped = true
			previousWhitespace = false
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			previousWhitespace = false
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			previousWhitespace = false
		case ' ', '\t':
			previousWhitespace = true
		case '#':
			if index == 0 || previousWhitespace {
				return value[:index]
			}
			previousWhitespace = false
		case ';':
			if previousWhitespace {
				return value[:index]
			}
			previousWhitespace = false
		default:
			previousWhitespace = false
		}
	}
	return value
}

// stripQuotes removes one layer of matching surrounding double quotes from
// a config value, as revel/config does for String lookups.
func stripQuotes(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
