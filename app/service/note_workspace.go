package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SaveNoteCommand = applicationnotes.SaveNoteCommand

// BeginNoteCreateAssetReceipt records the API create identity before upload
// side effects. Controllers bind bytes and metadata, but the notes service
// remains the sole owner of durable receipt transitions.
func (this *NoteService) BeginNoteCreateAssetReceipt(ownerID, noteID domain.ObjectID, operationID, inputDigest string, assets []applicationnotes.OperationAsset) WorkspaceErrorCategory {
	_, err := db.BeginWorkspaceOperation(context.Background(), applicationnotes.OperationReceipt{
		OperationID: operationID, OwnerID: ownerID, ResourceID: noteID,
		Kind: "note_create", InputDigest: inputDigest, Assets: append([]applicationnotes.OperationAsset(nil), assets...),
		Status: applicationnotes.OperationPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if err == nil {
		return ""
	}
	if errors.Is(err, applicationnotes.ErrOperationConflict) {
		return WorkspaceConflict
	}
	return WorkspaceStorage
}

func (this *NoteService) FailNoteCreateAssetReceipt(ownerID domain.ObjectID, operationID string) WorkspaceErrorCategory {
	if err := db.FailWorkspaceOperation(context.Background(), ownerID, operationID, "asset_upload"); err != nil {
		return WorkspaceStorage
	}
	return ""
}

// SaveNote applies metadata, content, history and one user USN as a single
// workspace mutation. Projections run only after the required commit.
func (this *NoteService) SaveNote(command SaveNoteCommand) WorkspaceCommandResult {
	result := WorkspaceCommandResult{RetrySafe: command.OperationID != ""}
	if !db.IsValidObjectIDHex(command.ActorUserID) || !db.IsValidObjectIDHex(command.NoteID) {
		result.Error = WorkspaceValidation
		return result
	}
	if command.AssetWork != nil && (command.AssetWork.Apply == nil || command.AssetWork.Verify == nil) {
		// Validate the side-effect contract before the required note mutation;
		// an invalid projection command must not consume a USN or commit a note.
		// A generation check alone cannot prove that an asset reconcile finished.
		result.Error = WorkspaceValidation
		return result
	}

	note, lookupErr := findNoteForWorkspaceSave(context.Background(), command.NoteID, command.ActorUserID)
	if lookupErr != nil && !errors.Is(lookupErr, mongo.ErrNoDocuments) {
		result.Error = WorkspaceStorage
		return result
	}
	if note.NoteId.IsZero() || note.IsDeleted {
		result.Error = WorkspaceNotFound
		return result
	}
	observedNote := note
	ownerID := note.UserId.Hex()
	if ownerID != command.ActorUserID && !shareService.HasUpdatePerm(ownerID, command.ActorUserID, command.NoteID) {
		result.Error = WorkspaceUnauthorized
		return result
	}
	operationID, inputDigest, identityErr := applicationnotes.NewSaveOperationIdentity(note.UserId, note.NoteId, command, note.Usn)
	if command.OperationID != "" {
		operationID, identityErr = applicationnotes.NewClientOperationIdentity("note_save", note.UserId, command.OperationID)
		if identityErr == nil {
			inputDigest, identityErr = applicationnotes.NewSaveOperationDigest(note.UserId, note.NoteId, command)
		}
	}
	if identityErr != nil {
		result.Error = WorkspaceValidation
		return result
	}
	type noteSaveBeforeState struct {
		Note    info.Note
		Content info.NoteContent
		History info.NoteContentHistory
	}
	var recoveredBefore *noteSaveBeforeState
	operationAlreadyCommitted := false
	committedUSN := 0
	receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), note.UserId, operationID)
	if receiptErr == nil {
		if receipt.InputDigest != inputDigest {
			result.Error = WorkspaceConflict
			return result
		}
		switch receipt.Status {
		case applicationnotes.OperationCommitted, applicationnotes.OperationCompensated, applicationnotes.OperationFailed:
			// Terminal receipts are redacted. The request already carries the
			// canonical desired values, and a committed receipt is replayed
			// without applying the required mutation again.
			operationAlreadyCommitted = receipt.Status == applicationnotes.OperationCommitted
			if operationAlreadyCommitted {
				committedUSN = receipt.AssignedUSN
			}
		default:
			var state noteSaveBeforeState
			if err := json.Unmarshal(receipt.BeforeState, &state); err != nil {
				result.Error = WorkspaceStorage
				return result
			}
			recoveredBefore = &state
			note = state.Note
		}
	} else if !errors.Is(receiptErr, mongo.ErrNoDocuments) && !errors.Is(receiptErr, db.ErrMongoClientNotInitialized) {
		result.Error = WorkspaceStorage
		return result
	}
	if !operationAlreadyCommitted && command.ExpectedUSN != nil && note.Usn != *command.ExpectedUSN {
		result.Error = WorkspaceConflict
		return result
	}

	beforeContent := info.NoteContent{}
	if recoveredBefore != nil {
		beforeContent = recoveredBefore.Content
	}
	contentChanged := false
	historyChanged := false
	requestedBlog, changesBlog := command.Metadata["IsBlog"].(bool)
	if command.Content != nil || changesBlog {
		if recoveredBefore == nil {
			beforeContent, lookupErr = findNoteContentForWorkspaceSave(context.Background(), note.NoteId, note.UserId)
			if lookupErr != nil && !errors.Is(lookupErr, mongo.ErrNoDocuments) {
				result.Error = WorkspaceStorage
				return result
			}
		}
		if beforeContent.NoteId.IsZero() {
			result.Error = WorkspaceNotFound
			return result
		}
		if command.Content != nil {
			abstract := beforeContent.Abstract
			if command.Abstract != nil {
				abstract = *command.Abstract
			}
			historyChanged = beforeContent.Content != *command.Content
			contentChanged = historyChanged || beforeContent.Abstract != abstract
		}
	}
	if len(command.Metadata) == 0 && !contentChanged && command.AssetWork == nil {
		if command.OperationID != "" {
			if operationAlreadyCommitted {
				result.Committed = true
				result.USN = committedUSN
				return result
			}
			noop := db.WorkspaceMutationPlan{
				OperationID: operationID,
				OwnerID:     note.UserId,
				ResourceID:  note.NoteId,
				Kind:        "note_save",
				InputDigest: inputDigest,
				AssignedUSN: func() int { return note.Usn },
				Steps: []db.WorkspaceMutationStep{{
					Name:       "noop",
					Apply:      func(context.Context) error { return nil },
					Verify:     func(context.Context) (bool, error) { return true, nil },
					ReplaySafe: true,
				}},
			}
			noopResult, err := db.RunWorkspaceRepair(context.Background(), noop)
			if err != nil || !noopResult.Committed {
				if errors.Is(err, applicationnotes.ErrOperationConflict) {
					result.Error = WorkspaceConflict
				} else {
					result.Error = WorkspaceStorage
				}
				return result
			}
			result.Committed = true
			result.USN = noopResult.Operation.AssignedUSN
			return result
		}
		result.Committed = true
		result.USN = note.Usn
		return result
	}
	if operationAlreadyCommitted && committedUSN > 0 && note.Usn != committedUSN {
		// The required mutation is already committed, but a later mutation has
		// advanced the resource generation. Do not replay an old projection
		// against the newer note. If this command had projections, an absent or
		// non-terminal projection receipt is still a partial write, not success.
		result.Committed = true
		result.USN = committedUSN
		if noteSaveNeedsProjection(command, note, contentChanged) {
			result.PartialWrite = true
			result.Error = WorkspacePartialWrite
			result.FailedStep = "projections"
			projection, projectionErr := db.GetWorkspaceOperation(context.Background(), note.UserId, operationID+":projections")
			if projectionErr == nil && projection.Status == applicationnotes.OperationCommitted {
				result.PartialWrite = false
				result.Error = ""
				result.FailedStep = ""
			} else if projectionErr != nil && !errors.Is(projectionErr, mongo.ErrNoDocuments) {
				result.Error = WorkspaceStorage
			}
		}
		return result
	}

	updatedTime := lea.FixUrlTime(command.UpdatedTime)
	metadata := cloneBSONMap(command.Metadata)
	if target, ok := metadata["NotebookId"].(domain.ObjectID); ok && !target.IsZero() && !notebookService.IsMyNotebook(target.Hex(), ownerID) {
		result.Error = WorkspaceValidation
		return result
	}

	// Serialize the complete note mutation, including its repairable
	// projections.  The lease is acquired against the observed generation
	// before USN allocation, so an attachment request cannot upload assets for
	// a request that lost the generation race.  Recovery may renew a lease
	// already stamped by the same operation after the note write advanced USN.
	leaseAcquired := false
	retainLease := false
	leaseExpectedUSN := note.Usn
	if operationAlreadyCommitted {
		leaseExpectedUSN = committedUSN
	}
	var leaseErr error
	if observedNote.MutationLeaseID == operationID {
		leaseErr = db.RenewWorkspaceNoteMutationLease(context.Background(), note.NoteId, note.UserId, operationID, time.Now())
	} else {
		leaseErr = db.AcquireWorkspaceNoteMutationLease(context.Background(), note.NoteId, note.UserId, leaseExpectedUSN, operationID, time.Now())
	}
	if leaseErr != nil {
		if errors.Is(leaseErr, db.ErrWorkspaceNoteLeaseUnavailable) || errors.Is(leaseErr, db.ErrDocumentNotFound) {
			result.Error = WorkspaceConflict
		} else {
			result.Error = WorkspaceStorage
		}
		return result
	}
	leaseAcquired = true
	if command.AssetWork != nil {
		command.AssetWork.OperationID = operationID
	}
	defer func() {
		if leaseAcquired && !retainLease {
			_ = db.ReleaseWorkspaceNoteMutationLease(context.Background(), note.NoteId, note.UserId, operationID)
		}
	}()
	metadata["UpdatedUserId"] = db.MustObjectIDFromHex(command.ActorUserID)
	metadata["UpdatedTime"] = updatedTime
	if note.IsBlog && note.HasSelfDefined {
		delete(metadata, "ImgSrc")
		delete(metadata, "Desc")
	}

	beforeHistory := info.NoteContentHistory{}
	historyExisted := false
	if historyChanged {
		if recoveredBefore != nil {
			beforeHistory = recoveredBefore.History
		} else {
			if db.NoteContentHistories == nil {
				result.Error = WorkspaceStorage
				return result
			}
			historyErr := db.NoteContentHistories.FindContext(context.Background(), bson.M{"_id": note.NoteId, "UserId": note.UserId}).One(&beforeHistory)
			if historyErr != nil && !errors.Is(historyErr, mongo.ErrNoDocuments) {
				result.Error = WorkspaceStorage
				return result
			}
		}
		historyExisted = !beforeHistory.NoteId.IsZero()
	}
	operationInput := struct {
		ActorID     string
		NoteID      string
		ExpectedUSN *int
		Metadata    bson.M
		Content     *string
		Abstract    *string
		UpdatedTime time.Time
	}{
		ActorID: command.ActorUserID, NoteID: command.NoteID, ExpectedUSN: command.ExpectedUSN,
		Metadata: metadata, Content: command.Content, Abstract: command.Abstract, UpdatedTime: updatedTime,
	}
	desiredState, desiredErr := applicationnotes.CanonicalState(operationInput)
	beforeState, beforeErr := applicationnotes.CanonicalState(noteSaveBeforeState{Note: note, Content: beforeContent, History: beforeHistory})
	if identityErr != nil || desiredErr != nil || beforeErr != nil {
		result.Error = WorkspaceValidation
		return result
	}

	var assignedUSN int
	var contentUpdate bson.M
	var nextHistory *info.NoteContentHistory
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: note.UserId, ResourceID: note.NoteId,
		Kind: "note_save", InputDigest: inputDigest, BeforeState: beforeState, DesiredState: desiredState,
		AssignedUSN: func() int { return assignedUSN },
		RestoreAssignedUSN: func(usn int) {
			assignedUSN = usn
			metadata["Usn"] = usn
		},
		RestoreDesiredState: func(payload []byte) error {
			state := struct {
				UpdatedTime time.Time
			}{}
			if err := json.Unmarshal(payload, &state); err != nil {
				return err
			}
			metadata["UpdatedTime"] = state.UpdatedTime
			updatedTime = state.UpdatedTime
			if contentUpdate != nil {
				contentUpdate["UpdatedTime"] = state.UpdatedTime
			}
			if nextHistory != nil && len(nextHistory.Histories) > 0 {
				nextHistory.Histories[0].UpdatedTime = state.UpdatedTime
			}
			return nil
		},
	}
	if command.AssetWork != nil {
		plan.Assets = append([]applicationnotes.OperationAsset(nil), command.AssetWork.Assets...)
	}
	plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
		Name: "allocate_usn",
		Apply: func(ctx context.Context) error {
			usn, err := db.AllocateUserUSN(ctx, note.UserId)
			if err == nil {
				assignedUSN = usn
				metadata["Usn"] = usn
			}
			return err
		},
		ReplaySafe: true,
	})
	plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
		Name: "note",
		Apply: func(ctx context.Context) error {
			filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "IsDeleted": false}
			filter["Usn"] = note.Usn
			db.AddWorkspaceNoteMutationLeaseFilter(filter, operationID, time.Now())
			return db.Notes.UpdateOneMatchedContext(ctx, filter, bson.M{"$set": metadata})
		},
		Verify: func(ctx context.Context) (bool, error) {
			filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "IsDeleted": false, "Usn": assignedUSN}
			db.AddWorkspaceNoteMutationLeaseFilter(filter, operationID, time.Now())
			for key, value := range metadata {
				filter[key] = value
			}
			var existing info.Note
			err := db.Notes.FindContext(ctx, filter).One(&existing)
			if errors.Is(err, mongo.ErrNoDocuments) {
				return false, nil
			}
			return err == nil, err
		},
		Compensate: func(ctx context.Context) error {
			filter := bson.M{"_id": note.NoteId, "UserId": note.UserId, "Usn": assignedUSN}
			filter["MutationLeaseId"] = operationID
			return db.Notes.UpdateOneMatchedContext(ctx, filter, note)
		},
	})
	if contentChanged {
		newAbstract := beforeContent.Abstract
		if command.Abstract != nil {
			newAbstract = *command.Abstract
		}
		contentUpdate = bson.M{
			"Content":       *command.Content,
			"Abstract":      newAbstract,
			"UpdatedTime":   updatedTime,
			"UpdatedUserId": db.MustObjectIDFromHex(command.ActorUserID),
		}
		if note.IsBlog && note.HasSelfDefined {
			delete(contentUpdate, "Abstract")
		}
		if changesBlog {
			contentUpdate["IsBlog"] = requestedBlog
		}
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "content",
			Apply: func(ctx context.Context) error {
				return db.NoteContents.UpdateOneMatchedContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId}, bson.M{"$set": contentUpdate})
			},
			Verify: func(ctx context.Context) (bool, error) {
				filter := bson.M{"_id": note.NoteId, "UserId": note.UserId}
				for key, value := range contentUpdate {
					filter[key] = value
				}
				var existing info.NoteContent
				err := db.NoteContents.FindContext(ctx, filter).One(&existing)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
			Compensate: func(ctx context.Context) error {
				filter := bson.M{"_id": note.NoteId, "UserId": note.UserId}
				for key, value := range contentUpdate {
					filter[key] = value
				}
				return db.NoteContents.UpdateOneMatchedContext(ctx, filter, beforeContent)
			},
		})

		if historyChanged {
			history := appendHistory(beforeHistory, info.EachHistory{
				UpdatedUserId: db.MustObjectIDFromHex(command.ActorUserID),
				UpdatedTime:   updatedTime,
				Content:       beforeContent.Content,
			})
			history.NoteId = note.NoteId
			history.UserId = note.UserId
			nextHistory = &history
			plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
				Name: "history",
				Apply: func(ctx context.Context) error {
					_, err := db.NoteContentHistories.UpsertContext(ctx,
						bson.M{"_id": note.NoteId, "UserId": note.UserId},
						history,
					)
					return err
				},
				Verify: func(ctx context.Context) (bool, error) {
					var existing info.NoteContentHistory
					err := db.NoteContentHistories.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "Histories": history.Histories}).One(&existing)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil, err
				},
				Compensate: func(ctx context.Context) error {
					if !historyExisted {
						return db.NoteContentHistories.RemoveContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "Histories": history.Histories})
					}
					return db.NoteContentHistories.UpdateOneMatchedContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "Histories": history.Histories}, beforeHistory)
				},
			})
		}
	} else if changesBlog {
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{
			Name: "content_blog",
			Apply: func(ctx context.Context) error {
				return db.NoteContents.UpdateOneMatchedContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId}, bson.M{"$set": bson.M{"IsBlog": requestedBlog}})
			},
			Verify: func(ctx context.Context) (bool, error) {
				var existing info.NoteContent
				err := db.NoteContents.FindContext(ctx, bson.M{"_id": note.NoteId, "UserId": note.UserId, "IsBlog": requestedBlog}).One(&existing)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
			Compensate: func(ctx context.Context) error {
				return db.NoteContents.UpdateOneMatchedContext(ctx,
					bson.M{"_id": note.NoteId, "UserId": note.UserId, "IsBlog": requestedBlog},
					bson.M{"$set": bson.M{"IsBlog": beforeContent.IsBlog}},
				)
			},
		})
	}

	if operationAlreadyCommitted {
		// A committed required operation is immutable. Only the repair receipt
		// below may be resumed; running this plan again would allocate another
		// USN for an otherwise idempotent retry.
		assignedUSN = committedUSN
		result.Committed = true
		result.USN = assignedUSN
	} else {
		mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
		result.AppliedSteps = mutation.AppliedSteps
		result.FailedStep = mutation.FailedStep
		result.PartialWrite = mutation.PartialWrite
		if err != nil || !mutation.Committed {
			if mutation.PartialWrite {
				retainLease = true
				result.Error = WorkspacePartialWrite
			} else {
				result.Error = workspaceMutationError(err)
			}
			return result
		}
		result.Committed = true
		result.USN = assignedUSN
	}

	repairPlan := db.WorkspaceMutationPlan{
		OperationID: operationID + ":projections", OwnerID: note.UserId, ResourceID: note.NoteId,
		Kind: "note_save_projections", InputDigest: inputDigest, DesiredState: desiredState,
		FailurePolicy: applicationnotes.FailurePending,
	}
	var assetProjectionStep *db.WorkspaceMutationStep
	if command.AssetWork != nil {
		repairPlan.Assets = append([]applicationnotes.OperationAsset(nil), command.AssetWork.Assets...)
		expectedAssetUSN := assignedUSN
		verifyAssetGeneration := func(ctx context.Context) (bool, error) {
			var current info.Note
			err := db.Notes.FindContext(ctx, bson.M{
				"_id": note.NoteId, "UserId": note.UserId, "Usn": expectedAssetUSN, "IsDeleted": false,
			}).One(&current)
			if errors.Is(err, mongo.ErrNoDocuments) {
				return false, nil
			}
			return err == nil, err
		}
		step := db.WorkspaceMutationStep{
			Name: "assets", ReplaySafe: true,
			Apply: func(ctx context.Context) error {
				// A repair may resume in a new process after the required note
				// write advanced its USN. Renew the same operation's fence before
				// any upload or attachment-row side effect.
				if err := db.AcquireWorkspaceNoteMutationLease(ctx, note.NoteId, note.UserId, expectedAssetUSN, operationID, time.Now()); err != nil {
					return err
				}
				valid, err := verifyAssetGeneration(ctx)
				if err != nil {
					return err
				}
				if !valid {
					return db.ErrDocumentNotFound
				}
				if err := command.AssetWork.Apply(ctx, expectedAssetUSN); err != nil {
					return err
				}
				valid, err = verifyAssetGeneration(ctx)
				if err != nil {
					return err
				}
				if !valid {
					return db.ErrDocumentNotFound
				}
				return nil
			},
			Verify: func(ctx context.Context) (bool, error) {
				if err := db.AcquireWorkspaceNoteMutationLease(ctx, note.NoteId, note.UserId, expectedAssetUSN, operationID, time.Now()); err != nil {
					if errors.Is(err, db.ErrWorkspaceNoteLeaseUnavailable) || errors.Is(err, db.ErrDocumentNotFound) {
						return false, nil
					}
					return false, err
				}
				valid, err := verifyAssetGeneration(ctx)
				if err != nil || !valid || command.AssetWork.Verify == nil {
					return valid, err
				}
				return command.AssetWork.Verify(ctx)
			},
		}
		assetProjectionStep = &step
	}
	recountsSourceNotebook := false
	if contentChanged {
		repairPlan.Steps = append(repairPlan.Steps, db.WorkspaceMutationStep{
			Name: "image_index", ReplaySafe: true,
			Apply: func(ctx context.Context) error {
				return noteImageService.updateNoteImages(ctx, note.UserId, note.NoteId, note.ImgSrc, *command.Content)
			},
			Verify: func(ctx context.Context) (bool, error) {
				return noteImageService.verifyNoteImages(ctx, note.UserId, note.NoteId, note.ImgSrc, *command.Content)
			},
		})
	}
	// Files is the API's complete requested asset set. Run its provider after
	// the content-derived image index so a combined update cannot commit a
	// projection that no longer matches the receipt-frozen Files manifest.
	if assetProjectionStep != nil {
		repairPlan.Steps = append(repairPlan.Steps, *assetProjectionStep)
	}
	if target, ok := metadata["NotebookId"].(domain.ObjectID); ok && target != note.NotebookId {
		recountsSourceNotebook = true
		repairPlan.Steps = append(repairPlan.Steps,
			db.WorkspaceMutationStep{Name: "source_notebook_count", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return notebookService.reCountNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return notebookService.verifyNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
			},
			db.WorkspaceMutationStep{Name: "target_notebook_count", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return notebookService.reCountNotebookNumberNotes(ctx, target, note.UserId)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return notebookService.verifyNotebookNumberNotes(ctx, target, note.UserId)
				},
			},
		)
	}
	if isTrash, ok := metadata["IsTrash"].(bool); ok {
		if !recountsSourceNotebook {
			repairPlan.Steps = append(repairPlan.Steps, db.WorkspaceMutationStep{Name: "trash_notebook_count", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return notebookService.reCountNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
				Verify: func(ctx context.Context) (bool, error) {
					return notebookService.verifyNotebookNumberNotes(ctx, note.NotebookId, note.UserId)
				},
			})
		}
		if isTrash {
			repairPlan.Steps = append(repairPlan.Steps, db.WorkspaceMutationStep{Name: "delete_shares", ReplaySafe: true,
				Apply: func(ctx context.Context) error { return shareService.deleteShareNoteAll(ctx, note.NoteId, note.UserId) },
				Verify: func(ctx context.Context) (bool, error) {
					return shareService.verifyShareNoteAllDeleted(ctx, note.NoteId, note.UserId)
				},
			})
		}
	}
	if tags, ok := metadata["Tags"].([]string); ok {
		repairPlan.Steps = append(repairPlan.Steps, db.WorkspaceMutationStep{Name: "tags", ReplaySafe: true,
			Apply:  func(ctx context.Context) error { return tagService.addTags(ctx, note.UserId, tags) },
			Verify: func(ctx context.Context) (bool, error) { return tagService.verifyTags(ctx, note.UserId, tags) },
		})
	}
	if len(repairPlan.Steps) > 0 {
		repair, repairErr := db.RunWorkspaceRepair(context.Background(), repairPlan)
		if repairErr != nil || !repair.Committed {
			retainLease = true
			if command.AssetWork != nil && repair.FailedStep == "assets" {
				result.Error = WorkspacePartialWrite
			} else {
				result.Error = WorkspaceSideEffect
			}
			result.PartialWrite = true
			result.FailedStep = repair.FailedStep
		}
	}
	return result
}

func findNoteForWorkspaceSave(ctx context.Context, noteID, actorUserID string) (info.Note, error) {
	if db.Notes == nil {
		return info.Note{}, db.ErrMongoClientNotInitialized
	}
	noteObjectID := db.MustObjectIDFromHex(noteID)
	actorObjectID := db.MustObjectIDFromHex(actorUserID)
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteObjectID, "UserId": actorObjectID}).One(&note)
	if err == nil || !errors.Is(err, mongo.ErrNoDocuments) {
		return note, err
	}
	err = db.Notes.FindContext(ctx, bson.M{"_id": noteObjectID, "IsDeleted": false}).One(&note)
	return note, err
}

func findNoteContentForWorkspaceSave(ctx context.Context, noteID, ownerID domain.ObjectID) (info.NoteContent, error) {
	if db.NoteContents == nil {
		return info.NoteContent{}, db.ErrMongoClientNotInitialized
	}
	var content info.NoteContent
	err := db.NoteContents.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}).One(&content)
	return content, err
}

func noteSaveNeedsProjection(command SaveNoteCommand, note info.Note, contentChanged bool) bool {
	if command.AssetWork != nil || contentChanged {
		return true
	}
	if _, blogChanged := command.Metadata["IsBlog"].(bool); blogChanged {
		return true
	}
	if target, ok := command.Metadata["NotebookId"].(domain.ObjectID); ok && target != note.NotebookId {
		return true
	}
	if _, ok := command.Metadata["IsTrash"].(bool); ok {
		return true
	}
	_, tagsChanged := command.Metadata["Tags"].([]string)
	return tagsChanged
}

func cloneBSONMap(source map[string]any) bson.M {
	cloned := make(bson.M, len(source)+3)
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func appendHistory(current info.NoteContentHistory, entry info.EachHistory) info.NoteContentHistory {
	histories := make([]info.EachHistory, 0, maxSize)
	histories = append(histories, entry)
	remaining := maxSize - 1
	if len(current.Histories) < remaining {
		remaining = len(current.Histories)
	}
	histories = append(histories, current.Histories[:remaining]...)
	current.Histories = histories
	return current
}

func workspaceMutationError(err error) WorkspaceErrorCategory {
	switch {
	case errors.Is(err, db.ErrDocumentNotFound), errors.Is(err, mongo.ErrNoDocuments):
		return WorkspaceConflict
	case errors.Is(err, db.ErrDuplicateIdentity):
		return WorkspaceDuplicate
	case errors.Is(err, context.DeadlineExceeded), mongo.IsTimeout(err):
		return WorkspaceTimeout
	case errors.Is(err, db.ErrPartialWrite):
		return WorkspacePartialWrite
	default:
		return WorkspaceStorage
	}
}
