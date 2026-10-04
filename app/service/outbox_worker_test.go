package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
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

func TestCommentOutboxInvalidPayloadIsDefiniteRejection(t *testing.T) {
	savedConfig := configService
	configService = &ConfigService{GlobalStringConfigs: map[string]string{}}
	defer func() { configService = savedConfig }()
	mailer := NewEmailService()
	mailer.send = func(context.Context, string, string, string) error {
		t.Fatal("invalid comment reached SMTP")
		return nil
	}
	event := db.OutboxEvent{Kind: "comment", Payload: map[string]any{"email": "user@example.test"}}
	if err := mailer.DeliverOutbox(context.Background(), event); !errors.Is(err, db.ErrOutboxTransportRejected) {
		t.Fatalf("invalid comment payload error = %v", err)
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

func TestCommentSMTPDataRejectionIsSafeToRetry(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 smtp.example.test ready\r\n")
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				serverDone <- readErr
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				_, _ = fmt.Fprint(conn, "250 smtp.example.test\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			case strings.HasPrefix(line, "DATA"):
				_, _ = fmt.Fprint(conn, "554 rejected before body\r\n")
				serverDone <- nil
				return
			default:
				serverDone <- fmt.Errorf("unexpected SMTP command: %q", line)
				return
			}
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = sendSMTPContext(ctx, smtpDeliveryConfig{host: host, port: port, username: "sender@example.test"}, []string{"user@example.test"}, []byte("body"))
	if !errors.Is(err, db.ErrOutboxTransportRejected) {
		t.Fatalf("DATA rejection = %v, want definite rejection", err)
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatal(serverErr)
	}
}

func TestCommentSMTPAcceptedBodyIgnoresQuitFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 smtp.example.test ready\r\n")
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				serverDone <- readErr
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				_, _ = fmt.Fprint(conn, "250 smtp.example.test\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			case strings.HasPrefix(line, "DATA"):
				_, _ = fmt.Fprint(conn, "354 send body\r\n")
			case line == ".\r\n":
				_, _ = fmt.Fprint(conn, "250 accepted\r\n")
			case strings.HasPrefix(line, "QUIT"):
				serverDone <- nil
				return
			}
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = sendSMTPContext(ctx, smtpDeliveryConfig{host: host, port: port, username: "sender@example.test"}, []string{"user@example.test"}, []byte("body"))
	if err != nil {
		t.Fatalf("accepted SMTP body = %v", err)
	}
	if serverErr := <-serverDone; serverErr != nil {
		t.Fatal(serverErr)
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
