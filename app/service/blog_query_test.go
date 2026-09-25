package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

func TestLookupUserBlogBySubDomainRejectsAmbiguousOwner(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	if _, err := db.UserBlogs.RemoveAll(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.UserBlogs.RemoveAll(map[string]any{}); err != nil {
			t.Errorf("clean user blog mappings: %v", err)
		}
	})
	if err := db.UserBlogs.Insert(
		info.UserBlog{UserId: db.NewObjectID(), SubDomain: "shared"},
		info.UserBlog{UserId: db.NewObjectID(), SubDomain: "shared"},
	); err != nil {
		t.Fatal(err)
	}
	if result, err := (&BlogService{}).LookupUserBlogBySubDomain("shared"); err == nil || !result.UserId.IsZero() {
		t.Fatalf("ambiguous subdomain mapping = %+v, err=%v", result, err)
	}
}

func TestArchiveNoteTimeUsesPublicTimeForNonDateSorts(t *testing.T) {
	note := info.Note{
		PublicTime:  time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC),
		CreatedTime: time.Date(2022, time.January, 2, 0, 0, 0, 0, time.UTC),
		UpdatedTime: time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC),
	}
	if got := archiveNoteTime(note, "Title"); !got.Equal(note.PublicTime) {
		t.Fatalf("Title archive time=%v, want public time %v", got, note.PublicTime)
	}
	if got := archiveNoteTime(note, "CreatedTime"); !got.Equal(note.CreatedTime) {
		t.Fatalf("CreatedTime archive time=%v, want created time %v", got, note.CreatedTime)
	}
	if got := archiveNoteTime(note, "UpdatedTime"); !got.Equal(note.UpdatedTime) {
		t.Fatalf("UpdatedTime archive time=%v, want updated time %v", got, note.UpdatedTime)
	}
}

func TestParseBlogQueryValidatesPresenceAndBounds(t *testing.T) {
	query, err := ParseBlogQuery(BlogQueryInput{
		Page:               "2",
		PagePresent:        true,
		PageSize:           "25",
		PageSizePresent:    true,
		Sort:               "Title",
		SortPresent:        true,
		Keywords:           "  hello world  ",
		Tag:                "  go  ",
		ConfiguredPageSize: 10,
		ConfiguredSort:     "UpdatedTime",
		IsAsc:              true,
	})
	if err != nil {
		t.Fatalf("ParseBlogQuery: %v", err)
	}
	if query.Page != 2 || query.PageSize != 25 || query.SortField != "Title" || !query.IsAsc || query.Keywords != "hello world" || query.Tag != "go" {
		t.Fatalf("query = %+v", query)
	}

	for _, input := range []BlogQueryInput{
		{Page: "0", PagePresent: true},
		{Page: "10001", PagePresent: true},
		{PageSize: "0", PageSizePresent: true},
		{PageSize: "101", PageSizePresent: true},
		{PageSize: "not-a-number", PageSizePresent: true},
		{Sort: "ReadNum", SortPresent: true},
	} {
		if _, err := ParseBlogQuery(input); !errors.Is(err, ErrInvalidBlogQuery) {
			t.Errorf("ParseBlogQuery(%+v) error = %v, want ErrInvalidBlogQuery", input, err)
		}
	}
}

func TestParseBlogQueryFallsBackOnlyForMissingLegacyConfig(t *testing.T) {
	query, err := ParseBlogQuery(BlogQueryInput{ConfiguredPageSize: 0, ConfiguredSort: "unknown"})
	if err != nil {
		t.Fatalf("ParseBlogQuery: %v", err)
	}
	if query.Page != 1 || query.PageSize != DefaultBlogPageSize || query.SortField != "PublicTime" {
		t.Fatalf("query = %+v", query)
	}
}

func TestNormalizeBlogTextUsesUnicodeTrimAndLimits(t *testing.T) {
	if got, err := NormalizeBlogText("\u3000 hello \t", 10, 20); err != nil || got != "hello" {
		t.Fatalf("NormalizeBlogText = (%q, %v)", got, err)
	}
	if _, err := NormalizeBlogText(strings.Repeat("界", MaxBlogTagRunes+1), MaxBlogTagRunes, MaxBlogTagBytes); err == nil {
		t.Fatal("accepted an overlong tag")
	}
	if _, err := NormalizeBlogText(string([]byte{0xff}), MaxBlogKeywordsRunes, MaxBlogKeywordsBytes); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

func TestBlogSortFieldsAppendStableTieBreaker(t *testing.T) {
	if got := strings.Join(BlogSortFields("Title", false), ","); got != "-Title,-_id" {
		t.Fatalf("descending sort = %q", got)
	}
	if got := strings.Join(BlogSortFields("Title", true), ","); got != "Title,_id" {
		t.Fatalf("ascending sort = %q", got)
	}
}

func TestValidJSONPCallbackAcceptsOnlyIdentifierPaths(t *testing.T) {
	valid := []string{"cb", "app.cb", "_$._9"}
	for _, value := range valid {
		if !ValidJSONPCallback(value) {
			t.Errorf("ValidJSONPCallback(%q) = false", value)
		}
	}
	invalid := []string{"", "cb()", "cb(1)", "app..cb", ".cb", "cb.", "a b", strings.Repeat("a", 129)}
	for _, value := range invalid {
		if ValidJSONPCallback(value) {
			t.Errorf("ValidJSONPCallback(%q) = true", value)
		}
	}
}

func TestCanonicalizeBlogHostNormalizesPortCaseTrailingDotAndIDN(t *testing.T) {
	tests := map[string]string{
		"Example.COM:443":  "example.com",
		"example.com.":     "example.com",
		"xn--bcher-kva.de": "xn--bcher-kva.de",
		"bücher.de":        "xn--bcher-kva.de",
		"[::1]:8443":       "::1",
	}
	for input, want := range tests {
		got, err := CanonicalizeBlogHost(input)
		if err != nil || got != want {
			t.Errorf("CanonicalizeBlogHost(%q) = (%q, %v), want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"example.com:0", "example.com:abc", "foo..example.com", "foo/bar", "foo.example.com:70000"} {
		if _, err := CanonicalizeBlogHost(input); !errors.Is(err, ErrInvalidBlogHost) {
			t.Errorf("CanonicalizeBlogHost(%q) error = %v, want ErrInvalidBlogHost", input, err)
		}
	}
}

func TestSelectBlogHostTrustsForwardedHeadersOnlyFromAllowlistedProxy(t *testing.T) {
	if got, err := SelectBlogHost("origin.example", "host=spoof.example", "spoof.example", "192.0.2.10:1234", "198.51.100.0/24"); err != nil || got != "origin.example" {
		t.Fatalf("untrusted forwarded host = (%q, %v)", got, err)
	}
	if got, err := SelectBlogHost("origin.example", "host=public.example", "public.example", "198.51.100.10:1234", "198.51.100.0/24"); err != nil || got != "public.example" {
		t.Fatalf("trusted forwarded host = (%q, %v)", got, err)
	}
	if _, err := SelectBlogHost("origin.example", "host=one.example", "two.example", "198.51.100.10:1234", "198.51.100.0/24"); !errors.Is(err, ErrInvalidBlogHost) {
		t.Fatalf("conflicting forwarded host error = %v", err)
	}
	if _, err := SelectBlogHost("origin.example", "host=one.example,host=two.example", "", "198.51.100.10:1234", "198.51.100.0/24"); !errors.Is(err, ErrInvalidBlogHost) {
		t.Fatalf("multi-valued forwarded host error = %v", err)
	}
	if _, err := SelectBlogHost("bad/host", "host=public.example", "", "198.51.100.10:1234", "198.51.100.0/24"); !errors.Is(err, ErrInvalidBlogHost) {
		t.Fatalf("malformed original Host behind trusted proxy error = %v", err)
	}
}

func TestCanonicalizeStoredCustomDomainRejectsIP(t *testing.T) {
	if _, err := CanonicalizeStoredCustomDomain("127.0.0.1"); !errors.Is(err, ErrInvalidBlogHost) {
		t.Fatalf("IP custom domain error = %v", err)
	}
}
