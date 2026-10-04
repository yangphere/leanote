package controllers

import (
	"net/url"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
)

func TestBindWebNoteOrContentPreservesIndexedTagsAndStrictExpectedUsn(t *testing.T) {
	params := &httpserver.Params{Query: url.Values{
		"NoteId":      {"507f1f77bcf86cd799439011"},
		"Tags[0]":     {"one"},
		"Tags[1]":     {"two"},
		"ExpectedUsn": {"7"},
	}}
	value, err := bindWebNoteOrContent(&httpserver.Context{Params: params})
	if err != nil {
		t.Fatalf("bind note: %v", err)
	}
	if value.Tags != "one,two" || value.ExpectedUsn != 7 {
		t.Fatalf("bound note = %#v", value)
	}

	params.Query["ExpectedUsn"] = []string{"overflow"}
	if _, err := bindWebNoteOrContent(&httpserver.Context{Params: params}); err == nil {
		t.Fatal("invalid ExpectedUsn was accepted")
	}
}

func TestNoteParameterStringsAcceptsProductionArrayEncodings(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{name: "repeated", query: url.Values{"noteIds": {"one", "two"}}, want: []string{"one", "two"}},
		{name: "bracket", query: url.Values{"noteIds[]": {"one", "two"}}, want: []string{"one", "two"}},
		{name: "indexed", query: url.Values{"noteIds[0]": {"one"}, "noteIds[1]": {"two"}}, want: []string{"one", "two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &httpserver.Params{Query: tt.query}
			got := noteParameterStrings(params, "noteIds")
			if len(got) != len(tt.want) {
				t.Fatalf("noteParameterStrings() = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("noteParameterStrings() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestBlogTargetNoteIDsPrefersListAndFallsBackToSingleID(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{name: "list takes precedence", query: url.Values{"noteIds[]": {"one", "two"}, "noteId": {"legacy"}}, want: []string{"one", "two"}},
		{name: "single fallback", query: url.Values{"noteId": {"legacy"}}, want: []string{"legacy"}},
		{name: "missing targets", query: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blogTargetNoteIDs(&httpserver.Params{Query: tt.query})
			if len(got) != len(tt.want) {
				t.Fatalf("blogTargetNoteIDs() = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("blogTargetNoteIDs() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestAllNotesToBlogRequiresEveryTargetAndContinuesAfterFailure(t *testing.T) {
	var calls []string
	publish := func(noteID string) bool {
		calls = append(calls, noteID)
		return noteID != "failed"
	}

	if got := allNotesToBlog(nil, publish); got {
		t.Fatal("allNotesToBlog(empty) = true, want false")
	}
	if got := allNotesToBlog([]string{"one", "two"}, publish); !got {
		t.Fatal("allNotesToBlog(all success) = false, want true")
	}
	if got := allNotesToBlog([]string{"failed", "remaining"}, publish); got {
		t.Fatal("allNotesToBlog(partial failure) = true, want false")
	}
	wantCalls := []string{"one", "two", "failed", "remaining"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("publish calls = %#v, want %#v", calls, wantCalls)
	}
	for i := range calls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("publish calls = %#v, want %#v", calls, wantCalls)
		}
	}
}

func TestPageParamDefaultsToFirstPage(t *testing.T) {
	for _, query := range []url.Values{
		nil,
		{"page": {"0"}},
		{"page": {"-2"}},
		{"page": {"invalid"}},
	} {
		if got := pageParam(&httpserver.Context{Params: &httpserver.Params{Query: query}}); got != 1 {
			t.Errorf("pageParam(%v) = %d, want 1", query, got)
		}
	}
	if got := pageParam(&httpserver.Context{Params: &httpserver.Params{Query: url.Values{"page": {"3"}}}}); got != 3 {
		t.Fatalf("pageParam(page=3) = %d, want 3", got)
	}
}

func TestRegisterNotesHTTPIncludesStrictNoteUpdateAction(t *testing.T) {
	registry := httpserver.NewRegistry()
	RegisterNotesHTTP(registry)
	for _, action := range []string{"UpdateNoteOrContent", "SearchNote"} {
		if _, ok := registry.Lookup("Note", action); !ok {
			t.Errorf("missing Note.%s", action)
		}
	}
	if _, ok := registry.Lookup("Notebook", "AddNotebook"); !ok {
		t.Error("missing Notebook.AddNotebook")
	}
	if _, ok := registry.Lookup("File", "CopyHttpImage"); !ok {
		t.Error("missing File.CopyHttpImage")
	}
	for _, action := range []struct{ controller, name string }{
		{"Note", "ExportPDF"}, {"Notebook", "SortNotebooks"},
	} {
		if _, ok := registry.Lookup(action.controller, action.name); !ok {
			t.Errorf("missing %s.%s", action.controller, action.name)
		}
	}
}

func TestRegisterNotesHTTPPreservesAnonymousContentWhitelist(t *testing.T) {
	registry := httpserver.NewRegistry()
	RegisterNotesHTTP(registry)
	for _, action := range []struct{ controller, name string }{
		{"File", "OutputImage"},
		{"Attach", "Download"},
	} {
		entry, ok := registry.Lookup(action.controller, action.name)
		if !ok {
			t.Fatalf("missing public action %s.%s", action.controller, action.name)
		}
		if len(entry.Befores) != 1 {
			t.Errorf("%s.%s before hooks = %d, want web session only for anonymous whitelist", action.controller, action.name, len(entry.Befores))
		}
	}
	for _, action := range []struct{ controller, name string }{{"File", "DeleteImage"}, {"Attach", "DownloadAll"}} {
		entry, ok := registry.Lookup(action.controller, action.name)
		if !ok {
			t.Fatalf("missing protected action %s.%s", action.controller, action.name)
		}
		if len(entry.Befores) != 2 {
			t.Errorf("%s.%s before hooks = %d, want session plus authentication", action.controller, action.name, len(entry.Befores))
		}
	}
}
