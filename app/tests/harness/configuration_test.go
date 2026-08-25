package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssertTestConfigurationRejectsMissingTestSection(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "db.dbname=leanote\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "conf/app.conf [test]") {
		t.Fatalf("assertTestConfiguration() error = %v, want missing test section error", err)
	}
}

func TestAssertTestConfigurationRejectsNonLoopbackTestAddress(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=0.0.0.0\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "http.addr") {
		t.Fatalf("assertTestConfiguration() error = %v, want non-loopback test address error", err)
	}
}

func TestAssertTestConfigurationRejectsGlobalDatabaseURL(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "db.url=mongodb://localhost:27017/leanote\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "db.url") {
		t.Fatalf("assertTestConfiguration() error = %v, want unsafe db.url error", err)
	}
}

func TestAssertTestConfigurationAcceptsTestDatabaseURL(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "db.url=mongodb://localhost:27017/leanote\n[test]\nhttp.addr=127.0.0.1\ndb.url=mongodb://localhost:27017/leanote_test?authSource=admin\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want test URL override to be accepted", err)
	}
}

func TestAssertTestConfigurationAcceptsDatabaseURLWithUnsupportedMgoOption(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\ndb.url=mongodb://localhost:27017/leanote_test?tlsCAFile=mongo-ca.pem\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want db.Init-compatible URL to be accepted", err)
	}
}

func TestAssertTestConfigurationRejectsUnsafeDatabaseFromUnsupportedMgoOption(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\ndb.url=mongodb://localhost:27017/leanote_test?tlsCAFile=/etc/ssl/leanote\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	err := assertTestConfiguration(repoRoot)
	if err == nil || !strings.Contains(err.Error(), "db.Init selects database") || !strings.Contains(err.Error(), "mgo.ParseURL also failed") {
		t.Fatalf("assertTestConfiguration() error = %v, want distinct db.Init/mgo.ParseURL error", err)
	}
}

func TestAssertTestConfigurationAcceptsGlobalDatabaseURL(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "db.url=mongodb://localhost:27017/leanote_test\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want safe global db.url to be accepted", err)
	}
}

func TestAssertTestConfigurationAcceptsDatabaseURLUsingConfiguredDatabaseName(t *testing.T) {
	for _, key := range []string{"db.url", "db.urlEnv"} {
		t.Run(key, func(t *testing.T) {
			repoRoot := writeTestConfigFixture(t, key+"=mongodb://localhost:27017/\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
			if err := assertTestConfiguration(repoRoot); err != nil {
				t.Fatalf("assertTestConfiguration() error = %v, want db.Init db.dbname fallback to be accepted", err)
			}
		})
	}
}

func TestAssertTestConfigurationRejectsDatabaseURLFromEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_TEST_MONGO_URL", "mongodb://localhost:27017/leanote")
	repoRoot := writeTestConfigFixture(t, "db.urlEnv=${LEANOTE_TEST_MONGO_URL}\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "db.urlEnv") {
		t.Fatalf("assertTestConfiguration() error = %v, want unsafe db.urlEnv error", err)
	}
}

func TestAssertTestConfigurationAcceptsDatabaseURLFromEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_TEST_MONGO_URL", "mongodb://localhost:27017/leanote_test")
	repoRoot := writeTestConfigFixture(t, "db.urlEnv=${LEANOTE_TEST_MONGO_URL}\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want safe db.urlEnv to be accepted", err)
	}
}

func TestAssertTestConfigurationAcceptsTestDatabaseURLFromEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_TEST_MONGO_URL", "mongodb://localhost:27017/leanote_test")
	repoRoot := writeTestConfigFixture(t, "db.urlEnv=mongodb://localhost:27017/leanote\n[test]\nhttp.addr=127.0.0.1\ndb.urlEnv=${LEANOTE_TEST_MONGO_URL}\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want safe test db.urlEnv to be accepted", err)
	}
}

func TestAssertTestConfigurationExpandsEmbeddedDatabaseEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_TEST_MONGO_HOST", "localhost:27017")
	for _, key := range []string{"db.url", "db.urlEnv"} {
		t.Run(key, func(t *testing.T) {
			repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\n"+key+"=mongodb://${LEANOTE_TEST_MONGO_HOST}/leanote_test\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
			if err := assertTestConfiguration(repoRoot); err != nil {
				t.Fatalf("assertTestConfiguration() error = %v, want embedded %s expression to be accepted", err, key)
			}
		})
	}
}

func TestAssertTestConfigurationRejectsEmptyDatabaseOverride(t *testing.T) {
	for _, key := range []string{"db.url", "db.urlEnv"} {
		t.Run(key, func(t *testing.T) {
			repoRoot := writeTestConfigFixture(t, key+"= # intentionally empty\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
			if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("assertTestConfiguration() error = %v, want empty %s error", err, key)
			}
		})
	}
}

func TestAssertTestConfigurationRejectsEmptyDatabaseEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_EMPTY_MONGO_URL", "")
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\ndb.urlEnv=${LEANOTE_EMPTY_MONGO_URL}\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "LEANOTE_EMPTY_MONGO_URL") {
		t.Fatalf("assertTestConfiguration() error = %v, want empty environment value error", err)
	}
}

func TestAssertTestConfigurationAcceptsInlineComments(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "; global comment\n[test] # isolated section\nhttp.addr=127.0.0.1 # loopback test server\ndb.dbname=leanote_test # isolated fixture\nsite.url=http://127.0.0.1:28017 ; fixed test port\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want nil", err)
	}
}

func TestAssertTestConfigurationAcceptsColonSeparator(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr: 127.0.0.1\ndb.url: mongodb://localhost:27017/leanote_test?authSource=admin\ndb.dbname: leanote_test\nsite.url: http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatalf("assertTestConfiguration() error = %v, want colon-separated options to be accepted", err)
	}
}

func TestAssertTestConfigurationRejectsContinuedDatabaseURL(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\ndb.url=mongodb://localhost:27017/leanote_test?\n  authSource=admin\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "single-line Mongo URL") {
		t.Fatalf("assertTestConfiguration() error = %v, want continued URL rejection", err)
	}
}

func TestAssertTestConfigurationRejectsUnsafeContinuedDatabaseURL(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "[test]\nhttp.addr=127.0.0.1\ndb.url=mongodb://localhost:27017/leanote_test?\n  tlsCAFile=/etc/ssl/leanote\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	err := assertTestConfiguration(repoRoot)
	if err == nil || !strings.Contains(err.Error(), "single-line Mongo URL") {
		t.Fatalf("assertTestConfiguration() error = %v, want continued URL rejection", err)
	}
}

func TestAssertTestConfigurationRejectsContinuationWithoutSection(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "  db.url=mongodb://localhost:27017/leanote\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	err := assertTestConfiguration(repoRoot)
	if err == nil || !strings.Contains(err.Error(), "continuation without an active section") {
		t.Fatalf("assertTestConfiguration() error = %v, want explicit continuation error", err)
	}
}

func TestAssertTestConfigurationRejectsDatabaseURLFragment(t *testing.T) {
	repoRoot := writeTestConfigFixture(t, "db.url=mongodb://localhost:27017/leanote_test#fragment\n[test]\nhttp.addr=127.0.0.1\ndb.dbname=leanote_test\nsite.url=http://127.0.0.1:28017\n")
	if err := assertTestConfiguration(repoRoot); err == nil || !strings.Contains(err.Error(), "db.url") {
		t.Fatalf("assertTestConfiguration() error = %v, want URL fragment to change the database name", err)
	}
}

func TestSplitConfigLinePreservesHashWithoutLeadingWhitespace(t *testing.T) {
	_, value, ok := splitConfigLine("db.url=mongodb://localhost:27017/leanote_test#fragment")
	if !ok || value != "mongodb://localhost:27017/leanote_test#fragment" {
		t.Fatalf("splitConfigLine() = %q, %v, want unmodified URL fragment", value, ok)
	}
}

func TestSplitConfigLineSupportsColonBeforeURLColon(t *testing.T) {
	key, value, ok := splitConfigLine("db.url: mongodb://localhost:27017/leanote_test")
	if !ok || key != "db.url" || value != "mongodb://localhost:27017/leanote_test" {
		t.Fatalf("splitConfigLine() = %q, %q, %v, want colon-separated key/value", key, value, ok)
	}
}

func writeTestConfigFixture(t *testing.T, body string) string {
	t.Helper()
	repoRoot := t.TempDir()
	confDir := filepath.Join(repoRoot, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "app.conf"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return repoRoot
}
