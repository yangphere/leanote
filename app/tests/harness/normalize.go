package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

var objectIDFields = []string{
	"AlbumId", "AttachId", "BlogId", "FileId", "NotebookId", "NoteId",
	"ParentNoteId", "ShareId", "TagId", "ThemeId", "UserId",
}

var requiredObjectIDFields = map[string]bool{
	"NoteId": true,
	"UserId": true,
}

var timeFields = []string{
	"CreatedTime", "UpdatedTime", "PublicTime", "DeleteTime",
}

var objectIDValuePattern = regexp.MustCompile(`^[0-9a-f]{24}$`)
var objectIDPattern = fieldValuePattern(objectIDFields, `[0-9a-f]{24}`)
var timestampValuePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-][0-9]{2}:[0-9]{2})$`)
var timePattern = fieldValuePattern(timeFields, `[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-][0-9]{2}:[0-9]{2})`)
var tokenPattern = regexp.MustCompile(`("Token"\s*:\s*)"[^"\\]+"`)
var generatedLogoPattern = regexp.MustCompile(`("Logo"\s*:\s*)"[^"\\]*/images/logo/[^"\\]+"`)
var unixTimePattern = regexp.MustCompile(`("LastSyncTime"\s*:\s*)[0-9]+`)
var extendedObjectIDPattern = fieldObjectPattern(objectIDFields)
var objectIDStringPattern = fieldStringPattern(objectIDFields)
var timeStringPattern = fieldStringPattern(timeFields)

var jsonComparableHeaders = map[string]struct{}{
	"Content-Type": {},
	"Location":     {},
}

var jsonIgnoredHeaders = map[string]struct{}{
	"Content-Length": {},
	"Date":           {},
	"Set-Cookie":     {},
}

var binaryComparableHeaders = map[string]struct{}{
	"Accept-Ranges":       {},
	"Content-Disposition": {},
	"Content-Type":        {},
}

var binaryIgnoredHeaders = map[string]struct{}{
	"Content-Length": {},
	"Date":           {},
	"Last-Modified":  {},
	"Set-Cookie":     {},
}

func fieldValuePattern(fields []string, valuePattern string) *regexp.Regexp {
	return regexp.MustCompile(`("(?:` + strings.Join(fields, "|") + `)"\s*:\s*)"(` + valuePattern + `)"`)
}

func fieldObjectPattern(fields []string) *regexp.Regexp {
	return regexp.MustCompile(`"(?:` + strings.Join(fields, "|") + `)"\s*:\s*\{`)
}

func fieldStringPattern(fields []string) *regexp.Regexp {
	return regexp.MustCompile(`"(` + strings.Join(fields, "|") + `)"\s*:\s*"([^"\\]*)"`)
}

// NormalizeBody replaces only documented dynamic fields in raw JSON. It never
// unmarshal-marshals the payload, so response key order stays observable.
func NormalizeBody(body []byte) ([]byte, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("response body is not valid JSON")
	}
	if match := extendedObjectIDPattern.Find(body); match != nil {
		return nil, fmt.Errorf("ObjectID contract field has extended JSON form: %s", match)
	}

	for _, match := range objectIDStringPattern.FindAllSubmatch(body, -1) {
		field := string(match[1])
		value := match[2]
		if len(value) == 0 && !requiredObjectIDFields[field] {
			continue
		}
		if !objectIDValuePattern.Match(value) {
			return nil, fmt.Errorf("ObjectID contract field %s has invalid value %q", match[1], match[2])
		}
	}
	for _, match := range timeStringPattern.FindAllSubmatch(body, -1) {
		if !timestampValuePattern.Match(match[2]) {
			return nil, fmt.Errorf("timestamp contract field %s has invalid value %q", match[1], match[2])
		}
	}

	normalized := objectIDPattern.ReplaceAll(body, []byte(`${1}"OID_TOKEN"`))
	normalized = timePattern.ReplaceAll(normalized, []byte(`${1}"TIME_TOKEN"`))
	normalized = tokenPattern.ReplaceAll(normalized, []byte(`${1}"AUTH_TOKEN"`))
	normalized = generatedLogoPattern.ReplaceAll(normalized, []byte(`${1}"LOGO_TOKEN"`))
	normalized = unixTimePattern.ReplaceAll(normalized, []byte(`${1}"UNIX_TIME_TOKEN"`))
	return normalized, nil
}

// NormalizeHeaders returns the closed comparison set. Every response header
// must be explicitly comparable or explicitly ignored.
func NormalizeHeaders(headers http.Header, binary bool) (map[string]string, error) {
	comparable := jsonComparableHeaders
	ignored := jsonIgnoredHeaders
	if binary {
		comparable = binaryComparableHeaders
		ignored = binaryIgnoredHeaders
	}

	normalized := make(map[string]string, len(comparable))
	keys := make([]string, 0, len(headers))
	for name := range headers {
		keys = append(keys, http.CanonicalHeaderKey(name))
	}
	sort.Strings(keys)
	for _, name := range keys {
		values := headers.Values(name)
		if _, ok := comparable[name]; ok {
			if len(values) != 1 {
				return nil, fmt.Errorf("header %s has %d values, expected exactly one", name, len(values))
			}
			normalized[name] = values[0]
			continue
		}
		if _, ok := ignored[name]; ok {
			continue
		}
		return nil, fmt.Errorf("response contains unknown header %s", name)
	}
	return normalized, nil
}
