package info

import (
	"github.com/yangphere/leanote/app/domain"
	"time"
)

type Album struct {
	AlbumId     domain.ObjectID `bson:"_id,omitempty"` //
	UserId      domain.ObjectID `bson:"UserId"`
	Name        string          `bson:"Name"` // album name
	Type        int             `bson:"Type"` // type, the default is image: 0
	Seq         int             `bson:"Seq"`
	CreatedTime time.Time       `bson:"CreatedTime"`
}
