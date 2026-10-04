package info

import (
	"github.com/yangphere/leanote/app/domain"
	"time"
)

// 举报
type Report struct {
	ReportId domain.ObjectID `bson:"_id"`
	NoteId   domain.ObjectID `bson:"NoteId"`

	UserId domain.ObjectID `bson:"UserId"` // UserId回复ToUserId
	Reason string          `bson:"Reason"` // 评论内容

	CommentId domain.ObjectID `bson:"CommendId,omitempty"` // 对某条评论进行回复

	CreatedTime time.Time `bson:"CreatedTime"`
}
