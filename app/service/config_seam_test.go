package service

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

type mapAppConfig map[string]string

func (m mapAppConfig) String(key string) (string, bool) {
	value, ok := m[key]
	return value, ok
}

// withAppConfigSource installs a first-party source and restores both the
// seam and the derived site.url domain globals afterwards.
func withAppConfigSource(t *testing.T, source AppConfigSource) {
	t.Helper()
	savedSource := appConfigSource
	savedDomain, savedSchema, savedPort := defaultDomain, schema, port
	t.Cleanup(func() {
		appConfigSource = savedSource
		defaultDomain, schema, port = savedDomain, savedSchema, savedPort
	})
	SetAppConfigSource(source)
}

func TestInitGlobalConfigsUsesInjectedAppConfigSource(t *testing.T) {
	withAppConfigSource(t, mapAppConfig{"adminUsername": "root", "site.url": "https://notes.example.test"})
	adminID := db.MustObjectIDFromHex("507f1f77bcf86cd799439011")
	demoID := db.MustObjectIDFromHex("507f1f77bcf86cd799439012")
	var lookedUp []string
	svc := &ConfigService{
		findAdminUser: func(login string) info.User {
			lookedUp = append(lookedUp, login)
			return info.User{UserId: adminID}
		},
		loadGlobalConfigs: func() ([]info.Config, error) {
			return []info.Config{
				{Key: "openRegister", ValueStr: "1"},
				{Key: "demoUserId", ValueStr: demoID.Hex()},
				{Key: "demoUsername", ValueStr: "demo@example.test"},
			}, nil
		},
		findDemoUser: func(string) (info.User, error) { return info.User{UserId: demoID}, nil },
	}
	if svc.GlobalSnapshotLoaded() {
		t.Fatal("snapshot reported loaded before InitGlobalConfigsWithError")
	}

	if err := svc.InitGlobalConfigsWithError(); err != nil {
		t.Fatalf("InitGlobalConfigsWithError: %v", err)
	}
	if len(lookedUp) != 1 || lookedUp[0] != "root" {
		t.Fatalf("admin lookups = %v, want the injected adminUsername", lookedUp)
	}
	if !svc.GlobalSnapshotLoaded() {
		t.Fatal("snapshot not reported loaded after success")
	}
	if svc.GetAdminUsername() != "root" || svc.GetAdminUserId() != adminID.Hex() {
		t.Fatalf("admin = %q/%q, want root/%s", svc.GetAdminUsername(), svc.GetAdminUserId(), adminID.Hex())
	}
	if got := svc.GetSiteUrl(); got != "https://notes.example.test" {
		t.Fatalf("site url = %q", got)
	}
	if !svc.IsOpenRegister() {
		t.Fatal("openRegister from the snapshot was not applied")
	}
	if isDemo, err := svc.IsDemoUser(demoID.Hex()); err != nil || !isDemo {
		t.Fatalf("demo policy = %t err=%v, want configured demo user", isDemo, err)
	}
	if svc.GetSchema() != "https://" || svc.GetDefaultDomain() != "notes.example.test" {
		t.Fatalf("site.url domain = %q%q", svc.GetSchema(), svc.GetDefaultDomain())
	}
}

func TestInitGlobalConfigsDefaultsAdminUsernameAndKeepsEmptySiteURL(t *testing.T) {
	withAppConfigSource(t, mapAppConfig{})
	var lookedUp string
	svc := &ConfigService{
		findAdminUser: func(login string) info.User {
			lookedUp = login
			return info.User{UserId: db.MustObjectIDFromHex("507f1f77bcf86cd799439011")}
		},
		loadGlobalConfigs: func() ([]info.Config, error) { return nil, nil },
	}
	if err := svc.InitGlobalConfigsWithError(); err != nil {
		t.Fatalf("InitGlobalConfigsWithError: %v", err)
	}
	if lookedUp != "admin" || svc.GetAdminUsername() != "admin" {
		t.Fatalf("admin username = %q (lookup %q), want default admin", svc.GetAdminUsername(), lookedUp)
	}
	if svc.GetSiteUrl() != "" || svc.IsOpenRegister() {
		t.Fatalf("site url = %q openRegister=%t, want empty/closed", svc.GetSiteUrl(), svc.IsOpenRegister())
	}
}

func TestInitGlobalConfigsFailsClosedWithoutAppConfigSource(t *testing.T) {
	withAppConfigSource(t, nil)
	called := false
	svc := &ConfigService{
		findAdminUser: func(string) info.User {
			called = true
			return info.User{UserId: db.MustObjectIDFromHex("507f1f77bcf86cd799439011")}
		},
		loadGlobalConfigs: func() ([]info.Config, error) { return nil, nil },
	}
	if err := svc.InitGlobalConfigsWithError(); err == nil {
		t.Fatal("InitGlobalConfigsWithError succeeded without an application config source")
	}
	if called || svc.GlobalSnapshotLoaded() {
		t.Fatalf("admin lookup=%t loaded=%t, want neither", called, svc.GlobalSnapshotLoaded())
	}
}

func TestInitGlobalConfigsFailureKeepsSnapshotUnloaded(t *testing.T) {
	withAppConfigSource(t, mapAppConfig{"adminUsername": "admin"})
	storageErr := errors.New("mongo unavailable")
	for _, test := range []struct {
		name      string
		admin     info.User
		configErr error
	}{
		{name: "missing admin user"},
		{name: "config load failure", admin: info.User{UserId: db.MustObjectIDFromHex("507f1f77bcf86cd799439011")}, configErr: storageErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := &ConfigService{
				findAdminUser:     func(string) info.User { return test.admin },
				loadGlobalConfigs: func() ([]info.Config, error) { return nil, test.configErr },
			}
			err := svc.InitGlobalConfigsWithError()
			if err == nil || svc.GlobalSnapshotLoaded() || svc.GetAdminUserId() != "" {
				t.Fatalf("err=%v loaded=%t admin=%q, want unloaded failure", err, svc.GlobalSnapshotLoaded(), svc.GetAdminUserId())
			}
			if test.configErr != nil && !errors.Is(err, storageErr) {
				t.Fatalf("err = %v, want wrapped storage error", err)
			}
		})
	}
}

func TestInitGlobalConfigsWithoutDatabaseReportsDependencies(t *testing.T) {
	withAppConfigSource(t, mapAppConfig{})
	savedConfigs := db.Configs
	db.Configs = nil
	t.Cleanup(func() { db.Configs = savedConfigs })
	svc := &ConfigService{}
	if err := svc.InitGlobalConfigsWithError(); !errors.Is(err, errConfigDependencies) {
		t.Fatalf("err = %v, want dependency error", err)
	}
	var nilService *ConfigService
	if nilService.GlobalSnapshotLoaded() {
		t.Fatal("nil ConfigService reported a loaded snapshot")
	}
}

func TestSetAppConfigSourceDerivesSiteDomainIdempotently(t *testing.T) {
	withAppConfigSource(t, mapAppConfig{"site.url": "http://localhost:9000"})
	SetAppConfigSource(mapAppConfig{"site.url": "http://localhost:9000"})
	svc := &ConfigService{}
	if got := svc.GetUserUrl("blog.example.test"); got != "http://blog.example.test:9000" {
		t.Fatalf("user url = %q, want port kept once", got)
	}
	if got := svc.GetUserSubUrl("alice"); got != "http://alice.localhost:9000" {
		t.Fatalf("user sub url = %q", got)
	}
	SetAppConfigSource(mapAppConfig{"site.url": "https://notes.example.test:80"})
	if svc.GetSchema() != "https://" || svc.GetDefaultDomain() != "notes.example.test:80" || svc.GetUserUrl("x") != "https://x" {
		t.Fatalf("https/80 derivation = %q %q %q", svc.GetSchema(), svc.GetDefaultDomain(), svc.GetUserUrl("x"))
	}
}
