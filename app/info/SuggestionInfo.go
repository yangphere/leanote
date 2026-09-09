package info

import (
	"github.com/yangphere/leanote/app/domain"
)

// 建议
type Suggestion struct {
	Id         domain.ObjectID `bson:"_id"`
	UserId     domain.ObjectID `bson:"UserId"`
	Addr       string          `bson:"Addr"`
	Suggestion string          `bson:"Suggestion"`
}
