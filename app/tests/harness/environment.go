package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	MongoContainerName = "leanote-test-mongo"
	MongoImage         = "mongo:8.0"
	MongoFixtureDB     = "leanote_test"
)

type commandRun func(name string, args ...string) (string, error)

type MongoEnvironment struct {
	RepoRoot string
	run      commandRun
	now      func() time.Time
	sleep    func(time.Duration)
}

func NewMongoEnvironment(repoRoot string) MongoEnvironment {
	return MongoEnvironment{
		RepoRoot: repoRoot,
		run:      runCommand,
		now:      time.Now,
		sleep:    time.Sleep,
	}
}

func (e MongoEnvironment) Up() (err error) {
	if e.RepoRoot == "" {
		return fmt.Errorf("repository root is required")
	}
	fixture := filepath.Join(e.RepoRoot, "mongodb_backup", "leanote_install_data")
	if info, err := os.Stat(fixture); err != nil || !info.IsDir() {
		return fmt.Errorf("Mongo fixture is unavailable at %s", fixture)
	}
	if e.run == nil {
		e.run = runCommand
	}
	if e.now == nil {
		e.now = time.Now
	}
	if e.sleep == nil {
		e.sleep = time.Sleep
	}

	// A prior --rm container might be gone already; the fresh run below is the
	// authoritative startup and must still surface errors.
	_, _ = e.run("docker", "rm", "-f", MongoContainerName)
	if _, err := e.run("docker", "run", "-d", "--rm", "--name", MongoContainerName, "-p", "27017:27017", MongoImage); err != nil {
		return fmt.Errorf("start MongoDB 5.0 fixture: %w", err)
	}
	// Once the container has been created, every later setup failure must
	// remove it so a following test can retry without a name collision.
	defer func() {
		if err != nil {
			_, _ = e.run("docker", "rm", "-f", MongoContainerName)
		}
	}()
	if err := e.waitForPing(); err != nil {
		return err
	}
	if _, err := e.run("docker", "cp", fixture, MongoContainerName+":/"); err != nil {
		return fmt.Errorf("copy Mongo fixture into container: %w", err)
	}
	if _, err := e.run("docker", "exec", MongoContainerName, "mongorestore", "--db", MongoFixtureDB, "--dir", "/leanote_install_data", "--drop"); err != nil {
		return fmt.Errorf("restore Mongo fixture: %w", err)
	}
	users, err := e.run("docker", "exec", MongoContainerName, "mongosh", "--quiet", MongoFixtureDB, "--eval", "db.users.countDocuments()")
	if err != nil {
		return fmt.Errorf("verify restored Mongo fixture: %w", err)
	}
	if strings.TrimSpace(users) != "2" {
		return fmt.Errorf("restored Mongo fixture has %q users, want 2", strings.TrimSpace(users))
	}
	return nil
}

func (e MongoEnvironment) Down() error {
	if e.run == nil {
		e.run = runCommand
	}
	output, err := e.run("docker", "rm", "-f", MongoContainerName)
	if err != nil {
		// Tearing down an environment that was never fully started (or was
		// already destroyed) is a no-op, not a failure; this keeps signal-time
		// cleanup idempotent.
		if strings.Contains(output, "No such container") {
			return nil
		}
		return err
	}
	return nil
}

func (e MongoEnvironment) waitForPing() error {
	deadline := e.now().Add(60 * time.Second)
	for {
		output, err := e.run("docker", "exec", MongoContainerName, "mongosh", "--quiet", "--eval", "db.runCommand({ping:1}).ok")
		if err == nil && strings.TrimSpace(output) == "1" {
			return nil
		}
		if !e.now().Before(deadline) {
			return fmt.Errorf("Mongo ping timeout after 60s")
		}
		e.sleep(500 * time.Millisecond)
	}
}

func runCommand(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
