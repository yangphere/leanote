package db

import (
	"fmt"

	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MustObjectIDFromHex parses a 24-char hex string into an ObjectID and panics
// on invalid input, replicating the failure semantics of mgo's bson.ObjectIdHex.
func MustObjectIDFromHex(hex string) ObjectID {
	id, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		panic(fmt.Sprintf("invalid ObjectId hex %q: %v", hex, err))
	}
	return ObjectID(id)
}

// NewObjectID generates a new ObjectID, replicating mgo's bson.NewObjectId.
func NewObjectID() ObjectID {
	return ObjectID(bson.NewObjectID())
}

// IsValidObjectIDHex reports whether hex decodes into an ObjectID.
func IsValidObjectIDHex(hex string) bool {
	_, err := bson.ObjectIDFromHex(hex)
	return err == nil
}
