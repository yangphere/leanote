package controllers

import (
	"testing"

	"github.com/yangphere/leanote/app/service"
)

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
