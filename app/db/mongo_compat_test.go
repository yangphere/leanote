package db

// Integration tests for the mgo-compatible wrapper over mongo-driver/v2.
// They require a reachable MongoDB (default mongodb://127.0.0.1:27017,
// override with LEANOTE_DB_TEST_URI) and use a dedicated throwaway database.

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	. "github.com/yangphere/leanote/app/lea"
)

const (
	testDatabaseName   = "leanote_wrapper_test"
	testCollectionName = "compat"
)

var (
	testConnectOnce sync.Once
	testColl        *Collection
	testRawColl     *mongo.Collection
	testSetupErr    error
)

func testCollection(t *testing.T) (*Collection, *mongo.Collection) {
	t.Helper()
	testConnectOnce.Do(func() {
		uri := os.Getenv("LEANOTE_DB_TEST_URI")
		if uri == "" {
			uri = "mongodb://127.0.0.1:27017"
		}
		ctx, cancel := contextWithTimeout(5 * time.Second)
		defer cancel()
		client, err := mongo.Connect(options.Client().ApplyURI(uri))
		if err != nil {
			testSetupErr = err
			return
		}
		if err := client.Ping(ctx, nil); err != nil {
			testSetupErr = err
			return
		}
		if err := client.Database(testDatabaseName).Drop(ctx); err != nil {
			testSetupErr = err
			return
		}
		testRawColl = client.Database(testDatabaseName).Collection(testCollectionName)
		testColl = wrapCollection(testRawColl)
	})
	if testSetupErr != nil {
		t.Skipf("MongoDB unavailable for wrapper tests: %v", testSetupErr)
	}
	return testColl, testRawColl
}

type compatDoc struct {
	ID     ObjectID `bson:"_id,omitempty"`
	UserId ObjectID `bson:"UserId"`
	Title  string   `bson:"Title"`
	Count  int      `bson:"Count"`
	Nested string   `bson:"Nested,omitempty"`
}

func seedCompatDocs(t *testing.T, docs []interface{}) {
	t.Helper()
	coll, raw := testCollection(t)
	if err := coll.Insert(docs...); err != nil {
		t.Fatalf("Insert seed: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := contextWithTimeout(5 * time.Second)
		defer cancel()
		if _, err := raw.DeleteMany(ctx, bson.M{}); err != nil {
			t.Fatalf("cleanup DeleteMany: %v", err)
		}
	})
}

func compatSeed() []interface{} {
	uid := MustObjectIDFromHex("507f1f77bcf86cd7994390aa")
	return []interface{}{
		compatDoc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439001"), UserId: uid, Title: "alpha", Count: 1},
		compatDoc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439002"), UserId: uid, Title: "beta", Count: 2},
		compatDoc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439003"), UserId: uid, Title: "gamma", Count: 3},
	}
}

func TestCompatFindSortSkipLimitAll(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	var got []compatDoc
	// Sort by Count descending: [gamma(3) beta(2) alpha(1)]; skip 1, take 2.
	if err := coll.Find(bson.M{"UserId": MustObjectIDFromHex("507f1f77bcf86cd7994390aa")}).
		Sort("-Count").Skip(1).Limit(2).All(&got); err != nil {
		t.Fatalf("Find chain: %v", err)
	}
	if len(got) != 2 || got[0].Title != "beta" || got[1].Title != "alpha" {
		t.Fatalf("unexpected page: %+v", got)
	}

	// Multi-field ascending sort must preserve field order (map-free).
	var multi []compatDoc
	if err := coll.Find(bson.M{}).Sort("UserId", "Count").All(&multi); err != nil {
		t.Fatalf("multi Sort: %v", err)
	}
	if len(multi) != 3 || multi[0].Count != 1 || multi[2].Count != 3 {
		t.Fatalf("unexpected multi sort: %+v", multi)
	}
}

func TestCompatFindSelectProjection(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	var got compatDoc
	if err := coll.Find(bson.M{"Title": "alpha"}).
		Select(bson.M{"Title": true, "UserId": true}).One(&got); err != nil {
		t.Fatalf("Select().One(): %v", err)
	}
	if got.Title != "alpha" {
		t.Fatalf("unexpected doc: %+v", got)
	}
	// Inclusion projections keep _id by default (server behavior, same as mgo).
	if got.ID.Hex() != "507f1f77bcf86cd799439001" {
		t.Fatalf("projected doc should keep _id, got %s", got.ID.Hex())
	}
}

func TestCompatOneNotFoundSemantics(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	var got compatDoc
	err := coll.Find(bson.M{"Title": "missing"}).One(&got)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("One() on missing doc = %v, want mongo.ErrNoDocuments", err)
	}
	if !Err(err) {
		t.Fatalf("Err(ErrNoDocuments) = false, want true (legacy not-found compatibility)")
	}
}

func TestCompatFindId(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	var got compatDoc
	if err := coll.FindId(MustObjectIDFromHex("507f1f77bcf86cd799439002")).One(&got); err != nil {
		t.Fatalf("FindId().One(): %v", err)
	}
	if got.Title != "beta" {
		t.Fatalf("unexpected doc: %+v", got)
	}
	if err := coll.FindId(MustObjectIDFromHex("507f1f77bcf86cd7994390ff")).One(&got); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("FindId() missing = %v, want mongo.ErrNoDocuments", err)
	}
}

func TestCompatInsertAndCount(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	if n := Count(coll, bson.M{"Title": "alpha"}); n != 1 {
		t.Fatalf("Count = %d, want 1", n)
	}
	if !Has(coll, bson.M{"Title": "alpha"}) || Has(coll, bson.M{"Title": "nope"}) {
		t.Fatal("Has() disagrees with seeded data")
	}
	if err := coll.Insert(compatDoc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439010"), UserId: MustObjectIDFromHex("507f1f77bcf86cd7994390aa"), Title: "delta"}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if n := Count(coll, bson.M{}); n != 4 {
		t.Fatalf("Count after insert = %d, want 4", n)
	}
}

func TestCompatUpdateAndUpsert(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	// Single update.
	if err := coll.Update(bson.M{"Title": "alpha"}, bson.M{"$set": bson.M{"Nested": "touched"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var got compatDoc
	if err := coll.Find(bson.M{"Title": "alpha"}).One(&got); err != nil || got.Nested != "touched" {
		t.Fatalf("update not visible: %+v err=%v", got, err)
	}

	// Single update on missing doc: driver v2 returns nil with matched count 0;
	// legacy semantics treat it as success (mgo's "not found" mapped to true).
	err := coll.Update(bson.M{"Title": "missing"}, bson.M{"$set": bson.M{"Nested": "x"}})
	if err != nil {
		t.Fatalf("Update missing = %v, want nil", err)
	}
	if !Err(err) {
		t.Fatal("Err(update-missing) = false, want true (legacy idempotent semantics)")
	}

	// Upsert inserts when missing.
	if _, err := coll.Upsert(bson.M{"Title": "made-up"}, bson.M{"$set": bson.M{"Count": 9}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if n := Count(coll, bson.M{"Title": "made-up"}); n != 1 {
		t.Fatalf("upsert did not insert, Count=%d", n)
	}

	// UpdateAll reports matched count.
	n, err := coll.UpdateAll(bson.M{"UserId": MustObjectIDFromHex("507f1f77bcf86cd7994390aa")}, bson.M{"$set": bson.M{"Nested": "bulk"}})
	if err != nil {
		t.Fatalf("UpdateAll: %v", err)
	}
	if n < 3 {
		t.Fatalf("UpdateAll matched %d, want >= 3", n)
	}
}

func TestCompatRemoveAndRemoveAll(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	if err := coll.Remove(bson.M{"Title": "alpha"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if Count(coll, bson.M{"Title": "alpha"}) != 0 {
		t.Fatal("Remove did not delete")
	}
	// Removing a missing doc is not an error (mgo semantics).
	if err := coll.Remove(bson.M{"Title": "alpha"}); err != nil {
		t.Fatalf("Remove on missing doc = %v, want nil", err)
	}

	n, err := coll.RemoveAll(bson.M{"UserId": MustObjectIDFromHex("507f1f77bcf86cd7994390aa")})
	if err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if n < 2 {
		t.Fatalf("RemoveAll removed %d, want >= 2", n)
	}
}

func TestCompatDistinct(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	var titles []string
	if err := coll.Find(bson.M{}).Distinct("Title", &titles); err != nil {
		t.Fatalf("Distinct into []string: %v", err)
	}
	if len(titles) != 3 {
		t.Fatalf("Distinct titles = %v, want 3 entries", titles)
	}

	var ids []ObjectID
	if err := coll.Find(bson.M{}).Distinct("_id", &ids); err != nil {
		t.Fatalf("Distinct into []ObjectID: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("Distinct ids = %v, want 3 entries", ids)
	}
}

func TestCompatDropIndexByKey(t *testing.T) {
	coll, raw := testCollection(t)
	seedCompatDocs(t, compatSeed())

	// mgo generates index names by joining key names with "_".
	const mgoStyleName = "UserId_ToUserId_NoteId"
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()
	if _, err := raw.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "UserId", Value: 1}, {Key: "ToUserId", Value: 1}, {Key: "NoteId", Value: 1}},
		Options: options.Index().SetName(mgoStyleName),
	}); err != nil {
		t.Fatalf("CreateOne: %v", err)
	}

	if err := coll.DropIndex("UserId", "ToUserId", "NoteId"); err != nil {
		t.Fatalf("DropIndex by keys: %v", err)
	}

	cur, err := raw.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("ListIndexes: %v", err)
	}
	var idx []bson.M
	if err := cur.All(ctx, &idx); err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	for _, spec := range idx {
		if name, _ := spec["name"].(string); name == mgoStyleName {
			t.Fatalf("index %q still present after DropIndex", mgoStyleName)
		}
	}
}

func TestCompatDuplicateKeyClassification(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	dup := compatDoc{ID: MustObjectIDFromHex("507f1f77bcf86cd799439001"), UserId: MustObjectIDFromHex("507f1f77bcf86cd7994390aa"), Title: "dup"}
	err := coll.Insert(dup)
	if err == nil {
		t.Fatal("duplicate _id insert must fail")
	}
	if !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("duplicate insert error not classified as duplicate key: %v", err)
	}
	if got := classifyError(err); got != "duplicate-key" {
		t.Fatalf("classifyError = %q, want duplicate-key", got)
	}
	if Err(err) {
		t.Fatal("duplicate key must not map to legacy success")
	}
}

func TestCompatTimeoutClassification(t *testing.T) {
	coll, _ := testCollection(t)
	seedCompatDocs(t, compatSeed())

	saved := operationTimeout
	operationTimeout = time.Nanosecond
	defer func() { operationTimeout = saved }()

	var got []compatDoc
	err := coll.Find(bson.M{}).All(&got)
	if err == nil {
		t.Fatal("find under a 1ns operation timeout must fail")
	}
	if got := classifyError(err); got != "timeout" {
		t.Fatalf("classifyError = %q, want timeout (err: %v)", got, err)
	}
	if Err(err) {
		t.Fatal("timeout must not map to legacy success")
	}
}

func TestDialUnreachableFails(t *testing.T) {
	saved := connectTimeout
	connectTimeout = 500 * time.Millisecond
	defer func() { connectTimeout = saved }()
	if err := dialMongo("mongodb://127.0.0.1:1/"); err == nil {
		t.Fatal("dial to a closed port must fail")
	}
}

func TestParseSortFields(t *testing.T) {
	d := parseSortFields([]string{"-Count", "Usn", "+Title"})
	want := bson.D{
		{Key: "Count", Value: -1},
		{Key: "Usn", Value: 1},
		{Key: "Title", Value: 1},
	}
	if len(d) != len(want) {
		t.Fatalf("parseSortFields length = %d, want %d", len(d), len(want))
	}
	for i := range want {
		if d[i] != want[i] {
			t.Fatalf("parseSortFields[%d] = %+v, want %+v", i, d[i], want[i])
		}
	}
}
