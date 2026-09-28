package httpserver

import "testing"

func TestNeedValidateWhitelistFailsClosedForUnknownActions(t *testing.T) {
	whitelist := map[string]map[string]bool{
		"Auth": {"Login": true},
	}
	for _, test := range []struct {
		controller string
		method     string
		want       bool
	}{
		{controller: "Auth", method: "Login", want: false},
		{controller: "Auth", method: "Logout", want: true},
		{controller: "Unknown", method: "Login", want: true},
	} {
		if got := NeedValidateWhitelist(whitelist, test.controller, test.method); got != test.want {
			t.Errorf("NeedValidateWhitelist(%q, %q) = %v, want %v", test.controller, test.method, got, test.want)
		}
	}
}
