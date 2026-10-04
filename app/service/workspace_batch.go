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
)

func workspaceBatchItemOperationID(batchOperationID, action, resourceID string, index int) string {
	if batchOperationID == "" || action == "" || resourceID == "" || index < 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("workspace-batch-item\x00%s\x00%s\x00%s\x00%d", batchOperationID, action, resourceID, index)))
	return "batch-item:" + hex.EncodeToString(sum[:])[:32]
}

// workspaceBatchOperationIdentity is the outer receipt identity.  The raw
// client operation ID is scoped by actor and action, while the receipt digest
// binds every ordered input that can change the set of child mutations.
func workspaceBatchOperationIdentity(actorUserID, action, clientOperationID string, noteIDs []string, notebookID, sharedOwnerID string, shared bool) (string, string, []byte, error) {
	owner, err := domain.ParseObjectID(actorUserID)
	if err != nil {
		return "", "", nil, err
	}
	operationID, err := applicationnotes.NewClientOperationIdentity("note_batch_"+action, owner, clientOperationID)
	if err != nil {
		return "", "", nil, err
	}
	input := struct {
		Action        string
		NoteIDs       []string
		NotebookID    string
		SharedOwnerID string
		Shared        bool
	}{Action: action, NoteIDs: append([]string(nil), noteIDs...), NotebookID: notebookID, SharedOwnerID: sharedOwnerID, Shared: shared}
	_, digest, desired, err := applicationnotes.NewOperationIdentity("note_batch_input", owner, owner, input)
	return operationID, digest, desired, err
}

type frozenWorkspaceBatchResult struct {
	Command WorkspaceCommandResult `json:"command"`
	// Notes is the exact metadata-only response returned by the original copy.
	// info.Note deliberately excludes note content; freezing the response keeps
	// a terminal retry independent from later edits to the destination notes.
	Notes []info.Note `json:"notes,omitempty"`
}

func freezeWorkspaceBatchResult(result frozenWorkspaceBatchResult) ([]byte, error) {
	return json.Marshal(result)
}

func restoreWorkspaceBatchResult(payload []byte) (frozenWorkspaceBatchResult, error) {
	var result frozenWorkspaceBatchResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return result, fmt.Errorf("restore workspace batch result: %w", err)
	}
	return result, nil
}

func runWorkspaceBatchReceipt(actorUserID, action, clientOperationID string, noteIDs []string, notebookID, sharedOwnerID string, shared bool, frozen *frozenWorkspaceBatchResult, execute func(string) error) error {
	if clientOperationID == "" {
		return execute("")
	}
	operationID, digest, desired, err := workspaceBatchOperationIdentity(actorUserID, action, clientOperationID, noteIDs, notebookID, sharedOwnerID, shared)
	if err != nil {
		return err
	}
	executed := false
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: db.MustObjectIDFromHex(actorUserID), ResourceID: db.MustObjectIDFromHex(actorUserID),
		Kind: "note_batch_" + action, InputDigest: digest, DesiredState: desired, FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{{Name: "items", ReplaySafe: true, Apply: func(ctx context.Context) error {
			executed = true
			return execute(operationID)
		}}},
		CaptureResultState: func() []byte {
			payload, captureErr := freezeWorkspaceBatchResult(*frozen)
			if captureErr != nil {
				return nil
			}
			return payload
		},
		RestoreResultState: func(payload []byte) error {
			restored, restoreErr := restoreWorkspaceBatchResult(payload)
			if restoreErr == nil {
				*frozen = restored
			}
			return restoreErr
		},
	}
	result, runErr := db.RunWorkspaceRepair(context.Background(), plan)
	if result.Committed && !executed && len(result.Operation.ResultState) == 0 {
		return fmt.Errorf("workspace batch committed without frozen result")
	}
	return runErr
}

func applyWorkspaceBatchRunError(result *WorkspaceCommandResult, runErr error) {
	if runErr == nil || result.Error != "" {
		return
	}
	if errors.Is(runErr, applicationnotes.ErrOperationConflict) {
		result.Error = WorkspaceConflict
		result.Committed = false
		return
	}
	result.Error = WorkspacePartialWrite
	result.PartialWrite = true
	result.Committed = false
}

type NoteBatchResult struct {
	WorkspaceCommandResult
	Notes []info.Note
}

func (this *NoteService) DeleteNotes(actorUserID string, noteIDs []string, shared bool) WorkspaceCommandResult {
	return this.DeleteNotesWithOperation(actorUserID, noteIDs, shared, "")
}

func (this *NoteService) DeleteNotesWithOperation(actorUserID string, noteIDs []string, shared bool, batchOperationID string) WorkspaceCommandResult {
	if batchOperationID != "" {
		var result WorkspaceCommandResult
		frozen := frozenWorkspaceBatchResult{}
		runErr := runWorkspaceBatchReceipt(actorUserID, "delete", batchOperationID, noteIDs, "", "", shared, &frozen, func(frozenBatchID string) error {
			result = this.deleteNotesWithOperation(actorUserID, noteIDs, shared, frozenBatchID)
			frozen.Command = result
			if result.OK() {
				return nil
			}
			return fmt.Errorf("note batch delete failed")
		})
		if frozen.Command.RetrySafe {
			result = frozen.Command
		}
		applyWorkspaceBatchRunError(&result, runErr)
		return result
	}
	return this.deleteNotesWithOperation(actorUserID, noteIDs, shared, "")
}

func (this *NoteService) deleteNotesWithOperation(actorUserID string, noteIDs []string, shared bool, batchOperationID string) WorkspaceCommandResult {
	result := WorkspaceCommandResult{Committed: true, Items: make([]WorkspaceItemResult, 0, len(noteIDs))}
	result.RetrySafe = batchOperationID != ""
	if !db.IsValidObjectIDHex(actorUserID) {
		result.Committed = false
		result.Error = WorkspaceValidation
		return result
	}
	for index, noteID := range noteIDs {
		if !db.IsValidObjectIDHex(noteID) {
			result.Items = append(result.Items, WorkspaceItemResult{ResourceID: noteID, Error: WorkspaceValidation})
			result.Committed = false
			result.Error = WorkspaceValidation
			continue
		}
		ok := false
		itemOperationID := workspaceBatchItemOperationID(batchOperationID, "delete", noteID, index)
		if shared {
			ok = trashService.DeleteSharedNoteWithOperation(noteID, actorUserID, itemOperationID)
		} else {
			ok = trashService.DeleteNoteWithOperation(noteID, actorUserID, itemOperationID)
		}
		item := WorkspaceItemResult{ResourceID: noteID, Committed: ok}
		if !ok {
			item.Error = WorkspaceStorage
			result.Committed = false
			result.Error = WorkspaceStorage
		}
		result.Items = append(result.Items, item)
	}
	finalizeWorkspaceBatch(&result)
	return result
}

func (this *NoteService) MoveNotes(actorUserID string, noteIDs []string, notebookID string) WorkspaceCommandResult {
	return this.MoveNotesWithOperation(actorUserID, noteIDs, notebookID, "")
}

func (this *NoteService) MoveNotesWithOperation(actorUserID string, noteIDs []string, notebookID, batchOperationID string) WorkspaceCommandResult {
	if batchOperationID != "" {
		var result WorkspaceCommandResult
		frozen := frozenWorkspaceBatchResult{}
		runErr := runWorkspaceBatchReceipt(actorUserID, "move", batchOperationID, noteIDs, notebookID, "", false, &frozen, func(frozenBatchID string) error {
			result = this.moveNotesWithOperation(actorUserID, noteIDs, notebookID, frozenBatchID)
			frozen.Command = result
			if result.OK() {
				return nil
			}
			return fmt.Errorf("note batch move failed")
		})
		if frozen.Command.RetrySafe {
			result = frozen.Command
		}
		applyWorkspaceBatchRunError(&result, runErr)
		return result
	}
	return this.moveNotesWithOperation(actorUserID, noteIDs, notebookID, "")
}

func (this *NoteService) moveNotesWithOperation(actorUserID string, noteIDs []string, notebookID, batchOperationID string) WorkspaceCommandResult {
	result := WorkspaceCommandResult{Committed: true, Items: make([]WorkspaceItemResult, 0, len(noteIDs))}
	result.RetrySafe = batchOperationID != ""
	if !db.IsValidObjectIDHex(actorUserID) || !db.IsValidObjectIDHex(notebookID) {
		result.Committed = false
		result.Error = WorkspaceValidation
		return result
	}
	for index, noteID := range noteIDs {
		if !db.IsValidObjectIDHex(noteID) {
			result.Items = append(result.Items, WorkspaceItemResult{ResourceID: noteID, Error: WorkspaceValidation})
			result.Committed = false
			result.Error = WorkspaceValidation
			continue
		}
		itemOperationID := workspaceBatchItemOperationID(batchOperationID, "move", noteID, index)
		note := this.MoveNoteWithOperation(noteID, notebookID, actorUserID, itemOperationID)
		item := WorkspaceItemResult{ResourceID: noteID, Committed: !note.NoteId.IsZero(), USN: note.Usn}
		if !item.Committed {
			item.Error = WorkspaceStorage
			result.Committed = false
			result.Error = WorkspaceStorage
		}
		result.Items = append(result.Items, item)
	}
	finalizeWorkspaceBatch(&result)
	return result
}

func (this *NoteService) CopyNotes(actorUserID string, noteIDs []string, notebookID string) NoteBatchResult {
	return this.CopyNotesWithOperation(actorUserID, noteIDs, notebookID, "")
}

func (this *NoteService) CopyNotesWithOperation(actorUserID string, noteIDs []string, notebookID, batchOperationID string) NoteBatchResult {
	return this.copyNotes(actorUserID, noteIDs, notebookID, "", batchOperationID)
}

func (this *NoteService) CopySharedNotes(actorUserID string, noteIDs []string, notebookID, ownerUserID string) NoteBatchResult {
	return this.CopySharedNotesWithOperation(actorUserID, noteIDs, notebookID, ownerUserID, "")
}

func (this *NoteService) CopySharedNotesWithOperation(actorUserID string, noteIDs []string, notebookID, ownerUserID, batchOperationID string) NoteBatchResult {
	return this.copyNotes(actorUserID, noteIDs, notebookID, ownerUserID, batchOperationID)
}

func (this *NoteService) copyNotes(actorUserID string, noteIDs []string, notebookID, sharedOwnerID, batchOperationID string) NoteBatchResult {
	if batchOperationID == "" {
		return this.copyNotesItems(actorUserID, noteIDs, notebookID, sharedOwnerID, "")
	}
	var result NoteBatchResult
	frozen := frozenWorkspaceBatchResult{}
	shared := sharedOwnerID != ""
	runErr := runWorkspaceBatchReceipt(actorUserID, "copy", batchOperationID, noteIDs, notebookID, sharedOwnerID, shared, &frozen, func(frozenBatchID string) error {
		result = this.copyNotesItems(actorUserID, noteIDs, notebookID, sharedOwnerID, frozenBatchID)
		frozen.Command = result.WorkspaceCommandResult
		frozen.Notes = append(frozen.Notes[:0], result.Notes...)
		if result.OK() {
			return nil
		}
		return fmt.Errorf("note batch copy failed")
	})
	if frozen.Command.RetrySafe {
		result.WorkspaceCommandResult = frozen.Command
		result.Notes = append([]info.Note(nil), frozen.Notes...)
	}
	applyWorkspaceBatchRunError(&result.WorkspaceCommandResult, runErr)
	return result
}

func (this *NoteService) copyNotesItems(actorUserID string, noteIDs []string, notebookID, sharedOwnerID, batchOperationID string) NoteBatchResult {
	result := NoteBatchResult{
		WorkspaceCommandResult: WorkspaceCommandResult{Committed: true, Items: make([]WorkspaceItemResult, 0, len(noteIDs))},
		Notes:                  make([]info.Note, 0, len(noteIDs)),
	}
	result.RetrySafe = batchOperationID != ""
	if !db.IsValidObjectIDHex(actorUserID) || !db.IsValidObjectIDHex(notebookID) || (sharedOwnerID != "" && !db.IsValidObjectIDHex(sharedOwnerID)) {
		result.Committed = false
		result.Error = WorkspaceValidation
		return result
	}
	for index, noteID := range noteIDs {
		if !db.IsValidObjectIDHex(noteID) {
			result.Items = append(result.Items, WorkspaceItemResult{ResourceID: noteID, Error: WorkspaceValidation})
			result.Committed = false
			result.Error = WorkspaceValidation
			continue
		}
		var note info.Note
		itemOperationID := workspaceBatchItemOperationID(batchOperationID, "copy", noteID, index)
		if sharedOwnerID == "" {
			note = this.CopyNoteWithOperation(noteID, notebookID, actorUserID, itemOperationID)
		} else {
			note = this.CopySharedNoteWithOperation(noteID, notebookID, sharedOwnerID, actorUserID, itemOperationID)
		}
		item := WorkspaceItemResult{ResourceID: noteID, Committed: !note.NoteId.IsZero(), USN: note.Usn}
		if !item.Committed {
			item.Error = WorkspaceStorage
			result.Committed = false
			result.Error = WorkspaceStorage
		} else {
			result.Notes = append(result.Notes, note)
		}
		result.Items = append(result.Items, item)
	}
	finalizeWorkspaceBatch(&result.WorkspaceCommandResult)
	return result
}

func finalizeWorkspaceBatch(result *WorkspaceCommandResult) {
	if result.Committed {
		return
	}
	for _, item := range result.Items {
		if item.Committed {
			result.PartialWrite = true
			result.Error = WorkspacePartialWrite
			return
		}
	}
}
