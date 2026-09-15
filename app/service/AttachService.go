package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/revel/revel"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type AttachService struct {
}

type webAttachUploadState struct {
	ExpectedUSN int `json:"expectedUsn"`
}

func webAttachUploadIdentity(ownerID, noteID, attachID domain.ObjectID, clientOperationID, title, fileType string, _ int, data []byte) (string, string, error) {
	operationID, err := applicationnotes.NewClientOperationIdentity("web_attach_upload", ownerID, clientOperationID)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(data)
	_, inputDigest, _, err := applicationnotes.NewOperationIdentity("web_attach_upload_input", ownerID, noteID, struct {
		AttachID string
		Title    string
		Type     string
		Size     int
		SHA256   string
	}{AttachID: attachID.Hex(), Title: title, Type: fileType, Size: len(data), SHA256: hex.EncodeToString(digest[:])})
	return operationID, inputDigest, err
}

// UploadWebAttach claims the canonical request before publishing bytes. Stable
// requests use a durable outer receipt so a crash can verify the file/row/note
// combination and resume without overwriting an already committed asset.
func (this *AttachService) UploadWebAttach(attach info.Attach, data []byte, clientOperationID string) (bool, string) {
	note := noteService.GetNoteById(attach.NoteId.Hex())
	if note.NoteId.IsZero() || note.IsDeleted || !shareService.HasUpdateNotePerm(note.NoteId.Hex(), attach.UploadUserId.Hex()) {
		return false, "No Perm"
	}
	attach.Size = int64(len(data))
	target := filepath.Join(revel.BasePath, filepath.FromSlash(strings.TrimLeft(attach.Path, "/")))
	if clientOperationID == "" {
		if err := publishFileNoClobber(target, data, 0777); err != nil {
			return false, "db error"
		}
		return this.AddAttachWithOperation(attach, false, "")
	}
	operationID, digest, err := webAttachUploadIdentity(note.UserId, note.NoteId, attach.AttachId, clientOperationID, attach.Title, attach.Type, note.Usn, data)
	if err != nil {
		return false, "db error"
	}
	contentDigest := sha256.Sum256(data)
	asset := applicationnotes.OperationAsset{AssetID: attach.AttachId.Hex(), IsAttach: true, ContentSHA256: hex.EncodeToString(contentDigest[:])}
	state := webAttachUploadState{ExpectedUSN: note.Usn}
	before, err := applicationnotes.CanonicalState(state)
	if err != nil {
		return false, "db error"
	}
	verifyFile := func() (bool, error) {
		got, statErr := fileDigest(target)
		if errors.Is(statErr, os.ErrNotExist) {
			return false, nil
		}
		return got == contentDigest, statErr
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: note.UserId, ResourceID: note.NoteId,
		Kind: "web_attach_upload", InputDigest: digest, Assets: []applicationnotes.OperationAsset{asset},
		BeforeState: before,
		RestoreBeforeState: func(payload []byte) error {
			return json.Unmarshal(payload, &state)
		},
		FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{
			{Name: "publish_file", ReplaySafe: false,
				Apply:  func(context.Context) error { return publishFileNoClobber(target, data, 0777) },
				Verify: func(context.Context) (bool, error) { return verifyFile() },
			},
			{Name: "attach_and_note", ReplaySafe: true,
				Apply: func(context.Context) error {
					// This step is the combined file/row/note boundary. A resumed
					// operation may reach it after the earlier publish step was
					// persisted, so re-establish the no-clobber file invariant before
					// accepting an existing row as success.
					if err := publishFileNoClobber(target, data, 0777); err != nil {
						return err
					}
					ok, msg := this.addAttachToNoteAtGeneration(attach, clientOperationID, state.ExpectedUSN)
					if !ok {
						return fmt.Errorf("web attachment mutation: %s", msg)
					}
					return nil
				},
				Verify: func(ctx context.Context) (bool, error) {
					var found info.Attach
					if err := db.Attachs.FindContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": note.NoteId, "UploadUserId": attach.UploadUserId, "Path": attach.Path, "Size": attach.Size}).One(&found); err != nil {
						if errors.Is(err, mongo.ErrNoDocuments) {
							return false, nil
						}
						return false, err
					}
					fileOK, err := verifyFile()
					return fileOK && this.verifyAttachNum(ctx, note.NoteId, note.UserId, attach.AttachId), err
				},
			},
		},
	}
	result, runErr := db.RunWorkspaceRepair(context.Background(), plan)
	if runErr != nil || !result.Committed {
		if result.PartialWrite {
			return false, "partial_write"
		}
		if errors.Is(runErr, applicationnotes.ErrOperationConflict) {
			return false, "conflict"
		}
		return false, "db error"
	}
	return true, "attach success"
}

// publishFileNoClobber stages bytes beside the destination and atomically
// links them into place. An existing destination is accepted only when its
// complete digest matches, so a conflicting retry can never overwrite the
// committed asset.
func publishFileNoClobber(target string, data []byte, mode os.FileMode) error {
	if existing, err := os.ReadFile(target); err == nil {
		if sha256.Sum256(existing) == sha256.Sum256(data) {
			return nil
		}
		return fmt.Errorf("asset identity conflict")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	pending := target + ".pending"
	file, err := os.OpenFile(pending, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Link(pending, target); err != nil {
		if existing, readErr := os.ReadFile(target); readErr == nil && sha256.Sum256(existing) == sha256.Sum256(data) {
			_ = os.Remove(pending)
			return nil
		}
		return err
	}
	if dir, err := os.Open(filepath.Dir(target)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	if err := os.Remove(pending); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func fileDigest(path string) ([32]byte, error) {
	var digest [32]byte
	file, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

// add attach
// api调用时, 添加attach之前是没有note的
// fromApi表示是api添加的, updateNote传过来的, 此时不要incNote's usn, 因为updateNote会inc的
func (this *AttachService) AddAttach(attach info.Attach, fromApi bool) (ok bool, msg string) {
	return this.AddAttachWithOperation(attach, fromApi, "")
}

func (this *AttachService) AddAttachWithOperation(attach info.Attach, fromApi bool, clientOperationID string) (ok bool, msg string) {
	attach.CreatedTime = time.Now()
	if !fromApi {
		return this.addAttachToNote(attach, clientOperationID)
	}
	ok = db.Insert(db.Attachs, attach)
	return
}

// addAttachToNote makes the note generation the commit fence for the Web
// attachment entry point. The attachment row and file already exist when
// this method is called, but they are not exposed as a success until the
// note's conditional mutation has acquired its lease and committed.
func (this *AttachService) addAttachToNote(attach info.Attach, clientOperationID string) (bool, string) {
	note := noteService.GetNoteById(attach.NoteId.Hex())
	if note.NoteId.IsZero() || note.IsDeleted || !shareService.HasUpdateNotePerm(note.NoteId.Hex(), attach.UploadUserId.Hex()) {
		return false, "No Perm"
	}
	return this.addAttachToNoteAtGeneration(attach, clientOperationID, note.Usn)
}

func (this *AttachService) addAttachToNoteAtGeneration(attach info.Attach, clientOperationID string, expectedGeneration int) (bool, string) {
	note := noteService.GetNoteById(attach.NoteId.Hex())
	if note.NoteId.IsZero() || note.IsDeleted || !shareService.HasUpdateNotePerm(note.NoteId.Hex(), attach.UploadUserId.Hex()) {
		return false, "No Perm"
	}
	if attach.CreatedTime.IsZero() {
		attach.CreatedTime = time.Now()
	}
	operationID := attachMutationIdentity(note.UserId, note.NoteId, attach.AttachId.Hex(), expectedGeneration)
	if clientOperationID != "" {
		operationID, _ = applicationnotes.NewClientOperationIdentity("web_attach_add", note.UserId, clientOperationID)
	}
	if operationID == "" {
		return false, "db error"
	}
	expectedUSN := expectedGeneration
	assetWork := &applicationnotes.AssetMutation{
		Assets: []applicationnotes.OperationAsset{{AssetID: attach.AttachId.Hex(), IsAttach: true}},
	}
	assetWork.Apply = func(ctx context.Context, committedUSN int) error {
		if err := this.insertAttachIfMissing(ctx, attach, attach.UploadUserId); err != nil {
			return err
		}
		return this.updateNoteAttachNumContext(ctx, note.NoteId, note.UserId, &committedUSN, assetWork.OperationID)
	}
	assetWork.Verify = func(ctx context.Context) (bool, error) {
		var found info.Attach
		err := db.Attachs.FindContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": note.NoteId, "UploadUserId": attach.UploadUserId}).One(&found)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return this.verifyAttachNum(ctx, note.NoteId, note.UserId, attach.AttachId), nil
	}
	result := noteService.SaveNote(applicationnotes.SaveNoteCommand{
		ActorUserID: attach.UploadUserId.Hex(), NoteID: note.NoteId.Hex(), OperationID: operationID,
		ExpectedUSN: &expectedUSN, AssetWork: assetWork, UpdatedTime: time.Now(),
	})
	if result.OK() {
		return true, "attach success"
	}
	if result.PartialWrite {
		return false, "partial_write"
	}
	return false, "db error"
}

func attachMutationIdentity(ownerID, noteID domain.ObjectID, attachID string, generation int) string {
	operationID, _, _, err := applicationnotes.NewOperationIdentity("web_attach_add", ownerID, noteID, struct {
		AttachID   string
		Generation int
	}{AttachID: attachID, Generation: generation})
	if err != nil {
		return ""
	}
	return operationID
}

// StableWebAttachID reserves the attachment row identity before the file is
// written, allowing a client retry to reuse the same path and row.
func StableWebAttachID(ownerID, noteID domain.ObjectID, clientOperationID string) domain.ObjectID {
	sum := sha256.Sum256([]byte("web-attach-asset\x00" + ownerID.Hex() + "\x00" + noteID.Hex() + "\x00" + strings.TrimSpace(clientOperationID)))
	var raw [12]byte
	copy(raw[:], sum[:12])
	if raw == ([12]byte{}) {
		raw[11] = 1
	}
	return domain.ObjectID(raw)
}

func (this *AttachService) insertAttachIfMissing(ctx context.Context, attach info.Attach, ownerID domain.ObjectID) error {
	var existing info.Attach
	err := db.Attachs.FindContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": attach.NoteId, "UploadUserId": ownerID}).One(&existing)
	if err == nil {
		if existing.Path != attach.Path || existing.Size != attach.Size {
			return fmt.Errorf("attachment identity conflict")
		}
		return nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	return db.Attachs.InsertContext(ctx, attach)
}

func (this *AttachService) verifyAttachNum(ctx context.Context, noteID, ownerID, attachID domain.ObjectID) bool {
	count, err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).Count()
	if err != nil {
		return false
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "AttachNum": count}).One(&note); err != nil {
		return false
	}
	return !note.NoteId.IsZero() && !attachID.IsZero()
}

// 更新笔记的附件个数
// addNum 1或-1
func (this *AttachService) updateNoteAttachNum(noteId ObjectID, addNum int) bool {
	num := db.Count(db.Attachs, bson.M{"NoteId": noteId})
	/*
		note := info.Note{}
		note = noteService.GetNoteById(noteId.Hex())
		note.AttachNum += addNum
		if note.AttachNum < 0 {
			note.AttachNum = 0
		}
		Log(note.AttachNum)
	*/
	return db.UpdateByQField(db.Notes, bson.M{"_id": noteId}, "AttachNum", num)
}

// list attachs
func (this *AttachService) ListAttachs(noteId, userId string) []info.Attach {
	attachs := []info.Attach{}

	// 判断是否有权限为笔记添加附件, userId为空时表示是分享笔记的附件
	if userId != "" && !shareService.HasUpdateNotePerm(noteId, userId) {
		return attachs
	}

	// 笔记是否是自己的
	note := noteService.GetNoteByIdAndUserId(noteId, userId)
	if note.NoteId.IsZero() {
		return attachs
	}

	// TODO 这里, 优化权限控制

	db.ListByQ(db.Attachs, bson.M{"NoteId": db.MustObjectIDFromHex(noteId)}, &attachs)

	return attachs
}

// api调用, 通过noteIds得到note's attachs, 通过noteId归类返回
func (this *AttachService) getAttachsByNoteIds(noteIds []ObjectID) map[string][]info.Attach {
	attachs := []info.Attach{}
	db.ListByQ(db.Attachs, bson.M{"NoteId": bson.M{"$in": noteIds}}, &attachs)
	noteAttchs := make(map[string][]info.Attach)
	for _, attach := range attachs {
		noteId := attach.NoteId.Hex()
		if itAttachs, ok := noteAttchs[noteId]; ok {
			noteAttchs[noteId] = append(itAttachs, attach)
		} else {
			noteAttchs[noteId] = []info.Attach{attach}
		}
	}
	return noteAttchs
}

func (this *AttachService) UpdateImageTitle(userId, fileId, title string) bool {
	return db.UpdateByIdAndUserIdField(db.Files, fileId, userId, "Title", title)
}

// Delete note to delete attas firstly
func (this *AttachService) DeleteAllAttachs(noteId, userId string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	return this.deleteAllAttachs(context.Background(), db.MustObjectIDFromHex(noteId), db.MustObjectIDFromHex(userId)) == nil
}

func (this *AttachService) deleteAllAttachs(ctx context.Context, noteID, ownerID domain.ObjectID) error {
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}).One(&note)
	if err != nil {
		return err
	}
	attachs := []info.Attach{}
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).All(&attachs); err != nil {
		return err
	}
	for _, attach := range attachs {
		path := strings.TrimLeft(attach.Path, "/")
		if err := os.Remove(revel.BasePath + "/" + path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	_, err = db.Attachs.RemoveAllContext(ctx, bson.M{"NoteId": noteID})
	return err
}

func (this *AttachService) verifyAllAttachsDeleted(ctx context.Context, noteID, ownerID domain.ObjectID) (bool, error) {
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}).One(&note)
	if err != nil {
		return false, err
	}
	var attach info.Attach
	err = db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).One(&attach)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	return false, err
}

// delete attach
// 删除附件为什么要incrNoteUsn ? 因为可能没有内容要修改的
func (this *AttachService) DeleteAttach(attachId, userId string) (bool, string) {
	return this.DeleteAttachWithOperation(attachId, userId, "")
}

type webAttachDeleteState struct {
	Attach info.Attach `json:"attach"`
	Note   info.Note   `json:"note"`
}

func webAttachDeleteIdentity(actorID, attachID domain.ObjectID, clientOperationID string) (string, string, error) {
	operationID, err := applicationnotes.NewClientOperationIdentity("web_attach_delete_request", actorID, clientOperationID)
	if err != nil {
		return "", "", err
	}
	_, digest, _, err := applicationnotes.NewOperationIdentity("web_attach_delete_input", actorID, attachID, struct{ AttachID string }{attachID.Hex()})
	return operationID, digest, err
}

func (this *AttachService) DeleteAttachWithOperation(attachIDText, userID, clientOperationID string) (bool, string) {
	if !db.IsValidObjectIDHex(attachIDText) || !db.IsValidObjectIDHex(userID) {
		return false, "no such item"
	}
	actorID := db.MustObjectIDFromHex(userID)
	attachID := db.MustObjectIDFromHex(attachIDText)
	if clientOperationID == "" {
		return this.deleteAttachCurrent(attachIDText, userID, "")
	}
	operationID, digest, err := webAttachDeleteIdentity(actorID, attachID, clientOperationID)
	if err != nil {
		return false, "db error"
	}
	state := webAttachDeleteState{}
	receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), actorID, operationID)
	if receiptErr == nil {
		if receipt.InputDigest != digest || len(receipt.Assets) != 1 || receipt.Assets[0].AssetID != attachIDText {
			return false, "conflict"
		}
		if receipt.Status == applicationnotes.OperationCommitted {
			return true, "delete file success"
		}
		if len(receipt.BeforeState) == 0 || json.Unmarshal(receipt.BeforeState, &state) != nil {
			return false, "partial_write"
		}
	} else if !errors.Is(receiptErr, mongo.ErrNoDocuments) {
		return false, "db error"
	} else {
		db.Get(db.Attachs, attachIDText, &state.Attach)
		if state.Attach.AttachId.IsZero() {
			return false, "no such item"
		}
		if !shareService.HasUpdateNotePerm(state.Attach.NoteId.Hex(), userID) {
			return false, "No Perm"
		}
		state.Note = noteService.GetNoteById(state.Attach.NoteId.Hex())
		if state.Note.NoteId.IsZero() || state.Note.IsDeleted {
			return false, "no such item"
		}
	}
	before, err := applicationnotes.CanonicalState(state)
	if err != nil {
		return false, "db error"
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: actorID, ResourceID: attachID, Kind: "web_attach_delete_request", InputDigest: digest,
		Assets: []applicationnotes.OperationAsset{{AssetID: attachIDText, IsAttach: true}}, BeforeState: before,
		RestoreBeforeState: func(payload []byte) error { return json.Unmarshal(payload, &state) }, FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{{Name: "delete_attachment", ReplaySafe: true,
			Apply: func(context.Context) error {
				ok, msg := this.deleteAttachState(state, userID, operationID+":note")
				if !ok {
					return fmt.Errorf("delete attachment: %s", msg)
				}
				return nil
			},
			Verify: func(ctx context.Context) (bool, error) {
				var found info.Attach
				err := db.Attachs.FindContext(ctx, bson.M{"_id": state.Attach.AttachId, "NoteId": state.Note.NoteId}).One(&found)
				if err == nil {
					return false, nil
				}
				if !errors.Is(err, mongo.ErrNoDocuments) {
					return false, err
				}
				if _, err := os.Stat(filepath.Join(revel.BasePath, filepath.FromSlash(strings.TrimLeft(state.Attach.Path, "/")))); err == nil {
					return false, nil
				} else if !errors.Is(err, os.ErrNotExist) {
					return false, err
				}
				return this.verifyAttachNumAfterDelete(ctx, state.Note.NoteId, state.Note.UserId), nil
			},
		}},
	}
	result, runErr := db.RunWorkspaceRepair(context.Background(), plan)
	if runErr == nil && result.Committed {
		return true, "delete file success"
	}
	if result.PartialWrite {
		return false, "partial_write"
	}
	if errors.Is(runErr, applicationnotes.ErrOperationConflict) {
		return false, "conflict"
	}
	return false, "db error"
}

func (this *AttachService) deleteAttachCurrent(attachID, userID, operationID string) (bool, string) {
	attach := info.Attach{}
	db.Get(db.Attachs, attachID, &attach)
	if attach.AttachId.IsZero() {
		return false, "no such item"
	}
	if !shareService.HasUpdateNotePerm(attach.NoteId.Hex(), userID) {
		return false, "No Perm"
	}
	note := noteService.GetNoteById(attach.NoteId.Hex())
	if note.NoteId.IsZero() || note.IsDeleted {
		return false, "no such item"
	}
	return this.deleteAttachState(webAttachDeleteState{Attach: attach, Note: note}, userID, operationID)
}

func (this *AttachService) deleteAttachState(state webAttachDeleteState, userID, operationID string) (bool, string) {
	attach, note := state.Attach, state.Note
	if operationID == "" {
		operationID = attachMutationIdentity(note.UserId, note.NoteId, attach.AttachId.Hex()+":delete", note.Usn)
	}
	expectedUSN := note.Usn
	assetWork := &applicationnotes.AssetMutation{Assets: []applicationnotes.OperationAsset{{AssetID: attach.AttachId.Hex(), IsAttach: true}}}
	assetWork.Apply = func(ctx context.Context, committedUSN int) error {
		path := filepath.Join(revel.BasePath, filepath.FromSlash(strings.TrimLeft(attach.Path, "/")))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := db.Attachs.RemoveContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": note.NoteId}); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		return this.updateNoteAttachNumContext(ctx, note.NoteId, note.UserId, &committedUSN, assetWork.OperationID)
	}
	assetWork.Verify = func(ctx context.Context) (bool, error) {
		var found info.Attach
		err := db.Attachs.FindContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": note.NoteId}).One(&found)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return this.verifyAttachNumAfterDelete(ctx, note.NoteId, note.UserId), nil
		}
		return false, err
	}
	result := noteService.SaveNote(applicationnotes.SaveNoteCommand{ActorUserID: userID, NoteID: note.NoteId.Hex(), OperationID: operationID, ExpectedUSN: &expectedUSN, AssetWork: assetWork, UpdatedTime: time.Now()})
	if result.OK() {
		return true, "delete file success"
	}
	if result.PartialWrite {
		return false, "partial_write"
	}
	return false, "db error"
}

func (this *AttachService) verifyAttachNumAfterDelete(ctx context.Context, noteID, ownerID domain.ObjectID) bool {
	count, err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).Count()
	if err != nil {
		return false
	}
	var note info.Note
	return db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "AttachNum": count}).One(&note) == nil
}

// 获取文件路径
// 要判断是否具有权限
// userId是否具有attach的访问权限
func (this *AttachService) GetAttach(attachId, userId string) (attach info.Attach) {
	if attachId == "" {
		return
	}

	attach = info.Attach{}
	db.Get(db.Attachs, attachId, &attach)
	path := attach.Path
	if path == "" {
		return
	}

	note := noteService.GetNoteById(attach.NoteId.Hex())

	// 判断权限

	// 笔记是否是公开的
	if note.IsBlog {
		return
	}

	// 笔记是否是我的
	if note.UserId.Hex() == userId {
		return
	}

	// 我是否有权限查看或协作
	if shareService.HasReadNotePerm(attach.NoteId.Hex(), userId) {
		return
	}

	attach = info.Attach{}
	return
}

// 复制笔记时需要复制附件
// noteService调用, 权限已判断
func (this *AttachService) CopyAttachs(noteId, toNoteId, toUserId string) bool {
	return this.CopyAttachsWithOperation(noteId, toNoteId, toUserId, "")
}

// CopyAttachsWithOperation copies into an already committed destination note.
// Stable calls reuse the destination attachment identity and remove a copied
// file when the attachment row cannot be committed.
func (this *AttachService) CopyAttachsWithOperation(noteId, toNoteId, toUserId, operationID string) bool {
	attachs := []info.Attach{}
	if err := db.Attachs.FindContext(context.Background(), bson.M{"NoteId": db.MustObjectIDFromHex(noteId)}).All(&attachs); err != nil {
		return false
	}
	return this.copyAttachSet(attachs, toNoteId, toUserId, operationID)
}

// CopyAttachsWithManifest copies only the source attachment identities frozen
// in the root shared-copy receipt. Re-enumerating the source note on retry
// would make a single client operation absorb attachments added later.
func (this *AttachService) CopyAttachsWithManifest(noteId, toNoteId, toUserId, operationID string, assets []applicationnotes.OperationAsset) bool {
	if operationID == "" || !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(toNoteId) || !db.IsValidObjectIDHex(toUserId) {
		return false
	}
	sourceNoteID := db.MustObjectIDFromHex(noteId)
	attachs := make([]info.Attach, 0)
	seen := make(map[string]bool)
	for _, asset := range assets {
		if !asset.IsAttach {
			continue
		}
		if !db.IsValidObjectIDHex(asset.LocalFileID) || seen[asset.LocalFileID] {
			return false
		}
		seen[asset.LocalFileID] = true
		if asset.AssetID != stableCopiedAttachID(operationID, asset.LocalFileID, toUserId).Hex() {
			return false
		}
		var attach info.Attach
		if err := db.Attachs.FindContext(context.Background(), bson.M{
			"_id": db.MustObjectIDFromHex(asset.LocalFileID), "NoteId": sourceNoteID,
		}).One(&attach); err != nil {
			return false
		}
		attachs = append(attachs, attach)
	}
	return this.copyAttachSet(attachs, toNoteId, toUserId, operationID)
}

func (this *AttachService) copyAttachSet(attachs []info.Attach, toNoteId, toUserId, operationID string) bool {
	toNoteIdO := db.MustObjectIDFromHex(toNoteId)
	for _, attach := range attachs {
		sourceAttachID := attach.AttachId.Hex()
		if operationID != "" {
			attach.AttachId = stableCopiedAttachID(operationID, sourceAttachID, toUserId)
			attach.UploadUserId = db.MustObjectIDFromHex(toUserId)
		} else {
			attach.AttachId = ObjectID{}
		}
		attach.NoteId = toNoteIdO
		attachOperationID := operationID
		if operationID != "" {
			attachOperationID += ":" + sourceAttachID
		}
		// 文件复制一份
		_, ext := SplitFilename(attach.Name)
		newFilename := NewGuid() + ext
		dir := "files/" + toUserId + "/attachs"
		if operationID != "" {
			dir = "files/" + toUserId + "/" + attach.AttachId.Hex() + "/attachs"
			newFilename = attach.AttachId.Hex() + ext
		}
		filePath := dir + "/" + newFilename
		data, err := os.ReadFile(filepath.Join(revel.BasePath, filepath.FromSlash(strings.TrimLeft(attach.Path, "/"))))
		if err != nil {
			return false
		}
		attach.Name = newFilename
		attach.Path = filePath
		attach.Size = int64(len(data))
		if operationID == "" {
			// Legacy shared-copy writes attachment files/rows before the target
			// note exists, then creates the note once with the source AttachNum.
			// Do not route this path through the Web attachment mutation, whose
			// permission/CAS boundary correctly requires an existing note and
			// would silently drop every legacy copied attachment here.
			target := filepath.Join(revel.BasePath, filepath.FromSlash(filePath))
			if err := publishFileNoClobber(target, data, 0777); err != nil {
				return false
			}
			attach.CreatedTime = time.Now()
			if err := db.Attachs.InsertContext(context.Background(), attach); err != nil {
				_ = os.Remove(target)
				return false
			}
			continue
		}

		if ok, _ := this.UploadWebAttach(attach, data, attachOperationID); !ok {
			return false
		}
	}

	return true
}

func stableCopiedAttachID(operationID, sourceAttachID, destinationOwnerID string) ObjectID {
	sum := sha256.Sum256([]byte("note-copy-attachment\x00" + operationID + "\x00" + sourceAttachID + "\x00" + destinationOwnerID))
	var raw [12]byte
	copy(raw[:], sum[:12])
	if raw == ([12]byte{}) {
		raw[11] = 1
	}
	return ObjectID(raw)
}

// 只留下files的数据, 其它的都删除
func (this *AttachService) UpdateOrDeleteAttachApi(noteId, userId string, files []info.NoteFile) bool {
	return this.UpdateOrDeleteAttachApiResult(context.Background(), noteId, userId, files) == nil
}

// UpdateOrDeleteAttachApiResult is the error-preserving reconcile boundary.
// Callers must invoke it after the note's ExpectedUSN CAS has succeeded.
func (this *AttachService) UpdateOrDeleteAttachApiResult(ctx context.Context, noteId, userId string, files []info.NoteFile) error {
	return this.updateOrDeleteAttachApiResult(ctx, noteId, userId, files, nil, "")
}

// UpdateOrDeleteAttachApiResultAtUSN reconciles API attachments only while the
// note is still at the generation that committed the enclosing mutation.
func (this *AttachService) UpdateOrDeleteAttachApiResultAtUSN(ctx context.Context, noteId, userId string, files []info.NoteFile, expectedUSN int) error {
	return this.UpdateOrDeleteAttachApiResultAtUSNWithOperation(ctx, noteId, userId, files, expectedUSN, "")
}

// UpdateOrDeleteAttachApiResultAtUSNWithOperation is the fenced form used by
// SaveNote's durable asset repair.  The operation's note lease is allowed to
// remain on the note while AttachNum is reconciled.
func (this *AttachService) UpdateOrDeleteAttachApiResultAtUSNWithOperation(ctx context.Context, noteId, userId string, files []info.NoteFile, expectedUSN int, operationID string) error {
	if expectedUSN <= 0 {
		return fmt.Errorf("reconcile attachments: invalid expected USN")
	}
	return this.updateOrDeleteAttachApiResult(ctx, noteId, userId, files, &expectedUSN, operationID)
}

// VerifyUpdateOrDeleteAttachApiAtUSN proves the durable state of an API asset
// reconciliation.  Checking only the note generation is insufficient: the
// upload and attachment writes are separate boundaries and an upload can have
// returned an error after creating a durable row.  This verifier is used for
// unknown-result recovery, so it must require the complete requested set.
func (this *AttachService) VerifyUpdateOrDeleteAttachApiAtUSN(ctx context.Context, noteId, userId string, files []info.NoteFile, expectedUSN int) (bool, error) {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return false, fmt.Errorf("verify attachments: invalid identity")
	}
	noteID := db.MustObjectIDFromHex(noteId)
	ownerID := db.MustObjectIDFromHex(userId)
	var note info.Note
	noteFilter := bson.M{"_id": noteID, "UserId": ownerID, "IsDeleted": false}
	if expectedUSN > 0 {
		noteFilter["Usn"] = expectedUSN
	}
	if err := db.Notes.FindContext(ctx, noteFilter).One(&note); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	wantAttach := make(map[string]struct{})
	wantImage := make(map[string]struct{})
	for _, file := range files {
		if file.FileId == "" {
			continue
		}
		if !db.IsValidObjectIDHex(file.FileId) {
			return false, nil
		}
		if file.IsAttach {
			wantAttach[file.FileId] = struct{}{}
		} else {
			wantImage[file.FileId] = struct{}{}
		}
	}
	var attachs []info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID, "UploadUserId": ownerID}).All(&attachs); err != nil {
		return false, err
	}
	if len(attachs) != len(wantAttach) || note.AttachNum != len(wantAttach) {
		return false, nil
	}
	for _, attach := range attachs {
		if _, ok := wantAttach[attach.AttachId.Hex()]; !ok {
			return false, nil
		}
		if strings.TrimSpace(attach.Path) == "" {
			return false, nil
		}
		if _, err := os.Stat(revel.BasePath + "/" + strings.TrimLeft(attach.Path, "/")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		} else if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
	}
	if len(wantImage) == 0 {
		return true, nil
	}
	var images []info.File
	if err := db.Files.FindContext(ctx, bson.M{"_id": bson.M{"$in": objectIDs(wantImage)}, "UserId": ownerID}).All(&images); err != nil {
		return false, err
	}
	if len(images) != len(wantImage) {
		return false, nil
	}
	for _, image := range images {
		if strings.TrimSpace(image.Path) == "" {
			return false, nil
		}
		if _, err := os.Stat(revel.BasePath + "/" + strings.TrimLeft(image.Path, "/")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		} else if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
	}
	return true, nil
}

func objectIDs(ids map[string]struct{}) []domain.ObjectID {
	result := make([]domain.ObjectID, 0, len(ids))
	for id := range ids {
		if db.IsValidObjectIDHex(id) {
			result = append(result, db.MustObjectIDFromHex(id))
		}
	}
	return result
}

func (this *AttachService) updateOrDeleteAttachApiResult(ctx context.Context, noteId, userId string, files []info.NoteFile, expectedUSN *int, operationID string) error {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return fmt.Errorf("reconcile attachments: invalid identity")
	}
	var attachs []info.Attach
	noteID := db.MustObjectIDFromHex(noteId)
	ownerID := db.MustObjectIDFromHex(userId)
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID, "UploadUserId": ownerID}).All(&attachs); err != nil {
		return err
	}
	nowAttachs := map[string]bool{}
	for _, file := range files {
		if file.IsAttach && file.FileId != "" {
			nowAttachs[file.FileId] = true
		}
	}
	for _, attach := range attachs {
		if nowAttachs[attach.AttachId.Hex()] {
			continue
		}
		path := strings.TrimLeft(attach.Path, "/")
		if err := os.Remove(revel.BasePath + "/" + path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// Remove the durable row only after the file is gone. If the database
		// write fails, the row remains available for a retry to finish cleanup.
		if err := db.Attachs.RemoveContext(ctx, bson.M{"_id": attach.AttachId, "NoteId": noteID, "UploadUserId": ownerID}); err != nil {
			return err
		}
	}
	return this.updateNoteAttachNumContext(ctx, noteID, ownerID, expectedUSN, operationID)
}

func (this *AttachService) updateNoteAttachNumContext(ctx context.Context, noteID, ownerID domain.ObjectID, expectedUSN *int, operationID string) error {
	count, err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).Count()
	if err != nil {
		return err
	}
	filter := attachNumUpdateFilterWithOperation(noteID, ownerID, expectedUSN, operationID)
	return db.Notes.UpdateOneMatchedContext(ctx,
		filter,
		bson.M{"$set": bson.M{"AttachNum": count}},
	)
}

func attachNumUpdateFilter(noteID, ownerID domain.ObjectID, expectedUSN *int) bson.M {
	return attachNumUpdateFilterWithOperation(noteID, ownerID, expectedUSN, "")
}

func attachNumUpdateFilterWithOperation(noteID, ownerID domain.ObjectID, expectedUSN *int, operationID string) bson.M {
	filter := bson.M{"_id": noteID, "UserId": ownerID}
	if expectedUSN != nil {
		filter["Usn"] = *expectedUSN
		filter["IsDeleted"] = false
		db.AddWorkspaceNoteMutationLeaseFilter(filter, operationID, time.Now())
	}
	return filter
}
