package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 发送邮件
type EmailLog struct {
	LogId lea.ObjectID `bson:"_id"`

	Email   string `bson:"Email"`   // 发送者
	Subject string `bson:"Subject"` // 主题
	Body    string `bson:"Body"`    // 内容
	Msg     string `bson:"Msg"`     // 发送失败信息
	Ok      bool   `bson:"Ok"`      // 发送是否成功

	CreatedTime time.Time `bson:"CreatedTime"`
}
