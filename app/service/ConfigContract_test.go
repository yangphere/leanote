package service

import "testing"

func TestRedactedConfigProjectionDoesNotExposeSecrets(t *testing.T) {
	service := &ConfigService{
		GlobalStringConfigs: map[string]string{"emailPassword": "real-secret", "siteUrl": "https://example.test"},
		GlobalAllConfigs:    map[string]interface{}{"demoPassword": "real-demo-secret", "siteUrl": "https://example.test"},
	}
	stringsProjection := service.RedactedStringConfigs()
	if stringsProjection["emailPassword"] != RedactedSecretValue || stringsProjection["siteUrl"] != "https://example.test" {
		t.Fatalf("string projection leaked or changed values: %#v", stringsProjection)
	}
	allProjection := service.RedactedConfigProjection()
	if allProjection["demoPassword"] != RedactedSecretValue {
		t.Fatalf("all projection leaked secret: %#v", allProjection)
	}
}

func TestUpdateGlobalStringConfigsTreatsMaskedSecretAsNoOp(t *testing.T) {
	service := &ConfigService{GlobalStringConfigs: map[string]string{"emailPassword": "existing"}}
	result := service.UpdateGlobalStringConfigs("", map[string]string{"emailPassword": RedactedSecretValue})
	if len(result.Skipped) != 1 || result.Skipped[0] != "emailPassword" || result.Err() != nil {
		t.Fatalf("masked mutation result=%+v err=%v", result, result.Err())
	}
}

func TestSecurityArrayConfigValidationPreflightsBeforeMongoWrite(t *testing.T) {
	service := &ConfigService{}
	result := service.UpdateGlobalConfigs("", nil, map[string][]string{
		"feedbackRecipients":       {"bad recipient"},
		"mongoExecutableAllowlist": {"relative-tool"},
	})
	if result.Err() == nil {
		t.Fatal("invalid security arrays were accepted")
	}
	if _, ok := result.Errors["feedbackRecipients"]; !ok {
		t.Fatal("feedback recipient validation error missing")
	}
	if _, ok := result.Errors["mongoExecutableAllowlist"]; !ok {
		t.Fatal("mongo allowlist validation error missing")
	}
}

func TestConfiguredSecurityArraysValidateAtStartup(t *testing.T) {
	if err := validateConfiguredSecurityArrays(map[string][]string{"feedbackRecipients": {"invalid"}}); err == nil {
		t.Fatal("invalid startup feedback recipients accepted")
	}
	if err := validateConfiguredSecurityArrays(map[string][]string{"mongoExecutableAllowlist": {"relative-tool"}}); err == nil {
		t.Fatal("invalid startup executable allowlist accepted")
	}
}
