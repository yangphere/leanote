package service

import (
	"reflect"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestActiveNotebookQueryExcludesTombstones(t *testing.T) {
	query := activeNotebookQuery("507f1f77bcf86cd799439011")
	if !reflect.DeepEqual(query["IsDeleted"], bson.M{"$ne": true}) {
		t.Fatalf("active notebook query=%v", query)
	}
	if query["UserId"] != db.MustObjectIDFromHex("507f1f77bcf86cd799439011") {
		t.Fatalf("active notebook query is not owner scoped: %v", query)
	}
}
