package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPersistShareGrantWithProjectionOrdersWrites(t *testing.T) {
	grantErr := errors.New("grant failed")
	var calls []string
	err := persistShareGrantWithProjection(
		func() error {
			calls = append(calls, "projection")
			return nil
		},
		func() error {
			calls = append(calls, "grant")
			return grantErr
		},
	)
	if !errors.Is(err, grantErr) {
		t.Fatalf("error=%v, want grant error", err)
	}
	if got := strings.Join(calls, ","); got != "projection,grant" {
		t.Fatalf("call order=%q", got)
	}
}

func TestPersistShareGrantWithProjectionSkipsGrantOnProjectionFailure(t *testing.T) {
	projectionErr := errors.New("projection failed")
	grantCalled := false
	err := persistShareGrantWithProjection(
		func() error { return projectionErr },
		func() error {
			grantCalled = true
			return nil
		},
	)
	if !errors.Is(err, projectionErr) || grantCalled {
		t.Fatalf("error=%v grantCalled=%t, want projection error without grant", err, grantCalled)
	}
}

func TestDeleteShareRecordsPropagatesAnyCleanupFailure(t *testing.T) {
	cleanupErr := errors.New("share note delete failed")
	var calls []string
	err := deleteShareRecords(context.Background(), nil, []shareDeletionStep{
		{name: "notebooks", remove: func(context.Context, interface{}) (int, error) {
			calls = append(calls, "notebooks")
			return 1, nil
		}},
		{name: "notes", remove: func(context.Context, interface{}) (int, error) {
			calls = append(calls, "notes")
			return 0, cleanupErr
		}},
		{name: "projection", remove: func(context.Context, interface{}) (int, error) {
			calls = append(calls, "projection")
			return 1, nil
		}},
	})
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("deleteShareRecords error=%v, want cleanup failure", err)
	}
	if got := strings.Join(calls, ","); got != "notebooks,notes,projection" {
		t.Fatalf("cleanup call order=%q", got)
	}
}

func TestGetShareNotebooksCheckedRejectsInvalidRecipient(t *testing.T) {
	_, _, err := (&ShareService{}).GetShareNotebooksChecked("not-an-object-id")
	if !errors.Is(err, ErrShareRecipient) {
		t.Fatalf("GetShareNotebooksChecked error=%v, want ErrShareRecipient", err)
	}
}

func TestParseShareExpiryRequiresExplicitWholeSecondTimezone(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		value   string
		present bool
		wantErr bool
	}{
		{name: "missing", present: false},
		{name: "utc", value: "2026-09-24T13:00:00Z", present: true},
		{name: "offset", value: "2026-09-24T21:00:00+08:00", present: true},
		{name: "empty", present: true, wantErr: true},
		{name: "whitespace", value: " 2026-09-24T13:00:00Z", present: true, wantErr: true},
		{name: "fraction", value: "2026-09-24T13:00:00.001Z", present: true, wantErr: true},
		{name: "no timezone", value: "2026-09-24T13:00:00", present: true, wantErr: true},
		{name: "past", value: "2026-09-24T12:00:00Z", present: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseShareExpiry(test.value, test.present, now)
			if test.wantErr != (err != nil) {
				t.Fatalf("ParseShareExpiry(%q, %t) error=%v, wantErr=%t", test.value, test.present, err, test.wantErr)
			}
			if !test.wantErr && test.present && got == nil {
				t.Fatal("valid present expiry returned nil")
			}
			if !test.wantErr && !test.present && got != nil {
				t.Fatalf("missing expiry returned %v", got)
			}
		})
	}
}

func TestResolveShareGrantPermissionUsesExpiryAndPrecedence(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	active := now.Add(time.Hour)
	expired := now.Add(-time.Second)
	tests := []struct {
		name             string
		direct           []shareGrant
		groups           []shareGrant
		wantPerm         int
		wantAllowed      bool
		wantErrDuplicate bool
	}{
		{name: "expired direct falls back to group", direct: []shareGrant{{Perm: 1, ExpiresAt: expired}}, groups: []shareGrant{{Perm: 0, ExpiresAt: active}}, wantAllowed: true},
		{name: "direct read overrides group write", direct: []shareGrant{{Perm: 0, ExpiresAt: active}}, groups: []shareGrant{{Perm: 1, ExpiresAt: active}}, wantAllowed: true},
		{name: "groups combine to write", groups: []shareGrant{{Perm: 0, ExpiresAt: active}, {Perm: 1, ExpiresAt: active}}, wantPerm: 1, wantAllowed: true},
		{name: "expired groups deny", groups: []shareGrant{{Perm: 1, ExpiresAt: now}, {Perm: 0, ExpiresAt: expired}}, wantAllowed: false},
		{name: "duplicate active direct fails closed", direct: []shareGrant{{Perm: 0, ExpiresAt: active}, {Perm: 1, ExpiresAt: active}}, wantErrDuplicate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			perm, allowed, err := resolveShareGrantPermission(test.direct, test.groups, now)
			if test.wantErrDuplicate {
				if err == nil || !errors.Is(err, ErrInvalidShareGrant) {
					t.Fatalf("error=%v, want invalid duplicate error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveShareGrantPermission: %v", err)
			}
			if perm != test.wantPerm || allowed != test.wantAllowed {
				t.Fatalf("permission=(%d,%t), want=(%d,%t)", perm, allowed, test.wantPerm, test.wantAllowed)
			}
		})
	}
}

func TestValidateShareGrantOptionsDoesNotClearExistingExpiryImplicitly(t *testing.T) {
	if err := validateShareGrantOptions(ShareGrantOptions{}, true); err != nil {
		t.Fatalf("legacy update without expiry: %v", err)
	}
	if err := validateShareGrantOptions(ShareGrantOptions{ClearExpiresAt: true}, false); !errors.Is(err, ErrInvalidShareGrant) {
		t.Fatalf("new grant clear expiry error=%v", err)
	}
	expiresAt := time.Now().UTC()
	if err := validateShareGrantOptions(ShareGrantOptions{ExpiresAt: &expiresAt, ClearExpiresAt: true}, true); !errors.Is(err, ErrInvalidShareGrant) {
		t.Fatalf("conflicting expiry options error=%v", err)
	}
}
