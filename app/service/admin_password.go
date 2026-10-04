package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var errDockerAdminNotConfigured = errors.New("docker administrator is not configured")

type dockerAdminPasswordConfig struct {
	email, initialPassword, secret string
	force                          bool
}

var dockerAdminConfig dockerAdminPasswordConfig

// ConfigureDockerAdminPassword installs the production-only credential seam.
// It is deliberately process-local; the password itself is never persisted.
func ConfigureDockerAdminPassword(email, initialPassword, secret string, force bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "demo@leanote.com" {
		return fmt.Errorf("docker administrator cannot use the disabled demo account")
	}
	if !IsEmail(email) || len(initialPassword) < 6 || strings.HasPrefix(initialPassword, "REPLACE_WITH_") || strings.TrimSpace(secret) == "" || strings.HasPrefix(strings.TrimSpace(secret), "REPLACE_WITH_") {
		return errDockerAdminNotConfigured
	}
	dockerAdminConfig = dockerAdminPasswordConfig{email: email, initialPassword: initialPassword, secret: strings.TrimSpace(secret), force: force}
	return nil
}

func ParseAdminForceEnvPassword(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("invalid LEANOTE_ADMIN_FORCE_ENV_PASSWORD")
	}
}

// BootstrapDockerAdmin reconciles the Docker seed administrator exactly once.
func BootstrapDockerAdmin(email, initialPassword, secret string, force bool) error {
	if err := ConfigureDockerAdminPassword(email, initialPassword, secret, force); err != nil {
		return err
	}
	if db.Users == nil || db.Configs == nil {
		return db.ErrMongoClientNotInitialized
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var markers []info.Config
	if err := db.Configs.Find(bson.M{"Key": "adminBootstrapCompleted"}).All(&markers); err != nil {
		return err
	}
	if len(markers) > 1 {
		return errors.New("docker administrator bootstrap marker is not unique")
	}
	var users []info.User
	if err := db.Users.Find(bson.M{"Email": email}).All(&users); err != nil {
		return err
	}
	if len(markers) == 0 {
		if len(users) == 0 {
			if err := db.Users.Find(bson.M{"Username": "admin"}).All(&users); err != nil {
				return err
			}
		}
		if len(users) != 1 {
			return fmt.Errorf("docker administrator identity is not unique")
		}
		user := users[0]
		username, err := dockerAdminUsername(email)
		if err != nil {
			return err
		}
		password := GenPwd(initialPassword)
		if password == "" {
			return fmt.Errorf("generate administrator password hash")
		}
		if err := db.Users.UpdateOneMatchedContext(nil, bson.M{"_id": user.UserId}, bson.M{"$set": bson.M{
			"Email": email, "Username": strings.ToLower(username), "UsernameRaw": username,
			"Pwd": password, "AdminPasswordSetupRequired": true,
			"AdminEnvPasswordFingerprint": adminPasswordFingerprint(secret, initialPassword), "Disabled": false,
		}}); err != nil {
			return err
		}
		if _, err := db.Configs.Upsert(bson.M{"Key": "adminBootstrapCompleted"}, bson.M{"$set": bson.M{"Key": "adminBootstrapCompleted", "ValueStr": user.UserId.Hex()}}); err != nil {
			return err
		}
	} else {
		var user info.User
		if !db.IsValidObjectIDHex(markers[0].ValueStr) {
			return errors.New("docker administrator marker has an invalid user id")
		}
		if err := db.Users.FindId(db.MustObjectIDFromHex(markers[0].ValueStr)).One(&user); err != nil {
			return fmt.Errorf("docker administrator marker user is unavailable: %w", err)
		}
		if force && !user.AdminPasswordSetupRequired && user.AdminEnvPasswordFingerprint == adminPasswordFingerprint(secret, initialPassword) {
			log.Printf("administrator ENV recovery mode remains enabled after password consumption; disable it")
		}
	}
	if _, err := db.Users.UpdateAll(bson.M{"Email": "demo@leanote.com"}, bson.M{"$set": bson.M{"Disabled": true}}); err != nil {
		return err
	}
	return nil
}

func dockerAdminUsername(email string) (string, error) {
	base := strings.SplitN(email, "@", 2)[0]
	var builder strings.Builder
	for _, r := range strings.ToLower(base) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	username := strings.Trim(builder.String(), "-_")
	if len(username) < 4 {
		return "", fmt.Errorf("docker administrator email prefix is too short")
	}
	return username, nil
}

func adminPasswordFingerprint(secret, password string) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func AdminPasswordChangeRequired(userID string) (bool, error) {
	if configService == nil || configService.GetAdminUserId() == "" || userID != configService.GetAdminUserId() {
		return false, nil
	}
	if db.Users == nil || !db.IsValidObjectIDHex(userID) {
		return false, db.ErrMongoClientNotInitialized
	}
	var user info.User
	if err := db.Users.FindIdContext(nil, db.MustObjectIDFromHex(userID)).One(&user); err != nil {
		return false, err
	}
	return user.AdminPasswordSetupRequired, nil
}

func updatePasswordAndClearAdminGate(ctx context.Context, userID domain.ObjectID, password string) error {
	if db.Users == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.UpdateOneMatchedContext(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"Pwd": password, "AdminPasswordSetupRequired": false}})
}

func forceUpdateAdminPassword(userID string, password string) error {
	if db.Users == nil || !db.IsValidObjectIDHex(userID) {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.UpdateOneMatchedContext(nil, bson.M{"_id": db.MustObjectIDFromHex(userID), "AdminPasswordSetupRequired": true}, bson.M{"$set": bson.M{"Pwd": password, "AdminPasswordSetupRequired": false}})
}

func loginWithDockerAdmin(emailOrUsername, password string) (info.User, bool, error) {
	cfg := dockerAdminConfig
	if cfg.email == "" || db.Users == nil || configService == nil || !db.IsValidObjectIDHex(configService.GetAdminUserId()) {
		return info.User{}, false, nil
	}
	var user info.User
	if err := db.Users.FindId(db.MustObjectIDFromHex(configService.GetAdminUserId())).One(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.User{}, false, nil
		}
		return info.User{}, true, fmt.Errorf("load Docker administrator: %w", err)
	}
	identity := strings.ToLower(strings.TrimSpace(emailOrUsername))
	if identity != strings.ToLower(user.Email) && identity != strings.ToLower(user.Username) {
		return info.User{}, false, nil
	}
	if user.Disabled {
		return info.User{}, true, ErrInvalidCredentials
	}
	if ComparePwd(password, user.Pwd) {
		return user, true, nil
	}
	if cfg.force && user.AdminEnvPasswordFingerprint != adminPasswordFingerprint(cfg.secret, cfg.initialPassword) && password == cfg.initialPassword {
		fingerprint := adminPasswordFingerprint(cfg.secret, cfg.initialPassword)
		if err := db.Users.UpdateOneMatchedContext(nil, bson.M{"_id": user.UserId, "AdminPasswordSetupRequired": false, "AdminEnvPasswordFingerprint": bson.M{"$ne": fingerprint}}, bson.M{"$set": bson.M{"AdminPasswordSetupRequired": true, "AdminEnvPasswordFingerprint": fingerprint}}); err != nil {
			return info.User{}, true, fmt.Errorf("consume Docker administrator recovery password: %w", err)
		}
		user.AdminPasswordSetupRequired = true
		return user, true, nil
	}
	return info.User{}, true, ErrInvalidCredentials
}
