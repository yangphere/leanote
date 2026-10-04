package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type GoldenMode string

const (
	Record GoldenMode = "record"
	Replay GoldenMode = "replay"
)

type Snapshot struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	normalized bool
}

type RequestSpec struct {
	Method string              `json:"method"`
	Path   string              `json:"path"`
	Query  map[string][]string `json:"query,omitempty"`
	Form   map[string][]string `json:"form,omitempty"`
	Files  map[string]FilePart `json:"files,omitempty"`
	Auth   string              `json:"auth,omitempty"`
	Binary bool                `json:"binary,omitempty"`
}

// FilePart is the deterministic file payload used by an HTTP golden case.
// Encoding/json stores Body as base64, so request fixtures remain portable.
type FilePart struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Body        []byte `json:"body"`
}

type GoldenStore struct {
	Mode GoldenMode
	Root string
}

type serializedSnapshot struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type goldenCase struct {
	Request  RequestSpec        `json:"request"`
	Response serializedSnapshot `json:"response"`
}

func GoldenModeFromEnvironment() (GoldenMode, error) {
	mode := GoldenMode(os.Getenv("LEANOTE_GOLDEN"))
	if mode == "" {
		return Replay, nil
	}
	if mode != Record && mode != Replay {
		return "", fmt.Errorf("LEANOTE_GOLDEN must be record or replay, got %q", mode)
	}
	return mode, nil
}

func (s GoldenStore) Assert(name string, actual Snapshot) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	actual, err = normalizeSnapshot(actual)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(serializedSnapshotFor(actual))
	if err != nil {
		return fmt.Errorf("encode actual golden: %w", err)
	}
	encoded = append(encoded, '\n')

	switch s.Mode {
	case Record:
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create golden directory: %w", err)
		}
		return ioutil.WriteFile(path, encoded, 0o644)
	case Replay:
		expected, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read golden %s: %w", name, err)
		}
		if !bytes.Equal(expected, encoded) {
			return fmt.Errorf("golden mismatch for %s\nwant: %s\n got: %s", name, expected, encoded)
		}
		return nil
	default:
		return fmt.Errorf("unsupported golden mode %q", s.Mode)
	}
}

func (s GoldenStore) AssertCase(name string, request RequestSpec, actual Snapshot) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if request.Binary && isJSON(actual.Headers["Content-Type"]) {
		return fmt.Errorf("binary golden %s received a JSON response", name)
	}
	actual, err = normalizeSnapshot(actual)
	if err != nil {
		return err
	}
	request = normalizeRequestSpec(request)
	if request.Binary {
		if strings.TrimSpace(actual.Headers["Content-Type"]) == "" {
			return fmt.Errorf("binary golden %s has no Content-Type", name)
		}
		if len(actual.Body) == 0 {
			return fmt.Errorf("binary golden %s has an empty body", name)
		}
		actual.Body = []byte("BINARY_BODY")
	}
	encoded, err := json.Marshal(goldenCase{
		Request:  request,
		Response: serializedSnapshotFor(actual),
	})
	if err != nil {
		return fmt.Errorf("encode actual golden case: %w", err)
	}
	encoded = append(encoded, '\n')

	switch s.Mode {
	case Record:
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create golden directory: %w", err)
		}
		return ioutil.WriteFile(path, encoded, 0o644)
	case Replay:
		expected, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read golden %s: %w", name, err)
		}
		if !bytes.Equal(expected, encoded) {
			return fmt.Errorf("golden mismatch for %s\nwant: %s\n got: %s", name, expected, encoded)
		}
		return nil
	default:
		return fmt.Errorf("unsupported golden mode %q", s.Mode)
	}
}

func serializedSnapshotFor(snapshot Snapshot) serializedSnapshot {
	return serializedSnapshot{
		Status:  snapshot.Status,
		Headers: snapshot.Headers,
		Body:    string(snapshot.Body),
	}
}

func (s GoldenStore) path(name string) (string, error) {
	if s.Root == "" {
		return "", fmt.Errorf("golden root is required")
	}
	clean := filepath.Clean(name)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid golden name %q", name)
	}
	return filepath.Join(s.Root, clean), nil
}

func normalizeSnapshot(snapshot Snapshot) (Snapshot, error) {
	if snapshot.normalized {
		return snapshot, nil
	}
	if snapshot.Headers == nil {
		snapshot.Headers = map[string]string{}
	}
	if isJSON(snapshot.Headers["Content-Type"]) {
		body, err := NormalizeBody(snapshot.Body)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Body = body
	}
	snapshot.normalized = true
	return snapshot, nil
}

func normalizeRequestSpec(request RequestSpec) RequestSpec {
	request.Query = normalizeRequestValues(request.Query)
	request.Form = normalizeRequestValues(request.Form)
	return request
}

func normalizeRequestValues(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return values
	}
	normalized := make(map[string][]string, len(values))
	for key, original := range values {
		replacement := append([]string(nil), original...)
		if isObjectIDField(key) {
			for index, value := range replacement {
				if objectIDValuePattern.MatchString(value) {
					replacement[index] = "OID_TOKEN"
				}
			}
		}
		normalized[key] = replacement
	}
	return normalized
}

func isObjectIDField(key string) bool {
	for _, field := range objectIDFields {
		if strings.EqualFold(key, field) {
			return true
		}
	}
	return false
}

func isJSON(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "application/json")
}
