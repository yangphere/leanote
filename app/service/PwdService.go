package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type PwdService struct {
	now                func() time.Time
	resolveToken       func(string, int) (info.Token, error)
	runPasswordReset   func(context.Context, func(context.Context) error, func(context.Context) error) error
	updatePassword     func(context.Context, domain.ObjectID, string) error
	consumeToken       func(context.Context, string, int, time.Time, time.Duration) error
	lookupUserID       func(string) (string, error)
	issuePasswordToken func(context.Context, string, string) (info.Token, error)
	enqueueOutbox      func(context.Context, db.OutboxEvent) error
}

func (s *PwdService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *PwdService) resolve(value string, tokenType int) (info.Token, error) {
	if s.resolveToken != nil {
		return s.resolveToken(value, tokenType)
	}
	if tokenService == nil {
		return info.Token{}, errors.New("token service is not initialized")
	}
	return tokenService.Resolve(value, tokenType)
}

func (s *PwdService) resetRunner() func(context.Context, func(context.Context) error, func(context.Context) error) error {
	if s.runPasswordReset != nil {
		return s.runPasswordReset
	}
	return db.RunPasswordTokenMutation
}

func (s *PwdService) update(ctx context.Context, userID domain.ObjectID, password string) error {
	if s.updatePassword != nil {
		return s.updatePassword(ctx, userID, password)
	}
	if db.Users == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.UpdateOneMatchedContext(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"Pwd": password}})
}

func (s *PwdService) consume(ctx context.Context, value string, tokenType int, now time.Time) error {
	if s.consumeToken != nil {
		return s.consumeToken(ctx, value, tokenType, now, tokenTTL(tokenType))
	}
	_, err := db.ConsumeValidActionToken(ctx, value, tokenType, now, tokenTTL(tokenType))
	return err
}

func (s *PwdService) lookup(email string) (string, error) {
	if s.lookupUserID != nil {
		return s.lookupUserID(email)
	}
	if userService == nil {
		return "", errors.New("user service is not initialized")
	}
	return userService.FindUserIDByEmail(email)
}

func (s *PwdService) issueResetToken(ctx context.Context, userID, email string) (info.Token, error) {
	if s.issuePasswordToken != nil {
		return s.issuePasswordToken(ctx, userID, email)
	}
	if tokenService == nil {
		return info.Token{}, errors.New("token service is not initialized")
	}
	return tokenService.Issue(userID, email, info.TokenPwd)
}

func (s *PwdService) enqueueResetPassword(ctx context.Context, token info.Token) error {
	event := db.OutboxEvent{
		IdempotencyKey: fmt.Sprintf("reset-password:%s:%d", token.UserId.Hex(), token.CreatedTime.UnixNano()),
		Kind:           "reset-password",
		AggregateID:    token.UserId,
		Payload: map[string]any{
			"userId":    token.UserId.Hex(),
			"email":     token.Email,
			"token":     token.Token,
			"tokenType": info.TokenPwd,
		},
	}
	if s.enqueueOutbox != nil {
		return s.enqueueOutbox(ctx, event)
	}
	return db.EnqueueOutbox(ctx, event)
}

// FindPwd issues a password-purpose action token and enqueues the reset mail
// side effect. Missing users return the same public success as accepted
// requests, while storage/outbox failures remain observable retry failures.
func (s *PwdService) FindPwd(email string) (ok bool, msg string) {
	email = strings.ToLower(strings.TrimSpace(email))
	userID, err := s.lookup(email)
	if err != nil {
		return false, "storage"
	}
	if userID == "" {
		return true, ""
	}
	token, err := s.issueResetToken(context.Background(), userID, email)
	if err != nil {
		return false, "storage"
	}
	if err := s.enqueueResetPassword(context.Background(), token); err != nil {
		return false, "side_effect"
	}
	return true, ""
}

// UpdatePwd changes a password and consumes its token inside one Mongo
// transaction. If the persistence layer cannot provide that boundary, neither
// mutation is attempted and the caller receives a retryable partial_write.
func (s *PwdService) UpdatePwd(tokenValue, pwd string) (bool, string) {
	token, err := s.resolve(tokenValue, info.TokenPwd)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrTokenExpired):
			return false, "过期"
		case errors.Is(err, db.ErrTokenTypeMismatch):
			return false, "类型错误"
		case errors.Is(err, ErrTokenNotFound):
			return false, "不存在"
		default:
			return false, "storage"
		}
	}
	password := GenPwd(pwd)
	if password == "" {
		return false, "GenerateHash error"
	}
	now := s.clock()
	err = s.resetRunner()(context.Background(), func(ctx context.Context) error {
		return s.update(ctx, token.UserId, password)
	}, func(ctx context.Context) error {
		return s.consume(ctx, tokenValue, info.TokenPwd, now)
	})
	if err != nil {
		return false, "partial_write"
	}
	return true, ""
}
