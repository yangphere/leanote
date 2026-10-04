package content

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

func TestCreateRepairPersistsBeforePublishAndVerifiesAmbiguousCommit(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	content := &fakeCreateContent{events: &events, verifyStatus: VerificationApplied}
	rows := &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowExact}}
	service := CreateRepairService{
		Content: content, Lifecycle: &fakeCreateLifecycle{events: &events}, Manifests: manifests, Rows: rows, Now: func() time.Time { return now },
	}
	command := createRepairCommand(t, &events, errors.New("ambiguous insert"))
	result, err := service.Execute(context.Background(), command)
	if err != nil || result.Status != CreateRepairCommitted {
		t.Fatalf("Execute() result=%+v err=%v", result, err)
	}
	want := []string{"manifest.create", "content.publish", "manifest.cas", "metadata.apply", "row.verify", "content.verify", "manifest.cas"}
	if strings.Join(events, ",") != strings.Join(want, ",") {
		t.Fatalf("events=%v want=%v", events, want)
	}
}

func TestCreateRepairQuarantinesThenReverifiesBeforeDiscard(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	lifecycle := &fakeCreateLifecycle{events: &events}
	rows := &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowAbsent}}
	service := CreateRepairService{
		Content: &fakeCreateContent{events: &events, verifyStatus: VerificationApplied}, Lifecycle: lifecycle,
		Manifests: manifests, Rows: rows, Now: func() time.Time { return now }, QuarantineDelay: time.Hour,
	}
	result, err := service.Execute(context.Background(), createRepairCommand(t, &events, errors.New("insert failed")))
	if errorCategoryOf(err) != ErrorPartialWrite || result.Status != CreateRepairPending || lifecycle.quarantines != 1 || lifecycle.purges != 0 {
		t.Fatalf("Execute() result=%+v err=%v lifecycle=%+v", result, err, lifecycle)
	}

	now = now.Add(2 * time.Hour)
	rows.statuses = []CreateRowStatus{CreateRowAbsent, CreateRowAbsent}
	recovery, err := service.RecoverAbandoned(context.Background(), 10)
	if err != nil || recovery.Discarded != 1 || lifecycle.purges != 1 {
		t.Fatalf("RecoverAbandoned() result=%+v err=%v lifecycle=%+v", recovery, err, lifecycle)
	}
	manifest := manifests.only(t)
	if manifest.Stage != CreateStageTerminal || manifest.Outcome != CreateOutcomeDiscarded {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestCreateRepairRestoresQuarantineWhenOwnerRowAppears(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	lifecycle := &fakeCreateLifecycle{events: &events}
	rows := &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowAbsent}}
	service := CreateRepairService{
		Content: &fakeCreateContent{events: &events, verifyStatus: VerificationApplied}, Lifecycle: lifecycle,
		Manifests: manifests, Rows: rows, Now: func() time.Time { return now },
	}
	_, _ = service.Execute(context.Background(), createRepairCommand(t, &events, errors.New("insert failed")))
	rows.statuses = []CreateRowStatus{CreateRowExact}
	recovery, err := service.RecoverAbandoned(context.Background(), 10)
	if err != nil || recovery.Committed != 1 || lifecycle.restores != 1 || lifecycle.purges != 0 {
		t.Fatalf("RecoverAbandoned() result=%+v err=%v lifecycle=%+v", recovery, err, lifecycle)
	}
}

func TestCreateRepairKeepsPreNoteAssetRepairableUntilCommit(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	service := CreateRepairService{
		Content:   &fakeCreateContent{events: &events, verifyStatus: VerificationApplied},
		Lifecycle: &fakeCreateLifecycle{events: &events}, Manifests: manifests,
		Rows: &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowExact, CreateRowExact}},
		Now:  func() time.Time { return now },
	}
	command := createRepairCommand(t, &events, nil)
	command.Identity.Kind = AssetAttachment
	command.Identity.ParentID = command.Identity.OwnerID
	command.Identity.Generation = 0
	command.Identity.PreNote = true

	result, err := service.Execute(context.Background(), command)
	if err != nil || result.Status != CreateRepairCommitted {
		t.Fatalf("Execute() result=%+v err=%v", result, err)
	}
	manifest := manifests.only(t)
	if manifest.Stage != CreateStagePublished || !manifest.PreNote {
		t.Fatalf("pre-note publish must remain repairable, manifest=%+v", manifest)
	}
	identity := PreNoteAssetIdentity{
		Action: command.Identity.Action, OwnerID: command.Identity.OwnerID, RecordOwnerID: command.Identity.RecordOwnerID,
		ParentID: command.Identity.ParentID, Kind: command.Identity.Kind, AssetID: command.Identity.AssetID,
	}
	if _, err := service.FinalizePreNote(context.Background(), identity); err != nil {
		t.Fatalf("FinalizePreNote() error=%v", err)
	}
	manifest = manifests.only(t)
	if manifest.Stage != CreateStageTerminal || manifest.Outcome != CreateOutcomeCommitted {
		t.Fatalf("finalized manifest=%+v", manifest)
	}
}

func TestCreateRepairDiscardsOnlyAnAbsentPreNoteAsset(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("asset"))
	identity := CreateIdentity{
		Action: "api_note_asset_upload", OwnerID: owner, RecordOwnerID: owner, ParentID: owner,
		Kind: AssetImage, AssetID: "507f1f77bcf86cd799439012", PreNote: true,
		ContentDigest: digest, RecordDigest: sha256.Sum256([]byte("row")), ContentSize: 5,
	}
	manifest, err := NewCreateManifest(identity, LogicalPath{Kind: RootPrivateFiles, Value: "owner/image.png"}, now)
	if err != nil {
		t.Fatal(err)
	}
	manifests.manifests[manifest.LookupKey] = manifest
	service := CreateRepairService{
		Content:   &fakeCreateContent{events: &events, verifyStatus: VerificationApplied},
		Lifecycle: &fakeCreateLifecycle{events: &events}, Manifests: manifests,
		Rows: &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowExact}},
		Now:  func() time.Time { return now },
	}
	lookup := PreNoteAssetIdentity{
		Action: identity.Action, OwnerID: owner, RecordOwnerID: owner, ParentID: owner,
		Kind: identity.Kind, AssetID: identity.AssetID,
	}
	if err := service.DiscardPreNote(context.Background(), lookup); errorCategoryOf(err) != ErrorConflict {
		t.Fatalf("DiscardPreNote() error=%v, want conflict while metadata remains", err)
	}
	if manifests.only(t).Stage != CreateStagePrepared {
		t.Fatalf("present pre-note asset must stay repairable")
	}

	service.Rows = &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowAbsent}}
	service.Content = &fakeCreateContent{events: &events, verifyStatus: VerificationNotApplied}
	if err := service.DiscardPreNote(context.Background(), lookup); err != nil {
		t.Fatalf("DiscardPreNote() error=%v", err)
	}
	if manifest := manifests.only(t); manifest.Stage != CreateStageTerminal || manifest.Outcome != CreateOutcomeDiscarded {
		t.Fatalf("discarded manifest=%+v", manifest)
	}
}

func TestCreateRepairDiscardPreNoteQuarantinesAndPurgesAnUnreferencedFile(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	events := []string{}
	manifests := newMemoryCreateManifests(&events)
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	digest := sha256.Sum256([]byte("asset"))
	identity := CreateIdentity{
		Action: "api_note_asset_upload", OwnerID: owner, RecordOwnerID: owner, ParentID: owner,
		Kind: AssetImage, AssetID: "507f1f77bcf86cd799439012", PreNote: true,
		ContentDigest: digest, RecordDigest: sha256.Sum256([]byte("row")), ContentSize: 5,
	}
	manifest, err := NewCreateManifest(identity, LogicalPath{Kind: RootPrivateFiles, Value: "owner/image.png"}, now)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = manifest.Published(now)
	if err != nil {
		t.Fatal(err)
	}
	manifests.manifests[manifest.LookupKey] = manifest
	lifecycle := &fakeCreateLifecycle{events: &events}
	service := CreateRepairService{
		Content: &fakeCreateContent{events: &events, verifyStatus: VerificationApplied}, Lifecycle: lifecycle,
		Manifests: manifests, Rows: &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowAbsent}},
		Now: func() time.Time { return now },
	}
	lookup := PreNoteAssetIdentity{Action: identity.Action, OwnerID: owner, RecordOwnerID: owner, ParentID: owner, Kind: identity.Kind, AssetID: identity.AssetID}
	if err := service.DiscardPreNote(context.Background(), lookup); err != nil {
		t.Fatalf("DiscardPreNote() error=%v", err)
	}
	if lifecycle.quarantines != 1 || lifecycle.purges != 1 {
		t.Fatalf("lifecycle=%+v", lifecycle)
	}
	if manifest := manifests.only(t); manifest.Stage != CreateStageTerminal || manifest.Outcome != CreateOutcomeDiscarded {
		t.Fatalf("discarded manifest=%+v", manifest)
	}
}

func TestRecoverAbandonedDoesNotSpendGenericBudgetOnPreNoteManifests(t *testing.T) {
	var events []string
	now := time.Unix(1_700_000_000, 0).UTC()
	command := createRepairCommand(t, &events, nil)
	ordinary, err := NewCreateManifest(command.Identity, command.Destination, now)
	if err != nil {
		t.Fatal(err)
	}
	preNoteIdentity := command.Identity
	preNoteIdentity.Action = "api_note_asset_upload"
	preNoteIdentity.ParentID = command.Identity.OwnerID
	preNoteIdentity.PreNote = true
	preNote, err := NewCreateManifest(preNoteIdentity, command.Destination, now)
	if err != nil {
		t.Fatal(err)
	}
	manifests := newMemoryCreateManifests(&events)
	manifests.manifests[ordinary.LookupKey] = ordinary
	manifests.manifests[preNote.LookupKey] = preNote
	store := &budgetIsolatedCreateManifests{memoryCreateManifests: manifests, preNote: preNote, ordinary: ordinary}
	service := CreateRepairService{
		Content: &fakeCreateContent{events: &events, verifyStatus: VerificationApplied}, Lifecycle: &fakeCreateLifecycle{events: &events},
		Manifests: store, Rows: &fakeCreateRows{events: &events, statuses: []CreateRowStatus{CreateRowExact}}, Now: func() time.Time { return now },
	}

	result, err := service.RecoverAbandoned(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Committed != 1 || result.Pending != 0 || result.Discarded != 0 {
		t.Fatalf("recovery result=%+v", result)
	}
	if store.nonPreNoteCalls != 1 {
		t.Fatalf("non-pre-note scan calls=%d", store.nonPreNoteCalls)
	}
}

func createRepairCommand(t *testing.T, events *[]string, applyErr error) CreateRepairCommand {
	t.Helper()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("image"))
	return CreateRepairCommand{
		Identity:    CreateIdentity{Action: "web_image_upload", OwnerID: owner, RecordOwnerID: owner, Kind: AssetImage, AssetID: "507f1f77bcf86cd799439012", ContentDigest: digest, RecordDigest: sha256.Sum256([]byte("row")), ContentSize: 5},
		Destination: LogicalPath{Kind: RootPrivateFiles, Value: "owner/image.png"}, Source: strings.NewReader("image"),
		ApplyMetadata: func(context.Context) error {
			*events = append(*events, "metadata.apply")
			return applyErr
		},
	}
}

type memoryCreateManifests struct {
	events    *[]string
	manifests map[string]CreateManifest
}

func newMemoryCreateManifests(events *[]string) *memoryCreateManifests {
	return &memoryCreateManifests{events: events, manifests: map[string]CreateManifest{}}
}

func (store *memoryCreateManifests) CreateCreateManifest(_ context.Context, manifest CreateManifest) error {
	*store.events = append(*store.events, "manifest.create")
	if _, found := store.manifests[manifest.LookupKey]; found {
		return conflictError("exists", nil)
	}
	store.manifests[manifest.LookupKey] = manifest
	return nil
}

func (store *memoryCreateManifests) LoadCreateManifest(_ context.Context, key string) (CreateManifest, bool, error) {
	manifest, found := store.manifests[key]
	return manifest, found, nil
}

func (store *memoryCreateManifests) CompareAndSwapCreateManifest(_ context.Context, key string, version uint64, digest [sha256.Size]byte, next CreateManifest) error {
	*store.events = append(*store.events, "manifest.cas")
	current, found := store.manifests[key]
	if !found || current.Version != version || current.StateDigest != digest {
		return conflictError("stale", nil)
	}
	store.manifests[key] = next
	return nil
}

func (store *memoryCreateManifests) ListActiveCreateManifests(_ context.Context, limit int) ([]CreateManifest, bool, error) {
	result := []CreateManifest{}
	for _, manifest := range store.manifests {
		if manifest.Stage != CreateStageTerminal {
			result = append(result, manifest)
		}
	}
	return result, len(result) > limit, nil
}

func (store *memoryCreateManifests) ListActiveNonPreNoteCreateManifests(_ context.Context, limit int) ([]CreateManifest, bool, error) {
	result := []CreateManifest{}
	for _, manifest := range store.manifests {
		if manifest.Stage != CreateStageTerminal && !manifest.PreNote {
			result = append(result, manifest)
		}
	}
	return result, len(result) > limit, nil
}

type budgetIsolatedCreateManifests struct {
	*memoryCreateManifests
	preNote         CreateManifest
	ordinary        CreateManifest
	nonPreNoteCalls int
}

func (store *budgetIsolatedCreateManifests) ListActiveCreateManifests(_ context.Context, _ int) ([]CreateManifest, bool, error) {
	return []CreateManifest{store.preNote}, true, nil
}

func (store *budgetIsolatedCreateManifests) ListActiveNonPreNoteCreateManifests(_ context.Context, _ int) ([]CreateManifest, bool, error) {
	store.nonPreNoteCalls++
	return []CreateManifest{store.ordinary}, false, nil
}

func (store *memoryCreateManifests) only(t *testing.T) CreateManifest {
	t.Helper()
	for _, manifest := range store.manifests {
		return manifest
	}
	t.Fatal("manifest missing")
	return CreateManifest{}
}

type fakeCreateContent struct {
	events       *[]string
	verifyStatus VerificationStatus
}

func (content *fakeCreateContent) Open(context.Context, LogicalPath) (OpenResult, error) {
	return OpenResult{}, errors.New("not implemented")
}

func (content *fakeCreateContent) Publish(_ context.Context, request PublishRequest) (PublishResult, error) {
	*content.events = append(*content.events, "content.publish")
	_, _ = io.Copy(io.Discard, request.Source)
	return PublishResult{Status: PublishApplied, Destination: request.Destination, Digest: request.Identity.Digest}, nil
}

func (content *fakeCreateContent) Verify(context.Context, VerifyRequest) (VerifyResult, error) {
	*content.events = append(*content.events, "content.verify")
	return VerifyResult{Status: content.verifyStatus}, nil
}

type fakeCreateRows struct {
	events   *[]string
	statuses []CreateRowStatus
}

func (rows *fakeCreateRows) VerifyCreateRow(context.Context, CreateManifest) (CreateRowStatus, error) {
	*rows.events = append(*rows.events, "row.verify")
	if len(rows.statuses) == 0 {
		return CreateRowAbsent, nil
	}
	status := rows.statuses[0]
	rows.statuses = rows.statuses[1:]
	return status, nil
}

type fakeCreateLifecycle struct {
	events      *[]string
	quarantines int
	restores    int
	purges      int
}

func (lifecycle *fakeCreateLifecycle) Quarantine(_ context.Context, source LogicalPath, _ string, _ [sha256.Size]byte, _ int64) (LogicalPath, error) {
	*lifecycle.events = append(*lifecycle.events, "lifecycle.quarantine")
	lifecycle.quarantines++
	return LogicalPath{Kind: source.Kind, Value: "objects/aa/asset"}, nil
}

func (lifecycle *fakeCreateLifecycle) Restore(context.Context, LogicalPath, LogicalPath, [sha256.Size]byte, int64) error {
	*lifecycle.events = append(*lifecycle.events, "lifecycle.restore")
	lifecycle.restores++
	return nil
}

func (lifecycle *fakeCreateLifecycle) Purge(context.Context, LogicalPath, [sha256.Size]byte, int64) error {
	*lifecycle.events = append(*lifecycle.events, "lifecycle.purge")
	lifecycle.purges++
	return nil
}
