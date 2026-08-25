package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMongoEnvironmentUpRestoresFixtureAndVerifiesUsers(t *testing.T) {
	repoRoot := t.TempDir()
	fixture := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}

	var commands []string
	runner := func(name string, args ...string) (string, error) {
		command := name + " " + strings.Join(args, " ")
		commands = append(commands, command)
		switch {
		case strings.Contains(command, "mongosh --quiet --eval"):
			return "1\n", nil
		case strings.Contains(command, "mongosh --quiet leanote_test"):
			return "2\n", nil
		default:
			return "", nil
		}
	}
	env := MongoEnvironment{RepoRoot: repoRoot, run: runner, sleep: func(time.Duration) {}}

	if err := env.Up(); err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	want := []string{
		"docker rm -f leanote-test-mongo",
		"docker run -d --rm --name leanote-test-mongo -p 27017:27017 mongo:5.0",
		"docker exec leanote-test-mongo mongosh --quiet --eval db.runCommand({ping:1}).ok",
		"docker cp " + fixture + " leanote-test-mongo:/",
		"docker exec leanote-test-mongo mongorestore --db leanote_test --dir /leanote_install_data --drop",
		"docker exec leanote-test-mongo mongosh --quiet leanote_test --eval db.users.countDocuments()",
	}
	if strings.Join(commands, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Up() commands =\n%s\nwant:\n%s", strings.Join(commands, "\n"), strings.Join(want, "\n"))
	}
}

func TestMongoEnvironmentUpFailsAfterPingTimeout(t *testing.T) {
	repoRoot := t.TempDir()
	fixture := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(0, 0)
	env := MongoEnvironment{
		RepoRoot: repoRoot,
		run: func(name string, args ...string) (string, error) {
			if name == "docker" && len(args) > 1 && args[0] == "exec" {
				return "", fmt.Errorf("not ready")
			}
			return "", nil
		},
		now:   func() time.Time { return now },
		sleep: func(delay time.Duration) { now = now.Add(delay) },
	}

	err := env.Up()
	if err == nil || !strings.Contains(err.Error(), "Mongo ping timeout after 60s") {
		t.Fatalf("Up() error = %v, want ping timeout", err)
	}
}

func TestMongoEnvironmentUpCleansContainerAfterSetupFailure(t *testing.T) {
	repoRoot := t.TempDir()
	fixture := filepath.Join(repoRoot, "mongodb_backup", "leanote_install_data")
	if err := os.MkdirAll(fixture, 0o755); err != nil {
		t.Fatal(err)
	}

	var commands []string
	runner := func(name string, args ...string) (string, error) {
		command := name + " " + strings.Join(args, " ")
		commands = append(commands, command)
		switch {
		case strings.Contains(command, "mongosh --quiet --eval"):
			return "1\n", nil
		case strings.HasPrefix(command, "docker cp "):
			return "", fmt.Errorf("copy failed")
		default:
			return "", nil
		}
	}
	env := MongoEnvironment{RepoRoot: repoRoot, run: runner, sleep: func(time.Duration) {}}

	if err := env.Up(); err == nil || !strings.Contains(err.Error(), "copy Mongo fixture") {
		t.Fatalf("Up() error = %v, want fixture copy failure", err)
	}
	if len(commands) == 0 || commands[len(commands)-1] != "docker rm -f leanote-test-mongo" {
		t.Fatalf("Up() cleanup commands = %v, want final container removal", commands)
	}
}

func TestMongoEnvironmentDownRemovesNamedContainer(t *testing.T) {
	var got string
	env := MongoEnvironment{run: func(name string, args ...string) (string, error) {
		got = name + " " + strings.Join(args, " ")
		return "", nil
	}}

	if err := env.Down(); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	if got != "docker rm -f leanote-test-mongo" {
		t.Fatalf("Down() command = %q", got)
	}
}
