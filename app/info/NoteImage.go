package info

import (
	"github.com/yangphere/leanote/app/domain"
)

// 笔记内部图片
type NoteImage struct {
	NoteImageId domain.ObjectID `bson:"_id,omitempty"` // 必须要设置bson:"_id" 不然mgo不会认为是主键
	NoteId      domain.ObjectID `bson:"NoteId"`        // 笔记
	ImageId     domain.ObjectID `bson:"ImageId"`       // 图片fileId
}
