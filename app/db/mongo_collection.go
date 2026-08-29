package db

import (
	"context"
	"errors"
	"net"
	"strings"

	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection wraps a mongo-driver/v2 collection behind the legacy mgo-style
// API. It is the single adaptation point: BSON conversion, operation timeout
// and error classification must not be duplicated in service code.
type Collection struct {
	coll *mongo.Collection
	name string
}

func wrapCollection(c *mongo.Collection) *Collection {
	return &Collection{coll: c, name: c.Name()}
}

func (c *Collection) logFailure(op string, err error) {
	Logf("mongo %s failed on collection %s [%s]: %v", op, c.name, classifyError(err), err)
}

// classifyError labels a driver error by category so logs and callers can
// tell duplicate keys, timeouts and network failures apart from other server
// errors; none of them may masquerade as success (R-B5).
func classifyError(err error) string {
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return "no-documents"
	case mongo.IsDuplicateKeyError(err):
		return "duplicate-key"
	case errors.Is(err, context.DeadlineExceeded),
		mongo.IsTimeout(err):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network"
	}
	return "command-error"
}

func (c *Collection) logNotFound(op string) {
	Logf("mongo %s on collection %s: no documents matched", op, c.name)
}

// Find starts a query chain compatible with mgo's Query.
func (c *Collection) Find(query interface{}) *Query {
	return &Query{coll: c, filter: query}
}

// FindId queries a single document by its _id field.
func (c *Collection) FindId(id interface{}) *Query {
	return c.Find(bson.M{"_id": id})
}

// DropIndex removes an index by its key fields, replicating mgo's index-name
// derivation: the field names joined with "_" in the given order.
func (c *Collection) DropIndex(key ...string) error {
	name := strings.Join(key, "_")
	ctx, cancel := operationContext()
	defer cancel()
	if err := c.coll.Indexes().DropOne(ctx, name); err != nil {
		c.logFailure("dropIndex "+name, err)
		return err
	}
	return nil
}

// Insert stores one or more documents.
func (c *Collection) Insert(docs ...interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()
	var err error
	if len(docs) == 1 {
		_, err = c.coll.InsertOne(ctx, docs[0])
	} else {
		_, err = c.coll.InsertMany(ctx, docs)
	}
	if err != nil {
		c.logFailure("insert", err)
	}
	return err
}

// Update replaces one document. Driver v2 reports a no-match as nil error
// with matched count 0 (mgo reported "not found"); the legacy bool contract
// treats both as success, and the no-match is logged for visibility.
// splitUpdateKind mirrors mgo's update semantics: a document whose first
// top-level key is a $-operator is an operator update; anything else is a
// full replacement. Driver v2 splits these across UpdateOne and ReplaceOne.
func splitUpdateKind(update interface{}) (replacement bool, err error) {
	raw, err := bson.Marshal(update)
	if err != nil {
		return false, err
	}
	elements, err := bson.Raw(raw).Elements()
	if err != nil {
		return false, err
	}
	if len(elements) == 0 {
		return true, nil // empty document replaces
	}
	return !strings.HasPrefix(elements[0].Key(), "$"), nil
}

// Update replaces one document. Replacement-style documents (no $ keys)
// route through ReplaceOne, mirroring mgo where Update accepted both forms.
func (c *Collection) Update(query, update interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()
	replacement, err := splitUpdateKind(update)
	if err != nil {
		c.logFailure("update", err)
		return err
	}
	var res *mongo.UpdateResult
	if replacement {
		res, err = c.coll.ReplaceOne(ctx, query, update)
	} else {
		res, err = c.coll.UpdateOne(ctx, query, update)
	}
	if err != nil {
		c.logFailure("update", err)
		return err
	}
	if res.MatchedCount == 0 {
		c.logNotFound("update")
	}
	return nil
}

// UpdateAll updates every matching document and returns the matched count.
func (c *Collection) UpdateAll(query, update interface{}) (int, error) {
	ctx, cancel := operationContext()
	defer cancel()
	res, err := c.coll.UpdateMany(ctx, query, update)
	if err != nil {
		c.logFailure("updateAll", err)
		return 0, err
	}
	return int(res.MatchedCount), nil
}

// Upsert updates a document or inserts it when the query matches nothing.
// Replacement-style documents route through ReplaceOne (mgo parity).
func (c *Collection) Upsert(query, update interface{}) (interface{}, error) {
	ctx, cancel := operationContext()
	defer cancel()
	replacement, err := splitUpdateKind(update)
	if err != nil {
		c.logFailure("upsert", err)
		return nil, err
	}
	var res *mongo.UpdateResult
	if replacement {
		res, err = c.coll.ReplaceOne(ctx, query, update, options.Replace().SetUpsert(true))
	} else {
		res, err = c.coll.UpdateOne(ctx, query, update, options.UpdateOne().SetUpsert(true))
	}
	if err != nil {
		c.logFailure("upsert", err)
		return nil, err
	}
	return res.UpsertedID, nil
}

// Remove deletes one document; no match is not an error (mgo semantics).
func (c *Collection) Remove(query interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()
	_, err := c.coll.DeleteOne(ctx, query)
	if err != nil {
		c.logFailure("remove", err)
	}
	return err
}

// RemoveAll deletes every matching document and returns the removed count.
func (c *Collection) RemoveAll(query interface{}) (int, error) {
	ctx, cancel := operationContext()
	defer cancel()
	res, err := c.coll.DeleteMany(ctx, query)
	if err != nil {
		c.logFailure("removeAll", err)
		return 0, err
	}
	return int(res.DeletedCount), nil
}
