package info

import (
	"github.com/yangphere/leanote/app/lea"
	"time"
)

// 只为blog, 不为note

type BlogItem struct {
	Note
	Abstract string
	Content  string // 可能是content的一部分, 截取. 点击more后就是整个信息了
	HasMore  bool   // 是否是否还有
	User     User   // 用户信息
}

type UserBlogBase struct {
	Logo     string `bson:"Logo"`
	Title    string `bson:"Title"`    // 标题
	SubTitle string `bson:"SubTitle"` // 副标题
	//	AboutMe  string `AboutMe`  // 关于我
}

type UserBlogComment struct {
	CanComment  bool   `bson:"CanComment"`  // 是否可以评论
	CommentType string `bson:"CommentType"` // default 或 disqus
	DisqusId    string `bson:"DisqusId"`
}

type UserBlogStyle struct {
	Style string `bson:"Style"` // 风格
	Css   string `bson:"Css"`   // 自定义css
}

// 每个用户一份博客设置信息
type UserBlog struct {
	UserId   lea.ObjectID `bson:"_id"` // 谁的
	Logo     string       `bson:"Logo"`
	Title    string       `bson:"Title"`    // 标题
	SubTitle string       `bson:"SubTitle"` // 副标题
	AboutMe  string       `bson:"AboutMe"`  // 关于我, 弃用

	CanComment bool `bson:"CanComment"` // 是否可以评论

	CommentType string `bson:"CommentType"` // default 或 disqus
	DisqusId    string `bson:"DisqusId"`

	Style string `bson:"Style"` // 风格
	Css   string `bson:"Css"`   // 自定义css

	ThemeId   lea.ObjectID `bson:"ThemeId,omitempty"`  // 主题Id
	ThemePath string       `bson:"ThemePath" json:"-"` // 不存值, 从Theme中获取, 相对路径 public/

	CateIds []string            `bson:"CateIds,omitempty"` // 分类Id, 排序好的
	Singles []map[string]string `bson:"Singles,omitempty"` // 单页, 排序好的, map包含: ["Title"], ["SingleId"]

	PerPageSize int    `bson:"PerPageSize,omitempty"`
	SortField   string `bson:"SortField"`       // 排序字段
	IsAsc       bool   `bson:"IsAsc,omitempty"` // 排序类型, 降序, 升序, 默认是false, 表示降序

	SubDomain string `bson:"SubDomain"` // 二级域名
	Domain    string `bson:"Domain"`    // 自定义域名

}

// 博客统计信息
type BlogStat struct {
	NoteId     lea.ObjectID `bson:"_id,omitempty"`
	ReadNum    int          `bson:"ReadNum,omitempty"`    // 阅读次数 2014/9/28
	LikeNum    int          `bson:"LikeNum,omitempty"`    // 点赞次数 2014/9/28
	CommentNum int          `bson:"CommentNum,omitempty"` // 评论次数 2014/9/28
}

// 单页
type BlogSingle struct {
	SingleId    lea.ObjectID `bson:"_id,omitempty"`
	UserId      lea.ObjectID `bson:"UserId"`
	Title       string       `bson:"Title"`
	UrlTitle    string       `bson:"UrlTitle"` // 2014/11/11
	Content     string       `bson:"Content"`
	UpdatedTime time.Time    `bson:"UpdatedTime"`
	CreatedTime time.Time    `bson:"CreatedTime"`
}

//------------------------
// 社交功能, 点赞, 分享, 评论

// 点赞记录
type BlogLike struct {
	LikeId      lea.ObjectID `bson:"_id,omitempty"`
	NoteId      lea.ObjectID `bson:"NoteId"`
	UserId      lea.ObjectID `bson:"UserId"`
	CreatedTime time.Time    `bson:"CreatedTime"`
}

// 评论
type BlogComment struct {
	CommentId lea.ObjectID `bson:"_id,omitempty"`
	NoteId    lea.ObjectID `bson:"NoteId"`

	UserId  lea.ObjectID `bson:"UserId"`  // UserId回复ToUserId
	Content string       `bson:"Content"` // 评论内容

	ToCommentId lea.ObjectID `bson:"ToCommendId,omitempty"` // 对某条评论进行回复
	ToUserId    lea.ObjectID `bson:"ToUserId,omitempty"`    // 为空表示直接评论, 不回空表示回复某人

	LikeNum     int      `bson:"LikeNum"`     // 点赞次数, 评论也可以点赞
	LikeUserIds []string `bson:"LikeUserIds"` // 点赞的用户ids

	CreatedTime time.Time `bson:"CreatedTime"`
}

type BlogCommentPublic struct {
	BlogComment
	IsILikeIt bool
}

type BlogUrls struct {
	IndexUrl    string
	CateUrl     string
	SearchUrl   string
	SingleUrl   string
	PostUrl     string
	ArchiveUrl  string
	TagsUrl     string
	TagPostsUrl string
}
