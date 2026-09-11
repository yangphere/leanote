package db

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func indexKeys(model mongo.IndexModel) []string {
	keys, ok := model.Keys.(bson.D)
	if !ok {
		return nil
	}
	got := make([]string, 0, len(keys))
	for _, key := range keys {
		got = append(got, key.Key)
	}
	return got
}

func indexOptions(model mongo.IndexModel) options.IndexOptions {
	var got options.IndexOptions
	if model.Options == nil {
		return got
	}
	for _, apply := range model.Options.List() {
		if err := apply(&got); err != nil {
			panic(err)
		}
	}
	return got
}

func hasUniqueIndex(models []mongo.IndexModel, want ...string) bool {
	for _, model := range models {
		if got := indexKeys(model); len(got) != len(want) {
			continue
		} else {
			match := true
			for i := range want {
				if got[i] != want[i] {
					match = false
					break
				}
			}
			if match && indexOptions(model).Unique != nil && *indexOptions(model).Unique {
				return true
			}
		}
	}
	return false
}

func hasIndex(models []mongo.IndexModel, field string, ttl int32) bool {
	for _, model := range models {
		if got := indexKeys(model); len(got) != 1 || got[0] != field {
			continue
		}
		opts := indexOptions(model)
		if opts.ExpireAfterSeconds != nil && *opts.ExpireAfterSeconds == ttl {
			return true
		}
	}
	return false
}

func hasIndexKeys(models []mongo.IndexModel, want ...string) bool {
	for _, model := range models {
		got := indexKeys(model)
		if len(got) != len(want) {
			continue
		}
		match := true
		for i := range want {
			if got[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestPersistenceIndexModelsDeclareRequiredBoundaries(t *testing.T) {
	models := PersistenceIndexModels()

	if !hasIndex(models["sessions"], "UpdatedTime", 10800) {
		t.Fatal("sessions must declare a 10800-second UpdatedTime TTL index")
	}
	if !hasUniqueIndex(models["sessions"], "SessionId") {
		t.Fatal("sessions must declare a unique SessionId index for atomic upsert")
	}
	if !hasUniqueIndex(models["tokens"], "UserId", "Type") {
		t.Fatal("tokens must declare a unique (UserId,Type) index")
	}
	if !hasUniqueIndex(models["users"], "Email") || !hasUniqueIndex(models["users"], "Username") {
		t.Fatal("users must declare unique Email and Username indexes")
	}
	if !hasUniqueIndex(models["users"], "ThirdType", "ThirdUserId") {
		t.Fatal("users must declare a unique third-party identity index")
	}
	if !hasIndexKeys(models["outbox"], "Status", "NextAttemptAt") {
		t.Fatal("outbox must declare a worker-ready status/time index")
	}
	if !hasUniqueIndex(models["outbox"], "IdempotencyKey") {
		t.Fatal("outbox must declare a unique idempotency-key index")
	}
}

func TestSessionExpiredUsesInclusiveThreeHourBoundary(t *testing.T) {
	const sessionTTL = 3 * time.Hour
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	if !SessionExpired(now.Add(-sessionTTL), now) {
		t.Fatal("session at exactly the TTL boundary must be expired")
	}
	if SessionExpired(now.Add(-sessionTTL).Add(time.Nanosecond), now) {
		t.Fatal("session newer than the TTL boundary must remain valid")
	}
	if !SessionExpired(time.Time{}, now) {
		t.Fatal("zero UpdatedTime must fail closed as an expired persisted session")
	}
}

func TestSessionFieldUpsertDoesNotUpdateSamePathTwice(t *testing.T) {
	now := time.Date(2026, 9, 11, 15, 0, 0, 0, time.UTC)
	update := sessionFieldUpsert("anonymous", bson.M{"Captcha": "answer"}, now)
	set := update["$set"].(bson.M)
	insert := update["$setOnInsert"].(bson.M)
	for key := range set {
		if _, duplicated := insert[key]; duplicated {
			t.Fatalf("session upsert updates %q in both $set and $setOnInsert: %#v", key, update)
		}
	}
}

func TestAPISessionTokenUsesRandomBase64URLAndDigestOnlyStorageForm(t *testing.T) {
	token, err := NewAPISessionToken()
	if err != nil {
		t.Fatalf("NewAPISessionToken: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != apiSessionTokenBytes {
		t.Fatalf("token=%q decoded=%d err=%v, want %d random bytes", token, len(raw), err, apiSessionTokenBytes)
	}
	digest := SessionTokenDigest(token)
	if digest == token || len(digest) != 64 {
		t.Fatalf("digest=%q must be a SHA-256 hex value distinct from the raw token", digest)
	}
	filters := sessionTokenFilters(token)
	if len(filters) != 1 || filters[0]["SessionId"] != digest {
		t.Fatalf("new token filters=%#v, want digest-only lookup", filters)
	}
}

func TestSessionTokenFiltersAllowOnlyStrictLegacyObjectIDs(t *testing.T) {
	legacy := "507f1f77bcf86cd799439011"
	filters := sessionTokenFilters(legacy)
	if len(filters) != 2 || filters[0]["SessionId"] != SessionTokenDigest(legacy) || filters[1]["SessionId"] != legacy {
		t.Fatalf("legacy filters=%#v, want digest then exact legacy ObjectID", filters)
	}
	filters = sessionTokenFilters("not-an-object-id")
	if len(filters) != 1 {
		t.Fatalf("non-legacy filters=%#v, want no plaintext fallback", filters)
	}
}

func TestActionTokenIssueCreatesIndependentIDAndExpiresInclusively(t *testing.T) {
	userID := domain.ObjectID{1}
	created := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	token := NewActionToken(userID, "secret", 0, created)

	if token.ID.IsZero() {
		t.Fatal("new action tokens must have an independent document ID")
	}
	if token.UserID != userID || token.Token != "secret" {
		t.Fatalf("new action token lost identity fields: %+v", token)
	}
	if !ActionTokenExpired(created, created.Add(2*time.Hour), 2*time.Hour) {
		t.Fatal("action token must expire at the inclusive boundary")
	}
	if ActionTokenExpired(created, created.Add(2*time.Hour-time.Nanosecond), 2*time.Hour) {
		t.Fatal("action token must remain valid just before expiry")
	}
}

func TestActionTokenConsumeFilterRejectsInclusiveTTLBoundary(t *testing.T) {
	consumedAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	ttl := 2 * time.Hour
	filter := actionTokenConsumeFilter("secret", 7, consumedAt, ttl)

	created, ok := filter["CreatedTime"].(bson.M)
	if !ok {
		t.Fatalf("CreatedTime filter = %#v, want BSON comparison", filter["CreatedTime"])
	}
	if len(created) != 1 || created["$gt"] != consumedAt.Add(-ttl) {
		t.Fatalf("CreatedTime filter = %#v, want strict $gt cutoff %s", created, consumedAt.Add(-ttl))
	}
	if filter["Token"] != "secret" || filter["Type"] != 7 {
		t.Fatalf("consume identity filter = %#v", filter)
	}
}

func TestDecodeLegacyActionTokenIsReadOnlyCompatible(t *testing.T) {
	userID := MustObjectIDFromHex("507f1f77bcf86cd799439011")
	raw := bson.M{
		"_id":         bson.ObjectID(userID),
		"Email":       "user@example.test",
		"Token":       "legacy-secret",
		"Type":        0,
		"CreatedTime": time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}

	token, err := DecodeActionToken(raw)
	if err != nil {
		t.Fatalf("DecodeActionToken: %v", err)
	}
	if !token.Legacy || token.UserID != userID || token.ID != userID {
		t.Fatalf("legacy token was not preserved as read-only compatible: %+v", token)
	}
}

func TestOutboxDeliveryFailureIsRetryableAndObservable(t *testing.T) {
	var delivered int
	store := NewMemoryOutboxStore()
	event := OutboxEvent{ID: NewObjectID(), Kind: "activate-email", AggregateID: domain.ObjectID{1}, NextAttemptAt: time.Unix(100, 0)}
	if err := store.Enqueue(context.Background(), event); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	err := store.Deliver(context.Background(), event.ID, time.Unix(100, 0), func(context.Context, OutboxEvent) error {
		delivered++
		return errors.New("transport unavailable")
	})
	if !errors.Is(err, ErrSideEffect) {
		t.Fatalf("Deliver error = %v, want ErrSideEffect", err)
	}
	if delivered != 1 {
		t.Fatalf("transport calls = %d, want 1", delivered)
	}
	got, ok := store.Get(event.ID)
	if !ok || got.Status != OutboxStatusRetry || got.Attempts != 1 || got.LastError == "" {
		t.Fatalf("failed delivery state = %+v, want retryable observable state", got)
	}

	if err := store.Deliver(context.Background(), event.ID, got.NextAttemptAt, func(context.Context, OutboxEvent) error { return nil }); err != nil {
		t.Fatalf("retry delivery: %v", err)
	}
	got, _ = store.Get(event.ID)
	if got.Status != OutboxStatusSent || got.Attempts != 2 {
		t.Fatalf("successful retry state = %+v, want sent after second attempt", got)
	}
}

func TestExecuteUserInitializationUsesTransactionBoundary(t *testing.T) {
	var applied bool
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{2},
		Steps: []InitializationStep{{
			Name: "user",
			Apply: func(context.Context) error {
				applied = true
				return nil
			},
		}},
	}
	runner := func(ctx context.Context, apply func(context.Context) error) error {
		if err := apply(ctx); err != nil {
			return err
		}
		return nil
	}

	result, err := ExecuteUserInitialization(context.Background(), plan, runner)
	if err != nil || !result.Committed || result.Mode != "transaction" || !applied {
		t.Fatalf("transaction result = %+v, err=%v", result, err)
	}
}

func TestExecuteUserInitializationCompensatesAppliedStepsOnPartialFailure(t *testing.T) {
	var compensated bool
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{3},
		Steps: []InitializationStep{
			{
				Name:  "user",
				Apply: func(context.Context) error { return nil },
				Compensate: func(context.Context) error {
					compensated = true
					return nil
				},
			},
			{Name: "defaults", Apply: func(context.Context) error { return errors.New("fixture failure") }},
		},
	}

	result, err := ExecuteUserInitialization(context.Background(), plan, nil)
	if !errors.Is(err, ErrPartialWrite) || !result.PartialWrite || !compensated || result.FailedStep != "defaults" {
		t.Fatalf("partial result = %+v, err=%v", result, err)
	}
}

func TestExecuteUserInitializationFallsBackWhenTransactionsUnsupported(t *testing.T) {
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{4},
		Steps:  []InitializationStep{{Name: "user", Apply: func(context.Context) error { return nil }}},
	}
	runner := func(context.Context, func(context.Context) error) error {
		return errors.New("transaction numbers are only allowed on a replica set member")
	}

	result, err := ExecuteUserInitialization(context.Background(), plan, runner)
	if err != nil || !result.Compensated || result.Mode != "compensation" {
		t.Fatalf("fallback result = %+v, err=%v", result, err)
	}
}

func TestPersistenceErrorPreservesStableCategory(t *testing.T) {
	err := &PersistenceError{Code: ErrDuplicateIdentity.Error(), Cause: errors.New("duplicate key")}
	if !IsDuplicateIdentityError(err) || !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("duplicate identity category was not preserved: %v", err)
	}
}

func TestPersistenceErrorDoesNotBroadenCategoryOrLeakCause(t *testing.T) {
	partial := &PersistenceError{Code: ErrPartialWrite.Error(), Cause: errors.New("users@example.test duplicate index")}
	if errors.Is(partial, ErrDuplicateIdentity) {
		t.Fatal("partial_write must not match duplicate_identity")
	}
	if errors.Is(partial, errors.New(ErrPartialWrite.Error())) {
		t.Fatal("persistence categories must not match arbitrary same-text errors")
	}
	if strings.Contains(partial.Error(), "users@example.test") || strings.Contains(partial.Error(), "duplicate index") {
		t.Fatalf("persistence error leaked driver cause: %q", partial.Error())
	}
}

func TestTransactionUnsupportedOnlyAcceptsCapabilityErrors(t *testing.T) {
	for _, code := range []int32{251, 263} {
		if transactionUnsupported(mongo.CommandError{Code: code, Message: "transient transaction failure"}) {
			t.Fatalf("command code %d must not trigger compensation fallback", code)
		}
	}
	if !transactionUnsupported(mongo.CommandError{Code: 20, Message: "Transaction numbers are only allowed on a replica set member or mongos"}) {
		t.Fatal("standalone capability error must trigger compensation fallback")
	}
}

func TestExecuteUserInitializationClearsAttemptStateOnTransactionRetry(t *testing.T) {
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{6},
		Steps:  []InitializationStep{{Name: "user", Apply: func(context.Context) error { return nil }}},
	}
	var calls int
	runner := func(ctx context.Context, apply func(context.Context) error) error {
		if err := apply(ctx); err != nil {
			return err
		}
		calls++
		return apply(ctx)
	}
	result, err := ExecuteUserInitialization(context.Background(), plan, runner)
	if err != nil || calls != 1 || len(result.AppliedSteps) != 1 || result.AppliedSteps[0] != "user" {
		t.Fatalf("retry state = %+v, calls=%d, err=%v; want one final attempt", result, calls, err)
	}
}

func TestExecuteUserInitializationDoesNotCompensateTransientTransactionError(t *testing.T) {
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{7},
		Steps:  []InitializationStep{{Name: "user", Apply: func(context.Context) error { return nil }}},
	}
	runner := func(context.Context, func(context.Context) error) error {
		return mongo.CommandError{Code: 251, Message: "transient transaction failure"}
	}
	result, err := ExecuteUserInitialization(context.Background(), plan, runner)
	if err == nil || result.Mode == "compensation" || result.Compensated {
		t.Fatalf("transient transaction error was incorrectly compensated: %+v, err=%v", result, err)
	}
}

func TestDecodeActionTokenRejectsAmbiguousIdentityAndFractionalType(t *testing.T) {
	userID := MustObjectIDFromHex("507f1f77bcf86cd799439011")
	base := bson.M{
		"_id":         bson.ObjectID(userID),
		"UserId":      bson.ObjectID(userID),
		"Token":       "secret",
		"CreatedTime": time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}
	if _, err := DecodeActionToken(withField(base, "UserId", bson.ObjectID{})); err == nil {
		t.Fatal("explicit zero UserId must not be treated as a legacy document")
	}
	if _, err := DecodeActionToken(withField(base, "Type", 1.9)); err == nil {
		t.Fatal("fractional token type must be rejected")
	}
	if _, err := DecodeActionToken(withField(base, "Type", 1.0)); err == nil {
		t.Fatal("floating-point token type must be rejected")
	}
}

func withField(input bson.M, key string, value interface{}) bson.M {
	copy := bson.M{}
	for k, v := range input {
		copy[k] = v
	}
	copy[key] = value
	return copy
}

func TestOutboxIdempotencyKeyDerivesStableEventID(t *testing.T) {
	base := OutboxEvent{IdempotencyKey: "register:507f1f77bcf86cd799439011", Kind: "activate-email", AggregateID: domain.ObjectID{8}}
	one := normalizeOutboxEvent(base, time.Unix(100, 0))
	two := normalizeOutboxEvent(base, time.Unix(200, 0))
	if one.ID.IsZero() || one.ID != two.ID {
		t.Fatalf("idempotency key did not produce a stable event id: one=%s two=%s", one.ID.Hex(), two.ID.Hex())
	}
}

func TestOutboxRejectsEventsWithoutAStableIdentity(t *testing.T) {
	store := NewMemoryOutboxStore()
	err := store.Enqueue(context.Background(), OutboxEvent{Kind: "activate-email", AggregateID: domain.ObjectID{11}})
	if err == nil {
		t.Fatal("outbox must reject an event without idempotency key or explicit ID")
	}
}

func TestMemoryOutboxReclaimsExpiredSendingLease(t *testing.T) {
	store := NewMemoryOutboxStore()
	now := time.Unix(100, 0)
	event := OutboxEvent{ID: NewObjectID(), Kind: "activate-email", AggregateID: domain.ObjectID{9}, NextAttemptAt: now}
	if err := store.Enqueue(context.Background(), event); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	store.mu.Lock()
	claimed := store.events[event.ID]
	claimed.Status = OutboxStatusSending
	claimed.LeaseUntil = now.Add(-time.Second)
	store.events[event.ID] = claimed
	store.mu.Unlock()
	if err := store.Deliver(context.Background(), event.ID, now, func(context.Context, OutboxEvent) error { return nil }); err != nil {
		t.Fatalf("reclaim delivery: %v", err)
	}
	got, _ := store.Get(event.ID)
	if got.Status != OutboxStatusSent || got.Attempts != 1 {
		t.Fatalf("reclaimed event = %+v, want sent with one attempt", got)
	}
}

func TestExecuteUserInitializationExposesOutboxSideEffectFailure(t *testing.T) {
	plan := UserInitializationPlan{
		UserID: domain.ObjectID{5},
		Steps:  []InitializationStep{{Name: "user", Apply: func(context.Context) error { return nil }}},
		Outbox: &OutboxEvent{Kind: "activate-email", AggregateID: domain.ObjectID{5}},
		EnqueueOutbox: func(context.Context, OutboxEvent) error {
			return errors.New("outbox storage unavailable")
		},
	}

	result, err := ExecuteUserInitialization(context.Background(), plan, nil)
	if !errors.Is(err, ErrSideEffect) || !errors.Is(err, ErrPartialWrite) || !result.PartialWrite || result.FailedStep != "outbox" {
		t.Fatalf("outbox failure result = %+v, err=%v", result, err)
	}
}

func TestExecutePasswordTokenMutationDoesNotConsumeAfterPasswordFailure(t *testing.T) {
	consumed := false
	err := ExecutePasswordTokenMutation(context.Background(), func(ctx context.Context, apply func(context.Context) error) error {
		return apply(ctx)
	}, func(context.Context) error {
		return errors.New("password update failed")
	}, func(context.Context) error {
		consumed = true
		return nil
	})
	if err == nil || consumed {
		t.Fatalf("mutation err=%v consumed=%v, want update failure before consume", err, consumed)
	}
}

func TestExecutePasswordTokenMutationFailsClosedWithoutTransaction(t *testing.T) {
	called := false
	err := ExecutePasswordTokenMutation(context.Background(), nil, func(context.Context) error {
		called = true
		return nil
	}, func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrPartialWrite) || called {
		t.Fatalf("no-transaction err=%v called=%v, want fail-closed partial_write", err, called)
	}
}
