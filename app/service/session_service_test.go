package service

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/db"
)

func TestSessionServiceCaptchaOperationsSurfaceUninitializedStore(t *testing.T) {
	saved := db.Sessions
	db.Sessions = nil
	defer func() { db.Sessions = saved }()

	service := &SessionService{}
	if err := service.SetCaptcha("anonymous-session", "answer"); !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("SetCaptcha error = %v, want ErrMongoClientNotInitialized", err)
	}
	if _, err := service.GetCaptcha("anonymous-session"); !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("GetCaptcha error = %v, want ErrMongoClientNotInitialized", err)
	}
}
