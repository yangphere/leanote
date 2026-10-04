package controllers

import (
	"encoding/json"
	"testing"

	"github.com/yangphere/leanote/app/service"
)

func TestWorkspaceWebSaveResponse(t *testing.T) {
	for _, tc := range []struct {
		name      string
		result    service.WorkspaceCommandResult
		protected bool
		wantOK    bool
		wantUSN   bool
	}{
		{"protected commit", service.WorkspaceCommandResult{Committed: true, USN: 47}, true, true, true},
		{"legacy commit", service.WorkspaceCommandResult{Committed: true, USN: 47}, false, true, false},
		{"conflict", service.WorkspaceCommandResult{USN: 47, Error: service.WorkspaceConflict}, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(workspaceWebSaveResponse(tc.result, tc.protected))
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]interface{}
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			if value["Ok"] != tc.wantOK {
				t.Fatalf("unexpected response: %s", data)
			}
			usn, present := value["Usn"]
			if present != tc.wantUSN || (present && usn != float64(47)) {
				t.Fatalf("wrong committed revision: %s", data)
			}
			wantFields := 6
			if tc.wantUSN {
				wantFields++
			}
			if len(value) != wantFields {
				t.Fatalf("changed envelope: %s", data)
			}
			if !tc.wantOK && value["Msg"] != "conflict" {
				t.Fatalf("lost error: %s", data)
			}
		})
	}
}

func TestWorkspaceWebSaveMessage(t *testing.T) {
	tests := []struct {
		name     string
		category service.WorkspaceErrorCategory
		want     string
	}{
		{name: "not found", category: service.WorkspaceNotFound, want: "notExists"},
		{name: "unauthorized", category: service.WorkspaceUnauthorized, want: "noAuth"},
		{name: "existing category", category: service.WorkspaceConflict, want: "conflict"},
		{name: "empty category", category: "", want: "saveFailed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := workspaceWebSaveMessage(test.category); got != test.want {
				t.Fatalf("workspaceWebSaveMessage(%q) = %q, want %q", test.category, got, test.want)
			}
		})
	}
}
