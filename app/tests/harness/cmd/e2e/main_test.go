package main

import "testing"

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
