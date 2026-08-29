package info

import (
	"github.com/yangphere/leanote/app/lea"
)

// 笔记内部图片
type NoteImage struct {
	NoteImageId lea.ObjectID `bson:"_id,omitempty"` // 必须要设置bson:"_id" 不然mgo不会认为是主键
	NoteId      lea.ObjectID `bson:"NoteId"`        // 笔记
	ImageId     lea.ObjectID `bson:"ImageId"`       // 图片fileId
}
