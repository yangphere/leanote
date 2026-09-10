package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ActionToken is the persistence-neutral action-token shape. ID is an
// independent document identity; UserID is an indexed ownership field.
type ActionToken struct {
	ID          domain.ObjectID `bson:"_id,omitempty"`
	UserID      domain.ObjectID `bson:"UserId"`
	Email       string          `bson:"Email"`
	Token       string          `bson:"Token"`
	Type        int             `bson:"Type"`
	CreatedTime time.Time       `bson:"CreatedTime"`
	ConsumedAt  *time.Time      `bson:"ConsumedAt,omitempty"`
	Legacy      bool            `bson:"-"`
}

func NewActionToken(userID domain.ObjectID, value string, tokenType int, createdAt time.Time) ActionToken {
	return ActionToken{
		ID:          NewObjectID(),
		UserID:      userID,
		Token:       value,
		Type:        tokenType,
		CreatedTime: createdAt,
	}
}

func ActionTokenExpired(createdAt, now time.Time, ttl time.Duration) bool {
	return !now.Before(createdAt.Add(ttl))
}

// DecodeActionToken accepts both the new schema and legacy documents whose
// _id was the user ID. Legacy documents are marked read-only and are never
// rewritten by this decoder.
func DecodeActionToken(raw bson.M) (ActionToken, error) {
	var token ActionToken
	var err error
	if token.ID, err = decodeObjectIDValue(raw["_id"]); err != nil {
		return token, fmt.Errorf("decode token _id: %w", err)
	}
	userIDValue, hasUserID := raw["UserId"]
	if hasUserID {
		token.UserID, err = decodeObjectIDValue(userIDValue)
		if err != nil {
			return token, fmt.Errorf("decode token UserId: %w", err)
		}
		if token.UserID.IsZero() {
			return token, fmt.Errorf("decode token UserId: zero identity")
		}
	} else if !token.ID.IsZero() {
		token.UserID = token.ID
		token.Legacy = true
	} else {
		return token, fmt.Errorf("decode token: missing identity")
	}
	if token.Token, err = stringValue(raw["Token"]); err != nil {
		return token, fmt.Errorf("decode token value: %w", err)
	}
	if token.Type, err = intValue(raw["Type"]); err != nil {
		return token, fmt.Errorf("decode token type: %w", err)
	}
	if value, ok := raw["Email"]; ok {
		token.Email, err = stringValue(value)
		if err != nil {
			return token, fmt.Errorf("decode token email: %w", err)
		}
	}
	if value, ok := raw["CreatedTime"]; ok {
		switch created := value.(type) {
		case time.Time:
			token.CreatedTime = created
		case bson.DateTime:
			token.CreatedTime = time.UnixMilli(int64(created))
		default:
			return token, fmt.Errorf("decode token CreatedTime: unsupported %T", value)
		}
	}
	if value, ok := raw["ConsumedAt"]; ok && value != nil {
		switch consumed := value.(type) {
		case time.Time:
			token.ConsumedAt = &consumed
		case bson.DateTime:
			converted := time.UnixMilli(int64(consumed))
			token.ConsumedAt = &converted
		default:
			return token, fmt.Errorf("decode token ConsumedAt: unsupported %T", value)
		}
	}
	return token, nil
}

func decodeObjectIDValue(value interface{}) (domain.ObjectID, error) {
	switch id := value.(type) {
	case domain.ObjectID:
		return id, nil
	case bson.ObjectID:
		return domain.ObjectID(id), nil
	case string:
		return domain.ParseObjectID(id)
	default:
		return domain.ObjectID{}, fmt.Errorf("unsupported ObjectID value %T", value)
	}
}

func stringValue(value interface{}) (string, error) {
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", value)
	}
	return s, nil
}

func intValue(value interface{}) (int, error) {
	switch n := value.(type) {
	case int:
		return n, nil
	case int32:
		return int(n), nil
	case int64:
		return int(n), nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", value)
	}
}

// IssueActionToken atomically replaces the active token for one user/purpose.
// It does not rewrite legacy _id=userId documents; those remain read-only.
func IssueActionToken(parent context.Context, userID domain.ObjectID, email, value string, tokenType int, createdAt time.Time) (ActionToken, error) {
	if Tokens == nil {
		return ActionToken{}, ErrMongoClientNotInitialized
	}
	if userID.IsZero() || value == "" {
		return ActionToken{}, fmt.Errorf("issue action token: invalid identity or token")
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	token := NewActionToken(userID, value, tokenType, createdAt)
	// Legacy documents use _id=userId and therefore cannot match the new
	// (UserId,Type) upsert filter. Retire the matching legacy token first so
	// re-issuing never leaves two active tokens for one purpose. The legacy
	// document remains readable for audit, but its token is no longer active.
	if _, err := Tokens.coll.UpdateOne(ctx, bson.M{
		"_id":        userID,
		"UserId":     bson.M{"$exists": false},
		"Type":       tokenType,
		"ConsumedAt": bson.M{"$exists": false},
	}, bson.M{"$set": bson.M{"ConsumedAt": createdAt}}); err != nil {
		return ActionToken{}, fmt.Errorf("retire legacy action token: %w", err)
	}
	filter := bson.M{"UserId": userID, "Type": tokenType}
	update := bson.M{
		"$set": bson.M{
			"UserId":      token.UserID,
			"Email":       token.Email,
			"Token":       token.Token,
			"Type":        token.Type,
			"CreatedTime": token.CreatedTime,
		},
		"$unset":       bson.M{"ConsumedAt": ""},
		"$setOnInsert": bson.M{"_id": token.ID},
	}
	var raw bson.M
	err := Tokens.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&raw)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// A concurrent issuer won the unique (UserId,Type) race. Read its
			// committed document and return it instead of surfacing a spurious
			// duplicate to the caller.
			if readErr := Tokens.coll.FindOne(ctx, filter).Decode(&raw); readErr == nil {
				return DecodeActionToken(raw)
			}
		}
		return ActionToken{}, fmt.Errorf("issue action token: %w", err)
	}
	return DecodeActionToken(raw)
}

// ResolveActionToken requires the expected purpose and leaves expired tokens
// untouched so a caller can report the correct failure without consuming it.
func ResolveActionToken(parent context.Context, value string, tokenType int, now time.Time, ttl time.Duration) (ActionToken, error) {
	if Tokens == nil {
		return ActionToken{}, ErrMongoClientNotInitialized
	}
	if value == "" {
		return ActionToken{}, mongo.ErrNoDocuments
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	err := Tokens.coll.FindOne(ctx, bson.M{
		"Token":      value,
		"Type":       tokenType,
		"ConsumedAt": bson.M{"$exists": false},
	}).Decode(&raw)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			var untyped bson.M
			typeErr := Tokens.coll.FindOne(ctx, bson.M{
				"Token":      value,
				"ConsumedAt": bson.M{"$exists": false},
			}).Decode(&untyped)
			if typeErr == nil {
				if actualType, typeErr := intValue(untyped["Type"]); typeErr == nil && actualType != tokenType {
					return ActionToken{}, ErrTokenTypeMismatch
				}
			} else if !errors.Is(typeErr, mongo.ErrNoDocuments) {
				return ActionToken{}, fmt.Errorf("probe action token type: %w", typeErr)
			}
		}
		return ActionToken{}, err
	}
	token, err := DecodeActionToken(raw)
	if err != nil {
		return ActionToken{}, err
	}
	if ActionTokenExpired(token.CreatedTime, now, ttl) {
		return token, ErrTokenExpired
	}
	return token, nil
}

// ConsumeActionToken atomically marks one matching token as used. A second
// consumer receives mongo.ErrNoDocuments and cannot reuse the token.
func ConsumeActionToken(parent context.Context, value string, tokenType int, consumedAt time.Time) (ActionToken, error) {
	if Tokens == nil {
		return ActionToken{}, ErrMongoClientNotInitialized
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	var raw bson.M
	err := Tokens.coll.FindOneAndUpdate(ctx, bson.M{
		"Token":      value,
		"Type":       tokenType,
		"ConsumedAt": bson.M{"$exists": false},
	}, bson.M{"$set": bson.M{"ConsumedAt": consumedAt}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&raw)
	if err != nil {
		return ActionToken{}, err
	}
	return DecodeActionToken(raw)
}
