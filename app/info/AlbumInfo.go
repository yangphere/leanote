package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

type Album struct {
	AlbumId     lea.ObjectID `bson:"_id,omitempty"` //
	UserId      lea.ObjectID `bson:"UserId"`
	Name        string       `bson:"Name"` // album name
	Type        int          `bson:"Type"` // type, the default is image: 0
	Seq         int          `bson:"Seq"`
	CreatedTime time.Time    `bson:"CreatedTime"`
}
