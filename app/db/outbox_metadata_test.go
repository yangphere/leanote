package db

import (
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

func TestDecodeCommentMetadataAcceptsTypedAndStrictLegacyEvents(t *testing.T) {
	commentID := domain.ObjectID{1}
	recipientID := domain.ObjectID{2}
	typed, err := DecodeCommentMetadata(OutboxEvent{Kind: "comment", AggregateID: commentID, CommentID: commentID, RecipientID: recipientID, EventVersion: 1, IdempotencyKey: "comment:" + commentID.Hex() + ":" + recipientID.Hex(), Payload: map[string]any{"recipientId": recipientID.Hex()}})
	if err != nil || typed.CommentID != commentID || typed.RecipientID != recipientID {
		t.Fatalf("typed decode=%+v err=%v", typed, err)
	}
	legacy, err := DecodeCommentMetadata(OutboxEvent{Kind: "comment", AggregateID: commentID, Payload: map[string]any{"recipientId": recipientID.Hex()}})
	if err != nil || legacy.CommentID != commentID || legacy.RecipientID != recipientID {
		t.Fatalf("legacy decode=%+v err=%v", legacy, err)
	}
	legacyWithKey, err := DecodeCommentMetadata(OutboxEvent{Kind: "comment", AggregateID: commentID, IdempotencyKey: "comment:" + commentID.Hex() + ":" + recipientID.Hex(), Payload: map[string]any{"recipientId": recipientID.Hex()}})
	if err != nil || legacyWithKey.CommentID != commentID || legacyWithKey.RecipientID != recipientID {
		t.Fatalf("legacy event with idempotency key decode=%+v err=%v", legacyWithKey, err)
	}
	if _, err := DecodeCommentMetadata(OutboxEvent{Kind: "comment", AggregateID: commentID, IdempotencyKey: "comment:" + commentID.Hex() + ":" + domain.ObjectID{3}.Hex(), Payload: map[string]any{"recipientId": recipientID.Hex()}}); err == nil {
		t.Fatal("legacy event with mismatched idempotency key accepted")
	}
	if _, err := DecodeCommentMetadata(OutboxEvent{Kind: "comment", AggregateID: commentID, Payload: map[string]any{"recipientId": "not-an-object-id"}}); err == nil {
		t.Fatal("invalid legacy recipient accepted")
	}
}

func TestDecodeCommentMetadataUsesThreeFieldDiscriminator(t *testing.T) {
	commentID := domain.ObjectID{3}
	recipientID := domain.ObjectID{4}
	typed := OutboxEvent{Kind: "comment", AggregateID: commentID, CommentID: commentID, RecipientID: recipientID, EventVersion: 1, Payload: map[string]any{"recipientId": recipientID.Hex()}}
	if _, err := DecodeCommentMetadata(typed); err == nil {
		t.Fatal("typed metadata without idempotency key accepted")
	}
	legacyWithKey := OutboxEvent{Kind: "comment", AggregateID: commentID, IdempotencyKey: "comment:" + commentID.Hex() + ":" + recipientID.Hex(), Payload: map[string]any{"recipientId": recipientID.Hex()}}
	if _, err := DecodeCommentMetadata(legacyWithKey); err != nil {
		t.Fatalf("legacy fixture with idempotency key rejected: %v", err)
	}
	typed.IdempotencyKey = "comment:" + commentID.Hex() + ":" + recipientID.Hex()
	if _, err := DecodeCommentMetadata(typed); err != nil {
		t.Fatalf("typed metadata with extra idempotency key rejected: %v", err)
	}
}

func TestAggregateBroadcastStateWaitsForEveryRecipient(t *testing.T) {
	state, terminal := aggregateBroadcastState([]OutboxEvent{
		{Kind: "broadcast", Status: OutboxStatusSent},
		{Kind: "broadcast", Status: OutboxStatusRetry},
	})
	if state != OutboxStatusRetry || terminal {
		t.Fatalf("partial batch state=%q terminal=%v", state, terminal)
	}
	state, terminal = aggregateBroadcastState([]OutboxEvent{
		{Kind: "broadcast", Status: OutboxStatusSent},
		{Kind: "broadcast", Status: OutboxStatusDead},
	})
	if state != OutboxStatusDead || !terminal {
		t.Fatalf("terminal batch state=%q terminal=%v", state, terminal)
	}
}
