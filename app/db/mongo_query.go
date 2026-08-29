package db

import (
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Query mirrors the mgo query chain: Find -> Sort/Skip/Limit/Select ->
// One/All/Count/Distinct. Sorting is stored as an ordered bson.D so that
// field order survives; maps are never used for sort documents.
type Query struct {
	coll       *Collection
	filter     interface{}
	sort       bson.D
	skip       *int64
	limit      *int64
	projection interface{}
}

// parseSortFields converts mgo-style sort strings ("-Count", "Usn", "+Title")
// into an ordered sort document.
func parseSortFields(fields []string) bson.D {
	d := make(bson.D, 0, len(fields))
	for _, f := range fields {
		name := f
		value := 1
		switch {
		case strings.HasPrefix(f, "-"):
			name = f[1:]
			value = -1
		case strings.HasPrefix(f, "+"):
			name = f[1:]
		}
		d = append(d, bson.E{Key: name, Value: value})
	}
	return d
}

// Sort chains sort fields; a leading "-" means descending, "+" or nothing
// ascending, matching mgo.
func (q *Query) Sort(fields ...string) *Query {
	q.sort = parseSortFields(fields)
	return q
}

// Skip chains the number of documents to skip; 0 means no skip, as in mgo.
func (q *Query) Skip(n int) *Query {
	v := int64(n)
	q.skip = &v
	return q
}

// Limit chains the maximum number of documents; 0 means no limit, as in mgo.
func (q *Query) Limit(n int) *Query {
	v := int64(n)
	q.limit = &v
	return q
}

// Select chains a projection document.
func (q *Query) Select(selector interface{}) *Query {
	q.projection = selector
	return q
}

// findOptions is the shared sort/skip/limit/projection state applied to both
// Find and FindOne builders (One and All duplicate nothing but the Apply).
type findOptions struct {
	sort       bson.D
	skip       *int64
	limit      *int64
	projection interface{}
}

func (q *Query) findOptions() findOptions {
	return findOptions{sort: q.sort, skip: q.skip, limit: q.limit, projection: q.projection}
}

func (o findOptions) applyFindOne(opts *options.FindOneOptionsBuilder) {
	if len(o.sort) > 0 {
		opts.SetSort(o.sort)
	}
	if o.skip != nil && *o.skip != 0 {
		opts.SetSkip(*o.skip)
	}
	if o.projection != nil {
		opts.SetProjection(o.projection)
	}
}

func (o findOptions) applyFind(opts *options.FindOptionsBuilder) {
	if len(o.sort) > 0 {
		opts.SetSort(o.sort)
	}
	if o.skip != nil && *o.skip != 0 {
		opts.SetSkip(*o.skip)
	}
	if o.limit != nil && *o.limit != 0 {
		opts.SetLimit(*o.limit)
	}
	if o.projection != nil {
		opts.SetProjection(o.projection)
	}
}

// One decodes the first matching document into result. A missing document
// returns mongo.ErrNoDocuments, which Err() maps to the legacy semantics.
// A chained Limit is intentionally ignored: FindOne in driver v2 has no
// limit option, and a limit is irrelevant to a single-document result —
// sorting + first document equals sorting + limit(1) (BlogService callers).
func (q *Query) One(result interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()

	opts := options.FindOne()
	q.findOptions().applyFindOne(opts)

	err := q.coll.coll.FindOne(ctx, q.filter, opts).Decode(result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			q.coll.logNotFound("findOne")
		} else {
			q.coll.logFailure("findOne", err)
		}
	}
	return err
}

// All decodes every matching document into result, always closing the cursor.
func (q *Query) All(result interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()

	opts := options.Find()
	q.findOptions().applyFind(opts)

	cursor, err := q.coll.coll.Find(ctx, q.filter, opts)
	if err != nil {
		q.coll.logFailure("find", err)
		return err
	}
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			q.coll.logFailure("cursor close", cerr)
		}
	}()

	if err := cursor.All(ctx, result); err != nil {
		q.coll.logFailure("findAll", err)
		return err
	}
	return nil
}

// Count returns the number of matching documents.
func (q *Query) Count() (int, error) {
	ctx, cancel := operationContext()
	defer cancel()

	n, err := q.coll.coll.CountDocuments(ctx, q.filter)
	if err != nil {
		q.coll.logFailure("count", err)
		return 0, err
	}
	return int(n), nil
}

// Distinct decodes the distinct values of key into result, preserving the
// element type of the caller's slice (mgo behavior).
func (q *Query) Distinct(key string, result interface{}) error {
	ctx, cancel := operationContext()
	defer cancel()

	if err := q.coll.coll.Distinct(ctx, key, q.filter).Decode(result); err != nil {
		q.coll.logFailure("distinct", err)
		return err
	}
	return nil
}
