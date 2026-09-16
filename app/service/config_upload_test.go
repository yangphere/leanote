package service

import (
	"math"
	"testing"
)

func TestGetUploadLimitBytesFailsClosedForMissingOrInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "missing", value: ""},
		{name: "invalid", value: "large"},
		{name: "zero", value: "0"},
		{name: "negative", value: "-1"},
		{name: "nan", value: "NaN"},
		{name: "infinite", value: "+Inf"},
		{name: "overflow", value: "1e30"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := ConfigService{GlobalStringConfigs: map[string]string{"uploadImageSize": test.value}}
			if _, err := service.GetUploadLimitBytes("uploadImageSize"); err == nil {
				t.Fatalf("value %q unexpectedly accepted", test.value)
			}
		})
	}
}

func TestGetUploadLimitBytesConvertsPositiveMegabytes(t *testing.T) {
	service := ConfigService{GlobalStringConfigs: map[string]string{"uploadImageSize": "1.5"}}
	got, err := service.GetUploadLimitBytes("uploadImageSize")
	if err != nil || got != 1_572_864 {
		t.Fatalf("got=%d err=%v", got, err)
	}
	service.GlobalStringConfigs["uploadImageSize"] = "0.000001"
	got, err = service.GetUploadLimitBytes("uploadImageSize")
	if err != nil || got != int64(math.Floor(1.048576)) {
		t.Fatalf("small got=%d err=%v", got, err)
	}
}
