package httpserver

import (
	"strings"
	"testing"
	"time"
)

func TestConfigGlobalAndSectionOverride(t *testing.T) {
	cfg, err := ParseConfig([]byte(strings.Join([]string{
		"http.port=9000",
		"db.dbname=leanote",
		"[prod]",
		"http.port=9100",
		"[test]",
		"http.addr=127.0.0.1",
		"db.dbname=leanote_test",
	}, "\n")), "test")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("http.addr"); !ok || v != "127.0.0.1" {
		t.Fatalf("http.addr = %q ok=%v, want 127.0.0.1 from [test]", v, ok)
	}
	if v, ok := cfg.String("db.dbname"); !ok || v != "leanote_test" {
		t.Fatalf("section must override default: got %q ok=%v", v, ok)
	}
	if v, ok := cfg.Int("http.port"); !ok || v != 9000 {
		t.Fatalf("key absent from active section falls back to default: got %d ok=%v", v, ok)
	}
}

func TestConfigMissingRunModeSectionIsFatal(t *testing.T) {
	_, err := ParseConfig([]byte("a=1\n"), "prod")
	if err == nil || !strings.Contains(err.Error(), "run mode") {
		t.Fatalf("ParseConfig err = %v, want missing-run-mode fatal", err)
	}
}

func TestConfigEnvInterpolation(t *testing.T) {
	t.Setenv("LEANOTE_TEST_DB_URL", "mongodb://127.0.0.1:27017/leanote_test")
	cfg, err := ParseConfig([]byte("db.urlEnv=${LEANOTE_TEST_DB_URL}\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("db.urlEnv"); !ok || v != "mongodb://127.0.0.1:27017/leanote_test" {
		t.Fatalf("db.urlEnv = %q ok=%v, want env-expanded value", v, ok)
	}

	// revel/config computeVar errors on an empty/unset ${VAR} resolution,
	// which collapses to found=false in Context.String — db.Init then falls
	// back to db.host/db.port instead of dialing an empty URL.
	cfg2, err := ParseConfig([]byte("x=${LEANOTE_TEST_EMPTY}\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if _, ok := cfg2.String("x"); ok {
		t.Fatalf("unset/empty ${VAR} must read as not-found")
	}
}

func TestConfigCycleExpansionIsAnError(t *testing.T) {
	// revel/config computeVar errors with "Possible cycle" when the depth
	// bound is exhausted; Context.String collapses that to found=false. The
	// same must hold here — a cycle must never surface a garbage value as
	// found.
	for _, conf := range []string{
		"a=%(a)s\n",
		"a=%(b)s\nb=%(a)s\n",
	} {
		cfg, err := ParseConfig([]byte(conf), "")
		if err != nil {
			t.Fatalf("ParseConfig(%q): %v", conf, err)
		}
		if v, ok := cfg.String("a"); ok || v != "" {
			t.Fatalf("cyclic key = %q ok=%v, want not-found collapse (conf=%q)", v, ok, conf)
		}
	}
}

func TestConfigUnparsableLineIsFatal(t *testing.T) {
	_, err := ParseConfig([]byte("justaword\n"), "")
	if err == nil || !strings.Contains(err.Error(), "could not parse line") {
		t.Fatalf("ParseConfig err = %v, want could-not-parse fatal", err)
	}
}

func TestConfigMultiplePlaceholdersInOneValue(t *testing.T) {
	cfg, err := ParseConfig([]byte("a=1\nb=2\ncombo=%(a)s-%(b)s-%(a)s\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("combo"); !ok || v != "1-2-1" {
		t.Fatalf("combo = %q ok=%v, want 1-2-1 (multi-match order preserved)", v, ok)
	}
}

func TestConfigVarThenEnvChain(t *testing.T) {
	t.Setenv("LEANOTE_TEST_PORT", "28017")
	cfg, err := ParseConfig([]byte("base=${LEANOTE_TEST_PORT}\ncombo=%(base)s/x\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("combo"); !ok || v != "28017/x" {
		t.Fatalf("combo = %q ok=%v, want 28017/x (var resolved first, then env in chain)", v, ok)
	}
}

func TestConfigExplicitEmptyReferencedKeyIsError(t *testing.T) {
	cfg, err := ParseConfig([]byte("e=\na=%(e)s\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("a"); ok || v != "" {
		t.Fatalf("a = %q ok=%v, want not-found collapse (explicit-empty reference errors like revel)", v, ok)
	}
}

func TestConfigEnvSetButEmptyIsNotFound(t *testing.T) {
	t.Setenv("LEANOTE_TEST_EMPTY", "")
	cfg, err := ParseConfig([]byte("x=${LEANOTE_TEST_EMPTY}\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if _, ok := cfg.String("x"); ok {
		t.Fatalf("set-but-empty ${VAR} must read as not-found")
	}
}

// TestParseRealShippedConfigs guards the strict (fatal-on-unparseable)
// parser against regressions on the actual shipped configuration files:
// every run-mode must load and key values must match the files.
func TestParseRealShippedConfigs(t *testing.T) {
	for _, path := range []string{"../../conf/app.conf", "../../conf/app.conf-default"} {
		for _, mode := range []string{"dev", "prod", "test"} {
			cfg, err := LoadConfigFile(path, mode)
			if err != nil {
				t.Fatalf("LoadConfigFile(%s, %s): %v", path, mode, err)
			}
			if cfg.Section() != mode {
				t.Fatalf("%s[%s]: Section() = %q", path, mode, cfg.Section())
			}
			if v, ok := cfg.Int("http.port"); !ok || v != 9000 {
				t.Fatalf("%s[%s]: http.port = %d ok=%v, want 9000", path, mode, v, ok)
			}
			if v, _ := cfg.String("adminUsername"); v != "admin" {
				t.Fatalf("%s[%s]: adminUsername = %q", path, mode, v)
			}
			if v, ok := cfg.String("app.secret"); !ok || v == "" {
				t.Fatalf("%s[%s]: app.secret missing/empty", path, mode)
			}
			wantDB := "leanote"
			if mode == "test" {
				wantDB = "leanote_test"
			}
			if v, ok := cfg.String("db.dbname"); !ok || v != wantDB {
				t.Fatalf("%s[%s]: db.dbname = %q ok=%v, want %q", path, mode, v, ok, wantDB)
			}
		}
		// Interpolation on the real file: [prod] log.warn.output = %(app.name)s.log.
		cfg, err := LoadConfigFile(path, "prod")
		if err != nil {
			t.Fatalf("LoadConfigFile(%s, prod): %v", path, err)
		}
		if v, ok := cfg.String("log.warn.output"); !ok || v != "leanote.log" {
			t.Fatalf("%s[prod]: log.warn.output = %q ok=%v, want leanote.log (%%(app.name)s interpolation)", path, v, ok)
		}
	}
}

func TestConfigVarInterpolation(t *testing.T) {
	cfg, err := ParseConfig([]byte(strings.Join([]string{
		"site=http://localhost:9000",
		"link=%(site)s/index",
	}, "\n")), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, _ := cfg.String("link"); v != "http://localhost:9000/index" {
		t.Fatalf("%%(...)s interpolation = %q", v)
	}
}

func TestConfigInlineCommentsAndQuotes(t *testing.T) {
	cfg, err := ParseConfig([]byte(strings.Join([]string{
		"db.port=9000 # required",
		`app.secret="quoted # value"`,
		"http.addr=0.0.0.0",
	}, "\n")), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, _ := cfg.String("db.port"); v != "9000" {
		t.Fatalf("inline comment must be stripped: %q", v)
	}
	if v, _ := cfg.String("app.secret"); v != "quoted # value" {
		t.Fatalf("quoted # must survive: %q", v)
	}
}

func TestConfigTypesAndDefaults(t *testing.T) {
	cfg, err := ParseConfig([]byte(strings.Join([]string{
		"cookie.secure=true",
		"session.expires=3h",
		"count=42",
	}, "\n")), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.Bool("cookie.secure"); !ok || !v {
		t.Fatalf("Bool = %v ok=%v", v, ok)
	}
	if v := cfg.BoolDefault("missing", true); !v {
		t.Fatal("BoolDefault missing must return default")
	}
	if v, ok := cfg.Int("count"); !ok || v != 42 {
		t.Fatalf("Int = %d ok=%v", v, ok)
	}
	if v := cfg.IntDefault("missing", 7); v != 7 {
		t.Fatal("IntDefault missing must return default")
	}
	d, err := time.ParseDuration(cfg.StringDefault("session.expires", "1h"))
	if err != nil || d != 3*time.Hour {
		t.Fatalf("duration round trip = %v err=%v", d, err)
	}
}

func TestConfigContinuationLines(t *testing.T) {
	cfg, err := ParseConfig([]byte("multi=first\n\tsecond\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, _ := cfg.String("multi"); !strings.Contains(v, "first") || !strings.Contains(v, "second") {
		t.Fatalf("continuation lines must append: %q", v)
	}
}

func TestConfigSecretAccessorForProdCheck(t *testing.T) {
	cfg, err := ParseConfig([]byte("[prod]\napp.secret=real-secret\n"), "prod")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("app.secret"); !ok || v != "real-secret" {
		t.Fatalf("app.secret = %q ok=%v", v, ok)
	}
}

func TestConfigCRLFInput(t *testing.T) {
	cfg, err := ParseConfig([]byte("db.dbname=leanote\r\nhttp.port=9000\r\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg.String("db.dbname"); !ok || v != "leanote" {
		t.Fatalf("CRLF input db.dbname = %q ok=%v (trailing CR must be stripped)", v, ok)
	}
	if v, ok := cfg.Int("http.port"); !ok || v != 9000 {
		t.Fatalf("CRLF input http.port = %d ok=%v", v, ok)
	}
}

func TestConfigBoolVocabulary(t *testing.T) {
	cfg, err := ParseConfig([]byte("a=on\nb=NO\nc=y\nd=Off\ne=true\nf=0\ng=bogus\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	want := map[string]struct {
		v     bool
		found bool
	}{
		"a": {true, true},
		"b": {false, true},
		"c": {true, true},
		"d": {false, true},
		"e": {true, true},
		"f": {false, true},
		"g": {false, false}, // unknown word: found=false (OptionError collapse)
	}
	for key, expect := range want {
		v, ok := cfg.Bool(key)
		if v != expect.v || ok != expect.found {
			t.Errorf("Bool(%q) = %v,%v; want %v,%v", key, v, ok, expect.v, expect.found)
		}
	}
	// "log.trace.output = off" — the exact line shipped in app.conf-default:
	// the value must read as "off" (revel Bool vocabulary), not fall back.
	cfg2, err := ParseConfig([]byte("log.trace.output = off\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if v, ok := cfg2.Bool("log.trace.output"); !ok || v != false {
		t.Fatalf("Bool(log.trace.output) = %v,%v; want false,true (revel y/n/on/off vocabulary)", v, ok)
	}
}
