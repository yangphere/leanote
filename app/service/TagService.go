package service

import (
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

/*
每添加,更新note时, 都将tag添加到tags表中
*/
type TagService struct {
}

/*
func (this *TagService) GetTags(userId string) []string {
	tag := info.Tag{}
	db.Get(db.Tags, userId, &tag)
	LogJ(tag)
	return tag.Tags
}
*/

func (this *TagService) AddTagsI(userId string, tags interface{}) bool {
	if ts, ok2 := tags.([]string); ok2 {
		return this.AddTags(userId, ts)
	}
	return false
}
func (this *TagService) AddTags(userId string, tags []string) bool {
	for _, tag := range tags {
		if !db.Upsert(db.Tags,
			bson.M{"_id": db.MustObjectIDFromHex(userId)},
			bson.M{"$addToSet": bson.M{"Tags": tag}}) {
			return false
		}
	}
	return true
}

//---------------------------
// v2
// 第二版标签, 单独一张表, 每一个tag一条记录

// 添加或更新标签, 先查下是否存在, 不存在则添加, 存在则更新
// 都要统计下tag的note数
// 什么时候调用? 笔记添加Tag, 删除Tag时
// 删除note时, 都可以调用
// 万能
func (this *TagService) AddOrUpdateTag(userId string, tag string) info.NoteTag {
	userIdO := db.MustObjectIDFromHex(userId)
	noteTag := info.NoteTag{}
	db.GetByQ(db.NoteTags, bson.M{"UserId": userIdO, "Tag": tag}, &noteTag)

	// 存在, 则更新之
	if !noteTag.TagId.IsZero() {
		// 统计note数
		count := noteService.CountNoteByTag(userId, tag)
		noteTag.Count = count
		noteTag.UpdatedTime = time.Now()
		//		noteTag.Usn = userService.IncrUsn(userId), 更新count而已

		// 之前删除过的, 现在要添加回来了
		if noteTag.IsDeleted {
			Log("之前删除过的, 现在要添加回来了:  " + tag)
			noteTag.Usn = userService.IncrUsn(userId)
			noteTag.IsDeleted = false
		}

		db.UpdateByIdAndUserId(db.NoteTags, noteTag.TagId.Hex(), userId, noteTag)
		return noteTag
	}

	// 不存在, 则创建之
	noteTag.TagId = db.NewObjectID()
	noteTag.Count = 1
	noteTag.Tag = tag
	noteTag.UserId = db.MustObjectIDFromHex(userId)
	noteTag.CreatedTime = time.Now()
	noteTag.UpdatedTime = noteTag.CreatedTime
	noteTag.Usn = userService.IncrUsn(userId)
	noteTag.IsDeleted = false
	db.Insert(db.NoteTags, noteTag)

	return noteTag
}

// 得到标签, 按更新时间来排序
func (this *TagService) GetTags(userId string) []info.NoteTag {
	tags := []info.NoteTag{}
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsDeleted": false}
	q := db.NoteTags.Find(query)
	sortFieldR := "-UpdatedTime"
	q.Sort(sortFieldR).All(&tags)
	return tags
}

// 删除标签
// 也删除所有的笔记含该标签的
// 返回noteId => usn
func (this *TagService) DeleteTag(userId string, tag string) map[string]int {
	usn := userService.IncrUsn(userId)
	if db.UpdateByQMap(db.NoteTags, bson.M{"UserId": db.MustObjectIDFromHex(userId), "Tag": tag}, bson.M{"Usn": usn, "IsDeleted": true}) {
		return noteService.UpdateNoteToDeleteTag(userId, tag)
	}
	return map[string]int{}
}

// 删除标签, 供API调用
func (this *TagService) DeleteTagApi(userId string, tag string, usn int) (ok bool, msg string, toUsn int) {
	noteTag := info.NoteTag{}
	db.GetByQ(db.NoteTags, bson.M{"UserId": db.MustObjectIDFromHex(userId), "Tag": tag}, &noteTag)

	if noteTag.TagId.IsZero() {
		return false, "notExists", 0
	}
	if noteTag.Usn > usn {
		return false, "conflict", 0
	}
	toUsn = userService.IncrUsn(userId)
	if db.UpdateByQMap(db.NoteTags, bson.M{"UserId": db.MustObjectIDFromHex(userId), "Tag": tag}, bson.M{"Usn": usn, "IsDeleted": true}) {
		return true, "", toUsn
	}
	return false, "", 0
}

// 重新统计标签的count
func (this *TagService) reCountTagCount(userId string, tags []string) {
	if tags == nil {
		return
	}
	for _, tag := range tags {
		this.AddOrUpdateTag(userId, tag)
	}
}

// 同步用
func (this *TagService) GeSyncTags(userId string, afterUsn, maxEntry int) []info.NoteTag {
	noteTags := []info.NoteTag{}
	q := db.NoteTags.Find(bson.M{"UserId": db.MustObjectIDFromHex(userId), "Usn": bson.M{"$gt": afterUsn}})
	q.Sort("Usn").Limit(maxEntry).All(&noteTags)
	return noteTags
}
