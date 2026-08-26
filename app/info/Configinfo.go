package info

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

// 配置, 每一个配置一行记录
type Config struct {
	ConfigId    bson.ObjectId       `bson:"_id"`
	UserId      bson.ObjectId       `bson:"UserId"`
	Key         string              `bson:"Key"`
	ValueStr    string              `bson:"ValueStr,omitempty"`    // "1"
	ValueArr    []string            `bson:"ValueArr,omitempty"`    // ["1","b","c"]
	ValueMap    map[string]string   `bson:"ValueMap,omitempty"`    // {"a":"bb", "CC":"xx"}
	ValueArrMap []map[string]string `bson:"ValueArrMap,omitempty"` // [{"a":"B"}, {}, {}]
	IsArr       bool                `bson:"IsArr"`                 // 是否是数组
	IsMap       bool                `bson:"IsMap"`                 // 是否是Map
	IsArrMap    bool                `bson:"IsArrMap"`              // 是否是数组Map

	// StringConfigs map[string]string   `StringConfigs` // key => value
	// ArrayConfigs  map[string][]string `ArrayConfigs`  // key => []value

	UpdatedTime time.Time `bson:"UpdatedTime"`
}
