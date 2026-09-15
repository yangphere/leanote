package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"regexp"
	"slices"
	"strings"
	"time"
)

type NoteService struct {
}

func (this *NoteService) CanCreateNote(actorUserId, ownerUserId, notebookId string) bool {
	if !db.IsValidObjectIDHex(actorUserId) || !db.IsValidObjectIDHex(ownerUserId) || !db.IsValidObjectIDHex(notebookId) {
		return false
	}
	if actorUserId == ownerUserId {
		return notebookService.IsMyNotebook(notebookId, ownerUserId)
	}
	return shareService.HasUpdateNotebookPerm(ownerUserId, actorUserId, notebookId)
}

const (
	noteInsertFailed        = "noteInsertFailed"
	noteContentInsertFailed = "noteContentInsertFailed"
	noteSaveFailed          = "saveFailed"
	noteContentSaveFailed   = "contentSaveFailed"
)

// 通过id, userId得到note
func (this *NoteService) GetNote(noteId, userId string) (note info.Note) {
	note = info.Note{}
	db.GetByIdAndUserId(db.Notes, noteId, userId, &note)
	return
}

// fileService调用
// 不能是已经删除了的, life bug, 客户端删除后, 竟然还能在web上打开
func (this *NoteService) GetNoteById(noteId string) (note info.Note) {
	note = info.Note{}
	if noteId == "" {
		return
	}
	db.GetByQ(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId), "IsDeleted": false}, &note)
	return
}
func (this *NoteService) GetNoteByIdAndUserId(noteId, userId string) (note info.Note) {
	note = info.Note{}
	if noteId == "" || userId == "" {
		return
	}
	db.GetByQ(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(userId), "IsDeleted": false}, &note)
	return
}

// 得到blog, blogService用
// 不要传userId, 因为是公开的
func (this *NoteService) GetBlogNote(noteId string) (note info.Note) {
	note = info.Note{}
	db.GetByQ(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId),
		"IsBlog": true, "IsTrash": false, "IsDeleted": false}, &note)
	return
}

// 通过id, userId得到noteContent
func (this *NoteService) GetNoteContent(noteContentId, userId string) (noteContent info.NoteContent) {
	noteContent = info.NoteContent{}
	db.GetByIdAndUserId(db.NoteContents, noteContentId, userId, &noteContent)
	return
}

// 得到笔记和内容
func (this *NoteService) GetNoteAndContent(noteId, userId string) (noteAndContent info.NoteAndContent) {
	note := this.GetNote(noteId, userId)
	noteContent := this.GetNoteContent(noteId, userId)
	return info.NoteAndContent{Note: note, NoteContent: noteContent}
}

func (this *NoteService) GetNoteBySrc(src, userId string) (note info.Note) {
	note = info.Note{}
	if src == "" {
		return
	}

	notes := []info.Note{}
	q := db.Notes.Find(bson.M{
		"UserId": db.MustObjectIDFromHex(userId),
		"Src":    src,
	})
	q.Sort("-Usn").Limit(1).All(&notes)
	if len(notes) > 0 {
		return notes[0]
	}
	// db.GetByQ(db.Notes, bson.M{"Src": src, "UserId": db.MustObjectIDFromHex(userId), "IsDeleted": false}, &note)
	return
}

func (this *NoteService) GetNoteAndContentBySrc(src, userId string) (noteId string, noteAndContent info.NoteAndContentSep) {
	note := this.GetNoteBySrc(src, userId)
	if !note.NoteId.IsZero() {
		noteId = note.NoteId.Hex()
		noteContent := this.GetNoteContent(note.NoteId.Hex(), userId)
		return noteId, info.NoteAndContentSep{NoteInfo: note, NoteContentInfo: noteContent}
	}
	return
}

// 获取同步的笔记
// > afterUsn的笔记
func (this *NoteService) GetSyncNotes(userId string, afterUsn, maxEntry int) []info.ApiNote {
	notes := []info.Note{}
	q := db.Notes.Find(bson.M{
		"UserId": db.MustObjectIDFromHex(userId),
		"Usn":    bson.M{"$gt": afterUsn},
	})
	q.Sort("Usn").Limit(maxEntry).All(&notes)

	return this.ToApiNotes(notes)
}

// note与apiNote的转换
func (this *NoteService) ToApiNotes(notes []info.Note) []info.ApiNote {
	// 2, 得到所有图片, 附件信息
	// 查images表, attachs表
	if len(notes) > 0 {
		noteIds := make([]ObjectID, len(notes))
		for i, note := range notes {
			noteIds[i] = note.NoteId
		}
		noteFilesMap := this.getFiles(noteIds)
		// 生成info.ApiNote
		apiNotes := make([]info.ApiNote, len(notes))
		for i, note := range notes {
			noteId := note.NoteId.Hex()
			apiNotes[i] = this.ToApiNote(&note, noteFilesMap[noteId])
		}
		return apiNotes
	}
	// 返回空的
	return []info.ApiNote{}
}

// note与apiNote的转换
func (this *NoteService) ToApiNote(note *info.Note, files []info.NoteFile) info.ApiNote {
	apiNote := info.ApiNote{
		NoteId:      note.NoteId.Hex(),
		NotebookId:  note.NotebookId.Hex(),
		UserId:      note.UserId.Hex(),
		Title:       note.Title,
		Tags:        note.Tags,
		IsMarkdown:  note.IsMarkdown,
		IsBlog:      note.IsBlog,
		IsTrash:     note.IsTrash,
		IsDeleted:   note.IsDeleted,
		Usn:         note.Usn,
		CreatedTime: note.CreatedTime,
		UpdatedTime: note.UpdatedTime,
		PublicTime:  note.PublicTime,
		Files:       files,
	}
	return apiNote
}

// getDirtyNotes, 把note的图片, 附件信息都发送给客户端
// 客户端保存到本地, 再获取图片, 附件

// 得到所有图片, 附件信息
// 查images表, attachs表
// [待测]
func (this *NoteService) getFiles(noteIds []ObjectID) map[string][]info.NoteFile {
	noteImages := noteImageService.getImagesByNoteIds(noteIds)
	noteAttachs := attachService.getAttachsByNoteIds(noteIds)

	noteFilesMap := map[string][]info.NoteFile{}

	for _, noteId := range noteIds {
		noteIdHex := noteId.Hex()
		noteFiles := []info.NoteFile{}
		// images
		if images, ok := noteImages[noteIdHex]; ok {
			for _, image := range images {
				noteFiles = append(noteFiles, info.NoteFile{
					FileId: image.FileId.Hex(),
					Type:   image.Type,
				})
			}
		}

		// attach
		if attachs, ok := noteAttachs[noteIdHex]; ok {
			for _, attach := range attachs {
				noteFiles = append(noteFiles, info.NoteFile{
					FileId:   attach.AttachId.Hex(),
					Type:     attach.Type,
					Title:    attach.Title,
					IsAttach: true,
				})
			}
		}

		noteFilesMap[noteIdHex] = noteFiles
	}

	return noteFilesMap
}

// 列出note, 排序规则, 还有分页
// CreatedTime, UpdatedTime, title 来排序
func (this *NoteService) ListNotes(userId, notebookId string,
	isTrash bool, pageNumber, pageSize int, sortField string, isAsc bool, isBlog bool) (count int, notes []info.Note) {
	notes = []info.Note{}
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)

	// 不是trash的
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsTrash": isTrash, "IsDeleted": false}
	if isBlog {
		query["IsBlog"] = true
	}
	if notebookId != "" {
		query["NotebookId"] = db.MustObjectIDFromHex(notebookId)
	}

	q := db.Notes.Find(query)

	// 总记录数
	count, _ = q.Count()

	q.Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes)
	return
}

// 通过noteIds来查询
// ShareService调用
func (this *NoteService) ListNotesByNoteIdsWithPageSort(noteIds []ObjectID, userId string,
	pageNumber, pageSize int, sortField string, isAsc bool, isBlog bool) (notes []info.Note) {
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)
	notes = []info.Note{}

	// 不是trash
	db.Notes.
		Find(bson.M{"_id": bson.M{"$in": noteIds}, "IsTrash": false}).
		Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes)
	return
}

// shareService调用
func (this *NoteService) ListNotesByNoteIds(noteIds []ObjectID) (notes []info.Note) {
	notes = []info.Note{}

	db.Notes.
		Find(bson.M{"_id": bson.M{"$in": noteIds}}).
		All(&notes)
	return
}

// blog需要
func (this *NoteService) ListNoteContentsByNoteIds(noteIds []ObjectID) (notes []info.NoteContent) {
	notes = []info.NoteContent{}

	db.NoteContents.
		Find(bson.M{"_id": bson.M{"$in": noteIds}}).
		All(&notes)
	return
}

// 只得到abstract, 不需要content
func (this *NoteService) ListNoteAbstractsByNoteIds(noteIds []ObjectID) (notes []info.NoteContent) {
	notes = []info.NoteContent{}
	db.ListByQWithFields(db.NoteContents, bson.M{"_id": bson.M{"$in": noteIds}}, []string{"_id", "Abstract"}, &notes)
	return
}
func (this *NoteService) ListNoteContentByNoteIds(noteIds []ObjectID) (notes []info.NoteContent) {
	notes = []info.NoteContent{}
	db.ListByQWithFields(db.NoteContents, bson.M{"_id": bson.M{"$in": noteIds}}, []string{"_id", "Abstract", "Content"}, &notes)
	return
}

// 添加笔记
// 首先要判断Notebook是否是Blog, 是的话设为blog
// [ok]

func (this *NoteService) AddNote(note info.Note, fromApi bool) info.Note {
	note, _, _ = this.addNoteResult(note, fromApi, false)
	return note
}

func (this *NoteService) addNoteResult(note info.Note, fromApi bool, strict bool) (info.Note, bool, string) {
	if note.NoteId.Hex() == "" {
		noteId := db.NewObjectID()
		note.NoteId = noteId
	}

	// 关于创建时间, 可能是客户端发来, 此时判断时间是否有
	note.CreatedTime = FixUrlTime(note.CreatedTime)
	note.UpdatedTime = FixUrlTime(note.UpdatedTime)

	note.IsTrash = false
	note.UpdatedUserId = note.UserId
	note.UrlTitle = GetUrTitle(note.UserId.Hex(), note.Title, "note", note.NoteId.Hex())
	usn, err := userService.AllocateUsn(context.Background(), note.UserId.Hex())
	if err != nil {
		return note, false, noteInsertFailed
	}
	note.Usn = usn

	notebookId := note.NotebookId.Hex()

	// api会传IsBlog, web不会传
	if !fromApi {
		// 设为blog
		note.IsBlog = notebookService.IsBlog(notebookId)
	}
	//	if note.IsBlog {
	note.PublicTime = note.UpdatedTime
	//	}

	ok := db.Insert(db.Notes, note)
	if strict && !ok {
		return note, false, noteInsertFailed
	}

	// tag1
	tagService.AddTags(note.UserId.Hex(), note.Tags)

	// recount notebooks' notes number
	notebookService.ReCountNotebookNumberNotes(notebookId)

	if !ok {
		return note, false, noteInsertFailed
	}
	return note, true, ""
}

// 添加共享d笔记
func (this *NoteService) AddSharedNote(note info.Note, myUserId ObjectID) info.Note {
	// 判断我是否有权限添加
	if shareService.HasUpdateNotebookPerm(note.UserId.Hex(), myUserId.Hex(), note.NotebookId.Hex()) {
		note.CreatedUserId = myUserId // 是我给共享我的人创建的
		return this.AddNote(note, false)
	}
	return info.Note{}
}

// 添加笔记本内容
// [ok]
func (this *NoteService) AddNoteContent(noteContent info.NoteContent) info.NoteContent {
	noteContent, _, _ = this.addNoteContentResult(noteContent, false)
	return noteContent
}

func (this *NoteService) addNoteContentResult(noteContent info.NoteContent, strict bool) (info.NoteContent, bool, string) {

	noteContent.CreatedTime = FixUrlTime(noteContent.CreatedTime)
	noteContent.UpdatedTime = FixUrlTime(noteContent.UpdatedTime)

	noteContent.UpdatedUserId = noteContent.UserId
	ok := db.Insert(db.NoteContents, noteContent)
	if strict && !ok {
		return noteContent, false, noteContentInsertFailed
	}

	// 更新笔记图片
	noteImageService.UpdateNoteImages(noteContent.UserId.Hex(), noteContent.NoteId.Hex(), "", noteContent.Content)

	if !ok {
		return noteContent, false, noteContentInsertFailed
	}
	return noteContent, true, ""
}

// API, abstract, desc需要这里获取
// 不需要
/*
func (this *NoteService) AddNoteAndContentApi(note info.Note, noteContent info.NoteContent, myUserId ObjectID) info.Note {
	if(note.NoteId.Hex() == "") {
		noteId := db.NewObjectID();
		note.NoteId = noteId;
	}
	note.CreatedTime = time.Now()
	note.UpdatedTime = note.CreatedTime
	note.IsTrash = false
	note.UpdatedUserId = note.UserId
	note.UrlTitle = GetUrTitle(note.UserId.Hex(), note.Title, "note")
	note.Usn = userService.IncrUsn(note.UserId.Hex())

	// desc这里获取
	desc := SubStringHTMLToRaw(noteContent.Content, 50)
	note.Desc = desc;

	// 设为blog
	notebookId := note.NotebookId.Hex()
	note.IsBlog = notebookService.IsBlog(notebookId)

	if note.IsBlog {
		note.PublicTime = note.UpdatedTime
	}

	db.Insert(db.Notes, note)

	// tag1, 不需要了
//	tagService.AddTags(note.UserId.Hex(), note.Tags)

	// recount notebooks' notes number
	notebookService.ReCountNotebookNumberNotes(notebookId)

	// 这里, 添加到内容中
	abstract := SubStringHTML(noteContent.Content, 200, "")
	noteContent.Abstract = abstract
	this.AddNoteContent(noteContent)

	return note
}*/

// 添加笔记和内容
// 这里使用 info.NoteAndContent 接收?
func (this *NoteService) AddNoteAndContentForController(note info.Note, noteContent info.NoteContent, updatedUserId string) info.Note {
	note, _, _ = this.AddNoteAndContentForControllerResult(note, noteContent, updatedUserId)
	return note
}

// AddNoteAndContentForControllerResult is the controller-specific creation
// boundary. It preserves service failure reasons so the HTTP endpoint cannot
// mistake a zero-value note or partial write for success.
func (this *NoteService) AddNoteAndContentForControllerResult(note info.Note, noteContent info.NoteContent, updatedUserId string) (info.Note, bool, string) {
	return this.addNoteAndContentResult(note, noteContent, db.MustObjectIDFromHex(updatedUserId), false, true)
}
func (this *NoteService) AddNoteAndContent(note info.Note, noteContent info.NoteContent, myUserId ObjectID) info.Note {
	var ok bool
	note, ok, _ = this.addNoteAndContentResult(note, noteContent, myUserId, false, false)
	if !ok {
		return info.Note{}
	}
	return note
}

func (this *NoteService) addNoteAndContentResult(note info.Note, noteContent info.NoteContent, myUserId ObjectID, fromApi, strict bool) (info.Note, bool, string) {
	return this.addNoteAndContentResultWithIdentity(note, noteContent, myUserId, fromApi, strict, "", "", nil)
}

func (this *NoteService) addNoteAndContentResultWithIdentity(note info.Note, noteContent info.NoteContent, myUserId ObjectID, fromApi, strict bool, forcedOperationID, forcedInputDigest string, assets []applicationnotes.OperationAsset) (info.Note, bool, string) {
	if note.NoteId.IsZero() {
		note.NoteId = db.NewObjectID()
	}
	if note.UserId == myUserId && !notebookService.IsMyNotebook(note.NotebookId.Hex(), note.UserId.Hex()) {
		return info.Note{}, false, "notebookIdNotExists"
	}
	if note.UserId != myUserId {
		if !shareService.HasUpdateNotebookPerm(note.UserId.Hex(), myUserId.Hex(), note.NotebookId.Hex()) {
			return info.Note{}, false, "noAuth"
		}
		note.CreatedUserId = myUserId
	}
	note.CreatedTime = FixUrlTime(note.CreatedTime)
	note.UpdatedTime = FixUrlTime(note.UpdatedTime)
	note.IsTrash = false
	note.UpdatedUserId = myUserId
	note.UrlTitle = GetUrTitle(note.UserId.Hex(), note.Title, "note", note.NoteId.Hex())
	if !fromApi {
		note.IsBlog = notebookService.IsBlog(note.NotebookId.Hex())
		noteContent.IsBlog = note.IsBlog
	}
	note.PublicTime = note.UpdatedTime

	noteContent.NoteId = note.NoteId
	noteContent.UserId = note.UserId
	noteContent.CreatedTime = FixUrlTime(noteContent.CreatedTime)
	noteContent.UpdatedTime = FixUrlTime(noteContent.UpdatedTime)
	noteContent.UpdatedUserId = myUserId
	identityNote := note
	identityNote.CreatedTime = time.Time{}
	identityNote.UpdatedTime = time.Time{}
	identityNote.PublicTime = time.Time{}
	identityContent := noteContent
	identityContent.CreatedTime = time.Time{}
	identityContent.UpdatedTime = time.Time{}
	operationID, inputDigest, _, identityErr := applicationnotes.NewOperationIdentity(
		"note_create", note.UserId, note.NoteId, struct {
			Note    info.Note
			Content info.NoteContent
		}{Note: identityNote, Content: identityContent},
	)
	if forcedOperationID != "" || forcedInputDigest != "" {
		if forcedOperationID == "" || forcedInputDigest == "" {
			return info.Note{}, false, noteSaveFailed
		}
		operationID = forcedOperationID
		inputDigest = forcedInputDigest
	}
	desiredState, desiredErr := applicationnotes.CanonicalState(struct {
		Note    info.Note
		Content info.NoteContent
	}{Note: note, Content: noteContent})
	if identityErr != nil || desiredErr != nil {
		return info.Note{}, false, noteSaveFailed
	}
	receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), note.UserId, operationID)
	hasReceipt := receiptErr == nil
	if receiptErr != nil && !errors.Is(receiptErr, mongo.ErrNoDocuments) && !errors.Is(receiptErr, db.ErrMongoClientNotInitialized) {
		return info.Note{}, false, noteSaveFailed
	}
	if hasReceipt {
		if receipt.InputDigest != inputDigest {
			return info.Note{}, false, string(WorkspaceConflict)
		}
		if receipt.Status == applicationnotes.OperationCommitted {
			existing := this.GetNote(note.NoteId.Hex(), note.UserId.Hex())
			if existing.NoteId.IsZero() {
				return info.Note{}, false, string(WorkspacePartialWrite)
			}
			existingContent := this.GetNoteContent(note.NoteId.Hex(), note.UserId.Hex())
			if !this.repairNoteCreationProjections(operationID, inputDigest, desiredState, existing, existingContent) {
				return existing, false, string(WorkspaceSideEffect)
			}
			return existing, true, ""
		}
	}
	if existing := this.GetNote(note.NoteId.Hex(), note.UserId.Hex()); !existing.NoteId.IsZero() && !hasReceipt {
		existingContent := this.GetNoteContent(note.NoteId.Hex(), note.UserId.Hex())
		if !existing.IsDeleted && sameNoteCreation(existing, existingContent, note, noteContent) {
			if this.repairNoteCreationProjections(operationID, inputDigest, desiredState, existing, existingContent) {
				return existing, true, ""
			}
			return existing, false, string(WorkspaceSideEffect)
		}
		return info.Note{}, false, noteInsertFailed
	}
	plan := db.WorkspaceMutationPlan{
		OperationID:        operationID,
		OwnerID:            note.UserId,
		ResourceID:         note.NoteId,
		Kind:               "note_create",
		InputDigest:        inputDigest,
		Assets:             append([]applicationnotes.OperationAsset(nil), assets...),
		DesiredState:       desiredState,
		AssignedUSN:        func() int { return note.Usn },
		RestoreAssignedUSN: func(usn int) { note.Usn = usn },
		RestoreDesiredState: func(payload []byte) error {
			state := struct {
				Note    info.Note
				Content info.NoteContent
			}{}
			if err := json.Unmarshal(payload, &state); err != nil {
				return err
			}
			note = state.Note
			noteContent = state.Content
			return nil
		},
		Steps: []db.WorkspaceMutationStep{
			{
				Name: "allocate_usn",
				Apply: func(ctx context.Context) error {
					usn, err := db.AllocateUserUSN(ctx, note.UserId)
					if err != nil {
						return err
					}
					note.Usn = usn
					return nil
				},
				ReplaySafe: true,
			},
			{
				Name: "note",
				Apply: func(ctx context.Context) error {
					return db.Notes.InsertContext(ctx, note)
				},
				Verify: func(ctx context.Context) (bool, error) {
					var existing info.Note
					err := db.Notes.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn}).One(&existing)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil && sameNoteMetadataCreation(existing, note), err
				},
				Compensate: func(ctx context.Context) error {
					return db.Notes.RemoveContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn})
				},
			},
			{
				Name: "content",
				Apply: func(ctx context.Context) error {
					return db.NoteContents.InsertContext(ctx, noteContent)
				},
				Verify: func(ctx context.Context) (bool, error) {
					var existing info.NoteContent
					err := db.NoteContents.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId}).One(&existing)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil && sameNoteContentCreation(existing, noteContent), err
				},
				Compensate: func(ctx context.Context) error {
					return db.NoteContents.RemoveContext(ctx, bson.M{
						"_id": note.NoteId, "UserId": note.UserId,
						"Content": noteContent.Content, "Abstract": noteContent.Abstract,
						"IsBlog": noteContent.IsBlog, "UpdatedTime": noteContent.UpdatedTime,
						"UpdatedUserId": noteContent.UpdatedUserId,
					})
				},
			},
		},
	}
	result, err := db.RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !result.Committed {
		if result.PartialWrite || errors.Is(err, db.ErrPartialWrite) {
			return info.Note{}, false, string(WorkspacePartialWrite)
		}
		if errors.Is(err, db.ErrDuplicateIdentity) {
			return info.Note{}, false, string(WorkspaceDuplicate)
		}
		return info.Note{}, false, noteSaveFailed
	}
	// A committed retry may enter RunWorkspaceMutation through a terminal
	// receipt.  Terminal receipts intentionally redact the desired payload, so
	// the request-local note can still have a zero USN.  Return the durable
	// document to keep the API idempotency response identical to the original
	// successful response.
	note = preferPersistedCreatedNote(note, this.GetNote(note.NoteId.Hex(), note.UserId.Hex()))

	// These are repairable projections outside the required note/content/USN
	// commit. A failure remains observable and is never reported as a clean
	// creation success.
	if !this.repairNoteCreationProjections(operationID, inputDigest, desiredState, note, noteContent) {
		return note, false, string(WorkspaceSideEffect)
	}
	return note, true, ""
}

func (this *NoteService) repairNoteCreationProjections(operationID, inputDigest string, desiredState []byte, note info.Note, noteContent info.NoteContent) bool {
	if receipt, err := db.GetWorkspaceOperation(context.Background(), note.UserId, operationID+":projections"); err == nil {
		if receipt.InputDigest != inputDigest {
			return false
		}
		if receipt.Status == applicationnotes.OperationCommitted {
			return true
		}
		if applicationnotes.IsTerminalOperation(receipt.Status) {
			return false
		}
		// A projection failure intentionally leaves a resumable receipt. Re-enter
		// the repair runner so it can verify the interrupted step and continue.
	} else if !errors.Is(err, mongo.ErrNoDocuments) && !errors.Is(err, db.ErrMongoClientNotInitialized) {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID + ":projections", OwnerID: note.UserId, ResourceID: note.NoteId,
		Kind: "note_create_projections", InputDigest: inputDigest, DesiredState: desiredState,
		FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{
			{Name: "tags", ReplaySafe: true,
				Apply:  func(ctx context.Context) error { return tagService.addTags(ctx, note.UserId, note.Tags) },
				Verify: func(ctx context.Context) (bool, error) { return tagService.verifyTags(ctx, note.UserId, note.Tags) },
			},
			{Name: "notebook_count", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return notebookService.reCountNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return notebookService.verifyNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
			},
			{Name: "image_index", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return noteImageService.updateNoteImages(ctx, note.UserId, note.NoteId, "", noteContent.Content)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return noteImageService.verifyNoteImages(ctx, note.UserId, note.NoteId, "", noteContent.Content)
				},
			},
		},
	}
	result, err := db.RunWorkspaceRepair(context.Background(), plan)
	return err == nil && result.Committed
}

func sameNoteCreation(existing info.Note, existingContent info.NoteContent, desired info.Note, desiredContent info.NoteContent) bool {
	return sameNoteMetadataCreation(existing, desired) && sameNoteContentCreation(existingContent, desiredContent)
}

func preferPersistedCreatedNote(request info.Note, persisted info.Note) info.Note {
	if !persisted.NoteId.IsZero() {
		return persisted
	}
	return request
}

func sameNoteMetadataCreation(existing info.Note, desired info.Note) bool {
	return existing.UserId == desired.UserId &&
		existing.CreatedUserId == desired.CreatedUserId &&
		existing.NotebookId == desired.NotebookId &&
		existing.Title == desired.Title &&
		existing.Desc == desired.Desc &&
		existing.Src == desired.Src &&
		existing.ImgSrc == desired.ImgSrc &&
		slices.Equal(existing.Tags, desired.Tags) &&
		existing.IsBlog == desired.IsBlog &&
		existing.IsMarkdown == desired.IsMarkdown &&
		existing.AttachNum == desired.AttachNum
}

func sameNoteContentCreation(existingContent info.NoteContent, desiredContent info.NoteContent) bool {
	return existingContent.NoteId == desiredContent.NoteId &&
		existingContent.UserId == desiredContent.UserId &&
		existingContent.IsBlog == desiredContent.IsBlog &&
		existingContent.Content == desiredContent.Content &&
		existingContent.Abstract == desiredContent.Abstract
}

func (this *NoteService) AddNoteAndContentApi(note info.Note, noteContent info.NoteContent, myUserId ObjectID) info.Note {
	var ok bool
	note, ok, _ = this.addNoteAndContentResult(note, noteContent, myUserId, true, false)
	if !ok {
		return info.Note{}
	}
	return note
}

func (this *NoteService) AddNoteAndContentApiResult(note info.Note, noteContent info.NoteContent, myUserId ObjectID) (info.Note, bool, string) {
	return this.addNoteAndContentResult(note, noteContent, myUserId, true, true)
}

// AddNoteAndContentApiResultWithIdentity lets the API adapter pre-create a
// durable operation before uploading assets. The identity is optional for
// legacy callers; when supplied, it must be reused for the whole create flow.
func (this *NoteService) AddNoteAndContentApiResultWithIdentity(note info.Note, noteContent info.NoteContent, myUserId ObjectID, operationID, inputDigest string, assets []applicationnotes.OperationAsset) (info.Note, bool, string) {
	return this.addNoteAndContentResultWithIdentity(note, noteContent, myUserId, true, true, operationID, inputDigest, assets)
}

// 当设置/取消了笔记为博客
func (this *NoteService) UpdateNoteContentIsBlog(noteId, userId string, isBlog bool) {
	db.UpdateByIdAndUserIdMap(db.NoteContents, noteId, userId, bson.M{"IsBlog": isBlog})
}

// 附件修改, 增加noteIncr
func (this *NoteService) IncrNoteUsn(noteId, userId string) int {
	note := this.GetNote(noteId, userId)
	if note.NoteId.IsZero() || note.IsDeleted {
		return 0
	}
	afterUsn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return 0
	}
	filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn, "IsDeleted": false}
	db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
	if err := db.Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": bson.M{"UpdatedTime": time.Now(), "Usn": afterUsn}}); err != nil {
		return 0
	}
	return afterUsn
}

// 这里要判断权限, 如果userId != updatedUserId, 那么需要判断权限
// [ok] TODO perm还没测 [del]
func (this *NoteService) UpdateNoteTitle(userId, updatedUserId, noteId, title string) bool {
	// updatedUserId 要修改userId的note, 此时需要判断是否有修改权限
	if userId != updatedUserId {
		if !shareService.HasUpdatePerm(userId, updatedUserId, noteId) {
			println("NO AUTH")
			return false
		}
	}
	note := this.GetNote(noteId, userId)
	if note.NoteId.IsZero() || note.IsDeleted {
		return false
	}

	usn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false
	}
	filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn, "IsDeleted": false}
	db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
	return db.Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": bson.M{"UpdatedUserId": db.MustObjectIDFromHex(updatedUserId), "Title": title, "UpdatedTime": time.Now(), "Usn": usn}}) == nil
}

// 修改笔记本内容
// [ok] TODO perm未测
// hasBeforeUpdateNote 之前是否更新过note其它信息, 如果有更新, usn不用更新
// TODO abstract这里生成
func (this *NoteService) UpdateNoteContent(updatedUserId, noteId, content, abstract string,
	hasBeforeUpdateNote bool,
	usn int, updatedTime time.Time) (bool, string, int) {
	// 是否已自定义
	note := this.GetNoteById(noteId)
	if note.NoteId.IsZero() {
		return false, "notExists", 0
	}
	userId := note.UserId.Hex()
	// updatedUserId 要修改userId的note, 此时需要判断是否有修改权限
	if userId != updatedUserId {
		if !shareService.HasUpdatePerm(userId, updatedUserId, noteId) {
			Log("NO AUTH")
			return false, "noAuth", 0
		}
	}
	noteContent := this.GetNoteContent(noteId, userId)
	if noteContent.NoteId.IsZero() {
		return false, "notExists", 0
	}
	if noteContent.Content == content && noteContent.Abstract == abstract {
		return true, "", 0
	}

	updatedTime = FixUrlTime(updatedTime)

	// abstract重置
	data := bson.M{"UpdatedUserId": db.MustObjectIDFromHex(updatedUserId),
		"Content":     content,
		"Abstract":    abstract,
		"UpdatedTime": updatedTime}

	if note.IsBlog && note.HasSelfDefined {
		delete(data, "Abstract")
	}

	// usn, 修改笔记不可能单独修改内容
	afterUsn := 0
	// 如果之前没有修改note其它信息, 那么usn++
	if !hasBeforeUpdateNote {
		// 需要验证
		if usn >= 0 && note.Usn != usn {
			return false, "conflict", 0
		}
		var err error
		afterUsn, err = userService.AllocateUsn(context.Background(), userId)
		if err != nil {
			return false, "storage", 0
		}
		filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn, "IsDeleted": false}
		db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
		if err := db.Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": bson.M{"Usn": afterUsn}}); err != nil {
			return false, noteSaveFailed, 0
		}
	}

	if db.UpdateByIdAndUserIdMap(db.NoteContents, noteId, userId, data) {
		// 这里, 添加历史记录
		noteContentHistoryService.AddHistory(noteId, userId, info.EachHistory{UpdatedUserId: db.MustObjectIDFromHex(updatedUserId),
			Content:     noteContent.Content,
			UpdatedTime: time.Now(),
		})

		// 更新笔记图片
		noteImageService.UpdateNoteImages(userId, noteId, note.ImgSrc, content)

		return true, "", afterUsn
	}
	return false, noteContentSaveFailed, 0
}

// ?????
// 这种方式太恶心, 改动很大
// 通过content修改笔记的imageIds列表
// src="http://localhost:9000/file/outputImage?fileId=541ae75499c37b6b79000005&noteId=541ae63c19807a4bb9000000"
func (this *NoteService) updateNoteImages(noteId string, content string) bool {
	return true
}

// 更新tags
// [ok] [del]
func (this *NoteService) UpdateTags(noteId string, userId string, tags []string) bool {
	note := this.GetNote(noteId, userId)
	if note.NoteId.IsZero() || note.IsDeleted {
		return false
	}
	usn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false
	}
	filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn, "IsDeleted": false}
	db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
	return db.Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": bson.M{"Tags": tags, "Usn": usn}}) == nil
}

func (this *NoteService) ToBlog(userId, noteId string, isBlog, isTop bool) bool {
	note := this.GetNote(noteId, userId)
	if note.NoteId.IsZero() || note.IsDeleted {
		return false
	}
	noteUpdate := bson.M{}
	if isTop {
		isBlog = true
	}
	if !isBlog {
		isTop = false
	}
	noteUpdate["IsBlog"] = isBlog
	noteUpdate["IsTop"] = isTop
	if isBlog {
		noteUpdate["PublicTime"] = time.Now()
	} else {
		noteUpdate["HasSelfDefined"] = false
	}
	usn, err := userService.AllocateUsn(context.Background(), userId)
	if err != nil {
		return false
	}
	noteUpdate["Usn"] = usn

	filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": note.Usn, "IsDeleted": false}
	db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
	ok := db.Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": noteUpdate}) == nil
	// 重新计算tags
	go (func() {
		this.UpdateNoteContentIsBlog(noteId, userId, isBlog)

		blogService.ReCountBlogTags(userId)
	})()
	return ok
}

// 移动note
// trash, 正常的都可以用
// 1. 要检查下notebookId是否是自己的
// 2. 要判断之前是否是blog, 如果不是, 那么notebook是否是blog?
func (this *NoteService) MoveNote(noteId, notebookId, userId string) info.Note {
	return this.MoveNoteWithOperation(noteId, notebookId, userId, "")
}

func restoreMoveRetryState(receipt applicationnotes.OperationReceipt) (int, time.Time, bool, error) {
	var before struct {
		Note info.Note `json:"Note"`
	}
	if len(receipt.BeforeState) == 0 || json.Unmarshal(receipt.BeforeState, &before) != nil || before.Note.NoteId.IsZero() {
		return 0, time.Time{}, false, fmt.Errorf("restore move retry: invalid before state")
	}
	var desired struct {
		Metadata map[string]json.RawMessage `json:"Metadata"`
	}
	if len(receipt.DesiredState) == 0 || json.Unmarshal(receipt.DesiredState, &desired) != nil {
		return 0, time.Time{}, false, fmt.Errorf("restore move retry: invalid desired state")
	}
	var publicTime time.Time
	raw, present := desired.Metadata["PublicTime"]
	if present {
		if err := json.Unmarshal(raw, &publicTime); err != nil {
			return 0, time.Time{}, false, fmt.Errorf("restore move retry public time: %w", err)
		}
	}
	return before.Note.Usn, publicTime, present, nil
}

func replayCommittedMoveNote(note info.Note, receipt applicationnotes.OperationReceipt) info.Note {
	if receipt.AssignedUSN > 0 {
		note.Usn = receipt.AssignedUSN
	}
	return note
}

func (this *NoteService) MoveNoteWithOperation(noteId, notebookId, userId, operationID string) info.Note {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(userId) {
		return info.Note{}
	}
	if notebookService.IsMyNotebook(notebookId, userId) {
		note := this.GetNote(noteId, userId)
		if note.NoteId.IsZero() || note.IsDeleted {
			return info.Note{}
		}
		metadata := map[string]any{"IsTrash": false, "NotebookId": db.MustObjectIDFromHex(notebookId)}
		expected := note.Usn
		var frozenPublicTime time.Time
		hasFrozenPublicTime := false
		if operationID != "" {
			normalizedID, err := applicationnotes.NewClientOperationIdentity("note_save", note.UserId, operationID)
			if err != nil {
				return info.Note{}
			}
			receipt, err := db.GetWorkspaceOperation(context.Background(), note.UserId, normalizedID)
			if err == nil {
				if receipt.ResourceID != note.NoteId {
					return info.Note{}
				}
				if receipt.Status == applicationnotes.OperationCommitted {
					return replayCommittedMoveNote(note, receipt)
				}
				if applicationnotes.IsTerminalOperation(receipt.Status) {
					return info.Note{}
				}
				expected, frozenPublicTime, hasFrozenPublicTime, err = restoreMoveRetryState(receipt)
				if err != nil {
					return info.Note{}
				}
			} else if !errors.Is(err, mongo.ErrNoDocuments) {
				return info.Note{}
			}
		}
		if hasFrozenPublicTime {
			metadata["IsBlog"] = true
			metadata["PublicTime"] = frozenPublicTime
		} else if !note.IsBlog && notebookService.IsBlog(notebookId) {
			metadata["IsBlog"] = true
			metadata["PublicTime"] = time.Now()
		}
		result := this.SaveNote(applicationnotes.SaveNoteCommand{
			ActorUserID: userId, NoteID: noteId, OperationID: operationID, ExpectedUSN: &expected,
			Metadata: metadata, UpdatedTime: time.Now(),
		})
		if !result.OK() {
			return info.Note{}
		}

		return this.GetNote(noteId, userId)
	}
	return info.Note{}
}

// 如果自己的blog状态是true, 不用改变,
// 否则, 如果notebookId的blog是true, 则改为true之
// 返回blog状态
// move, copy时用
func (this *NoteService) updateToNotebookBlog(noteId, notebookId, userId string) bool {
	if this.IsBlog(noteId) {
		return true
	}
	if notebookService.IsBlog(notebookId) {
		filter := bson.M{"_id": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(userId)}
		db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
		_ = db.Notes.UpdateOneMatchedContext(context.Background(), filter,
			bson.M{"$set": bson.M{"IsBlog": true, "PublicTime": time.Now()}}) // life
		return true
	}
	return false
}

// 判断是否是blog
func (this *NoteService) IsBlog(noteId string) bool {
	note := info.Note{}
	db.GetByQWithFields(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId)}, []string{"IsBlog"}, &note)
	return note.IsBlog
}

// 复制note
// 正常的可以用
// 先查, 再新建
// 要检查下notebookId是否是自己的
func (this *NoteService) CopyNote(noteId, notebookId, userId string) info.Note {
	return this.CopyNoteWithOperation(noteId, notebookId, userId, "")
}

func (this *NoteService) CopyNoteWithOperation(noteId, notebookId, userId, operationID string) info.Note {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(userId) {
		return info.Note{}
	}
	if notebookService.IsMyNotebook(notebookId, userId) {
		note := this.GetNote(noteId, userId)
		noteContent := this.GetNoteContent(noteId, userId)

		// A client operation generation freezes the destination identity before
		// the create side effects. Legacy calls retain the random destination and
		// are intentionally not retry-safe.
		if operationID == "" {
			note.NoteId = db.NewObjectID()
		} else {
			note.NoteId = stableCopyNoteIDForOwner(operationID, noteId, userId)
			copyID, digest, err := copyNoteOperationIdentity("note_copy", note.UserId, note.NoteId, noteId, notebookId, "", operationID)
			if err != nil {
				return info.Note{}
			}
			if existing := this.GetNote(note.NoteId.Hex(), userId); !existing.NoteId.IsZero() {
				receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), note.UserId, copyID)
				if !copyReceiptCanResume(receipt, receiptErr, digest) {
					return info.Note{}
				}
				// Always re-enter the durable create path. A committed primary
				// receipt may still have a missing projection receipt that must be
				// repaired before this retry can report success.
			}
		}
		note.NotebookId = db.MustObjectIDFromHex(notebookId)

		noteContent.NoteId = note.NoteId
		if operationID == "" {
			note = this.AddNoteAndContent(note, noteContent, note.UserId)
		} else {
			copyID, digest, err := copyNoteOperationIdentity("note_copy", note.UserId, note.NoteId, noteId, notebookId, "", operationID)
			if err != nil {
				return info.Note{}
			}
			var ok bool
			note, ok, _ = this.addNoteAndContentResultWithIdentity(note, noteContent, note.UserId, false, true, copyID, digest, nil)
			if !ok {
				return info.Note{}
			}
		}
		if note.NoteId.IsZero() {
			return info.Note{}
		}

		// 更新blog状态
		isBlog := this.updateToNotebookBlog(note.NoteId.Hex(), notebookId, userId)

		note.IsBlog = isBlog

		return note
	}

	return info.Note{}
}

// 复制别人的共享笔记给我
// 将别人可用的图片转为我的图片, 复制图片
func (this *NoteService) CopySharedNote(noteId, notebookId, fromUserId, myUserId string) info.Note {
	return this.CopySharedNoteWithOperation(noteId, notebookId, fromUserId, myUserId, "")
}

func (this *NoteService) CopySharedNoteWithOperation(noteId, notebookId, fromUserId, myUserId, operationID string) info.Note {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(notebookId) || !db.IsValidObjectIDHex(fromUserId) || !db.IsValidObjectIDHex(myUserId) {
		return info.Note{}
	}
	// 判断是否共享了给我
	// Log(notebookService.IsMyNotebook(notebookId, myUserId))
	if notebookService.IsMyNotebook(notebookId, myUserId) && shareService.HasReadPerm(fromUserId, myUserId, noteId) {
		note := this.GetNote(noteId, fromUserId)
		if note.NoteId.IsZero() {
			return info.Note{}
		}
		noteContent := this.GetNoteContent(noteId, fromUserId)
		sourceContent := noteContent
		destinationExists := false
		var frozenAssets []applicationnotes.OperationAsset

		if operationID == "" {
			note.NoteId = db.NewObjectID()
		} else {
			note.NoteId = stableCopyNoteIDForOwner(operationID, noteId, myUserId)
			copyID, digest, err := copyNoteOperationIdentity("note_shared_copy", db.MustObjectIDFromHex(myUserId), note.NoteId, noteId, notebookId, fromUserId, operationID)
			if err != nil {
				return info.Note{}
			}
			receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), db.MustObjectIDFromHex(myUserId), copyID)
			if receiptErr == nil {
				if !copyReceiptCanResume(receipt, nil, digest) {
					return info.Note{}
				}
				frozenAssets = append([]applicationnotes.OperationAsset(nil), receipt.Assets...)
			} else if !errors.Is(receiptErr, mongo.ErrNoDocuments) {
				return info.Note{}
			} else {
				frozenAssets, err = stableSharedCopyAssetManifest(noteId, myUserId, operationID, sourceContent.Content)
				if err != nil {
					return info.Note{}
				}
			}
			if existing := this.GetNote(note.NoteId.Hex(), myUserId); !existing.NoteId.IsZero() {
				if receiptErr == nil {
					destinationExists = true
				} else {
					return info.Note{}
				}
			}
		}
		destination := this.GetNote(note.NoteId.Hex(), myUserId)
		destinationExists = destinationExists || !destination.NoteId.IsZero()
		if destinationExists {
			note = destination
			noteContent = this.GetNoteContent(note.NoteId.Hex(), myUserId)
			// The committed destination content is the durable source snapshot for
			// this operation. Never re-read later source edits into a retry.
			sourceContent = noteContent
		}
		note.NotebookId = db.MustObjectIDFromHex(notebookId)
		note.UserId = db.MustObjectIDFromHex(myUserId)
		note.IsTop = false
		note.IsBlog = false // 别人的可能是blog

		note.ImgSrc = "" // 为什么清空, 因为图片需要复制, 先清空

		// content
		noteContent.NoteId = note.NoteId
		noteContent.UserId = note.UserId
		if operationID == "" {
			// Preserve the legacy, non-retry-safe ordering and one-USN create
			// boundary. Historically image/attachment copies happened before the
			// destination note was inserted, and copy failures were not surfaced.
			noteContent.Content = noteImageService.CopyNoteImages(noteId, fromUserId, note.NoteId.Hex(), noteContent.Content, myUserId)
			attachService.CopyAttachs(noteId, note.NoteId.Hex(), myUserId)
			note = this.AddNoteAndContent(note, noteContent, note.UserId)
			if note.NoteId.IsZero() {
				return info.Note{}
			}
			note.IsBlog = this.updateToNotebookBlog(note.NoteId.Hex(), notebookId, myUserId)
			return note
		}

		// 添加之. A stable destination can be read back on a retry; the
		// operation receipt then supplies the original committed result.
		if operationID != "" {
			copyID, digest, err := copyNoteOperationIdentity("note_shared_copy", note.UserId, note.NoteId, noteId, notebookId, fromUserId, operationID)
			if err != nil {
				return info.Note{}
			}
			var ok bool
			note, ok, _ = this.addNoteAndContentResultWithIdentity(note, noteContent, note.UserId, false, true, copyID, digest, frozenAssets)
			if !ok {
				return info.Note{}
			}
			receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), note.UserId, copyID)
			if !copyReceiptCanResume(receipt, receiptErr, digest) {
				return info.Note{}
			}
			frozenAssets = append([]applicationnotes.OperationAsset(nil), receipt.Assets...)
			noteContent = this.GetNoteContent(note.NoteId.Hex(), myUserId)
			sourceContent = noteContent
		}
		if note.NoteId.IsZero() {
			return info.Note{}
		}

		// Asset copies start only after the destination note/create receipt is
		// durable. Retries reuse the destination and can safely finish the
		// projection instead of returning the partially copied note early.
		copiedContent, copyErr := noteImageService.CopyNoteImagesWithManifest(noteId, fromUserId, note.NoteId.Hex(), sourceContent.Content, myUserId, operationID, frozenAssets)
		if copyErr != nil {
			return info.Note{}
		}
		if copiedContent != noteContent.Content {
			expected := note.Usn
			content := copiedContent
			abstract := noteContent.Abstract
			contentOperationID := ""
			if operationID != "" {
				contentOperationID = operationID + ":copied-content"
			}
			updated := this.SaveNote(applicationnotes.SaveNoteCommand{ActorUserID: myUserId, NoteID: note.NoteId.Hex(), OperationID: contentOperationID, ExpectedUSN: &expected, Content: &content, Abstract: &abstract, UpdatedTime: time.Now()})
			if !updated.OK() {
				return info.Note{}
			}
			note = this.GetNote(note.NoteId.Hex(), myUserId)
			noteContent = this.GetNoteContent(note.NoteId.Hex(), myUserId)
		}
		if !attachService.CopyAttachsWithManifest(noteId, note.NoteId.Hex(), myUserId, operationID, frozenAssets) {
			return info.Note{}
		}

		// 更新blog状态
		isBlog := this.updateToNotebookBlog(note.NoteId.Hex(), notebookId, myUserId)

		note.IsBlog = isBlog
		return note
	}

	return info.Note{}
}

func stableCopyNoteID(operationID, sourceNoteID string) domain.ObjectID {
	return stableCopyNoteIDForOwner(operationID, sourceNoteID, "")
}

func copyNoteOperationIdentity(kind string, ownerID, destinationID domain.ObjectID, sourceNoteID, notebookID, sharedOwnerID, clientOperationID string) (string, string, error) {
	operationID, digest, _, err := applicationnotes.NewOperationIdentity(kind, ownerID, destinationID, struct {
		SourceNoteID  string
		NotebookID    string
		SharedOwnerID string
		OperationID   string
	}{SourceNoteID: sourceNoteID, NotebookID: notebookID, SharedOwnerID: sharedOwnerID, OperationID: clientOperationID})
	return operationID, digest, err
}

func stableSharedCopyAssetManifest(sourceNoteID, destinationOwnerID, operationID, content string) ([]applicationnotes.OperationAsset, error) {
	assets := make([]applicationnotes.OperationAsset, 0)
	for _, sourceImageID := range noteImageSourceFileIDs(content) {
		imageOperationID := operationID + ":image:" + sourceImageID
		assets = append(assets, applicationnotes.OperationAsset{
			AssetID:     stableCopiedImageID(imageOperationID, sourceImageID, destinationOwnerID).Hex(),
			LocalFileID: sourceImageID,
			Index:       len(assets),
		})
	}
	var attachments []info.Attach
	if err := db.Attachs.FindContext(context.Background(), bson.M{"NoteId": db.MustObjectIDFromHex(sourceNoteID)}).All(&attachments); err != nil {
		return nil, err
	}
	slices.SortFunc(attachments, func(left, right info.Attach) int {
		return strings.Compare(left.AttachId.Hex(), right.AttachId.Hex())
	})
	for _, attach := range attachments {
		sourceAttachID := attach.AttachId.Hex()
		assets = append(assets, applicationnotes.OperationAsset{
			AssetID:     stableCopiedAttachID(operationID, sourceAttachID, destinationOwnerID).Hex(),
			LocalFileID: sourceAttachID,
			Index:       len(assets),
			IsAttach:    true,
		})
	}
	return assets, nil
}

func copyReceiptCanResume(receipt applicationnotes.OperationReceipt, receiptErr error, inputDigest string) bool {
	if receiptErr != nil || receipt.InputDigest != inputDigest {
		return false
	}
	return receipt.Status == applicationnotes.OperationCommitted || !applicationnotes.IsTerminalOperation(receipt.Status)
}

func stableCopyNoteIDForOwner(operationID, sourceNoteID, destinationOwnerID string) domain.ObjectID {
	sum := sha256.Sum256([]byte("note-copy-destination\x00" + operationID + "\x00" + sourceNoteID + "\x00" + destinationOwnerID))
	id, err := domain.ParseObjectID(hex.EncodeToString(sum[:12]))
	if err != nil || id.IsZero() {
		return db.NewObjectID()
	}
	return id
}

// 通过noteId得到notebookId
// shareService call
// [ok]
func (this *NoteService) GetNotebookId(noteId string) ObjectID {
	note := info.Note{}
	// db.Get(db.Notes, noteId, &note)
	// LogJ(note)
	db.GetByQWithFields(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId)}, []string{"NotebookId"}, &note)
	return note.NotebookId
}

// ------------------
// 搜索Note, 博客使用了
func (this *NoteService) SearchNote(key, userId string, pageNumber, pageSize int, sortField string, isAsc, isBlog bool) (count int, notes []info.Note) {
	notes = []info.Note{}
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)

	// 利用标题和desc, 不用content
	orQ := []bson.M{
		bson.M{"Title": bson.M{"$regex": bson.Regex{Pattern: ".*?" + key + ".*", Options: "i"}}},
		bson.M{"Desc": bson.M{"$regex": bson.Regex{Pattern: ".*?" + key + ".*", Options: "i"}}},
	}
	// 不是trash的
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId),
		"IsTrash":   false,
		"IsDeleted": false, // 不能搜索已删除了的
		"$or":       orQ,
	}
	if isBlog {
		query["IsBlog"] = true
	}
	q := db.Notes.Find(query)

	// 总记录数
	count, _ = q.Count()

	q.Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes)

	// 如果 < pageSize 那么搜索content, 且id不在这些id之间的
	if len(notes) < pageSize {
		notes = this.searchNoteFromContent(notes, userId, key, pageSize, sortFieldR, isBlog)
	}
	return
}

// 搜索noteContents, 补集pageSize个
func (this *NoteService) searchNoteFromContent(notes []info.Note, userId, key string, pageSize int, sortField string, isBlog bool) []info.Note {
	var remain = pageSize - len(notes)
	noteIds := make([]ObjectID, len(notes))
	for i, note := range notes {
		noteIds[i] = note.NoteId
	}
	noteContents := []info.NoteContent{}
	query := bson.M{
		"_id":     bson.M{"$nin": noteIds},
		"UserId":  db.MustObjectIDFromHex(userId),
		"Content": bson.M{"$regex": bson.Regex{Pattern: ".*?" + key + ".*", Options: "i"}},
	}
	if isBlog {
		query["IsBlog"] = true
	}
	db.NoteContents.
		Find(query).
		Sort(sortField).
		Limit(remain).
		Select(bson.M{"_id": true}).
		All(&noteContents)
	var lenContent = len(noteContents)
	if lenContent == 0 {
		return notes
	}

	// 收集ids
	noteIds2 := make([]ObjectID, lenContent)
	for i, content := range noteContents {
		noteIds2[i] = content.NoteId
	}

	// 得到notes
	notes2 := this.ListNotesByNoteIds(noteIds2)

	// 合并之
	// 不能是删除的
	for _, n := range notes2 {
		if !n.IsDeleted && !n.IsTrash {
			// notes = append(notes, notes2...)
			notes = append(notes, n)
		}
	}
	return notes
}

// ----------------
// tag搜索
func (this *NoteService) SearchNoteByTags(tags []string, userId string, pageNumber, pageSize int, sortField string, isAsc bool) (count int, notes []info.Note) {
	notes = []info.Note{}
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)

	// 不是trash的
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId),
		"IsTrash": false,
		"Tags":    bson.M{"$all": tags}}

	q := db.Notes.Find(query)

	// 总记录数
	count, _ = q.Count()

	q.Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes)
	return
}

// ------------
// 统计
func (this *NoteService) CountNote(userId string) int {
	q := bson.M{"IsTrash": false, "IsDeleted": false}
	if userId != "" {
		q["UserId"] = db.MustObjectIDFromHex(userId)
	}
	return db.Count(db.Notes, q)
}
func (this *NoteService) CountBlog(userId string) int {
	q := bson.M{"IsBlog": true, "IsTrash": false, "IsDeleted": false}
	if userId != "" {
		q["UserId"] = db.MustObjectIDFromHex(userId)
	}
	return db.Count(db.Notes, q)
}

// 通过标签来查询
func (this *NoteService) CountNoteByTag(userId string, tag string) int {
	if tag == "" {
		return 0
	}
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId),
		//		"IsTrash": false,
		"IsDeleted": false,
		"Tags":      bson.M{"$in": []string{tag}}}
	return db.Count(db.Notes, query)
}

// 删除tag
// 返回所有note的Usn
func (this *NoteService) UpdateNoteToDeleteTag(userId string, targetTag string) map[string]int {
	items, _ := this.UpdateNoteToDeleteTagResult(userId, targetTag)
	return items
}

func (this *NoteService) UpdateNoteToDeleteTagResult(userId string, targetTag string) (map[string]int, bool) {
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId),
		"Tags": bson.M{"$in": []string{targetTag}}}
	notes := []info.Note{}
	db.ListByQ(db.Notes, query, &notes)
	ret := map[string]int{}
	for _, note := range notes {
		tags := note.Tags
		if tags == nil {
			continue
		}
		for i, tag := range tags {
			if tag == targetTag {
				tags = append(tags[:i], tags[i+1:]...)
				break
			}
		}
		usn, err := userService.AllocateUsn(context.Background(), userId)
		if err != nil {
			return ret, false
		}
		filter := bson.M{"_id": note.NoteId, "UserId": db.MustObjectIDFromHex(userId), "Usn": note.Usn, "IsDeleted": false}
		db.AddWorkspaceNoteMutationLeaseFilter(filter, "", time.Now())
		if err := db.Notes.UpdateOneMatchedContext(context.Background(),
			filter,
			bson.M{"$set": bson.M{"Usn": usn, "Tags": tags}},
		); err != nil {
			return ret, false
		}
		ret[note.NoteId.Hex()] = usn
	}
	return ret, true
}

// api

// 得到笔记的内容, 此时将笔记内的链接转成标准的Leanote Url
// 将笔记的图片, 附件链接转换成 site.url/file/getImage?fileId=xxx,  site.url/file/getAttach?fileId=xxxx
func (this *NoteService) FixContentBad(content string, isMarkdown bool) string {
	baseUrl := configService.GetSiteUrl()

	baseUrlPattern := baseUrl

	// 避免https的url
	if baseUrl[0:8] == "https://" {
		baseUrlPattern = strings.Replace(baseUrl, "https://", "https*://", 1)
	} else {
		baseUrlPattern = strings.Replace(baseUrl, "http://", "https*://", 1)
	}

	patterns := []map[string]string{
		map[string]string{"src": "src", "middle": "/file/outputImage", "param": "fileId", "to": "getImage?fileId="},
		map[string]string{"src": "href", "middle": "/attach/download", "param": "attachId", "to": "getAttach?fileId="},
		// 该链接已失效, 不再支持
		map[string]string{"src": "href", "middle": "/attach/downloadAll", "param": "noteId", "to": "getAllAttachs?noteId="},
	}

	for _, eachPattern := range patterns {

		if !isMarkdown {

			// 富文本处理

			// <img src="http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1">
			// <a href="http://leanote.com/attach/download?attachId=5504243a38f4111dcb00017d"></a>

			var reg *regexp.Regexp
			if eachPattern["src"] == "src" {
				reg, _ = regexp.Compile("<img(?:[^>]+?)(" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})["']*)[^>]*>`)
			} else {
				reg, _ = regexp.Compile("<a(?:[^>]+?)(" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})["']*)[^>]*>`)
			}

			finds := reg.FindAllStringSubmatch(content, -1) // 查找所有的

			for _, eachFind := range finds {
				if len(eachFind) == 3 {
					// 这一行会非常慢!, content是全部的内容, 多次replace导致
					content = strings.Replace(content,
						eachFind[1],
						eachPattern["src"]+"=\""+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+"\"",
						1)
				}
			}
		} else {

			// markdown处理
			// ![](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1)
			// [selection 2.html](http://leanote.com/attach/download?attachId=5504262638f4111dcb00017f)
			// [all.tar.gz](http://leanote.com/attach/downloadAll?noteId=5503b57d59f81b4eb4000000)

			pre := "!"                        // 默认图片
			if eachPattern["src"] == "href" { // 是attach
				pre = ""
			}

			regImageMarkdown, _ := regexp.Compile(pre + `\[([^]]*?)\]\(` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})\)`)
			findsImageMarkdown := regImageMarkdown.FindAllStringSubmatch(content, -1) // 查找所有的
			// [[![](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 5503537b38f4111dcb0000d1] [![你好啊, 我很好, 为什么?](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 5503537b38f4111dcb0000d1]]
			for _, eachFind := range findsImageMarkdown {
				// [![你好啊, 我很好, 为什么?](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 你好啊, 我很好, 为什么? 5503537b38f4111dcb0000d1]
				if len(eachFind) == 3 {
					content = strings.Replace(content, eachFind[0], pre+"["+eachFind[1]+"]("+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+")", 1)
				}
			}
		}
	}

	return content
}

// 得到笔记的内容, 此时将笔记内的链接转成标准的Leanote Url
// 将笔记的图片, 附件链接转换成 site.url/file/getImage?fileId=xxx,  site.url/file/getAttach?fileId=xxxx
// 性能更好, 5倍的差距
func (this *NoteService) FixContent(content string, isMarkdown bool) string {
	baseUrl := configService.GetSiteUrl()

	baseUrlPattern := baseUrl

	// 避免https的url
	if baseUrl[0:8] == "https://" {
		baseUrlPattern = strings.Replace(baseUrl, "https://", "https*://", 1)
	} else {
		baseUrlPattern = strings.Replace(baseUrl, "http://", "https*://", 1)
	}
	baseUrlPattern = "(?:" + baseUrlPattern + ")*"

	Log(baseUrlPattern)

	patterns := []map[string]string{
		map[string]string{"src": "src", "middle": "/api/file/getImage", "param": "fileId", "to": "getImage?fileId="},
		map[string]string{"src": "src", "middle": "/file/outputImage", "param": "fileId", "to": "getImage?fileId="},

		map[string]string{"src": "href", "middle": "/attach/download", "param": "attachId", "to": "getAttach?fileId="},
		map[string]string{"src": "href", "middle": "/api/file/getAtach", "param": "fileId", "to": "getAttach?fileId="},

		// 该链接已失效, 不再支持
		// map[string]string{"src": "href", "middle": "/attach/downloadAll", "param": "noteId", "to": "getAllAttachs?noteId="},
	}

	for _, eachPattern := range patterns {

		if !isMarkdown {

			// 富文本处理

			// <img src="http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1">
			// <a href="http://leanote.com/attach/download?attachId=5504243a38f4111dcb00017d"></a>

			var reg *regexp.Regexp
			var reg2 *regexp.Regexp
			if eachPattern["src"] == "src" {
				reg, _ = regexp.Compile("<img(?:[^>]+?)(?:" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=(?:[a-z0-9A-Z]{24})["']*)[^>]*>`)
				reg2, _ = regexp.Compile("<img(?:[^>]+?)(" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})["']*)[^>]*>`)
			} else {
				reg, _ = regexp.Compile("<a(?:[^>]+?)(?:" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=(?:[a-z0-9A-Z]{24})["']*)[^>]*>`)
				reg2, _ = regexp.Compile("<a(?:[^>]+?)(" + eachPattern["src"] + `=['"]*` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})["']*)[^>]*>`)
			}

			// Log(reg2)

			content = reg.ReplaceAllStringFunc(content, func(str string) string {
				// str=这样的
				// <img src="http://localhost:9000/file/outputImage?fileId=563d706e99c37b48e0000001" alt="" data-mce-src="http://localhost:9000/file/outputImage?fileId=563d706e99c37b48e0000002">

				eachFind := reg2.FindStringSubmatch(str)
				str = strings.Replace(str,
					eachFind[1],
					eachPattern["src"]+"=\""+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+"\"",
					1)

				// fmt.Println(str)
				return str
			})
			/*
				finds := reg.FindAllStringSubmatch(content, -1) // 查找所有的

				for _, eachFind := range finds {
					if len(eachFind) == 3 {
						// 这一行会非常慢!, content是全部的内容, 多次replace导致
						content = strings.Replace(content,
							eachFind[1],
							eachPattern["src"]+"=\""+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+"\"",
							1)
					}
				}
			*/
		} else {

			// markdown处理
			// ![](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1)
			// [selection 2.html](http://leanote.com/attach/download?attachId=5504262638f4111dcb00017f)
			// [all.tar.gz](http://leanote.com/attach/downloadAll?noteId=5503b57d59f81b4eb4000000)

			pre := "!"                        // 默认图片
			if eachPattern["src"] == "href" { // 是attach
				pre = ""
			}

			regImageMarkdown, _ := regexp.Compile(pre + `\[(?:[^]]*?)\]\(` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=(?:[a-z0-9A-Z]{24})\)`)
			regImageMarkdown2, _ := regexp.Compile(pre + `\[([^]]*?)\]\(` + baseUrlPattern + eachPattern["middle"] + `\?` + eachPattern["param"] + `=([a-z0-9A-Z]{24})\)`)

			content = regImageMarkdown.ReplaceAllStringFunc(content, func(str string) string {
				// str=这样的
				// <img src="http://localhost:9000/file/outputImage?fileId=563d706e99c37b48e0000001" alt="" data-mce-src="http://localhost:9000/file/outputImage?fileId=563d706e99c37b48e0000002">

				eachFind := regImageMarkdown2.FindStringSubmatch(str)
				str = strings.Replace(str, eachFind[0], pre+"["+eachFind[1]+"]("+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+")", 1)

				// fmt.Println(str)
				return str
			})

			/*
				findsImageMarkdown := regImageMarkdown.FindAllStringSubmatch(content, -1) // 查找所有的
				// [[![](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 5503537b38f4111dcb0000d1] [![你好啊, 我很好, 为什么?](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 5503537b38f4111dcb0000d1]]
				for _, eachFind := range findsImageMarkdown {
					// [![你好啊, 我很好, 为什么?](http://leanote.com/file/outputImage?fileId=5503537b38f4111dcb0000d1) 你好啊, 我很好, 为什么? 5503537b38f4111dcb0000d1]
					if len(eachFind) == 3 {
						content = strings.Replace(content, eachFind[0], pre+"["+eachFind[1]+"]("+baseUrl+"/api/file/"+eachPattern["to"]+eachFind[2]+")", 1)
					}
				}
			*/
		}
	}

	return content
}
