package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sessionCookieSuffix = "_SESSION"

// SessionCodec encodes the web session into a single HMAC-authenticated
// cookie keyed by app.secret. It deliberately cannot read legacy Revel
// cookies: any undecodable input means an anonymous visitor who logs in
// again (the accepted deployment impact of C-b).
type SessionCodec struct {
	Secret  []byte        // raw app.secret; a derived key is used internally
	Prefix  string        // cookie.prefix
	Domain  string        // cookie.domain (empty = host-only)
	Secure  bool          // cookie.secure
	TTL     time.Duration // session.expires
	NowFunc func() time.Time
}

type sessionPayload struct {
	Keys map[string]string `json:"keys"`
	Exp  int64             `json:"exp"` // unix seconds
}

func (c *SessionCodec) derivedKey() []byte {
	sum := sha256.Sum256(c.Secret)
	return sum[:]
}

func (c *SessionCodec) cookieName() string {
	return c.Prefix + sessionCookieSuffix
}

func (c *SessionCodec) now() time.Time {
	if c.NowFunc != nil {
		return c.NowFunc()
	}
	return time.Now()
}

// Encode signs the session keys and returns the cookie to set on a response.
func (c *SessionCodec) Encode(keys map[string]string) (*http.Cookie, error) {
	now := c.now()
	payload := sessionPayload{Keys: keys, Exp: now.Add(c.TTL).Unix()}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("session encode: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	return &http.Cookie{
		Name:     c.cookieName(),
		Value:    body + "." + c.sign(body),
		Path:     "/",
		Domain:   c.Domain,
		Expires:  now.Add(c.TTL),
		MaxAge:   int(c.TTL / time.Second),
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// Decode verifies and parses a session cookie value. Every failure mode —
// malformed input, bad signature, expired payload — is an error; callers
// must treat all of them as an anonymous session.
func (c *SessionCodec) Decode(value string) (map[string]string, error) {
	body, sig, ok := strings.Cut(value, ".")
	if !ok {
		return nil, fmt.Errorf("session cookie: malformed")
	}
	expected := c.sign(body)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, fmt.Errorf("session cookie: signature mismatch")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("session cookie: bad payload encoding: %w", err)
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("session cookie: bad payload: %w", err)
	}
	if c.now().Unix() >= payload.Exp {
		return nil, fmt.Errorf("session cookie: expired")
	}
	return payload.Keys, nil
}

func (c *SessionCodec) sign(body string) string {
	mac := hmac.New(sha256.New, c.derivedKey())
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// NewSessionCodec builds a codec from app.conf values.
func NewSessionCodec(cfg *Config) *SessionCodec {
	secret, _ := cfg.String("app.secret")
	prefix, _ := cfg.String("cookie.prefix")
	domain, _ := cfg.String("cookie.domain")
	ttl, err := time.ParseDuration(cfg.StringDefault("session.expires", "3h"))
	if err != nil || ttl <= 0 {
		ttl = 3 * time.Hour
	}
	return &SessionCodec{
		Secret: []byte(secret),
		Prefix: prefix,
		Domain: domain,
		Secure: cfg.BoolDefault("cookie.secure", false),
		TTL:    ttl,
	}
}
