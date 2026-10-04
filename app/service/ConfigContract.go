package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AdminPrincipal struct {
	ActorID  domain.ObjectID
	Username string
}

var ErrAdminPrincipal = errors.New("admin principal validation failed")

func (this *ConfigService) RequireAdmin(ctx context.Context, userID, username string) error {
	_, err := this.ResolveAdminPrincipal(ctx, userID, username)
	return err
}

// ResolveAdminPrincipal is the single authorization fact source used by the
// admin adapter and services. It verifies both configured admin identity and
// the current persisted user record, failing closed on storage errors.
func (this *ConfigService) ResolveAdminPrincipal(ctx context.Context, userID, username string) (AdminPrincipal, error) {
	if this == nil || db.Users == nil {
		return AdminPrincipal{}, fmt.Errorf("%w: user storage is unavailable", ErrAdminPrincipal)
	}
	actorID, err := domain.ParseObjectID(strings.TrimSpace(userID))
	if err != nil || actorID.IsZero() || strings.TrimSpace(username) == "" || username != this.GetAdminUsername() {
		return AdminPrincipal{}, fmt.Errorf("%w: principal is not the configured administrator", ErrAdminPrincipal)
	}
	var user info.User
	if err := db.Users.FindIdContext(ctx, actorID).One(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return AdminPrincipal{}, fmt.Errorf("%w: administrator record is missing", ErrAdminPrincipal)
		}
		return AdminPrincipal{}, fmt.Errorf("%w: read administrator record: %v", ErrAdminPrincipal, err)
	}
	if user.UserId != actorID || user.Username != username {
		return AdminPrincipal{}, fmt.Errorf("%w: persisted identity mismatch", ErrAdminPrincipal)
	}
	return AdminPrincipal{ActorID: actorID, Username: username}, nil
}

const RedactedSecretValue = "[configured]"

var secretConfigKeys = map[string]struct{}{
	"emailPassword": {}, "demoPassword": {}, "mongoPassword": {}, "db.password": {},
	"actionToken": {}, "callbackSecret": {}, "smtpPassword": {},
}

// IsSecretConfigKey is shared by admin views, logs and mutation handling.
func IsSecretConfigKey(key string) bool {
	_, ok := secretConfigKeys[key]
	return ok || strings.Contains(strings.ToLower(key), "password") || strings.Contains(strings.ToLower(key), "secret")
}

func (this *ConfigService) RedactedStringConfigs() map[string]string {
	result := make(map[string]string, len(this.GlobalStringConfigs))
	for key, value := range this.GlobalStringConfigs {
		if IsSecretConfigKey(key) {
			result[key] = RedactedSecretValue
		} else {
			result[key] = value
		}
	}
	return result
}

func (this *ConfigService) RedactedConfigProjection() map[string]interface{} {
	result := make(map[string]interface{}, len(this.GlobalAllConfigs))
	for key, value := range this.GlobalAllConfigs {
		if IsSecretConfigKey(key) {
			result[key] = RedactedSecretValue
		} else {
			result[key] = value
		}
	}
	return result
}

func isMaskedSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == RedactedSecretValue || trimmed == "********" || trimmed == "••••••••"
}

// ConfigMutationResult preserves per-key outcomes so a later successful key
// can never overwrite an earlier failure in the HTTP envelope.
type ConfigMutationResult struct {
	Applied []string
	Skipped []string
	Errors  map[string]error
}

func (r ConfigMutationResult) Err() error {
	for key, err := range r.Errors {
		return fmt.Errorf("config key %s: %w", key, err)
	}
	return nil
}

// UpdateGlobalStringConfigs validates all keys before applying them in a
// stable order. Callers must inspect Err and may expose Applied/Errors for
// reconciliation; the cache is only changed after each key's read-back.
func (this *ConfigService) UpdateGlobalStringConfigs(userID string, values map[string]string) ConfigMutationResult {
	result := ConfigMutationResult{Errors: map[string]error{}}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		if IsSecretConfigKey(key) && isMaskedSecret(value) {
			result.Skipped = append(result.Skipped, key)
			continue
		}
		if strings.TrimSpace(key) == "" {
			result.Errors[key] = errors.New("configuration key is blank")
			continue
		}
		if err := this.updateGlobalConfigWithError(userID, key, value, false, false, false); err != nil {
			result.Errors[key] = err
			continue
		}
		result.Applied = append(result.Applied, key)
	}
	return result
}

func (this *ConfigService) UpdateGlobalConfigs(userID string, stringsValues map[string]string, arrays map[string][]string) ConfigMutationResult {
	result := ConfigMutationResult{Errors: map[string]error{}}
	// Preflight security-sensitive arrays before any durable write. This keeps
	// a valid later key from being committed after an invalid policy entry.
	arrays = cloneArrayConfigInput(arrays)
	for _, key := range []string{"mongoExecutableAllowlist", "pdfExecutableAllowlist"} {
		if values, ok := arrays[key]; ok {
			normalized, err := NormalizeExecutableAllowlist(values)
			if err != nil {
				result.Errors[key] = err
			} else {
				arrays[key] = normalized
			}
		}
	}
	if values, ok := arrays["feedbackRecipients"]; ok {
		normalized, err := ValidateFeedbackRecipients(values)
		if err != nil {
			result.Errors["feedbackRecipients"] = err
		} else {
			arrays["feedbackRecipients"] = normalized
		}
	}
	keys := make([]string, 0, len(stringsValues)+len(arrays))
	for key := range stringsValues {
		keys = append(keys, key)
	}
	for key := range arrays {
		if _, duplicate := stringsValues[key]; duplicate {
			result.Errors[key] = errors.New("configuration key has conflicting value kinds")
			continue
		}
		keys = append(keys, key)
	}
	if len(result.Errors) > 0 {
		return result
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err, exists := result.Errors[key]; exists {
			_ = err
			continue
		}
		if value, ok := stringsValues[key]; ok {
			if IsSecretConfigKey(key) && isMaskedSecret(value) {
				result.Skipped = append(result.Skipped, key)
				continue
			}
			if err := this.updateGlobalConfigWithError(userID, key, value, false, false, false); err != nil {
				result.Errors[key] = err
			} else {
				result.Applied = append(result.Applied, key)
			}
			continue
		}
		if _, exists := result.Errors[key]; exists {
			continue
		}
		if err := this.updateGlobalConfigWithError(userID, key, arrays[key], true, false, false); err != nil {
			result.Errors[key] = err
		} else {
			result.Applied = append(result.Applied, key)
		}
	}
	return result
}

func cloneArrayConfigInput(values map[string][]string) map[string][]string {
	result := make(map[string][]string, len(values))
	for key, value := range values {
		result[key] = append([]string(nil), value...)
	}
	return result
}
