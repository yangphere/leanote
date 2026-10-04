package db

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrSessionNotFound = errors.New("session not found")

const apiSessionTokenBytes = 32

// NewAPISessionToken returns the client-facing credential for a new API
// session. Callers must persist it through CreateSessionForToken, which stores
// only its digest.
func NewAPISessionToken() (string, error) {
	raw := make([]byte, apiSessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate API session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// NewAnonymousSessionID creates the stable cookie-local identity used for the
// no-token API fallback. It is not an API credential.
func NewAnonymousSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate anonymous session ID: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// GetOrCreateSession atomically materializes the server-side state for a
// stable anonymous session ID. Query and decode failures remain observable;
// callers must not substitute an empty session.
func GetOrCreateSession(parent context.Context, sessionID string, now time.Time) (info.Session, error) {
	var session info.Session
	if Sessions == nil {
		return session, fmt.Errorf("get session: %w", ErrMongoClientNotInitialized)
	}
	if sessionID == "" {
		return session, fmt.Errorf("get session: %w", ErrSessionNotFound)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	err := Sessions.coll.FindOneAndUpdate(
		ctx,
		bson.M{"SessionId": sessionID},
		bson.M{"$setOnInsert": newSessionFields(sessionID, now)},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&raw)
	if err != nil {
		return session, fmt.Errorf("get or create session: %w", err)
	}
	if err := decodeBSONMap(raw, &session); err != nil {
		return session, fmt.Errorf("decode session: %w", err)
	}
	return session, nil
}

// UpsertSessionFields persists anonymous-session state in one write. The
// upsert removes the former read-then-insert/read-then-update gaps and the
// returned error is the persistence acknowledgement used by controllers.
func UpsertSessionFields(parent context.Context, sessionID string, fields bson.M, now time.Time) error {
	if Sessions == nil {
		return fmt.Errorf("upsert session: %w", ErrMongoClientNotInitialized)
	}
	if sessionID == "" {
		return fmt.Errorf("upsert session: %w", ErrSessionNotFound)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	if _, err := Sessions.coll.UpdateOne(
		ctx,
		bson.M{"SessionId": sessionID},
		sessionFieldUpsert(sessionID, fields, now),
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

// IncrementSessionLoginTimes atomically increments the credential-failure
// counter while still creating the anonymous session on its first failure.
func IncrementSessionLoginTimes(parent context.Context, sessionID string, now time.Time) error {
	if Sessions == nil {
		return fmt.Errorf("increment session login times: %w", ErrMongoClientNotInitialized)
	}
	if sessionID == "" {
		return fmt.Errorf("increment session login times: %w", ErrSessionNotFound)
	}
	update := sessionFieldUpsert(sessionID, nil, now)
	insert := update["$setOnInsert"].(bson.M)
	delete(insert, "LoginTimes")
	update["$inc"] = bson.M{"LoginTimes": 1}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	if _, err := Sessions.coll.UpdateOne(
		ctx,
		bson.M{"SessionId": sessionID},
		update,
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("increment session login times: %w", err)
	}
	return nil
}

func newSessionFields(sessionID string, now time.Time) bson.M {
	return bson.M{
		"_id":         NewObjectID(),
		"SessionId":   sessionID,
		"LoginTimes":  0,
		"Captcha":     "",
		"UserId":      "",
		"CreatedTime": now,
		"UpdatedTime": now,
	}
}

func sessionFieldUpsert(sessionID string, fields bson.M, now time.Time) bson.M {
	set := make(bson.M, len(fields)+1)
	for key, value := range fields {
		set[key] = value
	}
	set["UpdatedTime"] = now
	insert := newSessionFields(sessionID, now)
	for key := range set {
		delete(insert, key)
	}
	return bson.M{"$set": set, "$setOnInsert": insert}
}

// SessionTokenDigest is the at-rest representation of an API token.
func SessionTokenDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// sessionTokenFilters is the sole token matching policy for API sessions.
// New documents are always searched by digest first. Only historical ObjectID
// tokens may be retried verbatim, preventing arbitrary plaintext fallback.
func sessionTokenFilters(token string) []bson.M {
	filters := []bson.M{{"SessionId": SessionTokenDigest(token)}}
	if IsValidObjectIDHex(token) {
		filters = append(filters, bson.M{"SessionId": token})
	}
	return filters
}

// CreateSessionForToken persists a new API session without retaining the
// client-facing token in MongoDB.
func CreateSessionForToken(parent context.Context, token, userID string, now time.Time) error {
	if Sessions == nil {
		return fmt.Errorf("create session: %w", ErrMongoClientNotInitialized)
	}
	if token == "" || userID == "" {
		return fmt.Errorf("create session: %w", ErrSessionNotFound)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	session := info.Session{
		Id:          NewObjectID(),
		SessionId:   SessionTokenDigest(token),
		UserId:      userID,
		CreatedTime: now,
		UpdatedTime: now,
	}
	if _, err := Sessions.coll.InsertOne(ctx, session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionExpired applies the inclusive application-side idle boundary. A zero
// timestamp is treated as expired so absent/legacy values fail closed.
func SessionExpired(updatedAt, now time.Time) bool {
	return !now.Before(updatedAt.Add(SessionIdleTTL))
}

// RefreshSession atomically refreshes a still-valid session. The expiry check
// is part of the update filter, so a concurrent request cannot resurrect a
// session after the three-hour boundary.
func RefreshSession(sessionID string, now time.Time) (bool, error) {
	return RefreshSessionContext(context.Background(), sessionID, now)
}

func RefreshSessionContext(parent context.Context, sessionID string, now time.Time) (bool, error) {
	if Sessions == nil {
		return false, fmt.Errorf("refresh session: %w", ErrMongoClientNotInitialized)
	}
	if sessionID == "" {
		return false, fmt.Errorf("refresh session: %w", ErrSessionNotFound)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	update := bson.M{"$set": bson.M{"UpdatedTime": now}}
	for _, filter := range sessionTokenFilters(sessionID) {
		filter["UpdatedTime"] = bson.M{"$gt": now.Add(-SessionIdleTTL)}
		var raw bson.M
		err := Sessions.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("refresh session: %w", err)
		}
		return true, nil
	}
	return false, fmt.Errorf("refresh session: %w", ErrSessionNotFound)
}

// ResolveSessionAndRefresh returns a valid session and refreshes its idle
// timestamp in the same atomic find-and-update operation. Expired and absent
// sessions intentionally share ErrSessionNotFound at this boundary so the
// identity layer cannot accidentally create a new session on lookup.
func ResolveSessionAndRefresh(parent context.Context, sessionID string, now time.Time) (info.Session, error) {
	var session info.Session
	if Sessions == nil {
		return session, fmt.Errorf("resolve session: %w", ErrMongoClientNotInitialized)
	}
	if sessionID == "" {
		return session, fmt.Errorf("resolve session: %w", ErrSessionNotFound)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	update := bson.M{"$set": bson.M{"UpdatedTime": now}}
	for _, filter := range sessionTokenFilters(sessionID) {
		filter["UpdatedTime"] = bson.M{"$gt": now.Add(-SessionIdleTTL)}
		var raw bson.M
		err := Sessions.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return session, fmt.Errorf("resolve session: %w", err)
		}
		if err := decodeBSONMap(raw, &session); err != nil {
			return session, fmt.Errorf("decode session: %w", err)
		}
		return session, nil
	}
	return session, fmt.Errorf("resolve session: %w", ErrSessionNotFound)
}

// DeleteSessionForToken removes a session through the same digest-first
// matcher used by resolve/refresh. A missing mapping is reported separately
// from storage failure so logout can stay idempotent without claiming cleanup
// succeeded after a database error.
func DeleteSessionForToken(parent context.Context, token string) (bool, error) {
	if Sessions == nil {
		return false, fmt.Errorf("delete session: %w", ErrMongoClientNotInitialized)
	}
	if token == "" {
		return false, nil
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	for _, filter := range sessionTokenFilters(token) {
		result, err := Sessions.coll.DeleteOne(ctx, filter)
		if err != nil {
			return false, fmt.Errorf("delete session: %w", err)
		}
		if result.DeletedCount == 1 {
			return true, nil
		}
	}
	return false, nil
}

// SessionUserExists validates that a resolved session still points to a live
// user. Invalid historical IDs and missing users are deliberately anonymous,
// while database failures remain observable to the adapter.
func SessionUserExists(parent context.Context, userID string) (bool, error) {
	if Users == nil {
		return false, fmt.Errorf("resolve session user: %w", ErrMongoClientNotInitialized)
	}
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return false, nil
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	err = Users.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&raw)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("resolve session user: %w", err)
	}
	return true, nil
}

func decodeBSONMap(raw bson.M, result interface{}) error {
	data, err := bson.Marshal(raw)
	if err != nil {
		return err
	}
	decoder := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(data)))
	decoder.SetRegistry(lea.CodecRegistry)
	return decoder.Decode(result)
}
