package db

import (
	"bytes"
	"context"
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
	filter := bson.M{
		"SessionId":   sessionID,
		"UpdatedTime": bson.M{"$gt": now.Add(-SessionIdleTTL)},
	}
	update := bson.M{"$set": bson.M{"UpdatedTime": now}}
	var raw bson.M
	err := Sessions.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, fmt.Errorf("refresh session: %w", ErrSessionNotFound)
	}
	if err != nil {
		return false, fmt.Errorf("refresh session: %w", err)
	}
	return true, nil
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
	filter := bson.M{
		"SessionId":   sessionID,
		"UpdatedTime": bson.M{"$gt": now.Add(-SessionIdleTTL)},
	}
	update := bson.M{"$set": bson.M{"UpdatedTime": now}}
	var raw bson.M
	err := Sessions.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return session, fmt.Errorf("resolve session: %w", ErrSessionNotFound)
	}
	if err != nil {
		return session, fmt.Errorf("resolve session: %w", err)
	}
	if err := decodeBSONMap(raw, &session); err != nil {
		return session, fmt.Errorf("decode session: %w", err)
	}
	return session, nil
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
