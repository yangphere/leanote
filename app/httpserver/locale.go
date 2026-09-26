package httpserver

import (
	"net/http"
	"strings"
)

// LocaleResolverFromConfig returns the request locale precedence used by the
// production adapter: configured cookie, Accept-Language, then default.
func LocaleResolverFromConfig(cfg *Config) func(*http.Request) string {
	cookieName := "LEANOTE_LANG"
	defaultLocale := "en-us"
	if cfg != nil {
		prefix := strings.TrimSpace(cfg.StringDefault("cookie.prefix", "LEANOTE"))
		cookieName = cfg.StringDefault("i18n.cookie", prefix+"_LANG")
		defaultLocale = cfg.StringDefault("i18n.default_language", defaultLocale)
	}
	return func(r *http.Request) string {
		if r != nil && cookieName != "" {
			if cookie, err := r.Cookie(cookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
				return strings.ToLower(strings.TrimSpace(cookie.Value))
			}
		}
		if r != nil {
			for _, candidate := range strings.Split(r.Header.Get("Accept-Language"), ",") {
				candidate = strings.TrimSpace(strings.SplitN(candidate, ";", 2)[0])
				if candidate != "" && candidate != "*" {
					return strings.ToLower(candidate)
				}
			}
		}
		return strings.ToLower(defaultLocale)
	}
}
