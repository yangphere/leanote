package info

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// effectiveBSONTag mirrors mgo's getStructInfo tag resolution: an explicit
// `bson:"..."` wins; otherwise a raw tag without ':' is used verbatim. This
// keeps the assertions valid both before and after the legacy-tag migration.
func effectiveBSONTag(f reflect.StructField) (key string, omitEmpty bool, ok bool) {
	tag := f.Tag.Get("bson")
	if tag == "" && !strings.Contains(string(f.Tag), ":") {
		tag = string(f.Tag)
	}
	if tag == "" || tag == "-" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	key = parts[0]
	for _, flag := range parts[1:] {
		if flag == "omitempty" {
			omitEmpty = true
		}
	}
	return key, omitEmpty, true
}

func contractFieldByName(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		if f.Name == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// TestLegacyTagInventorySemantics pins the effective mgo tag semantics (key
// name + omitempty flag) of every legacy field recorded in Phase 0.
func TestLegacyTagInventorySemantics(t *testing.T) {
	if len(legacyTagInventory) != 205 {
		t.Fatalf("inventory rows = %d, want 205", len(legacyTagInventory))
	}
	for _, spec := range legacyTagInventory {
		rt, ok := contractRegistry[spec.TypeName]
		if !ok {
			t.Fatalf("type %q missing from contractRegistry", spec.TypeName)
		}
		field, found := contractFieldByName(rt, spec.FieldName)
		if !found {
			t.Fatalf("%s.%s not found", spec.TypeName, spec.FieldName)
		}
		key, omit, _ := effectiveBSONTag(field)
		if key != spec.BsonKey {
			t.Errorf("%s.%s bson key = %q, want %q", spec.TypeName, spec.FieldName, key, spec.BsonKey)
		}
		if omit != spec.OmitEmpty {
			t.Errorf("%s.%s omitempty = %v, want %v", spec.TypeName, spec.FieldName, omit, spec.OmitEmpty)
		}
	}
}

// TestLegacyTagZeroBehavior re-derives the frozen zero-value matrix through
// real mgo marshal: presence, absence or whole-struct marshal error.
func TestLegacyTagZeroBehavior(t *testing.T) {
	for _, spec := range legacyTagInventory {
		spec := spec
		t.Run(spec.TypeName+"."+spec.FieldName, func(t *testing.T) {
			rt := contractRegistry[spec.TypeName]
			inst := reflect.New(rt).Elem()
			seedExcept(inst, spec.FieldName, spec.IsObjectId)
			m, err := toBsonM(inst.Interface())
			switch spec.ZeroState {
			case zeroMarshalError:
				if err == nil {
					t.Fatalf("expected marshal error for zero %s.%s", spec.TypeName, spec.FieldName)
				}
			case zeroPresent:
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if _, ok := m[spec.BsonKey]; !ok {
					t.Fatalf("key %q absent, want present", spec.BsonKey)
				}
			case zeroAbsent:
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				if _, ok := m[spec.BsonKey]; ok {
					t.Fatalf("key %q present, want absent (zero value)", spec.BsonKey)
				}
			default:
				t.Fatalf("unknown zero state %q", spec.ZeroState)
			}
		})
	}
}

// marshalWithLeaRegistry encodes v exactly like the driver does in
// production (client registry includes the explicit ObjectID codecs).
func marshalWithLeaRegistry(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
	enc.SetRegistry(lea.CodecRegistry)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func sortedKeys(m bson.M) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func expectedKeys(encoded string) []string {
	return strings.Split(encoded, ",")
}

// timeAwareEqual compares values where time.Time uses .Equal so decoded UTC /
// local zone differences do not fail the round-trip check.
func timeAwareEqual(a, b interface{}) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if va.Type() != vb.Type() {
		return false
	}
	if ta, ok := a.(time.Time); ok {
		tb := b.(time.Time)
		return ta.Equal(tb)
	}
	switch va.Kind() {
	case reflect.Slice, reflect.Array:
		if va.Len() != vb.Len() {
			return false
		}
		for i := 0; i < va.Len(); i++ {
			if !timeAwareEqual(va.Index(i).Interface(), vb.Index(i).Interface()) {
				return false
			}
		}
		return true
	case reflect.Map:
		if va.Len() != vb.Len() {
			return false
		}
		for _, k := range va.MapKeys() {
			other := vb.MapIndex(k)
			if !other.IsValid() {
				return false
			}
			if !timeAwareEqual(va.MapIndex(k).Interface(), other.Interface()) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < va.NumField(); i++ {
			f := va.Type().Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			if !timeAwareEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
				return false
			}
		}
		return true
	case reflect.Ptr:
		if va.IsNil() || vb.IsNil() {
			return va.IsNil() == vb.IsNil()
		}
		return timeAwareEqual(va.Elem().Interface(), vb.Elem().Interface())
	}
	return reflect.DeepEqual(va.Interface(), vb.Interface())
}

// TestFixtureBSONKeysAndRoundTrip proves, per model, that marshalling emits
// exactly the frozen key set and that bytes -> struct reproduces the fixture.
func TestFixtureBSONKeysAndRoundTrip(t *testing.T) {
	if len(fixtureKeySets) != len(contractRegistry) {
		t.Fatalf("fixtureKeySets=%d registry=%d, want equal", len(fixtureKeySets), len(contractRegistry))
	}
	fixtures := buildFixtures()
	for name, rt := range contractRegistry {
		raw, ok := fixtures[name]
		if !ok {
			t.Fatalf("fixture %q missing from buildFixtures", name)
		}
		if reflect.TypeOf(raw) != rt {
			t.Fatalf("fixture %q type %T != registry %s", name, raw, rt)
		}
		data, marshalErr := marshalWithLeaRegistry(raw)
		if marshalErr != nil {
			t.Fatalf("%s marshal: %v", name, marshalErr)
		}
		var m bson.M
		if err := bson.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s unmarshal to M: %v", name, err)
		}
		got := sortedKeys(m)
		want := expectedKeys(fixtureKeySets[name])
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s keys:\n got %v\nwant %v", name, got, want)
		}
		back := reflect.New(rt)
		dec := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(data)))
		dec.SetRegistry(lea.CodecRegistry)
		if err := dec.Decode(back.Interface()); err != nil {
			t.Fatalf("%s unmarshal: %v", name, err)
		}
		if !timeAwareEqual(raw, back.Elem().Interface()) {
			t.Errorf("%s bson round-trip mismatch\n %+v\n %+v", name, raw, back.Elem().Interface())
		}
	}
}

// TestJSONContractFrozen locks the external encoding/json representation of
// every covered model (field names, order, zero handling, ObjectId hex).
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

// TestAllRegistryFieldsHaveExplicitBsonTags enforces the migration rule on
// every registered db-facing model: the driver matches untagged fields by
// lowercased name (mgo kept Go casing), so any field added later without an
// explicit bson tag must fail here, not in production data.
func TestAllRegistryFieldsHaveExplicitBsonTags(t *testing.T) {
	for name, rt := range contractRegistry {
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if field.PkgPath != "" {
				continue // unexported
			}
			if _, ok := field.Tag.Lookup("bson"); !ok {
				t.Errorf("%s.%s has no explicit bson tag", name, field.Name)
			}
		}
	}
}

// TestLegacyTagsEliminated is the post-migration guard: every field that used
// a namespace-less legacy tag now carries an explicit bson tag whose effective
// semantics equal the frozen pre-migration contract.
func TestLegacyTagsEliminated(t *testing.T) {
	for _, spec := range legacyTagInventory {
		spec := spec
		t.Run(spec.TypeName+"."+spec.FieldName, func(t *testing.T) {
			rt := contractRegistry[spec.TypeName]
			field, found := contractFieldByName(rt, spec.FieldName)
			if !found {
				t.Fatalf("%s.%s not found", spec.TypeName, spec.FieldName)
			}
			raw := string(field.Tag)
			if !strings.Contains(raw, ":") {
				t.Fatalf("raw tag %q lost its bson namespace", raw)
			}
			explicit := field.Tag.Get("bson")
			if explicit == "" {
				t.Fatalf("explicit bson tag missing, raw=%q", raw)
			}
			parts := strings.Split(explicit, ",")
			if parts[0] != spec.BsonKey {
				t.Errorf("bson key = %q, want %q", parts[0], spec.BsonKey)
			}
			omit := len(parts) > 1 && parts[1] == "omitempty"
			if omit != spec.OmitEmpty {
				t.Errorf("omitempty = %v, want %v", omit, spec.OmitEmpty)
			}
		})
	}
}
