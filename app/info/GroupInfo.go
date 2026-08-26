package info

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

// 分组
type Group struct {
	GroupId     bson.ObjectId `bson:"_id"`       // 谁的
	UserId      bson.ObjectId `bson:"UserId"`    // 所有者Id
	Title       string        `bson:"Title"`     // 标题
	UserCount   int           `bson:"UserCount"` // 用户数
	CreatedTime time.Time     `bson:"CreatedTime"`

	Users []User `bson:"Users,omitempty"` // 分组下的用户, 不保存, 仅查看
}

// 分组好友
type GroupUser struct {
	GroupUserId bson.ObjectId `bson:"_id"`     // 谁的
	GroupId     bson.ObjectId `bson:"GroupId"` // 分组
	UserId      bson.ObjectId `bson:"UserId"`  //  用户
	CreatedTime time.Time     `bson:"CreatedTime"`
}
