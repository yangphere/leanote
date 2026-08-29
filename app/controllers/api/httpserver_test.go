package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	apiMongoURL           = "mongodb://127.0.0.1:27017"
	apiTestDatabasePrefix = "leanote_api_test_"
)

var apiTestDatabaseCounter uint64

type apiTestAccount struct {
	email    string
	password string
}

// firstPartyAPIApp boots the real conf/routes with the api batch registered.
// Requires a reachable MongoDB at 127.0.0.1:27017 (skipped otherwise,
// mirroring the CI split between DB-independent and DB tests).
func firstPartyAPIApp(t *testing.T) (*httpserver.App, apiTestAccount) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:27017", 300*time.Millisecond)
	if err != nil {
		t.Skip("MongoDB not reachable")
	}
	conn.Close()

	databaseName := apiTestDatabaseName()
	initDatabaseStub(databaseName)
	t.Cleanup(func() { dropAPITestDatabase(t, databaseName) })
	service.InitService()
	InitService()
	account := seedAPITestAccount(t)

	data, err := os.ReadFile("../../../conf/routes")
	if err != nil {
		t.Fatalf("read conf/routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(data)
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	registry := httpserver.NewRegistry()
	RegisterHTTP(registry, "test")
	return &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: registry,
	}, account
}

// initDatabaseStub mirrors cmd/leanote's initDatabase for tests.
func initDatabaseStub(databaseName string) {
	db.Init(apiMongoURL+"/"+databaseName, databaseName)
}

func apiTestDatabaseName() string {
	sequence := atomic.AddUint64(&apiTestDatabaseCounter, 1)
	return fmt.Sprintf("%s%d_%d_%d", apiTestDatabasePrefix, os.Getpid(), time.Now().UnixNano(), sequence)
}

func seedAPITestAccount(t *testing.T) apiTestAccount {
	t.Helper()
	account := apiTestAccount{
		email:    fmt.Sprintf("api-test-%d@example.test", atomic.AddUint64(&apiTestDatabaseCounter, 1)),
		password: "abc123",
	}
	user := info.User{
		UserId:      db.NewObjectID(),
		Email:       account.email,
		Username:    account.email,
		Pwd:         lea.GenPwd(account.password),
		CreatedTime: time.Now(),
	}
	if user.Pwd == "" {
		t.Fatal("generate API test account password hash")
	}
	if !db.Insert(db.Users, user) {
		t.Fatal("insert API test account")
	}
	return account
}

func dropAPITestDatabase(t testing.TB, databaseName string) {
	t.Helper()
	opts := options.Client().ApplyURI(apiMongoURL)
	opts.SetConnectTimeout(300 * time.Millisecond)
	opts.SetServerSelectionTimeout(300 * time.Millisecond)
	client, err := mongo.Connect(opts)
	if err != nil {
		t.Errorf("connect to drop API test database %q: %v", databaseName, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Database(databaseName).Drop(ctx); err != nil {
		t.Errorf("drop API test database %q: %v", databaseName, err)
	}
	if err := client.Disconnect(context.Background()); err != nil {
		t.Errorf("disconnect API test database client: %v", err)
	}
}

func apiPost(t *testing.T, app *httpserver.App, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func apiLoginForm(account apiTestAccount, password string) string {
	return "email=" + url.QueryEscape(account.email) + "&pwd=" + url.QueryEscape(password)
}

func TestApiAuthLoginRoundTrip(t *testing.T) {
	app, account := firstPartyAPIApp(t)

	// Wrong password: wrongUsernameOrPassword envelope.
	rec := apiPost(t, app, "/api/auth/login", apiLoginForm(account, "wrong"))
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	var bad struct {
		Ok  bool
		Msg string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bad); err != nil || bad.Ok {
		t.Fatalf("bad login body = %q err=%v", rec.Body.String(), err)
	}

	// Good credentials: token issued.
	rec = apiPost(t, app, "/api/auth/login", apiLoginForm(account, account.password))
	var ok struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil || !ok.Ok || ok.Token == "" {
		t.Fatalf("login body = %q err=%v", rec.Body.String(), err)
	}
}

func TestApiAuthLogoutClearsToken(t *testing.T) {
	app, account := firstPartyAPIApp(t)
	login := apiPost(t, app, "/api/auth/login", apiLoginForm(account, account.password))
	var issued struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(login.Body.Bytes(), &issued); err != nil || !issued.Ok {
		t.Fatalf("login = %q", login.Body.String())
	}

	rec := apiPost(t, app, "/api/auth/logout", "token="+issued.Token)
	var out struct {
		Ok bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.Ok {
		t.Fatalf("logout body = %q", rec.Body.String())
	}
	// The stored userId for the token must be gone.
	if uid := sessionService.GetUserId(issued.Token); uid != "" {
		t.Fatalf("token still resolves userId %q after logout", uid)
	}
}

func TestApiTagLifecycle(t *testing.T) {
	app, account := firstPartyAPIApp(t)
	login := apiPost(t, app, "/api/auth/login", apiLoginForm(account, account.password))
	var issued struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(login.Body.Bytes(), &issued); err != nil || !issued.Ok {
		t.Fatalf("login = %q", login.Body.String())
	}

	// Add a tag: returns the tag document with Usn.
	rec := apiPost(t, app, "/api/tag/addTag", "tag=probe-tag&token="+issued.Token)
	var added struct {
		TagId string
		Usn   int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.TagId == "" {
		t.Fatalf("addTag body = %q err=%v", rec.Body.String(), err)
	}

	// GetSyncTags lists it.
	rec = apiPost(t, app, "/api/tag/getSyncTags", "afterUsn=0&maxEntry=100&token="+issued.Token)
	var tags []struct {
		Tag string
		Usn int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tags); err != nil {
		t.Fatalf("getSyncTags body = %q", rec.Body.String())
	}
	found := false
	for _, tg := range tags {
		if tg.Tag == "probe-tag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("probe-tag missing from sync tags: %q", rec.Body.String())
	}

	// Delete it at the Usn the addTag response reported (tags[0] may be a
	// different tag on a warm database).
	rec = apiPost(t, app, "/api/tag/deleteTag", "tag=probe-tag&usn="+strconv.Itoa(added.Usn)+"&token="+issued.Token)
	var deleted struct {
		Ok bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil || !deleted.Ok {
		t.Fatalf("deleteTag body = %q", rec.Body.String())
	}
}

func TestApiTestDatabaseIsIsolated(t *testing.T) {
	name := apiTestDatabaseName()
	if name == "leanote_test" || !strings.HasPrefix(name, apiTestDatabasePrefix) {
		t.Fatalf("API integration tests must use an isolated database, got %q", name)
	}
}
