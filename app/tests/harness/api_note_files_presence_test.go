package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestFilesPresenceFixtureDirectoryIsUniqueAndBounded(t *testing.T) {
	repoRoot := t.TempDir()
	filesRoot := filepath.Join(repoRoot, "files")
	if err := os.Mkdir(filesRoot, 0o755); err != nil {
		t.Fatalf("create files root: %v", err)
	}

	first := newFilesPresenceFixtureDirectory(t, repoRoot)
	second := newFilesPresenceFixtureDirectory(t, repoRoot)
	if first == second {
		t.Fatalf("fixture directories must be unique, both were %q", first)
	}
	for _, directory := range []string{first, second} {
		relative, err := filepath.Rel(filesRoot, directory)
		if err != nil {
			t.Fatalf("resolve fixture directory relative to files root: %v", err)
		}
		if relative == "." || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
			t.Fatalf("fixture directory %q escapes files root %q", directory, filesRoot)
		}
	}
}

func TestRemoveFilesPresenceFixtureDirectoryRejectsOutsideTarget(t *testing.T) {
	repoRoot := t.TempDir()
	filesRoot := filepath.Join(repoRoot, "files")
	outside := filepath.Join(repoRoot, "must-survive")
	if err := os.Mkdir(filesRoot, 0o755); err != nil {
		t.Fatalf("create files root: %v", err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}

	if err := removeFilesPresenceFixtureDirectory(filesRoot, outside); err == nil {
		t.Fatal("remove outside fixture directory succeeded, want boundary error")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside directory was removed: %v", err)
	}
}

func TestRemoveFilesPresenceFixtureDirectoryRejectsUnexpectedFile(t *testing.T) {
	repoRoot := t.TempDir()
	filesRoot := filepath.Join(repoRoot, "files")
	if err := os.Mkdir(filesRoot, 0o755); err != nil {
		t.Fatalf("create files root: %v", err)
	}
	directory, err := os.MkdirTemp(filesRoot, "test_files_presence-")
	if err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	unexpectedPath := filepath.Join(directory, "unexpected.txt")
	if err := os.WriteFile(unexpectedPath, []byte("must survive cleanup\n"), 0o644); err != nil {
		t.Fatalf("write unexpected file: %v", err)
	}

	if err := removeFilesPresenceFixtureDirectory(filesRoot, directory); err == nil {
		t.Fatal("remove non-empty fixture directory succeeded, want explicit error")
	}
	if _, err := os.Stat(unexpectedPath); err != nil {
		t.Fatalf("unexpected file was removed: %v", err)
	}
	if err := os.Remove(unexpectedPath); err != nil {
		t.Fatalf("remove unexpected test file: %v", err)
	}
	if err := os.Remove(directory); err != nil {
		t.Fatalf("remove empty fixture directory: %v", err)
	}
}

func newFilesPresenceFixtureDirectory(t testing.TB, repoRoot string) string {
	t.Helper()
	filesRoot := filepath.Join(repoRoot, "files")
	resolvedRoot, err := filepath.EvalSymlinks(filesRoot)
	if err != nil {
		t.Fatalf("resolve files root: %v", err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		t.Fatalf("make files root absolute: %v", err)
	}

	directory, err := os.MkdirTemp(resolvedRoot, "test_files_presence-")
	if err != nil {
		t.Fatalf("create files-presence fixture directory: %v", err)
	}
	if err := validateFilesPresenceFixtureDirectory(resolvedRoot, directory); err != nil {
		_ = os.Remove(directory)
		t.Fatalf("validate files-presence fixture directory: %v", err)
	}
	t.Cleanup(func() {
		if err := removeFilesPresenceFixtureDirectory(resolvedRoot, directory); err != nil {
			t.Errorf("remove files-presence fixture directory: %v", err)
		}
	})
	return directory
}

func removeFilesPresenceFixtureDirectory(filesRoot, directory string) error {
	resolvedRoot, err := filepath.EvalSymlinks(filesRoot)
	if err != nil {
		return fmt.Errorf("resolve files root: %w", err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return fmt.Errorf("make files root absolute: %w", err)
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("make fixture directory absolute: %w", err)
	}
	if err := validateFilesPresenceFixtureDirectory(resolvedRoot, directory); err != nil {
		return err
	}

	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("resolve fixture directory: %w", err)
	}
	if err := validateFilesPresenceFixtureDirectory(resolvedRoot, resolvedDirectory); err != nil {
		return err
	}
	if err := os.Remove(directory); err != nil {
		return fmt.Errorf("remove empty fixture directory: %w", err)
	}
	return nil
}

func registerFilesPresenceFixtureFileCleanup(t testing.TB, directory, path string) {
	t.Helper()
	t.Cleanup(func() {
		if err := removeFilesPresenceFixtureFile(directory, path); err != nil {
			t.Errorf("remove files-presence fixture file: %v", err)
		}
	})
}

func removeFilesPresenceFixtureFile(directory, path string) error {
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("resolve fixture directory: %w", err)
	}
	resolvedDirectory, err = filepath.Abs(resolvedDirectory)
	if err != nil {
		return fmt.Errorf("make fixture directory absolute: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("make fixture file absolute: %w", err)
	}
	if err := validateFilesPresenceFixtureDirectory(resolvedDirectory, path); err != nil {
		return fmt.Errorf("validate fixture file: %w", err)
	}

	fileInfo, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect fixture file: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("fixture file %q is not a regular file", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove fixture file: %w", err)
	}
	return nil
}

func validateFilesPresenceFixtureDirectory(filesRoot, directory string) error {
	relative, err := filepath.Rel(filesRoot, directory)
	if err != nil {
		return fmt.Errorf("resolve fixture directory relative to files root: %w", err)
	}
	if relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("fixture directory %q is outside files root %q", directory, filesRoot)
	}
	return nil
}

func TestAPIUpdateNoteFilesPresenceContract(t *testing.T) {
	_, client, repoRoot := startBaselineServer(t)
	database := fixtureDatabase(t)
	noteID := db.MustObjectIDFromHex(fixtureActiveNoteID)
	ownerID := db.MustObjectIDFromHex(fixtureAdminID)
	attachID := db.NewObjectID()
	obsoleteAttachID := db.NewObjectID()
	attachDir := newFilesPresenceFixtureDirectory(t, repoRoot)
	attachPath := filepath.Join(attachDir, "attach.txt")
	obsoleteAttachPath := filepath.Join(attachDir, "obsolete.txt")
	attachRelativePath, err := filepath.Rel(repoRoot, attachPath)
	if err != nil {
		t.Fatalf("resolve fixture attachment path: %v", err)
	}
	obsoleteAttachRelativePath, err := filepath.Rel(repoRoot, obsoleteAttachPath)
	if err != nil {
		t.Fatalf("resolve obsolete fixture attachment path: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := harnessContext()
		defer cancel()
		_, _ = database.Collection("attachs").DeleteMany(ctx, bson.M{"_id": bson.M{"$in": []domain.ObjectID{attachID, obsoleteAttachID}}})
	})
	contents := []byte("files presence contract attachment\n")
	if err := os.WriteFile(attachPath, contents, 0o644); err != nil {
		t.Fatalf("write files-presence fixture attachment: %v", err)
	}
	registerFilesPresenceFixtureFileCleanup(t, attachDir, attachPath)
	if err := os.WriteFile(obsoleteAttachPath, []byte("obsolete attachment\n"), 0o644); err != nil {
		t.Fatalf("write obsolete files-presence fixture attachment: %v", err)
	}
	registerFilesPresenceFixtureFileCleanup(t, attachDir, obsoleteAttachPath)

	ctx, cancel := harnessContext()
	defer cancel()
	if _, err := database.Collection("attachs").InsertOne(ctx, info.Attach{
		AttachId:     attachID,
		NoteId:       noteID,
		UploadUserId: ownerID,
		Name:         "attach.txt",
		Title:        "attach.txt",
		Path:         filepath.ToSlash(attachRelativePath),
		Type:         "txt",
		Size:         int64(len(contents)),
		CreatedTime:  time.Date(2015, 1, 20, 11, 13, 41, 0, time.FixedZone("CST", 8*60*60)),
	}); err != nil {
		t.Fatalf("insert files-presence fixture attachment: %v", err)
	}
	if _, err := database.Collection("attachs").InsertOne(ctx, info.Attach{
		AttachId:     obsoleteAttachID,
		NoteId:       noteID,
		UploadUserId: ownerID,
		Name:         "obsolete.txt",
		Title:        "obsolete.txt",
		Path:         filepath.ToSlash(obsoleteAttachRelativePath),
		Type:         "txt",
		Size:         int64(len("obsolete attachment\n")),
		CreatedTime:  time.Date(2015, 1, 20, 11, 13, 42, 0, time.FixedZone("CST", 8*60*60)),
	}); err != nil {
		t.Fatalf("insert obsolete files-presence fixture attachment: %v", err)
	}
	result, err := database.Collection("notes").UpdateOne(ctx,
		bson.M{"_id": noteID, "UserId": ownerID},
		bson.M{"$set": bson.M{"AttachNum": 2}},
	)
	if err != nil {
		t.Fatalf("align fixture attachment count: %v", err)
	}
	if result.MatchedCount != 1 {
		t.Fatalf("align fixture attachment count matched %d notes, want 1", result.MatchedCount)
	}

	usn := fixtureNoteUSN(t, database, noteID, ownerID)
	usn = updateAPIFileSet(t, client, usn, "files absent preserves assets", map[string][]string{})
	assertFixtureAttachmentSet(t, database, noteID, ownerID, attachID, obsoleteAttachID)

	usn = updateAPIFileSet(t, client, usn, "disabled markers and orphan file data preserve assets", map[string][]string{
		"FilesPresent":              {"0"},
		"HasFiles":                  {"0"},
		"FileDatas[orphan-file-id]": {"not a files collection"},
	})
	assertFixtureAttachmentSet(t, database, noteID, ownerID, attachID, obsoleteAttachID)

	usn = updateAPIFileSet(t, client, usn, "non-empty files replace the complete set", map[string][]string{
		"Files[0][FileId]":   {attachID.Hex()},
		"Files[0][IsAttach]": {"true"},
		"Files[0][HasBody]":  {"false"},
		"Files[0][Title]":    {"attach.txt"},
		"Files[0][Type]":     {"txt"},
	})
	assertFixtureAttachmentSet(t, database, noteID, ownerID, attachID)

	_ = updateAPIFileSet(t, client, usn, "explicit empty files clears assets", map[string][]string{
		"FilesPresent": {"1"},
	})
	assertFixtureAttachmentSet(t, database, noteID, ownerID)
}

func updateAPIFileSet(t testing.TB, client *Client, usn int, title string, files map[string][]string) int {
	t.Helper()
	form := map[string][]string{
		"NoteId": {fixtureActiveNoteID},
		"Title":  {title},
		"Usn":    {fmt.Sprintf("%d", usn)},
	}
	for key, values := range files {
		form[key] = values
	}
	status, headers, body, err := client.doRaw(RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/note/updateNote",
		Form:   form,
		Auth:   "admin",
	})
	if err != nil {
		t.Fatalf("update API note files: %v", err)
	}
	if status != http.StatusOK || headers.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("update API note files response: status=%d content-type=%q body=%s", status, headers.Get("Content-Type"), body)
	}
	var response struct {
		Usn int
		Msg string
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode update API note files response %s: %v", body, err)
	}
	if response.Msg != "" || response.Usn <= usn {
		t.Fatalf("update API note files response = %s, want committed Usn > %d", body, usn)
	}
	return response.Usn
}

func fixtureNoteUSN(t testing.TB, database *mongo.Database, noteID, ownerID domain.ObjectID) int {
	t.Helper()
	ctx, cancel := harnessContext()
	defer cancel()
	var note info.Note
	if err := database.Collection("notes").FindOne(ctx, bson.M{"_id": noteID, "UserId": ownerID}).Decode(&note); err != nil {
		t.Fatalf("read fixture note generation: %v", err)
	}
	return note.Usn
}

func assertFixtureAttachmentSet(t testing.TB, database *mongo.Database, noteID, ownerID domain.ObjectID, want ...domain.ObjectID) {
	t.Helper()
	ctx, cancel := harnessContext()
	defer cancel()
	cursor, err := database.Collection("attachs").Find(ctx, bson.M{
		"NoteId":       noteID,
		"UploadUserId": ownerID,
	})
	if err != nil {
		t.Fatalf("find fixture attachments: %v", err)
	}
	defer cursor.Close(ctx)
	var attachments []info.Attach
	if err := cursor.All(ctx, &attachments); err != nil {
		t.Fatalf("read fixture attachments: %v", err)
	}
	wantIDs := make(map[string]struct{}, len(want))
	for _, attachID := range want {
		wantIDs[attachID.Hex()] = struct{}{}
	}
	if len(attachments) != len(wantIDs) {
		t.Fatalf("fixture attachment count = %d, want %d", len(attachments), len(wantIDs))
	}
	for _, attachment := range attachments {
		if _, ok := wantIDs[attachment.AttachId.Hex()]; !ok {
			t.Fatalf("unexpected fixture attachment %s", attachment.AttachId.Hex())
		}
	}
	var note info.Note
	if err := database.Collection("notes").FindOne(ctx, bson.M{"_id": noteID, "UserId": ownerID}).Decode(&note); err != nil {
		t.Fatalf("read fixture note attachment count: %v", err)
	}
	if note.AttachNum != len(wantIDs) {
		t.Fatalf("fixture note attachment count = %d, want %d", note.AttachNum, len(wantIDs))
	}
}
