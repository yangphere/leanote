package api

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/service"
)

// markGlobalConfigLoaded treats the test's in-memory ConfigService as a
// published snapshot; production publishes it via InitGlobalConfigsWithError.
func markGlobalConfigLoaded(t *testing.T) {
	t.Helper()
	saved := globalConfigLoaded
	globalConfigLoaded = func() bool { return true }
	t.Cleanup(func() { globalConfigLoaded = saved })
}

func TestPrincipalPolicyFromConfigFailsClosedWhenSnapshotNotLoaded(t *testing.T) {
	saved := configService
	// Same empty demo/admin configuration as the "unconfigured" case below,
	// but never published: it must not read as "demo not configured".
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{}}
	defer func() { configService = saved }()

	role, isDemo, err := PrincipalPolicyFromConfig()("507f1f77bcf86cd799439012")
	if !errors.Is(err, service.ErrGlobalConfigNotLoaded) {
		t.Fatalf("unloaded snapshot error = %v, want ErrGlobalConfigNotLoaded", err)
	}
	if errors.Is(err, service.ErrDemoConfiguration) || role != "" || isDemo {
		t.Fatalf("unloaded snapshot = role %q demo=%t err=%v, want fail closed", role, isDemo, err)
	}

	principal, err := httpserver.AuthenticatedPrincipalWithPolicy("507f1f77bcf86cd799439012", httpserver.PrincipalSourceAPIToken, httpserver.TokenStateValid, PrincipalPolicyFromConfig())
	if err == nil || principal.UserID != "" || principal.Role != httpserver.PrincipalRoleAnonymous {
		t.Fatalf("principal with unloaded snapshot = %+v err=%v, want rejection", principal, err)
	}
}

func TestPrincipalPolicyFromConfigLeavesUnconfiguredDemoAnonymous(t *testing.T) {
	markGlobalConfigLoaded(t)
	saved := configService
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{}}
	defer func() { configService = saved }()

	role, isDemo, err := PrincipalPolicyFromConfig()("507f1f77bcf86cd799439012")
	if err != nil || role != httpserver.PrincipalRoleMember || isDemo {
		t.Fatalf("unconfigured principal = role %q demo=%t err=%v", role, isDemo, err)
	}
}

func TestPrincipalPolicyFromConfigFailsClosedOnInvalidDemoConfiguration(t *testing.T) {
	markGlobalConfigLoaded(t)
	saved := configService
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{
		"demoUserId":   "not-an-object-id",
		"demoUsername": "demo@example.test",
	}}
	defer func() { configService = saved }()

	_, _, err := PrincipalPolicyFromConfig()("507f1f77bcf86cd799439012")
	if !errors.Is(err, service.ErrDemoConfiguration) {
		t.Fatalf("invalid demo configuration error = %v", err)
	}
}
