package httpserver

import (
	"net/http"
	"testing"
)

func TestLocaleResolverHonorsCookieAcceptLanguageAndDefault(t *testing.T) {
	cfg, err := ParseConfig([]byte("cookie.prefix=LEANOTE\ni18n.default_language=zh-cn\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	resolve := LocaleResolverFromConfig(cfg)

	request, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept-Language", "fr-FR,zh-CN;q=0.8")
	if got := resolve(request); got != "fr-fr" {
		t.Fatalf("Accept-Language locale = %q, want fr-fr", got)
	}
	request.AddCookie(&http.Cookie{Name: "LEANOTE_LANG", Value: "de-DE"})
	if got := resolve(request); got != "de-de" {
		t.Fatalf("cookie locale = %q, want de-de", got)
	}

	request = request.Clone(request.Context())
	request.Header.Del("Accept-Language")
	request.Header.Del("Cookie")
	if got := resolve(request); got != "zh-cn" {
		t.Fatalf("default locale = %q, want zh-cn", got)
	}
}

func TestLocaleResolverUsesConfiguredCookieName(t *testing.T) {
	cfg, err := ParseConfig([]byte("i18n.cookie=LANG\ni18n.default_language=en-us\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	resolve := LocaleResolverFromConfig(cfg)
	request, err := http.NewRequest(http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "LANG", Value: "pt-PT"})
	if got := resolve(request); got != "pt-pt" {
		t.Fatalf("configured cookie locale = %q, want pt-pt", got)
	}
}
