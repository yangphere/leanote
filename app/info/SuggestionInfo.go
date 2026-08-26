package info

import (
	"gopkg.in/mgo.v2/bson"
)

// 建议
type Suggestion struct {
	Id         bson.ObjectId `bson:"_id"`
	UserId     bson.ObjectId `bson:"UserId"`
	Addr       string        `bson:"Addr"`
	Suggestion string        `bson:"Suggestion"`
}
