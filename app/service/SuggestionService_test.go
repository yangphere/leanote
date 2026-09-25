package service

import (
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestValidateSuggestionInput(t *testing.T) {
	valid := info.Suggestion{UserId: domain.ObjectID{1}, Addr: "user@example.com", Suggestion: "  hello 世界  "}
	if err := validateSuggestionInput(valid, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("valid suggestion rejected: %v", err)
	}
	tests := []struct {
		name         string
		suggestion   info.Suggestion
		submissionID string
	}{
		{"blank", info.Suggestion{Suggestion: " \t\n"}, ""},
		{"control", info.Suggestion{Suggestion: "hello\x00"}, ""},
		{"display-name", info.Suggestion{Addr: "Name <user@example.com>", Suggestion: "hello"}, ""},
		{"bad-id", info.Suggestion{UserId: domain.ObjectID{1}, Suggestion: "hello"}, "ABC"},
		{"anonymous-id", info.Suggestion{Suggestion: "hello"}, "0123456789abcdef0123456789abcdef"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateSuggestionInput(test.suggestion, test.submissionID); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateSuggestionInputLimitsUnicodeAndBytes(t *testing.T) {
	tooManyRunes := info.Suggestion{Suggestion: strings.Repeat("界", 2001)}
	if err := validateSuggestionInput(tooManyRunes, ""); err == nil {
		t.Fatal("expected rune limit error")
	}
	tooManyBytes := info.Suggestion{Suggestion: strings.Repeat("a", 8193)}
	if err := validateSuggestionInput(tooManyBytes, ""); err == nil {
		t.Fatal("expected byte limit error")
	}
}

func TestValidateSuggestionInputAllowsMultilineFormatting(t *testing.T) {
	input := info.Suggestion{Suggestion: "line one\nline two\r\n\tindented"}
	if err := validateSuggestionInput(input, ""); err != nil {
		t.Fatalf("multiline suggestion rejected: %v", err)
	}
	for _, control := range []rune{'\x01', '\x0b', '\x0c', '\x1f', '\x7f'} {
		if err := validateSuggestionInput(info.Suggestion{Suggestion: string([]rune{'a', control, 'b'})}, ""); err == nil {
			t.Fatalf("control U+%04X accepted", control)
		}
	}
}
