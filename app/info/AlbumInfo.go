package info

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

type Album struct {
	AlbumId     bson.ObjectId `bson:"_id,omitempty"` //
	UserId      bson.ObjectId `bson:"UserId"`
	Name        string        `bson:"Name"` // album name
	Type        int           `bson:"Type"` // type, the default is image: 0
	Seq         int           `bson:"Seq"`
	CreatedTime time.Time     `bson:"CreatedTime"`
}
