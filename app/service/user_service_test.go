package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestUserServiceUpdateEmailDoesNotConsumeWhenEmailWriteFails(t *testing.T) {
	consumed := false
	service := UserService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Email: "next@example.test", Token: "token", Type: info.TokenUpdateEmail}, nil
		},
		emailExists: func(string) bool { return false },
		runActionTokenMutation: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); err == nil {
				t.Fatal("email update must fail in this fixture")
			}
			return errors.New("email write failed")
		},
		updateUserFields: func(context.Context, domain.ObjectID, bson.M) error {
			return errors.New("write failed")
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			consumed = true
			return nil
		},
	}

	ok, msg, email := service.UpdateEmail("token")
	if ok || msg != "partial_write" || email != "next@example.test" || consumed {
		t.Fatalf("UpdateEmail = ok:%v msg:%q email:%q consumed:%v, want partial_write without consume", ok, msg, email, consumed)
	}
}

func TestUserServiceUpdateEmailDoesNotConsumeWhenUserIsMissing(t *testing.T) {
	consumed := false
	service := UserService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Email: "next@example.test", Token: "token", Type: info.TokenUpdateEmail}, nil
		},
		emailExists: func(string) bool { return false },
		runActionTokenMutation: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); !errors.Is(err, db.ErrDocumentNotFound) {
				t.Fatalf("update err=%v, want ErrDocumentNotFound", err)
			}
			return db.ErrDocumentNotFound
		},
		updateUserFields: func(context.Context, domain.ObjectID, bson.M) error {
			return db.ErrDocumentNotFound
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			consumed = true
			return nil
		},
	}

	ok, msg, email := service.UpdateEmail("token")
	if ok || msg != "partial_write" || email != "next@example.test" || consumed {
		t.Fatalf("UpdateEmail missing user = ok:%v msg:%q email:%q consumed:%v, want partial_write without consume", ok, msg, email, consumed)
	}
}

func TestUserServiceActiveEmailDoesNotReportSuccessWhenConsumeFails(t *testing.T) {
	updated := false
	service := UserService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Email: "user@example.test", Token: "token", Type: info.TokenActiveEmail}, nil
		},
		runActionTokenMutation: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); err != nil {
				t.Fatalf("update: %v", err)
			}
			return consume(context.Background())
		},
		updateUserFields: func(context.Context, domain.ObjectID, bson.M) error {
			updated = true
			return nil
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			return errors.New("consume failed")
		},
	}

	ok, msg, email := service.ActiveEmail("token")
	if !updated || ok || msg != "partial_write" || email != "user@example.test" {
		t.Fatalf("ActiveEmail = updated:%v ok:%v msg:%q email:%q, want partial_write after consume failure", updated, ok, msg, email)
	}
}

func TestUserServiceActiveEmailDoesNotConsumeWhenUserIsMissing(t *testing.T) {
	consumed := false
	service := UserService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Email: "user@example.test", Token: "token", Type: info.TokenActiveEmail}, nil
		},
		runActionTokenMutation: func(_ context.Context, update func(context.Context) error, consume func(context.Context) error) error {
			if err := update(context.Background()); !errors.Is(err, db.ErrDocumentNotFound) {
				t.Fatalf("update err=%v, want ErrDocumentNotFound", err)
			}
			return db.ErrDocumentNotFound
		},
		updateUserFields: func(context.Context, domain.ObjectID, bson.M) error {
			return db.ErrDocumentNotFound
		},
		consumeToken: func(context.Context, string, int, time.Time, time.Duration) error {
			consumed = true
			return nil
		},
	}

	ok, msg, email := service.ActiveEmail("token")
	if ok || msg != "partial_write" || email != "user@example.test" || consumed {
		t.Fatalf("ActiveEmail missing user = ok:%v msg:%q email:%q consumed:%v, want partial_write without consume", ok, msg, email, consumed)
	}
}

func TestUserServiceUpdateEmailRejectsDuplicateBeforeMutation(t *testing.T) {
	mutated := false
	service := UserService{
		resolveToken: func(string, int) (info.Token, error) {
			return info.Token{UserId: domain.ObjectID{1}, Email: "Used@Example.Test", Token: "token", Type: info.TokenUpdateEmail}, nil
		},
		emailExists: func(email string) bool {
			if email != "used@example.test" {
				t.Fatalf("duplicate lookup email=%q, want normalized lowercase", email)
			}
			return true
		},
		runActionTokenMutation: func(context.Context, func(context.Context) error, func(context.Context) error) error {
			mutated = true
			return nil
		},
	}

	ok, msg, email := service.UpdateEmail("token")
	if ok || msg != "该邮箱已注册" || email != "used@example.test" || mutated {
		t.Fatalf("UpdateEmail duplicate = ok:%v msg:%q email:%q mutated:%v", ok, msg, email, mutated)
	}
}

func TestUserServiceRejectsInvalidObjectIDsForProfileMutations(t *testing.T) {
	service := UserService{}
	if ok, _ := service.UpdateUsername("not-object-id", "next"); ok {
		t.Fatal("UpdateUsername accepted an invalid user id")
	}
	if ok := service.UpdateAvatar("not-object-id", "/avatar.png"); ok {
		t.Fatal("UpdateAvatar accepted an invalid user id")
	}
	if ok, _ := service.UpdatePwd("not-object-id", "old-password", "new-password"); ok {
		t.Fatal("UpdatePwd accepted an invalid user id")
	}
}

func TestEmailTokenTimeoutsUsePurposeSpecificDurations(t *testing.T) {
	saved := tokenService
	tokenService = &TokenService{}
	defer func() { tokenService = saved }()

	if got := tokenTimeoutHours(info.TokenPwd); got != "2" {
		t.Fatalf("password token timeout=%q, want 2", got)
	}
	if got := tokenTimeoutHours(info.TokenUpdateEmail); got != "2" {
		t.Fatalf("update-email token timeout=%q, want 2", got)
	}
	if got := tokenTimeoutHours(info.TokenActiveEmail); got != "48" {
		t.Fatalf("active-email token timeout=%q, want 48", got)
	}
}

func TestUserServiceFindUserIDByEmailSurfacesStorageError(t *testing.T) {
	saved := db.Users
	db.Users = nil
	defer func() { db.Users = saved }()

	service := UserService{}
	_, err := service.FindUserIDByEmail("user@example.test")
	if !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindUserIDByEmail error=%v, want initialized-storage error", err)
	}
}

func TestUserServiceFindUserInfoByNameSurfacesStorageError(t *testing.T) {
	saved := db.Users
	db.Users = nil
	defer func() { db.Users = saved }()

	service := UserService{}
	_, err := service.FindUserInfoByName("user@example.test")
	if !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindUserInfoByName error=%v, want initialized-storage error", err)
	}
}

func TestUserServiceFindUserInfoByThirdIdentitySurfacesStorageError(t *testing.T) {
	saved := db.Users
	db.Users = nil
	defer func() { db.Users = saved }()

	service := UserService{}
	_, err := service.FindUserInfoByThirdIdentity(info.ThirdGithub, "third-id")
	if !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("FindUserInfoByThirdIdentity error=%v, want initialized-storage error", err)
	}
}
