package info

import (
	"github.com/yangphere/leanote/app/domain"
	"time"
)

// 分组
type Group struct {
	GroupId     domain.ObjectID `bson:"_id"`       // 谁的
	UserId      domain.ObjectID `bson:"UserId"`    // 所有者Id
	Title       string          `bson:"Title"`     // 标题
	UserCount   int             `bson:"UserCount"` // 用户数
	CreatedTime time.Time       `bson:"CreatedTime"`

	Users []User `bson:"Users,omitempty"` // 分组下的用户, 不保存, 仅查看
}

// 分组好友
type GroupUser struct {
	GroupUserId domain.ObjectID `bson:"_id"`     // 谁的
	GroupId     domain.ObjectID `bson:"GroupId"` // 分组
	UserId      domain.ObjectID `bson:"UserId"`  //  用户
	CreatedTime time.Time       `bson:"CreatedTime"`
}
