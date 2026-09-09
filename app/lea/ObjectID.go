package lea

import (
	"fmt"
	"reflect"

	"github.com/yangphere/leanote/app/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ObjectID is retained as a migration alias. New domain models must import
// domain.ObjectID directly; this alias is removed after downstream adapters
// complete their migration.
type ObjectID = domain.ObjectID

// CodecRegistry is the default BSON registry plus explicit codecs for the
// domain ObjectID. BSON conversion belongs to this adapter layer rather than
// the domain package.
var CodecRegistry = func() *bson.Registry {
	reg := bson.NewRegistry()
	t := reflect.TypeOf(ObjectID{})
	reg.RegisterTypeEncoder(t, bson.ValueEncoderFunc(func(_ bson.EncodeContext, vw bson.ValueWriter, val reflect.Value) error {
		if !val.CanInterface() {
			return fmt.Errorf("ObjectID encoder: value not addressable")
		}
		id, ok := val.Interface().(ObjectID)
		if !ok {
			return fmt.Errorf("ObjectID encoder: expected %T, got %T", ObjectID{}, val.Interface())
		}
		return vw.WriteObjectID(bson.ObjectID(id))
	}))
	reg.RegisterTypeDecoder(t, bson.ValueDecoderFunc(func(_ bson.DecodeContext, vr bson.ValueReader, val reflect.Value) error {
		// Legacy documents may store IDs as BSON strings. Empty strings map to
		// zero, 24-character hex strings are decoded, and every other value is
		// rejected at the persistence boundary.
		switch vr.Type() {
		case bson.TypeObjectID:
			oid, err := vr.ReadObjectID()
			if err != nil {
				return err
			}
			if !val.CanSet() {
				return fmt.Errorf("ObjectID decoder: value not settable")
			}
			val.Set(reflect.ValueOf(ObjectID(oid)))
			return nil
		case bson.TypeString:
			s, err := vr.ReadString()
			if err != nil {
				return err
			}
			parsed, err := domain.ParseObjectID(s)
			if err != nil {
				return err
			}
			if !val.CanSet() {
				return fmt.Errorf("ObjectID decoder: value not settable")
			}
			val.Set(reflect.ValueOf(parsed))
			return nil
		default:
			return fmt.Errorf("cannot decode BSON %s into an ObjectID", vr.Type())
		}
	}))
	return reg
}()
