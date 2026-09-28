package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// RegisterNotesHTTP wires the B3 notes/content actions. Catch-all routes keep
// their legacy any-method behavior; /note/:noteId remains GET-only in the
// route table and is therefore registered without an adapter method guard.
func RegisterNotesHTTP(rs *httpserver.Registry, runModes ...string) {
	public := []httpserver.BeforeFunc{webSessionBefore}
	auth := []httpserver.BeforeFunc{webSessionBefore, requireWebAuthentication}
	runMode := ""
	if len(runModes) > 0 {
		runMode = runModes[0]
	}
	n := &NotesHTTPServer{RunMode: runMode}
	for _, name := range []string{"ListNotes", "ListTrashNotes", "GetNoteAndContent", "GetNoteContent", "UpdateNoteOrContent", "DeleteNote", "DeleteTrash", "MoveNote", "CopyNote", "CopySharedNote", "SearchNote", "SearchNoteByTags", "SetNote2Blog", "ExportPDF", "ExportPdf", "ToPdf", "GetNoteAndContentBySrc"} {
		rs.Register("Note", name, auth, n.dispatch)
	}
	rs.RegisterMethods("Note", "Index", []string{"GET"}, auth, n.Index)
	for _, name := range []string{"Index", "GetNotebooks", "DeleteNotebook", "AddNotebook", "UpdateNotebookTitle", "DragNotebooks", "SortNotebooks", "SetNotebook2Blog"} {
		rs.Register("Notebook", name, auth, n.dispatch)
	}
	for _, name := range []string{"UpdateTag", "DeleteTag"} {
		rs.Register("Tag", name, auth, n.dispatch)
	}
	rs.Register("NoteContentHistory", "ListHistories", auth, n.dispatch)
	for _, name := range []string{"UploadBlogLogo", "PasteImage", "UploadAvatar", "UploadImageLeaui", "GetImages", "UpdateImageTitle", "DeleteImage", "CopyImage", "CopyHttpImage"} {
		rs.Register("File", name, auth, n.dispatch)
	}
	rs.Register("File", "OutputImage", public, n.dispatch)
	for _, name := range []string{"UploadAttach", "DeleteAttach", "GetAttachs", "DownloadAll"} {
		rs.Register("Attach", name, auth, n.dispatch)
	}
	rs.Register("Attach", "Download", public, n.dispatch)
	for _, name := range []string{"Index", "GetAlbums", "DeleteAlbum", "AddAlbum", "UpdateAlbum"} {
		rs.Register("Album", name, auth, n.dispatch)
	}
}

type DragNotebooksInfo struct {
	CurNotebookId    string
	ParentNotebookId string
	Siblings         []string
	OperationId      string
}

func workspaceWebSaveMessage(category service.WorkspaceErrorCategory) string {
	switch category {
	case service.WorkspaceNotFound:
		return "notExists"
	case service.WorkspaceUnauthorized:
		return "noAuth"
	default:
		return nonEmptySaveMessage(string(category))
	}
}

func nonEmptySaveMessage(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "saveFailed"
	}
	return msg
}

type NotesHTTPServer struct {
	RunMode string
}

type notesDownloadResult struct {
	status                         int
	contentType, disposition, name string
	reader                         io.ReadCloser
	data                           []byte
}

func (r notesDownloadResult) Apply(w http.ResponseWriter, _ *http.Request) {
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

func bindWebNoteOrContent(c *httpserver.Context) (info.NoteOrContent, error) {
	value := info.NoteOrContent{
		NotebookId: c.Params.String("NotebookId"), NoteId: c.Params.String("NoteId"),
		OperationId: c.Params.String("OperationId"), UserId: c.Params.String("UserId"),
		Title: c.Params.String("Title"), Desc: c.Params.String("Desc"), Src: c.Params.String("Src"),
		ImgSrc: c.Params.String("ImgSrc"), Content: c.Params.String("Content"),
		Abstract: c.Params.String("Abstract"), FromUserId: c.Params.String("FromUserId"),
		IsNew: c.Params.Bool("IsNew", false), IsMarkdown: c.Params.Bool("IsMarkdown", false),
		IsBlog: c.Params.Bool("IsBlog", false),
	}
	value.Tags = strings.Join(noteParameterStrings(c.Params, "Tags"), ",")
	if c.Params.Has("ExpectedUsn") {
		usn, err := c.Params.StrictInt("ExpectedUsn")
		if err != nil {
			return info.NoteOrContent{}, err
		}
		value.ExpectedUsn = usn
	}
	return value, nil
}

func noteParameterStrings(params *httpserver.Params, name string) []string {
	if values := params.Strings(name); len(values) > 0 {
		return values
	}
	if values := params.Strings(name + "[]"); len(values) > 0 {
		return values
	}
	values := []string{}
	for index := 0; ; index++ {
		value, ok := params.Get(fmt.Sprintf("%s[%d]", name, index))
		if !ok {
			break
		}
		values = append(values, value)
	}
	return values
}

func (s *NotesHTTPServer) updateNoteOrContent(c *httpserver.Context) httpserver.Result {
	if noteService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	noteOrContent, err := bindWebNoteOrContent(c)
	if err != nil {
		return c.RenderJSON(info.Re{Msg: "validation"})
	}
	if noteOrContent.IsNew {
		if !db.IsValidObjectIDHex(noteOrContent.NoteId) {
			return c.RenderJSON(info.Re{Msg: "noteIdNotExists"})
		}
		if !db.IsValidObjectIDHex(noteOrContent.NotebookId) {
			return c.RenderJSON(info.Re{Msg: "notebookIdNotExists"})
		}
		ownerID := c.GetPrincipal().UserID
		if noteOrContent.FromUserId != "" {
			if !db.IsValidObjectIDHex(noteOrContent.FromUserId) {
				return c.RenderJSON(info.Re{Msg: "noAuth"})
			}
			ownerID = noteOrContent.FromUserId
		}
		owner := db.MustObjectIDFromHex(ownerID)
		note := info.Note{UserId: owner, NoteId: db.MustObjectIDFromHex(noteOrContent.NoteId), NotebookId: db.MustObjectIDFromHex(noteOrContent.NotebookId), Title: noteOrContent.Title, Src: noteOrContent.Src, Tags: strings.Split(noteOrContent.Tags, ","), Desc: noteOrContent.Desc, ImgSrc: noteOrContent.ImgSrc, IsBlog: noteOrContent.IsBlog, IsMarkdown: noteOrContent.IsMarkdown}
		content := info.NoteContent{NoteId: note.NoteId, UserId: owner, IsBlog: note.IsBlog, Content: noteOrContent.Content, Abstract: noteOrContent.Abstract}
		created, ok, message := noteService.AddNoteAndContentForControllerResult(note, content, c.GetPrincipal().UserID)
		re := info.NewRe()
		re.Ok, re.Msg = ok, message
		if ok {
			re.Item = created
		}
		if !ok && strings.TrimSpace(re.Msg) == "" {
			re.Msg = "saveFailed"
		}
		return c.RenderJSON(re)
	}
	metadata := bson.M{}
	if c.Params.Has("Desc") {
		metadata["Desc"] = noteOrContent.Desc
	}
	if c.Params.Has("ImgSrc") {
		metadata["ImgSrc"] = noteOrContent.ImgSrc
	}
	if c.Params.Has("Title") {
		metadata["Title"] = noteOrContent.Title
	}
	if c.Params.Has("Tags") || c.Params.Has("Tags[]") || c.Params.Has("Tags[0]") {
		metadata["Tags"] = noteParameterStrings(c.Params, "Tags")
	}
	if c.Params.Has("NotebookId") {
		if !db.IsValidObjectIDHex(noteOrContent.NotebookId) {
			return c.RenderJSON(info.Re{Msg: "notebookIdNotExists"})
		}
		metadata["NotebookId"] = db.MustObjectIDFromHex(noteOrContent.NotebookId)
	}
	if c.Params.Has("IsTrash") {
		metadata["IsTrash"] = c.Params.Bool("IsTrash", false)
	}
	if c.Params.Has("IsBlog") {
		metadata["IsBlog"] = noteOrContent.IsBlog
	}
	var content, abstract *string
	if c.Params.Has("Content") {
		contentValue := noteOrContent.Content
		abstractValue := noteOrContent.Abstract
		if abstractValue == "" {
			metadata["Desc"] = lea.SubStringHTMLToRaw(contentValue, 200)
			abstractValue = lea.SubStringHTML(contentValue, 200, "")
		} else {
			metadata["Desc"] = lea.SubStringHTMLToRaw(abstractValue, 200)
		}
		content, abstract = &contentValue, &abstractValue
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	var expectedUSN *int
	if c.Params.Has("ExpectedUsn") {
		expectedUSN = &noteOrContent.ExpectedUsn
	}
	result := noteService.SaveNote(service.SaveNoteCommand{ActorUserID: c.GetPrincipal().UserID, OperationID: noteOrContent.OperationId, NoteID: noteOrContent.NoteId, ExpectedUSN: expectedUSN, Metadata: metadata, Content: content, Abstract: abstract, UpdatedTime: time.Now()})
	re := info.NewRe()
	re.Ok = result.OK()
	if !re.Ok {
		re.Msg = workspaceWebSaveMessage(result.Error)
	}
	return c.RenderJSON(re)
}

func (s *NotesHTTPServer) Index(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	if userService == nil || notebookService == nil || shareService == nil || noteService == nil || tagService == nil || configService == nil {
		return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
	}
	userInfo := userService.GetUserAndBlogUrl(c.GetPrincipal().UserID)
	userID := userInfo.UserId.Hex()
	if userID == "" {
		return c.Redirect("/login")
	}

	args["openRegister"] = configService.IsOpenRegister()
	notebooks := notebookService.GetNotebooks(userID)
	shareNotebooks, sharedUserInfos, err := shareService.GetShareNotebooksChecked(userID)
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
	}

	notes := make([]info.Note, 0)
	noteContent := info.NoteContent{}
	noteID := c.Params.String("noteId")
	hasRightNoteID := false
	if len(notebooks) > 0 && db.IsValidObjectIDHex(noteID) {
		note := noteService.GetNoteById(noteID)
		if !note.NoteId.IsZero() {
			noteOwner := note.UserId.Hex()
			noteContent = noteService.GetNoteContent(noteID, noteOwner)
			hasRightNoteID = true
			args["curNoteId"] = noteID
			args["curNotebookId"] = note.NotebookId.Hex()
			if noteOwner != userID {
				if shareService.HasReadPerm(noteOwner, userID, noteID) {
					args["curSharedNoteNotebookId"] = note.NotebookId.Hex()
					args["curSharedUserId"] = noteOwner
				} else {
					hasRightNoteID = false
				}
			} else {
				_, notes = noteService.ListNotes(userID, note.NotebookId.Hex(), false, pageParam(c), 50, defaultSortField, false, false)
				if len(notes) > 1 {
					ordered := make([]info.Note, 0, len(notes))
					ordered = append(ordered, note)
					for _, candidate := range notes {
						if candidate.NoteId != note.NoteId {
							ordered = append(ordered, candidate)
						}
					}
					notes = ordered
				}
			}
		}
		_, latestNotes := noteService.ListNotes(userID, "", false, pageParam(c), 50, defaultSortField, false, false)
		args["latestNotes"] = latestNotes
	}
	if !hasRightNoteID {
		_, notes = noteService.ListNotes(userID, "", false, pageParam(c), 50, defaultSortField, false, false)
		if len(notes) > 0 {
			noteContent = noteService.GetNoteContent(notes[0].NoteId.Hex(), userID)
			args["curNoteId"] = notes[0].NoteId.Hex()
		}
	}

	args["isAdmin"] = configService.GetAdminUsername() == userInfo.Username
	args["userInfo"] = userInfo
	args["notebooks"] = notebooks
	args["shareNotebooks"] = shareNotebooks
	args["sharedUserInfos"] = sharedUserInfos
	args["notes"] = notes
	args["noteContentJson"] = noteContent
	args["noteContent"] = noteContent.Content
	args["tags"] = tagService.GetTags(userID)
	args["globalConfigs"] = configService.GetGlobalConfigForUser()
	templateName := "note/note.html"
	if s.RunMode == "dev" {
		templateName = "note/note-dev.html"
	}
	return c.RenderTemplate(templateName, args)
}

func (s *NotesHTTPServer) uploadAttach(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if attachService == nil || configService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	file, header, err := c.Params.FormFile("file")
	if err != nil || file == nil || header == nil {
		re.Msg = "error"
		return c.RenderJSON(re)
	}
	defer file.Close()
	limit, err := configService.GetUploadLimitBytes("uploadAttachSize")
	if err != nil {
		re.Msg = "upload config error"
		return c.RenderJSON(re)
	}
	attachment, ok, msg := attachService.UploadWebAttachment(service.WebAttachmentUploadInput{
		ActorID: c.GetPrincipal().UserID, NoteID: c.Params.String("noteId"), OriginalFilename: header.Filename,
		Reader: file, Limit: limit, OperationID: strings.TrimSpace(c.Params.String("OperationId")),
	})
	re.Id, re.Ok, re.Msg, re.Item = attachment.AttachId.Hex(), ok, msg, attachment
	if msg == "too large" {
		re.Msg = fmt.Sprintf("The file's size is bigger than %vM", float64(limit)/(1024*1024))
	}
	if ok {
		re.Msg = "success"
	}
	return c.RenderJSON(re)
}

func (s *NotesHTTPServer) downloadAttach(c *httpserver.Context, all bool) httpserver.Result {
	if attachService == nil {
		return c.RenderText("")
	}
	var reader io.ReadCloser
	var name string
	var err error
	if all {
		var title string
		reader, title, err = attachService.OpenReadableArchive(c.Request.Context(), c.GetPrincipal().UserID, c.Params.String("noteId"))
		name = applicationcontent.ArchiveDownloadFilename(title)
	} else {
		var download applicationcontent.AttachmentDownload
		download, err = attachService.OpenReadable(c.Request.Context(), c.GetPrincipal().UserID, c.Params.String("attachId"))
		if err == nil {
			reader, name = download.Reader, applicationcontent.AttachmentDownloadFilename(download.DisplayName)
		}
	}
	if err != nil || reader == nil {
		return c.RenderText("")
	}
	return notesDownloadResult{status: http.StatusOK, contentType: "application/octet-stream", disposition: "attachment", name: name, reader: reader}
}

func (s *NotesHTTPServer) uploadImage(c *httpserver.Context, kind service.ImageUploadKind, configKey, albumID string) httpserver.Result {
	return c.RenderJSON(s.uploadImageResult(c, kind, configKey, albumID))
}

func (s *NotesHTTPServer) uploadImageResult(c *httpserver.Context, kind service.ImageUploadKind, configKey, albumID string) info.Re {
	re := info.NewRe()
	if fileService == nil || configService == nil {
		re.Msg = "storage"
		return re
	}
	file, header, err := c.Params.FormFile("file")
	if err != nil || file == nil || header == nil {
		re.Msg = "error"
		return re
	}
	defer file.Close()
	limit, err := configService.GetUploadLimitBytes(configKey)
	if err != nil {
		re.Msg = "upload config error"
		return re
	}
	displayName := ""
	if kind == service.ImageUploadPrivate && c.Action == "PasteImage" {
		displayName = c.Message("unTitled")
	}
	result, err := fileService.UploadImage(c.Request.Context(), service.ImageUploadInput{ActorID: c.GetPrincipal().UserID, AlbumID: albumID, Kind: kind, Reader: file, Limit: limit, OriginalName: header.Filename, DisplayName: displayName, Budget: applicationcontent.HardImageBudget()})
	if err != nil {
		var contentErr *applicationcontent.Error
		switch {
		case errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorTooLarge:
			re.Msg = fmt.Sprintf("The file Size is bigger than %vM", float64(limit)/(1024*1024))
		case errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorUnsupportedMedia:
			re.Msg = "Please upload image"
		case errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorPartialWrite:
			re.Id, re.Msg = result.ExternalID, "partial_write"
		default:
			re.Msg = "error"
		}
		return re
	}
	result.File.Path = ""
	re.Ok, re.Code, re.Msg, re.Id, re.Item = true, 1, "Upload Success!", result.ExternalID, result.File
	return re
}

func (s *NotesHTTPServer) exportPDF(c *httpserver.Context, api bool) httpserver.Result {
	reMsg := "error"
	actorID, actorErr := domain.ParseObjectID(c.GetPrincipal().UserID)
	noteID, noteErr := domain.ParseObjectID(c.Params.String("noteId"))
	if actorErr != nil || noteErr != nil || actorID.IsZero() || noteID.IsZero() || service.ContentPDF == nil {
		if api {
			return c.RenderJSON(info.ApiRe{Msg: "noteNotExists"})
		}
		return c.RenderText(reMsg)
	}
	artifact, err := service.ContentPDF.Export(c.Request.Context(), actorID, noteID)
	if err != nil {
		var contentErr *applicationcontent.Error
		if api && errors.As(err, &contentErr) && (contentErr.Category == applicationcontent.ErrorUnauthorized || contentErr.Category == applicationcontent.ErrorNotFound) {
			return c.RenderJSON(info.ApiRe{Msg: "noteNotExists"})
		}
		if api {
			return c.RenderJSON(info.ApiRe{Msg: "sysError"})
		}
		if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorUnauthorized {
			return c.RenderText("No Perm")
		}
		return c.RenderText(reMsg)
	}
	return notesDownloadResult{status: http.StatusOK, contentType: "application/pdf", disposition: "attachment", name: artifact.Filename, data: artifact.Data}
}

// dispatch is intentionally explicit at registration time. It provides the
// stable legacy envelope for actions whose richer service wiring is still
// owned by the application service; no reflection or controller fallback is
// used by the HTTP registry.
func (s *NotesHTTPServer) dispatch(c *httpserver.Context) httpserver.Result {
	userID := c.GetPrincipal().UserID
	switch c.Controller + "." + c.Action {
	case "Attach.UploadAttach":
		return s.uploadAttach(c)
	case "Attach.Download":
		return s.downloadAttach(c, false)
	case "Attach.DownloadAll":
		return s.downloadAttach(c, true)
	case "File.UploadBlogLogo":
		re := s.uploadImageResult(c, service.ImageUploadBlogLogo, "uploadBlogLogoSize", "")
		args := prepareView(c)
		args["fileUrlPath"], args["resultCode"], args["resultMsg"] = re.Id, re.Code, re.Msg
		return c.RenderTemplate("file/blog_logo.html", args)
	case "File.PasteImage":
		re := s.uploadImageResult(c, service.ImageUploadPrivate, "uploadImageSize", "")
		if re.Ok && c.Params.String("noteId") != "" && fileService != nil {
			if copied, copiedID := fileService.CopyImageForNote(c.Request.Context(), userID, re.Id, c.Params.String("noteId")); copied {
				re.Id = copiedID
			}
		}
		return c.RenderJSON(re)
	case "File.UploadImageLeaui":
		return s.uploadImage(c, service.ImageUploadPrivate, "uploadImageSize", c.Params.String("albumId"))
	case "File.UploadAvatar":
		if configService == nil || !configService.GlobalSnapshotLoaded() {
			return c.RenderJSON(info.Re{Ok: false, Msg: "configuration"})
		}
		isDemo, err := configService.IsDemoUser(userID)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: demoPolicyErrorMessage(err)})
		}
		if isDemo {
			return c.RenderJSON(info.Re{Ok: false, Msg: "cannotUpdateDemo"})
		}
		re := s.uploadImageResult(c, service.ImageUploadAvatar, "uploadAvatarSize", "")
		if re.Ok {
			if userService == nil || !userService.UpdateAvatar(userID, re.Id) {
				re.Ok, re.Msg = false, "storage"
			} else if err := c.SetSession("Logo", re.Id); err != nil {
				re.Ok, re.Msg = false, "storage"
			}
		}
		return c.RenderJSON(re)
	case "Note.ExportPdf":
		return s.exportPDF(c, false)
	case "Note.UpdateNoteOrContent":
		return s.updateNoteOrContent(c)
	case "Note.ListNotes":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		_, notes := noteService.ListNotes(userID, c.Params.String("notebookId"), false, pageParam(c), pageSize, defaultSortField, false, false)
		return c.RenderJSON(notes)
	case "Note.ListTrashNotes":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		_, notes := noteService.ListNotes(userID, "", true, pageParam(c), pageSize, defaultSortField, false, false)
		return c.RenderJSON(notes)
	case "Note.GetNoteAndContent":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteService.GetNoteAndContent(c.Params.String("noteId"), userID))
	case "Note.GetNoteContent":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteService.GetNoteContent(c.Params.String("noteId"), userID))
	case "Note.GetNoteAndContentBySrc":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		id, value := noteService.GetNoteAndContentBySrc(c.Params.String("src"), userID)
		re := info.Re{}
		if id != "" {
			re.Ok = true
			re.Item = value
		}
		return c.RenderJSON(re)
	case "Note.DeleteNote":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteService.DeleteNotesWithOperation(userID, c.Params.Strings("noteIds"), c.Params.Bool("isShared", false), c.Params.String("OperationId")).OK())
	case "Note.MoveNote":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteService.MoveNotesWithOperation(userID, c.Params.Strings("noteIds"), c.Params.String("notebookId"), c.Params.String("OperationId")).OK())
	case "Note.CopyNote":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		r := noteService.CopyNotesWithOperation(userID, c.Params.Strings("noteIds"), c.Params.String("notebookId"), c.Params.String("OperationId"))
		return c.RenderJSON(info.Re{Ok: r.OK(), Item: r.Notes})
	case "Note.CopySharedNote":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		r := noteService.CopySharedNotesWithOperation(userID, c.Params.Strings("noteIds"), c.Params.String("notebookId"), c.Params.String("fromUserId"), c.Params.String("OperationId"))
		return c.RenderJSON(info.Re{Ok: r.OK(), Item: r.Notes})
	case "Note.SearchNoteByTags":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		_, notes := noteService.SearchNoteByTags(noteParameterStrings(c.Params, "tags"), userID, pageParam(c), pageSize, "UpdatedTime", false)
		return c.RenderJSON(notes)
	case "Note.SearchNote":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		_, notes := noteService.SearchNote(c.Params.String("key"), userID, pageParam(c), pageSize, "UpdatedTime", false, false)
		return c.RenderJSON(notes)
	case "Note.SetNote2Blog":
		if noteService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteService.ToBlog(userID, c.Params.String("noteId"), c.Params.Bool("isBlog", false), c.Params.Bool("isTop", false)))
	case "Note.ExportPDF":
		return s.exportPDF(c, false)
	case "Note.ToPdf":
		_, _ = c.Params.String("noteId"), c.Params.String("appKey")
		return c.RenderText("no note")
	case "Notebook.Index":
		return c.RenderJSON(info.Notebook{
			NotebookId: db.MustObjectIDFromHex(c.Params.String("notebookId")),
			Title:      c.Params.String("title"),
			Seq:        c.Params.Int("seq", 0),
		})
	case "Note.DeleteTrash":
		if trashService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(trashService.DeleteTrash(c.Params.String("noteId"), userID))
	case "Notebook.GetNotebooks":
		if notebookService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(notebookService.GetNotebooks(userID))
	case "Notebook.DeleteNotebook":
		if notebookService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		ok, msg := notebookService.DeleteNotebook(userID, c.Params.String("notebookId"))
		return c.RenderJSON(info.Re{Ok: ok, Msg: msg})
	case "Tag.UpdateTag":
		if tagService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		item, ok, msg := tagService.AddOrUpdateTagResult(userID, c.Params.String("tag"))
		return c.RenderJSON(info.Re{Ok: ok, Msg: msg, Item: item})
	case "Tag.DeleteTag":
		if tagService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		item, ok := tagService.DeleteTagResult(userID, c.Params.String("tag"))
		return c.RenderJSON(info.Re{Ok: ok, Item: item})
	case "Notebook.UpdateNotebookTitle":
		if notebookService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(notebookService.UpdateNotebookTitle(c.Params.String("notebookId"), userID, c.Params.String("title")))
	case "Notebook.SetNotebook2Blog":
		if notebookService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(notebookService.ToBlog(userID, c.Params.String("notebookId"), c.Params.Bool("isBlog", false)))
	case "Notebook.AddNotebook":
		if notebookService == nil {
			return c.RenderJSON(false)
		}
		id := c.Params.String("notebookId")
		parentID := c.Params.String("parentNotebookId")
		if !db.IsValidObjectIDHex(id) || (parentID != "" && !db.IsValidObjectIDHex(parentID)) || !db.IsValidObjectIDHex(userID) {
			return c.RenderJSON(false)
		}
		n := info.Notebook{NotebookId: db.MustObjectIDFromHex(id), Title: c.Params.String("title"), Seq: -1, UserId: db.MustObjectIDFromHex(userID)}
		if parentID != "" {
			n.ParentNotebookId = db.MustObjectIDFromHex(parentID)
		}
		ok, n := notebookService.AddNotebook(n)
		if !ok {
			return c.RenderJSON(false)
		}
		return c.RenderJSON(n)
	case "Notebook.DragNotebooks":
		if notebookService == nil {
			return c.RenderJSON(false)
		}
		var data DragNotebooksInfo
		if err := json.Unmarshal([]byte(c.Params.String("data")), &data); err != nil {
			return c.RenderJSON(false)
		}
		return c.RenderJSON(notebookService.DragNotebooks(userID, data.CurNotebookId, data.ParentNotebookId, data.Siblings, data.OperationId))
	case "Notebook.SortNotebooks":
		if notebookService == nil {
			return c.RenderJSON(false)
		}
		var sequences map[string]int
		if err := json.Unmarshal([]byte(c.Params.String("data")), &sequences); err != nil {
			if err := json.Unmarshal([]byte(c.Params.String("notebookId2Seqs")), &sequences); err != nil {
				return c.RenderJSON(false)
			}
		}
		return c.RenderJSON(notebookService.SortNotebooks(userID, sequences, c.Params.String("OperationId")))
	case "NoteContentHistory.ListHistories":
		if noteContentHistoryService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(noteContentHistoryService.ListHistories(c.Params.String("noteId"), userID))
	case "Album.GetAlbums":
		if albumService == nil {
			return httpserver.JSONResult(http.StatusInternalServerError, []info.Album{})
		}
		albums, err := albumService.ListAlbums(c.Request.Context(), userID)
		if err != nil {
			return httpserver.JSONResult(http.StatusInternalServerError, []info.Album{})
		}
		return c.RenderJSON(albums)
	case "File.GetImages":
		if fileService == nil {
			return httpserver.JSONResult(http.StatusInternalServerError, info.Page{List: []info.File{}})
		}
		page, err := fileService.ListImagesReadable(c.Request.Context(), userID, c.Params.String("albumId"), c.Params.String("key"), pageParam(c), 12)
		if err != nil {
			return httpserver.JSONResult(500, info.Page{List: []info.File{}})
		}
		return c.RenderJSON(page)
	case "Attach.GetAttachs":
		if attachService == nil {
			return c.RenderJSON(info.Re{Msg: "error"})
		}
		items, _, err := attachService.ListReadable(c.Request.Context(), userID, c.Params.String("noteId"))
		if err != nil {
			return c.RenderJSON(info.Re{Msg: "error"})
		}
		re := info.NewRe()
		re.Ok, re.List = true, items
		return c.RenderJSON(re)
	case "Attach.DeleteAttach":
		if attachService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		ok, msg := attachService.DeleteAttachWithOperation(c.Params.String("attachId"), userID, c.Params.String("OperationId"))
		return c.RenderJSON(info.Re{Ok: ok, Msg: msg})
	case "File.UpdateImageTitle":
		if fileService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(info.Re{Ok: fileService.UpdateImageTitleResult(c.Request.Context(), userID, c.Params.String("fileId"), c.Params.String("title")) == nil})
	case "File.DeleteImage":
		if fileService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		ok, msg := fileService.DeleteImageWithOperation(c.Request.Context(), userID, c.Params.String("fileId"), strings.TrimSpace(c.Params.String("OperationId")))
		return c.RenderJSON(info.Re{Ok: ok, Msg: msg})
	case "File.CopyImage":
		if fileService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		if c.Params.String("userId") != userID || c.Params.String("toUserId") != userID {
			return c.RenderJSON(info.NewRe())
		}
		ok, id := fileService.CopyImage(userID, c.Params.String("fileId"), userID)
		return c.RenderJSON(info.Re{Ok: ok, Id: id})
	case "File.CopyHttpImage":
		if fileService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		if configService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		limit, err := configService.GetUploadLimitBytes("uploadImageSize")
		if err != nil {
			return c.RenderJSON(info.Re{Msg: "upload config error"})
		}
		ok, id, msg := fileService.CopyHTTPImage(c.Request.Context(), userID, c.Params.String("src"), limit)
		return c.RenderJSON(info.Re{Ok: ok, Id: id, Msg: msg})
	case "File.OutputImage":
		if fileService == nil {
			return c.RenderText("")
		}
		d, err := fileService.OpenReadableImage(c.Request.Context(), userID, c.Params.String("fileId"))
		if err != nil {
			return c.RenderText("")
		}
		ct := mime.TypeByExtension(filepath.Ext(d.Name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		return notesDownloadResult{status: http.StatusOK, contentType: ct, disposition: "inline", name: d.Name, reader: d.Reader}
	case "Album.DeleteAlbum":
		if albumService == nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		ok, msg := albumService.DeleteAlbum(userID, c.Params.String("albumId"))
		return c.RenderJSON(info.Re{Ok: ok, Msg: msg})
	case "Album.UpdateAlbum":
		if albumService == nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		return c.RenderJSON(info.Re{Ok: albumService.UpdateAlbumResult(c.Request.Context(), userID, c.Params.String("albumId"), c.Params.String("name")) == nil})
	case "Album.Index":
		args := prepareView(c)
		args["userInfo"] = currentUser(c)
		return c.RenderTemplate("album/index.html", args)
	case "Album.AddAlbum":
		if albumService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		a, err := albumService.CreateAlbum(c.Request.Context(), userID, c.Params.String("name"), domain.ObjectID{}, -1)
		if err != nil {
			return c.RenderJSON(false)
		}
		return c.RenderJSON(a)
	}
	return c.RenderJSON(info.Re{Ok: false, Msg: "not_implemented"})
}
