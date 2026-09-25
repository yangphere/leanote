package service

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

func TestBuildBroadcastPlanFreezesRecipientIdentity(t *testing.T) {
	actor := domain.ObjectID{1}
	submission, plan, err := BuildBroadcastPlan(actor, "0123456789abcdef0123456789abcdef", []string{"A@example.com", "a@example.com", "b@example.com"}, "Subject", "<p>body</p>")
	if err != nil {
		t.Fatalf("BuildBroadcastPlan: %v", err)
	}
	if !submission.RetrySafe || len(plan.Events) != 2 || len(plan.Receipt.OutboxIds) != 2 {
		t.Fatalf("submission=%+v receipt=%+v events=%d", submission, plan.Receipt, len(plan.Events))
	}
	if plan.Events[0].IdempotencyKey == plan.Events[1].IdempotencyKey {
		t.Fatal("recipient events share an identity")
	}
}

func TestBuildBroadcastPlanLegacyReconciliationIsNotRetrySafe(t *testing.T) {
	submission, plan, err := BuildBroadcastPlan(domain.ObjectID{2}, "", []string{"user@example.com"}, "Subject", "body")
	if err != nil || submission.RetrySafe || len(submission.BatchID) != 32 || plan.Receipt.RetrySafe {
		t.Fatalf("submission=%+v plan=%+v err=%v", submission, plan.Receipt, err)
	}
}

func TestBuildBroadcastPlanRejectsHeaderInjectionAndConflictShape(t *testing.T) {
	if _, _, err := BuildBroadcastPlan(domain.ObjectID{3}, "0123456789abcdef0123456789abcdef", []string{"user@example.com"}, "bad\r\nsubject", "body"); !errors.Is(err, ErrBroadcastValidation) {
		t.Fatalf("header injection error=%v", err)
	}
	if _, _, err := BuildBroadcastPlan(domain.ObjectID{3}, "ABC", []string{"user@example.com"}, "subject", "body"); !errors.Is(err, ErrBroadcastValidation) {
		t.Fatalf("invalid batch error=%v", err)
	}
}

func TestBuildBroadcastPlanIgnoresBlankRecipientLines(t *testing.T) {
	_, plan, err := BuildBroadcastPlan(domain.ObjectID{4}, "0123456789abcdef0123456789abcdef", []string{"", "  ", "user@example.com", "\t"}, "subject", "body")
	if err != nil {
		t.Fatalf("BuildBroadcastPlan: %v", err)
	}
	if len(plan.Events) != 1 || len(plan.Receipt.RecipientSnapshot) != 1 || plan.Receipt.RecipientSnapshot[0] != "user@example.com" {
		t.Fatalf("blank lines were not filtered: receipt=%+v events=%d", plan.Receipt, len(plan.Events))
	}
}

func TestBuildBroadcastPlanRejectsTrimmedHeaderInjection(t *testing.T) {
	if _, _, err := BuildBroadcastPlan(domain.ObjectID{5}, "0123456789abcdef0123456789abcdef", []string{"user@example.com\n"}, "subject", "body"); !errors.Is(err, ErrBroadcastValidation) {
		t.Fatalf("trailing newline recipient error=%v, want validation", err)
	}
}
