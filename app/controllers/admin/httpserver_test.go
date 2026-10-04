package admin

import (
	"net/http/httptest"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/service"
)

func TestRegisterHTTPContainsAdminInventory(t *testing.T) {
	r := httpserver.NewRegistry()
	RegisterHTTP(r)
	for _, name := range []string{
		"Admin.Index", "Admin.T", "Admin.GetView", "AdminBlog.Index", "AdminBlog.SetRecommend",
		"AdminData.Backup", "AdminData.Download", "AdminEmail.SendToUsers", "AdminSetting.ExportPdf",
		"AdminUpgrade.UpgradeBlog", "AdminUser.DoResetPwd",
	} {
		parts := splitAction(name)
		if _, ok := r.Lookup(parts[0], parts[1]); !ok {
			t.Fatalf("missing registered action %s", name)
		}
	}
}

func TestRequireAdminHTTPFailsClosed(t *testing.T) {
	c := &httpserver.Context{Request: httptest.NewRequest("GET", "/admin/index", nil)}
	result := requireAdminHTTP(c)
	if result == nil {
		t.Fatal("anonymous principal unexpectedly authorized")
	}
}

func TestRegisterHTTPUsesInjectedSessionBefore(t *testing.T) {
	r := httpserver.NewRegistry()
	called := false
	sessionBefore := func(*httpserver.Context) httpserver.Result {
		called = true
		return nil
	}
	RegisterHTTP(r, HTTPDeps{SessionBefore: sessionBefore})
	entry, ok := r.Lookup("Admin", "Index")
	if !ok {
		t.Fatal("Admin.Index was not registered")
	}
	if len(entry.Befores) != 2 {
		t.Fatalf("Admin.Index before count = %d, want session and admin hooks", len(entry.Befores))
	}
	entry.Befores[0](&httpserver.Context{})
	if !called {
		t.Fatal("injected session before was not attached to admin actions")
	}
}

func TestAdminViewArgsExposeConfigurationProjections(t *testing.T) {
	previous := configService
	configService = &service.ConfigService{
		GlobalStringConfigs: map[string]string{"siteUrl": "https://example.test", "demoPassword": "secret"},
		GlobalArrayConfigs:  map[string][]string{"recommendTags": {"go"}},
		GlobalMapConfigs:    map[string]map[string]string{"oldEmails": {"subject": "body"}},
		GlobalArrMapConfigs: map[string][]map[string]string{"backups": {{"path": "2026-09-27"}}},
	}
	t.Cleanup(func() { configService = previous })

	args := (&server{}).viewArgs(&httpserver.Context{Locale: "zh-cn", ViewArgs: map[string]interface{}{"existing": true}})
	for _, key := range []string{"str", "arr", "map", "arrMap", "version", "locale", "currentLocale", "existing"} {
		if _, ok := args[key]; !ok {
			t.Errorf("view args missing %q: %#v", key, args)
		}
	}
	if got := args["str"].(map[string]string)["demoPassword"]; got != service.RedactedSecretValue {
		t.Fatalf("secret config = %q, want redacted value", got)
	}
}

func splitAction(name string) [2]string {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return [2]string{name[:i], name[i+1:]}
		}
	}
	return [2]string{name, ""}
}
