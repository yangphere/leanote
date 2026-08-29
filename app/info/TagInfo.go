package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 这里主要是为了统计每个tag的note数目
// 暂时没用
/*
type TagNote struct {
	TagId   lea.ObjectID `bson:"_id,omitempty"` // 必须要设置bson:"_id" 不然mgo不会认为是主键
	UserId  lea.ObjectID `bson:"UserId"`
	Tag   string        `Title`   // 标题
	NoteNum int           `NoteNum` // note数目
}
*/

// 每个用户一条记录, 存储用户的所有tags
type Tag struct {
	UserId lea.ObjectID `bson:"_id"`
	Tags   []string     `bson:"Tags"`
}

// v2 版标签
type NoteTag struct {
	TagId       lea.ObjectID `bson:"_id"`
	UserId      lea.ObjectID `bson:"UserId"` // 谁的
	Tag         string       `bson:"Tag"`    // UserId, Tag是唯一索引
	Usn         int          `bson:"Usn"`    // Update Sequence Number
	Count       int          `bson:"Count"`  // 笔记数
	CreatedTime time.Time    `bson:"CreatedTime"`
	UpdatedTime time.Time    `bson:"UpdatedTime"`
	IsDeleted   bool         `bson:"IsDeleted"` // 删除位
}

type TagCount struct {
	TagCountId lea.ObjectID `bson:"_id,omitempty"`
	UserId     lea.ObjectID `bson:"UserId"` // 谁的
	Tag        string       `bson:"Tag"`
	IsBlog     bool         `bson:"IsBlog"` // 是否是博客的tag统计
	Count      int          `bson:"Count"`  // 统计数量
}

/*
type TagsCounts []TagCount
func (this TagsCounts) Len() int {
	return len(this)
}
func (this TagsCounts) Less(i, j int) bool {
	return this[i].Count > this[j].Count
}
func (this TagsCounts) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}
*/
