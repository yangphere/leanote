package controllers

import "testing"

func TestPublishingJSONPCallbackValidation(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		ok          bool
	}{
		{"cb", "valid identifier", true},
		{"app.callbacks.done", "dotted identifier", true},
		{"", "required callback", false},
		{"x);alert(1)//", "javascript payload", false},
		{"a-b", "hyphenated identifier", false},
	} {
		if got := validJSONPCallback(tc.value, false); got != tc.ok {
			t.Errorf("callback %q (%s) = %v, want %v", tc.value, tc.want, got, tc.ok)
		}
	}
	if !validJSONPCallback("", true) {
		t.Fatal("optional callback should allow an empty value")
	}
}

func TestPublishingCommentSubmissionIDShape(t *testing.T) {
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"0123456789abcdef0123456789abcdef", true},
		{"0123456789ABCDEF0123456789abcdef", false},
		{"0123456789abcdef", false},
		{"0123456789abcdef0123456789abcdef0", false},
	} {
		if got := commentSubmissionPattern.MatchString(tc.value); got != tc.ok {
			t.Errorf("submissionId %q = %v, want %v", tc.value, got, tc.ok)
		}
	}
}
