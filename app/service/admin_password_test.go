package service

import "testing"

func TestParseAdminForceEnvPasswordIsStrict(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"", false}, {"false", false}, {"true", true},
	} {
		got, err := ParseAdminForceEnvPassword(test.value)
		if err != nil || got != test.want {
			t.Fatalf("ParseAdminForceEnvPassword(%q) = %v, %v; want %v, nil", test.value, got, err, test.want)
		}
	}
	if _, err := ParseAdminForceEnvPassword("1"); err == nil {
		t.Fatal("numeric force flag was accepted")
	}
}

func TestAdminPasswordFingerprintUsesSecretAndDoesNotExposePassword(t *testing.T) {
	first := adminPasswordFingerprint("secret-a", "initial-password")
	if len(first) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(first))
	}
	if first == adminPasswordFingerprint("secret-b", "initial-password") {
		t.Fatal("fingerprint did not depend on secret")
	}
	if first == adminPasswordFingerprint("secret-a", "other-password") {
		t.Fatal("fingerprint did not depend on password")
	}
}

func TestDockerAdminUsernameNormalizesEmailPrefix(t *testing.T) {
	for _, test := range []struct {
		email, want string
	}{
		{"Admin.Example@example.com", "admin-example"},
		{"user_name@example.com", "user_name"},
	} {
		got, err := dockerAdminUsername(test.email)
		if err != nil || got != test.want {
			t.Fatalf("dockerAdminUsername(%q) = %q, %v; want %q", test.email, got, err, test.want)
		}
	}
	if _, err := dockerAdminUsername("abc@example.com"); err == nil {
		t.Fatal("short email prefix was accepted")
	}
}

func TestConfigureDockerAdminPasswordRejectsInvalidInput(t *testing.T) {
	if err := ConfigureDockerAdminPassword("invalid", "short", "", false); err == nil {
		t.Fatal("invalid Docker administrator configuration was accepted")
	}
	for _, test := range []struct {
		name, password, secret string
	}{
		{"password placeholder", "REPLACE_WITH_A_RANDOM_PASSWORD", "application-secret"},
		{"secret placeholder", "valid-password", "REPLACE_WITH_OPENSSL_OUTPUT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ConfigureDockerAdminPassword("admin@example.com", test.password, test.secret, false); err == nil {
				t.Fatal("placeholder configuration was accepted")
			}
		})
	}
}
