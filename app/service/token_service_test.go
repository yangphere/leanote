package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestTokenServiceResolveUsesExpectedPurposeAndInjectedClock(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	want := db.ActionToken{
		ID:          domain.ObjectID{1},
		UserID:      domain.ObjectID{2},
		Email:       "user@example.test",
		Token:       "value",
		Type:        info.TokenPwd,
		CreatedTime: now.Add(-time.Hour),
	}
	var gotType int
	service := TokenService{
		now: func() time.Time { return now },
		resolveAction: func(_ context.Context, value string, tokenType int, gotNow time.Time, ttl time.Duration) (db.ActionToken, error) {
			if value != want.Token || !gotNow.Equal(now) || ttl != 2*time.Hour {
				t.Fatalf("resolve args value=%q now=%s ttl=%s", value, gotNow, ttl)
			}
			gotType = tokenType
			return want, nil
		},
	}

	token, err := service.Resolve(want.Token, info.TokenPwd)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if gotType != info.TokenPwd || token.UserId != want.UserID || token.Type != info.TokenPwd {
		t.Fatalf("resolved token=%+v type=%d", token, gotType)
	}
}

func TestTokenServiceResolvePreservesPurposeAndExpiryCategories(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for name, wantErr := range map[string]error{
		"type mismatch": db.ErrTokenTypeMismatch,
		"expired":       db.ErrTokenExpired,
		"not found":     ErrTokenNotFound,
		"storage":       errors.New("storage unavailable"),
	} {
		t.Run(name, func(t *testing.T) {
			service := TokenService{
				now: func() time.Time { return now },
				resolveAction: func(context.Context, string, int, time.Time, time.Duration) (db.ActionToken, error) {
					return db.ActionToken{}, wantErr
				},
			}
			_, err := service.Resolve("value", info.TokenPwd)
			if name == "storage" {
				if err == nil || errors.Is(err, db.ErrTokenExpired) || errors.Is(err, db.ErrTokenTypeMismatch) || errors.Is(err, ErrTokenNotFound) {
					t.Fatalf("storage error category lost: %v", err)
				}
				return
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("Resolve error=%v, want %v", err, wantErr)
			}
		})
	}
}

func TestTokenServiceConsumePassesPurposeTTLToAtomicConsumer(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	called := false
	service := TokenService{
		now: func() time.Time { return now },
		consumeAction: func(_ context.Context, value string, tokenType int, consumedAt time.Time, ttl time.Duration) (db.ActionToken, error) {
			called = true
			if value != "value" || tokenType != info.TokenPwd || !consumedAt.Equal(now) || ttl != 2*time.Hour {
				t.Fatalf("consume args value=%q type=%d now=%s ttl=%s", value, tokenType, consumedAt, ttl)
			}
			return db.ActionToken{Token: value, Type: tokenType, CreatedTime: now.Add(-time.Hour)}, nil
		},
	}

	if _, err := service.Consume("value", info.TokenPwd); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if !called {
		t.Fatal("atomic consumer was not called")
	}
}
