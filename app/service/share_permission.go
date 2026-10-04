package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrInvalidShareGrant = errors.New("invalid share grant")
	ErrShareResource     = errors.New("shared resource is unavailable")
	ErrShareRecipient    = errors.New("share recipient is unavailable")
)

type ShareGrantOptions struct {
	ExpiresAt      *time.Time
	ClearExpiresAt bool
	Now            time.Time
}

type shareGrant struct {
	Perm      int
	Direct    bool
	ExpiresAt time.Time
}

var shareExpiryOffset = regexp.MustCompile(`[+-][0-9]{2}:[0-9]{2}$`)

// ParseShareExpiry validates the optional wire value once at the service
// boundary. A missing value is different from an explicitly empty value.
func ParseShareExpiry(raw string, present bool, now time.Time) (*time.Time, error) {
	if !present {
		return nil, nil
	}
	if raw == "" || strings.TrimSpace(raw) != raw || strings.Contains(raw, ".") {
		return nil, fmt.Errorf("%w: expiresAt must be a whole-second RFC3339 instant", ErrInvalidShareGrant)
	}
	if !strings.HasSuffix(raw, "Z") && !shareExpiryOffset.MatchString(raw) {
		return nil, fmt.Errorf("%w: expiresAt must include a timezone offset", ErrInvalidShareGrant)
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil || parsed.Nanosecond() != 0 {
		return nil, fmt.Errorf("%w: expiresAt is not a valid whole-second instant", ErrInvalidShareGrant)
	}
	if !parsed.After(now.UTC()) {
		return nil, fmt.Errorf("%w: expiresAt must be later than server time", ErrInvalidShareGrant)
	}
	parsed = parsed.UTC().Truncate(time.Second)
	return &parsed, nil
}

func validateShareGrantOptions(options ShareGrantOptions, existing bool) error {
	if options.ExpiresAt != nil && options.ClearExpiresAt {
		return fmt.Errorf("%w: expiresAt and clearExpiresAt are mutually exclusive", ErrInvalidShareGrant)
	}
	if options.ClearExpiresAt && !existing {
		return fmt.Errorf("%w: clearExpiresAt requires an existing grant", ErrInvalidShareGrant)
	}
	if options.ExpiresAt != nil {
		now := options.Now
		if now.IsZero() {
			now = time.Now().UTC()
		}
		if !options.ExpiresAt.After(now.UTC()) || options.ExpiresAt.Nanosecond() != 0 {
			return fmt.Errorf("%w: expiresAt must be later than server time and whole-second", ErrInvalidShareGrant)
		}
	}
	return nil
}

func activeShareGrant(grant shareGrant, now time.Time) bool {
	return grant.ExpiresAt.IsZero() || now.Before(grant.ExpiresAt)
}

func resolveShareGrantPermission(direct, groups []shareGrant, now time.Time) (perm int, allowed bool, err error) {
	activeDirect := make([]shareGrant, 0, len(direct))
	for _, grant := range direct {
		if grant.Perm != 0 && grant.Perm != 1 {
			return 0, false, fmt.Errorf("%w: permission must be 0 or 1", ErrInvalidShareGrant)
		}
		if activeShareGrant(grant, now) {
			activeDirect = append(activeDirect, grant)
		}
	}
	if len(activeDirect) > 1 {
		return 0, false, fmt.Errorf("%w: duplicate direct grants", ErrInvalidShareGrant)
	}
	if len(activeDirect) == 1 {
		return activeDirect[0].Perm, true, nil
	}

	for _, grant := range groups {
		if grant.Perm != 0 && grant.Perm != 1 {
			return 0, false, fmt.Errorf("%w: permission must be 0 or 1", ErrInvalidShareGrant)
		}
		if activeShareGrant(grant, now) && grant.Perm > perm {
			perm = grant.Perm
		}
		if activeShareGrant(grant, now) {
			allowed = true
		}
	}
	return perm, allowed, nil
}

func (s *ShareService) actorGroupIDs(ctx context.Context, actorID domain.ObjectID) ([]domain.ObjectID, error) {
	if db.Groups == nil || db.GroupUsers == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	var owned []info.Group
	if err := db.Groups.FindContext(ctx, bson.M{"UserId": actorID}).All(&owned); err != nil {
		return nil, err
	}
	var memberships []info.GroupUser
	if err := db.GroupUsers.FindContext(ctx, bson.M{"UserId": actorID}).All(&memberships); err != nil {
		return nil, err
	}
	seen := make(map[domain.ObjectID]struct{}, len(owned)+len(memberships))
	ids := make([]domain.ObjectID, 0, len(owned)+len(memberships))
	for _, group := range owned {
		if _, ok := seen[group.GroupId]; !ok {
			seen[group.GroupId] = struct{}{}
			ids = append(ids, group.GroupId)
		}
	}
	for _, membership := range memberships {
		if _, ok := seen[membership.GroupId]; !ok {
			seen[membership.GroupId] = struct{}{}
			ids = append(ids, membership.GroupId)
		}
	}
	return ids, nil
}

func (s *ShareService) resolveGrantPermission(ctx context.Context, ownerID, actorID, resourceID domain.ObjectID, resourceField string, now time.Time) (int, bool, error) {
	groups, err := s.actorGroupIDs(ctx, actorID)
	if err != nil {
		return 0, false, err
	}
	or := []bson.M{{"ToUserId": actorID}}
	if len(groups) > 0 {
		or = append(or, bson.M{"ToGroupId": bson.M{"$in": groups}})
	}
	var grants []struct {
		Perm      int             `bson:"Perm"`
		ToUserID  domain.ObjectID `bson:"ToUserId"`
		ToGroupID domain.ObjectID `bson:"ToGroupId"`
		ExpiresAt time.Time       `bson:"ExpiresAt"`
	}
	collection := db.ShareNotes
	filter := bson.M{"UserId": ownerID, resourceField: resourceID, "$or": or}
	if resourceField == "NotebookId" {
		collection = db.ShareNotebooks
	}
	if collection == nil {
		return 0, false, db.ErrMongoClientNotInitialized
	}
	if err := collection.FindContext(ctx, filter).All(&grants); err != nil {
		return 0, false, err
	}
	direct := make([]shareGrant, 0, 1)
	groupGrants := make([]shareGrant, 0, len(grants))
	for _, grant := range grants {
		value := shareGrant{Perm: grant.Perm, ExpiresAt: grant.ExpiresAt}
		if !grant.ToUserID.IsZero() {
			value.Direct = true
			direct = append(direct, value)
		} else if !grant.ToGroupID.IsZero() {
			groupGrants = append(groupGrants, value)
		}
	}
	return resolveShareGrantPermission(direct, groupGrants, now.UTC())
}

// ResolveNotePermission is the single read/update permission seam for notes.
// It returns allowed=false for a missing or expired grant and propagates DB
// errors so callers cannot mistake storage failure for denial or success.
func (s *ShareService) ResolveNotePermission(ctx context.Context, ownerID, actorID, noteID domain.ObjectID, now time.Time) (int, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if db.Notes == nil {
		return 0, false, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "IsTrash": false, "IsDeleted": false}).One(&note); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if ownerID == actorID {
		return 1, true, nil
	}
	perm, allowed, err := s.resolveGrantPermission(ctx, ownerID, actorID, noteID, "NoteId", now)
	if err != nil || allowed {
		return perm, allowed, err
	}
	return s.resolveNotebookPermission(ctx, ownerID, actorID, note.NotebookId, now)
}

func (s *ShareService) resolveNotebookPermission(ctx context.Context, ownerID, actorID, notebookID domain.ObjectID, now time.Time) (int, bool, error) {
	if db.Notebooks == nil {
		return 0, false, db.ErrMongoClientNotInitialized
	}
	var notebook info.Notebook
	if err := db.Notebooks.FindContext(ctx, bson.M{"_id": notebookID, "UserId": ownerID, "IsTrash": false, "IsDeleted": false}).One(&notebook); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if ownerID == actorID {
		return 1, true, nil
	}
	return s.resolveGrantPermission(ctx, ownerID, actorID, notebookID, "NotebookId", now)
}

func (s *ShareService) upsertShareNote(ctx context.Context, ownerID, noteID, recipientID domain.ObjectID, groupID domain.ObjectID, perm int, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 {
		return fmt.Errorf("%w: permission must be 0 or 1", ErrInvalidShareGrant)
	}
	var existing info.ShareNote
	filter := bson.M{"UserId": ownerID, "NoteId": noteID}
	if !recipientID.IsZero() {
		filter["ToUserId"] = recipientID
	} else {
		filter["ToGroupId"] = groupID
	}
	if err := db.ShareNotes.FindContext(ctx, filter).One(&existing); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	} else if err == nil {
		options.Now = optionsNow(options.Now)
		if err := validateShareGrantOptions(options, true); err != nil {
			return err
		}
	} else {
		if err := validateShareGrantOptions(options, false); err != nil {
			return err
		}
	}
	update := bson.M{"$set": bson.M{"Perm": perm}, "$setOnInsert": bson.M{
		"_id": db.NewObjectID(), "UserId": ownerID, "NoteId": noteID, "CreatedTime": optionsNow(options.Now),
	}}
	if !recipientID.IsZero() {
		update["$setOnInsert"].(bson.M)["ToUserId"] = recipientID
	} else {
		update["$setOnInsert"].(bson.M)["ToGroupId"] = groupID
	}
	if options.ExpiresAt != nil {
		update["$set"].(bson.M)["ExpiresAt"] = options.ExpiresAt.UTC().Truncate(time.Second)
	}
	if options.ClearExpiresAt {
		update["$unset"] = bson.M{"ExpiresAt": ""}
	}
	persist := func() error {
		_, err := db.ShareNotes.UpsertContext(ctx, filter, update)
		if mongo.IsDuplicateKeyError(err) {
			err = db.ShareNotes.UpdateOneMatchedContext(ctx, filter, update)
		}
		return err
	}
	if recipientID.IsZero() {
		return persist()
	}
	return persistShareGrantWithProjection(
		func() error { return s.ensureShareProjection(ctx, ownerID, recipientID) }, persist,
	)
}

func (s *ShareService) upsertShareNotebook(ctx context.Context, ownerID, notebookID, recipientID domain.ObjectID, groupID domain.ObjectID, perm int, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 {
		return fmt.Errorf("%w: permission must be 0 or 1", ErrInvalidShareGrant)
	}
	var existing info.ShareNotebook
	filter := bson.M{"UserId": ownerID, "NotebookId": notebookID}
	if !recipientID.IsZero() {
		filter["ToUserId"] = recipientID
	} else {
		filter["ToGroupId"] = groupID
	}
	err := db.ShareNotebooks.FindContext(ctx, filter).One(&existing)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if err := validateShareGrantOptions(options, err == nil); err != nil {
		return err
	}
	update := bson.M{"$set": bson.M{"Perm": perm}, "$setOnInsert": bson.M{
		"_id": db.NewObjectID(), "UserId": ownerID, "NotebookId": notebookID, "CreatedTime": optionsNow(options.Now),
	}}
	if !recipientID.IsZero() {
		update["$setOnInsert"].(bson.M)["ToUserId"] = recipientID
	} else {
		update["$setOnInsert"].(bson.M)["ToGroupId"] = groupID
	}
	if options.ExpiresAt != nil {
		update["$set"].(bson.M)["ExpiresAt"] = options.ExpiresAt.UTC().Truncate(time.Second)
	}
	if options.ClearExpiresAt {
		update["$unset"] = bson.M{"ExpiresAt": ""}
	}
	persist := func() error {
		_, err := db.ShareNotebooks.UpsertContext(ctx, filter, update)
		if mongo.IsDuplicateKeyError(err) {
			err = db.ShareNotebooks.UpdateOneMatchedContext(ctx, filter, update)
		}
		return err
	}
	if recipientID.IsZero() {
		return persist()
	}
	return persistShareGrantWithProjection(
		func() error { return s.ensureShareProjection(ctx, ownerID, recipientID) }, persist,
	)
}

func (s *ShareService) ensureOwnedNote(ctx context.Context, ownerID, noteID domain.ObjectID) error {
	if db.Notes == nil {
		return db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "IsTrash": false, "IsDeleted": false}).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrShareResource
	}
	return err
}

func (s *ShareService) ensureOwnedNotebook(ctx context.Context, ownerID, notebookID domain.ObjectID) error {
	if db.Notebooks == nil {
		return db.ErrMongoClientNotInitialized
	}
	var notebook info.Notebook
	err := db.Notebooks.FindContext(ctx, bson.M{"_id": notebookID, "UserId": ownerID, "IsTrash": false, "IsDeleted": false}).One(&notebook)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrShareResource
	}
	return err
}

func (s *ShareService) ensureOwnedGroup(ctx context.Context, ownerID, groupID domain.ObjectID) error {
	if db.Groups == nil {
		return db.ErrMongoClientNotInitialized
	}
	var group info.Group
	err := db.Groups.FindContext(ctx, bson.M{"_id": groupID, "UserId": ownerID}).One(&group)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrShareRecipient
	}
	return err
}

func (s *ShareService) ensureShareProjection(ctx context.Context, ownerID, recipientID domain.ObjectID) error {
	if db.HasShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	_, err := db.HasShareNotes.UpsertContext(ctx,
		bson.M{"UserId": ownerID, "ToUserId": recipientID},
		bson.M{"$setOnInsert": bson.M{"_id": db.NewObjectID(), "UserId": ownerID, "ToUserId": recipientID}})
	return err
}

func persistShareGrantWithProjection(ensureProjection, persistGrant func() error) error {
	if err := ensureProjection(); err != nil {
		return err
	}
	return persistGrant()
}

func (s *ShareService) AddShareNoteToUserIdWithOptions(noteID string, perm int, ownerID, recipientID string, options ShareGrantOptions) error {
	if !db.IsValidObjectIDHex(noteID) || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(recipientID) || ownerID == recipientID {
		return ErrInvalidShareGrant
	}
	if db.ShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	ctx := context.Background()
	owner := db.MustObjectIDFromHex(ownerID)
	if err := s.ensureOwnedNote(ctx, owner, db.MustObjectIDFromHex(noteID)); err != nil {
		return err
	}
	return s.upsertShareNote(ctx, owner, db.MustObjectIDFromHex(noteID), db.MustObjectIDFromHex(recipientID), domain.ObjectID{}, perm, options)
}

func (s *ShareService) AddShareNotebookToUserIdWithOptions(notebookID string, perm int, ownerID, recipientID string, options ShareGrantOptions) error {
	if !db.IsValidObjectIDHex(notebookID) || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(recipientID) || ownerID == recipientID {
		return ErrInvalidShareGrant
	}
	if db.ShareNotebooks == nil {
		return db.ErrMongoClientNotInitialized
	}
	ctx := context.Background()
	owner := db.MustObjectIDFromHex(ownerID)
	if err := s.ensureOwnedNotebook(ctx, owner, db.MustObjectIDFromHex(notebookID)); err != nil {
		return err
	}
	return s.upsertShareNotebook(ctx, owner, db.MustObjectIDFromHex(notebookID), db.MustObjectIDFromHex(recipientID), domain.ObjectID{}, perm, options)
}

// ValidateShareBatchOptions performs every recipient existence check needed by
// clearExpiresAt before any grant is changed. This keeps a mixed new/existing
// batch from partially clearing existing grants.
func (s *ShareService) ValidateShareBatchOptions(ctx context.Context, collection *db.Collection, filterField string, ownerID, resourceID domain.ObjectID, recipientIDs []domain.ObjectID, options ShareGrantOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !options.ClearExpiresAt {
		return nil
	}
	if len(recipientIDs) == 0 {
		return fmt.Errorf("%w: clearExpiresAt requires recipients", ErrInvalidShareGrant)
	}
	if collection == nil {
		return db.ErrMongoClientNotInitialized
	}
	for _, recipientID := range recipientIDs {
		filter := bson.M{"UserId": ownerID, filterField: resourceID, "ToUserId": recipientID}
		var existing bson.Raw
		if err := collection.FindContext(ctx, filter).One(&existing); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return fmt.Errorf("%w: clearExpiresAt requires an existing grant for every recipient", ErrInvalidShareGrant)
			}
			return err
		}
	}
	return nil
}

func (s *ShareService) ValidateShareNoteBatch(noteID, ownerID string, recipientIDs []string, options ShareGrantOptions) error {
	if !db.IsValidObjectIDHex(noteID) || !db.IsValidObjectIDHex(ownerID) {
		return ErrInvalidShareGrant
	}
	owner := db.MustObjectIDFromHex(ownerID)
	note := db.MustObjectIDFromHex(noteID)
	if err := s.ensureOwnedNote(context.Background(), owner, note); err != nil {
		return err
	}
	ids := make([]domain.ObjectID, 0, len(recipientIDs))
	for _, recipientID := range recipientIDs {
		if !db.IsValidObjectIDHex(recipientID) || recipientID == ownerID {
			return ErrInvalidShareGrant
		}
		ids = append(ids, db.MustObjectIDFromHex(recipientID))
	}
	return s.ValidateShareBatchOptions(context.Background(), db.ShareNotes, "NoteId", owner, note, ids, options)
}

func (s *ShareService) ValidateShareNotebookBatch(notebookID, ownerID string, recipientIDs []string, options ShareGrantOptions) error {
	if !db.IsValidObjectIDHex(notebookID) || !db.IsValidObjectIDHex(ownerID) {
		return ErrInvalidShareGrant
	}
	owner := db.MustObjectIDFromHex(ownerID)
	notebook := db.MustObjectIDFromHex(notebookID)
	if err := s.ensureOwnedNotebook(context.Background(), owner, notebook); err != nil {
		return err
	}
	ids := make([]domain.ObjectID, 0, len(recipientIDs))
	for _, recipientID := range recipientIDs {
		if !db.IsValidObjectIDHex(recipientID) || recipientID == ownerID {
			return ErrInvalidShareGrant
		}
		ids = append(ids, db.MustObjectIDFromHex(recipientID))
	}
	return s.ValidateShareBatchOptions(context.Background(), db.ShareNotebooks, "NotebookId", owner, notebook, ids, options)
}

func (s *ShareService) updateShareNoteGrant(ctx context.Context, ownerID, noteID, recipientID domain.ObjectID, perm int, options ShareGrantOptions) error {
	if db.ShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	if err := s.ensureOwnedNote(ctx, ownerID, noteID); err != nil {
		return err
	}
	var existing info.ShareNote
	filter := bson.M{"UserId": ownerID, "NoteId": noteID, "ToUserId": recipientID}
	if err := db.ShareNotes.FindContext(ctx, filter).One(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrShareRecipient
		}
		return err
	}
	if err := validateShareGrantOptions(options, true); err != nil {
		return err
	}
	set := bson.M{"Perm": perm}
	if options.ExpiresAt != nil {
		set["ExpiresAt"] = options.ExpiresAt.UTC().Truncate(time.Second)
	}
	update := bson.M{"$set": set}
	if options.ClearExpiresAt {
		update["$unset"] = bson.M{"ExpiresAt": ""}
	}
	return db.ShareNotes.UpdateOneMatchedContext(ctx, filter, update)
}

func (s *ShareService) updateShareNotebookGrant(ctx context.Context, ownerID, notebookID, recipientID domain.ObjectID, perm int, options ShareGrantOptions) error {
	if db.ShareNotebooks == nil {
		return db.ErrMongoClientNotInitialized
	}
	if err := s.ensureOwnedNotebook(ctx, ownerID, notebookID); err != nil {
		return err
	}
	var existing info.ShareNotebook
	filter := bson.M{"UserId": ownerID, "NotebookId": notebookID, "ToUserId": recipientID}
	if err := db.ShareNotebooks.FindContext(ctx, filter).One(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrShareRecipient
		}
		return err
	}
	if err := validateShareGrantOptions(options, true); err != nil {
		return err
	}
	set := bson.M{"Perm": perm}
	if options.ExpiresAt != nil {
		set["ExpiresAt"] = options.ExpiresAt.UTC().Truncate(time.Second)
	}
	update := bson.M{"$set": set}
	if options.ClearExpiresAt {
		update["$unset"] = bson.M{"ExpiresAt": ""}
	}
	return db.ShareNotebooks.UpdateOneMatchedContext(ctx, filter, update)
}

func (s *ShareService) UpdateShareNotePermWithOptions(noteID string, perm int, ownerID, recipientID string, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 || !db.IsValidObjectIDHex(noteID) || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(recipientID) || ownerID == recipientID {
		return ErrInvalidShareGrant
	}
	return s.updateShareNoteGrant(context.Background(), db.MustObjectIDFromHex(ownerID), db.MustObjectIDFromHex(noteID), db.MustObjectIDFromHex(recipientID), perm, options)
}

func (s *ShareService) UpdateShareNotebookPermWithOptions(notebookID string, perm int, ownerID, recipientID string, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 || !db.IsValidObjectIDHex(notebookID) || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(recipientID) || ownerID == recipientID {
		return ErrInvalidShareGrant
	}
	return s.updateShareNotebookGrant(context.Background(), db.MustObjectIDFromHex(ownerID), db.MustObjectIDFromHex(notebookID), db.MustObjectIDFromHex(recipientID), perm, options)
}

func (s *ShareService) AddShareNoteGroupWithOptions(ownerID, noteID, groupID string, perm int, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(noteID) || !db.IsValidObjectIDHex(groupID) {
		return ErrInvalidShareGrant
	}
	ctx := context.Background()
	owner := db.MustObjectIDFromHex(ownerID)
	note := db.MustObjectIDFromHex(noteID)
	group := db.MustObjectIDFromHex(groupID)
	if err := s.ensureOwnedNote(ctx, owner, note); err != nil {
		return err
	}
	if err := s.ensureOwnedGroup(ctx, owner, group); err != nil {
		return err
	}
	return s.upsertShareNote(ctx, owner, note, domain.ObjectID{}, group, perm, options)
}

func (s *ShareService) AddShareNotebookGroupWithOptions(ownerID, notebookID, groupID string, perm int, options ShareGrantOptions) error {
	if perm != 0 && perm != 1 || !db.IsValidObjectIDHex(ownerID) || !db.IsValidObjectIDHex(notebookID) || !db.IsValidObjectIDHex(groupID) {
		return ErrInvalidShareGrant
	}
	ctx := context.Background()
	owner := db.MustObjectIDFromHex(ownerID)
	notebook := db.MustObjectIDFromHex(notebookID)
	group := db.MustObjectIDFromHex(groupID)
	if err := s.ensureOwnedNotebook(ctx, owner, notebook); err != nil {
		return err
	}
	if err := s.ensureOwnedGroup(ctx, owner, group); err != nil {
		return err
	}
	return s.upsertShareNotebook(ctx, owner, notebook, domain.ObjectID{}, group, perm, options)
}

func optionsNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC().Truncate(time.Second)
	}
	return now.UTC().Truncate(time.Second)
}
