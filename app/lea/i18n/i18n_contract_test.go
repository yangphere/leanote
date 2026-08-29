package i18n

import (
	"path/filepath"
	"runtime"
	"testing"
)

func loadRepoMessages(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	if err := loadMessages(filepath.Join(repoRoot, "messages")); err != nil {
		t.Fatalf("load repository messages: %v", err)
	}
}

// TestMessageContract pins the robfig/config-backed lookup semantics for all
// seven shipped locales. robfig/config itself is frozen for this task
// (replacement deferred to MOD-003); this contract is its pre-replacement
// baseline.
func TestMessageContract(t *testing.T) {
	loadRepoMessages(t)

	wantLangs := []string{"de-de", "en-us", "es-co", "fr-fr", "pt-pt", "zh-cn", "zh-hk"}
	// frozen legacy quirk: the walk also registers the messages root directory
	// itself as an empty pseudo-locale, so MessageLanguages reports 8 entries
	if got := len(MessageLanguages()); got != len(wantLangs)+1 {
		t.Fatalf("MessageLanguages = %d (%v), want %d incl. legacy root pseudo-locale", got, MessageLanguages(), len(wantLangs)+1)
	}
	for _, lang := range wantLangs {
		if !HasLang(lang) {
			t.Errorf("HasLang(%q) = false", lang)
		}
	}
	if !HasLang("messages") {
		t.Error(`HasLang("messages") = false; legacy root pseudo-locale must stay registered`)
	}

	search := map[string]string{
		"de-de": "Suchen", "en-us": "Search", "es-co": "Buscar",
		"fr-fr": "Chercher", "pt-pt": "Pesquisar",
		"zh-cn": "搜索", "zh-hk": "搜尋",
	}
	for lang, want := range search {
		if got := Message(lang, "Search"); got != want {
			t.Errorf("Message(%q, Search) = %q, want %q", lang, got, want)
		}
	}

	if got := Message("en-us", "userHasBeenRegistered", "foo"); got != "foo has been registered" {
		t.Errorf("interpolation en-us = %q", got)
	}
	if got := Message("zh-cn", "userHasBeenRegistered", "foo"); got != "foo已被注册" {
		t.Errorf("interpolation zh-cn = %q", got)
	}

	const missing = "__definitely_missing_message_key__"
	if got := Message("zh-cn", missing); got != "??? "+missing+" ???" {
		t.Errorf("missing key = %q, want placeholder form", got)
	}
}
