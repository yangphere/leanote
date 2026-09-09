package info

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// TestJSONContractFrozen locks the external encoding/json representation of
// every covered model (field names, order, zero handling, ObjectID hex).
//
// This test intentionally stays free of the mongo_contract build tag. JSON is
// a domain boundary and must remain covered when MongoDB is unavailable; the
// BSON key/round-trip probes remain in model_contract_test.go behind their
// explicit driver tag.
func TestJSONContractFrozen(t *testing.T) {
	fixtures := buildFixtures()
	names := make([]string, 0, len(jsonFullGoldens))
	for k := range jsonFullGoldens {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, ok := fixtures[name]
		if !ok {
			t.Fatalf("fixture %q missing", name)
		}
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("%s json marshal: %v", name, err)
		}
		if string(data) != jsonFullGoldens[name] {
			t.Errorf("%s full json drift\n got %s\nwant %s", name, data, jsonFullGoldens[name])
		}
		zero := reflect.New(reflect.TypeOf(raw)).Elem().Interface()
		zdata, err := json.Marshal(zero)
		if err != nil {
			t.Fatalf("%s zero json marshal: %v", name, err)
		}
		if string(zdata) != jsonZeroGoldens[name] {
			t.Errorf("%s zero json drift\n got %s\nwant %s", name, zdata, jsonZeroGoldens[name])
		}
	}
}
