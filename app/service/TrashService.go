package service

import (
	"context"
	"errors"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// 回收站
// 可以移到noteSerice中

// 不能删除notebook, 如果其下有notes!
// 这样回收站里只有note

// 删除笔记后(或删除笔记本)后入回收站
// 把note, notebook设个标记即可!
// 已经在trash里的notebook, note不能是共享!, 所以要删除共享

type TrashService struct {
}

func findOwnedNoteForMutation(ctx context.Context, noteId, userId string) (info.Note, error) {
	if db.Notes == nil {
		return info.Note{}, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, db.GetIdAndUserIdQ(noteId, userId)).One(&note)
	return note, err
}

func findActiveNoteByIDForMutation(ctx context.Context, noteId string) (info.Note, error) {
	if db.Notes == nil {
		return info.Note{}, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteId), "IsDeleted": false}).One(&note)
	return note, err
}

//---------------------

// 删除note
// 应该放在回收站里
// 有trashService
func (this *TrashService) DeleteNote(noteId, userId string) bool {
	return this.DeleteNoteWithOperation(noteId, userId, "")
}

func (this *TrashService) DeleteNoteWithOperation(noteId, userId, operationID string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	note, err := findOwnedNoteForMutation(context.Background(), noteId, userId)
	if err != nil {
		return false
	}
	// 如果是垃圾, 则彻底删除
	if note.IsTrash {
		if operationID != "" {
			// SaveNote scopes the client generation to the note_save action.  A
			// retry observes IsTrash=true, so looking up the raw client value
			// here would miss the committed receipt and accidentally turn a
			// lost-response retry into permanent deletion.
			normalizedID, err := applicationnotes.NewClientOperationIdentity("note_save", note.UserId, operationID)
			if err != nil {
				return false
			}
			receipt, err := db.GetWorkspaceOperation(context.Background(), note.UserId, normalizedID)
			replay, proceed := trashedNoteDeleteDecision(receipt, err, note.NoteId)
			if replay {
				return true
			}
			if !proceed {
				return false
			}
		}
		return this.DeleteTrashWithOperation(noteId, userId, operationID)
	}
	if note.NoteId.IsZero() || note.IsDeleted {
		return false
	}

	expected := note.Usn
	result := noteService.SaveNote(applicationnotes.SaveNoteCommand{
		ActorUserID: userId, NoteID: noteId, OperationID: operationID, ExpectedUSN: &expected,
		Metadata: map[string]any{"IsTrash": true},
	})
	return result.OK()
}

func trashedNoteDeleteDecision(receipt applicationnotes.OperationReceipt, err error, resourceID domain.ObjectID) (replay, proceed bool) {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, true
	}
	if err != nil {
		return false, false
	}
	if receipt.ResourceID == resourceID && receipt.Status == applicationnotes.OperationCommitted {
		return true, false
	}
	return false, false
}

// 删除别人共享给我的笔记
// 先判断我是否有权限, 笔记是否是我创建的
func (this *TrashService) DeleteSharedNote(noteId, myUserId string) bool {
	return this.DeleteSharedNoteWithOperation(noteId, myUserId, "")
}

func (this *TrashService) DeleteSharedNoteWithOperation(noteId, myUserId, operationID string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(myUserId) {
		return false
	}
	note, err := findActiveNoteByIDForMutation(context.Background(), noteId)
	if err != nil {
		return false
	}
	userId := note.UserId.Hex()
	if shareService.HasUpdatePerm(userId, myUserId, noteId) && note.CreatedUserId.Hex() == myUserId {
		if operationID != "" {
			normalizedID, err := applicationnotes.NewClientOperationIdentity("note_save", note.UserId, operationID)
			if err != nil {
				return false
			}
			if receipt, err := db.GetWorkspaceOperation(context.Background(), note.UserId, normalizedID); err == nil {
				return receipt.ResourceID == note.NoteId && receipt.Status == applicationnotes.OperationCommitted
			} else if !errors.Is(err, mongo.ErrNoDocuments) && !errors.Is(err, db.ErrMongoClientNotInitialized) {
				return false
			}
		}
		expected := note.Usn
		result := noteService.SaveNote(applicationnotes.SaveNoteCommand{
			ActorUserID: myUserId, NoteID: noteId, OperationID: operationID, ExpectedUSN: &expected,
			Metadata: map[string]any{"IsTrash": true},
		})
		return result.OK()
	}
	return false
}

// 删除trash
func (this *TrashService) DeleteTrash(noteId, userId string) bool {
	return this.DeleteTrashWithOperation(noteId, userId, "")
}

func (this *TrashService) DeleteTrashWithOperation(noteId, userId, operationID string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	note, err := findOwnedNoteForMutation(context.Background(), noteId, userId)
	if err != nil {
		return false
	}
	if note.NoteId.IsZero() {
		return false
	}
	ok, _, _ := this.deleteTrashDurable(note, userId, nil, operationID)
	return ok
}

func (this *TrashService) DeleteTrashApi(noteId, userId string, usn int) (bool, string, int) {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) {
		return false, "notExists", 0
	}
	note, err := findOwnedNoteForMutation(context.Background(), noteId, userId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, "notExists", 0
		}
		return false, "storage", 0
	}

	if note.NoteId.IsZero() {
		return false, "notExists", 0
	}
	if !note.IsDeleted && note.Usn != usn {
		return false, "conflict", 0
	}
	if note.IsDeleted {
		return this.deleteTrashDurable(note, userId, nil, "")
	}

	expected := usn
	return this.deleteTrashDurable(note, userId, &expected, "")
}

func (this *TrashService) deleteTrashDurable(note info.Note, userId string, expected *int, clientOperationID string) (bool, string, int) {
	afterUSN := note.Usn
	if !note.IsDeleted {
		current := note.Usn
		if expected != nil {
			current = *expected
		}
		result := noteService.SaveNote(applicationnotes.SaveNoteCommand{
			ActorUserID: userId, NoteID: note.NoteId.Hex(), ExpectedUSN: &current,
			Metadata: map[string]any{"IsDeleted": true},
		})
		if !result.Committed {
			if result.Error == WorkspaceConflict {
				return false, "conflict", 0
			}
			return false, string(result.Error), result.USN
		}
		afterUSN = result.USN
	}
	operationID, digest, desired, err := applicationnotes.NewOperationIdentity("note_delete_cleanup", note.UserId, note.NoteId, struct {
		TombstoneUSN      int
		ClientOperationID string
	}{TombstoneUSN: afterUSN, ClientOperationID: clientOperationID})
	if err != nil {
		return false, "storage", afterUSN
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: note.UserId, ResourceID: note.NoteId,
		Kind: "note_delete_cleanup", InputDigest: digest, DesiredState: desired,
		FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{
			{Name: "attachments", ReplaySafe: true,
				Apply: func(ctx context.Context) error { return attachService.deleteAllAttachs(ctx, note.NoteId, note.UserId) },
				Verify: func(ctx context.Context) (bool, error) {
					return attachService.verifyAllAttachsDeleted(ctx, note.NoteId, note.UserId)
				},
			},
			{Name: "image_index", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					_, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"NoteId": note.NoteId})
					return err
				},
				Verify: func(ctx context.Context) (bool, error) {
					var image info.NoteImage
					err := db.NoteImages.FindContext(ctx, bson.M{"NoteId": note.NoteId}).One(&image)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return true, nil
					}
					return false, err
				},
			},
			{Name: "content", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return db.NoteContents.RemoveContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId})
				},
				Verify: func(ctx context.Context) (bool, error) {
					var content info.NoteContent
					err := db.NoteContents.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId}).One(&content)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return true, nil
					}
					return false, err
				},
			},
			{Name: "history", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return db.NoteContentHistories.RemoveContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId})
				},
				Verify: func(ctx context.Context) (bool, error) {
					var history info.NoteContentHistory
					err := db.NoteContentHistories.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId}).One(&history)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return true, nil
					}
					return false, err
				},
			},
			{Name: "notebook_count", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return notebookService.reCountNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return notebookService.verifyNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
			},
			{Name: "tag_counts", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					for _, tag := range note.Tags {
						if tag != "" {
							if err := tagService.reCountExistingTag(ctx, note.UserId, tag); err != nil {
								return err
							}
						}
					}
					return nil
				},
				Verify: func(ctx context.Context) (bool, error) {
					return tagService.verifyTagCounts(ctx, note.UserId, note.Tags)
				},
			},
		},
	}
	if err := db.QuarantineWorkspaceOperationsForResource(context.Background(), note.UserId, note.NoteId, operationID); err != nil {
		return false, "partial_write", afterUSN
	}
	cleanup, cleanupErr := db.RunWorkspaceRepair(context.Background(), plan)
	if cleanupErr != nil || !cleanup.Committed {
		return false, "partial_write", afterUSN
	}
	if err := db.DeleteWorkspaceOperationsForResource(context.Background(), note.UserId, note.NoteId); err != nil {
		return false, "partial_write", afterUSN
	}
	return true, "", afterUSN
}

// 列出note, 排序规则, 还有分页
// CreatedTime, UpdatedTime, title 来排序
func (this *TrashService) ListNotes(userId string,
	pageNumber, pageSize int, sortField string, isAsc bool) (notes []info.Note) {
	_, notes = noteService.ListNotes(userId, "", true, pageNumber, pageSize, sortField, isAsc, false)
	return
}
