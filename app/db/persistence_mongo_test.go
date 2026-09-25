package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func persistenceTestDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	_, raw := testCollection(t)
	return raw.Database()
}

func TestPersistenceIndexesCreateOnMongo(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	for name := range PersistenceIndexModels() {
		if err := databaseRef.Collection(name).Drop(context.Background()); err != nil {
			t.Fatalf("drop %s: %v", name, err)
		}
	}

	saved := database
	database = databaseRef
	defer func() { database = saved }()
	if err := EnsurePersistenceIndexes(context.Background()); err != nil {
		t.Fatalf("EnsurePersistenceIndexes: %v", err)
	}

	for collectionName, models := range PersistenceIndexModels() {
		cursor, err := databaseRef.Collection(collectionName).Indexes().List(context.Background())
		if err != nil {
			t.Fatalf("list %s indexes: %v", collectionName, err)
		}
		var indexes []bson.M
		if err := cursor.All(context.Background(), &indexes); err != nil {
			t.Fatalf("decode %s indexes: %v", collectionName, err)
		}
		for _, model := range models {
			want := *indexOptions(model).Name
			found := false
			for _, index := range indexes {
				if index["name"] == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s index %q was not created; indexes=%v", collectionName, want, indexes)
			}
		}
	}
}

func TestPersistenceIndexesRejectHistoricalDuplicatesBeforeCreate(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	for name := range PersistenceIndexModels() {
		if err := databaseRef.Collection(name).Drop(context.Background()); err != nil {
			t.Fatalf("drop %s: %v", name, err)
		}
	}
	if _, err := databaseRef.Collection("users").InsertMany(context.Background(), []interface{}{
		bson.M{"_id": bson.NewObjectID(), "Email": "duplicate@example.test"},
		bson.M{"_id": bson.NewObjectID(), "Email": "duplicate@example.test"},
	}); err != nil {
		t.Fatalf("seed duplicate users: %v", err)
	}

	saved := database
	database = databaseRef
	defer func() { database = saved }()
	err := EnsurePersistenceIndexes(context.Background())
	var conflict *PersistenceIndexConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("EnsurePersistenceIndexes error=%v, want PersistenceIndexConflictError", err)
	}
	if conflict.Collection != "users" || conflict.Index != "users_Email_unique" || conflict.Count != 1 {
		t.Fatalf("index conflict=%+v, want users email one duplicate group", conflict)
	}
}

func TestSharePreflightRejectsDocumentsOutsideInstalledValidator(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	saved := database
	database = databaseRef
	t.Cleanup(func() { database = saved })
	for _, name := range []string{"share_notes", "share_notebooks"} {
		if err := databaseRef.Collection(name).Drop(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	collection := databaseRef.Collection("share_notes")
	valid := func() bson.M {
		return bson.M{
			"_id": bson.NewObjectID(), "UserId": bson.NewObjectID(),
			"NoteId": bson.NewObjectID(), "ToUserId": bson.NewObjectID(), "Perm": int32(1),
		}
	}
	tests := []struct {
		name   string
		change func(bson.M)
	}{
		{"missing owner", func(doc bson.M) { delete(doc, "UserId") }},
		{"zero owner", func(doc bson.M) { doc["UserId"] = bson.ObjectID{} }},
		{"wrong owner type", func(doc bson.M) { doc["UserId"] = "owner" }},
		{"missing resource", func(doc bson.M) { delete(doc, "NoteId") }},
		{"zero resource", func(doc bson.M) { doc["NoteId"] = bson.ObjectID{} }},
		{"wrong resource type", func(doc bson.M) { doc["NoteId"] = "note" }},
		{"missing permission", func(doc bson.M) { delete(doc, "Perm") }},
		{"wrong permission value", func(doc bson.M) { doc["Perm"] = int32(2) }},
		{"wrong permission type", func(doc bson.M) { doc["Perm"] = "write" }},
		{"two recipients", func(doc bson.M) { doc["ToGroupId"] = bson.NewObjectID() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := collection.Drop(context.Background()); err != nil {
				t.Fatal(err)
			}
			doc := valid()
			test.change(doc)
			if _, err := collection.InsertOne(context.Background(), doc); err != nil {
				t.Fatal(err)
			}
			var conflict *ShareSchemaConflictError
			if err := preflightShareDocuments(context.Background()); !errors.As(err, &conflict) || conflict.Collection != "share_notes" || conflict.Count != 1 {
				t.Fatalf("preflight conflict=%+v err=%v", conflict, err)
			}
		})
	}
}

func TestCustomDomainPreflightRejectsCanonicalCollisions(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	saved := database
	database = databaseRef
	t.Cleanup(func() { database = saved })
	collection := databaseRef.Collection("user_blogs")
	for _, test := range []struct {
		name    string
		domains []any
	}{
		{name: "case and trailing dot", domains: []any{"Example.com.", "example.com"}},
		{name: "unicode and punycode", domains: []any{"bücher.de", "xn--bcher-kva.de"}},
		{name: "nonstring", domains: []any{int32(42)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := collection.Drop(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, value := range test.domains {
				if _, err := collection.InsertOne(context.Background(), bson.M{"_id": bson.NewObjectID(), "Domain": value}); err != nil {
					t.Fatal(err)
				}
			}
			var conflict *CustomDomainPreflightError
			wantDuplicateGroups := int64(1)
			if test.name == "nonstring" {
				wantDuplicateGroups = 0
			}
			if err := preflightCustomDomains(context.Background()); !errors.As(err, &conflict) || conflict.InvalidCount != 1 || conflict.DuplicateGroups != wantDuplicateGroups || len(conflict.KeyDigests) == 0 {
				t.Fatalf("custom domain preflight conflict=%+v err=%v", conflict, err)
			}
		})
	}
}

func TestActionTokenMongoReplaceAndConsume(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_tokens")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop tokens: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["tokens"][0]); err != nil {
		t.Fatalf("create token index: %v", err)
	}

	saved := Tokens
	Tokens = wrapCollection(collection)
	defer func() { Tokens = saved }()

	userID := MustObjectIDFromHex("507f1f77bcf86cd799439011")
	created := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	one, err := IssueActionToken(context.Background(), userID, "user@example.test", "first", 0, created)
	if err != nil {
		t.Fatalf("issue first token: %v", err)
	}
	two, err := IssueActionToken(context.Background(), userID, "user@example.test", "second", 0, created.Add(time.Minute))
	if err != nil {
		t.Fatalf("replace token: %v", err)
	}
	if two.ID.IsZero() || two.UserID != userID || two.Token != "second" {
		t.Fatalf("replacement token = %+v", two)
	}
	if two.Email != "user@example.test" {
		t.Fatalf("replacement token email=%q, want issued email", two.Email)
	}
	if one.ID != two.ID {
		t.Fatalf("replacement should atomically update one active document: first=%s second=%s", one.ID.Hex(), two.ID.Hex())
	}

	resolved, err := ResolveActionToken(context.Background(), "second", 0, created.Add(time.Minute), 2*time.Hour)
	if err != nil || resolved.Token != "second" || resolved.Email != "user@example.test" {
		t.Fatalf("resolve replacement token = %+v, err=%v", resolved, err)
	}
	if _, err := ResolveActionToken(context.Background(), "second", 1, created.Add(time.Minute), 2*time.Hour); !errors.Is(err, ErrTokenTypeMismatch) {
		t.Fatalf("wrong token type err=%v, want ErrTokenTypeMismatch", err)
	}
	if _, err := ConsumeValidActionToken(context.Background(), "second", 0, two.CreatedTime.Add(2*time.Hour), 2*time.Hour); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("consume at TTL boundary err=%v, want mongo.ErrNoDocuments", err)
	}
	if _, err := ConsumeValidActionToken(context.Background(), "second", 0, created.Add(2*time.Minute), 2*time.Hour); err != nil {
		t.Fatalf("consume token: %v", err)
	}
	if _, err := ConsumeValidActionToken(context.Background(), "second", 0, created.Add(3*time.Minute), 2*time.Hour); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("second consume err=%v, want mongo.ErrNoDocuments", err)
	}
}

func TestActionTokenMongoReissueRetiresLegacyDocument(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_tokens_legacy_reissue")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop legacy tokens: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["tokens"][0]); err != nil {
		t.Fatalf("create token index: %v", err)
	}
	saved := Tokens
	Tokens = wrapCollection(collection)
	defer func() { Tokens = saved }()

	userID := MustObjectIDFromHex("507f1f77bcf86cd799439014")
	created := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	if _, err := collection.InsertOne(context.Background(), bson.M{
		"_id":         bson.ObjectID(userID),
		"Email":       "legacy@example.test",
		"Token":       "legacy-secret",
		"Type":        0,
		"CreatedTime": created,
	}); err != nil {
		t.Fatalf("insert legacy token: %v", err)
	}

	issued, err := IssueActionToken(context.Background(), userID, "legacy@example.test", "fresh-secret", 0, created.Add(time.Minute))
	if err != nil {
		t.Fatalf("reissue legacy token: %v", err)
	}
	if issued.ID == userID || issued.Legacy || issued.Token != "fresh-secret" {
		t.Fatalf("reissued token = %+v, want independent active document", issued)
	}
	count, err := collection.CountDocuments(context.Background(), bson.M{
		"UserId": userID, "Type": 0, "ConsumedAt": bson.M{"$exists": false},
	})
	if err != nil || count != 1 {
		t.Fatalf("active token count=%d err=%v, want one", count, err)
	}
	if _, err := ResolveActionToken(context.Background(), "legacy-secret", 0, created.Add(time.Minute), 2*time.Hour); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("retired legacy token resolve err=%v, want mongo.ErrNoDocuments", err)
	}
}

func TestActionTokenMongoConcurrentIssueKeepsOneActiveDocument(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_tokens_concurrent")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop concurrent tokens: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["tokens"][0]); err != nil {
		t.Fatalf("create concurrent token index: %v", err)
	}
	saved := Tokens
	Tokens = wrapCollection(collection)
	defer func() { Tokens = saved }()

	userID := MustObjectIDFromHex("507f1f77bcf86cd799439013")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := IssueActionToken(context.Background(), userID, "user@example.test", fmt.Sprintf("concurrent-%d", i), 0, time.Now())
			errs <- err
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		err := <-errs
		if err != nil {
			t.Fatalf("concurrent issue error: %v", err)
		}
	}
	count, err := collection.CountDocuments(context.Background(), bson.M{"UserId": userID, "Type": 0})
	if err != nil || count != 1 {
		t.Fatalf("active token count=%d err=%v, want one", count, err)
	}
}

func TestSessionMongoRefreshRejectsInclusiveExpiry(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_sessions")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}
	saved := Sessions
	Sessions = wrapCollection(collection)
	defer func() { Sessions = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	token := "api-session-token"
	session := info.Session{Id: NewObjectID(), SessionId: SessionTokenDigest(token), UserId: "user-1", CreatedTime: now.Add(-time.Hour), UpdatedTime: now.Add(-time.Hour)}
	if err := Sessions.Insert(session); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	got, err := ResolveSessionAndRefresh(context.Background(), token, now)
	if err != nil || !got.UpdatedTime.Equal(now) {
		t.Fatalf("refresh valid session = %+v, err=%v", got, err)
	}

	if err := Sessions.Update(bson.M{"SessionId": session.SessionId}, bson.M{"$set": bson.M{"UpdatedTime": now.Add(-SessionIdleTTL)}}); err != nil {
		t.Fatalf("age session to boundary: %v", err)
	}
	if _, err := ResolveSessionAndRefresh(context.Background(), token, now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session err=%v, want ErrSessionNotFound", err)
	}
}

func TestSessionMongoResolveAndDeleteUseDigestThenLegacyFallback(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_sessions_token_lookup")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}
	saved := Sessions
	Sessions = wrapCollection(collection)
	defer func() { Sessions = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rawToken := "api-session-token"
	legacyToken := "507f1f77bcf86cd799439011"
	for _, session := range []info.Session{
		{Id: NewObjectID(), SessionId: SessionTokenDigest(rawToken), UserId: "digest-user", CreatedTime: now, UpdatedTime: now},
		{Id: NewObjectID(), SessionId: legacyToken, UserId: "legacy-user", CreatedTime: now, UpdatedTime: now},
	} {
		if err := Sessions.Insert(session); err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	got, err := ResolveSessionAndRefresh(context.Background(), rawToken, now.Add(time.Minute))
	if err != nil || got.UserId != "digest-user" {
		t.Fatalf("resolve digest token=%+v err=%v", got, err)
	}
	got, err = ResolveSessionAndRefresh(context.Background(), legacyToken, now.Add(2*time.Minute))
	if err != nil || got.UserId != "legacy-user" {
		t.Fatalf("resolve legacy token=%+v err=%v", got, err)
	}
	deleted, err := DeleteSessionForToken(context.Background(), legacyToken)
	if err != nil || !deleted {
		t.Fatalf("delete legacy token deleted=%v err=%v", deleted, err)
	}
	if _, err := ResolveSessionAndRefresh(context.Background(), legacyToken, now.Add(3*time.Minute)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("legacy token remained resolvable: %v", err)
	}
}

func TestUserDuplicateEmailMapsToStableIdentityError(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_users")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop users: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["users"][0]); err != nil {
		t.Fatalf("create user index: %v", err)
	}
	first := bson.M{"_id": bson.NewObjectID(), "Email": "same@example.test"}
	second := bson.M{"_id": bson.NewObjectID(), "Email": "same@example.test"}
	if _, err := collection.InsertOne(context.Background(), first); err != nil {
		t.Fatalf("insert first user: %v", err)
	}
	_, err := collection.InsertOne(context.Background(), second)
	if err == nil || !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate email err=%v, want duplicate key", err)
	}
	mapped := MapDuplicateIdentityError(err)
	if !IsDuplicateIdentityError(mapped) || DuplicateIdentityField(err) != "email" {
		t.Fatalf("mapped duplicate = %v, field=%q", mapped, DuplicateIdentityField(err))
	}
}

func TestUserConcurrentDuplicateEmailHasOneWinner(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_users_concurrent")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop concurrent users: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["users"][0]); err != nil {
		t.Fatalf("create concurrent user index: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := collection.InsertOne(context.Background(), bson.M{"_id": bson.NewObjectID(), "Email": "race@example.test"})
			errs <- err
		}()
	}
	wg.Wait()
	winners := 0
	duplicates := 0
	for i := 0; i < 8; i++ {
		if err := <-errs; err == nil {
			winners++
		} else if mongo.IsDuplicateKeyError(err) && IsDuplicateIdentityError(MapDuplicateIdentityError(err)) {
			duplicates++
		} else {
			t.Fatalf("unexpected concurrent insert error: %v", err)
		}
	}
	if winners != 1 || duplicates != 7 {
		t.Fatalf("concurrent duplicate results: winners=%d duplicates=%d", winners, duplicates)
	}
}

func TestOutboxMongoRetryAndSuccess(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_outbox")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	saved := Outbox
	Outbox = wrapCollection(collection)
	defer func() { Outbox = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	event := OutboxEvent{ID: NewObjectID(), Kind: "activate-email", AggregateID: NewObjectID(), NextAttemptAt: now}
	if _, err := EnqueueOutboxEvent(context.Background(), event); err != nil {
		t.Fatalf("enqueue outbox: %v", err)
	}
	if err := DeliverOutbox(context.Background(), event.ID, now, func(context.Context, OutboxEvent) error {
		return errors.New("transport unavailable")
	}); !errors.Is(err, ErrSideEffect) {
		t.Fatalf("failed delivery err=%v, want ErrSideEffect", err)
	}

	var retry OutboxEvent
	if err := Outbox.Find(bson.M{"_id": event.ID}).One(&retry); err != nil {
		t.Fatalf("read retry state: %v", err)
	}
	if retry.Status != OutboxStatusRetry || retry.Attempts != 1 || retry.LastError == "" {
		t.Fatalf("retry state = %+v", retry)
	}
	if err := DeliverOutbox(context.Background(), event.ID, retry.NextAttemptAt, func(context.Context, OutboxEvent) error { return nil }); err != nil {
		t.Fatalf("successful retry: %v", err)
	}
	if err := Outbox.Find(bson.M{"_id": event.ID}).One(&retry); err != nil {
		t.Fatalf("read sent state: %v", err)
	}
	if retry.Status != OutboxStatusSent || retry.Attempts != 2 {
		t.Fatalf("sent state = %+v", retry)
	}
}

func TestCommentOutboxMongoUsesHandoffGate(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_comment_outbox_handoff")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved := Outbox
	Outbox = wrapCollection(collection)
	defer func() { Outbox = saved }()

	now := time.Now().UTC().Truncate(time.Second)
	event := OutboxEvent{ID: NewObjectID(), Kind: "comment", AggregateID: NewObjectID(), NextAttemptAt: now}
	if _, err := EnqueueOutboxEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmCommentOutbox(context.Background(), event.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := DeliverOutbox(context.Background(), event.ID, now, func(_ context.Context, claimed OutboxEvent) error {
		if claimed.Status != OutboxStatusHandedOff || claimed.TransportHandedOffAt.IsZero() {
			return fmt.Errorf("comment reached transport without handoff gate: %+v", claimed)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stored OutboxEvent
	if err := Outbox.FindId(event.ID).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status != OutboxStatusSent || stored.TransportHandedOffAt.IsZero() {
		t.Fatalf("comment handoff state = %+v", stored)
	}
	rejected := OutboxEvent{ID: NewObjectID(), Kind: "comment", AggregateID: NewObjectID(), NextAttemptAt: now}
	if _, err := EnqueueOutboxEvent(context.Background(), rejected); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmCommentOutbox(context.Background(), rejected.ID, now); err != nil {
		t.Fatal(err)
	}
	secret := "private SMTP recipient@example.test"
	err := DeliverOutbox(context.Background(), rejected.ID, now, func(context.Context, OutboxEvent) error {
		return fmt.Errorf("%w: %s", ErrOutboxTransportRejected, secret)
	})
	if !errors.Is(err, ErrSideEffect) || strings.Contains(err.Error(), secret) {
		t.Fatalf("definite rejection error = %v", err)
	}
	if err := Outbox.FindId(rejected.ID).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status != OutboxStatusRetry || stored.LastError != "transport_rejected" || !stored.TransportHandedOffAt.IsZero() || stored.Attempts != 1 {
		t.Fatalf("definite rejection state = %+v", stored)
	}
	if err := CancelOutboxForAggregate(context.Background(), rejected.AggregateID, "comment deleted"); err != nil {
		t.Fatalf("rejected notification was not cancellable: %v", err)
	}
}

func TestOutboxMongoIdempotencyLeaseAndIndependentFailureUpdate(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_outbox_lease")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), PersistenceIndexModels()["outbox"][1]); err != nil {
		t.Fatalf("create outbox idempotency index: %v", err)
	}
	saved := Outbox
	Outbox = wrapCollection(collection)
	defer func() { Outbox = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	event := OutboxEvent{
		IdempotencyKey: "register:507f1f77bcf86cd799439015",
		Kind:           "activate-email",
		AggregateID:    MustObjectIDFromHex("507f1f77bcf86cd799439015"),
		NextAttemptAt:  now,
	}
	first, err := EnqueueOutboxEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	second, err := EnqueueOutboxEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent enqueue IDs differ: %s and %s", first.ID.Hex(), second.ID.Hex())
	}
	if count, err := collection.CountDocuments(context.Background(), bson.M{}); err != nil || count != 1 {
		t.Fatalf("outbox count=%d err=%v, want one", count, err)
	}
	stableID := NewObjectID()
	stable := OutboxEvent{ID: stableID, Kind: "reset-password", AggregateID: first.AggregateID, NextAttemptAt: now}
	if _, err := EnqueueOutboxEvent(context.Background(), stable); err != nil {
		t.Fatalf("enqueue explicit-id event: %v", err)
	}
	recovered, err := EnqueueOutboxEvent(context.Background(), stable)
	if err != nil || recovered.ID != stableID {
		t.Fatalf("explicit-id retry = %+v err=%v, want existing event", recovered, err)
	}
	if count, err := collection.CountDocuments(context.Background(), bson.M{}); err != nil || count != 2 {
		t.Fatalf("outbox count after explicit-id retry=%d err=%v, want two distinct events", count, err)
	}

	if _, err := collection.UpdateOne(context.Background(), bson.M{"_id": first.ID}, bson.M{"$set": bson.M{
		"Status": OutboxStatusSending, "LeaseUntil": now.Add(-time.Second), "UpdatedAt": now.Add(-OutboxLeaseTTL),
	}}); err != nil {
		t.Fatalf("seed expired lease: %v", err)
	}
	parent, cancelParent := context.WithCancel(context.Background())
	err = DeliverOutbox(parent, first.ID, now, func(context.Context, OutboxEvent) error {
		cancelParent()
		return errors.New("transport unavailable")
	})
	if !errors.Is(err, ErrSideEffect) {
		t.Fatalf("delivery error=%v, want ErrSideEffect", err)
	}
	var retry OutboxEvent
	if err := collection.FindOne(context.Background(), bson.M{"_id": first.ID}).Decode(&retry); err != nil {
		t.Fatalf("read retry state: %v", err)
	}
	if retry.Status != OutboxStatusRetry || retry.LeaseUntil != (time.Time{}) || retry.LastError == "" {
		t.Fatalf("state after expired lease and canceled transport = %+v, want retry without lease", retry)
	}
}

func TestOutboxMongoDoesNotOverwriteAReclaimedLease(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_outbox_reclaimed_lease")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	saved := Outbox
	Outbox = wrapCollection(collection)
	defer func() { Outbox = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	event := OutboxEvent{ID: NewObjectID(), Kind: "activate-email", AggregateID: NewObjectID(), NextAttemptAt: now}
	if _, err := EnqueueOutboxEvent(context.Background(), event); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	err := DeliverOutbox(context.Background(), event.ID, now, func(context.Context, OutboxEvent) error {
		_, updateErr := collection.UpdateOne(context.Background(), bson.M{"_id": event.ID}, bson.M{
			"$set": bson.M{"LeaseId": "reclaimed-by-another-worker", "LeaseUntil": now.Add(OutboxLeaseTTL)},
		})
		return updateErr
	})
	if !errors.Is(err, ErrSideEffect) {
		t.Fatalf("delivery error=%v, want side-effect failure after lease loss", err)
	}
	var got OutboxEvent
	if err := collection.FindOne(context.Background(), bson.M{"_id": event.ID}).Decode(&got); err != nil {
		t.Fatalf("read sending event: %v", err)
	}
	if got.Status != OutboxStatusSending || got.LeaseID != "reclaimed-by-another-worker" {
		t.Fatalf("stale worker overwrote reclaimed state: %+v", got)
	}
}

func TestOutboxMongoMalformedEventPreservesAttemptLimit(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_outbox_malformed")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	saved := Outbox
	Outbox = wrapCollection(collection)
	defer func() { Outbox = saved }()

	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	eventID := NewObjectID()
	if _, err := collection.InsertOne(context.Background(), bson.M{
		"_id": eventID, "Kind": "activate-email", "AggregateId": NewObjectID(),
		"Payload": "malformed-payload", "Status": OutboxStatusPending, "Attempts": OutboxMaxAttempts - 1,
		"NextAttemptAt": now, "CreatedAt": now, "UpdatedAt": now,
	}); err != nil {
		t.Fatalf("insert malformed outbox event: %v", err)
	}
	err := DeliverOutbox(context.Background(), eventID, now, func(context.Context, OutboxEvent) error {
		t.Fatal("malformed event must not reach transport")
		return nil
	})
	if err == nil {
		t.Fatal("malformed outbox event must fail")
	}
	var got bson.M
	if err := collection.FindOne(context.Background(), bson.M{"_id": eventID}).Decode(&got); err != nil {
		t.Fatalf("read malformed event state: %v", err)
	}
	attempts, attemptErr := intValue(got["Attempts"])
	_, hasLease := got["LeaseId"]
	if got["Status"] != OutboxStatusDead || attemptErr != nil || attempts != OutboxMaxAttempts || hasLease {
		t.Fatalf("malformed event state=%+v, want dead at max attempts without lease", got)
	}
}

func TestStandaloneMongoInitializationFallsBackWithoutFalseCommit(t *testing.T) {
	databaseRef := persistenceTestDatabase(t)
	collection := databaseRef.Collection("persistence_initialization")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatalf("drop initialization collection: %v", err)
	}
	savedClient, savedDatabase, savedUsers := client, database, Users
	client, database, Users = databaseRef.Client(), databaseRef, wrapCollection(collection)
	defer func() { client, database, Users = savedClient, savedDatabase, savedUsers }()

	userID := MustObjectIDFromHex("507f1f77bcf86cd799439012")
	plan := UserInitializationPlan{
		UserID: userID,
		Steps: []InitializationStep{
			{
				Name: "user",
				Apply: func(ctx context.Context) error {
					return Users.InsertContext(ctx, bson.M{"_id": userID, "Email": "standalone@example.test"})
				},
				Compensate: func(ctx context.Context) error {
					return Users.RemoveContext(ctx, bson.M{"_id": userID})
				},
			},
			{Name: "defaults", Apply: func(context.Context) error { return errors.New("fixture initialization failure") }},
		},
	}

	result, err := RunUserInitialization(context.Background(), plan)
	if !errors.Is(err, ErrPartialWrite) || !result.PartialWrite || result.Mode != "compensation" {
		t.Fatalf("standalone result = %+v, err=%v", result, err)
	}
	if count, countErr := Users.Find(bson.M{"_id": userID}).Count(); countErr != nil || count != 0 {
		t.Fatalf("compensation left user document count=%d err=%v", count, countErr)
	}
}

func TestReplicaSetMongoInitializationCommitsAtomically(t *testing.T) {
	if os.Getenv("LEANOTE_DB_TEST_TRANSACTIONS") != "1" {
		t.Skip("set LEANOTE_DB_TEST_TRANSACTIONS=1 with a replica-set MongoDB fixture")
	}
	databaseRef := persistenceTestDatabase(t)
	usersCollection := databaseRef.Collection("persistence_transaction_users")
	outboxCollection := databaseRef.Collection("persistence_transaction_outbox")
	if err := usersCollection.Drop(context.Background()); err != nil {
		t.Fatalf("drop users: %v", err)
	}
	if err := outboxCollection.Drop(context.Background()); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	savedClient, savedUsers, savedOutbox := client, Users, Outbox
	client, Users, Outbox = databaseRef.Client(), wrapCollection(usersCollection), wrapCollection(outboxCollection)
	defer func() { client, Users, Outbox = savedClient, savedUsers, savedOutbox }()

	userID := MustObjectIDFromHex("507f1f77bcf86cd799439016")
	result, err := RunUserInitialization(context.Background(), UserInitializationPlan{
		UserID: userID,
		Steps: []InitializationStep{{
			Name: "user",
			Apply: func(ctx context.Context) error {
				return Users.InsertContext(ctx, bson.M{"_id": userID, "Email": "transaction@example.test"})
			},
		}},
		Outbox: &OutboxEvent{
			IdempotencyKey: "register:" + userID.Hex(),
			Kind:           "activate-email",
			AggregateID:    userID,
		},
	})
	if err != nil || !result.Committed || result.Mode != "transaction" {
		t.Fatalf("transaction result=%+v err=%v, want committed transaction", result, err)
	}
	if count, err := usersCollection.CountDocuments(context.Background(), bson.M{"_id": userID}); err != nil || count != 1 {
		t.Fatalf("transaction user count=%d err=%v, want one", count, err)
	}
	if count, err := outboxCollection.CountDocuments(context.Background(), bson.M{"AggregateId": userID}); err != nil || count != 1 {
		t.Fatalf("transaction outbox count=%d err=%v, want one", count, err)
	}
}
