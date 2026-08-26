package info

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

// 举报
type Report struct {
	ReportId bson.ObjectId `bson:"_id"`
	NoteId   bson.ObjectId `bson:"NoteId"`

	UserId bson.ObjectId `bson:"UserId"` // UserId回复ToUserId
	Reason string        `bson:"Reason"` // 评论内容

	CommentId bson.ObjectId `bson:"CommendId,omitempty"` // 对某条评论进行回复

	CreatedTime time.Time `bson:"CreatedTime"`
}
