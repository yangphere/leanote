package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const (
	infoDir    = "app/info"
	outputPath = ".trellis/tasks/09-08-domain-contracts/research/model-catalog.json"
)

var persistenceTypes = set(
	"Album", "Attach", "BlogComment", "BlogLike", "BlogSingle", "Config",
	"EmailLog", "File", "Group", "GroupUser", "HasShareNote", "Note",
	"NoteContent", "NoteContentHistory", "NoteImage", "Notebook", "NoteTag",
	"Report", "Session", "ShareNote", "ShareNotebook", "Suggestion", "Tag",
	"TagCount", "Theme", "Token", "User", "UserBlog",
)

var apiTypes = set("ApiNote", "ApiNoteContent", "ApiNotebook", "ApiRe", "ApiUser", "AuthOk", "NoteFile", "Re", "ReUpdate")
var requestTypes = set("NoteOrContent", "UserAccount")
var templateTypes = set("Archive", "ArchiveMonth", "BlogCommentPublic", "BlogInfoCustom", "BlogItem", "BlogUrls", "Cate", "Post")
var valueTypes = set("ShareNotebooksByUser", "SubNotebooks", "SubShareNotebooks")

var projectionOnly = set("Group.Users", "ShareNote.ToGroup", "ShareNotebook.ToGroup", "UserBlog.ThemePath")

// These names come from the checked-in contract snapshot. The generator keeps
// the source artifact authoritative while exposing coverage gaps explicitly.
var jsonFixtureTypes = set(
	"Album", "ApiNoteContent", "ApiNotebook", "ArchiveMonth", "Attach", "BlogComment", "BlogItem", "BlogLike", "BlogSingle", "BlogStat", "Config", "EachHistory", "EmailLog", "File", "Group", "GroupUser", "Note", "NoteAndContent", "NoteAndContentSep", "NoteContent", "NoteContentHistory", "NoteTag", "Notebook", "Notebooks", "Report", "Session", "ShareNote", "ShareNoteWithPerm", "ShareNotebook", "ShareNotebooks", "Suggestion", "Tag", "TagCount", "Theme", "Token", "User", "UserAndBlog", "UserAndBlogUrl", "UserBlog", "UserBlogBase", "UserBlogComment", "UserBlogStyle",
	"HasShareNote", "NoteImage",
)
var bsonFixtureTypes = set(
	"Album", "ApiNoteContent", "ApiNotebook", "Attach", "BlogComment", "BlogLike", "BlogSingle", "BlogStat", "Config", "EachHistory", "EmailLog", "File", "Group", "GroupUser", "Note", "NoteContent", "NoteContentHistory", "NoteTag", "Notebook", "Report", "Session", "ShareNote", "ShareNotebook", "Suggestion", "Tag", "TagCount", "Theme", "Token", "User", "UserAndBlog", "UserAndBlogUrl", "UserBlog", "UserBlogBase", "UserBlogComment", "UserBlogStyle",
)

type sourceRef struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type fieldEntry struct {
	Name             string                 `json:"name"`
	Source           sourceRef              `json:"source"`
	Wire             map[string]interface{} `json:"wire"`
	BSON             map[string]interface{} `json:"bson"`
	PersistenceState string                 `json:"persistence_state"`
}

type typeEntry struct {
	Name           string                 `json:"name"`
	Status         string                 `json:"status"`
	PrimaryRole    string                 `json:"primary_role"`
	SecondaryRoles []string               `json:"secondary_roles"`
	ConsumerStatus string                 `json:"consumer_status"`
	Consumers      []string               `json:"consumers"`
	Source         sourceRef              `json:"source"`
	Fields         []fieldEntry           `json:"fields"`
	Fixtures       map[string]interface{} `json:"fixtures"`
}

func set(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func main() {
	entries, err := readTypes()
	if err != nil {
		panic(err)
	}
	if len(entries) != 69 {
		panic(fmt.Sprintf("active type count = %d, want 69", len(entries)))
	}

	catalog := map[string]interface{}{
		"schema_version": "leanote.domain-model-catalog.v1",
		"source": map[string]interface{}{
			"package":        "app/info",
			"generated_from": []string{"app/info/*.go", "app/db/Mgo.go", "conf/routes", "app/controllers/api/*.go", "app/controllers/api/API列表-v0.1.md", "app/tests/golden/**", ".trellis/tasks/09-08-domain-contracts/research/input-contracts.json"},
		},
		"counts": map[string]int{
			"active_types":           len(entries),
			"commented_declarations": 3,
			"business_collections":   28,
			"api_actions":            len(apiVariants()),
		},
		"types":                  entries,
		"commented_declarations": commentedDeclarations(),
		"collections":            collections(),
		"api_variants":           apiVariants(),
		"decisions":              decisions(),
		"fixed_boundaries": []string{
			"D-05: production app/info and DB-independent tests import neither app/lea, Revel, nor the Mongo driver",
		},
	}

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		panic(err)
	}
}

func readTypes() ([]typeEntry, error) {
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, infoDir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}
	pkg := packages["info"]
	if pkg == nil {
		return nil, fmt.Errorf("package info not found")
	}
	structs := make(map[string]*ast.StructType)
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if structure, ok := typeSpec.Type.(*ast.StructType); ok {
					structs[typeSpec.Name.Name] = structure
				}
			}
		}
	}

	entries := make([]typeEntry, 0, 69)
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if !ast.IsExported(typeSpec.Name.Name) {
					continue
				}
				position := fset.Position(typeSpec.Pos())
				role, secondary := classify(typeSpec.Name.Name)
				entry := typeEntry{
					Name:           typeSpec.Name.Name,
					Status:         "active",
					PrimaryRole:    role,
					SecondaryRoles: secondary,
					ConsumerStatus: consumerStatus(role),
					Consumers:      consumers(role),
					Source:         sourceRef{File: filepath.ToSlash(position.Filename), Line: position.Line},
					Fields:         fieldsFor(fset, typeSpec.Name.Name, typeSpec.Type, role, secondary, structs),
					Fixtures: map[string]interface{}{
						"json":   fixtureJSON(typeSpec.Name.Name),
						"bson":   fixtureBSON(typeSpec.Name.Name, role, secondary),
						"binder": fixtureBinder(role),
						"status": "partial",
					},
				}
				entries = append(entries, entry)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func classify(name string) (string, []string) {
	switch {
	case persistenceTypes[name]:
		return "persistence", []string{}
	case apiTypes[name]:
		secondary := []string{}
		if name == "ApiNoteContent" || name == "ApiNotebook" {
			secondary = append(secondary, "persistence")
		}
		if name == "ApiNote" || name == "NoteFile" {
			secondary = append(secondary, "request_input")
		}
		return "api_dto", secondary
	case requestTypes[name]:
		return "request_input", []string{}
	case templateTypes[name]:
		return "template_projection", []string{}
	case valueTypes[name]:
		return "value_type", []string{}
	default:
		return "internal_projection", []string{}
	}
}

func consumers(role string) []string {
	switch role {
	case "persistence":
		return []string{"app/db", "app/service"}
	case "api_dto":
		return []string{"app/controllers/api", "app/service"}
	case "request_input":
		return []string{"app/controllers", "app/controllers/api"}
	case "template_projection":
		return []string{"app/controllers", "app/views"}
	default:
		return []string{"app/service"}
	}
}

func consumerStatus(role string) string {
	if role == "internal_projection" || role == "value_type" {
		return "unknown"
	}
	return "confirmed"
}

func fieldsFor(fset *token.FileSet, typeName string, expression ast.Expr, role string, secondary []string, structs map[string]*ast.StructType) []fieldEntry {
	structure, ok := expression.(*ast.StructType)
	if !ok {
		return []fieldEntry{}
	}
	result := make([]fieldEntry, 0, structure.Fields.NumFields())
	for _, field := range structure.Fields.List {
		names := field.Names
		anonymous := len(names) == 0
		if len(names) == 0 {
			names = []*ast.Ident{{Name: expressionString(fset, field.Type)}}
		}
		for _, name := range names {
			position := fset.Position(field.Pos())
			rawTag := ""
			if field.Tag != nil {
				rawTag = strings.Trim(field.Tag.Value, "`")
			}
			tag := reflect.StructTag(rawTag)
			jsonName := interface{}(name.Name)
			jsonEmbedded := false
			var jsonEmbeddedNames []string
			if anonymous {
				jsonName = nil
				jsonEmbedded = true
				jsonEmbeddedNames = embeddedJSONNames(fset, field.Type, structs)
			}
			if value, ok := tag.Lookup("json"); ok {
				base := strings.Split(value, ",")[0]
				switch base {
				case "-":
					jsonName = nil
				case "":
				default:
					jsonName = base
				}
			}
			var bsonName interface{}
			omitEmpty := false
			if value, ok := tag.Lookup("bson"); ok {
				parts := strings.Split(value, ",")
				bsonName = parts[0]
				for _, part := range parts[1:] {
					omitEmpty = omitEmpty || part == "omitempty"
				}
			}
			state := "not_applicable"
			if role == "persistence" || contains(secondary, "persistence") {
				state = "read_write"
				if projectionOnly[typeName+"."+name.Name] {
					state = "projection_only"
				}
			}
			result = append(result, fieldEntry{
				Name:   name.Name,
				Source: sourceRef{File: filepath.ToSlash(position.Filename), Line: position.Line},
				Wire: map[string]interface{}{
					"json_name":     jsonName,
					"null_behavior": nullBehavior(field.Type),
					"json_embedded": jsonEmbedded,
				},
				BSON: map[string]interface{}{
					"tag":        bsonName,
					"tag_source": "app/info struct tag",
					"omitempty":  omitEmpty,
				},
				PersistenceState: state,
			})
			if anonymous {
				result[len(result)-1].Wire["json_embedded_names"] = jsonEmbeddedNames
			}
		}
	}
	return result
}

func embeddedJSONNames(fset *token.FileSet, expression ast.Expr, structs map[string]*ast.StructType) []string {
	name := expressionString(fset, expression)
	structure := structs[name]
	if structure == nil {
		return []string{}
	}
	result := []string{}
	for _, field := range structure.Fields.List {
		if len(field.Names) == 0 {
			result = append(result, embeddedJSONNames(fset, field.Type, structs)...)
			continue
		}
		for _, fieldName := range field.Names {
			jsonName := fieldName.Name
			if field.Tag != nil {
				tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
				if value, ok := tag.Lookup("json"); ok {
					base := strings.Split(value, ",")[0]
					switch base {
					case "-":
						continue
					case "":
					default:
						jsonName = base
					}
				}
			}
			result = append(result, jsonName)
		}
	}
	return result
}

func expressionString(fset *token.FileSet, expression ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, expression); err != nil {
		return "embedded"
	}
	return strings.TrimPrefix(buffer.String(), "*")
}

func nullBehavior(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.ArrayType:
		if value.Len == nil {
			return "nil -> null; non-nil empty -> []"
		}
	case *ast.MapType:
		return "nil -> null; non-nil empty -> {}"
	case *ast.InterfaceType:
		return "nil -> null; non-nil value must be JSON-compatible"
	case *ast.StarExpr:
		return "nil -> null"
	}
	return "encoding/json default zero value"
}

func fixtureJSON(typeName string) string {
	if jsonFixtureTypes[typeName] {
		return "present"
	}
	return "unknown"
}

func fixtureBSON(typeName, role string, secondary []string) string {
	if bsonFixtureTypes[typeName] {
		return "present"
	}
	if role == "persistence" || contains(secondary, "persistence") {
		return "unknown"
	}
	return "not_applicable"
}

func fixtureBinder(role string) string {
	if role == "request_input" || role == "api_dto" {
		return "unknown"
	}
	return "not_applicable"
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func commentedDeclarations() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "TagNote", "source": sourceRef{File: "app/info/TagInfo.go", Line: 11}, "reason": "commented legacy model; not part of the compiled package"},
		{"name": "TagsCounts", "source": sourceRef{File: "app/info/TagInfo.go", Line: 46}, "reason": "commented legacy sortable alias; not part of the compiled package"},
		{"name": "ShareNotebooksByUsers", "source": sourceRef{File: "app/info/ShareNotebookNoteInfo.go", Line: 122}, "reason": "commented legacy alias; not part of the compiled package"},
	}
}

func collections() []map[string]interface{} {
	models := []struct {
		name   string
		model  interface{}
		line   int
		status string
	}{
		{"notebooks", "Notebook", 114, "read_write"}, {"notes", "Note", 117, "read_write"},
		{"note_contents", "NoteContent", 120, "read_write"}, {"note_content_histories", "NoteContentHistory", 121, "read_write"},
		{"share_notes", "ShareNote", 124, "read_write"}, {"share_notebooks", "ShareNotebook", 125, "read_write"},
		{"has_share_notes", "HasShareNote", 126, "read_write"}, {"users", "User", 129, "read_write"},
		{"groups", "Group", 131, "read_write"}, {"group_users", "GroupUser", 132, "read_write"},
		{"blogs", nil, 135, "declared_only"}, {"tags", "Tag", 138, "read_write"},
		{"note_tags", "NoteTag", 139, "read_write"}, {"tag_count", "TagCount", 140, "read_write"},
		{"user_blogs", "UserBlog", 143, "read_write"}, {"blog_singles", "BlogSingle", 144, "read_write"},
		{"themes", "Theme", 145, "read_write"}, {"tokens", "Token", 148, "read_write"},
		{"suggestions", "Suggestion", 151, "read_write"}, {"albums", "Album", 154, "read_write"},
		{"files", "File", 155, "read_write"}, {"attachs", "Attach", 156, "read_write"},
		{"note_images", "NoteImage", 158, "read_write"}, {"configs", "Config", 160, "read_write"},
		{"email_logs", "EmailLog", 161, "read_write"}, {"blog_likes", "BlogLike", 164, "read_write"},
		{"blog_comments", "BlogComment", 165, "read_write"}, {"reports", "Report", 168, "read_write"},
		{"sessions", "Session", 171, "read_write"},
	}
	result := make([]map[string]interface{}, 0, len(models))
	for _, item := range models {
		indexes := []map[string]interface{}{}
		switch item.name {
		case "share_notes":
			indexes = append(indexes, map[string]interface{}{"fields": []string{"UserId", "ToUserId", "NoteId"}, "status": "historical_intent", "evidence": []string{"app/info/ShareNotebookNoteInfo.go:135-137"}})
		case "has_share_notes":
			indexes = append(indexes, map[string]interface{}{"fields": []string{"UserId", "ToUserId"}, "status": "historical_intent", "evidence": []string{"app/info/ShareNotebookNoteInfo.go:149-152"}})
		}
		result = append(result, map[string]interface{}{
			"name": item.name, "model": item.model, "status": item.status,
			"evidence": []string{fmt.Sprintf("app/db/Mgo.go:%d", item.line)}, "unique_indexes": indexes,
		})
	}
	return result
}

func apiVariants() []map[string]interface{} {
	// The generic route is expanded here instead of inventing a new URL
	// registry. Every active (non-commented) Revel action is listed, while
	// evidence_status=partial records that real HTTP replay is still owned by
	// the interface/delivery tasks. Request/success/failure objects deliberately
	// carry explicit unknown markers where source material does not settle a
	// detail (notably binary errors and binder behavior).
	return []map[string]interface{}{
		apiVariant("/api/auth/login", "ApiAuth.Login", "GET", "not_required", map[string]interface{}{"source": "form", "fields": []string{"email", "pwd"}, "required": []string{"email", "pwd"}}, "AuthOk", "ApiRe", []string{"app/controllers/api/ApiAuthController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/auth/logout", "ApiAuth.Logout", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"token"}, "required": []string{"token"}}, "ApiRe", "ApiRe", []string{"app/controllers/api/ApiAuthController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/auth/register", "ApiAuth.Register", "POST", "not_required", map[string]interface{}{"source": "form", "fields": []string{"email", "pwd"}, "required": []string{"email", "pwd"}}, "ApiRe", "ApiRe", []string{"app/controllers/api/ApiAuthController.go", "app/controllers/api/API列表-v0.1.md"}),

		apiVariant("/api/user/info", "ApiUser.Info", "GET", "required", map[string]interface{}{"source": "session", "fields": []string{}}, "ApiUser", "ApiRe", []string{"app/controllers/api/ApiUserController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/user/updateUsername", "ApiUser.UpdateUsername", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"username"}, "required": []string{"username"}}, "ApiRe", "ApiRe", []string{"app/controllers/api/ApiUserController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/user/updatePwd", "ApiUser.UpdatePwd", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"oldPwd", "pwd"}, "required": []string{"oldPwd", "pwd"}}, "ApiRe", "ApiRe", []string{"app/controllers/api/ApiUserController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/user/getSyncState", "ApiUser.GetSyncState", "POST", "required", map[string]interface{}{"source": "session", "fields": []string{}}, "object{LastSyncUsn,LastSyncTime}", "ApiRe", []string{"app/controllers/api/ApiUserController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/user/updateLogo", "ApiUser.UpdateLogo", "POST", "required", map[string]interface{}{"source": "multipart", "fields": []string{"file"}, "required": []string{"file"}}, "object{Logo:string}", "ApiRe", []string{"app/controllers/api/ApiUserController.go", "app/controllers/api/API列表-v0.1.md"}),

		apiVariant("/api/notebook/getSyncNotebooks", "ApiNotebook.GetSyncNotebooks", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"afterUsn", "maxEntry"}, "optional": []string{"afterUsn", "maxEntry"}}, "[]ApiNotebook", "ApiRe", []string{"app/controllers/api/ApiNotebookController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/notebook/getNotebooks", "ApiNotebook.GetNotebooks", "GET", "required", map[string]interface{}{"source": "session", "fields": []string{}}, "[]ApiNotebook", "ApiRe", []string{"app/controllers/api/ApiNotebookController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/notebook/addNotebook", "ApiNotebook.AddNotebook", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"title", "parentNotebookId", "seq"}, "required": []string{"title", "seq"}, "optional": []string{"parentNotebookId"}}, "ApiNotebook", "Re", []string{"app/controllers/api/ApiNotebookController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/notebook/updateNotebook", "ApiNotebook.UpdateNotebook", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"notebookId", "title", "parentNotebookId", "seq", "usn"}, "required": []string{"notebookId", "usn"}, "optional": []string{"title", "parentNotebookId", "seq"}}, "ApiNotebook", "ApiRe", []string{"app/controllers/api/ApiNotebookController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/notebook/deleteNotebook", "ApiNotebook.DeleteNotebook", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"notebookId", "usn"}, "required": []string{"notebookId", "usn"}}, "ApiRe", "ApiRe", []string{"app/controllers/api/ApiNotebookController.go", "app/controllers/api/API列表-v0.1.md"}),

		apiVariant("/api/note/getSyncNotes", "ApiNote.GetSyncNotes", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"afterUsn", "maxEntry"}, "optional": []string{"afterUsn", "maxEntry"}}, "[]ApiNote", "ApiRe", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/getNotes", "ApiNote.GetNotes", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"notebookId", "page"}, "required": []string{"notebookId"}, "optional": []string{"page"}}, "[]ApiNote", "ApiRe", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/getTrashNotes", "ApiNote.GetTrashNotes", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"page"}, "optional": []string{"page"}}, "[]ApiNote", "ApiRe", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/getNote", "ApiNote.GetNote", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"noteId"}, "required": []string{"noteId"}}, "ApiNote", "ApiRe", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/getNoteAndContent", "ApiNote.GetNoteAndContent", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"noteId"}, "required": []string{"noteId"}}, "ApiNote", "unknown", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/getNoteContent", "ApiNote.GetNoteContent", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"noteId"}, "required": []string{"noteId"}}, "ApiNoteContent", "unknown", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/addNote", "ApiNote.AddNote", "POST", "required", map[string]interface{}{"source": "multipart/form", "body": "ApiNote", "required": []string{"NotebookId", "Title", "Content"}, "optional": []string{"Tags", "Abstract", "IsMarkdown", "Files"}, "notes": []string{"Files body parts use FileDatas[<LocalFileId>]"}}, "ApiNote", "Re", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/updateNote", "ApiNote.UpdateNote", "POST", "required", map[string]interface{}{"source": "multipart/form", "body": "ApiNote", "required": []string{"NoteId", "Usn"}, "optional": []string{"NotebookId", "Title", "Tags", "Content", "Abstract", "IsMarkdown", "IsTrash", "Files"}, "notes": []string{"Tags and Files presence are determined by form keys; runtime binder fixture is downstream"}}, "ApiNote", "ReUpdate", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/deleteTrash", "ApiNote.DeleteTrash", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"noteId", "usn"}, "required": []string{"noteId", "usn"}}, "ReUpdate", "ReUpdate", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/note/exportPdf", "ApiNote.ExportPdf", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"noteId"}, "required": []string{"noteId"}}, "binary", "ApiRe-or-text", []string{"app/controllers/api/ApiNoteController.go", "app/controllers/api/API列表-v0.1.md"}),

		apiVariant("/api/tag/getSyncTags", "ApiTag.GetSyncTags", "GET", "required", map[string]interface{}{"source": "query", "fields": []string{"afterUsn", "maxEntry"}, "optional": []string{"afterUsn", "maxEntry"}}, "[]string", "ApiRe", []string{"app/controllers/api/ApiTagController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/tag/addTag", "ApiTag.AddTag", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"tag"}, "required": []string{"tag"}}, "NoteTag", "Re", []string{"app/controllers/api/ApiTagController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/tag/deleteTag", "ApiTag.DeleteTag", "POST", "required", map[string]interface{}{"source": "form", "fields": []string{"tag", "usn"}, "required": []string{"tag", "usn"}}, "ReUpdate", "ReUpdate", []string{"app/controllers/api/ApiTagController.go", "app/controllers/api/API列表-v0.1.md"}),

		apiVariant("/api/file/getImage", "ApiFile.GetImage", "GET", "not_required", map[string]interface{}{"source": "query", "fields": []string{"fileId"}, "required": []string{"fileId"}}, "binary", "binary-or-text", []string{"app/controllers/api/ApiFileController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/file/getAttach", "ApiFile.GetAttach", "GET", "not_required", map[string]interface{}{"source": "query", "fields": []string{"fileId"}, "required": []string{"fileId"}}, "binary", "binary-or-text", []string{"app/controllers/api/ApiFileController.go", "app/controllers/api/API列表-v0.1.md"}),
		apiVariant("/api/file/getAllAttachs", "ApiFile.GetAllAttachs", "GET", "not_required", map[string]interface{}{"source": "query", "fields": []string{"noteId"}, "required": []string{"noteId"}}, "binary", "text", []string{"app/controllers/api/ApiFileController.go", "app/controllers/api/API列表-v0.1.md"}),
	}
}

func apiVariant(route, action, method, auth string, request map[string]interface{}, successBody, failureBody string, evidence []string) map[string]interface{} {
	return map[string]interface{}{
		"route":           route,
		"action":          action,
		"method":          method,
		"auth":            auth,
		"request":         request,
		"success":         map[string]interface{}{"status": 200, "content_type": contentType(successBody), "body": successBody},
		"failure":         map[string]interface{}{"status": 200, "content_type": contentType(failureBody), "body": failureBody, "unknown": []string{"exact message/status/header behavior requires route fixture"}},
		"evidence_status": "partial",
		"evidence":        evidence,
	}
}

func contentType(body string) string {
	switch {
	case strings.HasPrefix(body, "binary:"):
		return strings.TrimPrefix(body, "binary:")
	case body == "binary" || body == "binary-or-text" || body == "text" || body == "unknown" || strings.Contains(body, "or-text"):
		return "unknown"
	default:
		return "application/json"
	}
}

func decisions() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "D-01", "status": "decided", "question": "notebook/tag delete USN and sync semantics", "decision": "allocate a new user USN, persist an IsDeleted tombstone with that USN, and return it from sync while preserving conflict and HTTP envelopes", "impact": "application-notes must replace the recorded legacy defect", "evidence": []string{"prd.md#已确认决策", "app/tests/harness/usn_test.go"}},
		{"id": "D-02", "status": "decided", "question": "neutral ObjectID package and compatibility lifetime", "decision": "use app/domain.ObjectID, keep BSON conversion in app/db, and retain lea.ObjectID only as a migration alias until downstream migration completes", "impact": "app/info and its DB-independent tests must have no lea, Revel, or Mongo driver dependency", "evidence": []string{"prd.md#已确认决策", "design.md#21-中立值类型"}},
		{"id": "D-03", "status": "decided", "question": "dynamic JSON value domain", "decision": "preserve JSON-compatible values, nil and empty shapes; reject non-encodable values centrally without fallback", "impact": "DTO splitting must preserve wire bytes", "evidence": []string{"prd.md#已确认决策"}},
		{"id": "D-04", "status": "decided", "question": "endpoint error mapping", "decision": "freeze message, status, Content-Type and envelope per route/action; add fixtures for gaps and do not unify codes across endpoints", "impact": "interface-http owns endpoint fixtures and unknown entries", "evidence": []string{"prd.md#已确认决策", "conf/routes"}},
		{"id": "D-06", "status": "decided", "question": "atomic USN and note-save partial writes", "decision": "use atomic increment or CAS retry and prefer one transaction for note metadata, content and USN; otherwise use explicit compensation and return partial_write", "impact": "application-notes and infrastructure-persistence jointly own concurrency and idempotency evidence", "evidence": []string{"prd.md#已确认决策", "app/tests/harness/note_save_contract_test.go"}},
	}
}
