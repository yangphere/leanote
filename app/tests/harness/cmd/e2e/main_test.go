package main

import (
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/tests/harness"
)

func TestAssertSupervisorEnvironmentRejectsServiceDeclaration(t *testing.T) {
	t.Setenv(harness.RequireMongoEnv, "1")
	t.Setenv(harness.ServiceMongoURLEnv, "")
	err := assertSupervisorEnvironment()
	if err == nil || !strings.Contains(err.Error(), "always self-provisions") {
		t.Fatalf("assertSupervisorEnvironment() = %v, want service-declaration rejection", err)
	}
}

func TestAssertSupervisorEnvironmentRejectsLeakedServiceURI(t *testing.T) {
	t.Setenv(harness.RequireMongoEnv, "")
	t.Setenv(harness.ServiceMongoURLEnv, "mongodb://127.0.0.1:27017/"+harness.MongoFixtureDB)
	err := assertSupervisorEnvironment()
	if err == nil || !strings.Contains(err.Error(), "always self-provisions") {
		t.Fatalf("assertSupervisorEnvironment() = %v, want rejection of a leaked service URI", err)
	}
}

func TestMarkerSelectorRequiresAndScopesCurrentRun(t *testing.T) {
	var state supervisorState
	if selector, ok := state.markerSelector(); ok || selector != nil {
		t.Fatalf("marker selector should be disabled before a run is registered: %#v, %v", selector, ok)
	}

	state.setMarker("run-123", "digest-abc")
	selector, ok := state.markerSelector()
	if !ok {
		t.Fatal("marker selector should be enabled after registration")
	}
	want := map[string]string{
		"kind":        e2eRunKind,
		"runId":       "run-123",
		"tokenSha256": "digest-abc",
	}
	for key, expected := range want {
		if actual, ok := selector[key].(string); !ok || actual != expected {
			t.Errorf("selector[%q] = %#v, want %q", key, selector[key], expected)
		}
	}
	if len(selector) != len(want) {
		t.Fatalf("selector contains unexpected fields: %#v", selector)
	}
}
