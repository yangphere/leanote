package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// Attach belongs to note
type Attach struct {
	AttachId     lea.ObjectID `bson:"_id,omitempty"` //
	NoteId       lea.ObjectID `bson:"NoteId"`        //
	UploadUserId lea.ObjectID `bson:"UploadUserId"`  // 可以不是note owner, 协作者userId
	Name         string       `bson:"Name"`          // file name, md5, such as 13232312.doc
	Title        string       `bson:"Title"`         // raw file name
	Size         int64        `bson:"Size"`          // file size (byte)
	Type         string       `bson:"Type"`          // file type, "doc" = word
	Path         string       `bson:"Path"`          // the file path such as: files/userId/attachs/adfadf.doc
	CreatedTime  time.Time    `bson:"CreatedTime"`

	// FromFileId lea.ObjectID `bson:"FromFileId,omitempty"` // copy from fileId, for collaboration
}
