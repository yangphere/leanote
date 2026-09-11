package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var ErrTokenNotFound = errors.New("token not found")

type TokenService struct {
	now           func() time.Time
	issueAction   func(context.Context, domain.ObjectID, string, string, int, time.Time) (db.ActionToken, error)
	resolveAction func(context.Context, string, int, time.Time, time.Duration) (db.ActionToken, error)
	consumeAction func(context.Context, string, int, time.Time, time.Duration) (db.ActionToken, error)
}

func (s *TokenService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *TokenService) issuer() func(context.Context, domain.ObjectID, string, string, int, time.Time) (db.ActionToken, error) {
	if s.issueAction != nil {
		return s.issueAction
	}
	return db.IssueActionToken
}

func (s *TokenService) resolver() func(context.Context, string, int, time.Time, time.Duration) (db.ActionToken, error) {
	if s.resolveAction != nil {
		return s.resolveAction
	}
	return db.ResolveActionToken
}

func (s *TokenService) consumer() func(context.Context, string, int, time.Time, time.Duration) (db.ActionToken, error) {
	if s.consumeAction != nil {
		return s.consumeAction
	}
	return db.ConsumeValidActionToken
}

func tokenTTL(tokenType int) time.Duration {
	switch tokenType {
	case info.TokenPwd:
		return time.Duration(info.PwdOverHours * float64(time.Hour))
	case info.TokenUpdateEmail:
		return time.Duration(info.UpdateEmailOverHours * float64(time.Hour))
	default:
		return time.Duration(info.ActiveEmailOverHours * float64(time.Hour))
	}
}

func toInfoToken(token db.ActionToken) info.Token {
	return info.Token{UserId: token.UserID, Email: token.Email, Token: token.Token, Type: token.Type, CreatedTime: token.CreatedTime}
}

// Issue replaces the active action token for one user and purpose through the
// persistence-owned unique-token seam.
func (s *TokenService) Issue(userID, email string, tokenType int) (info.Token, error) {
	id, err := domain.ParseObjectID(userID)
	if err != nil || id.IsZero() {
		return info.Token{}, fmt.Errorf("issue token: invalid user ID")
	}
	value := NewGuidWith(email)
	if value == "" {
		return info.Token{}, fmt.Errorf("issue token: generate value")
	}
	token, err := s.issuer()(context.Background(), id, email, value, tokenType, s.clock())
	if err != nil {
		return info.Token{}, fmt.Errorf("issue token: %w", err)
	}
	return toInfoToken(token), nil
}

// NewToken is retained for legacy callers. New code should use Issue so a
// persistence failure cannot be mistaken for an empty/unknown token.
func (s *TokenService) NewToken(userID, email string, tokenType int) string {
	token, err := s.Issue(userID, email, tokenType)
	if err != nil {
		return ""
	}
	return token.Token
}

// DeleteToken remains for compatibility with legacy callers that invalidate
// by owner; password/email flows use Consume instead.
func (s *TokenService) DeleteToken(userID string, tokenType int) bool {
	id, err := domain.ParseObjectID(userID)
	if err != nil || id.IsZero() {
		return false
	}
	return db.Delete(db.Tokens, bson.M{"UserId": id, "Type": tokenType})
}

func (s *TokenService) GetOverHours(tokenType int) float64 {
	return tokenTTL(tokenType).Hours()
}

// Resolve validates value, purpose and inclusive expiry through the db seam.
// It preserves typed failures so adapters can map type mismatch, expiry,
// missing values and storage incidents without broad fallbacks.
func (s *TokenService) Resolve(value string, tokenType int) (info.Token, error) {
	if value == "" {
		return info.Token{}, ErrTokenNotFound
	}
	token, err := s.resolver()(context.Background(), value, tokenType, s.clock(), tokenTTL(tokenType))
	if errors.Is(err, mongo.ErrNoDocuments) {
		return info.Token{}, ErrTokenNotFound
	}
	if err != nil {
		return info.Token{}, fmt.Errorf("resolve token: %w", err)
	}
	return toInfoToken(token), nil
}

// Consume atomically marks a resolved action token used. Callers must only
// invoke it after their transaction boundary has committed the paired write.
func (s *TokenService) Consume(value string, tokenType int) (info.Token, error) {
	token, err := s.consumer()(context.Background(), value, tokenType, s.clock(), tokenTTL(tokenType))
	if errors.Is(err, mongo.ErrNoDocuments) {
		return info.Token{}, ErrTokenNotFound
	}
	if err != nil {
		return info.Token{}, fmt.Errorf("consume token: %w", err)
	}
	return toInfoToken(token), nil
}

// VerifyToken preserves the legacy return shape while delegating all purpose,
// expiry and clock handling to Resolve.
func (s *TokenService) VerifyToken(value string, tokenType int) (bool, string, info.Token) {
	token, err := s.Resolve(value, tokenType)
	switch {
	case err == nil:
		return true, "", token
	case errors.Is(err, db.ErrTokenTypeMismatch):
		return false, "类型错误", info.Token{}
	case errors.Is(err, db.ErrTokenExpired):
		return false, "过期", info.Token{}
	case errors.Is(err, ErrTokenNotFound):
		return false, "不存在", info.Token{}
	default:
		return false, "storage", info.Token{}
	}
}
