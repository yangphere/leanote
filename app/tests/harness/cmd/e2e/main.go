// Command e2e is the isolated test-mode supervisor for browser E2E runs.
//
// It owns the full identity contract required by the jquery-upgrade PRD
// (R-jQ3/R-jQ6): restore the leanote_test Mongo fixture, generate a fresh
// cryptographic run token, write the unique e2e_runs marker, rotate the
// fixture admin password, start the Revel test-mode server with the run
// token injected, run the requested child command with the per-run
// credentials in its environment, then unconditionally stop the server,
// delete the marker and destroy the fixture.
//
// Usage:
//
//	go run ./app/tests/harness/cmd/e2e -- <command> [args...]
//
// The raw token and password are never printed; when running under GitHub
// Actions they are masked via workflow commands before the child starts.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/tests/harness"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	fixtureDatabaseName = "leanote_test"
	fixtureDatabaseURL  = "mongodb://127.0.0.1:27017/" + fixtureDatabaseName
	e2eRunKind          = "browser-e2e"
	e2eRunTokenEnv      = "LEANOTE_E2E_RUN_TOKEN"
	adminUsername       = "admin"
)

// supervisorState tracks the live child process and the Revel server so the
// signal handler can tear the whole process tree down in the right order:
// terminate the child and its process group, stop the server, remove the
// marker, destroy the fixture. Go defers do not run on os.Exit, so the
// signal path performs the same steps itself.
type supervisorState struct {
	mu                sync.Mutex
	server            *harness.Server
	child             *exec.Cmd
	markerRunID       string
	markerTokenSha256 string
}

var supervisor supervisorState

func (s *supervisorState) setServer(server *harness.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server = server
}

func (s *supervisorState) setChild(command *exec.Cmd) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.child = command
}

// setMarker publishes the exact marker identity before the database insert.
// Publishing first closes the interrupt window in which the marker may have
// been written while the signal handler still has no selector to remove it.
func (s *supervisorState) setMarker(runID, tokenSha256 string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markerRunID = runID
	s.markerTokenSha256 = tokenSha256
}

func (s *supervisorState) markerSelector() (bson.M, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markerRunID == "" || s.markerTokenSha256 == "" {
		return nil, false
	}
	return bson.M{
		"kind":        e2eRunKind,
		"runId":       s.markerRunID,
		"tokenSha256": s.markerTokenSha256,
	}, true
}

func (s *supervisorState) stopChild() error {
	s.mu.Lock()
	command := s.child
	s.mu.Unlock()
	if command == nil || command.Process == nil {
		return nil
	}
	if err := killProcessTree(command); err != nil {
		return fmt.Errorf("terminate child process tree: %w", err)
	}
	return nil
}

func (s *supervisorState) stopServer() error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Close()
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "e2e harness: %v\n", err)
		os.Exit(1)
	}
}

// installSignalTeardown tears the environment down when the supervisor is
// interrupted, mirroring the normal-path shutdown order (child tree →
// server → marker → fixture) and aggregating every close error.
func installSignalTeardown() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		received := <-signals
		fmt.Fprintf(os.Stderr, "e2e harness: %v received; tearing down\n", received)
		var errs []error
		if err := supervisor.stopChild(); err != nil {
			errs = append(errs, err)
		}
		if err := supervisor.stopServer(); err != nil {
			errs = append(errs, fmt.Errorf("stop server: %w", err))
		}
		errs = append(errs, teardown()...)
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "e2e harness: teardown: %v\n", err)
		}
		os.Exit(130)
	}()
}

// assertSupervisorEnvironment enforces the supervisor's exclusive self-built
// contract before any resource is created: it never consumes an external
// service (a service declaration plus a self-built attempt is a conflict),
// and the Mongo port must be free so a stale listener fails with a clear
// message instead of an opaque docker port-allocation error.
func assertSupervisorEnvironment() error {
	if os.Getenv(harness.RequireMongoEnv) == "1" {
		return fmt.Errorf("%s declares a service MongoDB, but the e2e supervisor always self-provisions; unset it", harness.RequireMongoEnv)
	}
	if os.Getenv(harness.ServiceMongoURLEnv) != "" {
		return fmt.Errorf("%s declares a service MongoDB, but the e2e supervisor always self-provisions; unset it", harness.ServiceMongoURLEnv)
	}
	if err := harness.AssertPortFree(harness.DefaultMongoAddr); err != nil {
		return fmt.Errorf("e2e supervisor requires an exclusive MongoDB port: %w", err)
	}
	return nil
}

func run(args []string) (childExitError error) {
	separator := -1
	for index, arg := range args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(args) {
		return fmt.Errorf("usage: go run ./app/tests/harness/cmd/e2e -- <command> [args...]")
	}
	child := args[separator+1:]

	repoRoot, err := harness.RepositoryRoot()
	if err != nil {
		return err
	}

	if err := assertSupervisorEnvironment(); err != nil {
		return err
	}

	environment := harness.NewMongoEnvironment(repoRoot)
	// Install the signal handler BEFORE any resource is created: teardown is
	// idempotent, so an interrupt during fixture startup (docker run/restore/
	// ping) still removes a partially created container.
	installSignalTeardown()
	if err := environment.Up(); err != nil {
		return fmt.Errorf("bring up Mongo fixture: %w", err)
	}
	// Every failure after the fixture is up must still tear the fixture and
	// the server down; teardown failures must fail the run as well.
	defer func() {
		var errs []error
		if err := supervisor.stopChild(); err != nil {
			errs = append(errs, err)
		}
		if err := supervisor.stopServer(); err != nil {
			errs = append(errs, fmt.Errorf("stop server: %w", err))
		}
		errs = append(errs, teardown()...)
		for _, teardownErr := range errs {
			fmt.Fprintf(os.Stderr, "e2e harness: teardown: %v\n", teardownErr)
		}
		if len(errs) > 0 && childExitError == nil {
			childExitError = fmt.Errorf("harness teardown failed")
		}
	}()

	client, err := mongo.Connect(options.Client().ApplyURI(fixtureDatabaseURL).SetConnectTimeout(10 * time.Second).
		SetRegistry(lea.CodecRegistry).SetBSONOptions(&options.BSONOptions{DefaultDocumentM: true}))
	if err != nil {
		return fmt.Errorf("connect fixture database: %w", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := client.Ping(pingCtx, nil); err != nil {
		pingCancel()
		return fmt.Errorf("ping fixture database: %w", err)
	}
	pingCancel()
	database := client.Database(fixtureDatabaseName)

	runToken, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate run token: %w", err)
	}
	runId, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate run id: %w", err)
	}
	tokenSha256 := sha256Hex(runToken)
	// Publish the selector before writing the marker so an interrupt at any
	// point during the insert can still remove only this run's marker.
	supervisor.setMarker(runId, tokenSha256)
	insertCtx, insertCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if _, err := database.Collection("e2e_runs").InsertOne(insertCtx, bson.M{
		"runId":       runId,
		"kind":        e2eRunKind,
		"tokenSha256": tokenSha256,
		"createdAt":   time.Now().UTC(),
	}); err != nil {
		insertCancel()
		return fmt.Errorf("write e2e_runs marker: %w", err)
	}
	insertCancel()

	var admin struct {
		UserId   lea.ObjectID `bson:"_id"`
		Email    string       `bson:"Email"`
		Username string       `bson:"Username"`
	}
	adminCtx, adminCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer adminCancel()
	if err := database.Collection("users").FindOne(adminCtx, bson.M{"Username": adminUsername}).Decode(&admin); err != nil {
		return fmt.Errorf("locate fixture admin %q: %w", adminUsername, err)
	}
	password, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate admin password: %w", err)
	}
	if err := database.Collection("users").FindOneAndUpdate(adminCtx, bson.M{"_id": admin.UserId}, bson.M{"$set": bson.M{"Pwd": lea.GenPwd(password)}}).Err(); err != nil {
		return fmt.Errorf("rotate fixture admin password: %w", err)
	}

	mask(runToken)
	mask(password)

	os.Setenv(e2eRunTokenEnv, runToken)
	server, err := harness.StartServerProcessWithRegistration(supervisor.setServer)
	if err != nil {
		return fmt.Errorf("start test-mode server: %w", err)
	}

	command := exec.Command(child[0], child[1:]...)
	command.Env = append(os.Environ(),
		"LEANOTE_BASE_URL="+server.BaseURL,
		"LEANOTE_E2E_EMAIL="+admin.Email,
		"LEANOTE_E2E_PASSWORD="+password,
		e2eRunTokenEnv+"="+runToken,
	)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := setChildProcessGroup(command); err != nil {
		return fmt.Errorf("prepare child process supervision: %w", err)
	}
	supervisor.setChild(command)
	fmt.Fprintf(os.Stderr, "e2e harness: running %s against %s\n", strings.Join(child, " "), server.BaseURL)
	if err := startChild(command); err != nil {
		return fmt.Errorf("start child command: %w", err)
	}
	return command.Wait()
}

// teardown removes the e2e_runs marker and destroys the Mongo fixture.
// Every step runs unconditionally: a marker-removal failure must never
// leak the container, so errors are aggregated instead of short-circuiting.
func teardown() []error {
	var errs []error
	if selector, ok := supervisor.markerSelector(); ok {
		client, err := mongo.Connect(options.Client().ApplyURI(fixtureDatabaseURL).SetConnectTimeout(10 * time.Second).
			SetRegistry(lea.CodecRegistry).SetBSONOptions(&options.BSONOptions{DefaultDocumentM: true}))
		if err != nil {
			errs = append(errs, fmt.Errorf("connect fixture database for marker removal: %w", err))
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if _, err := client.Database(fixtureDatabaseName).Collection("e2e_runs").DeleteMany(ctx, selector); err != nil {
				errs = append(errs, fmt.Errorf("delete e2e_runs marker: %w", err))
			}
			cancel()
			_ = client.Disconnect(context.Background())
		}
	}
	root, rootErr := harness.RepositoryRoot()
	if rootErr != nil {
		errs = append(errs, fmt.Errorf("resolve repository root for fixture teardown: %w", rootErr))
	} else if err := harness.NewMongoEnvironment(root).Down(); err != nil {
		errs = append(errs, fmt.Errorf("remove Mongo fixture: %w", err))
	}
	return errs
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func mask(value string) {
	if os.Getenv("GITHUB_ACTIONS") == "true" && value != "" {
		fmt.Printf("::add-mask::%s\n", value)
	}
}
