package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
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
	if !db.IsValidObjectIDHex(userId) {
		return false
	}
	return this.addTags(context.Background(), db.MustObjectIDFromHex(userId), tags) == nil
}

func (this *TagService) addTags(ctx context.Context, ownerID ObjectID, tags []string) error {
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if _, err := db.Tags.UpsertContext(ctx,
			bson.M{"_id": ownerID},
			bson.M{"$addToSet": bson.M{"Tags": tag}},
		); err != nil {
			return err
		}
	}
	return nil
}

func (this *TagService) verifyTags(ctx context.Context, ownerID ObjectID, tags []string) (bool, error) {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "" {
			filtered = append(filtered, tag)
		}
	}
	if len(filtered) == 0 {
		return true, nil
	}
	var stored info.Tag
	err := db.Tags.FindContext(ctx, bson.M{"_id": ownerID, "Tags": bson.M{"$all": filtered}}).One(&stored)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
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
	noteTag, _, _ := this.AddOrUpdateTagResult(userId, tag)
	return noteTag
}

func (this *TagService) AddOrUpdateTagResult(userId string, tag string) (info.NoteTag, bool, string) {
	if !db.IsValidObjectIDHex(userId) || tag == "" {
		return info.NoteTag{}, false, "validation"
	}
	userIdO := db.MustObjectIDFromHex(userId)
	noteTag := info.NoteTag{}
	if db.NoteTags == nil {
		return info.NoteTag{}, false, "storage"
	}
	if err := db.NoteTags.FindContext(context.Background(), bson.M{"UserId": userIdO, "Tag": tag}).One(&noteTag); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return info.NoteTag{}, false, "storage"
	}

	// 存在, 则更新之
	if !noteTag.TagId.IsZero() {
		originalUSN := noteTag.Usn
		// 统计note数
		count := noteService.CountNoteByTag(userId, tag)
		noteTag.Count = count
		noteTag.UpdatedTime = time.Now()
		//		noteTag.Usn = userService.IncrUsn(userId), 更新count而已

		// 之前删除过的, 现在要添加回来了
		if noteTag.IsDeleted {
			Log("之前删除过的, 现在要添加回来了:  " + tag)
			usn, err := userService.AllocateUsn(context.Background(), userId)
			if err != nil {
				return info.NoteTag{}, false, "storage"
			}
			noteTag.Usn = usn
			noteTag.IsDeleted = false
		}

		if err := db.NoteTags.UpdateOneMatchedContext(context.Background(), bson.M{"_id": noteTag.TagId, "UserId": userIdO, "Usn": originalUSN}, bson.M{"$set": bson.M{"Count": noteTag.Count, "Tag": noteTag.Tag, "IsDeleted": noteTag.IsDeleted, "Usn": noteTag.Usn, "UpdatedTime": noteTag.UpdatedTime}}); err != nil {
			return info.NoteTag{}, false, "storage"
		}
		return noteTag, true, ""
	}

	// 不存在, 则创建之
	noteTag.TagId = db.NewObjectID()
	noteTag.Count = 1
	noteTag.Tag = tag
	noteTag.UserId = db.MustObjectIDFromHex(userId)
	noteTag.CreatedTime = time.Now()
	noteTag.UpdatedTime = noteTag.CreatedTime
	usn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return info.NoteTag{}, false, "storage"
	}
	noteTag.Usn = usn
	noteTag.IsDeleted = false
	if !db.Insert(db.NoteTags, noteTag) {
		return info.NoteTag{}, false, "storage"
	}

	return noteTag, true, ""
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
	items, _ := this.DeleteTagResult(userId, tag)
	return items
}

func (this *TagService) DeleteTagResult(userId string, tag string) (map[string]int, bool) {
	if !db.IsValidObjectIDHex(userId) || tag == "" {
		return nil, false
	}
	userID := db.MustObjectIDFromHex(userId)
	noteTag := info.NoteTag{}
	if db.NoteTags == nil {
		return nil, false
	}
	if err := db.NoteTags.FindContext(context.Background(), bson.M{"UserId": userID, "Tag": tag}).One(&noteTag); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false
	}
	if noteTag.TagId.IsZero() {
		return nil, false
	}
	request := struct {
		TagID string
		Tag   string
	}{TagID: noteTag.TagId.Hex(), Tag: tag}
	operationID, digest, desired, err := applicationnotes.NewOperationIdentity("web_tag_detach", userID, noteTag.TagId, request)
	if err != nil {
		return nil, false
	}
	receipt, getErr := db.GetWorkspaceOperation(context.Background(), userID, operationID)
	if getErr == nil && receipt.Status == applicationnotes.OperationCommitted {
		if noteTag.IsDeleted && receipt.StepUSNs["tag_tombstone"] == noteTag.Usn {
			return tagDetachResultFromReceipt(receipt), true
		}
		// The old committed receipt belongs to an earlier tag generation. A
		// later reactivation is a new real delete, not a retry of that receipt.
		generationRequest := struct {
			TagID      string
			Tag        string
			Generation int
		}{TagID: noteTag.TagId.Hex(), Tag: tag, Generation: noteTag.Usn}
		operationID, digest, desired, err = applicationnotes.NewOperationIdentity("web_tag_detach", userID, noteTag.TagId, generationRequest)
		if err != nil {
			return nil, false
		}
		getErr = mongo.ErrNoDocuments
	} else if getErr != nil && !errors.Is(getErr, mongo.ErrNoDocuments) {
		return nil, false
	}
	type frozenDetachState struct {
		Tag   info.NoteTag
		Notes []info.Note
	}
	notes := []info.Note{}
	if getErr == nil {
		if receipt.Status == applicationnotes.OperationCommitted {
			return tagDetachResultFromReceipt(receipt), true
		}
		// Recovery must use the first receipt's before-image. Re-querying here
		// can add notes tagged concurrently after the operation began.
		var frozen frozenDetachState
		if len(receipt.BeforeState) == 0 || json.Unmarshal(receipt.BeforeState, &frozen) != nil || frozen.Tag.TagId.IsZero() {
			return nil, false
		}
		noteTag, notes = frozen.Tag, append([]info.Note(nil), frozen.Notes...)
	} else if !errors.Is(getErr, mongo.ErrNoDocuments) {
		return nil, false
	} else if err := db.Notes.Find(bson.M{"UserId": userID, "Tags": bson.M{"$in": []string{tag}}, "IsDeleted": false}).All(&notes); err != nil {
		return nil, false
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].NoteId.Hex() < notes[j].NoteId.Hex() })
	ret := map[string]int{}
	noteStates := make(map[string]info.Note, len(notes))
	noteIDs := make([]string, 0, len(notes))
	for _, note := range notes {
		id := note.NoteId.Hex()
		noteStates[id] = note
		noteIDs = append(noteIDs, id)
	}
	beforeState, stateErr := json.Marshal(frozenDetachState{Tag: noteTag, Notes: notes})
	if stateErr != nil {
		return nil, false
	}
	plan := db.WorkspaceMutationPlan{OperationID: operationID, OwnerID: userID, ResourceID: noteTag.TagId, Kind: "web_tag_detach", InputDigest: digest, BeforeState: beforeState, DesiredState: desired, FailurePolicy: applicationnotes.FailurePending}
	plan.RestoreBeforeState = func(payload []byte) error {
		var frozen frozenDetachState
		if err := json.Unmarshal(payload, &frozen); err != nil {
			return err
		}
		if frozen.Tag.TagId != noteTag.TagId || frozen.Tag.UserId != userID || !sameNoteIDSet(frozen.Notes, noteIDs) {
			return errors.New("tag detach before state target set changed")
		}
		noteTag = frozen.Tag
		noteStates = make(map[string]info.Note, len(frozen.Notes))
		for _, note := range frozen.Notes {
			noteStates[note.NoteId.Hex()] = note
		}
		return nil
	}
	if !noteTag.IsDeleted {
		assignedUSN := 0
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "tag_tombstone", ReplaySafe: true,
			AssignedUSN:        func() int { return assignedUSN },
			RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
			Apply: func(ctx context.Context) error {
				originalUSN := noteTag.Usn
				usn := assignedUSN
				if usn <= 0 {
					var err error
					usn, err = userService.AllocateUsn(ctx, userId)
					if err != nil {
						return err
					}
					assignedUSN = usn
				}
				return db.NoteTags.UpdateOneMatchedContext(ctx, bson.M{"_id": noteTag.TagId, "UserId": userID, "Usn": originalUSN, "IsDeleted": false}, bson.M{"$set": bson.M{"Usn": usn, "IsDeleted": true, "UpdatedTime": time.Now()}})
			},
			Verify: func(ctx context.Context) (bool, error) {
				if assignedUSN <= 0 {
					return false, nil
				}
				var found info.NoteTag
				err := db.NoteTags.FindContext(ctx, bson.M{"_id": noteTag.TagId, "UserId": userID, "Usn": assignedUSN, "IsDeleted": true}).One(&found)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
		})
	}
	for _, noteID := range noteIDs {
		assignedUSN := 0
		tagsFor := func(note info.Note) []string {
			tags := make([]string, 0, len(note.Tags))
			for _, value := range note.Tags {
				if value != tag {
					tags = append(tags, value)
				}
			}
			return tags
		}
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "note:" + noteID, ReplaySafe: true,
			AssignedUSN:        func() int { return assignedUSN },
			RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
			Apply: func(ctx context.Context) error {
				note, ok := noteStates[noteID]
				if !ok {
					return errors.New("tag detach before state note missing")
				}
				usn := assignedUSN
				if usn <= 0 {
					var err error
					usn, err = userService.AllocateUsn(ctx, userId)
					if err != nil {
						return err
					}
					assignedUSN = usn
				}
				tags := tagsFor(note)
				filter := bson.M{"_id": note.NoteId, "UserId": userID, "Usn": note.Usn, "IsDeleted": false}
				db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
				if err := db.Notes.UpdateOneMatchedContext(ctx, filter, bson.M{"$set": bson.M{"Usn": usn, "Tags": tags}}); err != nil {
					return err
				}
				ret[note.NoteId.Hex()] = usn
				return nil
			},
			Verify: func(ctx context.Context) (bool, error) {
				note, ok := noteStates[noteID]
				if !ok || assignedUSN <= 0 {
					return false, nil
				}
				tags := tagsFor(note)
				var found info.Note
				err := db.Notes.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": userID, "Usn": assignedUSN, "Tags": tags, "IsDeleted": false}).One(&found)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				if err == nil {
					ret[note.NoteId.Hex()] = found.Usn
				}
				return err == nil, err
			},
		})
	}
	if len(plan.Steps) == 0 {
		return ret, true
	}
	result, err := db.RunWorkspaceRepair(context.Background(), plan)
	return ret, err == nil && result.Committed
}

func tagDetachResultFromReceipt(receipt applicationnotes.OperationReceipt) map[string]int {
	result := make(map[string]int)
	for step, usn := range receipt.StepUSNs {
		if strings.HasPrefix(step, "note:") && usn > 0 {
			result[strings.TrimPrefix(step, "note:")] = usn
		}
	}
	return result
}

func sameNoteIDSet(notes []info.Note, ids []string) bool {
	if len(notes) != len(ids) {
		return false
	}
	seen := make(map[string]struct{}, len(notes))
	for _, note := range notes {
		seen[note.NoteId.Hex()] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			return false
		}
	}
	return true
}

// 删除标签, 供API调用
func (this *TagService) DeleteTagApi(userId string, tag string, usn int) (ok bool, msg string, toUsn int) {
	if !db.IsValidObjectIDHex(userId) {
		return false, "notExists", 0
	}
	noteTag := info.NoteTag{}
	if db.NoteTags == nil {
		return false, "storage", 0
	}
	if err := db.NoteTags.FindContext(context.Background(), bson.M{"UserId": db.MustObjectIDFromHex(userId), "Tag": tag}).One(&noteTag); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, "notExists", 0
		}
		return false, "storage", 0
	}

	if noteTag.TagId.IsZero() || noteTag.IsDeleted {
		return false, "notExists", 0
	}
	if noteTag.Usn != usn {
		return false, "conflict", 0
	}
	var err error
	toUsn, err = userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false, "storage", 0
	}
	if err := db.NoteTags.UpdateOneMatchedContext(context.Background(),
		bson.M{"UserId": db.MustObjectIDFromHex(userId), "Tag": tag, "Usn": usn, "IsDeleted": false},
		bson.M{"$set": bson.M{"Usn": toUsn, "IsDeleted": true, "UpdatedTime": time.Now()}},
	); err == nil {
		return true, "", toUsn
	} else if !errors.Is(err, db.ErrDocumentNotFound) {
		return false, "storage", 0
	}
	return false, "conflict", 0
}

// 重新统计标签的count
func (this *TagService) reCountTagCount(userId string, tags []string) {
	this.reCountTagCountResult(userId, tags)
}

func (this *TagService) reCountTagCountResult(userId string, tags []string) bool {
	if !db.IsValidObjectIDHex(userId) {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		if err := this.reCountExistingTag(context.Background(), ownerID, tag); err != nil {
			return false
		}
	}
	return true
}

func (this *TagService) reCountExistingTag(ctx context.Context, ownerID ObjectID, tag string) error {
	count, err := db.Notes.FindContext(ctx, bson.M{
		"UserId": ownerID, "IsDeleted": false, "Tags": bson.M{"$in": []string{tag}},
	}).Count()
	if err != nil {
		return err
	}
	_, err = db.NoteTags.UpdateAllContext(ctx,
		bson.M{"UserId": ownerID, "Tag": tag, "IsDeleted": false},
		bson.M{"$set": bson.M{"Count": count, "UpdatedTime": time.Now()}},
	)
	return err
}

func (this *TagService) verifyExistingTagCount(ctx context.Context, ownerID ObjectID, tag string) (bool, error) {
	count, err := db.Notes.FindContext(ctx, bson.M{
		"UserId": ownerID, "IsDeleted": false, "Tags": bson.M{"$in": []string{tag}},
	}).Count()
	if err != nil {
		return false, err
	}
	var noteTag info.NoteTag
	err = db.NoteTags.FindContext(ctx, bson.M{"UserId": ownerID, "Tag": tag}).One(&noteTag)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return noteTag.IsDeleted || noteTag.Count == count, nil
}

func (this *TagService) verifyTagCounts(ctx context.Context, ownerID ObjectID, tags []string) (bool, error) {
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		verified, err := this.verifyExistingTagCount(ctx, ownerID, tag)
		if err != nil || !verified {
			return false, err
		}
	}
	return true, nil
}

// 同步用
func (this *TagService) GeSyncTags(userId string, afterUsn, maxEntry int) []info.NoteTag {
	noteTags := []info.NoteTag{}
	q := db.NoteTags.Find(bson.M{"UserId": db.MustObjectIDFromHex(userId), "Usn": bson.M{"$gt": afterUsn}})
	q.Sort("Usn").Limit(maxEntry).All(&noteTags)
	return noteTags
}
