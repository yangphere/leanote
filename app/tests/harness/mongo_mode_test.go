package harness

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setMongoModeEnvironment(t *testing.T, requireMongo, serviceURI string) {
	t.Helper()
	t.Setenv(RequireMongoEnv, requireMongo)
	t.Setenv(ServiceMongoURLEnv, serviceURI)
}

func TestResolveMongoTestModeSelfProvisionedUsesContainerAddress(t *testing.T) {
	setMongoModeEnvironment(t, "", "")
	mode, uri, err := ResolveMongoTestMode()
	if err != nil {
		t.Fatalf("ResolveMongoTestMode() error = %v", err)
	}
	if mode != MongoSelfProvisioned {
		t.Fatalf("mode = %v, want MongoSelfProvisioned", mode)
	}
	if uri != defaultServiceMongoURI {
		t.Fatalf("uri = %q, want %q", uri, defaultServiceMongoURI)
	}
}

func TestResolveMongoTestModeServiceBackedUsesDefaultURI(t *testing.T) {
	setMongoModeEnvironment(t, "1", "")
	mode, uri, err := ResolveMongoTestMode()
	if err != nil {
		t.Fatalf("ResolveMongoTestMode() error = %v", err)
	}
	if mode != MongoServiceBacked {
		t.Fatalf("mode = %v, want MongoServiceBacked", mode)
	}
	if uri != defaultServiceMongoURI {
		t.Fatalf("uri = %q, want %q", uri, defaultServiceMongoURI)
	}
}

func TestResolveMongoTestModeServiceBackedAcceptsOverrideWithFixtureDatabase(t *testing.T) {
	override := "mongodb://127.0.0.1:28017/" + MongoFixtureDB
	setMongoModeEnvironment(t, "1", override)
	mode, uri, err := ResolveMongoTestMode()
	if err != nil {
		t.Fatalf("ResolveMongoTestMode() error = %v", err)
	}
	if mode != MongoServiceBacked {
		t.Fatalf("mode = %v, want MongoServiceBacked", mode)
	}
	if uri != override {
		t.Fatalf("uri = %q, want override %q", uri, override)
	}
}

func TestResolveMongoTestModeRejectsNonFixtureDatabase(t *testing.T) {
	setMongoModeEnvironment(t, "1", "mongodb://127.0.0.1:27017/leanote")
	_, _, err := ResolveMongoTestMode()
	if err == nil || !strings.Contains(err.Error(), `must target database "leanote_test"`) {
		t.Fatalf("ResolveMongoTestMode() error = %v, want database mismatch", err)
	}
}

func TestResolveMongoTestModeRejectsConflictingEnvironmentSources(t *testing.T) {
	setMongoModeEnvironment(t, "", "mongodb://127.0.0.1:27017/"+MongoFixtureDB)
	_, _, err := ResolveMongoTestMode()
	if err == nil || !strings.Contains(err.Error(), "mixes two environment sources") {
		t.Fatalf("ResolveMongoTestMode() error = %v, want conflicting sources failure", err)
	}
}

func TestSanitizeMongoURIStripsCredentials(t *testing.T) {
	got := SanitizeMongoURI("mongodb://user:secret@127.0.0.1:27017/leanote_test?replicaSet=rs0")
	want := "mongodb://127.0.0.1:27017/leanote_test?replicaSet=rs0"
	if got != want {
		t.Fatalf("SanitizeMongoURI() = %q, want %q", got, want)
	}
}

func TestRestoreServiceFixtureNeverInvokesDocker(t *testing.T) {
	repoRoot := t.TempDir()
	fixture := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	var commands []string
	env := MongoEnvironment{
		RepoRoot: repoRoot,
		run: func(name string, args ...string) (string, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return "", nil
		},
		ping:     func(string) error { return nil },
		lookPath: func(string) (string, error) { return "/usr/bin/mongorestore", nil },
	}

	if err := env.RestoreServiceFixture("mongodb://127.0.0.1:27017/" + MongoFixtureDB); err != nil {
		t.Fatalf("RestoreServiceFixture() error = %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %v, want exactly one restore command", commands)
	}
	for _, command := range commands {
		if strings.HasPrefix(command, "docker") {
			t.Fatalf("service-backed mode invoked docker: %s", command)
		}
	}
	want := fmt.Sprintf("mongorestore --uri mongodb://127.0.0.1:27017/%s --db %s --dir %s --drop --quiet", MongoFixtureDB, MongoFixtureDB, fixture)
	if commands[0] != want {
		t.Fatalf("restore command = %q, want %q", commands[0], want)
	}
}

func TestRestoreServiceFixtureFailsClosedBeforeAnyRestore(t *testing.T) {
	repoRoot := t.TempDir()
	fixture := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	var commands []string
	noopRun := func(name string, args ...string) (string, error) {
		commands = append(commands, name)
		return "", nil
	}

	unreachable := MongoEnvironment{RepoRoot: repoRoot, run: noopRun,
		ping:     func(string) error { return fmt.Errorf("connection refused") },
		lookPath: func(string) (string, error) { return "/usr/bin/mongorestore", nil }}
	if err := unreachable.RestoreServiceFixture(defaultServiceMongoURI); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("unreachable error = %v, want explicit failure", err)
	}

	missingTool := MongoEnvironment{RepoRoot: repoRoot, run: noopRun,
		ping:     func(string) error { return nil },
		lookPath: func(string) (string, error) { return "", fmt.Errorf("exec: not found") }}
	if err := missingTool.RestoreServiceFixture(defaultServiceMongoURI); err == nil || !strings.Contains(err.Error(), "requires mongorestore") {
		t.Fatalf("missing tool error = %v, want explicit failure", err)
	}

	missingFixture := MongoEnvironment{RepoRoot: t.TempDir(), run: noopRun,
		ping:     func(string) error { return nil },
		lookPath: func(string) (string, error) { return "/usr/bin/mongorestore", nil }}
	if err := missingFixture.RestoreServiceFixture(defaultServiceMongoURI); err == nil || !strings.Contains(err.Error(), "fixture is unavailable") {
		t.Fatalf("missing fixture error = %v, want explicit failure", err)
	}

	if len(commands) != 0 {
		t.Fatalf("commands = %v, want no restore attempts on failure paths", commands)
	}
}

func TestAssertPortFreeDetectsOccupiedAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	t.Cleanup(func() { _ = listener.Close() })

	if err := AssertPortFree(address); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("AssertPortFree(occupied) = %v, want already-in-use failure", err)
	}

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freeAddress := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err := AssertPortFree(freeAddress); err != nil {
		t.Fatalf("AssertPortFree(free) = %v, want nil", err)
	}
}
