package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 随机token
// 验证邮箱
// 找回密码

// token type
const (
	TokenPwd = iota
	TokenActiveEmail
	TokenUpdateEmail
)

// 过期时间
const (
	PwdOverHours         = 2.0
	ActiveEmailOverHours = 48.0
	UpdateEmailOverHours = 2.0
)

type Token struct {
	UserId      lea.ObjectID `bson:"_id"`
	Email       string       `bson:"Email"`
	Token       string       `bson:"Token"`
	Type        int          `bson:"Type"`
	CreatedTime time.Time    `bson:"CreatedTime"`
}
