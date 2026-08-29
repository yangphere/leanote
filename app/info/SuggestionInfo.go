package info

import (
	"github.com/yangphere/leanote/app/lea"
)

// 建议
type Suggestion struct {
	Id         lea.ObjectID `bson:"_id"`
	UserId     lea.ObjectID `bson:"UserId"`
	Addr       string       `bson:"Addr"`
	Suggestion string       `bson:"Suggestion"`
}
