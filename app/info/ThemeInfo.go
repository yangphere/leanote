package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 主题, 每个用户有多个主题, 这里面有主题的配置信息
// 模板, css, js, images, 都在路径Path下
type Theme struct {
	ThemeId   lea.ObjectID           `bson:"_id,omitempty"` // 必须要设置bson:"_id" 不然mgo不会认为是主键
	UserId    lea.ObjectID           `bson:"UserId"`
	Name      string                 `bson:"Name"`
	Version   string                 `bson:"Version"`
	Author    string                 `bson:"Author"`
	AuthorUrl string                 `bson:"AuthorUrl"`
	Path      string                 `bson:"Path"`     // 文件夹路径, public/upload/54d7620d99c37b030600002c/themes/54d867c799c37b533e000001
	Info      map[string]interface{} `bson:"Info"`     // 所有信息
	IsActive  bool                   `bson:"IsActive"` // 是否在用

	IsDefault bool   `bson:"IsDefault"`       // leanote默认主题, 如果用户修改了默认主题, 则先copy之. 也是admin用户的主题
	Style     string `bson:"Style,omitempty"` // 之前的, 只有default的用户才有blog_default, blog_daqi, blog_left_fixed

	CreatedTime time.Time `bson:"CreatedTime"`
	UpdatedTime time.Time `bson:"UpdatedTime"`
}
