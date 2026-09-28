package controllers_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/controllers"
	adminControllers "github.com/yangphere/leanote/app/controllers/admin"
	apiControllers "github.com/yangphere/leanote/app/controllers/api"
	memberControllers "github.com/yangphere/leanote/app/controllers/member"
	"github.com/yangphere/leanote/app/httpserver"
)

// The archived B0 inventory remains the authoritative route-action baseline.
func TestRegistryMatchesB0Inventory(t *testing.T) {
	want := readInventoryActions(t)
	registry := httpserver.NewRegistry()
	controllers.RegisterHTTP(registry, "test", &httpserver.Config{})
	memberControllers.RegisterHTTP(registry)
	adminControllers.RegisterHTTP(registry)
	apiControllers.RegisterHTTP(registry, "test")

	got := make(map[string]bool)
	for _, name := range registry.Registered() {
		got[name] = true
	}
	var missing, extra []string
	for name := range want {
		if !got[name] {
			missing = append(missing, name)
		}
	}
	for name := range got {
		if !want[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("registry/inventory mismatch: missing=%d (%s), extra=%d (%s); every routable inventory action must be registered", len(missing), previewNames(missing), len(extra), previewNames(extra))
	}
}

func readInventoryActions(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", ".trellis", "tasks", "archive", "2026-09", "09-08-interface-http", "research", "action-inventory.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read action inventory: %v", err)
	}
	const begin, end = "<!-- ROUTABLE_ACTIONS_BEGIN -->", "<!-- ROUTABLE_ACTIONS_END -->"
	section := string(data)
	start := strings.Index(section, begin)
	finish := strings.Index(section, end)
	if start < 0 || finish <= start {
		t.Fatalf("inventory action markers missing")
	}
	section = section[start+len(begin) : finish]
	want := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			want[name] = true
		}
	}
	return want
}

func previewNames(names []string) string {
	if len(names) > 8 {
		return strings.Join(names[:8], ", ") + ", ..."
	}
	return strings.Join(names, ", ")
}
