package info

// Code generated from the pre-migration behavior snapshot (Phase 0/1 of 08-25-go-toolchain).
// Source of truth: go vet snapshot + mgo v1.0 behavior probes captured on 2026-08-26.
// DO NOT EDIT expectations by hand; regenerate from a fresh probe instead.
//
// 2026-08-29 (08-25-mongo-driver-migration): the 17 ObjectID fields frozen as
// zeroMarshalError are updated to zeroPresent. mgo's ObjectId (string kind, zero
// value "") failed to marshal when zero; mongo-driver/v2's lea.ObjectID ([12]byte)
// marshals a zero value as ObjectId(000000000000000000000000). Writes of zero
// ObjectID fields failed loudly under mgo, so no working path depended on the old
// behavior; the change is sanctioned as part of the driver migration.
//
// 2026-08-29 (same migration): UserAndBlogUrl's embedded User key moves
// "user" -> "User". mgo lowercased anonymous field names; mongo-driver v2
// keys non-inline anonymous struct fields by their Go type name and ignores
// any tag name, so the emitted key is "User" no matter what. The explicit
// bson tag on the field exists only to satisfy the explicit-tag contract
// scan and was chosen to match the emitted key. Neither UserAndBlog nor
// UserAndBlogUrl is ever inserted into or decoded from MongoDB (view
// assemblies only), so the key casing is inert outside this probe.

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	zeroPresent      = "Present"
	zeroAbsent       = "Absent"
	zeroMarshalError = "MarshalError"
)

type legacyTagSpec struct {
	TypeName   string
	FieldName  string
	BsonKey    string
	OmitEmpty  bool
	IsObjectId bool
	ZeroState  string
}

var legacyTagInventory = []legacyTagSpec{
	{"ApiNoteContent", "Content", "Content", false, false, zeroPresent},
	{"BlogComment", "NoteId", "NoteId", false, true, zeroPresent},
	{"BlogComment", "UserId", "UserId", false, true, zeroPresent},
	{"BlogComment", "Content", "Content", false, false, zeroPresent},
	{"BlogComment", "ToCommentId", "ToCommendId", true, true, zeroAbsent},
	{"BlogComment", "ToUserId", "ToUserId", true, true, zeroAbsent},
	{"BlogComment", "LikeNum", "LikeNum", false, false, zeroPresent},
	{"BlogComment", "LikeUserIds", "LikeUserIds", false, false, zeroPresent},
	{"BlogComment", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"BlogStat", "ReadNum", "ReadNum", true, false, zeroAbsent},
	{"BlogStat", "LikeNum", "LikeNum", true, false, zeroAbsent},
	{"BlogStat", "CommentNum", "CommentNum", true, false, zeroAbsent},
	{"NoteTag", "UserId", "UserId", false, true, zeroPresent},
	{"NoteTag", "Tag", "Tag", false, false, zeroPresent},
	{"NoteTag", "Usn", "Usn", false, false, zeroPresent},
	{"NoteTag", "Count", "Count", false, false, zeroPresent},
	{"NoteTag", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"NoteTag", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"NoteTag", "IsDeleted", "IsDeleted", false, false, zeroPresent},
	{"Tag", "Tags", "Tags", false, false, zeroPresent},
	{"UserBlogComment", "CanComment", "CanComment", false, false, zeroPresent},
	{"UserBlogComment", "CommentType", "CommentType", false, false, zeroPresent},
	{"UserBlogComment", "DisqusId", "DisqusId", false, false, zeroPresent},
	{"ApiNotebook", "Seq", "Seq", false, false, zeroPresent},
	{"ApiNotebook", "Title", "Title", false, false, zeroPresent},
	{"ApiNotebook", "UrlTitle", "UrlTitle", false, false, zeroPresent},
	{"ApiNotebook", "IsBlog", "IsBlog", true, false, zeroAbsent},
	{"ApiNotebook", "CreatedTime", "CreatedTime", true, false, zeroAbsent},
	{"ApiNotebook", "UpdatedTime", "UpdatedTime", true, false, zeroAbsent},
	{"ApiNotebook", "Usn", "Usn", false, false, zeroPresent},
	{"ApiNotebook", "IsDeleted", "IsDeleted", false, false, zeroPresent},
	{"BlogSingle", "UserId", "UserId", false, true, zeroPresent},
	{"BlogSingle", "Title", "Title", false, false, zeroPresent},
	{"BlogSingle", "UrlTitle", "UrlTitle", false, false, zeroPresent},
	{"BlogSingle", "Content", "Content", false, false, zeroPresent},
	{"BlogSingle", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"BlogSingle", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"File", "Name", "Name", false, false, zeroPresent},
	{"File", "Title", "Title", false, false, zeroPresent},
	{"File", "Size", "Size", false, false, zeroPresent},
	{"File", "Type", "Type", false, false, zeroPresent},
	{"File", "Path", "Path", false, false, zeroPresent},
	{"File", "IsDefaultAlbum", "IsDefaultAlbum", false, false, zeroPresent},
	{"File", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Notebook", "Seq", "Seq", false, false, zeroPresent},
	{"Notebook", "Title", "Title", false, false, zeroPresent},
	{"Notebook", "UrlTitle", "UrlTitle", false, false, zeroPresent},
	{"Notebook", "NumberNotes", "NumberNotes", false, false, zeroPresent},
	{"Notebook", "IsTrash", "IsTrash", true, false, zeroAbsent},
	{"Notebook", "IsBlog", "IsBlog", true, false, zeroAbsent},
	{"Notebook", "CreatedTime", "CreatedTime", true, false, zeroAbsent},
	{"Notebook", "UpdatedTime", "UpdatedTime", true, false, zeroAbsent},
	{"Notebook", "Usn", "Usn", false, false, zeroPresent},
	{"Notebook", "IsDeleted", "IsDeleted", false, false, zeroPresent},
	{"UserAndBlog", "Email", "Email", false, false, zeroPresent},
	{"UserAndBlog", "Username", "Username", false, false, zeroPresent},
	{"UserAndBlog", "Logo", "Logo", false, false, zeroPresent},
	{"UserAndBlog", "BlogTitle", "BlogTitle", false, false, zeroPresent},
	{"UserAndBlog", "BlogLogo", "BlogLogo", false, false, zeroPresent},
	{"UserAndBlog", "BlogUrl", "BlogUrl", false, false, zeroPresent},
	{"UserBlogStyle", "Style", "Style", false, false, zeroPresent},
	{"UserBlogStyle", "Css", "Css", false, false, zeroPresent},
	{"Attach", "Name", "Name", false, false, zeroPresent},
	{"Attach", "Title", "Title", false, false, zeroPresent},
	{"Attach", "Size", "Size", false, false, zeroPresent},
	{"Attach", "Type", "Type", false, false, zeroPresent},
	{"Attach", "Path", "Path", false, false, zeroPresent},
	{"Attach", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Group", "UserId", "UserId", false, true, zeroPresent},
	{"Group", "Title", "Title", false, false, zeroPresent},
	{"Group", "UserCount", "UserCount", false, false, zeroPresent},
	{"Group", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Group", "Users", "Users", true, false, zeroAbsent},
	{"NoteContentHistory", "Histories", "Histories", false, false, zeroPresent},
	{"UserAndBlogUrl", "BlogUrl", "BlogUrl", false, false, zeroPresent},
	{"UserAndBlogUrl", "PostUrl", "PostUrl", false, false, zeroPresent},
	{"BlogLike", "NoteId", "NoteId", false, true, zeroPresent},
	{"BlogLike", "UserId", "UserId", false, true, zeroPresent},
	{"BlogLike", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Config", "UserId", "UserId", false, true, zeroPresent},
	{"Config", "Key", "Key", false, false, zeroPresent},
	{"Config", "ValueStr", "ValueStr", true, false, zeroAbsent},
	{"Config", "ValueArr", "ValueArr", true, false, zeroAbsent},
	{"Config", "ValueMap", "ValueMap", true, false, zeroAbsent},
	{"Config", "ValueArrMap", "ValueArrMap", true, false, zeroAbsent},
	{"Config", "IsArr", "IsArr", false, false, zeroPresent},
	{"Config", "IsMap", "IsMap", false, false, zeroPresent},
	{"Config", "IsArrMap", "IsArrMap", false, false, zeroPresent},
	{"Config", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"Report", "NoteId", "NoteId", false, true, zeroPresent},
	{"Report", "UserId", "UserId", false, true, zeroPresent},
	{"Report", "Reason", "Reason", false, false, zeroPresent},
	{"Report", "CommentId", "CommendId", true, true, zeroAbsent},
	{"Report", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Session", "LoginTimes", "LoginTimes", false, false, zeroPresent},
	{"Session", "Captcha", "Captcha", false, false, zeroPresent},
	{"Session", "UserId", "UserId", false, false, zeroPresent},
	{"Session", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Session", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"ShareNote", "ToGroup", "ToGroup", true, false, zeroPresent},
	{"ShareNote", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"TagCount", "UserId", "UserId", false, true, zeroPresent},
	{"TagCount", "Tag", "Tag", false, false, zeroPresent},
	{"TagCount", "IsBlog", "IsBlog", false, false, zeroPresent},
	{"TagCount", "Count", "Count", false, false, zeroPresent},
	{"Album", "Name", "Name", false, false, zeroPresent},
	{"Album", "Type", "Type", false, false, zeroPresent},
	{"Album", "Seq", "Seq", false, false, zeroPresent},
	{"Album", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"EachHistory", "UpdatedUserId", "UpdatedUserId", false, true, zeroPresent},
	{"EachHistory", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"EachHistory", "Content", "Content", false, false, zeroPresent},
	{"GroupUser", "GroupId", "GroupId", false, true, zeroPresent},
	{"GroupUser", "UserId", "UserId", false, true, zeroPresent},
	{"GroupUser", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Note", "Title", "Title", false, false, zeroPresent},
	{"Note", "Desc", "Desc", false, false, zeroPresent},
	{"Note", "Src", "Src", true, false, zeroAbsent},
	{"Note", "ImgSrc", "ImgSrc", false, false, zeroPresent},
	{"Note", "Tags", "Tags", true, false, zeroAbsent},
	{"Note", "IsTrash", "IsTrash", false, false, zeroPresent},
	{"Note", "IsBlog", "IsBlog", true, false, zeroAbsent},
	{"Note", "UrlTitle", "UrlTitle", true, false, zeroAbsent},
	{"Note", "IsRecommend", "IsRecommend", true, false, zeroAbsent},
	{"Note", "IsTop", "IsTop", true, false, zeroAbsent},
	{"Note", "HasSelfDefined", "HasSelfDefined", false, false, zeroPresent},
	{"Note", "ReadNum", "ReadNum", true, false, zeroAbsent},
	{"Note", "LikeNum", "LikeNum", true, false, zeroAbsent},
	{"Note", "CommentNum", "CommentNum", true, false, zeroAbsent},
	{"Note", "IsMarkdown", "IsMarkdown", false, false, zeroPresent},
	{"Note", "AttachNum", "AttachNum", false, false, zeroPresent},
	{"Note", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Note", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"Note", "RecommendTime", "RecommendTime", true, false, zeroAbsent},
	{"Note", "PublicTime", "PublicTime", true, false, zeroAbsent},
	{"Note", "Usn", "Usn", false, false, zeroPresent},
	{"Note", "IsDeleted", "IsDeleted", false, false, zeroPresent},
	{"ShareNotebook", "ToGroup", "ToGroup", true, false, zeroPresent},
	{"ShareNotebook", "CreatedTime", "CreatedTime", true, false, zeroAbsent},
	{"Token", "Email", "Email", false, false, zeroPresent},
	{"Token", "Token", "Token", false, false, zeroPresent},
	{"Token", "Type", "Type", false, false, zeroPresent},
	{"Token", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"User", "Email", "Email", false, false, zeroPresent},
	{"User", "Verified", "Verified", false, false, zeroPresent},
	{"User", "Username", "Username", false, false, zeroPresent},
	{"User", "UsernameRaw", "UsernameRaw", false, false, zeroPresent},
	{"User", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"User", "Logo", "Logo", false, false, zeroPresent},
	{"User", "Theme", "Theme", false, false, zeroPresent},
	{"User", "NotebookWidth", "NotebookWidth", false, false, zeroPresent},
	{"User", "NoteListWidth", "NoteListWidth", false, false, zeroPresent},
	{"User", "MdEditorWidth", "MdEditorWidth", false, false, zeroPresent},
	{"User", "LeftIsMin", "LeftIsMin", false, false, zeroPresent},
	{"User", "ThirdUserId", "ThirdUserId", false, false, zeroPresent},
	{"User", "ThirdUsername", "ThirdUsername", false, false, zeroPresent},
	{"User", "ThirdType", "ThirdType", false, false, zeroPresent},
	{"User", "FromUserId", "FromUserId", true, true, zeroAbsent},
	{"User", "Usn", "Usn", false, false, zeroPresent},
	{"UserBlog", "Logo", "Logo", false, false, zeroPresent},
	{"UserBlog", "Title", "Title", false, false, zeroPresent},
	{"UserBlog", "SubTitle", "SubTitle", false, false, zeroPresent},
	{"UserBlog", "AboutMe", "AboutMe", false, false, zeroPresent},
	{"UserBlog", "CanComment", "CanComment", false, false, zeroPresent},
	{"UserBlog", "CommentType", "CommentType", false, false, zeroPresent},
	{"UserBlog", "DisqusId", "DisqusId", false, false, zeroPresent},
	{"UserBlog", "Style", "Style", false, false, zeroPresent},
	{"UserBlog", "Css", "Css", false, false, zeroPresent},
	{"UserBlog", "ThemeId", "ThemeId", true, true, zeroAbsent},
	{"UserBlog", "CateIds", "CateIds", true, false, zeroAbsent},
	{"UserBlog", "Singles", "Singles", true, false, zeroAbsent},
	{"UserBlog", "PerPageSize", "PerPageSize", true, false, zeroAbsent},
	{"UserBlog", "SortField", "SortField", false, false, zeroPresent},
	{"UserBlog", "IsAsc", "IsAsc", true, false, zeroAbsent},
	{"UserBlog", "SubDomain", "SubDomain", false, false, zeroPresent},
	{"UserBlog", "Domain", "Domain", false, false, zeroPresent},
	{"EmailLog", "Email", "Email", false, false, zeroPresent},
	{"EmailLog", "Subject", "Subject", false, false, zeroPresent},
	{"EmailLog", "Body", "Body", false, false, zeroPresent},
	{"EmailLog", "Msg", "Msg", false, false, zeroPresent},
	{"EmailLog", "Ok", "Ok", false, false, zeroPresent},
	{"EmailLog", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Theme", "UserId", "UserId", false, true, zeroPresent},
	{"Theme", "Name", "Name", false, false, zeroPresent},
	{"Theme", "Version", "Version", false, false, zeroPresent},
	{"Theme", "Author", "Author", false, false, zeroPresent},
	{"Theme", "AuthorUrl", "AuthorUrl", false, false, zeroPresent},
	{"Theme", "Path", "Path", false, false, zeroPresent},
	{"Theme", "Info", "Info", false, false, zeroPresent},
	{"Theme", "IsActive", "IsActive", false, false, zeroPresent},
	{"Theme", "IsDefault", "IsDefault", false, false, zeroPresent},
	{"Theme", "Style", "Style", true, false, zeroAbsent},
	{"Theme", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"Theme", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"UserBlogBase", "Logo", "Logo", false, false, zeroPresent},
	{"UserBlogBase", "Title", "Title", false, false, zeroPresent},
	{"UserBlogBase", "SubTitle", "SubTitle", false, false, zeroPresent},
	{"NoteContent", "IsBlog", "IsBlog", true, false, zeroAbsent},
	{"NoteContent", "Content", "Content", false, false, zeroPresent},
	{"NoteContent", "Abstract", "Abstract", false, false, zeroPresent},
	{"NoteContent", "CreatedTime", "CreatedTime", false, false, zeroPresent},
	{"NoteContent", "UpdatedTime", "UpdatedTime", false, false, zeroPresent},
	{"Suggestion", "UserId", "UserId", false, true, zeroPresent},
	{"Suggestion", "Addr", "Addr", false, false, zeroPresent},
	{"Suggestion", "Suggestion", "Suggestion", false, false, zeroPresent},
}

var fixtureKeySets = map[string]string{
	"ApiNoteContent":     "Content,UserId,_id",
	"BlogComment":        "Content,CreatedTime,LikeNum,LikeUserIds,NoteId,ToCommendId,ToUserId,UserId,_id",
	"BlogStat":           "CommentNum,LikeNum,ReadNum,_id",
	"NoteTag":            "Count,CreatedTime,IsDeleted,Tag,UpdatedTime,UserId,Usn,_id",
	"Tag":                "Tags,_id",
	"UserBlogComment":    "CanComment,CommentType,DisqusId",
	"ApiNotebook":        "CreatedTime,IsBlog,IsDeleted,ParentNotebookId,Seq,Title,UpdatedTime,UrlTitle,UserId,Usn,_id",
	"BlogSingle":         "Content,CreatedTime,Title,UpdatedTime,UrlTitle,UserId,_id",
	"File":               "AlbumId,CreatedTime,FromFileId,IsDefaultAlbum,Name,Path,Size,Title,Type,UserId,_id",
	"Notebook":           "CreatedTime,IsBlog,IsDeleted,IsTrash,NumberNotes,ParentNotebookId,Seq,Title,UpdatedTime,UrlTitle,UserId,Usn,_id",
	"UserAndBlog":        "BlogLogo,BlogTitle,BlogUrl,Email,Logo,Username,_id,blogurls",
	"UserBlogStyle":      "Css,Style",
	"Attach":             "CreatedTime,Name,NoteId,Path,Size,Title,Type,UploadUserId,_id",
	"Group":              "CreatedTime,Title,UserCount,UserId,Users,_id",
	"NoteContentHistory": "Histories,UserId,_id",
	"UserAndBlogUrl":     "BlogUrl,PostUrl,User",
	"BlogLike":           "CreatedTime,NoteId,UserId,_id",
	"Config":             "IsArr,IsArrMap,IsMap,Key,UpdatedTime,UserId,ValueArr,ValueArrMap,ValueMap,ValueStr,_id",
	"Report":             "CommendId,CreatedTime,NoteId,Reason,UserId,_id",
	"Session":            "Captcha,CreatedTime,LoginTimes,SessionId,UpdatedTime,UserId,_id",
	"ShareNote":          "CreatedTime,NoteId,Perm,ToGroup,ToGroupId,ToUserId,UserId,_id",
	"TagCount":           "Count,IsBlog,Tag,UserId,_id",
	"Album":              "CreatedTime,Name,Seq,Type,UserId,_id",
	"EachHistory":        "Content,UpdatedTime,UpdatedUserId",
	"GroupUser":          "CreatedTime,GroupId,UserId,_id",
	"Note":               "AttachNum,CommentNum,CreatedTime,CreatedUserId,Desc,HasSelfDefined,ImgSrc,IsBlog,IsDeleted,IsMarkdown,IsRecommend,IsTop,IsTrash,LikeNum,NotebookId,PublicTime,ReadNum,RecommendTime,Src,Tags,Title,UpdatedTime,UpdatedUserId,UrlTitle,UserId,Usn,_id",
	"ShareNotebook":      "CreatedTime,NotebookId,Perm,Seq,ToGroup,ToGroupId,ToUserId,UserId,_id",
	"Token":              "CreatedTime,Email,Token,Type,_id",
	"User":               "AccountEndTime,AccountStartTime,AccountType,AttachNum,AttachSize,CreatedTime,Email,FromUserId,FullSyncBefore,ImageNum,ImageSize,LeftIsMin,Logo,MaxAttachNum,MaxAttachSize,MaxImageNums,MaxImageSize,MaxPerAttachSize,MdEditorWidth,NoteListWidth,NotebookWidth,Pwd,Theme,ThirdType,ThirdUserId,ThirdUsername,Username,UsernameRaw,Usn,Verified,_id",
	"UserBlog":           "AboutMe,CanComment,CateIds,CommentType,Css,DisqusId,Domain,IsAsc,Logo,PerPageSize,Singles,SortField,Style,SubDomain,SubTitle,ThemeId,ThemePath,Title,_id",
	"EmailLog":           "Body,CreatedTime,Email,Msg,Ok,Subject,_id",
	"Theme":              "Author,AuthorUrl,CreatedTime,Info,IsActive,IsDefault,Name,Path,Style,UpdatedTime,UserId,Version,_id",
	"UserBlogBase":       "Logo,SubTitle,Title",
	"NoteContent":        "Abstract,Content,CreatedTime,IsBlog,UpdatedTime,UpdatedUserId,UserId,_id",
	"Suggestion":         "Addr,Suggestion,UserId,_id",
}

var jsonFullGoldens = map[string]string{
	"Album":              "{\"AlbumId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Name\":\"album-name\",\"Type\":2,\"Seq\":3,\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"ApiNoteContent":     "{\"NoteId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Content\":\"\\u003cp\\u003eapi content\\u003c/p\\u003e\"}",
	"ApiNotebook":        "{\"NotebookId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ParentNotebookId\":\"54d7620d99c37b0306000003\",\"Seq\":4,\"Title\":\"nb-title\",\"UrlTitle\":\"nb-url\",\"IsBlog\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Usn\":7,\"IsDeleted\":true}",
	"ArchiveMonth":       "{\"Month\":5,\"Posts\":[{\"NoteId\":\"54d7620d99c37b0306000032\",\"Title\":\"post-title\",\"UrlTitle\":\"post-url\",\"ImgSrc\":\"pi.png\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"Desc\":\"pd\",\"Abstract\":\"pa\",\"Content\":\"\\u003cp\\u003epc\\u003c/p\\u003e\",\"Tags\":[\"pt\"],\"CommentNum\":1,\"ReadNum\":2,\"LikeNum\":3,\"IsMarkdown\":true}]}",
	"Attach":             "{\"AttachId\":\"54d7620d99c37b0306000001\",\"NoteId\":\"54d7620d99c37b0306000002\",\"UploadUserId\":\"54d7620d99c37b0306000003\",\"Name\":\"attach-name\",\"Title\":\"attach-title\",\"Size\":12345,\"Type\":\"doc\",\"Path\":\"files/x/a.doc\",\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"BlogComment":        "{\"CommentId\":\"54d7620d99c37b0306000001\",\"NoteId\":\"54d7620d99c37b0306000002\",\"UserId\":\"54d7620d99c37b0306000003\",\"Content\":\"comment-content\",\"ToCommentId\":\"54d7620d99c37b0306000004\",\"ToUserId\":\"54d7620d99c37b0306000005\",\"LikeNum\":2,\"LikeUserIds\":[\"54d7620d99c37b0306000006\",\"54d7620d99c37b0306000007\"],\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"BlogItem":           "{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"CreatedUserId\":\"54d7620d99c37b0306000016\",\"NotebookId\":\"54d7620d99c37b0306000017\",\"Title\":\"note-title\",\"Desc\":\"note-desc\",\"Src\":\"note-src\",\"ImgSrc\":\"img.png\",\"Tags\":[\"t1\",\"t2\"],\"IsTrash\":true,\"IsBlog\":true,\"UrlTitle\":\"note-url\",\"IsRecommend\":true,\"IsTop\":true,\"HasSelfDefined\":true,\"ReadNum\":1,\"LikeNum\":2,\"CommentNum\":3,\"IsMarkdown\":true,\"AttachNum\":4,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"RecommendTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\",\"Usn\":42,\"IsDeleted\":false,\"Abstract\":\"abstract-text\",\"Content\":\"\\u003cp\\u003eitem\\u003c/p\\u003e\",\"HasMore\":true,\"User\":{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}}",
	"BlogLike":           "{\"LikeId\":\"54d7620d99c37b0306000001\",\"NoteId\":\"54d7620d99c37b0306000002\",\"UserId\":\"54d7620d99c37b0306000003\",\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"BlogSingle":         "{\"SingleId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Title\":\"single-title\",\"UrlTitle\":\"single-url\",\"Content\":\"\\u003cp\\u003esingle\\u003c/p\\u003e\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"BlogStat":           "{\"NoteId\":\"54d7620d99c37b0306000001\",\"ReadNum\":11,\"LikeNum\":22,\"CommentNum\":33}",
	"Config":             "{\"ConfigId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Key\":\"cfg-key\",\"ValueStr\":\"v-str\",\"ValueArr\":[\"a\",\"b\"],\"ValueMap\":{\"m1\":\"mv\"},\"ValueArrMap\":[{\"am\":\"av\"}],\"IsArr\":true,\"IsMap\":true,\"IsArrMap\":true,\"UpdatedTime\":\"2020-05-06T07:08:09Z\"}",
	"EachHistory":        "{\"UpdatedUserId\":\"54d7620d99c37b0306000001\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Content\":\"history-content\"}",
	"EmailLog":           "{\"LogId\":\"54d7620d99c37b0306000001\",\"Email\":\"a@b.c\",\"Subject\":\"sub\",\"Body\":\"body\",\"Msg\":\"msg\",\"Ok\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"File":               "{\"FileId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"AlbumId\":\"54d7620d99c37b0306000003\",\"Name\":\"file-name\",\"Title\":\"file-title\",\"Size\":999,\"Type\":\"\",\"Path\":\"files/p\",\"IsDefaultAlbum\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"FromFileId\":\"54d7620d99c37b0306000004\"}",
	"Group":              "{\"GroupId\":\"54d7620d99c37b030600001e\",\"UserId\":\"54d7620d99c37b030600001f\",\"Title\":\"group-title\",\"UserCount\":5,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Users\":[{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}]}",
	"GroupUser":          "{\"GroupUserId\":\"54d7620d99c37b0306000001\",\"GroupId\":\"54d7620d99c37b0306000002\",\"UserId\":\"54d7620d99c37b0306000003\",\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"Note":               "{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"CreatedUserId\":\"54d7620d99c37b0306000016\",\"NotebookId\":\"54d7620d99c37b0306000017\",\"Title\":\"note-title\",\"Desc\":\"note-desc\",\"Src\":\"note-src\",\"ImgSrc\":\"img.png\",\"Tags\":[\"t1\",\"t2\"],\"IsTrash\":true,\"IsBlog\":true,\"UrlTitle\":\"note-url\",\"IsRecommend\":true,\"IsTop\":true,\"HasSelfDefined\":true,\"ReadNum\":1,\"LikeNum\":2,\"CommentNum\":3,\"IsMarkdown\":true,\"AttachNum\":4,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"RecommendTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\",\"Usn\":42,\"IsDeleted\":false}",
	"NoteAndContent":     "{\"CreatedUserId\":\"54d7620d99c37b0306000016\",\"NotebookId\":\"54d7620d99c37b0306000017\",\"Title\":\"note-title\",\"Desc\":\"note-desc\",\"Src\":\"note-src\",\"ImgSrc\":\"img.png\",\"Tags\":[\"t1\",\"t2\"],\"IsTrash\":true,\"UrlTitle\":\"note-url\",\"IsRecommend\":true,\"IsTop\":true,\"HasSelfDefined\":true,\"ReadNum\":1,\"LikeNum\":2,\"CommentNum\":3,\"IsMarkdown\":true,\"AttachNum\":4,\"RecommendTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"Usn\":42,\"IsDeleted\":false,\"Content\":\"\\u003cp\\u003ec\\u003c/p\\u003e\",\"Abstract\":\"abs\"}",
	"NoteAndContentSep":  "{\"NoteInfo\":{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"CreatedUserId\":\"54d7620d99c37b0306000016\",\"NotebookId\":\"54d7620d99c37b0306000017\",\"Title\":\"note-title\",\"Desc\":\"note-desc\",\"Src\":\"note-src\",\"ImgSrc\":\"img.png\",\"Tags\":[\"t1\",\"t2\"],\"IsTrash\":true,\"IsBlog\":true,\"UrlTitle\":\"note-url\",\"IsRecommend\":true,\"IsTop\":true,\"HasSelfDefined\":true,\"ReadNum\":1,\"LikeNum\":2,\"CommentNum\":3,\"IsMarkdown\":true,\"AttachNum\":4,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"RecommendTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\",\"Usn\":42,\"IsDeleted\":false},\"NoteContentInfo\":{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"IsBlog\":true,\"Content\":\"\\u003cp\\u003ec\\u003c/p\\u003e\",\"Abstract\":\"abs\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\"}}",
	"NoteContent":        "{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"IsBlog\":true,\"Content\":\"\\u003cp\\u003ec\\u003c/p\\u003e\",\"Abstract\":\"abs\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\"}",
	"NoteContentHistory": "{\"NoteId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Histories\":[{\"UpdatedUserId\":\"54d7620d99c37b0306000003\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Content\":\"h\"}]}",
	"NoteTag":            "{\"TagId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Tag\":\"tag-name\",\"Usn\":9,\"Count\":8,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"IsDeleted\":true}",
	"Notebook":           "{\"NotebookId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ParentNotebookId\":\"54d7620d99c37b0306000003\",\"Seq\":5,\"Title\":\"nb-title\",\"UrlTitle\":\"nb-url\",\"NumberNotes\":12,\"IsTrash\":true,\"IsBlog\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Usn\":13,\"IsDeleted\":true}",
	"Notebooks":          "{\"NotebookId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ParentNotebookId\":\"54d7620d99c37b0306000003\",\"Seq\":6,\"Title\":\"nb-title\",\"UrlTitle\":\"nb-url\",\"NumberNotes\":12,\"IsTrash\":true,\"IsBlog\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Usn\":13,\"IsDeleted\":true,\"Subs\":[{\"NotebookId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ParentNotebookId\":\"54d7620d99c37b0306000003\",\"Seq\":6,\"Title\":\"nb-title\",\"UrlTitle\":\"nb-url\",\"NumberNotes\":12,\"IsTrash\":true,\"IsBlog\":true,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Usn\":13,\"IsDeleted\":true,\"Subs\":null}]}",
	"Report":             "{\"ReportId\":\"54d7620d99c37b0306000001\",\"NoteId\":\"54d7620d99c37b0306000002\",\"UserId\":\"54d7620d99c37b0306000003\",\"Reason\":\"reason\",\"CommentId\":\"54d7620d99c37b0306000004\",\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"Session":            "{\"Id\":\"54d7620d99c37b0306000001\",\"SessionId\":\"sid\",\"LoginTimes\":3,\"Captcha\":\"cap\",\"UserId\":\"user-str-id\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\"}",
	"ShareNote":          "{\"ShareNoteId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ToUserId\":\"54d7620d99c37b0306000003\",\"ToGroupId\":\"54d7620d99c37b0306000004\",\"ToGroup\":{\"GroupId\":\"54d7620d99c37b030600001e\",\"UserId\":\"54d7620d99c37b030600001f\",\"Title\":\"group-title\",\"UserCount\":5,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Users\":[{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}]},\"NoteId\":\"54d7620d99c37b0306000005\",\"Perm\":1,\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"ShareNoteWithPerm":  "{\"NoteId\":\"54d7620d99c37b0306000014\",\"UserId\":\"54d7620d99c37b0306000015\",\"CreatedUserId\":\"54d7620d99c37b0306000016\",\"NotebookId\":\"54d7620d99c37b0306000017\",\"Title\":\"note-title\",\"Desc\":\"note-desc\",\"Src\":\"note-src\",\"ImgSrc\":\"img.png\",\"Tags\":[\"t1\",\"t2\"],\"IsTrash\":true,\"IsBlog\":true,\"UrlTitle\":\"note-url\",\"IsRecommend\":true,\"IsTop\":true,\"HasSelfDefined\":true,\"ReadNum\":1,\"LikeNum\":2,\"CommentNum\":3,\"IsMarkdown\":true,\"AttachNum\":4,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"RecommendTime\":\"2020-05-06T07:08:09Z\",\"PublicTime\":\"2020-05-06T07:08:09Z\",\"UpdatedUserId\":\"54d7620d99c37b0306000018\",\"Usn\":42,\"IsDeleted\":false,\"Perm\":1}",
	"ShareNotebook":      "{\"ShareNotebookId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"ToUserId\":\"54d7620d99c37b0306000003\",\"ToGroupId\":\"54d7620d99c37b0306000004\",\"ToGroup\":{\"GroupId\":\"54d7620d99c37b030600001e\",\"UserId\":\"54d7620d99c37b030600001f\",\"Title\":\"group-title\",\"UserCount\":5,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Users\":[{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}]},\"NotebookId\":\"54d7620d99c37b0306000005\",\"Seq\":6,\"Perm\":1,\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"ShareNotebooks":     "{\"ParentNotebookId\":\"54d7620d99c37b0306000003\",\"Title\":\"nb-title\",\"UrlTitle\":\"nb-url\",\"NumberNotes\":12,\"IsTrash\":true,\"IsBlog\":true,\"UpdatedTime\":\"2020-05-06T07:08:09Z\",\"Usn\":13,\"IsDeleted\":true,\"ShareNotebookId\":\"54d7620d99c37b0306000001\",\"ToUserId\":\"54d7620d99c37b0306000003\",\"ToGroupId\":\"54d7620d99c37b0306000004\",\"ToGroup\":{\"GroupId\":\"54d7620d99c37b030600001e\",\"UserId\":\"54d7620d99c37b030600001f\",\"Title\":\"group-title\",\"UserCount\":5,\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Users\":[{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}]},\"Perm\":1,\"Subs\":null,\"Seq\":6,\"NotebookId\":\"54d7620d99c37b0306000001\",\"IsDefault\":true}",
	"Suggestion":         "{\"Id\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Addr\":\"addr\",\"Suggestion\":\"sugg\"}",
	"Tag":                "{\"UserId\":\"54d7620d99c37b0306000001\",\"Tags\":[\"x\",\"y\"]}",
	"TagCount":           "{\"TagCountId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Tag\":\"tc\",\"IsBlog\":true,\"Count\":7}",
	"Theme":              "{\"ThemeId\":\"54d7620d99c37b0306000001\",\"UserId\":\"54d7620d99c37b0306000002\",\"Name\":\"theme-name\",\"Version\":\"1.0\",\"Author\":\"au\",\"AuthorUrl\":\"url\",\"Path\":\"themes/p\",\"Info\":{\"k\":\"v\"},\"IsActive\":true,\"IsDefault\":true,\"Style\":\"style\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"UpdatedTime\":\"2020-05-06T07:08:09Z\"}",
	"Token":              "{\"UserId\":\"54d7620d99c37b0306000001\",\"Email\":\"t@e.c\",\"Token\":\"tok\",\"Type\":1,\"CreatedTime\":\"2020-05-06T07:08:09Z\"}",
	"User":               "{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":10,\"NoteListWidth\":20,\"MdEditorWidth\":30,\"LeftIsMin\":true,\"ThirdUserId\":\"third-id\",\"ThirdUsername\":\"third-name\",\"ThirdType\":0,\"FromUserId\":\"54d7620d99c37b030600005b\",\"Usn\":60,\"FullSyncBefore\":\"2020-05-06T07:08:09Z\"}",
	"UserAndBlog":        "{\"UserId\":\"54d7620d99c37b0306000001\",\"Email\":\"ub@e.c\",\"Username\":\"ubname\",\"Logo\":\"ub-logo\",\"BlogTitle\":\"bt\",\"BlogLogo\":\"bl\",\"BlogUrl\":\"/blog\",\"IndexUrl\":\"/i\",\"CateUrl\":\"/c\",\"SearchUrl\":\"/s\",\"SingleUrl\":\"/si\",\"PostUrl\":\"/p\",\"ArchiveUrl\":\"/ar\",\"TagsUrl\":\"/t\",\"TagPostsUrl\":\"/tp\"}",
	"UserAndBlogUrl":     "{\"UserId\":\"54d7620d99c37b030600005a\",\"Email\":\"u@e.c\",\"Verified\":true,\"Username\":\"uname\",\"UsernameRaw\":\"uName\",\"CreatedTime\":\"2020-05-06T07:08:09Z\",\"Logo\":\"logo.png\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\",\"BlogUrl\":\"bu\",\"PostUrl\":\"pu\"}",
	"UserBlog":           "{\"UserId\":\"54d7620d99c37b0306000001\",\"Logo\":\"blog-logo\",\"Title\":\"blog-title\",\"SubTitle\":\"blog-sub\",\"AboutMe\":\"about\",\"CanComment\":true,\"CommentType\":\"disqus\",\"DisqusId\":\"disqus-1\",\"Style\":\"style-x\",\"Css\":\".css{}\",\"ThemeId\":\"54d7620d99c37b0306000008\",\"CateIds\":[\"54d7620d99c37b0306000009\"],\"Singles\":[{\"SingleId\":\"54d7620d99c37b030600000a\",\"Title\":\"st\"}],\"PerPageSize\":10,\"SortField\":\"CreatedTime\",\"IsAsc\":true,\"SubDomain\":\"sub\",\"Domain\":\"dom\"}",
	"UserBlogBase":       "{\"Logo\":\"lb\",\"Title\":\"tb\",\"SubTitle\":\"sb\"}",
	"UserBlogComment":    "{\"CanComment\":true,\"CommentType\":\"default\",\"DisqusId\":\"did\"}",
	"UserBlogStyle":      "{\"Style\":\"ss\",\"Css\":\".cc{}\"}",
}

var jsonZeroGoldens = map[string]string{
	"Album":              "{\"AlbumId\":\"\",\"UserId\":\"\",\"Name\":\"\",\"Type\":0,\"Seq\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"ApiNoteContent":     "{\"NoteId\":\"\",\"UserId\":\"\",\"Content\":\"\"}",
	"ApiNotebook":        "{\"NotebookId\":\"\",\"UserId\":\"\",\"ParentNotebookId\":\"\",\"Seq\":0,\"Title\":\"\",\"UrlTitle\":\"\",\"IsBlog\":false,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"Usn\":0,\"IsDeleted\":false}",
	"ArchiveMonth":       "{\"Month\":0,\"Posts\":null}",
	"Attach":             "{\"AttachId\":\"\",\"NoteId\":\"\",\"UploadUserId\":\"\",\"Name\":\"\",\"Title\":\"\",\"Size\":0,\"Type\":\"\",\"Path\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"BlogComment":        "{\"CommentId\":\"\",\"NoteId\":\"\",\"UserId\":\"\",\"Content\":\"\",\"ToCommentId\":\"\",\"ToUserId\":\"\",\"LikeNum\":0,\"LikeUserIds\":null,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"BlogItem":           "{\"NoteId\":\"\",\"UserId\":\"\",\"CreatedUserId\":\"\",\"NotebookId\":\"\",\"Title\":\"\",\"Desc\":\"\",\"Src\":\"\",\"ImgSrc\":\"\",\"Tags\":null,\"IsTrash\":false,\"IsBlog\":false,\"UrlTitle\":\"\",\"IsRecommend\":false,\"IsTop\":false,\"HasSelfDefined\":false,\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0,\"IsMarkdown\":false,\"AttachNum\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"RecommendTime\":\"0001-01-01T00:00:00Z\",\"PublicTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\",\"Usn\":0,\"IsDeleted\":false,\"Abstract\":\"\",\"Content\":\"\",\"HasMore\":false,\"User\":{\"UserId\":\"\",\"Email\":\"\",\"Verified\":false,\"Username\":\"\",\"UsernameRaw\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Logo\":\"\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}}",
	"BlogLike":           "{\"LikeId\":\"\",\"NoteId\":\"\",\"UserId\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"BlogSingle":         "{\"SingleId\":\"\",\"UserId\":\"\",\"Title\":\"\",\"UrlTitle\":\"\",\"Content\":\"\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"BlogStat":           "{\"NoteId\":\"\",\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0}",
	"Config":             "{\"ConfigId\":\"\",\"UserId\":\"\",\"Key\":\"\",\"ValueStr\":\"\",\"ValueArr\":null,\"ValueMap\":null,\"ValueArrMap\":null,\"IsArr\":false,\"IsMap\":false,\"IsArrMap\":false,\"UpdatedTime\":\"0001-01-01T00:00:00Z\"}",
	"EachHistory":        "{\"UpdatedUserId\":\"\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"Content\":\"\"}",
	"EmailLog":           "{\"LogId\":\"\",\"Email\":\"\",\"Subject\":\"\",\"Body\":\"\",\"Msg\":\"\",\"Ok\":false,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"File":               "{\"FileId\":\"\",\"UserId\":\"\",\"AlbumId\":\"\",\"Name\":\"\",\"Title\":\"\",\"Size\":0,\"Type\":\"\",\"Path\":\"\",\"IsDefaultAlbum\":false,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"FromFileId\":\"\"}",
	"Group":              "{\"GroupId\":\"\",\"UserId\":\"\",\"Title\":\"\",\"UserCount\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Users\":null}",
	"GroupUser":          "{\"GroupUserId\":\"\",\"GroupId\":\"\",\"UserId\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"Note":               "{\"NoteId\":\"\",\"UserId\":\"\",\"CreatedUserId\":\"\",\"NotebookId\":\"\",\"Title\":\"\",\"Desc\":\"\",\"Src\":\"\",\"ImgSrc\":\"\",\"Tags\":null,\"IsTrash\":false,\"IsBlog\":false,\"UrlTitle\":\"\",\"IsRecommend\":false,\"IsTop\":false,\"HasSelfDefined\":false,\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0,\"IsMarkdown\":false,\"AttachNum\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"RecommendTime\":\"0001-01-01T00:00:00Z\",\"PublicTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\",\"Usn\":0,\"IsDeleted\":false}",
	"NoteAndContent":     "{\"CreatedUserId\":\"\",\"NotebookId\":\"\",\"Title\":\"\",\"Desc\":\"\",\"Src\":\"\",\"ImgSrc\":\"\",\"Tags\":null,\"IsTrash\":false,\"UrlTitle\":\"\",\"IsRecommend\":false,\"IsTop\":false,\"HasSelfDefined\":false,\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0,\"IsMarkdown\":false,\"AttachNum\":0,\"RecommendTime\":\"0001-01-01T00:00:00Z\",\"PublicTime\":\"0001-01-01T00:00:00Z\",\"Usn\":0,\"IsDeleted\":false,\"Content\":\"\",\"Abstract\":\"\"}",
	"NoteAndContentSep":  "{\"NoteInfo\":{\"NoteId\":\"\",\"UserId\":\"\",\"CreatedUserId\":\"\",\"NotebookId\":\"\",\"Title\":\"\",\"Desc\":\"\",\"Src\":\"\",\"ImgSrc\":\"\",\"Tags\":null,\"IsTrash\":false,\"IsBlog\":false,\"UrlTitle\":\"\",\"IsRecommend\":false,\"IsTop\":false,\"HasSelfDefined\":false,\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0,\"IsMarkdown\":false,\"AttachNum\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"RecommendTime\":\"0001-01-01T00:00:00Z\",\"PublicTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\",\"Usn\":0,\"IsDeleted\":false},\"NoteContentInfo\":{\"NoteId\":\"\",\"UserId\":\"\",\"IsBlog\":false,\"Content\":\"\",\"Abstract\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\"}}",
	"NoteContent":        "{\"NoteId\":\"\",\"UserId\":\"\",\"IsBlog\":false,\"Content\":\"\",\"Abstract\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\"}",
	"NoteContentHistory": "{\"NoteId\":\"\",\"UserId\":\"\",\"Histories\":null}",
	"NoteTag":            "{\"TagId\":\"\",\"UserId\":\"\",\"Tag\":\"\",\"Usn\":0,\"Count\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"IsDeleted\":false}",
	"Notebook":           "{\"NotebookId\":\"\",\"UserId\":\"\",\"ParentNotebookId\":\"\",\"Seq\":0,\"Title\":\"\",\"UrlTitle\":\"\",\"NumberNotes\":0,\"IsTrash\":false,\"IsBlog\":false,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"Usn\":0,\"IsDeleted\":false}",
	"Notebooks":          "{\"NotebookId\":\"\",\"UserId\":\"\",\"ParentNotebookId\":\"\",\"Seq\":0,\"Title\":\"\",\"UrlTitle\":\"\",\"NumberNotes\":0,\"IsTrash\":false,\"IsBlog\":false,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"Usn\":0,\"IsDeleted\":false,\"Subs\":null}",
	"Report":             "{\"ReportId\":\"\",\"NoteId\":\"\",\"UserId\":\"\",\"Reason\":\"\",\"CommentId\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"Session":            "{\"Id\":\"\",\"SessionId\":\"\",\"LoginTimes\":0,\"Captcha\":\"\",\"UserId\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\"}",
	"ShareNote":          "{\"ShareNoteId\":\"\",\"UserId\":\"\",\"ToUserId\":\"\",\"ToGroupId\":\"\",\"ToGroup\":{\"GroupId\":\"\",\"UserId\":\"\",\"Title\":\"\",\"UserCount\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Users\":null},\"NoteId\":\"\",\"Perm\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"ShareNoteWithPerm":  "{\"NoteId\":\"\",\"UserId\":\"\",\"CreatedUserId\":\"\",\"NotebookId\":\"\",\"Title\":\"\",\"Desc\":\"\",\"Src\":\"\",\"ImgSrc\":\"\",\"Tags\":null,\"IsTrash\":false,\"IsBlog\":false,\"UrlTitle\":\"\",\"IsRecommend\":false,\"IsTop\":false,\"HasSelfDefined\":false,\"ReadNum\":0,\"LikeNum\":0,\"CommentNum\":0,\"IsMarkdown\":false,\"AttachNum\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"RecommendTime\":\"0001-01-01T00:00:00Z\",\"PublicTime\":\"0001-01-01T00:00:00Z\",\"UpdatedUserId\":\"\",\"Usn\":0,\"IsDeleted\":false,\"Perm\":0}",
	"ShareNotebook":      "{\"ShareNotebookId\":\"\",\"UserId\":\"\",\"ToUserId\":\"\",\"ToGroupId\":\"\",\"ToGroup\":{\"GroupId\":\"\",\"UserId\":\"\",\"Title\":\"\",\"UserCount\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Users\":null},\"NotebookId\":\"\",\"Seq\":0,\"Perm\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"ShareNotebooks":     "{\"ParentNotebookId\":\"\",\"Title\":\"\",\"UrlTitle\":\"\",\"NumberNotes\":0,\"IsTrash\":false,\"IsBlog\":false,\"UpdatedTime\":\"0001-01-01T00:00:00Z\",\"Usn\":0,\"IsDeleted\":false,\"ShareNotebookId\":\"\",\"ToUserId\":\"\",\"ToGroupId\":\"\",\"ToGroup\":{\"GroupId\":\"\",\"UserId\":\"\",\"Title\":\"\",\"UserCount\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Users\":null},\"Perm\":0,\"Subs\":null,\"Seq\":0,\"NotebookId\":\"\",\"IsDefault\":false}",
	"Suggestion":         "{\"Id\":\"\",\"UserId\":\"\",\"Addr\":\"\",\"Suggestion\":\"\"}",
	"Tag":                "{\"UserId\":\"\",\"Tags\":null}",
	"TagCount":           "{\"TagCountId\":\"\",\"UserId\":\"\",\"Tag\":\"\",\"IsBlog\":false,\"Count\":0}",
	"Theme":              "{\"ThemeId\":\"\",\"UserId\":\"\",\"Name\":\"\",\"Version\":\"\",\"Author\":\"\",\"AuthorUrl\":\"\",\"Path\":\"\",\"Info\":null,\"IsActive\":false,\"IsDefault\":false,\"Style\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"UpdatedTime\":\"0001-01-01T00:00:00Z\"}",
	"Token":              "{\"UserId\":\"\",\"Email\":\"\",\"Token\":\"\",\"Type\":0,\"CreatedTime\":\"0001-01-01T00:00:00Z\"}",
	"User":               "{\"UserId\":\"\",\"Email\":\"\",\"Verified\":false,\"Username\":\"\",\"UsernameRaw\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Logo\":\"\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\"}",
	"UserAndBlog":        "{\"UserId\":\"\",\"Email\":\"\",\"Username\":\"\",\"Logo\":\"\",\"BlogTitle\":\"\",\"BlogLogo\":\"\",\"BlogUrl\":\"\",\"IndexUrl\":\"\",\"CateUrl\":\"\",\"SearchUrl\":\"\",\"SingleUrl\":\"\",\"PostUrl\":\"\",\"ArchiveUrl\":\"\",\"TagsUrl\":\"\",\"TagPostsUrl\":\"\"}",
	"UserAndBlogUrl":     "{\"UserId\":\"\",\"Email\":\"\",\"Verified\":false,\"Username\":\"\",\"UsernameRaw\":\"\",\"CreatedTime\":\"0001-01-01T00:00:00Z\",\"Logo\":\"\",\"Theme\":\"\",\"NotebookWidth\":0,\"NoteListWidth\":0,\"MdEditorWidth\":0,\"LeftIsMin\":false,\"ThirdUserId\":\"\",\"ThirdUsername\":\"\",\"ThirdType\":0,\"FromUserId\":\"\",\"Usn\":0,\"FullSyncBefore\":\"0001-01-01T00:00:00Z\",\"BlogUrl\":\"\",\"PostUrl\":\"\"}",
	"UserBlog":           "{\"UserId\":\"\",\"Logo\":\"\",\"Title\":\"\",\"SubTitle\":\"\",\"AboutMe\":\"\",\"CanComment\":false,\"CommentType\":\"\",\"DisqusId\":\"\",\"Style\":\"\",\"Css\":\"\",\"ThemeId\":\"\",\"CateIds\":null,\"Singles\":null,\"PerPageSize\":0,\"SortField\":\"\",\"IsAsc\":false,\"SubDomain\":\"\",\"Domain\":\"\"}",
	"UserBlogBase":       "{\"Logo\":\"\",\"Title\":\"\",\"SubTitle\":\"\"}",
	"UserBlogComment":    "{\"CanComment\":false,\"CommentType\":\"\",\"DisqusId\":\"\"}",
	"UserBlogStyle":      "{\"Style\":\"\",\"Css\":\"\"}",
}

// contractRegistry mirrors every model type covered by the inventory/goldens.
var contractRegistry = map[string]reflect.Type{
	"ApiNoteContent":     reflect.TypeOf(ApiNoteContent{}),
	"BlogComment":        reflect.TypeOf(BlogComment{}),
	"BlogStat":           reflect.TypeOf(BlogStat{}),
	"NoteTag":            reflect.TypeOf(NoteTag{}),
	"Tag":                reflect.TypeOf(Tag{}),
	"UserBlogComment":    reflect.TypeOf(UserBlogComment{}),
	"ApiNotebook":        reflect.TypeOf(ApiNotebook{}),
	"BlogSingle":         reflect.TypeOf(BlogSingle{}),
	"File":               reflect.TypeOf(File{}),
	"Notebook":           reflect.TypeOf(Notebook{}),
	"UserAndBlog":        reflect.TypeOf(UserAndBlog{}),
	"UserBlogStyle":      reflect.TypeOf(UserBlogStyle{}),
	"Attach":             reflect.TypeOf(Attach{}),
	"Group":              reflect.TypeOf(Group{}),
	"NoteContentHistory": reflect.TypeOf(NoteContentHistory{}),
	"UserAndBlogUrl":     reflect.TypeOf(UserAndBlogUrl{}),
	"BlogLike":           reflect.TypeOf(BlogLike{}),
	"Config":             reflect.TypeOf(Config{}),
	"Report":             reflect.TypeOf(Report{}),
	"Session":            reflect.TypeOf(Session{}),
	"ShareNote":          reflect.TypeOf(ShareNote{}),
	"TagCount":           reflect.TypeOf(TagCount{}),
	"Album":              reflect.TypeOf(Album{}),
	"EachHistory":        reflect.TypeOf(EachHistory{}),
	"GroupUser":          reflect.TypeOf(GroupUser{}),
	"Note":               reflect.TypeOf(Note{}),
	"ShareNotebook":      reflect.TypeOf(ShareNotebook{}),
	"Token":              reflect.TypeOf(Token{}),
	"User":               reflect.TypeOf(User{}),
	"UserBlog":           reflect.TypeOf(UserBlog{}),
	"EmailLog":           reflect.TypeOf(EmailLog{}),
	"Theme":              reflect.TypeOf(Theme{}),
	"UserBlogBase":       reflect.TypeOf(UserBlogBase{}),
	"NoteContent":        reflect.TypeOf(NoteContent{}),
	"Suggestion":         reflect.TypeOf(Suggestion{}),
}

var oidType = reflect.TypeOf(lea.ObjectID{})

func leaMustOid(hex string) lea.ObjectID {
	oid, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return lea.ObjectID(oid)
}

func mustOid(hex string) lea.ObjectID {
	oid, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return lea.ObjectID(oid)
}

var timeType = reflect.TypeOf(time.Time{})

var counter int32

func nextOid() lea.ObjectID {
	counter++
	return mustOid(fmt.Sprintf("54d7620d99c37b0306000%03x", counter%1000))
}

func oid(n int) lea.ObjectID {
	return mustOid(fmt.Sprintf("54d7620d99c37b0306000%03x", n))
}

func ts() time.Time { return time.Date(2020, 5, 6, 7, 8, 9, 0, time.UTC) }

func exportedFields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		out = append(out, f)
	}
	return out
}

func seedAll(v reflect.Value) {
	if v.Type() == oidType {
		if v.String() == "" {
			v.Set(reflect.ValueOf(nextOid()))
		}
		return
	}
	if v.Type() == timeType {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for _, f := range exportedFields(v.Type()) {
			seedAll(v.FieldByIndex(f.Index))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			seedAll(v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			seedAll(v.MapIndex(k))
		}
	}
}

func makeNonZero(t reflect.Type, depth int) reflect.Value {
	if depth > 4 {
		return reflect.Zero(t)
	}
	if t == oidType {
		return reflect.ValueOf(nextOid())
	}
	if t == timeType {
		return reflect.ValueOf(ts())
	}
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf("probe-val")
	case reflect.Bool:
		return reflect.ValueOf(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(7).Convert(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(7).Convert(t)
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(1.5).Convert(t)
	case reflect.Slice:
		s := reflect.MakeSlice(t, 1, 1)
		s.Index(0).Set(makeNonZero(t.Elem(), depth+1))
		return s
	case reflect.Map:
		m := reflect.MakeMap(t)
		mt := m.Type()
		m.SetMapIndex(reflect.Zero(mt.Key()), makeNonZero(mt.Elem(), depth+1))
		return m
	case reflect.Struct:
		z := reflect.New(t).Elem()
		for _, f := range exportedFields(t) {
			z.FieldByIndex(f.Index).Set(makeNonZero(f.Type, depth+1))
		}
		return z
	case reflect.Ptr:
		p := reflect.New(t.Elem())
		p.Elem().Set(makeNonZero(t.Elem(), depth+1))
		return p
	}
	return reflect.Zero(t)
}

func toBsonM(v interface{}) (bson.M, error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m bson.M
	if err := bson.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func legacyFields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for _, f := range exportedFields(t) {
		raw := string(f.Tag)
		if raw == "" || raw == "-" || strings.Contains(raw, ":") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// seedExcept fills every zero ObjectId except the named field (when skipTarget).
func seedExcept(v reflect.Value, skip string, skipTarget bool) {
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == oidType {
			return
		}
		if v.Type() == timeType {
			return
		}
		for _, f := range exportedFields(v.Type()) {
			fv := v.FieldByIndex(f.Index)
			if skipTarget && f.Name == skip {
				continue
			}
			if f.Type == oidType && fv.CanSet() && fv.String() == "" {
				fv.Set(reflect.ValueOf(nextOid()))
				continue
			}
			seedExcept(fv, skip, false)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			seedExcept(v.Index(i), skip, false)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			seedExcept(v.MapIndex(k), skip, false)
		}
	}
}

func miniUser() User {
	return User{
		UserId: oid(90), Email: "u@e.c", Verified: true, Username: "uname",
		UsernameRaw: "uName", Pwd: "pwd-hash", CreatedTime: ts(), Logo: "logo.png",
	}
}

func miniNote() Note {
	return Note{
		NoteId: oid(20), UserId: oid(21), CreatedUserId: oid(22), NotebookId: oid(23),
		Title: "note-title", Desc: "note-desc", Src: "note-src", ImgSrc: "img.png",
		Tags: []string{"t1", "t2"}, IsTrash: true, IsBlog: true, UrlTitle: "note-url",
		IsRecommend: true, IsTop: true, HasSelfDefined: true,
		ReadNum: 1, LikeNum: 2, CommentNum: 3, IsMarkdown: true, AttachNum: 4,
		CreatedTime: ts(), UpdatedTime: ts(), RecommendTime: ts(), PublicTime: ts(),
		UpdatedUserId: oid(24), Usn: 42,
	}
}

func miniNoteContent() NoteContent {
	return NoteContent{
		NoteId: oid(20), UserId: oid(21), IsBlog: true, Content: "<p>c</p>",
		Abstract: "abs", CreatedTime: ts(), UpdatedTime: ts(), UpdatedUserId: oid(24),
	}
}

func miniGroup() Group {
	return Group{GroupId: oid(30), UserId: oid(31), Title: "group-title",
		UserCount: 5, CreatedTime: ts(), Users: []User{miniUser()}}
}

func buildFixtures() map[string]interface{} {
	fix := map[string]interface{}{}
	fix["Album"] = Album{AlbumId: oid(1), UserId: oid(2), Name: "album-name", Type: 2, Seq: 3, CreatedTime: ts()}
	fix["ApiNoteContent"] = ApiNoteContent{NoteId: oid(1), UserId: oid(2), Content: "<p>api content</p>"}
	fix["ApiNotebook"] = ApiNotebook{NotebookId: oid(1), UserId: oid(2), ParentNotebookId: oid(3), Seq: 4, Title: "nb-title", UrlTitle: "nb-url", IsBlog: true, CreatedTime: ts(), UpdatedTime: ts(), Usn: 7, IsDeleted: true}
	fix["Attach"] = Attach{AttachId: oid(1), NoteId: oid(2), UploadUserId: oid(3), Name: "attach-name", Title: "attach-title", Size: 12345, Type: "doc", Path: "files/x/a.doc", CreatedTime: ts()}
	fix["BlogComment"] = BlogComment{CommentId: oid(1), NoteId: oid(2), UserId: oid(3), Content: "comment-content", ToCommentId: oid(4), ToUserId: oid(5), LikeNum: 2, LikeUserIds: []string{oid(6).Hex(), oid(7).Hex()}, CreatedTime: ts()}
	fix["BlogLike"] = BlogLike{LikeId: oid(1), NoteId: oid(2), UserId: oid(3), CreatedTime: ts()}
	fix["BlogSingle"] = BlogSingle{SingleId: oid(1), UserId: oid(2), Title: "single-title", UrlTitle: "single-url", Content: "<p>single</p>", UpdatedTime: ts(), CreatedTime: ts()}
	fix["BlogStat"] = BlogStat{NoteId: oid(1), ReadNum: 11, LikeNum: 22, CommentNum: 33}
	fix["Config"] = Config{ConfigId: oid(1), UserId: oid(2), Key: "cfg-key", ValueStr: "v-str", ValueArr: []string{"a", "b"}, ValueMap: map[string]string{"m1": "mv"}, ValueArrMap: []map[string]string{{"am": "av"}}, IsArr: true, IsMap: true, IsArrMap: true, UpdatedTime: ts()}
	fix["EachHistory"] = EachHistory{UpdatedUserId: oid(1), UpdatedTime: ts(), Content: "history-content"}
	fix["EmailLog"] = EmailLog{LogId: oid(1), Email: "a@b.c", Subject: "sub", Body: "body", Msg: "msg", Ok: true, CreatedTime: ts()}
	fix["File"] = File{FileId: oid(1), UserId: oid(2), AlbumId: oid(3), Name: "file-name", Title: "file-title", Size: 999, Type: "", Path: "files/p", IsDefaultAlbum: true, CreatedTime: ts(), FromFileId: oid(4)}
	fix["Group"] = miniGroup()
	fix["GroupUser"] = GroupUser{GroupUserId: oid(1), GroupId: oid(2), UserId: oid(3), CreatedTime: ts()}
	fix["Note"] = miniNote()
	fix["NoteContent"] = miniNoteContent()
	fix["NoteContentHistory"] = NoteContentHistory{NoteId: oid(1), UserId: oid(2), Histories: []EachHistory{{UpdatedUserId: oid(3), UpdatedTime: ts(), Content: "h"}}}
	fix["Notebook"] = Notebook{NotebookId: oid(1), UserId: oid(2), ParentNotebookId: oid(3), Seq: 5, Title: "nb-title", UrlTitle: "nb-url", NumberNotes: 12, IsTrash: true, IsBlog: true, CreatedTime: ts(), UpdatedTime: ts(), Usn: 13, IsDeleted: true}
	fix["NoteTag"] = NoteTag{TagId: oid(1), UserId: oid(2), Tag: "tag-name", Usn: 9, Count: 8, CreatedTime: ts(), UpdatedTime: ts(), IsDeleted: true}
	fix["Report"] = Report{ReportId: oid(1), NoteId: oid(2), UserId: oid(3), Reason: "reason", CommentId: oid(4), CreatedTime: ts()}
	fix["Session"] = Session{Id: oid(1), SessionId: "sid", LoginTimes: 3, Captcha: "cap", UserId: "user-str-id", CreatedTime: ts(), UpdatedTime: ts()}
	fix["ShareNote"] = ShareNote{ShareNoteId: oid(1), UserId: oid(2), ToUserId: oid(3), ToGroupId: oid(4), ToGroup: miniGroup(), NoteId: oid(5), Perm: 1, CreatedTime: ts()}
	fix["ShareNotebook"] = ShareNotebook{ShareNotebookId: oid(1), UserId: oid(2), ToUserId: oid(3), ToGroupId: oid(4), ToGroup: miniGroup(), NotebookId: oid(5), Seq: 6, Perm: 1, CreatedTime: ts()}
	fix["Suggestion"] = Suggestion{Id: oid(1), UserId: oid(2), Addr: "addr", Suggestion: "sugg"}
	fix["Tag"] = Tag{UserId: oid(1), Tags: []string{"x", "y"}}
	fix["TagCount"] = TagCount{TagCountId: oid(1), UserId: oid(2), Tag: "tc", IsBlog: true, Count: 7}
	fix["Theme"] = Theme{ThemeId: oid(1), UserId: oid(2), Name: "theme-name", Version: "1.0", Author: "au", AuthorUrl: "url", Path: "themes/p", Info: map[string]interface{}{"k": "v"}, IsActive: true, IsDefault: true, Style: "style", CreatedTime: ts(), UpdatedTime: ts()}
	fix["Token"] = Token{UserId: oid(1), Email: "t@e.c", Token: "tok", Type: 1, CreatedTime: ts()}
	u := miniUser()
	u.NotebookWidth, u.NoteListWidth, u.MdEditorWidth, u.LeftIsMin = 10, 20, 30, true
	u.ThirdUserId, u.ThirdUsername, u.ThirdType = "third-id", "third-name", ThirdGithub
	u.ImageNum, u.ImageSize, u.AttachNum, u.AttachSize = 1, 2, 3, 4
	u.FromUserId = oid(91)
	u.AccountType, u.AccountStartTime, u.AccountEndTime = "premium", ts(), ts()
	u.MaxImageNum, u.MaxImageSize, u.MaxAttachNum, u.MaxAttachSize, u.MaxPerAttachSize = 100, 200, 300, 400, 500
	u.Usn, u.FullSyncBefore = 60, ts()
	fix["User"] = u
	fix["UserAndBlog"] = UserAndBlog{
		UserId: oid(1), Email: "ub@e.c", Username: "ubname", Logo: "ub-logo",
		BlogTitle: "bt", BlogLogo: "bl", BlogUrl: "/blog",
		BlogUrls: BlogUrls{IndexUrl: "/i", CateUrl: "/c", SearchUrl: "/s", SingleUrl: "/si", PostUrl: "/p", ArchiveUrl: "/ar", TagsUrl: "/t", TagPostsUrl: "/tp"},
	}
	fix["UserAndBlogUrl"] = UserAndBlogUrl{User: miniUser(), BlogUrl: "bu", PostUrl: "pu"}
	fix["UserBlog"] = UserBlog{
		UserId: oid(1), Logo: "blog-logo", Title: "blog-title", SubTitle: "blog-sub", AboutMe: "about",
		CanComment: true, CommentType: "disqus", DisqusId: "disqus-1",
		Style: "style-x", Css: ".css{}", ThemeId: oid(8), ThemePath: "themes/tp",
		CateIds:     []string{oid(9).Hex()},
		Singles:     []map[string]string{{"Title": "st", "SingleId": oid(10).Hex()}},
		PerPageSize: 10, SortField: "CreatedTime", IsAsc: true,
		SubDomain: "sub", Domain: "dom",
	}
	fix["UserBlogBase"] = UserBlogBase{Logo: "lb", Title: "tb", SubTitle: "sb"}
	fix["UserBlogComment"] = UserBlogComment{CanComment: true, CommentType: "default", DisqusId: "did"}
	fix["UserBlogStyle"] = UserBlogStyle{Style: "ss", Css: ".cc{}"}
	fix["BlogItem"] = BlogItem{Note: miniNote(), Abstract: "abstract-text", Content: "<p>item</p>", HasMore: true, User: miniUser()}
	fix["NoteAndContent"] = NoteAndContent{Note: miniNote(), NoteContent: miniNoteContent()}
	fix["NoteAndContentSep"] = NoteAndContentSep{NoteInfo: miniNote(), NoteContentInfo: miniNoteContent()}
	fix["ShareNoteWithPerm"] = ShareNoteWithPerm{Note: miniNote(), Perm: 1}
	nb := fix["Notebook"].(Notebook)
	sn := fix["ShareNotebook"].(ShareNotebook)
	// align values shared by flattened BSON keys across embedded layers so a
	// marshal->unmarshal round-trip reproduces the original struct exactly
	nb.Seq = sn.Seq
	fix["ShareNotebooks"] = ShareNotebooks{Notebook: nb, ShareNotebook: sn, Seq: sn.Seq, NotebookId: nb.NotebookId, IsDefault: true}
	child := &Notebooks{Notebook: nb}
	fix["Notebooks"] = Notebooks{Notebook: nb, Subs: SubNotebooks{child}}
	post := &Post{NoteId: oid(50).Hex(), Title: "post-title", UrlTitle: "post-url", ImgSrc: "pi.png", CreatedTime: ts(), UpdatedTime: ts(), PublicTime: ts(), Desc: "pd", Abstract: "pa", Content: "<p>pc</p>", Tags: []string{"pt"}, CommentNum: 1, ReadNum: 2, LikeNum: 3, IsMarkdown: true}
	fix["ArchiveMonth"] = ArchiveMonth{Month: 5, Posts: []*Post{post}}
	return fix
}
