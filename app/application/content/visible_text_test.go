package content

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanVisibleTextTrimsAndRejectsControlCharacters(t *testing.T) {
	got, err := CleanVisibleText(` C:\fakepath\report`+"\x00"+`name.txt `, false)
	if err != nil {
		t.Fatalf("CleanVisibleText() error = %v", err)
	}
	if got != "reportname.txt" {
		t.Fatalf("CleanVisibleText() = %q, want reportname.txt", got)
	}
	if got, err := CleanVisibleText("../nested/album", false); err != nil || got != "album" {
		t.Fatalf("CleanVisibleText(path) = %q, %v", got, err)
	}
}

func TestCleanVisibleTextUsesUTF8ByteLimitWithoutTruncating(t *testing.T) {
	original := strings.Repeat("界", 86)
	got, err := CleanVisibleText(original, false)
	if err == nil {
		t.Fatal("CleanVisibleText() accepted 258 UTF-8 bytes")
	}
	if got != "" {
		t.Fatalf("CleanVisibleText() returned truncated value %q", got)
	}
	var contentErr *Error
	if !errors.As(err, &contentErr) || contentErr.Category != ErrorValidation || contentErr.Code != "visible_text_too_long" {
		t.Fatalf("CleanVisibleText() error = %#v", err)
	}
	if original != strings.Repeat("界", 86) {
		t.Fatal("input was modified")
	}
}

func TestCleanVisibleTextEmptyPolicyAndInvalidUTF8(t *testing.T) {
	if _, err := CleanVisibleText(" \n\t ", false); err == nil {
		t.Fatal("CleanVisibleText() accepted an empty required value")
	}
	if got, err := CleanVisibleText(" \n\t ", true); err != nil || got != "" {
		t.Fatalf("CleanVisibleText(allowEmpty) = %q, %v", got, err)
	}
	if _, err := CleanVisibleText(string([]byte{0xff}), false); err == nil {
		t.Fatal("CleanVisibleText() accepted invalid UTF-8")
	}
}
