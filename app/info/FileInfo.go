package info

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

type File struct {
	FileId         bson.ObjectId `bson:"_id,omitempty"` //
	UserId         bson.ObjectId `bson:"UserId"`
	AlbumId        bson.ObjectId `bson:"AlbumId"`
	Name           string        `bson:"Name"`  // file name
	Title          string        `bson:"Title"` // file name or user defined for search
	Size           int64         `bson:"Size"`  // file size (byte)
	Type           string        `bson:"Type"`  // file type, "" = image, "doc" = word
	Path           string        `bson:"Path"`  // the file path
	IsDefaultAlbum bool          `bson:"IsDefaultAlbum"`
	CreatedTime    time.Time     `bson:"CreatedTime"`

	FromFileId bson.ObjectId `bson:"FromFileId,omitempty"` // copy from fileId, for collaboration
}
