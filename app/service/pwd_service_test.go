package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestPwdServiceUpdatePwdDoesNotConsumeWhenPasswordWriteFails(t *testing.T) {
	service := PwdService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Token: "token", Type: info.TokenPwd}, nil
		},
		runPasswordReset: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); err == nil {
				t.Fatal("update must fail in this fixture")
			}
			return errors.New("password write failed")
		},
		updatePassword: func(context.Context, domain.ObjectID, string) error {
			return errors.New("write failed")
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			t.Fatal("consume must not run after a password write failure")
			return nil
		},
	}

	ok, msg := service.UpdatePwd("token", "new-password")
	if ok || msg != "partial_write" {
		t.Fatalf("UpdatePwd = ok:%v msg:%q, want partial_write failure", ok, msg)
	}
}

func TestPwdServiceUpdatePwdDoesNotReportSuccessWhenConsumeFails(t *testing.T) {
	updated := false
	service := PwdService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Token: "token", Type: info.TokenPwd}, nil
		},
		runPasswordReset: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); err != nil {
				t.Fatalf("update: %v", err)
			}
			return consume(context.Background())
		},
		updatePassword: func(context.Context, domain.ObjectID, string) error {
			updated = true
			return nil
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			return errors.New("consume failed")
		},
	}

	ok, msg := service.UpdatePwd("token", "new-password")
	if !updated || ok || msg != "partial_write" {
		t.Fatalf("UpdatePwd = updated:%v ok:%v msg:%q, want non-success partial_write", updated, ok, msg)
	}
}

func TestPwdServiceUpdatePwdDoesNotConsumeWhenUserIsMissing(t *testing.T) {
	consumed := false
	service := PwdService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Token: "token", Type: info.TokenPwd}, nil
		},
		runPasswordReset: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); !errors.Is(err, db.ErrDocumentNotFound) {
				t.Fatalf("update err=%v, want ErrDocumentNotFound", err)
			}
			return db.ErrDocumentNotFound
		},
		updatePassword: func(context.Context, domain.ObjectID, string) error {
			return db.ErrDocumentNotFound
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			consumed = true
			return nil
		},
	}

	ok, msg := service.UpdatePwd("token", "new-password")
	if ok || msg != "partial_write" || consumed {
		t.Fatalf("UpdatePwd missing user = ok:%v msg:%q consumed:%v, want partial_write without consume", ok, msg, consumed)
	}
}

func TestPwdServiceFindPwdDoesNotRevealMissingAccount(t *testing.T) {
	issued := false
	service := PwdService{
		lookupUserID: func(email string) (string, error) {
			if email != "missing@example.test" {
				t.Fatalf("lookup email=%q, want normalized lowercase", email)
			}
			return "", nil
		},
		issuePasswordToken: func(context.Context, string, string) (info.Token, error) {
			issued = true
			return info.Token{}, nil
		},
		enqueueOutbox: func(context.Context, db.OutboxEvent) error {
			t.Fatal("missing accounts must not enqueue reset-password mail")
			return nil
		},
	}

	ok, msg := service.FindPwd("Missing@Example.Test")
	if !ok || msg != "" || issued {
		t.Fatalf("FindPwd missing account = ok:%v msg:%q issued:%v, want public success without token issue", ok, msg, issued)
	}
}

func TestPwdServiceFindPwdEnqueuesResetPasswordOutbox(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	userID := domain.ObjectID{2}
	token := info.Token{UserId: userID, Email: "user@example.test", Token: "reset-token", Type: info.TokenPwd, CreatedTime: now}
	enqueued := false
	service := PwdService{
		now: func() time.Time { return now },
		lookupUserID: func(email string) (string, error) {
			if email != "user@example.test" {
				t.Fatalf("lookup email=%q, want normalized lowercase", email)
			}
			return userID.Hex(), nil
		},
		issuePasswordToken: func(_ context.Context, gotUserID, email string) (info.Token, error) {
			if gotUserID != userID.Hex() || email != "user@example.test" {
				t.Fatalf("issue token user=%q email=%q", gotUserID, email)
			}
			return token, nil
		},
		enqueueOutbox: func(_ context.Context, event db.OutboxEvent) error {
			enqueued = true
			if event.Kind != "reset-password" || event.AggregateID != userID || !strings.HasPrefix(event.IdempotencyKey, "reset-password:"+userID.Hex()+":") {
				t.Fatalf("outbox event identity = %+v", event)
			}
			if strings.Contains(event.IdempotencyKey, "reset-token") {
				t.Fatalf("outbox idempotency key leaked token: %q", event.IdempotencyKey)
			}
			if event.Payload["email"] != "user@example.test" || event.Payload["token"] != "reset-token" || event.Payload["tokenType"] != info.TokenPwd {
				t.Fatalf("outbox payload=%#v", event.Payload)
			}
			return nil
		},
	}

	ok, msg := service.FindPwd("User@Example.Test")
	if !ok || msg != "" || !enqueued {
		t.Fatalf("FindPwd existing account = ok:%v msg:%q enqueued:%v, want public success after outbox enqueue", ok, msg, enqueued)
	}
}

func TestPwdServiceFindPwdSurfacesOutboxFailureWithoutLeakingAccountExistence(t *testing.T) {
	userID := domain.ObjectID{3}
	service := PwdService{
		lookupUserID: func(string) (string, error) {
			return userID.Hex(), nil
		},
		issuePasswordToken: func(context.Context, string, string) (info.Token, error) {
			return info.Token{UserId: userID, Email: "user@example.test", Token: "reset-token", Type: info.TokenPwd}, nil
		},
		enqueueOutbox: func(context.Context, db.OutboxEvent) error {
			return errors.New("outbox unavailable")
		},
	}

	ok, msg := service.FindPwd("user@example.test")
	if ok || msg != "side_effect" {
		t.Fatalf("FindPwd outbox failure = ok:%v msg:%q, want side_effect failure", ok, msg)
	}
}

func TestPwdServiceFindPwdSurfacesDefaultLookupStorageError(t *testing.T) {
	saved := userService
	storageErr := errors.New("database unavailable")
	userService = &UserService{
		findUserIDByEmail: func(email string) (string, error) {
			if email != "user@example.test" {
				t.Fatalf("lookup email=%q, want normalized lowercase", email)
			}
			return "", storageErr
		},
	}
	defer func() { userService = saved }()

	service := PwdService{}
	ok, msg := service.FindPwd(" User@Example.Test ")
	if ok || msg != "storage" {
		t.Fatalf("FindPwd storage lookup = ok:%v msg:%q, want storage failure", ok, msg)
	}
}
