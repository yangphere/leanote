package harness

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	MongoContainerName = "leanote-test-mongo"
	MongoImage         = "docker.io/library/mongo:8.0@sha256:376f5173003b5408d7b8e6989667231c0bf0cefdce379d7c814910429d1a7a85"
	MongoFixtureDB     = "leanote_test"
	// DefaultMongoAddr is the address both modes converge on: the self-built
	// container publishes it and the CI service listens there.
	DefaultMongoAddr = "127.0.0.1:27017"
	// RequireMongoEnv selects service-backed mode: an external MongoDB must
	// already be listening; the harness must not start containers.
	RequireMongoEnv = "LEANOTE_REQUIRE_MONGO"
	// ServiceMongoURLEnv optionally overrides the service-backed URI; its
	// database name must stay leanote_test.
	ServiceMongoURLEnv = "LEANOTE_TEST_MONGO_URL"
)

const defaultFixtureMongoURI = "mongodb://" + DefaultMongoAddr + "/" + MongoFixtureDB

type MongoTestMode int

const (
	// MongoSelfProvisioned keeps the historical behavior: the harness owns
	// the leanote-test-mongo container for the duration of each test.
	MongoSelfProvisioned MongoTestMode = iota
	// MongoServiceBacked consumes an externally provided MongoDB (for
	// example the CI service container) without any docker involvement.
	MongoServiceBacked
)

// ResolveMongoTestMode reads the mode environment on every call; callers must
// not cache the result because configuration_test mutates these variables via
// t.Setenv inside the same test binary. In self-provisioned mode the returned
// URI is the container's fixed address.
func ResolveMongoTestMode() (MongoTestMode, string, error) {
	uri := defaultFixtureMongoURI
	if os.Getenv(RequireMongoEnv) != "1" {
		if override := os.Getenv(ServiceMongoURLEnv); override != "" {
			return MongoSelfProvisioned, uri, fmt.Errorf(
				"%s declares a service MongoDB but %s is unset; this mixes two environment sources, unset one of them",
				ServiceMongoURLEnv, RequireMongoEnv)
		}
		return MongoSelfProvisioned, uri, nil
	}
	if override := os.Getenv(ServiceMongoURLEnv); override != "" {
		uri = override
	}
	database, err := mongoDatabaseName(uri)
	if err != nil {
		return MongoServiceBacked, uri, fmt.Errorf("%s is invalid: %w", ServiceMongoURLEnv, err)
	}
	if database != MongoFixtureDB {
		return MongoServiceBacked, uri, fmt.Errorf("service MongoDB URI must target database %q, got %q", MongoFixtureDB, database)
	}
	return MongoServiceBacked, uri, nil
}

func mongoDatabaseName(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "mongodb" {
		return "", fmt.Errorf("scheme must be mongodb, got %q", parsed.Scheme)
	}
	return strings.TrimPrefix(parsed.Path, "/"), nil
}

// SanitizeMongoURI strips credentials so the URI is safe to embed in logs and
// failure summaries.
func SanitizeMongoURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "mongodb://[invalid]"
	}
	parsed.User = nil
	return parsed.String()
}

type commandRun func(name string, args ...string) (string, error)

type MongoEnvironment struct {
	RepoRoot      string
	run           commandRun
	now           func() time.Time
	sleep         func(time.Duration)
	lookPath      func(string) (string, error)
	ping          func(uri string) error
	verifyFixture func(uri string) error
}

func NewMongoEnvironment(repoRoot string) MongoEnvironment {
	return MongoEnvironment{
		RepoRoot: repoRoot,
		run:      runCommand,
		now:      time.Now,
		sleep:    time.Sleep,
	}
}

func (e MongoEnvironment) fixtureDir() (string, error) {
	if e.RepoRoot == "" {
		return "", fmt.Errorf("repository root is required")
	}
	fixture := filepath.Join(e.RepoRoot, "mongodb_backup", "leanote_install_data")
	if info, err := os.Stat(fixture); err != nil || !info.IsDir() {
		return "", fmt.Errorf("Mongo fixture is unavailable at %s", fixture)
	}
	return fixture, nil
}

func (e MongoEnvironment) Up() (err error) {
	fixture, err := e.fixtureDir()
	if err != nil {
		return err
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
		return fmt.Errorf("start MongoDB 8.0 fixture: %w", err)
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

// RestoreServiceFixture resets the external service database to the fixture
// baseline. It runs on the host (mongorestore on PATH) and must never invoke
// docker: service-backed mode owns no containers. Called once per test so the
// suites stay order-independent, mirroring the in-container restore of Up(),
// including the post-restore users verification.
func (e MongoEnvironment) RestoreServiceFixture(uri string) error {
	if e.run == nil {
		e.run = runCommand
	}
	if e.ping == nil {
		e.ping = pingMongoURI
	}
	if e.lookPath == nil {
		e.lookPath = exec.LookPath
	}
	if e.verifyFixture == nil {
		e.verifyFixture = verifyServiceFixture
	}
	if err := e.ping(uri); err != nil {
		return fmt.Errorf("service MongoDB at %s is unreachable: %w", SanitizeMongoURI(uri), err)
	}
	if _, err := e.lookPath("mongorestore"); err != nil {
		return fmt.Errorf("service-backed mode requires mongorestore on PATH: %w", err)
	}
	fixture, err := e.fixtureDir()
	if err != nil {
		return err
	}
	if _, err := e.run("mongorestore", "--uri", uri, "--db", MongoFixtureDB, "--dir", fixture, "--drop", "--quiet"); err != nil {
		// runCommand echoes the full command line — including the URI and any
		// credentials it carries — so scrub the URI before the failure can
		// reach a test log.
		return fmt.Errorf("restore fixture into service MongoDB: %s",
			strings.ReplaceAll(err.Error(), uri, SanitizeMongoURI(uri)))
	}
	if err := e.verifyFixture(uri); err != nil {
		return fmt.Errorf("verify service fixture: %w", err)
	}
	return nil
}

// verifyServiceFixture mirrors Up()'s post-restore users check so a wrong or
// partial fixture fails identically in both modes.
func verifyServiceFixture(uri string) error {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(10 * time.Second))
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := client.Database(MongoFixtureDB).Collection("users").CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count != 2 {
		return fmt.Errorf("restored service fixture has %d users, want 2", count)
	}
	return nil
}

func pingMongoURI(uri string) error {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(10 * time.Second))
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return client.Ping(ctx, nil)
}

// AssertPortFree fails fast when something already listens on addr. The e2e
// supervisor must own Mongo exclusively; without this check an already-bound
// port surfaces later as an opaque docker port-allocation error.
func AssertPortFree(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%s is already in use; the self-built harness requires it free", addr)
	}
	if err := listener.Close(); err != nil {
		return fmt.Errorf("release %s after probe: %w", addr, err)
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
