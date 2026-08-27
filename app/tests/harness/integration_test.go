package harness

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/yangphere/leanote/app/info"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	fixtureAdminID      = "5368c1aa99c37b029d000001"
	fixtureDemoID       = "540817e099c37b583c000001"
	fixtureNotebookID   = "548125adf4e872105c000007"
	fixtureActiveNoteID = "5483207cf4e87203a4000001"
	fixtureTrashNoteID  = "5481481bf4e87273d2000003"
	fixtureDatabaseURL  = "mongodb://127.0.0.1:27017/leanote_test"
	fixtureDatabaseName = "leanote_test"
	seedImageID         = "650000000000000000000001"
	seedAttachID        = "650000000000000000000002"
)

var configEnvironmentPattern = regexp.MustCompile(`\$\{([a-zA-Z0-9_.-]+)}`)

func startBaselineServer(t testing.TB) (*Server, *Client, string) {
	t.Helper()
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertTestConfiguration(repoRoot); err != nil {
		t.Fatal(err)
	}
	environment := NewMongoEnvironment(repoRoot)
	if err := environment.Up(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Down(); err != nil {
			t.Error(err)
		}
	})

	server := StartServer(t)
	client := NewClient(server.BaseURL)
	if err := client.Login("admin", "admin@leanote.com", "abc123"); err != nil {
		t.Fatalf("login fixture admin: %v", err)
	}
	return server, client, repoRoot
}

func assertTestConfiguration(repoRoot string) error {
	contents, err := os.ReadFile(filepath.Join(repoRoot, "conf", "app.conf"))
	if err != nil {
		return fmt.Errorf("read test configuration: %w", err)
	}
	sections := map[string]map[string]string{"": {}}
	section := ""
	option := ""
	for lineNumber, rawLine := range strings.Split(string(contents), "\n") {
		// Keep leading whitespace until continuation handling. robfig/config
		// treats an indented line as part of the preceding option value.
		rawLine = strings.TrimSuffix(rawLine, "\r")
		indented := len(rawLine) > 0 && (rawLine[0] == ' ' || rawLine[0] == '\t')
		line := stripInlineComment(rawLine)
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, ";") {
			continue
		}
		if indented {
			if section == "" || option == "" {
				return fmt.Errorf("conf/app.conf line %d: continuation without an active section and option", lineNumber+1)
			}
			sections[section][option] += "\n" + strings.TrimSpace(line)
			continue
		}
		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
			section = strings.TrimSpace(trimmedLine[1 : len(trimmedLine)-1])
			if _, ok := sections[section]; !ok {
				sections[section] = map[string]string{}
			}
			option = ""
			continue
		}
		key, value, ok := splitConfigLine(trimmedLine)
		if !ok {
			continue
		}
		sections[section][key] = value
		option = key
	}

	testValues := sections["test"]
	for _, expected := range []struct {
		key   string
		value string
	}{
		{key: "http.addr", value: "127.0.0.1"},
		{key: "db.dbname", value: "leanote_test"},
		{key: "site.url", value: fmt.Sprintf("http://127.0.0.1:%d", TestPort)},
	} {
		if actual := testValues[expected.key]; actual != expected.value {
			return fmt.Errorf("conf/app.conf [test] %s = %q, want %q", expected.key, actual, expected.value)
		}
	}

	// db.Init checks db.url and db.urlEnv before db.dbname. Reject an active
	// global or test override unless it resolves to the isolated fixture DB.
	for _, key := range []string{"db.url", "db.urlEnv"} {
		value, ok := effectiveConfigValue(sections, "test", key)
		if !ok {
			continue
		}
		value, err = resolveDatabaseConfigValue(value)
		if err != nil {
			return fmt.Errorf("conf/app.conf %s: %w", key, err)
		}
		dialInfo, parseErr := mgo.ParseURL(value)
		initDatabase := databaseNameFromInit(value)
		if initDatabase == "" {
			// db.Init falls back to db.dbname when the URL has no database
			// segment. That value was already required to be leanote_test above.
			initDatabase = testValues["db.dbname"]
		}
		if initDatabase != fixtureDatabaseName {
			if parseErr != nil {
				return fmt.Errorf("conf/app.conf %s: db.Init selects database %q (want %q); mgo.ParseURL also failed: %v", key, initDatabase, fixtureDatabaseName, parseErr)
			}
			return fmt.Errorf("conf/app.conf %s: db.Init selects database %q (want %q); mgo.ParseURL selects %q", key, initDatabase, fixtureDatabaseName, dialInfo.Database)
		}
		// mgo.ParseURL rejects options unknown to its old allow-list (for
		// example tlsCAFile), while db.Init still uses the URL's final path
		// segment. A safe db.Init result is therefore accepted when parsing
		// fails, but a successful parser result must also target the fixture.
		if parseErr == nil && dialInfo.Database != "" && dialInfo.Database != fixtureDatabaseName {
			return fmt.Errorf("conf/app.conf %s: mgo.ParseURL selects database %q (want %q); db.Init selects %q", key, dialInfo.Database, fixtureDatabaseName, initDatabase)
		}
	}
	return nil
}

func databaseNameFromInit(value string) string {
	parts := strings.Split(value, "/")
	database := parts[len(parts)-1]
	if query := strings.Index(database, "?"); query >= 0 {
		database = database[:query]
	}
	return database
}

func resolveDatabaseConfigValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("is enabled but empty")
	}
	matches := configEnvironmentPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) > 0 {
		var expanded strings.Builder
		cursor := 0
		for _, match := range matches {
			name := value[match[2]:match[3]]
			environmentValue, ok := os.LookupEnv(name)
			if !ok || strings.TrimSpace(environmentValue) == "" {
				return "", fmt.Errorf("is enabled but environment variable %q is empty", name)
			}
			expanded.WriteString(value[cursor:match[0]])
			expanded.WriteString(environmentValue)
			cursor = match[1]
		}
		expanded.WriteString(value[cursor:])
		value = strings.TrimSpace(expanded.String())
	}
	if strings.Contains(value, "${") {
		return "", fmt.Errorf("contains an unresolved environment expression")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("must be a single-line Mongo URL; continuation is not supported")
	}
	return value, nil
}

func splitConfigLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	separator := strings.IndexAny(line, "=:")
	if separator <= 0 {
		return "", "", false
	}
	value := stripInlineComment(line[separator+1:])
	return strings.TrimSpace(line[:separator]), value, true
}

func stripInlineComment(value string) string {
	var quote rune
	escaped := false
	previousWhitespace := false
	for index, character := range value {
		if escaped {
			escaped = false
			previousWhitespace = false
			continue
		}
		if character == '\\' {
			escaped = true
			previousWhitespace = false
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			previousWhitespace = unicode.IsSpace(character)
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			previousWhitespace = false
			continue
		}
		if previousWhitespace && (character == '#' || character == ';') {
			return strings.TrimSpace(value[:index])
		}
		previousWhitespace = unicode.IsSpace(character)
	}
	return strings.TrimSpace(value)
}

func effectiveConfigValue(sections map[string]map[string]string, section, key string) (string, bool) {
	if values, ok := sections[section]; ok {
		if value, exists := values[key]; exists {
			return value, true
		}
	}
	value, ok := sections[""][key]
	return value, ok
}

func goldenStoreForTest(t testing.TB, repoRoot string) GoldenStore {
	t.Helper()
	mode, err := GoldenModeFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	return GoldenStore{Mode: mode, Root: filepath.Join(repoRoot, "app", "tests", "golden")}
}

func captureGolden(t testing.TB, store GoldenStore, client *Client, name string, request RequestSpec) Snapshot {
	t.Helper()
	snapshot, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s: execute %s %s: %v", name, request.Method, request.Path, err)
	}
	if request.Binary && (snapshot.Headers["Content-Type"] == "" || len(snapshot.Body) == 0) {
		t.Fatalf("%s: binary response must have Content-Type and a non-empty body: %#v", name, snapshot)
	}
	if err := store.AssertCase(name, request, snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func fixtureSession(t testing.TB) *mgo.Session {
	t.Helper()
	session, err := mgo.DialWithTimeout(fixtureDatabaseURL, 5*time.Second)
	if err != nil {
		t.Fatalf("connect fixture database: %v", err)
	}
	t.Cleanup(session.Close)
	return session
}

func fixtureNotebookByTitle(t testing.TB, title string) (string, int) {
	t.Helper()
	var notebook info.Notebook
	err := fixtureSession(t).DB(fixtureDatabaseName).C("notebooks").Find(bson.M{
		"UserId": bson.ObjectIdHex(fixtureAdminID),
		"Title":  title,
	}).One(&notebook)
	if err != nil {
		t.Fatalf("find notebook %q: %v", title, err)
	}
	return notebook.NotebookId.Hex(), notebook.Usn
}

func fixtureNoteByTitle(t testing.TB, title string) (string, int) {
	t.Helper()
	var note info.Note
	err := fixtureSession(t).DB(fixtureDatabaseName).C("notes").Find(bson.M{
		"UserId": bson.ObjectIdHex(fixtureAdminID),
		"Title":  title,
	}).One(&note)
	if err != nil {
		t.Fatalf("find note %q: %v", title, err)
	}
	return note.NoteId.Hex(), note.Usn
}

func fixtureTagByName(t testing.TB, tag string) int {
	t.Helper()
	var noteTag info.NoteTag
	err := fixtureSession(t).DB(fixtureDatabaseName).C("note_tags").Find(bson.M{
		"UserId": bson.ObjectIdHex(fixtureAdminID),
		"Tag":    tag,
	}).One(&noteTag)
	if err != nil {
		t.Fatalf("find tag %q: %v", tag, err)
	}
	return noteTag.Usn
}

func seedBinaryFiles(t testing.TB, repoRoot string) {
	t.Helper()
	seedDir := filepath.Join(repoRoot, "files", "test_seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("create binary seed directory: %v", err)
	}
	// Register cleanup before writing either seed so partial setup failures do
	// not leave files in the worktree.
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(repoRoot, "files", "About Leanote.tar.gz"))
		_ = os.RemoveAll(seedDir)
	})
	imagePath := filepath.Join(seedDir, "image.png")
	attachPath := filepath.Join(seedDir, "attach.txt")
	if err := os.WriteFile(imagePath, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, 0o644); err != nil {
		t.Fatalf("write image seed: %v", err)
	}
	if err := os.WriteFile(attachPath, []byte("leanote regression attachment\n"), 0o644); err != nil {
		t.Fatalf("write attachment seed: %v", err)
	}

	session := fixtureSession(t)
	database := session.DB(fixtureDatabaseName)
	// Register cleanup before touching Mongo so a partial seed failure cannot
	// leave files or records behind for a later test.
	t.Cleanup(func() {
		_, _ = database.C("files").RemoveAll(bson.M{"_id": bson.ObjectIdHex(seedImageID)})
		_, _ = database.C("attachs").RemoveAll(bson.M{"_id": bson.ObjectIdHex(seedAttachID)})
	})
	if err := database.C("files").Insert(info.File{
		FileId:      bson.ObjectIdHex(seedImageID),
		UserId:      bson.ObjectIdHex(fixtureAdminID),
		AlbumId:     bson.ObjectIdHex("52d3e8ac99c37b7f0d000001"),
		Name:        "image.png",
		Title:       "image.png",
		Path:        "files/test_seed/image.png",
		Size:        8,
		CreatedTime: time.Date(2015, 1, 20, 11, 13, 41, 0, time.FixedZone("CST", 8*60*60)),
	}); err != nil {
		t.Fatalf("insert image seed: %v", err)
	}
	if err := database.C("attachs").Insert(info.Attach{
		AttachId:     bson.ObjectIdHex(seedAttachID),
		NoteId:       bson.ObjectIdHex(fixtureActiveNoteID),
		UploadUserId: bson.ObjectIdHex(fixtureAdminID),
		Name:         "attach.txt",
		Title:        "attach.txt",
		Path:         "files/test_seed/attach.txt",
		Type:         "txt",
		Size:         int64(len("leanote regression attachment\n")),
		CreatedTime:  time.Date(2015, 1, 20, 11, 13, 41, 0, time.FixedZone("CST", 8*60*60)),
	}); err != nil {
		t.Fatalf("insert attachment seed: %v", err)
	}
}

func removeGeneratedAdminLogo(t testing.TB, repoRoot string) {
	t.Helper()
	var user info.User
	err := fixtureSession(t).DB(fixtureDatabaseName).C("users").FindId(bson.ObjectIdHex(fixtureAdminID)).One(&user)
	if err != nil || user.Logo == "" {
		return
	}
	logoURL, err := url.Parse(user.Logo)
	if err != nil || logoURL.Path == "" || !strings.HasPrefix(logoURL.Path, "/public/upload/") {
		return
	}
	_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(strings.TrimPrefix(logoURL.Path, "/"))))
}

func describeSnapshot(snapshot Snapshot) string {
	return fmt.Sprintf("status=%d headers=%v body=%q", snapshot.Status, snapshot.Headers, snapshot.Body)
}
