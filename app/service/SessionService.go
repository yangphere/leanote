package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	// . "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	//	"strings"
)

// Session存储到mongodb中
type SessionService struct {
}

// IssueUserToken creates a new API credential and stores only its digest.
// It deliberately does not reuse Get: resolving a missing credential must not
// create a session record.
func (s *SessionService) IssueUserToken(userID string) (string, error) {
	token, err := db.NewAPISessionToken()
	if err != nil {
		return "", err
	}
	if err := db.CreateSessionForToken(context.Background(), token, userID, time.Now()); err != nil {
		return "", err
	}
	return token, nil
}

// ResolveUserID resolves an existing API token. Missing and expired records
// remain errors so adapters cannot silently fall back or recreate them.
func (s *SessionService) ResolveUserID(token string) (string, error) {
	session, err := db.ResolveSessionAndRefresh(context.Background(), token, time.Now())
	if err != nil {
		return "", err
	}
	if session.UserId == "" {
		return "", fmt.Errorf("resolve session user: %w", db.ErrSessionNotFound)
	}
	exists, err := db.SessionUserExists(context.Background(), session.UserId)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("resolve session user: %w", db.ErrSessionNotFound)
	}
	return session.UserId, nil
}

// ClearUserToken exposes a cleanup result for logout. Missing credentials are
// idempotent; callers must still surface storage errors.
func (s *SessionService) ClearUserToken(token string) (bool, error) {
	return db.DeleteSessionForToken(context.Background(), token)
}

// 注销时清空session
func (this *SessionService) Clear(sessionId string) bool {
	_, err := this.ClearUserToken(sessionId)
	return err == nil
}
func (this *SessionService) Get(sessionId string) (info.Session, error) {
	return db.GetOrCreateSession(context.Background(), sessionId, time.Now())
}

//------------------
// 错误次数处理

// 登录错误时间是否已超过了
func (this *SessionService) LoginTimesIsOver(sessionId string) (bool, error) {
	session, err := this.Get(sessionId)
	if err != nil {
		return false, err
	}
	return session.LoginTimes > 5, nil
}

// 登录成功后清空错误次数
func (this *SessionService) ClearLoginTimes(sessionId string) error {
	return db.UpsertSessionFields(context.Background(), sessionId, bson.M{"LoginTimes": 0}, time.Now())
}

// ClearTransientSessionState removes login failure and captcha state carried
// by the anonymous _ID after a successful authentication boundary.
func (this *SessionService) ClearTransientSessionState(sessionId string) error {
	return db.UpsertSessionFields(context.Background(), sessionId,
		bson.M{"LoginTimes": 0, "Captcha": ""}, time.Now())
}

// 增加错误次数
func (this *SessionService) IncrLoginTimes(sessionId string) error {
	return db.IncrementSessionLoginTimes(context.Background(), sessionId, time.Now())
}

// ----------
// 验证码
func (this *SessionService) GetCaptcha(sessionId string) (string, error) {
	session, err := this.Get(sessionId)
	if err != nil {
		return "", err
	}
	return session.Captcha, nil
}
func (this *SessionService) SetCaptcha(sessionId, captcha string) error {
	return db.UpsertSessionFields(context.Background(), sessionId, bson.M{"Captcha": captcha}, time.Now())
}

// -----------
// API
func (this *SessionService) GetUserId(sessionId string) string {
	userID, err := this.ResolveUserID(sessionId)
	if err != nil {
		return ""
	}
	return userID
}

// 登录成功后设置userId
func (this *SessionService) SetUserId(sessionId, userId string) bool {
	return db.CreateSessionForToken(context.Background(), sessionId, userId, time.Now()) == nil
}
