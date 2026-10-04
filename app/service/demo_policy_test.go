package service

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

func TestDemoPolicyRejectsMissingInvalidAndMismatchedConfiguration(t *testing.T) {
	configuredID := "507f1f77bcf86cd799439011"
	otherID := db.MustObjectIDFromHex("507f1f77bcf86cd799439012")
	cases := []struct {
		name     string
		configs  map[string]string
		resolved info.User
	}{
		{name: "missing id", configs: map[string]string{"demoUsername": "demo@example.test"}},
		{name: "invalid id", configs: map[string]string{"demoUserId": "invalid", "demoUsername": "demo@example.test"}},
		{name: "missing username", configs: map[string]string{"demoUserId": configuredID}},
		{name: "username missing", configs: map[string]string{"demoUserId": configuredID, "demoUsername": "missing@example.test"}},
		{name: "username mismatch", configs: map[string]string{"demoUserId": configuredID, "demoUsername": "demo@example.test"}, resolved: info.User{UserId: otherID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := ConfigService{
				GlobalStringConfigs: tc.configs,
				findDemoUser: func(string) (info.User, error) {
					return tc.resolved, nil
				},
			}
			if _, err := service.DemoAccount(); !errors.Is(err, ErrDemoConfiguration) {
				t.Fatalf("DemoAccount error = %v, want ErrDemoConfiguration", err)
			}
		})
	}
}

func TestDemoPolicyMatchesOnlyValidatedConfiguredIdentity(t *testing.T) {
	id := db.MustObjectIDFromHex("507f1f77bcf86cd799439011")
	service := ConfigService{
		GlobalStringConfigs: map[string]string{
			"demoUserId":   id.Hex(),
			"demoUsername": " Demo@Example.Test ",
		},
		findDemoUser: func(login string) (info.User, error) {
			if login != "demo@example.test" {
				t.Fatalf("lookup login = %q, want normalized demo login", login)
			}
			return info.User{UserId: id}, nil
		},
	}

	account, err := service.DemoAccount()
	if err != nil {
		t.Fatalf("DemoAccount: %v", err)
	}
	if account.UserID != id || account.Login != "demo@example.test" {
		t.Fatalf("DemoAccount = %+v", account)
	}
	if matched, err := service.IsDemoUser(id.Hex()); err != nil || !matched {
		t.Fatalf("IsDemoUser configured identity = %v, %v", matched, err)
	}
	if matched, err := service.IsDemoUser("507f1f77bcf86cd799439012"); err != nil || matched {
		t.Fatalf("IsDemoUser other identity = %v, %v", matched, err)
	}
}

func TestDemoPolicyPreservesLookupStorageError(t *testing.T) {
	storageErr := errors.New("database unavailable")
	service := ConfigService{
		GlobalStringConfigs: map[string]string{
			"demoUserId":   "507f1f77bcf86cd799439011",
			"demoUsername": "demo@example.test",
		},
		findDemoUser: func(string) (info.User, error) {
			return info.User{}, storageErr
		},
	}

	_, err := service.DemoAccount()
	if !errors.Is(err, storageErr) || errors.Is(err, ErrDemoConfiguration) {
		t.Fatalf("DemoAccount error = %v, want storage cause without configuration category", err)
	}
}
