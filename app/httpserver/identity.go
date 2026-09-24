package httpserver

import "fmt"

// SessionReader is the application-facing view of a decoded web session.
// Implementations must return an error for storage/decoding failures rather
// than treating them as an empty authenticated session.
type SessionReader interface {
	Get(key string) (string, bool, error)
}

// SessionWriter is the application-facing session mutation boundary. Commit
// includes cookie encoding and header publication.
type SessionWriter interface {
	Set(key, value string) error
	Delete(key string) error
	Commit() error
}

type PrincipalSource string
type TokenState string
type PrincipalRole string
type PrincipalPolicy func(userID string) (PrincipalRole, bool, error)

const (
	PrincipalSourceNone       PrincipalSource = "none"
	PrincipalSourceWebSession PrincipalSource = "web_session"
	PrincipalSourceAPIToken   PrincipalSource = "api_token"
	TokenStateAbsent          TokenState      = "absent"
	TokenStateValid           TokenState      = "valid"
	PrincipalRoleAnonymous    PrincipalRole   = "anonymous"
	PrincipalRoleMember       PrincipalRole   = "member"
	PrincipalRoleAdmin        PrincipalRole   = "admin"
)

// Principal is the single request-local authentication decision consumed by
// adapters. UserID is empty for anonymous requests; callers must not infer an
// identity from request fields after this value has been established.
type Principal struct {
	UserID     string
	Source     PrincipalSource
	TokenState TokenState
	Role       PrincipalRole
	IsDemo     bool
}

func AnonymousPrincipal() Principal {
	return Principal{Source: PrincipalSourceNone, TokenState: TokenStateAbsent, Role: PrincipalRoleAnonymous}
}

func AuthenticatedPrincipal(userID string, source PrincipalSource, tokenState TokenState) Principal {
	principal, err := AuthenticatedPrincipalWithPolicy(userID, source, tokenState, nil)
	if err != nil {
		return AnonymousPrincipal()
	}
	return principal
}

func AuthenticatedPrincipalWithPolicy(userID string, source PrincipalSource, tokenState TokenState, policy PrincipalPolicy) (Principal, error) {
	if userID == "" {
		return AnonymousPrincipal(), nil
	}

	role := PrincipalRoleMember
	isDemo := false
	if policy != nil {
		var err error
		role, isDemo, err = policy(userID)
		if err != nil {
			return AnonymousPrincipal(), err
		}
		if role != PrincipalRoleMember && role != PrincipalRoleAdmin {
			return AnonymousPrincipal(), fmt.Errorf("invalid principal role %q", role)
		}
	}

	return Principal{
		UserID:     userID,
		Source:     source,
		TokenState: tokenState,
		Role:       role,
		IsDemo:     isDemo,
	}, nil
}
