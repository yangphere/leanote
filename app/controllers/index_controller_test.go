package controllers

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

func TestSuggestionErrorMessageUsesStableCategories(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"validation", service.ErrSuggestionValidation, "suggestion.validation"},
		{"conflict", service.ErrSuggestionConflict, "suggestion.conflict"},
		{"side effect", service.ErrSuggestionSideEffect, "suggestion.side_effect"},
		{"unknown", errors.New("unexpected"), "suggestion.unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := suggestionErrorMessage(test.err); got != test.want {
				t.Fatalf("suggestionErrorMessage(%v)=%q, want %q", test.err, got, test.want)
			}
		})
	}
}

func TestApplySuggestionErrorPreservesReconciliationID(t *testing.T) {
	re := info.NewRe()
	reconciliationID := (domain.ObjectID{1}).Hex()
	applySuggestionError(&re, service.SuggestionSubmission{ReconciliationID: reconciliationID}, service.ErrSuggestionSideEffect)
	if re.Msg != "suggestion.side_effect" || re.Id != reconciliationID {
		t.Fatalf("mapped suggestion error=%+v", re)
	}
}
