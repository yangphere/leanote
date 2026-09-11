package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestEmailOutboxTransportUsesPersistedActivationToken(t *testing.T) {
	savedConfig := configService
	savedToken := tokenService
	configService = &ConfigService{GlobalStringConfigs: map[string]string{
		"siteUrl":                      "https://notes.example.test",
		"emailTemplateRegisterSubject": "Activate {{.user.username}}",
		"emailTemplateRegister":        "token={{.token}} url={{.tokenUrl}}",
	}}
	tokenService = nil
	defer func() {
		configService = savedConfig
		tokenService = savedToken
	}()

	var recipient, subject, body string
	mailer := NewEmailService()
	mailer.send = func(_ context.Context, to, gotSubject, gotBody string) error {
		recipient, subject, body = to, gotSubject, gotBody
		return nil
	}
	userID := db.MustObjectIDFromHex("507f1f77bcf86cd799439011")
	event := db.OutboxEvent{
		Kind:        "activate-email",
		AggregateID: userID,
		Payload: map[string]any{
			"userId":    userID.Hex(),
			"email":     "user@example.test",
			"username":  "demo-user",
			"token":     "persisted-activation-token",
			"tokenType": info.TokenActiveEmail,
		},
	}

	if err := mailer.DeliverOutbox(context.Background(), event); err != nil {
		t.Fatalf("DeliverOutbox: %v", err)
	}
	if recipient != "user@example.test" || subject != "Activate demo-user" {
		t.Fatalf("mail recipient/subject = %q/%q", recipient, subject)
	}
	if !strings.Contains(body, "persisted-activation-token") || !strings.Contains(body, "/user/activeEmail?token=persisted-activation-token") {
		t.Fatalf("activation body did not use persisted token: %q", body)
	}
}

func TestEmailOutboxSMTPStopsWhenContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	savedConfig := configService
	configService = &ConfigService{GlobalStringConfigs: map[string]string{
		"emailHost": host, "emailPort": port,
		"emailUsername": "sender@example.test", "emailPassword": "secret",
	}}
	defer func() { configService = savedConfig }()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewEmailService().SendEmailContext(ctx, "user@example.test", "subject", "body")
	}()
	var serverConn net.Conn
	select {
	case serverConn = <-accepted:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("SMTP connection was not accepted")
	}
	defer serverConn.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled SMTP delivery returned nil")
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP delivery did not stop after cancellation")
	}
}

func TestOutboxWorkerStopsWhenContextIsCancelled(t *testing.T) {
	worker := NewOutboxWorker(func(context.Context, db.OutboxEvent) error { return nil })
	worker.pollInterval = time.Millisecond
	worker.nextEvent = func(context.Context, time.Time) (db.OutboxEvent, error) {
		return db.OutboxEvent{}, mongo.ErrNoDocuments
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("outbox worker did not stop after cancellation")
	}
}

func TestOutboxWorkerContinuesAfterRecordedDeliveryFailure(t *testing.T) {
	id := db.NewObjectID()
	deliveryErr := errors.New("smtp unavailable")
	processed := make(chan struct{}, 1)
	calls := 0
	worker := NewOutboxWorker(func(context.Context, db.OutboxEvent) error { return nil })
	worker.pollInterval = time.Millisecond
	worker.nextEvent = func(context.Context, time.Time) (db.OutboxEvent, error) {
		calls++
		if calls == 1 {
			return db.OutboxEvent{ID: id}, nil
		}
		return db.OutboxEvent{}, mongo.ErrNoDocuments
	}
	worker.deliverEvent = func(context.Context, db.OutboxEvent, time.Time, db.OutboxTransport) error {
		processed <- struct{}{}
		return deliveryErr
	}
	worker.onError = func(error) {}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	select {
	case <-processed:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("outbox worker did not attempt due event")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("outbox worker did not stop after delivery failure")
	}
}
