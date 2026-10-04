package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func RegisterNotesContentHTTP(rs *httpserver.Registry, before httpserver.BeforeFunc) {
	n := &NotesContentHTTPServer{}
	for _, name := range []string{"GetSyncNotes", "GetNotes", "GetTrashNotes", "GetNote", "GetNoteAndContent", "GetNoteContent", "AddNote", "UpdateNote", "DeleteTrash", "GetHistories", "ExportPdf"} {
		rs.Register("ApiNote", name, []httpserver.BeforeFunc{before}, n.dispatch)
	}
	for _, name := range []string{"GetSyncNotebooks", "GetNotebooks", "AddNotebook", "UpdateNotebook", "DeleteNotebook"} {
		rs.Register("ApiNotebook", name, []httpserver.BeforeFunc{before}, n.dispatch)
	}
	for _, name := range []string{"GetImage", "GetAttach", "GetAllAttachs"} {
		rs.RegisterMethods("ApiFile", name, []string{"GET"}, []httpserver.BeforeFunc{before}, n.dispatch)
	}
}

type NotesContentHTTPServer struct{}

// apiPage preserves the legacy API controller's GetPage default. A missing,
// malformed or non-positive page starts at page one; passing zero through to
// NoteService would produce a negative Mongo skip.
func apiPage(c *httpserver.Context) int {
	page := c.Params.Int("page", 1)
	if page < 1 {
		return 1
	}
	return page
}

func nonEmptyAPIMessage(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "saveFailed"
	}
	return msg
}

func workspaceAPIMessage(category service.WorkspaceErrorCategory) string {
	switch category {
	case service.WorkspaceNotFound:
		return "notExists"
	case service.WorkspaceUnauthorized:
		return "noAuth"
	default:
		return string(category)
	}
}

func bindAPINote(c *httpserver.Context) info.ApiNote {
	note := info.ApiNote{
		NoteId: c.Params.String("NoteId"), NotebookId: c.Params.String("NotebookId"), UserId: c.Params.String("UserId"),
		Title: c.Params.String("Title"), Desc: c.Params.String("Desc"), Abstract: c.Params.String("Abstract"),
		Content: c.Params.String("Content"), IsMarkdown: c.Params.Bool("IsMarkdown", false),
		IsBlog: c.Params.Bool("IsBlog", false), IsTrash: c.Params.Bool("IsTrash", false), IsDeleted: c.Params.Bool("IsDeleted", false),
		Tags: c.Params.Strings("Tags"),
	}
	if len(note.Tags) == 0 {
		note.Tags = c.Params.Strings("Tags[]")
	}
	if len(note.Tags) == 0 {
		for index := 0; ; index++ {
			value, ok := c.Params.Get("Tags[" + strconv.Itoa(index) + "]")
			if !ok {
				break
			}
			note.Tags = append(note.Tags, value)
		}
	}
	if value := c.Params.String("Usn"); value != "" {
		note.Usn = c.Params.Int("Usn", 0)
	}
	note.Files = bindAPINoteFiles(c)
	return note
}

var apiNoteFileFieldPattern = regexp.MustCompile(`^Files\[(\d+)\]\[(FileId|LocalFileId|Type|Title|HasBody|IsAttach)\]$`)

func bindAPINoteFiles(c *httpserver.Context) []info.NoteFile {
	if c == nil || c.Request == nil {
		return nil
	}
	_ = c.Request.ParseMultipartForm(32 << 20)
	max := -1
	for key := range c.Request.Form {
		match := apiNoteFileFieldPattern.FindStringSubmatch(key)
		if match == nil {
			continue
		}
		index, err := strconv.Atoi(match[1])
		if err == nil && index > max {
			max = index
		}
	}
	if max < 0 {
		return nil
	}
	files := make([]info.NoteFile, max+1)
	for index := range files {
		prefix := fmt.Sprintf("Files[%d]", index)
		files[index] = info.NoteFile{
			FileId: c.Params.String(prefix + "[FileId]"), LocalFileId: c.Params.String(prefix + "[LocalFileId]"),
			Type: c.Params.String(prefix + "[Type]"), Title: c.Params.String(prefix + "[Title]"),
			HasBody: c.Params.Bool(prefix+"[HasBody]", false), IsAttach: c.Params.Bool(prefix+"[IsAttach]", false),
		}
	}
	return files
}

func apiNoteAssetsSupplied(c *httpserver.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	_ = c.Request.ParseMultipartForm(32 << 20)
	for key := range c.Request.Form {
		if key == "Files" || strings.HasPrefix(key, "Files[") {
			return true
		}
	}
	for _, marker := range []string{"FilesPresent", "HasFiles"} {
		for _, value := range c.Request.Form[marker] {
			if strings.TrimSpace(value) == "1" {
				return true
			}
		}
	}
	// FileDatas carries the binary payload in multipart file parts. A plain
	// form field with that prefix is metadata from older clients and must not
	// make an update look like a complete asset replacement.
	if c.Request.MultipartForm != nil {
		for key, files := range c.Request.MultipartForm.File {
			if (key == "FileDatas" || strings.HasPrefix(key, "FileDatas[")) && len(files) > 0 {
				return true
			}
		}
	}
	return false
}

func apiNoteUpdatedTime(c *httpserver.Context) time.Time {
	value := strings.TrimSpace(c.Params.String("UpdatedTime"))
	if value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Now()
}

func (s *NotesContentHTTPServer) dispatch(c *httpserver.Context) httpserver.Result {
	uid := apiUserId(c)
	switch c.Controller + "." + c.Action {
	case "ApiNote.GetSyncNotes":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		max := c.Params.Int("maxEntry", 0)
		if max == 0 {
			max = 100
		}
		return c.RenderJSON(noteService.GetSyncNotes(uid, c.Params.Int("afterUsn", 0), max))
	case "ApiNote.GetNotes":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		id := c.Params.String("notebookId")
		if id != "" && !isHex(id) {
			return c.RenderJSON(info.ApiRe{Msg: "notebookIdInvalid"})
		}
		_, notes := noteService.ListNotes(uid, id, false, apiPage(c), pageSize, "UpdatedTime", false, false)
		return c.RenderJSON(noteService.ToApiNotes(notes))
	case "ApiNote.GetTrashNotes":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		_, notes := noteService.ListNotes(uid, "", true, apiPage(c), pageSize, "UpdatedTime", false, false)
		return c.RenderJSON(noteService.ToApiNotes(notes))
	case "ApiNote.GetNote":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		id := c.Params.String("noteId")
		if !isHex(id) {
			return c.RenderJSON(info.ApiRe{Msg: "noteIdInvalid"})
		}
		note := noteService.GetNote(id, uid)
		if note.NoteId.IsZero() {
			return c.RenderJSON(info.ApiRe{Msg: "notExists"})
		}
		return c.RenderJSON(noteService.ToApiNotes([]info.Note{note})[0])
	case "ApiNote.GetNoteAndContent":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		v := noteService.GetNoteAndContent(c.Params.String("noteId"), uid)
		out := noteService.ToApiNotes([]info.Note{v.Note})[0]
		out.Content = noteService.FixContent(v.Content, v.IsMarkdown)
		return c.RenderJSON(out)
	case "ApiNote.GetNoteContent":
		if noteService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		note := noteService.GetNote(c.Params.String("noteId"), uid)
		content := noteService.GetNoteContent(c.Params.String("noteId"), uid)
		if content.Content != "" {
			content.Content = noteService.FixContent(content.Content, note.IsMarkdown)
		}
		return c.RenderJSON(info.ApiNoteContent{NoteId: content.NoteId, UserId: content.UserId, Content: content.Content})
	case "ApiNote.AddNote":
		return s.addNote(c, uid)
	case "ApiNote.UpdateNote":
		return s.updateNote(c, uid)
	case "ApiNotebook.GetSyncNotebooks":
		if notebookService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		max := c.Params.Int("maxEntry", 0)
		if max == 0 {
			max = 100
		}
		return c.RenderJSON((&ApiNotebookServer{}).sync(uid, c.Params.Int("afterUsn", 0), max))
	case "ApiNotebook.GetNotebooks":
		if notebookService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		return c.RenderJSON((&ApiNotebookServer{}).all(uid))
	case "ApiFile.GetImage":
		if fileService == nil {
			return c.RenderText("")
		}
		return s.file(c, false)
	case "ApiFile.GetAttach":
		if attachService == nil {
			return c.RenderText("No Such File")
		}
		return s.file(c, true)
	case "ApiFile.GetAllAttachs":
		if strings.TrimSpace(apiUserId(c)) == "" {
			return c.RenderText("")
		}
		if attachService == nil {
			return c.RenderText("")
		}
		archive, name, err := attachService.OpenReadableArchive(c.Request.Context(), apiUserId(c), c.Params.String("noteId"))
		if err != nil || archive == nil {
			return c.RenderText("")
		}
		return downloadResult{status: http.StatusOK, contentType: "application/x-compressed", disposition: "attachment", name: applicationcontent.ArchiveDownloadFilename(name), reader: archive}
	case "ApiNote.GetHistories":
		if noteContentHistoryService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		return c.RenderJSON(noteContentHistoryService.ListHistories(c.Params.String("noteId"), uid))
	case "ApiNote.ExportPdf":
		actorID, actorErr := domain.ParseObjectID(uid)
		noteID, noteErr := domain.ParseObjectID(c.Params.String("noteId"))
		if actorErr != nil || noteErr != nil || actorID.IsZero() || noteID.IsZero() || service.ContentPDF == nil {
			return c.RenderJSON(info.ApiRe{Msg: "noteNotExists"})
		}
		artifact, err := service.ContentPDF.Export(c.Request.Context(), actorID, noteID)
		if err != nil {
			var contentErr *applicationcontent.Error
			if errors.As(err, &contentErr) && (contentErr.Category == applicationcontent.ErrorUnauthorized || contentErr.Category == applicationcontent.ErrorNotFound) {
				return c.RenderJSON(info.ApiRe{Msg: "noteNotExists"})
			}
			return c.RenderJSON(info.ApiRe{Msg: "sysError"})
		}
		return downloadResult{status: http.StatusOK, contentType: "application/pdf", disposition: "attachment", name: artifact.Filename, data: artifact.Data}
	case "ApiNote.DeleteTrash":
		if trashService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		if trashService == nil {
			return c.RenderJSON(info.ApiRe{Msg: "storage"})
		}
		ok, msg, usn := trashService.DeleteTrashApi(c.Params.String("noteId"), uid, c.Params.Int("usn", 0))
		return c.RenderJSON(info.ReUpdate{Ok: ok, Msg: msg, Usn: usn})
	case "ApiNotebook.AddNotebook":
		if notebookService == nil || !isHex(uid) {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		return (&ApiNotebookServer{}).add(c, uid)
	case "ApiNotebook.UpdateNotebook":
		if notebookService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		ok, msg, n := notebookService.UpdateNotebookApi(uid, c.Params.String("notebookId"), c.Params.String("title"), c.Params.String("parentNotebookId"), c.Params.Int("seq", 0), c.Params.Int("usn", 0))
		if !ok {
			return c.RenderJSON(info.ApiRe{Msg: msg})
		}
		return c.RenderJSON(fixAPINotebooks([]info.Notebook{n})[0])
	case "ApiNotebook.DeleteNotebook":
		if notebookService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		ok, msg := notebookService.DeleteNotebookForce(uid, c.Params.String("notebookId"), c.Params.Int("usn", 0))
		return c.RenderJSON(info.ApiRe{Ok: ok, Msg: msg})
	}
	return c.RenderJSON(info.NewApiRe())
}

func (s *NotesContentHTTPServer) addNote(c *httpserver.Context, uid string) httpserver.Result {
	re := info.NewRe()
	if noteService == nil || !isHex(uid) {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	request := bindAPINote(c)
	if request.NotebookId == "" || !isHex(request.NotebookId) {
		re.Msg = "notebookIdNotExists"
		return c.RenderJSON(re)
	}
	if !noteService.CanCreateNote(uid, uid, request.NotebookId) {
		re.Msg = "notebookIdNotExists"
		return c.RenderJSON(re)
	}
	actorID := db.MustObjectIDFromHex(uid)
	noteID := db.NewObjectID()
	if request.NoteId != "" {
		if !isHex(request.NoteId) {
			re.Msg = "noteIdNotExists"
			return c.RenderJSON(re)
		}
		noteID = db.MustObjectIDFromHex(request.NoteId)
	}
	request.NoteId = noteID.Hex()

	var createOperationID, createInputDigest string
	var createAssets []applicationnotes.OperationAsset
	var assetCandidates map[int]service.APINoteAssetCandidate
	var cleanupAfterCreateFailure func() error
	attachNum := 0
	if len(request.Files) > 0 {
		if attachService == nil {
			re.Msg = "storage"
			return c.RenderJSON(re)
		}
		contentDigests := make(map[int]string, len(request.Files))
		for index, file := range request.Files {
			if !file.HasBody {
				continue
			}
			if file.LocalFileId == "" {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			reader, header, err := c.Params.FormFile("FileDatas[" + file.LocalFileId + "]")
			if err != nil || header == nil {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			candidate, message := attachService.PrepareAPINoteAsset(service.APINoteAssetPrepareInput{
				ActorID: uid, NoteID: noteID.Hex(), OriginalFilename: header.Filename, IsAttach: file.IsAttach, Reader: reader,
			})
			if message != "" {
				re.Msg = message
				return c.RenderJSON(re)
			}
			contentDigests[index] = candidate.Digest
		}

		operationID, inputDigest, assets, err := newAPINoteCreateOperationWithDigests(actorID, noteID, request, contentDigests)
		if err != nil {
			re.Msg = "saveFailed"
			return c.RenderJSON(re)
		}
		if len(assets) > 0 {
			createOperationID, createInputDigest, createAssets = operationID, inputDigest, assets
			if category := noteService.BeginNoteCreateAssetReceipt(actorID, noteID, createOperationID, createInputDigest, createAssets); category != "" {
				if category == service.WorkspaceConflict {
					re.Msg = "conflict"
				} else {
					re.Msg = "saveFailed"
				}
				return c.RenderJSON(re)
			}
		}

		assetByIndex := make(map[int]string, len(createAssets))
		for _, asset := range createAssets {
			assetByIndex[asset.Index] = asset.AssetID
		}
		cleanupFiles := append([]info.NoteFile(nil), request.Files...)
		for index := range cleanupFiles {
			if !cleanupFiles[index].HasBody {
				cleanupFiles[index].FileId = ""
				continue
			}
			cleanupFiles[index].FileId = assetByIndex[index]
			request.Files[index].FileId = cleanupFiles[index].FileId
		}
		attemptedFiles := make([]info.NoteFile, 0, len(request.Files))
		cleanupAfterCreateFailure = func() error {
			if len(attemptedFiles) > 0 {
				if err := attachService.CleanupAPINoteAssets(noteID.Hex(), actorID.Hex(), attemptedFiles); err != nil {
					return err
				}
			}
			if createOperationID != "" {
				if category := noteService.FailNoteCreateAssetReceipt(actorID, createOperationID); category != "" {
					return fmt.Errorf("close note create asset receipt: %s", category)
				}
			}
			return nil
		}

		assetCandidates = make(map[int]service.APINoteAssetCandidate)
		for index, file := range request.Files {
			if !file.HasBody {
				continue
			}
			assetID := assetByIndex[index]
			if assetID == "" {
				re.Msg = "fileRequired"
				if cleanupAfterCreateFailure != nil {
					if err := cleanupAfterCreateFailure(); err != nil {
						re.Msg = "partial_write"
					}
				}
				return c.RenderJSON(re)
			}
			reader, header, err := c.Params.FormFile("FileDatas[" + file.LocalFileId + "]")
			if err != nil || header == nil {
				re.Msg = "fileRequired"
				if cleanupAfterCreateFailure != nil {
					if err := cleanupAfterCreateFailure(); err != nil {
						re.Msg = "partial_write"
					}
				}
				return c.RenderJSON(re)
			}
			candidate, message := attachService.PrepareAPINoteAsset(service.APINoteAssetPrepareInput{
				ActorID: uid, NoteID: noteID.Hex(), AssetID: assetID, OriginalFilename: header.Filename, IsAttach: file.IsAttach, Reader: reader,
			})
			if message != "" {
				re.Msg = message
				if cleanupAfterCreateFailure != nil {
					if err := cleanupAfterCreateFailure(); err != nil {
						re.Msg = "partial_write"
					}
				}
				return c.RenderJSON(re)
			}
			assetCandidates[index] = candidate
		}

		for index, file := range request.Files {
			if !file.HasBody {
				continue
			}
			attemptedFiles = append(attemptedFiles, cleanupFiles[index])
			ok, message, _ := attachService.PublishAPINoteAsset(assetCandidates[index])
			if ok {
				if file.IsAttach {
					attachNum++
				}
				continue
			}
			re.Msg = "fileUploadError"
			if message == "partial_write" {
				re.Msg = "partial_write"
			}
			if err := cleanupAfterCreateFailure(); err != nil {
				re.Msg = "partial_write"
			}
			return c.RenderJSON(re)
		}
	}

	request.Content = rewriteAPINoteContentLinks(request.Content, request.Files)
	now := time.Now()
	if request.CreatedTime.IsZero() {
		request.CreatedTime = now
	}
	if request.UpdatedTime.IsZero() {
		request.UpdatedTime = now
	}
	if request.Abstract == "" {
		request.Abstract = lea.SubStringHTML(request.Content, 200, "")
	}
	desc := request.Desc
	if desc == "" {
		desc = lea.SubStringHTMLToRaw(request.Abstract, 200)
	}
	note := info.Note{UserId: actorID, NoteId: noteID, NotebookId: db.MustObjectIDFromHex(request.NotebookId), Title: request.Title, Desc: desc, Tags: request.Tags, IsBlog: request.IsBlog, IsMarkdown: request.IsMarkdown, AttachNum: attachNum, CreatedTime: request.CreatedTime, UpdatedTime: request.UpdatedTime}
	content := info.NoteContent{NoteId: noteID, UserId: actorID, IsBlog: request.IsBlog, Content: request.Content, Abstract: request.Abstract, CreatedTime: request.CreatedTime, UpdatedTime: request.UpdatedTime}
	committedAssets := make([]applicationnotes.OperationAsset, 0, len(request.Files))
	for index, file := range request.Files {
		if file.FileId == "" {
			continue
		}
		asset := applicationnotes.OperationAsset{AssetID: file.FileId, LocalFileID: file.LocalFileId, Index: index, IsAttach: file.IsAttach}
		for _, frozen := range createAssets {
			if frozen.AssetID == file.FileId {
				asset.ContentSHA256 = frozen.ContentSHA256
				break
			}
		}
		committedAssets = append(committedAssets, asset)
	}
	var created info.Note
	var ok bool
	var msg string
	if createOperationID != "" {
		created, ok, msg = noteService.AddNoteAndContentApiResultWithIdentity(note, content, actorID, createOperationID, createInputDigest, committedAssets)
	} else {
		created, ok, msg = noteService.AddNoteAndContentApiResultWithAssets(note, content, actorID, committedAssets)
	}
	if !ok || created.NoteId.IsZero() {
		re.Msg = nonEmptyAPIMessage(msg)
		if created.NoteId.IsZero() && cleanupAfterCreateFailure != nil {
			if err := cleanupAfterCreateFailure(); err != nil {
				re.Msg = "partial_write"
			}
		}
		return c.RenderJSON(re)
	}
	if len(request.Files) > 0 && len(assetCandidates) > 0 {
		if err := attachService.FinalizeAPINoteAssets(created.NoteId.Hex(), actorID.Hex(), request.Files); err != nil {
			re.Msg = "partial_write"
			return c.RenderJSON(re)
		}
	}
	request.NoteId, request.UserId, request.Usn = created.NoteId.Hex(), uid, created.Usn
	request.CreatedTime, request.UpdatedTime = created.CreatedTime, created.UpdatedTime
	request.Content, request.Abstract = "", ""
	return c.RenderJSON(request)
}

func rewriteAPINoteContentLinks(content string, files []info.NoteFile) string {
	for _, file := range files {
		if file.LocalFileId == "" || file.FileId == "" || file.LocalFileId == file.FileId {
			continue
		}
		action := "getImage"
		if file.IsAttach {
			action = "getAttach"
		}
		pattern := regexp.MustCompile(`https?://[^/\s"']+/api/file/` + action + `\?fileId=` + regexp.QuoteMeta(file.LocalFileId) + `([^A-Za-z0-9_-]|$)`)
		content = pattern.ReplaceAllString(content, `/api/file/`+action+`?fileId=`+file.FileId+`$1`)
	}
	return content
}

func (s *NotesContentHTTPServer) updateNote(c *httpserver.Context, uid string) httpserver.Result {
	re := info.NewReUpdate()
	if noteService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	request := bindAPINote(c)
	if request.NoteId == "" {
		re.Msg = "noteIdNotExists"
		return c.RenderJSON(re)
	}
	if !c.Params.Has("Usn") {
		re.Msg = "usnNotExists"
		return c.RenderJSON(re)
	}
	usn, err := c.Params.StrictInt("Usn")
	if err != nil || usn <= 0 {
		re.Msg = "usnNotExists"
		return c.RenderJSON(re)
	}
	request.Usn = usn
	note := noteService.GetNote(request.NoteId, uid)
	if note.NoteId.IsZero() {
		re.Msg = "notExists"
		return c.RenderJSON(re)
	}
	filesPresent := apiNoteAssetsSupplied(c)
	var assetWork *applicationnotes.AssetMutation
	if filesPresent {
		if attachService == nil {
			re.Msg = "storage"
			return c.RenderJSON(re)
		}
		ownerID := db.MustObjectIDFromHex(uid)
		contentDigests := make(map[int]string, len(request.Files))
		for index := range request.Files {
			file := &request.Files[index]
			if !file.HasBody {
				continue
			}
			if file.LocalFileId == "" {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			reader, header, err := c.Params.FormFile("FileDatas[" + file.LocalFileId + "]")
			if err != nil || header == nil {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			candidate, message := attachService.PrepareAPINoteAsset(service.APINoteAssetPrepareInput{
				ActorID: uid, NoteID: request.NoteId, OriginalFilename: header.Filename, IsAttach: file.IsAttach, Reader: reader,
			})
			if message != "" {
				re.Msg = message
				return c.RenderJSON(re)
			}
			contentDigests[index] = candidate.Digest
		}
		assetSeed := stableAPIUpdateAssetSeedWithDigests(ownerID, note.NoteId, usn, request, contentDigests)
		if assetSeed == "" {
			re.Msg = "saveFailed"
			return c.RenderJSON(re)
		}
		candidates := make(map[int]service.APINoteAssetCandidate)
		assets := make([]applicationnotes.OperationAsset, 0, len(request.Files))
		for index := range request.Files {
			file := &request.Files[index]
			if file.HasBody {
				reader, header, err := c.Params.FormFile("FileDatas[" + file.LocalFileId + "]")
				if err != nil || header == nil {
					re.Msg = "fileRequired"
					return c.RenderJSON(re)
				}
				assetID := stableAPIAssetID(assetSeed, file.LocalFileId, index, file.IsAttach)
				candidate, message := attachService.PrepareAPINoteAsset(service.APINoteAssetPrepareInput{ActorID: uid, NoteID: request.NoteId, AssetID: assetID, OriginalFilename: header.Filename, IsAttach: file.IsAttach, Reader: reader})
				if message != "" {
					re.Msg = message
					return c.RenderJSON(re)
				}
				file.FileId = assetID
				candidates[index] = candidate
				assets = append(assets, applicationnotes.OperationAsset{AssetID: file.FileId, LocalFileID: file.LocalFileId, ContentSHA256: candidate.Digest, Index: index, IsAttach: file.IsAttach})
				continue
			}
			if file.FileId != "" {
				if !isHex(file.FileId) {
					re.Msg = "fileRequired"
					return c.RenderJSON(re)
				}
				assets = append(assets, applicationnotes.OperationAsset{AssetID: file.FileId, Index: index, IsAttach: file.IsAttach})
			}
		}
		assetWork = &applicationnotes.AssetMutation{Assets: assets}
		assetWork.Apply = func(ctx context.Context, expectedUSN int) error {
			for index, candidate := range candidates {
				ok, message, _ := attachService.PublishAPINoteAsset(candidate)
				if !ok {
					if message == "" {
						message = "fileUploadError"
					}
					return fmt.Errorf("upload asset %d: %s", index, message)
				}
			}
			if err := service.ContentAssets.ReconcileNote(ctx, applicationnotes.ReconcileNoteAssetsCommand{OperationID: assetWork.OperationID, ActorID: ownerID, OwnerID: note.UserId, NoteID: note.NoteId, Generation: expectedUSN}); err != nil {
				return err
			}
			return attachService.FinalizeAPINoteAssets(note.NoteId.Hex(), note.UserId.Hex(), request.Files)
		}
		assetWork.Verify = func(ctx context.Context) (bool, error) {
			return service.ContentAssets.VerifyReconcileNote(ctx, applicationnotes.ReconcileNoteAssetsCommand{OperationID: assetWork.OperationID, ActorID: ownerID, OwnerID: note.UserId, NoteID: note.NoteId})
		}
	}
	metadata := bson.M{}
	if c.Params.Has("Desc") {
		metadata["Desc"] = request.Desc
	}
	if c.Params.Has("Title") {
		metadata["Title"] = request.Title
	}
	if c.Params.Has("IsTrash") {
		metadata["IsTrash"] = request.IsTrash
	}
	if c.Params.Has("IsBlog") {
		metadata["IsBlog"] = request.IsBlog
	}
	if c.Params.Has("Tags") || c.Params.Has("Tags[]") || c.Params.Has("Tags[0]") {
		metadata["Tags"] = request.Tags
	}
	if c.Params.Has("NotebookId") {
		if !isHex(request.NotebookId) || !noteService.CanCreateNote(uid, uid, request.NotebookId) {
			re.Msg = "notebookIdNotExists"
			return c.RenderJSON(re)
		}
		metadata["NotebookId"] = db.MustObjectIDFromHex(request.NotebookId)
	}
	updatedTime := apiNoteUpdatedTime(c)
	metadata["UpdatedTime"] = updatedTime
	var content, abstract *string
	if c.Params.Has("Content") {
		abstractValue := request.Abstract
		if abstractValue == "" {
			abstractValue = lea.SubStringHTML(request.Content, 200, "")
		}
		metadata["Desc"] = lea.SubStringHTMLToRaw(abstractValue, 200)
		contentValue := request.Content
		content, abstract = &contentValue, &abstractValue
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	expectedUSN := request.Usn
	result := noteService.SaveNote(service.SaveNoteCommand{ActorUserID: uid, NoteID: request.NoteId, ExpectedUSN: &expectedUSN, Metadata: metadata, Content: content, Abstract: abstract, UpdatedTime: updatedTime, AssetWork: assetWork})
	re.Ok, re.Msg, re.Usn = result.OK(), workspaceAPIMessage(result.Error), result.USN
	if !re.Ok {
		return c.RenderJSON(re)
	}
	request.UserId, request.Content, request.Abstract = uid, "", ""
	request.Usn, request.UpdatedTime = re.Usn, updatedTime
	return c.RenderJSON(request)
}

func (s *NotesContentHTTPServer) file(c *httpserver.Context, attach bool) httpserver.Result {
	var reader io.ReadCloser
	var name string
	var err error
	if attach {
		d, e := attachService.OpenReadable(c.Request.Context(), apiUserId(c), c.Params.String("fileId"))
		if e == nil {
			reader, name = d.Reader, d.DisplayName
		}
		err = e
	} else {
		d, e := fileService.OpenReadableImage(c.Request.Context(), apiUserId(c), c.Params.String("fileId"))
		if e == nil {
			reader, name = d.Reader, d.Name
		}
		err = e
	}
	if err != nil || reader == nil {
		if attach {
			return c.RenderText("No Such File")
		}
		return c.RenderText("")
	}
	disposition := "inline"
	if attach {
		disposition = "attachment"
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return downloadResult{status: http.StatusOK, contentType: contentType, disposition: disposition, name: name, reader: reader}
}

type downloadResult struct {
	status                         int
	contentType, disposition, name string
	reader                         io.ReadCloser
	data                           []byte
}

func (r downloadResult) Apply(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", r.contentType)
	w.Header().Set("Content-Disposition", httpserver.DownloadDisposition(r.disposition, r.name))
	w.WriteHeader(r.status)
	if r.reader != nil {
		defer r.reader.Close()
		_, _ = io.Copy(w, r.reader)
		return
	}
	_, _ = w.Write(r.data)
}

type ApiNotebookServer struct{}

func (s *ApiNotebookServer) sync(uid string, after, max int) []info.ApiNotebook {
	return fixAPINotebooks(notebookService.GeSyncNotebooks(uid, after, max))
}
func (s *ApiNotebookServer) all(uid string) []info.ApiNotebook {
	return fixAPINotebooks(notebookService.GetActiveNotebooks(uid))
}
func (s *ApiNotebookServer) add(c *httpserver.Context, uid string) httpserver.Result {
	n := info.Notebook{NotebookId: db.NewObjectID(), Title: c.Params.String("title"), Seq: c.Params.Int("seq", 0), UserId: db.MustObjectIDFromHex(uid)}
	if parent := c.Params.String("parentNotebookId"); parent != "" {
		if !isHex(parent) {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "notebookIdNotExists"})
		}
		n.ParentNotebookId = db.MustObjectIDFromHex(parent)
	}
	ok, n := notebookService.AddNotebook(n)
	if !ok {
		return c.RenderJSON(info.ApiRe{})
	}
	return c.RenderJSON(fixAPINotebooks([]info.Notebook{n})[0])
}
func fixAPINotebooks(in []info.Notebook) []info.ApiNotebook {
	out := make([]info.ApiNotebook, len(in))
	for i := range in {
		n := in[i]
		out[i] = info.ApiNotebook{NotebookId: n.NotebookId, UserId: n.UserId, ParentNotebookId: n.ParentNotebookId, Seq: n.Seq, Title: n.Title, UrlTitle: n.UrlTitle, IsBlog: n.IsBlog, CreatedTime: n.CreatedTime, UpdatedTime: n.UpdatedTime, Usn: n.Usn, IsDeleted: n.IsDeleted}
	}
	return out
}
func isHex(v string) bool {
	return len(strings.TrimSpace(v)) == 24 && db.IsValidObjectIDHex(strings.TrimSpace(v))
}
