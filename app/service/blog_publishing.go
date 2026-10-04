package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type noteBlogOperationInput struct {
	IsBlog     bool   `json:"isBlog"`
	IsTop      bool   `json:"isTop"`
	Generation int    `json:"generation"`
	Nonce      string `json:"nonce,omitempty"`
}

type noteBlogBeforeState struct {
	Note      info.Note
	Content   info.NoteContent
	TagCounts []info.TagCount
}

type noteBlogDesiredState struct {
	IsBlog         bool
	IsTop          bool
	HasSelfDefined bool
	PublicTime     time.Time
}

type notebookBlogOperationInput struct {
	IsBlog     bool   `json:"isBlog"`
	Generation int    `json:"generation"`
	Nonce      string `json:"nonce,omitempty"`
}

type notebookBlogBeforeState struct {
	Notebook     info.Notebook
	ChildNoteIDs []string
}

type notebookBlogDesiredState struct {
	IsBlog       bool
	ChildNoteIDs []string
}

type notebookBlogResultState struct {
	IsBlog       bool
	ChildNoteIDs []string
}

func newWorkspaceIntentNonce() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (this *NoteService) toBlogWithReceipt(userID, noteID string, isBlog, isTop bool) bool {
	if !db.IsValidObjectIDHex(userID) || !db.IsValidObjectIDHex(noteID) || db.Notes == nil || db.NoteContents == nil || db.TagCounts == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userID)
	noteObjectID := db.MustObjectIDFromHex(noteID)

	note := info.Note{}
	if err := db.Notes.FindContext(context.Background(), bson.M{
		"_id": noteObjectID, "UserId": ownerID, "IsDeleted": false,
	}).One(&note); err != nil || note.NoteId.IsZero() || note.IsTrash {
		return false
	}
	content := info.NoteContent{}
	if err := db.NoteContents.FindContext(context.Background(), bson.M{
		"_id": noteObjectID, "UserId": ownerID,
	}).One(&content); err != nil || content.NoteId.IsZero() {
		return false
	}

	if isTop {
		isBlog = true
	}
	if !isBlog {
		isTop = false
	}
	pending, hasPending, err := db.UnfinishedWorkspaceOperation(context.Background(), ownerID, noteObjectID, "note_blog")
	if err != nil {
		return false
	}
	input := noteBlogOperationInput{IsBlog: isBlog, IsTop: isTop, Generation: note.Usn}
	operationID, inputDigest, desiredPayload, err := applicationnotes.NewOperationIdentity("note_blog", ownerID, noteObjectID, input)
	if err != nil {
		return false
	}
	if hasPending {
		operationID, inputDigest = pending.OperationID, pending.InputDigest
	} else {
		existing, lookupErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
		switch {
		case lookupErr == nil && !applicationnotes.IsTerminalOperation(existing.Status):
			hasPending, pending = true, existing
		case lookupErr == nil:
			input.Nonce, err = newWorkspaceIntentNonce()
			if err != nil {
				return false
			}
			operationID, inputDigest, desiredPayload, err = applicationnotes.NewOperationIdentity("note_blog", ownerID, noteObjectID, input)
			if err != nil {
				return false
			}
		case !errors.Is(lookupErr, mongo.ErrNoDocuments):
			return false
		}
	}

	tagCounts, err := loadBlogTagCounts(context.Background(), ownerID)
	if err != nil {
		return false
	}
	desired := noteBlogDesiredState{
		IsBlog:         isBlog,
		IsTop:          isTop,
		HasSelfDefined: note.HasSelfDefined,
		PublicTime:     note.PublicTime,
	}
	if !isBlog {
		desired.HasSelfDefined = false
	} else if !note.IsBlog {
		desired.PublicTime = time.Now()
	}
	if !hasPending && note.IsBlog == isBlog && note.IsTop == isTop && (isBlog || !note.HasSelfDefined) &&
		verifyNoteBlogMutation(context.Background(), ownerID, noteObjectID, desired, note.Usn) {
		return true
	}
	before := noteBlogBeforeState{Note: note, Content: content, TagCounts: tagCounts}

	receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
	switch {
	case receiptErr == nil && receipt.Status != applicationnotes.OperationCommitted:
		if len(receipt.BeforeState) == 0 || len(receipt.DesiredState) == 0 {
			return false
		}
		if err := json.Unmarshal(receipt.BeforeState, &before); err != nil || before.Note.NoteId != noteObjectID || before.Note.UserId != ownerID {
			return false
		}
		if err := json.Unmarshal(receipt.DesiredState, &desired); err != nil || desired.IsBlog != isBlog || desired.IsTop != isTop {
			return false
		}
	case receiptErr != nil && !errors.Is(receiptErr, mongo.ErrNoDocuments):
		return false
	}

	beforePayload, err := json.Marshal(before)
	if err != nil {
		return false
	}
	desiredPayload, err = json.Marshal(desired)
	if err != nil {
		return false
	}
	var assignedUSN int
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: noteObjectID,
		Kind: "note_blog", InputDigest: inputDigest, BeforeState: beforePayload, DesiredState: desiredPayload,
		AssignedUSN: func() int { return assignedUSN },
		RestoreAssignedUSN: func(usn int) {
			assignedUSN = usn
		},
		RestoreBeforeState: func(payload []byte) error {
			var frozen noteBlogBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Note.NoteId != noteObjectID || frozen.Note.UserId != ownerID {
				return fmt.Errorf("note blog before state target changed")
			}
			before = frozen
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen noteBlogDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil {
				return err
			}
			desired = frozen
			return nil
		},
		FailurePolicy: applicationnotes.FailurePending,
	}
	plan.Steps = []db.WorkspaceMutationStep{
		{
			Name: "note", ReplaySafe: true,
			AssignedUSN:        func() int { return assignedUSN },
			RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
			Apply: func(ctx context.Context) error {
				if assignedUSN <= 0 {
					var err error
					assignedUSN, err = db.AllocateUserUSN(ctx, ownerID)
					if err != nil {
						return err
					}
				}
				update := bson.M{"IsBlog": desired.IsBlog, "IsTop": desired.IsTop, "Usn": assignedUSN}
				if desired.IsBlog {
					update["PublicTime"] = desired.PublicTime
				} else {
					update["HasSelfDefined"] = false
				}
				filter := bson.M{"_id": before.Note.NoteId, "UserId": ownerID, "Usn": before.Note.Usn, "IsDeleted": false, "IsTrash": false}
				db.AddWorkspaceNoteMutationLeaseFilter(filter, operationID, time.Now())
				return db.Notes.UpdateOneMatchedContext(ctx, filter, bson.M{"$set": update})
			},
			Verify: func(ctx context.Context) (bool, error) {
				if assignedUSN <= 0 {
					return false, nil
				}
				filter := bson.M{"_id": before.Note.NoteId, "UserId": ownerID, "Usn": assignedUSN, "IsDeleted": false, "IsTrash": false, "IsBlog": desired.IsBlog, "IsTop": desired.IsTop}
				if desired.IsBlog {
					filter["PublicTime"] = desired.PublicTime
				}
				var current info.Note
				err := db.Notes.FindContext(ctx, filter).One(&current)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
		},
		{
			Name: "content_blog", ReplaySafe: true,
			Apply: func(ctx context.Context) error {
				return db.NoteContents.UpdateOneMatchedContext(ctx,
					bson.M{"_id": before.Note.NoteId, "UserId": ownerID},
					bson.M{"$set": bson.M{"IsBlog": desired.IsBlog}},
				)
			},
			Verify: func(ctx context.Context) (bool, error) {
				var current info.NoteContent
				err := db.NoteContents.FindContext(ctx, bson.M{"_id": before.Note.NoteId, "UserId": ownerID, "IsBlog": desired.IsBlog}).One(&current)
				if errors.Is(err, mongo.ErrNoDocuments) {
					return false, nil
				}
				return err == nil, err
			},
		},
		{
			Name: "blog_tags", ReplaySafe: true,
			Apply: func(ctx context.Context) error {
				return replaceBlogTagCountsContext(ctx, ownerID, before.TagCounts)
			},
			Verify: func(ctx context.Context) (bool, error) {
				return verifyBlogTagCountsContext(ctx, ownerID)
			},
			Compensate: func(ctx context.Context) error {
				return restoreBlogTagCountsContext(ctx, ownerID, before.TagCounts)
			},
		},
	}

	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !mutation.Committed {
		return false
	}
	if assignedUSN <= 0 {
		assignedUSN = mutation.Operation.AssignedUSN
	}
	return verifyNoteBlogMutation(context.Background(), ownerID, noteObjectID, desired, assignedUSN)
}

func (this *NotebookService) toBlogWithReceipt(userID, notebookID string, isBlog bool) bool {
	if !db.IsValidObjectIDHex(userID) || !db.IsValidObjectIDHex(notebookID) || db.Notebooks == nil || db.Notes == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userID)
	notebookObjectID := db.MustObjectIDFromHex(notebookID)
	notebook := info.Notebook{}
	if err := db.Notebooks.FindContext(context.Background(), bson.M{
		"_id": notebookObjectID, "UserId": ownerID, "IsDeleted": false, "IsTrash": false,
	}).One(&notebook); err != nil || notebook.NotebookId.IsZero() {
		return false
	}
	pending, hasPending, err := db.UnfinishedWorkspaceOperation(context.Background(), ownerID, notebookObjectID, "notebook_blog")
	if err != nil {
		return false
	}
	input := notebookBlogOperationInput{IsBlog: isBlog, Generation: notebook.Usn}
	operationID, inputDigest, _, err := applicationnotes.NewOperationIdentity("notebook_blog", ownerID, notebookObjectID, input)
	if err != nil {
		return false
	}
	reuseCommitted := false
	if hasPending {
		operationID, inputDigest = pending.OperationID, pending.InputDigest
	} else if notebook.Usn > 0 {
		committed, found, err := db.LatestCommittedWorkspaceOperation(context.Background(), ownerID, notebookObjectID, "notebook_blog", notebook.Usn)
		if err != nil {
			return false
		}
		if found {
			var frozen notebookBlogResultState
			if len(committed.ResultState) == 0 || json.Unmarshal(committed.ResultState, &frozen) != nil {
				return false
			}
			if frozen.IsBlog == isBlog {
				operationID, inputDigest = committed.OperationID, committed.InputDigest
				reuseCommitted = true
			}
		}
	}
	if !hasPending {
		existing, lookupErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
		switch {
		case lookupErr == nil && !applicationnotes.IsTerminalOperation(existing.Status):
			hasPending, pending = true, existing
		case lookupErr == nil && !reuseCommitted:
			input.Nonce, err = newWorkspaceIntentNonce()
			if err != nil {
				return false
			}
			operationID, inputDigest, _, err = applicationnotes.NewOperationIdentity("notebook_blog", ownerID, notebookObjectID, input)
			if err != nil {
				return false
			}
		case !errors.Is(lookupErr, mongo.ErrNoDocuments) && lookupErr != nil:
			return false
		}
	}
	desired := notebookBlogDesiredState{IsBlog: isBlog}
	before := notebookBlogBeforeState{Notebook: notebook}
	receipt, receiptErr := db.GetWorkspaceOperation(context.Background(), ownerID, operationID)
	switch {
	case receiptErr == nil:
		if receipt.Status == applicationnotes.OperationCommitted {
			var frozen notebookBlogResultState
			if len(receipt.ResultState) == 0 || json.Unmarshal(receipt.ResultState, &frozen) != nil || frozen.IsBlog != isBlog {
				return false
			}
			desired.ChildNoteIDs = append([]string(nil), frozen.ChildNoteIDs...)
		} else {
			if len(receipt.BeforeState) == 0 || len(receipt.DesiredState) == 0 {
				return false
			}
			if err := json.Unmarshal(receipt.BeforeState, &before); err != nil || before.Notebook.NotebookId != notebookObjectID || before.Notebook.UserId != ownerID {
				return false
			}
			if err := json.Unmarshal(receipt.DesiredState, &desired); err != nil || desired.IsBlog != isBlog {
				return false
			}
		}
	case receiptErr != nil && !errors.Is(receiptErr, mongo.ErrNoDocuments):
		return false
	case errors.Is(receiptErr, mongo.ErrNoDocuments):
		childNotes := []info.Note{}
		if err := db.Notes.FindContext(context.Background(), bson.M{
			"UserId": ownerID, "NotebookId": notebookObjectID, "IsDeleted": false, "IsTrash": false,
		}).Sort("_id").All(&childNotes); err != nil {
			return false
		}
		desired.ChildNoteIDs = make([]string, 0, len(childNotes))
		for _, note := range childNotes {
			desired.ChildNoteIDs = append(desired.ChildNoteIDs, note.NoteId.Hex())
		}
		before.ChildNoteIDs = append([]string(nil), desired.ChildNoteIDs...)
	}
	if !validNotebookBlogChildIDs(desired.ChildNoteIDs) {
		return false
	}
	if len(before.ChildNoteIDs) == 0 {
		before.ChildNoteIDs = append([]string(nil), desired.ChildNoteIDs...)
	}
	beforePayload, err := json.Marshal(before)
	if err != nil {
		return false
	}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	var assignedUSN int
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: notebookObjectID,
		Kind: "notebook_blog", InputDigest: inputDigest, BeforeState: beforePayload, DesiredState: desiredPayload,
		AssignedUSN:        func() int { return assignedUSN },
		RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
		RestoreBeforeState: func(payload []byte) error {
			var frozen notebookBlogBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Notebook.NotebookId != notebookObjectID || frozen.Notebook.UserId != ownerID {
				return fmt.Errorf("notebook blog before state target changed")
			}
			before = frozen
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen notebookBlogDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil {
				return err
			}
			desired = frozen
			return nil
		},
		CaptureResultState: func() []byte {
			payload, err := json.Marshal(notebookBlogResultState{IsBlog: desired.IsBlog, ChildNoteIDs: desired.ChildNoteIDs})
			if err != nil {
				return nil
			}
			return payload
		},
		RestoreResultState: func(payload []byte) error {
			var frozen notebookBlogResultState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.IsBlog != isBlog || !validNotebookBlogChildIDs(frozen.ChildNoteIDs) {
				return fmt.Errorf("notebook blog result state changed")
			}
			desired.IsBlog = frozen.IsBlog
			desired.ChildNoteIDs = append([]string(nil), frozen.ChildNoteIDs...)
			return nil
		},
		FailurePolicy: applicationnotes.FailurePending,
		Steps: []db.WorkspaceMutationStep{
			{
				Name: "notebook", ReplaySafe: true,
				AssignedUSN:        func() int { return assignedUSN },
				RestoreAssignedUSN: func(usn int) { assignedUSN = usn },
				Apply: func(ctx context.Context) error {
					if assignedUSN <= 0 {
						var err error
						assignedUSN, err = db.AllocateUserUSN(ctx, ownerID)
						if err != nil {
							return err
						}
					}
					filter := bson.M{"_id": before.Notebook.NotebookId, "UserId": ownerID, "Usn": before.Notebook.Usn, "IsDeleted": false, "IsTrash": false}
					return db.Notebooks.UpdateOneMatchedContext(ctx, filter, bson.M{"$set": bson.M{"IsBlog": desired.IsBlog, "Usn": assignedUSN}})
				},
				Verify: func(ctx context.Context) (bool, error) {
					if assignedUSN <= 0 {
						return false, nil
					}
					var current info.Notebook
					err := db.Notebooks.FindContext(ctx, bson.M{"_id": before.Notebook.NotebookId, "UserId": ownerID, "Usn": assignedUSN, "IsDeleted": false, "IsTrash": false, "IsBlog": desired.IsBlog}).One(&current)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil, err
				},
			},
		},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !mutation.Committed {
		return false
	}
	if assignedUSN <= 0 {
		assignedUSN = mutation.Operation.AssignedUSN
	}
	_ = assignedUSN

	childService := &NoteService{}
	allChildrenSucceeded := true
	for _, childNoteID := range desired.ChildNoteIDs {
		if !childService.toBlogWithReceipt(ownerID.Hex(), childNoteID, desired.IsBlog, false) {
			allChildrenSucceeded = false
		}
	}
	return allChildrenSucceeded
}

func validNotebookBlogChildIDs(childNoteIDs []string) bool {
	seen := make(map[string]struct{}, len(childNoteIDs))
	for _, childNoteID := range childNoteIDs {
		if !db.IsValidObjectIDHex(childNoteID) {
			return false
		}
		if _, exists := seen[childNoteID]; exists {
			return false
		}
		seen[childNoteID] = struct{}{}
	}
	return true
}

func loadBlogTagCounts(ctx context.Context, ownerID domain.ObjectID) ([]info.TagCount, error) {
	if db.TagCounts == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	counts := []info.TagCount{}
	if err := db.TagCounts.FindContext(ctx, bson.M{"UserId": ownerID, "IsBlog": true}).All(&counts); err != nil {
		return nil, fmt.Errorf("load blog tag counts: %w", err)
	}
	return counts, nil
}

func expectedBlogTagCounts(ctx context.Context, ownerID domain.ObjectID) ([]info.TagCount, error) {
	if db.Notes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	notes := []info.Note{}
	if err := db.Notes.FindContext(ctx, bson.M{"UserId": ownerID, "IsBlog": true, "IsTrash": false, "IsDeleted": false}).All(&notes); err != nil {
		return nil, fmt.Errorf("list blog notes for tag counts: %w", err)
	}
	tagCounts := make(map[string]int)
	for _, note := range notes {
		for _, tag := range note.Tags {
			tagCounts[tag]++
		}
	}
	tags := make([]string, 0, len(tagCounts))
	for tag := range tagCounts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	result := make([]info.TagCount, 0, len(tags))
	for _, tag := range tags {
		result = append(result, info.TagCount{TagCountId: db.NewObjectID(), UserId: ownerID, Tag: tag, IsBlog: true, Count: tagCounts[tag]})
	}
	return result, nil
}

func replaceBlogTagCountsContext(ctx context.Context, ownerID domain.ObjectID, before []info.TagCount) error {
	if db.TagCounts == nil {
		return db.ErrMongoClientNotInitialized
	}
	expected, err := expectedBlogTagCounts(ctx, ownerID)
	if err != nil {
		return err
	}
	filter := bson.M{"UserId": ownerID, "IsBlog": true}
	if _, err := db.TagCounts.RemoveAllContext(ctx, filter); err != nil {
		return fmt.Errorf("clear blog tag counts: %w", err)
	}
	if len(expected) == 0 {
		return nil
	}
	documents := make([]interface{}, len(expected))
	for index := range expected {
		documents[index] = expected[index]
	}
	if err := db.TagCounts.InsertContext(ctx, documents...); err != nil {
		restoreErr := restoreBlogTagCountsContext(ctx, ownerID, before)
		return errors.Join(fmt.Errorf("write blog tag counts: %w", err), restoreErr)
	}
	return nil
}

func restoreBlogTagCountsContext(ctx context.Context, ownerID domain.ObjectID, before []info.TagCount) error {
	if db.TagCounts == nil {
		return db.ErrMongoClientNotInitialized
	}
	if _, err := db.TagCounts.RemoveAllContext(ctx, bson.M{"UserId": ownerID, "IsBlog": true}); err != nil {
		return fmt.Errorf("clear blog tag counts for restore: %w", err)
	}
	if len(before) == 0 {
		return nil
	}
	documents := make([]interface{}, len(before))
	for index := range before {
		documents[index] = before[index]
	}
	if err := db.TagCounts.InsertContext(ctx, documents...); err != nil {
		return fmt.Errorf("restore blog tag counts: %w", err)
	}
	return nil
}

func verifyBlogTagCountsContext(ctx context.Context, ownerID domain.ObjectID) (bool, error) {
	expected, err := expectedBlogTagCounts(ctx, ownerID)
	if err != nil {
		return false, err
	}
	actual, err := loadBlogTagCounts(ctx, ownerID)
	if err != nil {
		return false, err
	}
	if len(expected) != len(actual) {
		return false, nil
	}
	want := make(map[string]int, len(expected))
	for _, count := range expected {
		want[count.Tag] = count.Count
	}
	for _, count := range actual {
		if !count.IsBlog || want[count.Tag] != count.Count {
			return false, nil
		}
		delete(want, count.Tag)
	}
	return len(want) == 0, nil
}

func verifyNoteBlogMutation(ctx context.Context, ownerID, noteID domain.ObjectID, desired noteBlogDesiredState, assignedUSN int) bool {
	if assignedUSN <= 0 || db.Notes == nil || db.NoteContents == nil {
		return false
	}
	filter := bson.M{"_id": noteID, "UserId": ownerID, "Usn": assignedUSN, "IsDeleted": false, "IsTrash": false, "IsBlog": desired.IsBlog, "IsTop": desired.IsTop}
	if desired.IsBlog {
		filter["PublicTime"] = desired.PublicTime
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, filter).One(&note); err != nil {
		return false
	}
	var content info.NoteContent
	if err := db.NoteContents.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "IsBlog": desired.IsBlog}).One(&content); err != nil {
		return false
	}
	ok, err := verifyBlogTagCountsContext(ctx, ownerID)
	return err == nil && ok
}
