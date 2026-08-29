package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 举报
type Report struct {
	ReportId lea.ObjectID `bson:"_id"`
	NoteId   lea.ObjectID `bson:"NoteId"`

	UserId lea.ObjectID `bson:"UserId"` // UserId回复ToUserId
	Reason string       `bson:"Reason"` // 评论内容

	CommentId lea.ObjectID `bson:"CommendId,omitempty"` // 对某条评论进行回复

	CreatedTime time.Time `bson:"CreatedTime"`
}
