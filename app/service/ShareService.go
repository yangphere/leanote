package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sort"
	"time"
)

// 共享Notebook, Note服务
type ShareService struct {
}

func validShareIDs(values ...string) bool {
	for _, value := range values {
		if !db.IsValidObjectIDHex(value) {
			return false
		}
	}
	return true
}

//-----------------------------------
// 返回shareNotebooks, sharedUserInfos
// info.ShareNotebooksByUser, []info.User

// 总体来说, 这个方法比较麻烦, 速度未知. 以后按以下方案来缓存用户基础数据

// 以后建个用户的基本数据表, 放所有notebook, sharedNotebook的缓存!!
// 每更新一次则启动一个goroutine异步更新
// 共享只能共享本notebook下的, 如果其子也要共享, 必须设置其子!!!
// 那么, 父, 子在shareNotebooks表中都会有记录

// 得到用户的所有*被*共享的Notebook
// 1 得到别人共享给我的所有notebooks
// 2 按parent进行层次化
// 3 每个层按seq进行排序
// 4 按用户分组
// [ok]

// 谁共享给了我的Query
func (this *ShareService) getOrQ(userId string) bson.M {
	query, _ := this.getOrQChecked(userId)
	return query
}

func (this *ShareService) getOrQChecked(userId string) (bson.M, error) {
	if !db.IsValidObjectIDHex(userId) {
		return nil, ErrShareRecipient
	}
	groupIds, err := this.actorGroupIDs(context.Background(), db.MustObjectIDFromHex(userId))
	if err != nil {
		return nil, err
	}

	q := bson.M{}
	if len(groupIds) > 0 {
		orQ := []bson.M{
			bson.M{"ToUserId": db.MustObjectIDFromHex(userId)},
			bson.M{"ToGroupId": bson.M{"$in": groupIds}},
		}
		// 不是trash的
		q["$or"] = orQ
	} else {
		q["ToUserId"] = db.MustObjectIDFromHex(userId)
	}
	return q, nil
}

// 得到共享给我的笔记本和用户(谁共享给了我)
func (this *ShareService) GetShareNotebooks(userId string) (info.ShareNotebooksByUser, []info.User) {
	shareNotebooks, userInfos, _ := this.GetShareNotebooksChecked(userId)
	return shareNotebooks, userInfos
}

func (this *ShareService) GetShareNotebooksChecked(userId string) (info.ShareNotebooksByUser, []info.User, error) {
	if !db.IsValidObjectIDHex(userId) {
		return nil, nil, ErrShareRecipient
	}
	if db.ShareNotes == nil || db.ShareNotebooks == nil || db.Notebooks == nil || db.Notes == nil || db.Users == nil {
		return nil, nil, db.ErrMongoClientNotInitialized
	}
	query, err := this.getOrQChecked(userId)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	actorID := db.MustObjectIDFromHex(userId)

	var noteShares []info.ShareNote
	if err := db.ShareNotes.FindContext(context.Background(), query).All(&noteShares); err != nil {
		return nil, nil, fmt.Errorf("list shared note grants: %w", err)
	}
	var notebookShares []info.ShareNotebook
	if err := db.ShareNotebooks.FindContext(context.Background(), query).All(&notebookShares); err != nil {
		return nil, nil, fmt.Errorf("list shared notebook grants: %w", err)
	}

	ownerIDs := make([]ObjectID, 0, len(noteShares)+len(notebookShares))
	ownerSeen := make(map[ObjectID]struct{}, len(noteShares)+len(notebookShares))
	addOwner := func(ownerID ObjectID) {
		if ownerID.IsZero() || ownerID == actorID {
			return
		}
		if _, ok := ownerSeen[ownerID]; !ok {
			ownerSeen[ownerID] = struct{}{}
			ownerIDs = append(ownerIDs, ownerID)
		}
	}

	validNoteIDs := make([]ObjectID, 0, len(noteShares))
	validNoteOwners := make(map[ObjectID]ObjectID, len(noteShares))
	for _, share := range noteShares {
		if err := validateShareRecipient(share.ToUserId, share.ToGroupId); err != nil {
			return nil, nil, err
		}
		if !activeShareGrant(shareGrant{Perm: share.Perm, ExpiresAt: share.ExpiresAt}, now) {
			continue
		}
		validNoteIDs = append(validNoteIDs, share.NoteId)
		validNoteOwners[share.NoteId] = share.UserId
	}
	if len(validNoteIDs) > 0 {
		var notes []info.Note
		if err := db.Notes.FindContext(context.Background(), bson.M{
			"_id":       bson.M{"$in": validNoteIDs},
			"IsTrash":   false,
			"IsDeleted": false,
		}).All(&notes); err != nil {
			return nil, nil, fmt.Errorf("list shared notes: %w", err)
		}
		for _, note := range notes {
			if note.UserId == validNoteOwners[note.NoteId] {
				addOwner(note.UserId)
			}
		}
	}

	notebookCandidates := make(map[ObjectID][]info.ShareNotebook, len(notebookShares))
	for _, share := range notebookShares {
		if err := validateShareRecipient(share.ToUserId, share.ToGroupId); err != nil {
			return nil, nil, err
		}
		if !activeShareGrant(shareGrant{Perm: share.Perm, ExpiresAt: share.ExpiresAt}, now) {
			continue
		}
		notebookCandidates[share.NotebookId] = append(notebookCandidates[share.NotebookId], share)
	}
	if len(notebookCandidates) == 0 && len(ownerIDs) == 0 {
		return info.ShareNotebooksByUser{}, []info.User{}, nil
	}

	notebookIDs := make([]ObjectID, 0, len(notebookCandidates))
	for notebookID := range notebookCandidates {
		notebookIDs = append(notebookIDs, notebookID)
	}
	var notebooks []info.Notebook
	if len(notebookIDs) > 0 {
		if err := db.Notebooks.FindContext(context.Background(), bson.M{
			"_id":       bson.M{"$in": notebookIDs},
			"IsTrash":   false,
			"IsDeleted": false,
		}).All(&notebooks); err != nil {
			return nil, nil, fmt.Errorf("list shared notebooks: %w", err)
		}
	}
	notebooksByID := make(map[ObjectID]info.Notebook, len(notebooks))
	shareByNotebookID := make(map[ObjectID]info.ShareNotebook, len(notebooks))
	for _, notebook := range notebooks {
		perm, allowed, err := this.resolveNotebookPermission(context.Background(), notebook.UserId, actorID, notebook.NotebookId, now)
		if err != nil {
			return nil, nil, err
		}
		if !allowed {
			continue
		}
		selected := info.ShareNotebook{UserId: notebook.UserId, NotebookId: notebook.NotebookId, Perm: perm}
		notebooksByID[notebook.NotebookId] = notebook
		shareByNotebookID[notebook.NotebookId] = selected
		addOwner(notebook.UserId)
	}

	if len(ownerIDs) > 0 {
		var users []info.User
		if err := db.Users.FindContext(context.Background(), bson.M{"_id": bson.M{"$in": ownerIDs}}).All(&users); err != nil {
			return nil, nil, fmt.Errorf("list shared notebook owners: %w", err)
		}
		usersByID := make(map[ObjectID]info.User, len(users))
		for _, user := range users {
			usersByID[user.UserId] = user
		}
		userInfos := make([]info.User, 0, len(ownerIDs))
		for _, ownerID := range ownerIDs {
			if user, ok := usersByID[ownerID]; ok {
				userInfos = append(userInfos, user)
			}
		}

		subNotebooks := make([]info.Notebook, 0, len(notebooksByID))
		for _, notebook := range notebooksByID {
			subNotebooks = append(subNotebooks, notebook)
		}
		subTree := ParseAndSortNotebooks(subNotebooks, false, false)
		shareMap := make(map[ObjectID]info.ShareNotebook, len(shareByNotebookID))
		for notebookID, share := range shareByNotebookID {
			shareMap[notebookID] = share
		}
		sharedTree := this.parseToSubShareNotebooks(&subTree, &shareMap)
		grouped := make(info.ShareNotebooksByUser)
		for _, each := range sharedTree {
			ownerID := each.Notebook.UserId
			if ownerID.IsZero() || ownerID == actorID {
				continue
			}
			grouped[ownerID.Hex()] = append(grouped[ownerID.Hex()], each)
		}
		for ownerID, each := range grouped {
			grouped[ownerID] = sortSubShareNotebooks(each)
		}
		return grouped, userInfos, nil
	}
	return info.ShareNotebooksByUser{}, []info.User{}, nil
}

func validateShareRecipient(toUserID, toGroupID ObjectID) error {
	if toUserID.IsZero() == toGroupID.IsZero() {
		return fmt.Errorf("%w: grant must have exactly one recipient", ErrInvalidShareGrant)
	}
	return nil
}

// 排序
func sortSubShareNotebooks(eachNotebooks info.SubShareNotebooks) info.SubShareNotebooks {
	// 遍历子, 则子往上进行排序
	for _, eachNotebook := range eachNotebooks {
		if eachNotebook.Subs != nil && len(eachNotebook.Subs) > 0 {
			eachNotebook.Subs = sortSubShareNotebooks(eachNotebook.Subs)
		}
	}

	// 子排完了, 本层排
	sort.Sort(&eachNotebooks)
	return eachNotebooks
}

// 将普通的notebooks添加perm及shareNotebook信息
func (this *ShareService) parseToSubShareNotebooks(subNotebooks *info.SubNotebooks, shareNotebooksMap *map[ObjectID]info.ShareNotebook) info.SubShareNotebooks {
	subShareNotebooks := info.SubShareNotebooks{}
	for _, each := range *subNotebooks {
		shareNotebooks := info.ShareNotebooks{}
		shareNotebooks.Notebook = each.Notebook                              // 基本信息有了
		shareNotebooks.ShareNotebook = (*shareNotebooksMap)[each.NotebookId] // perm有了

		// 公用的, 单独赋值
		shareNotebooks.Seq = shareNotebooks.ShareNotebook.Seq
		shareNotebooks.NotebookId = shareNotebooks.ShareNotebook.NotebookId

		// 还有其子, 递归解析之
		if each.Subs != nil && len(each.Subs) > 0 {
			shareNotebooks.Subs = this.parseToSubShareNotebooks(&each.Subs, shareNotebooksMap)
		}
		subShareNotebooks = append(subShareNotebooks, shareNotebooks)
	}

	return subShareNotebooks
}

//-------------

// 得到共享笔记本下的notes
func (this *ShareService) ListShareNotesByNotebookId(notebookId, myUserId, sharedUserId string,
	page, pageSize int, sortField string, isAsc bool) []info.ShareNoteWithPerm {
	notes, _ := this.ListShareNotesByNotebookIdChecked(notebookId, myUserId, sharedUserId, page, pageSize, sortField, isAsc)
	return notes
}

func (this *ShareService) ListShareNotesByNotebookIdChecked(notebookId, myUserId, sharedUserId string,
	page, pageSize int, sortField string, isAsc bool) ([]info.ShareNoteWithPerm, error) {
	if !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(myUserId) || !db.IsValidObjectIDHex(sharedUserId) || db.ShareNotebooks == nil || db.ShareNotes == nil || db.Notes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	ownerID := db.MustObjectIDFromHex(sharedUserId)
	actorID := db.MustObjectIDFromHex(myUserId)
	notebookIDValue := db.MustObjectIDFromHex(notebookId)
	now := time.Now().UTC()
	_, allowed, err := this.resolveNotebookPermission(context.Background(), ownerID, actorID, notebookIDValue, now)
	if err != nil {
		return nil, fmt.Errorf("resolve shared notebook permission: %w", err)
	}
	if !allowed {
		return nil, ErrShareResource
	}

	_, sortFieldR := parsePageAndSort(page, pageSize, sortField, isAsc)
	notes := []info.Note{}
	if err := db.Notes.Find(bson.M{
		"UserId":     ownerID,
		"NotebookId": notebookIDValue,
		"IsTrash":    false,
		"IsDeleted":  false,
	}).Sort(sortFieldR).All(&notes); err != nil {
		return nil, fmt.Errorf("list shared notebook notes: %w", err)
	}
	notesWithPerm := make([]info.ShareNoteWithPerm, 0, len(notes))
	for _, note := range notes {
		notePerm, noteAllowed, err := this.ResolveNotePermission(context.Background(), ownerID, actorID, note.NoteId, now)
		if err != nil {
			return nil, fmt.Errorf("resolve shared note permission: %w", err)
		}
		if noteAllowed {
			notesWithPerm = append(notesWithPerm, info.ShareNoteWithPerm{Note: note, Perm: notePerm})
		}
	}
	return paginateShareNotes(notesWithPerm, page, pageSize), nil
}

// 得到note的perm信息
//func (this *ShareService) getNotesPerm(noteIds []ObjectID, myUserId, sharedUserId string) map[ObjectID]int {
//	shareNotes := []info.ShareNote{}
//	db.ListByQ(db.ShareNotes,
//		bson.M{
//			"NoteId": bson.M{"$in": noteIds},
//			"UserId": db.MustObjectIDFromHex(sharedUserId),
//			"ToUserId": db.MustObjectIDFromHex(myUserId)}, &shareNotes)
//
//	notesPerm := make(map[ObjectID]int, len(shareNotes))
//	for _, each := range shareNotes {
//		notesPerm[each.NoteId] = each.Perm
//	}
//
//	return notesPerm
//}

// 得到默认的单个的notes 共享集
// 如果真要支持排序, 这里得到所有共享的notes, 到noteService方再sort和limit
// 可以这样! 到时将零散的共享noteId放在用户基本数据中
// 这里不好排序
func (this *ShareService) ListShareNotes(myUserId, sharedUserId string,
	pageNumber, pageSize int, sortField string, isAsc bool) []info.ShareNoteWithPerm {
	notes, _ := this.ListShareNotesChecked(myUserId, sharedUserId, pageNumber, pageSize, sortField, isAsc)
	return notes
}

func (this *ShareService) ListShareNotesChecked(myUserId, sharedUserId string,
	pageNumber, pageSize int, sortField string, isAsc bool) ([]info.ShareNoteWithPerm, error) {
	if !db.IsValidObjectIDHex(myUserId) || !db.IsValidObjectIDHex(sharedUserId) || db.ShareNotes == nil || db.Notes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}

	q, err := this.getOrQChecked(myUserId)
	if err != nil {
		return nil, err
	}
	q["UserId"] = db.MustObjectIDFromHex(sharedUserId)

	shareNotes := []info.ShareNote{}
	if err := db.ShareNotes.Find(q).All(&shareNotes); err != nil {
		return nil, fmt.Errorf("list shared notes: %w", err)
	}

	if len(shareNotes) == 0 {
		return []info.ShareNoteWithPerm{}, nil
	}

	_, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)
	noteIds := make([]ObjectID, 0, len(shareNotes))
	for _, each := range shareNotes {
		noteIds = append(noteIds, each.NoteId)
	}
	notes := []info.Note{}
	if err := db.Notes.Find(bson.M{
		"_id":       bson.M{"$in": noteIds},
		"UserId":    db.MustObjectIDFromHex(sharedUserId),
		"IsTrash":   false,
		"IsDeleted": false,
	}).Sort(sortFieldR).All(&notes); err != nil {
		return nil, fmt.Errorf("load shared notes: %w", err)
	}
	now := time.Now().UTC()
	ownerID := db.MustObjectIDFromHex(sharedUserId)
	actorID := db.MustObjectIDFromHex(myUserId)
	notesWithPerm := make([]info.ShareNoteWithPerm, 0, len(notes))
	for _, note := range notes {
		perm, allowed, err := this.ResolveNotePermission(context.Background(), ownerID, actorID, note.NoteId, now)
		if err != nil {
			return nil, fmt.Errorf("resolve shared note permission: %w", err)
		}
		if allowed {
			notesWithPerm = append(notesWithPerm, info.ShareNoteWithPerm{Note: note, Perm: perm})
		}
	}
	return paginateShareNotes(notesWithPerm, pageNumber, pageSize), nil
}

func paginateShareNotes(notes []info.ShareNoteWithPerm, page, pageSize int) []info.ShareNoteWithPerm {
	skip, _ := parsePageAndSort(page, pageSize, "", false)
	if skip >= len(notes) {
		return []info.ShareNoteWithPerm{}
	}
	end := skip + pageSize
	if end > len(notes) {
		end = len(notes)
	}
	return notes[skip:end]
}

func (this *ShareService) notes2NotesWithPerm(notes []info.Note) {

}

// 添加一个notebook共享
// [ok]
func (this *ShareService) AddShareNotebook1(shareNotebook info.ShareNotebook) bool {
	if db.ShareNotebooks == nil {
		return false
	}
	shareNotebook.CreatedTime = time.Now()
	persist := func() error {
		return db.ShareNotebooks.Insert(shareNotebook)
	}
	if shareNotebook.ToUserId.IsZero() {
		return persist() == nil
	}
	return persistShareGrantWithProjection(
		func() error {
			return this.ensureShareProjection(context.Background(), shareNotebook.UserId, shareNotebook.ToUserId)
		}, persist,
	) == nil
}

// 添加共享笔记本
func (this *ShareService) AddShareNotebook(notebookId string, perm int, userId, email string) (bool, string, string) {
	// 通过email得到被共享的userId
	toUserId := userService.GetUserId(email)
	if toUserId == "" {
		return false, "无该用户", ""
	}
	return this.AddShareNotebookToUserId(notebookId, perm, userId, toUserId)
}

// 第三方注册时没有email
func (this *ShareService) AddShareNotebookToUserId(notebookId string, perm int, userId, toUserId string) (bool, string, string) {
	err := this.AddShareNotebookToUserIdWithOptions(notebookId, perm, userId, toUserId, ShareGrantOptions{Now: time.Now().UTC()})
	if err != nil {
		return false, err.Error(), toUserId
	}
	return true, "", toUserId
}

// 添加一个note共享
// [ok]
/*
func (this *ShareService) AddShareNote(shareNote info.ShareNote) bool {
	shareNote.CreatedTime = time.Now()
	return db.Insert(db.ShareNotes, shareNote)
}
*/
func (this *ShareService) AddShareNote(noteId string, perm int, userId, email string) (bool, string, string) {
	// 通过email得到被共享的userId
	toUserId := userService.GetUserId(email)
	if toUserId == "" {
		return false, "无该用户", ""
	}
	return this.AddShareNoteToUserId(noteId, perm, userId, toUserId)
}

// 第三方测试没有userId
func (this *ShareService) AddShareNoteToUserId(noteId string, perm int, userId, toUserId string) (bool, string, string) {
	err := this.AddShareNoteToUserIdWithOptions(noteId, perm, userId, toUserId, ShareGrantOptions{Now: time.Now().UTC()})
	if err != nil {
		return false, err.Error(), toUserId
	}
	return true, "", toUserId
}

// updatedUserId是否有查看userId noteId的权限?
// userId是所有者
func (this *ShareService) HasReadPerm(userId, updatedUserId, noteId string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(updatedUserId) || !db.IsValidObjectIDHex(noteId) {
		return false
	}
	_, allowed, err := this.ResolveNotePermission(context.Background(), db.MustObjectIDFromHex(userId), db.MustObjectIDFromHex(updatedUserId), db.MustObjectIDFromHex(noteId), time.Now().UTC())
	return err == nil && allowed
}

// updatedUserId是否有修改userId noteId的权限?
func (this *ShareService) HasUpdatePerm(userId, updatedUserId, noteId string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(updatedUserId) || !db.IsValidObjectIDHex(noteId) {
		return false
	}
	perm, allowed, err := this.ResolveNotePermission(context.Background(), db.MustObjectIDFromHex(userId), db.MustObjectIDFromHex(updatedUserId), db.MustObjectIDFromHex(noteId), time.Now().UTC())
	return err == nil && allowed && perm == 1
}

// updatedUserId是否有修改userId notebookId的权限?
func (this *ShareService) HasUpdateNotebookPerm(userId, updatedUserId, notebookId string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(updatedUserId) || !db.IsValidObjectIDHex(notebookId) {
		return false
	}
	perm, allowed, err := this.resolveNotebookPermission(context.Background(), db.MustObjectIDFromHex(userId), db.MustObjectIDFromHex(updatedUserId), db.MustObjectIDFromHex(notebookId), time.Now().UTC())
	return err == nil && allowed && perm == 1
}

// 共享note, notebook时使用
func (this *ShareService) AddHasShareNote(userId, toUserId string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(toUserId) || userId == toUserId || db.HasShareNotes == nil {
		return false
	}
	err := this.ensureShareProjection(context.Background(), db.MustObjectIDFromHex(userId), db.MustObjectIDFromHex(toUserId))
	return err == nil
}

// userId是否被共享了noteId
func (this *ShareService) HasSharedNote(noteId, myUserId string) bool {
	if !validShareIDs(noteId, myUserId) || db.ShareNotes == nil {
		return false
	}
	return db.Has(db.ShareNotes, bson.M{"ToUserId": db.MustObjectIDFromHex(myUserId), "NoteId": db.MustObjectIDFromHex(noteId)})
}

// noteId的notebook是否共享了给我
func (this *ShareService) HasSharedNotebook(noteId, myUserId, sharedUserId string) bool {
	if !validShareIDs(noteId, myUserId, sharedUserId) || db.ShareNotebooks == nil || noteService == nil {
		return false
	}
	notebookId := noteService.GetNotebookId(noteId)
	if !notebookId.IsZero() {
		return db.Has(db.ShareNotebooks, bson.M{"NotebookId": notebookId,
			"UserId":   db.MustObjectIDFromHex(sharedUserId),
			"ToUserId": db.MustObjectIDFromHex(myUserId),
		})
	}
	return false
}

// 得到共享的笔记内容
// 并返回笔记的权限!!!
func (this *ShareService) GetShareNoteContent(noteId, myUserId, sharedUserId string) (noteContent info.NoteContent) {
	noteContent, _ = this.GetShareNoteContentChecked(noteId, myUserId, sharedUserId)
	return noteContent
}

func (this *ShareService) GetShareNoteContentChecked(noteId, myUserId, sharedUserId string) (info.NoteContent, error) {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(myUserId) || !db.IsValidObjectIDHex(sharedUserId) || db.NoteContents == nil {
		return info.NoteContent{}, db.ErrMongoClientNotInitialized
	}
	_, allowed, err := this.ResolveNotePermission(context.Background(), db.MustObjectIDFromHex(sharedUserId), db.MustObjectIDFromHex(myUserId), db.MustObjectIDFromHex(noteId), time.Now().UTC())
	if err != nil {
		return info.NoteContent{}, err
	}
	if !allowed {
		return info.NoteContent{}, ErrShareResource
	}
	noteContent := info.NoteContent{}
	if err := db.NoteContents.Find(bson.M{"_id": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(sharedUserId)}).One(&noteContent); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.NoteContent{}, ErrShareResource
		}
		return info.NoteContent{}, fmt.Errorf("load shared note content: %w", err)
	}
	return noteContent, nil
}

// 查看note的分享信息
// 分享给了哪些用户和权限
// ShareNotes表 userId = me, noteId = ...
// 还要查看该note的notebookId分享的信息
func (this *ShareService) ListNoteShareUserInfo(noteId, userId string) []info.ShareUserInfo {
	if !validShareIDs(noteId, userId) || db.ShareNotes == nil || db.ShareNotebooks == nil {
		return nil
	}
	// 得到shareNote信息, 得到所有的ToUserId
	shareNotes := []info.ShareNote{}
	db.ListByQLimit(db.ShareNotes,
		bson.M{
			"NoteId":    db.MustObjectIDFromHex(noteId),
			"UserId":    db.MustObjectIDFromHex(userId),
			"ToGroupId": bson.M{"$exists": false},
		}, &shareNotes, 100)

	//	Log("<<>>>>")
	//	Log(len(shareNotes))

	if len(shareNotes) == 0 {
		return nil
	}

	shareNotesMap := make(map[ObjectID]info.ShareNote, len(shareNotes))
	for _, each := range shareNotes {
		shareNotesMap[each.ToUserId] = each
	}

	toUserIds := make([]ObjectID, len(shareNotes))
	for i, eachShareNote := range shareNotes {
		toUserIds[i] = eachShareNote.ToUserId
	}

	note := noteService.GetNote(noteId, userId)
	if note.NoteId.IsZero() {
		return nil
	}

	// 查看其notebook的shareNotebooks信息
	shareNotebooks := []info.ShareNotebook{}
	db.ListByQ(db.ShareNotebooks,
		bson.M{"NotebookId": note.NotebookId, "UserId": db.MustObjectIDFromHex(userId), "ToUserId": bson.M{"$in": toUserIds}},
		&shareNotebooks)
	shareNotebooksMap := make(map[ObjectID]info.ShareNotebook, len(shareNotebooks))
	for _, each := range shareNotebooks {
		shareNotebooksMap[each.ToUserId] = each
	}

	// 得到用户信息
	userInfos := userService.ListUserInfosByUserIds(toUserIds)

	if len(userInfos) == 0 {
		return nil
	}

	shareUserInfos := make([]info.ShareUserInfo, len(userInfos))

	for i, userInfo := range userInfos {
		_, hasNotebook := shareNotebooksMap[userInfo.UserId]
		shareUserInfos[i] = info.ShareUserInfo{ToUserId: userInfo.UserId,
			Email:             userInfo.Email,
			Perm:              shareNotesMap[userInfo.UserId].Perm,
			NotebookHasShared: hasNotebook,
		}
	}

	return shareUserInfos
}

// 得到notebook的share信息
// TODO 这里必须要分页, 最多取100个用户; 限制
func (this *ShareService) ListNotebookShareUserInfo(notebookId, userId string) []info.ShareUserInfo {
	if !validShareIDs(notebookId, userId) || db.ShareNotebooks == nil {
		return nil
	}
	// notebook的shareNotebooks信息
	shareNotebooks := []info.ShareNotebook{}

	db.ListByQLimit(db.ShareNotebooks,
		bson.M{
			"NotebookId": db.MustObjectIDFromHex(notebookId),
			"UserId":     db.MustObjectIDFromHex(userId),
			"ToGroupId":  bson.M{"$exists": false},
		},
		&shareNotebooks, 100)

	if len(shareNotebooks) == 0 {
		return nil
	}

	// 得到用户信息
	toUserIds := make([]ObjectID, len(shareNotebooks))
	for i, each := range shareNotebooks {
		toUserIds[i] = each.ToUserId
	}
	userInfos := userService.ListUserInfosByUserIds(toUserIds)

	if len(userInfos) == 0 {
		return nil
	}

	shareNotebooksMap := make(map[ObjectID]info.ShareNotebook, len(shareNotebooks))
	for _, each := range shareNotebooks {
		shareNotebooksMap[each.ToUserId] = each
	}

	shareUserInfos := make([]info.ShareUserInfo, len(userInfos))
	for i, userInfo := range userInfos {
		shareUserInfos[i] = info.ShareUserInfo{ToUserId: userInfo.UserId,
			Email: userInfo.Email,
			Perm:  shareNotebooksMap[userInfo.UserId].Perm,
		}
	}

	return shareUserInfos
}

// ----------------
// 改变note share权限
func (this *ShareService) UpdateShareNotePerm(noteId string, perm int, userId, toUserId string) bool {
	return this.UpdateShareNotePermWithOptions(noteId, perm, userId, toUserId, ShareGrantOptions{Now: time.Now().UTC()}) == nil
}

func (this *ShareService) UpdateShareNotebookPerm(notebookId string, perm int, userId, toUserId string) bool {
	return this.UpdateShareNotebookPermWithOptions(notebookId, perm, userId, toUserId, ShareGrantOptions{Now: time.Now().UTC()}) == nil
}

// ---------------
// 删除share note
func (this *ShareService) DeleteShareNote(noteId string, userId, toUserId string) bool {
	if !validShareIDs(noteId, userId, toUserId) || db.ShareNotes == nil {
		return false
	}
	return db.DeleteAll(db.ShareNotes,
		bson.M{"NoteId": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(userId), "ToUserId": db.MustObjectIDFromHex(toUserId)})
}

// 删除笔记时要删除该noteId的所有...
func (this *ShareService) DeleteShareNoteAll(noteId string, userId string) bool {
	if !validShareIDs(noteId, userId) || db.ShareNotes == nil {
		return false
	}
	return this.deleteShareNoteAll(context.Background(), db.MustObjectIDFromHex(noteId), db.MustObjectIDFromHex(userId)) == nil
}

func (this *ShareService) deleteShareNoteAll(ctx context.Context, noteID, ownerID ObjectID) error {
	_, err := db.ShareNotes.RemoveAllContext(ctx, bson.M{"NoteId": noteID, "UserId": ownerID})
	return err
}

func (this *ShareService) verifyShareNoteAllDeleted(ctx context.Context, noteID, ownerID ObjectID) (bool, error) {
	var share info.ShareNote
	err := db.ShareNotes.FindContext(ctx, bson.M{"NoteId": noteID, "UserId": ownerID}).One(&share)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	return false, err
}

// 删除share notebook
func (this *ShareService) DeleteShareNotebook(notebookId string, userId, toUserId string) bool {
	if !validShareIDs(notebookId, userId, toUserId) || db.ShareNotebooks == nil {
		return false
	}
	return db.DeleteAll(db.ShareNotebooks,
		bson.M{"NotebookId": db.MustObjectIDFromHex(notebookId), "UserId": db.MustObjectIDFromHex(userId), "ToUserId": db.MustObjectIDFromHex(toUserId)})
}

// 删除userId分享给toUserId的所有
type shareDeletionStep struct {
	name   string
	remove func(context.Context, interface{}) (int, error)
}

func deleteShareRecords(ctx context.Context, query interface{}, steps []shareDeletionStep) error {
	var cleanupErr error
	for _, step := range steps {
		if _, err := step.remove(ctx, query); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete %s: %w", step.name, err))
		}
	}
	return cleanupErr
}

func (this *ShareService) DeleteUserShareNoteAndNotebook(userId, toUserId string) bool {
	if !validShareIDs(userId, toUserId) || db.ShareNotebooks == nil || db.ShareNotes == nil || db.HasShareNotes == nil {
		return false
	}
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId), "ToUserId": db.MustObjectIDFromHex(toUserId)}
	steps := []shareDeletionStep{
		{name: "share_notebooks", remove: db.ShareNotebooks.RemoveAllContext},
		{name: "share_notes", remove: db.ShareNotes.RemoveAllContext},
		{name: "has_share_notes", remove: db.HasShareNotes.RemoveAllContext},
	}
	return deleteShareRecords(context.Background(), query, steps) == nil
}

// 用户userId是否有修改noteId的权限
func (this *ShareService) HasUpdateNotePerm(noteId, userId string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) || noteService == nil {
		return false
	}
	note := noteService.GetNoteById(noteId)
	if note.NoteId.IsZero() {
		return false
	}
	if note.UserId.Hex() == userId {
		return true
	}
	return this.HasUpdatePerm(note.UserId.Hex(), userId, noteId)
}

// 用户userId是否有查看noteId的权限
func (this *ShareService) HasReadNotePerm(noteId, userId string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) || noteService == nil {
		return false
	}
	note := noteService.GetNoteById(noteId)
	if note.NoteId.IsZero() {
		return false
	}
	if note.UserId.Hex() == userId {
		return true
	}
	return this.HasReadPerm(note.UserId.Hex(), userId, noteId)
}

//----------------
// 用户分组

// 得到笔记分享给的groups
func (this *ShareService) GetNoteShareGroups(noteId, userId string) []info.ShareNote {
	if !validShareIDs(noteId, userId) || db.ShareNotes == nil || groupService == nil {
		return nil
	}
	groups := groupService.GetGroupsContainOf(userId)

	// 得到有分享的分组
	shares := []info.ShareNote{}
	db.ListByQ(db.ShareNotes,
		bson.M{"NoteId": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(userId), "ToGroupId": bson.M{"$exists": true}}, &shares)
	mapShares := map[ObjectID]info.ShareNote{}
	for _, share := range shares {
		mapShares[share.ToGroupId] = share
	}

	// 所有的groups都有share, 但没有share的group没有shareId
	shares2 := make([]info.ShareNote, len(groups))
	for i, group := range groups {
		share, ok := mapShares[group.GroupId]
		if !ok {
			share = info.ShareNote{}
		}
		share.ToGroup = group
		shares2[i] = share
	}

	return shares2
}

// 共享笔记给分组
func (this *ShareService) AddShareNoteGroup(userId, noteId, groupId string, perm int) bool {
	return this.AddShareNoteGroupWithOptions(userId, noteId, groupId, perm, ShareGrantOptions{Now: time.Now().UTC()}) == nil
}

// 删除
func (this *ShareService) DeleteShareNoteGroup(userId, noteId, groupId string) bool {
	if !validShareIDs(userId, noteId, groupId) || db.ShareNotes == nil {
		return false
	}
	return db.Delete(db.ShareNotes, bson.M{"NoteId": db.MustObjectIDFromHex(noteId),
		"UserId":    db.MustObjectIDFromHex(userId),
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}

//-------

// 得到笔记本分享给的groups
func (this *ShareService) GetNotebookShareGroups(notebookId, userId string) []info.ShareNotebook {
	if !validShareIDs(notebookId, userId) || db.ShareNotebooks == nil || groupService == nil {
		return nil
	}
	groups := groupService.GetGroupsContainOf(userId)

	// 得到有分享的分组
	shares := []info.ShareNotebook{}
	db.ListByQ(db.ShareNotebooks,
		bson.M{"NotebookId": db.MustObjectIDFromHex(notebookId), "UserId": db.MustObjectIDFromHex(userId), "ToGroupId": bson.M{"$exists": true}}, &shares)
	mapShares := map[ObjectID]info.ShareNotebook{}
	for _, share := range shares {
		mapShares[share.ToGroupId] = share
	}
	LogJ(shares)

	// 所有的groups都有share, 但没有share的group没有shareId
	shares2 := make([]info.ShareNotebook, len(groups))
	for i, group := range groups {
		share, ok := mapShares[group.GroupId]
		if !ok {
			share = info.ShareNotebook{}
		}
		share.ToGroup = group
		shares2[i] = share
	}

	return shares2
}

// 共享笔记给分组
func (this *ShareService) AddShareNotebookGroup(userId, notebookId, groupId string, perm int) bool {
	return this.AddShareNotebookGroupWithOptions(userId, notebookId, groupId, perm, ShareGrantOptions{Now: time.Now().UTC()}) == nil
}

// 删除
func (this *ShareService) DeleteShareNotebookGroup(userId, notebookId, groupId string) bool {
	if !validShareIDs(userId, notebookId, groupId) || db.ShareNotebooks == nil {
		return false
	}
	return db.Delete(db.ShareNotebooks, bson.M{"NotebookId": db.MustObjectIDFromHex(notebookId),
		"UserId":    db.MustObjectIDFromHex(userId),
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}

//--------------------
// 删除组时, 删除所有的
//--------------------

func (this *ShareService) DeleteAllShareNotebookGroup(groupId string) bool {
	if !validShareIDs(groupId) || db.ShareNotebooks == nil {
		return false
	}
	return db.Delete(db.ShareNotebooks, bson.M{
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}
func (this *ShareService) DeleteAllShareNoteGroup(groupId string) bool {
	if !validShareIDs(groupId) || db.ShareNotes == nil {
		return false
	}
	return db.Delete(db.ShareNotes, bson.M{
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}

//--------------------
// 删除组内用户时, 删除其分享的
//--------------------

func (this *ShareService) DeleteShareNotebookGroupWhenDeleteGroupUser(userId, groupId string) bool {
	if !validShareIDs(userId, groupId) || db.ShareNotebooks == nil {
		return false
	}
	return db.Delete(db.ShareNotebooks, bson.M{
		"UserId":    db.MustObjectIDFromHex(userId),
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}

func (this *ShareService) DeleteShareNoteGroupWhenDeleteGroupUser(userId, groupId string) bool {
	if !validShareIDs(userId, groupId) || db.ShareNotes == nil {
		return false
	}
	return db.Delete(db.ShareNotes, bson.M{
		"UserId":    db.MustObjectIDFromHex(userId),
		"ToGroupId": db.MustObjectIDFromHex(groupId),
	})
}
