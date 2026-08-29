package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

type File struct {
	FileId         lea.ObjectID `bson:"_id,omitempty"` //
	UserId         lea.ObjectID `bson:"UserId"`
	AlbumId        lea.ObjectID `bson:"AlbumId"`
	Name           string       `bson:"Name"`  // file name
	Title          string       `bson:"Title"` // file name or user defined for search
	Size           int64        `bson:"Size"`  // file size (byte)
	Type           string       `bson:"Type"`  // file type, "" = image, "doc" = word
	Path           string       `bson:"Path"`  // the file path
	IsDefaultAlbum bool         `bson:"IsDefaultAlbum"`
	CreatedTime    time.Time    `bson:"CreatedTime"`

	FromFileId lea.ObjectID `bson:"FromFileId,omitempty"` // copy from fileId, for collaboration
}
