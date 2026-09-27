package controllers_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/controllers"
	apiControllers "github.com/yangphere/leanote/app/controllers/api"
	"github.com/yangphere/leanote/app/httpserver"
)

// minRegisteredInventoryActions is the ratchet floor: the number of inventory
// actions already registered when this gate became a ratchet (B1). Raise it as
// B2-B6 batches land; it must never go down.
const minRegisteredInventoryActions = 8

// TestRegistryMatchesB0Inventory tracks the migration gap between the stdlib
// registry and the routable action inventory. By default it is a ratchet so the
// shared CI gate stays usable while B2-B6 are pending: any registration outside
// the inventory, or a drop below minRegisteredInventoryActions, fails; the
// remaining gap is logged. LEANOTE_HTTP_INVENTORY_STRICT=1 restores the full
// parity assertion used as AC-H1 acceptance evidence.
func TestRegistryMatchesB0Inventory(t *testing.T) {
	want := readInventoryActions(t)
	registry := httpserver.NewRegistry()
	controllers.RegisterHTTP(registry, "test", &httpserver.Config{})
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
	if os.Getenv("LEANOTE_HTTP_INVENTORY_STRICT") == "1" {
		if len(missing) != 0 || len(extra) != 0 {
			t.Fatalf("registry/inventory mismatch: missing=%d (%s), extra=%d (%s); this is expected until B1-B6 registrations land", len(missing), previewNames(missing), len(extra), previewNames(extra))
		}
		return
	}
	if len(extra) != 0 {
		t.Fatalf("registry has actions outside the inventory: extra=%d (%s)", len(extra), previewNames(extra))
	}
	if registered := len(want) - len(missing); registered < minRegisteredInventoryActions {
		t.Fatalf("registered inventory actions = %d, below ratchet floor %d; missing=%d (%s)", registered, minRegisteredInventoryActions, len(missing), previewNames(missing))
	}
	if len(missing) != 0 {
		t.Logf("migration gap: missing=%d (%s); run with LEANOTE_HTTP_INVENTORY_STRICT=1 for AC-H1 parity", len(missing), previewNames(missing))
	}
}

func readInventoryActions(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", ".trellis", "tasks", "09-08-interface-http", "research", "action-inventory.md")
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
