package httpserver

import (
	"errors"
	"testing"
)

var _ SessionReader = (*Context)(nil)
var _ SessionWriter = (*Context)(nil)

type testSessionReader struct {
	value string
	err   error
}

func (r testSessionReader) Get(string) (string, bool, error) {
	if r.err != nil {
		return "", false, r.err
	}
	return r.value, true, nil
}

type testSessionWriter struct {
	setKey   string
	setValue string
	deleted  string
	commits  int
	err      error
}

func (w *testSessionWriter) Set(key, value string) error {
	w.setKey, w.setValue = key, value
	return w.err
}

func (w *testSessionWriter) Delete(key string) error {
	w.deleted = key
	return w.err
}

func (w *testSessionWriter) Commit() error {
	w.commits++
	return w.err
}

func TestContextSessionWriterReportsCommitFailure(t *testing.T) {
	wantErr := errors.New("cookie write failed")
	ctx := &Context{Session: map[string]string{}}
	ctx.sessionCommit = func() error { return wantErr }

	if err := ctx.Set("UserId", "user-1"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if value, ok, err := ctx.Get("UserId"); err != nil || !ok || value != "user-1" {
		t.Fatalf("Get = (%q, %t, %v), want user-1", value, ok, err)
	}
	if err := ctx.Commit(); !errors.Is(err, wantErr) {
		t.Fatalf("Commit error = %v, want %v", err, wantErr)
	}

	if err := ctx.Delete("UserId"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok, err := ctx.Get("UserId"); err != nil || ok {
		t.Fatalf("deleted session value = (ok=%t, err=%v), want absent", ok, err)
	}
}

func TestAnonymousPrincipalIsExplicit(t *testing.T) {
	principal := AnonymousPrincipal()
	if principal.UserID != "" || principal.Source != PrincipalSourceNone || principal.TokenState != TokenStateAbsent || principal.Role != PrincipalRoleAnonymous || principal.IsDemo {
		t.Fatalf("anonymous principal = %+v", principal)
	}
}

func TestContextUsesInjectedSessionBoundaries(t *testing.T) {
	readerErr := errors.New("read failed")
	writerErr := errors.New("write failed")
	writer := &testSessionWriter{err: writerErr}
	ctx := &Context{
		Session:       map[string]string{"fallback": "must not be read"},
		SessionReader: testSessionReader{value: "from-reader"},
		SessionWriter: writer,
	}

	value, ok, err := ctx.Get("key")
	if err != nil || !ok || value != "from-reader" {
		t.Fatalf("injected Get = (%q, %t, %v)", value, ok, err)
	}
	if err := ctx.Set("key", "value"); !errors.Is(err, writerErr) || writer.setKey != "key" || writer.setValue != "value" {
		t.Fatalf("injected Set = err %v, writer=%+v", err, writer)
	}
	if err := ctx.Delete("key"); !errors.Is(err, writerErr) || writer.deleted != "key" {
		t.Fatalf("injected Delete = err %v, writer=%+v", err, writer)
	}
	if err := ctx.Commit(); !errors.Is(err, writerErr) || writer.commits != 1 {
		t.Fatalf("injected Commit = err %v, writer=%+v", err, writer)
	}

	ctx.SessionReader = testSessionReader{err: readerErr}
	if _, _, err := ctx.Get("key"); !errors.Is(err, readerErr) {
		t.Fatalf("injected reader error = %v, want %v", err, readerErr)
	}
}

func TestAuthenticatedPrincipalUsesAdminAndDemoPolicy(t *testing.T) {
	principal, err := AuthenticatedPrincipalWithPolicy(
		"admin-id",
		PrincipalSourceWebSession,
		TokenStateAbsent,
		func(userID string) (PrincipalRole, bool, error) {
			if userID != "admin-id" {
				t.Fatalf("policy userID = %q", userID)
			}
			return PrincipalRoleAdmin, true, nil
		},
	)
	if err != nil || principal.Role != PrincipalRoleAdmin || !principal.IsDemo || principal.UserID != "admin-id" {
		t.Fatalf("configured principal = %+v, err=%v", principal, err)
	}

	policyErr := errors.New("invalid demo configuration")
	principal, err = AuthenticatedPrincipalWithPolicy("user-id", PrincipalSourceAPIToken, TokenStateValid, func(string) (PrincipalRole, bool, error) {
		return "", false, policyErr
	})
	if !errors.Is(err, policyErr) || principal.Role != PrincipalRoleAnonymous || principal.UserID != "" {
		t.Fatalf("policy failure = %+v, err=%v", principal, err)
	}
}
