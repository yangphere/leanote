package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// blog
/*
note, notebook都可设为blog
关键是, 怎么得到blog列表? 还要分页

??? 不用新建, 直接使用notes表, 添加IsBlog字段. 新建表 blogs {NoteId, UserId, CreatedTime, IsTop(置顶)}, NoteId, UserId 为unique!!

// 设置一个note为blog
添加到blogs中

// 设置/取消notebook为blog
创建一个note时, 如果其notebookId已设为blog, 那么添加该note到blog中.
设置一个notebook为blog时, 将其下所有的note添加到blogs里 -> 更新其IsBlog为true
取消一个notebook不为blog时, 删除其下的所有note -> 更新其IsBlog为false

*/
type BlogService struct {
}

var ErrPublicBlogNotFound = errors.New("public blog not found")

func publicBlogNote(noteId string) (info.Note, bool) {
	note, err := publicBlogNoteChecked(noteId)
	return note, err == nil && !note.NoteId.IsZero()
}

func publicBlogNoteChecked(noteId string) (info.Note, error) {
	if !db.IsValidObjectIDHex(noteId) {
		return info.Note{}, ErrPublicBlogNotFound
	}
	if db.Notes == nil || db.NoteContents == nil {
		return info.Note{}, db.ErrMongoClientNotInitialized
	}
	note := info.Note{}
	err := db.Notes.Find(bson.M{
		"_id":       db.MustObjectIDFromHex(noteId),
		"IsBlog":    true,
		"IsTrash":   false,
		"IsDeleted": false,
	}).One(&note)
	if note, err = publicBlogNoteLookupResult(note, err); err != nil {
		return info.Note{}, err
	}
	var content info.NoteContent
	err = db.NoteContents.Find(bson.M{
		"_id":    note.NoteId,
		"UserId": note.UserId,
		"IsBlog": true,
	}).One(&content)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.Note{}, ErrPublicBlogNotFound
		}
		return info.Note{}, fmt.Errorf("load public blog projection: %w", err)
	}
	if content.NoteId.IsZero() {
		return info.Note{}, ErrPublicBlogNotFound
	}
	return note, nil
}

func publicBlogNoteLookupResult(note info.Note, err error) (info.Note, error) {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return info.Note{}, ErrPublicBlogNotFound
	}
	if err != nil {
		return info.Note{}, fmt.Errorf("load public blog: %w", err)
	}
	if note.NoteId.IsZero() {
		return info.Note{}, ErrPublicBlogNotFound
	}
	return note, nil
}

func validCommentContent(content string) bool {
	return content != "" &&
		strings.TrimSpace(content) != "" &&
		utf8.ValidString(content) &&
		len(content) <= 8*1024 &&
		utf8.RuneCountInString(content) <= 2000
}

// 得到博客统计信息
// ReadNum, LikeNum, CommentNum
func (this *BlogService) GetBlogStat(noteId string) (stat info.BlogStat) {
	stat, _ = this.GetBlogStatChecked(noteId)
	return stat
}

func (this *BlogService) GetBlogStatChecked(noteId string) (info.BlogStat, error) {
	note, err := publicBlogNoteChecked(noteId)
	if err != nil {
		return info.BlogStat{}, err
	}
	return info.BlogStat{NoteId: note.NoteId, ReadNum: note.ReadNum, LikeNum: note.LikeNum, CommentNum: note.CommentNum}, nil
}

// 通过id或urlTitle得到博客
func (this *BlogService) GetBlogByIdAndUrlTitle(userId string, noteIdOrUrlTitle string) (blog info.BlogItem) {
	blog, _ = this.GetBlogByIdAndUrlTitleChecked(userId, noteIdOrUrlTitle)
	return blog
}

// 得到某博客具体信息
func (this *BlogService) GetBlog(noteId string) (blog info.BlogItem) {
	blog, _ = this.GetBlogChecked(noteId)
	return blog
}

func (this *BlogService) GetBlogByIdAndUrlTitleChecked(userId, noteIdOrUrlTitle string) (info.BlogItem, error) {
	if !db.IsValidObjectIDHex(userId) {
		return info.BlogItem{}, ErrPublicBlogNotFound
	}
	if !db.IsValidObjectIDHex(noteIdOrUrlTitle) && noteIdOrUrlTitle == "" {
		return info.BlogItem{}, ErrPublicBlogNotFound
	}
	if db.Notes == nil || db.NoteContents == nil {
		return info.BlogItem{}, db.ErrMongoClientNotInitialized
	}

	query := bson.M{
		"UserId":    db.MustObjectIDFromHex(userId),
		"IsBlog":    true,
		"IsTrash":   false,
		"IsDeleted": false,
	}
	if IsObjectId(noteIdOrUrlTitle) {
		query["_id"] = db.MustObjectIDFromHex(noteIdOrUrlTitle)
	} else {
		query["UrlTitle"] = encodeValue(noteIdOrUrlTitle)
	}
	note := info.Note{}
	err := db.Notes.Find(query).One(&note)
	if _, err := publicBlogNoteLookupResult(note, err); err != nil {
		return info.BlogItem{}, err
	}
	return this.GetBlogItemChecked(note)
}

func (this *BlogService) GetBlogChecked(noteId string) (info.BlogItem, error) {
	note, err := publicBlogNoteChecked(noteId)
	if err != nil {
		return info.BlogItem{}, err
	}
	return this.GetBlogItemChecked(note)
}

func (this *BlogService) GetBlogItemChecked(note info.Note) (info.BlogItem, error) {
	if note.NoteId.IsZero() || !note.IsBlog || note.IsTrash || note.IsDeleted {
		return info.BlogItem{}, ErrPublicBlogNotFound
	}
	if db.NoteContents == nil {
		return info.BlogItem{}, db.ErrMongoClientNotInitialized
	}
	noteContent := info.NoteContent{}
	err := db.NoteContents.Find(bson.M{
		"_id":    note.NoteId,
		"UserId": note.UserId,
		"IsBlog": true,
	}).One(&noteContent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.BlogItem{}, fmt.Errorf("public blog content not found: %w", ErrPublicBlogNotFound)
		}
		return info.BlogItem{}, fmt.Errorf("load public blog content: %w", err)
	}
	if noteContent.NoteId.IsZero() {
		return info.BlogItem{}, fmt.Errorf("public blog content not found: %w", ErrPublicBlogNotFound)
	}
	return info.BlogItem{
		Note:     note,
		Abstract: noteContent.Abstract,
		Content:  noteContent.Content,
		HasMore:  false,
		User:     info.User{},
	}, nil
}
func (this *BlogService) GetBlogItem(note info.Note) (blog info.BlogItem) {
	if note.NoteId.IsZero() || !note.IsBlog {
		return info.BlogItem{}
	}

	// 内容
	noteContent := noteService.GetNoteContent(note.NoteId.Hex(), note.UserId.Hex())

	// 组装成blogItem
	blog = info.BlogItem{Note: note, Abstract: noteContent.Abstract, Content: noteContent.Content, HasMore: false, User: info.User{}}

	return
}

// 得到用户共享的notebooks
// 3/19 博客不是deleted
func (this *BlogService) ListBlogNotebooks(userId string) []info.Notebook {
	notebooks, _ := this.ListBlogNotebooksChecked(userId)
	return notebooks
}

func (this *BlogService) ListBlogNotebooksChecked(userId string) ([]info.Notebook, error) {
	if !db.IsValidObjectIDHex(userId) || db.Notebooks == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	notebooks := []info.Notebook{}
	orQ := []bson.M{
		bson.M{"IsDeleted": false},
		bson.M{"IsDeleted": bson.M{"$exists": false}},
	}
	if err := db.Notebooks.Find(bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsBlog": true, "$or": orQ}).All(&notebooks); err != nil {
		return nil, fmt.Errorf("list blog notebooks: %w", err)
	}
	return notebooks, nil
}

// 博客列表
// userId 表示谁的blog
func (this *BlogService) ListBlogs(userId, notebookId string, page, pageSize int, sortField string, isAsc bool) (info.Page, []info.BlogItem) {
	count, notes := noteService.ListNotes(userId, notebookId, false, page, pageSize, sortField, isAsc, true)

	if notes == nil || len(notes) == 0 {
		return info.Page{}, nil
	}

	// 得到content, 并且每个都要substring
	noteIds := make([]ObjectID, len(notes))
	for i, note := range notes {
		noteIds[i] = note.NoteId
	}

	// 直接得到noteContents表的abstract
	// 这里可能是乱序的
	noteContents := noteService.ListNoteAbstractsByNoteIds(noteIds) // 返回[info.NoteContent]
	noteContentsMap := make(map[ObjectID]info.NoteContent, len(noteContents))
	for _, noteContent := range noteContents {
		noteContentsMap[noteContent.NoteId] = noteContent
	}

	// 组装成blogItem
	// 按照notes的顺序
	blogs := make([]info.BlogItem, len(noteIds))
	for i, note := range notes {
		hasMore := true
		var content string
		var abstract string
		if noteContent, ok := noteContentsMap[note.NoteId]; ok {
			abstract = noteContent.Abstract
			content = noteContent.Content
		}
		blogs[i] = info.BlogItem{Note: note, Abstract: abstract, Content: content, HasMore: hasMore, User: info.User{}}
	}

	pageInfo := info.NewPage(page, pageSize, count, nil)

	return pageInfo, blogs
}

func (this *BlogService) ListBlogsChecked(userId, notebookId string, page, pageSize int, sortField string, isAsc bool) (info.Page, []info.BlogItem, error) {
	if !db.IsValidObjectIDHex(userId) || db.Notes == nil || db.NoteContents == nil {
		return info.Page{}, nil, db.ErrMongoClientNotInitialized
	}
	if page < 1 || page > MaxBlogPage || pageSize < 1 || pageSize > MaxBlogPageSize {
		return info.Page{}, nil, ErrInvalidBlogQuery
	}
	sortField = NormalizeBlogSortField(sortField)
	query := bson.M{
		"UserId":    db.MustObjectIDFromHex(userId),
		"IsTrash":   false,
		"IsDeleted": false,
		"IsBlog":    true,
	}
	if notebookId != "" {
		if !db.IsValidObjectIDHex(notebookId) {
			return info.Page{}, nil, fmt.Errorf("%w: notebookId", ErrInvalidBlogQuery)
		}
		query["NotebookId"] = db.MustObjectIDFromHex(notebookId)
	}
	q := db.Notes.Find(query)
	count, err := q.Count()
	if err != nil {
		return info.Page{}, nil, fmt.Errorf("count public blogs: %w", err)
	}
	notes := []info.Note{}
	if err := q.Sort(BlogSortFields(sortField, isAsc)...).Skip((page - 1) * pageSize).Limit(pageSize).All(&notes); err != nil {
		return info.Page{}, nil, fmt.Errorf("list public blogs: %w", err)
	}
	blogs, err := this.blogItemsFromNotesChecked(notes)
	if err != nil {
		return info.Page{}, nil, err
	}
	return info.NewPage(page, pageSize, count, nil), blogs, nil
}

func (this *BlogService) blogItemsFromNotesChecked(notes []info.Note) ([]info.BlogItem, error) {
	if len(notes) == 0 {
		return []info.BlogItem{}, nil
	}
	noteIDs := make([]ObjectID, len(notes))
	for i, note := range notes {
		noteIDs[i] = note.NoteId
	}
	contents := []info.NoteContent{}
	if err := db.NoteContents.Find(bson.M{"_id": bson.M{"$in": noteIDs}, "UserId": notes[0].UserId, "IsBlog": true}).All(&contents); err != nil {
		return nil, fmt.Errorf("load public blog contents: %w", err)
	}
	contentByNote := make(map[ObjectID]info.NoteContent, len(contents))
	for _, content := range contents {
		contentByNote[content.NoteId] = content
	}
	blogs := make([]info.BlogItem, len(notes))
	for i, note := range notes {
		content, ok := contentByNote[note.NoteId]
		if !ok || content.NoteId.IsZero() || !content.IsBlog {
			return nil, fmt.Errorf("public blog content not found for %s: %w", note.NoteId.Hex(), ErrPublicBlogNotFound)
		}
		blogs[i] = info.BlogItem{Note: note, Abstract: content.Abstract, Content: content.Content, HasMore: true}
	}
	return blogs, nil
}

// 得到博客的标签, 那得先得到所有博客, 比较慢
/*
[
	{Tag:xxx, Count: 32}
]
*/
func (this *BlogService) GetBlogTags(userId string) []info.TagCount {
	tags, _ := this.GetBlogTagsChecked(userId)
	return tags
}

func (this *BlogService) GetBlogTagsChecked(userId string) ([]info.TagCount, error) {
	if !db.IsValidObjectIDHex(userId) || db.TagCounts == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	// 得到所有博客
	tagCounts := []info.TagCount{}
	// tag不能为空
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsBlog": true, "Tag": bson.M{"$ne": ""}}
	if err := db.TagCounts.Find(query).Sort("-Count").All(&tagCounts); err != nil {
		return nil, fmt.Errorf("list blog tags: %w", err)
	}
	return tagCounts, nil
}

// 重新计算博客的标签
// 在设置设置/取消为博客时调用
func (this *BlogService) ReCountBlogTags(userId string) bool {
	if !db.IsValidObjectIDHex(userId) {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	before, err := loadBlogTagCounts(context.Background(), ownerID)
	if err != nil {
		return false
	}
	return replaceBlogTagCountsContext(context.Background(), ownerID, before) == nil
}

// 归档博客
/*
数据: 按年汇总
[
archive1,
archive2,
]
archive的数据类型是
{
Year: 2014
Posts: []
}
*/
func (this *BlogService) ListBlogsArchive(userId, notebookId string, year, month int, sortField string, isAsc bool) []info.Archive {
	archives, _ := this.ListBlogsArchiveChecked(userId, notebookId, year, month, sortField, isAsc)
	return archives
}

func (this *BlogService) ListBlogsArchiveChecked(userId, notebookId string, year, month int, sortField string, isAsc bool) ([]info.Archive, error) {
	if !db.IsValidObjectIDHex(userId) || db.Notes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	if year < 0 || month < 0 || month > 12 {
		return nil, fmt.Errorf("%w: archive date", ErrInvalidBlogQuery)
	}
	if notebookId != "" && !db.IsValidObjectIDHex(notebookId) {
		return nil, fmt.Errorf("%w: notebookId", ErrInvalidBlogQuery)
	}
	sortField = NormalizeBlogSortField(sortField)
	//	_, notes := noteService.ListNotes(userId, notebookId, false, 1, 99999, sortField, isAsc, true);
	q := bson.M{"UserId": db.MustObjectIDFromHex(userId), "IsBlog": true, "IsTrash": false, "IsDeleted": false}
	if notebookId != "" {
		q["NotebookId"] = db.MustObjectIDFromHex(notebookId)
	}
	if year > 0 {
		now := time.Now()
		nextYear := year
		nextMonth := month
		if month == 0 {
			month = 1
			nextYear = year + 1
			nextMonth = month
		} else if month >= 12 {
			month = 12
			nextYear = year + 1
			nextMonth = 1
		} else { // month 在1-12之间
			nextMonth = month + 1
		}
		leftT := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
		rightT := time.Date(nextYear, time.Month(nextMonth), 1, 0, 0, 0, 0, now.Location())
		if sortField == "CreatedTime" || sortField == "UpdatedTime" {
			q[sortField] = bson.M{"$gte": leftT, "$lt": rightT}
		} else {
			q["PublicTime"] = bson.M{"$gte": leftT, "$lt": rightT}
		}
	}

	notes := []info.Note{}
	if err := db.Notes.Find(q).Sort(BlogSortFields(sortField, isAsc)...).All(&notes); err != nil {
		return nil, fmt.Errorf("list blog archive: %w", err)
	}

	if notes == nil || len(notes) == 0 {
		return nil, nil
	}

	postsByYear := map[int][]*info.Post{}
	postsByMonth := map[int]map[int][]*info.Post{}
	for _, note := range notes {
		t := archiveNoteTime(note, sortField)
		year := t.Year()
		month := int(t.Month())
		pt := this.FixNote(note)
		p := &pt
		postsByYear[year] = append(postsByYear[year], p)
		if postsByMonth[year] == nil {
			postsByMonth[year] = map[int][]*info.Post{}
		}
		postsByMonth[year][month] = append(postsByMonth[year][month], p)
	}

	years := make([]int, 0, len(postsByYear))
	for year := range postsByYear {
		years = append(years, year)
	}
	sort.Ints(years)
	if !isAsc {
		reverseInts(years)
	}
	arcs := make([]info.Archive, 0, len(years))
	for _, year := range years {
		months := make([]int, 0, len(postsByMonth[year]))
		for month := range postsByMonth[year] {
			months = append(months, month)
		}
		sort.Ints(months)
		if !isAsc {
			reverseInts(months)
		}
		monthArchives := make([]info.ArchiveMonth, 0, len(months))
		for _, month := range months {
			monthArchives = append(monthArchives, info.ArchiveMonth{Month: month, Posts: postsByMonth[year][month]})
		}
		arcs = append(arcs, info.Archive{Year: year, Posts: postsByYear[year], MonthAchives: monthArchives})
	}

	return arcs, nil
}

func archiveNoteTime(note info.Note, sortField string) time.Time {
	switch sortField {
	case "CreatedTime":
		return note.CreatedTime
	case "UpdatedTime":
		return note.UpdatedTime
	default:
		return note.PublicTime
	}
}

func reverseInts(values []int) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

// 根据tag搜索博客
func (this *BlogService) SearchBlogByTags(tags []string, userId string, pageNumber, pageSize int, sortField string, isAsc bool) (pageInfo info.Page, blogs []info.BlogItem) {
	pageInfo, blogs, _ = this.SearchBlogByTagsChecked(tags, userId, pageNumber, pageSize, sortField, isAsc)
	return pageInfo, blogs
}

func (this *BlogService) SearchBlogByTagsChecked(tags []string, userId string, pageNumber, pageSize int, sortField string, isAsc bool) (pageInfo info.Page, blogs []info.BlogItem, err error) {
	if !db.IsValidObjectIDHex(userId) || db.Notes == nil || db.NoteContents == nil {
		return info.Page{}, nil, db.ErrMongoClientNotInitialized
	}
	if pageNumber < 1 || pageNumber > MaxBlogPage || pageSize < 1 || pageSize > MaxBlogPageSize {
		return info.Page{}, nil, ErrInvalidBlogQuery
	}
	notes := []info.Note{}
	sortField = NormalizeBlogSortField(sortField)
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)

	// 不是trash的
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId),
		"IsTrash":   false,
		"IsDeleted": false,
		"IsBlog":    true,
		"Tags":      bson.M{"$all": tags}}

	q := db.Notes.Find(query)

	// 总记录数
	count, err := q.Count()
	if err != nil {
		return info.Page{}, nil, fmt.Errorf("count blog tags: %w", err)
	}
	if count == 0 {
		return info.NewPage(pageNumber, pageSize, 0, nil), []info.BlogItem{}, nil
	}

	if err := q.Sort(append([]string{sortFieldR}, BlogSortFields(sortField, isAsc)[1])...).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes); err != nil {
		return info.Page{}, nil, fmt.Errorf("list blog tags: %w", err)
	}

	blogs, err = this.blogItemsFromNotesChecked(notes)
	if err != nil {
		return info.Page{}, nil, err
	}
	pageInfo = info.NewPage(pageNumber, pageSize, count, nil)

	return pageInfo, blogs, nil
}

func (this *BlogService) notes2BlogItems(notes []info.Note) []info.BlogItem {
	// 得到content, 并且每个都要substring
	noteIds := make([]ObjectID, len(notes))
	for i, note := range notes {
		noteIds[i] = note.NoteId
	}

	// 直接得到noteContents表的abstract
	// 这里可能是乱序的
	noteContents := noteService.ListNoteContentByNoteIds(noteIds) // 返回[info.NoteContent]
	noteContentsMap := make(map[ObjectID]info.NoteContent, len(noteContents))
	for _, noteContent := range noteContents {
		noteContentsMap[noteContent.NoteId] = noteContent
	}

	// 组装成blogItem
	// 按照notes的顺序
	blogs := make([]info.BlogItem, len(noteIds))
	for i, note := range notes {
		hasMore := true
		var content, abstract string
		if noteContent, ok := noteContentsMap[note.NoteId]; ok {
			abstract = noteContent.Abstract
			content = noteContent.Content
		}
		blogs[i] = info.BlogItem{Note: note, Abstract: abstract, Content: content, HasMore: hasMore, User: info.User{}}
	}
	return blogs
}
func (this *BlogService) SearchBlog(key, userId string, page, pageSize int, sortField string, isAsc bool) (info.Page, []info.BlogItem) {
	key, err := NormalizeBlogText(key, MaxBlogKeywordsRunes, MaxBlogKeywordsBytes)
	if err != nil {
		return info.Page{}, nil
	}
	sortField = NormalizeBlogSortField(sortField)
	count, notes := noteService.SearchNote(key, userId, page, pageSize, sortField, isAsc, true)

	if notes == nil || len(notes) == 0 {
		return info.Page{}, nil
	}

	blogs := this.notes2BlogItems(notes)
	pageInfo := info.NewPage(page, pageSize, count, nil)
	return pageInfo, blogs
}

func (this *BlogService) SearchBlogChecked(key, userId string, page, pageSize int, sortField string, isAsc bool) (info.Page, []info.BlogItem, error) {
	if !db.IsValidObjectIDHex(userId) || db.Notes == nil || db.NoteContents == nil {
		return info.Page{}, nil, db.ErrMongoClientNotInitialized
	}
	key, err := NormalizeBlogText(key, MaxBlogKeywordsRunes, MaxBlogKeywordsBytes)
	if err != nil {
		return info.Page{}, nil, fmt.Errorf("%w: keywords", ErrInvalidBlogQuery)
	}
	if page < 1 || page > MaxBlogPage || pageSize < 1 || pageSize > MaxBlogPageSize {
		return info.Page{}, nil, ErrInvalidBlogQuery
	}
	sortField = NormalizeBlogSortField(sortField)
	ownerID := db.MustObjectIDFromHex(userId)
	query := bson.M{
		"UserId":    ownerID,
		"IsTrash":   false,
		"IsDeleted": false,
		"IsBlog":    true,
	}
	if key != "" {
		pattern := bson.Regex{Pattern: ".*?" + regexp.QuoteMeta(key) + ".*", Options: "i"}
		contentIDs := []ObjectID{}
		if err := db.NoteContents.Find(bson.M{"UserId": ownerID, "IsBlog": true, "Content": bson.M{"$regex": pattern}}).Select(bson.M{"_id": true}).All(&contentIDs); err != nil {
			return info.Page{}, nil, fmt.Errorf("search public blog contents: %w", err)
		}
		query["$or"] = []bson.M{
			{"Title": bson.M{"$regex": pattern}},
			{"Desc": bson.M{"$regex": pattern}},
			{"_id": bson.M{"$in": contentIDs}},
		}
	}
	q := db.Notes.Find(query)
	count, err := q.Count()
	if err != nil {
		return info.Page{}, nil, fmt.Errorf("count public blog search: %w", err)
	}
	notes := []info.Note{}
	if err := q.Sort(BlogSortFields(sortField, isAsc)...).Skip((page - 1) * pageSize).Limit(pageSize).All(&notes); err != nil {
		return info.Page{}, nil, fmt.Errorf("search public blogs: %w", err)
	}
	blogs, err := this.blogItemsFromNotesChecked(notes)
	if err != nil {
		return info.Page{}, nil, err
	}
	return info.NewPage(page, pageSize, count, nil), blogs, nil
}

// 上一篇文章, 下一篇文章
// sorterField, baseTime是基准, sorterField=PublicTime, title
// isAsc是用户自定义的排序方式
func (this *BlogService) PreNextBlog(userId string, sorterField string, isAsc bool, noteId string, baseTime interface{}) (info.Post, info.Post) {
	prePost, nextPost, _ := this.PreNextBlogChecked(userId, sorterField, isAsc, noteId, baseTime)
	return prePost, nextPost
}

func (this *BlogService) PreNextBlogChecked(userId string, sorterField string, isAsc bool, noteId string, baseTime interface{}) (info.Post, info.Post, error) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(noteId) {
		return info.Post{}, info.Post{}, ErrPublicBlogNotFound
	}
	if err := ValidateBlogSortField(sorterField); err != nil || baseTime == nil {
		return info.Post{}, info.Post{}, ErrInvalidBlogQuery
	}
	if db.Notes == nil {
		return info.Post{}, info.Post{}, db.ErrMongoClientNotInitialized
	}

	userIdO := db.MustObjectIDFromHex(userId)
	noteIdO := db.MustObjectIDFromHex(noteId)
	currentQuery := bson.M{
		"_id":       noteIdO,
		"UserId":    userIdO,
		"IsTrash":   false,
		"IsDeleted": false,
		"IsBlog":    true,
	}
	currentNote := info.Note{}
	if err := db.Notes.Find(currentQuery).One(&currentNote); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.Post{}, info.Post{}, ErrPublicBlogNotFound
		}
		return info.Post{}, info.Post{}, fmt.Errorf("load current public blog: %w", err)
	}

	currentValue := blogSortValue(currentNote, sorterField)
	previousQuery, previousSort := blogNeighborQuery(userIdO, sorterField, noteIdO, currentValue, true, isAsc)
	note := info.Note{}
	if err := db.Notes.Find(previousQuery).Sort(previousSort...).Limit(1).One(&note); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return info.Post{}, info.Post{}, fmt.Errorf("load previous public blog: %w", err)
	}

	note2 := info.Note{}
	nextQuery, nextSort := blogNeighborQuery(userIdO, sorterField, noteIdO, currentValue, false, isAsc)
	if err := db.Notes.Find(nextQuery).Sort(nextSort...).Limit(1).One(&note2); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return info.Post{}, info.Post{}, fmt.Errorf("load next public blog: %w", err)
	}

	return this.FixNote(note), this.FixNote(note2), nil
}

func blogSortValue(note info.Note, sortField string) interface{} {
	switch sortField {
	case "CreatedTime":
		return note.CreatedTime
	case "UpdatedTime":
		return note.UpdatedTime
	case "Title":
		return note.Title
	default:
		return note.PublicTime
	}
}

func blogNeighborQuery(userID ObjectID, sortField string, currentID ObjectID, currentValue interface{}, previous, isAsc bool) (bson.M, []string) {
	operator := "$gt"
	if isAsc == previous {
		operator = "$lt"
	}
	query := bson.M{
		"UserId":    userID,
		"IsTrash":   false,
		"IsDeleted": false,
		"IsBlog":    true,
		"$or": []bson.M{
			{sortField: bson.M{operator: currentValue}},
			{sortField: currentValue, "_id": bson.M{operator: currentID}},
		},
	}
	sortAscending := isAsc
	if previous {
		sortAscending = !sortAscending
	}
	return query, BlogSortFields(sortField, sortAscending)
}

// -------
// p
// 平台 lea+
// 博客列表
func (this *BlogService) ListAllBlogs(userId, tag string, keywords string, isRecommend bool, page, pageSize int, sorterField string, isAsc bool) (info.Page, []info.BlogItem) {
	pageInfo := info.Page{CurPage: page}
	notes := []info.Note{}
	sorterField = NormalizeBlogSortField(sorterField)

	skipNum, sortFieldR := parsePageAndSort(page, pageSize, sorterField, isAsc)

	// 不是trash的
	query := bson.M{"IsTrash": false, "IsDeleted": false, "IsBlog": true, "Title": bson.M{"$ne": "欢迎来到leanote!"}}
	if tag != "" {
		query["Tags"] = bson.M{"$in": []string{tag}}
	}
	if userId != "" {
		query["UserId"] = db.MustObjectIDFromHex(userId)
	}
	// 平台列表必须先通过统一 demo 身份配置校验；配置异常时不冒险
	// 暴露本应排除的 demo 内容。
	if userId == "" {
		demo, err := configService.DemoAccount()
		if err != nil {
			return pageInfo, nil
		}
		query["UserId"] = bson.M{"$ne": demo.UserID}
	}

	if isRecommend {
		query["IsRecommend"] = isRecommend
	}
	if normalizedKeywords, err := NormalizeBlogText(keywords, MaxBlogKeywordsRunes, MaxBlogKeywordsBytes); err == nil && normalizedKeywords != "" {
		query["Title"] = bson.M{"$regex": bson.Regex{Pattern: ".*?" + regexp.QuoteMeta(normalizedKeywords) + ".*", Options: "i"}}
	}
	q := db.Notes.Find(query)

	// 总记录数
	count, _ := q.Count()

	q.Sort(append([]string{sortFieldR}, BlogSortFields(sorterField, isAsc)[1])...).
		Skip(skipNum).
		Limit(pageSize).
		All(&notes)

	if notes == nil || len(notes) == 0 {
		return pageInfo, nil
	}

	// 得到content, 并且每个都要substring
	noteIds := make([]ObjectID, len(notes))
	userIds := make([]ObjectID, len(notes))
	for i, note := range notes {
		noteIds[i] = note.NoteId
		userIds[i] = note.UserId
	}

	// 可以不要的
	// 直接得到noteContents表的abstract
	// 这里可能是乱序的
	/*
		noteContents := noteService.ListNoteAbstractsByNoteIds(noteIds) // 返回[info.NoteContent]
		noteContentsMap := make(map[ObjectID]info.NoteContent, len(noteContents))
		for _, noteContent := range noteContents {
			noteContentsMap[noteContent.NoteId] = noteContent
		}
	*/

	// 得到用户信息
	userMap := userService.MapUserInfoAndBlogInfosByUserIds(userIds)

	// 组装成blogItem
	// 按照notes的顺序
	blogs := make([]info.BlogItem, len(noteIds))
	for i, note := range notes {
		hasMore := true
		var content string
		/*
			if noteContent, ok := noteContentsMap[note.NoteId]; ok {
				content = noteContent.Abstract
			}
		*/
		if len(note.Tags) == 1 && note.Tags[0] == "" {
			note.Tags = nil
		}
		blogs[i] = info.BlogItem{Note: note, Abstract: "", Content: content, HasMore: hasMore, User: userMap[note.UserId]}
	}
	pageInfo = info.NewPage(page, pageSize, count, nil)

	return pageInfo, blogs
}

// ------------------------
// 博客设置
func (this *BlogService) fixUserBlog(userBlog *info.UserBlog) {
	// Logo路径问题, 有些有http: 有些没有
	if userBlog.Logo != "" && !strings.HasPrefix(userBlog.Logo, "http") {
		userBlog.Logo = strings.Trim(userBlog.Logo, "/")
		userBlog.Logo = "/" + userBlog.Logo
	}

	if NormalizeBlogSortField(userBlog.SortField) != userBlog.SortField {
		if userBlog.SortField != "" {
			Logf("invalid blog SortField %q; using PublicTime", userBlog.SortField)
		}
		userBlog.SortField = "PublicTime"
	}
	if userBlog.PerPageSize < 1 || userBlog.PerPageSize > MaxBlogPageSize {
		if userBlog.PerPageSize != 0 {
			Logf("invalid blog PerPageSize %d; using %d", userBlog.PerPageSize, DefaultBlogPageSize)
		}
		userBlog.PerPageSize = DefaultBlogPageSize
	}

	// themePath
	if userBlog.Style == "" {
		userBlog.Style = defaultStyle
	}
	if userBlog.ThemeId.IsZero() {
		userBlog.ThemePath = themeService.GetDefaultThemePath(userBlog.Style)
	} else {
		userBlog.ThemePath = themeService.GetThemePath(userBlog.UserId.Hex(), userBlog.ThemeId.Hex())
	}
}
func (this *BlogService) GetUserBlog(userId string) info.UserBlog {
	userBlog, _ := this.GetUserBlogChecked(userId)
	return userBlog
}

func (this *BlogService) GetUserBlogChecked(userId string) (info.UserBlog, error) {
	if !db.IsValidObjectIDHex(userId) || db.UserBlogs == nil {
		return info.UserBlog{}, db.ErrMongoClientNotInitialized
	}
	userBlog := info.UserBlog{}
	if err := db.UserBlogs.FindId(db.MustObjectIDFromHex(userId)).One(&userBlog); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return info.UserBlog{}, nil
		}
		return info.UserBlog{}, fmt.Errorf("load user blog: %w", err)
	}
	this.fixUserBlog(&userBlog)
	return userBlog, nil
}

// 修改之
func (this *BlogService) UpdateUserBlog(userBlog info.UserBlog) bool {
	if userBlog.UserId.IsZero() {
		return false
	}
	if userBlog.Domain != "" {
		domain, err := CanonicalizeStoredCustomDomain(userBlog.Domain)
		if err != nil {
			return false
		}
		userBlog.Domain = domain
	}
	return db.Upsert(db.UserBlogs, bson.M{"_id": userBlog.UserId}, userBlog)
}

// 修改之UserBlogBase
func (this *BlogService) UpdateUserBlogBase(userId string, userBlog info.UserBlogBase) bool {
	ok := db.UpdateByQMap(db.UserBlogs, bson.M{"_id": db.MustObjectIDFromHex(userId)}, userBlog)
	return ok
}
func (this *BlogService) UpdateUserBlogComment(userId string, userBlog info.UserBlogComment) bool {
	return db.UpdateByQMap(db.UserBlogs, bson.M{"_id": db.MustObjectIDFromHex(userId)}, userBlog)
}
func (this *BlogService) UpdateUserBlogStyle(userId string, userBlog info.UserBlogStyle) bool {
	return db.UpdateByQMap(db.UserBlogs, bson.M{"_id": db.MustObjectIDFromHex(userId)}, userBlog)
}

// 分页与排序
func (this *BlogService) UpdateUserBlogPaging(userId string, perPageSize int, sortField string, isAsc bool) (ok bool, msg string) {
	if perPageSize < 1 || perPageSize > MaxBlogPageSize {
		return false, "invalidPageSize"
	}
	if err := ValidateBlogSortField(sortField); err != nil {
		return false, "invalidSortField"
	}
	if ok, msg = Vds(map[string]string{"perPageSize": strconv.Itoa(perPageSize), "sortField": sortField}); !ok {
		return
	}
	ok = db.UpdateByQMap(db.UserBlogs, bson.M{"_id": db.MustObjectIDFromHex(userId)},
		bson.M{"PerPageSize": perPageSize, "SortField": sortField, "IsAsc": isAsc})
	return
}

func (this *BlogService) GetUserBlogBySubDomain(subDomain string) info.UserBlog {
	blogUser, _ := this.LookupUserBlogBySubDomain(subDomain)
	return blogUser
}
func (this *BlogService) GetUserBlogByDomain(domain string) info.UserBlog {
	blogUser, _ := this.LookupUserBlogByDomain(domain)
	return blogUser
}

func (this *BlogService) LookupUserBlogBySubDomain(subDomain string) (info.UserBlog, error) {
	return this.lookupUserBlog(bson.M{"SubDomain": subDomain})
}

func (this *BlogService) LookupUserBlogByDomain(domain string) (info.UserBlog, error) {
	canonical, err := CanonicalizeStoredCustomDomain(domain)
	if err != nil {
		return info.UserBlog{}, err
	}
	return this.lookupUserBlog(bson.M{"Domain": canonical})
}

func (this *BlogService) lookupUserBlog(query bson.M) (info.UserBlog, error) {
	if db.UserBlogs == nil {
		return info.UserBlog{}, db.ErrMongoClientNotInitialized
	}
	var matches []info.UserBlog
	if err := db.UserBlogs.Find(query).Limit(2).All(&matches); err != nil {
		return info.UserBlog{}, fmt.Errorf("lookup user blog: %w", err)
	}
	if len(matches) == 0 {
		return info.UserBlog{}, nil
	}
	if len(matches) > 1 {
		return info.UserBlog{}, fmt.Errorf("ambiguous user blog mapping")
	}
	blogUser := matches[0]
	this.fixUserBlog(&blogUser)
	return blogUser, nil
}

//---------------------
// 后台管理

// 推荐博客
func (this *BlogService) SetRecommend(noteId string, isRecommend bool) bool {
	data := bson.M{"IsRecommend": isRecommend}
	if isRecommend {
		data["RecommendTime"] = time.Now()
	}
	return db.UpdateByQMap(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId), "IsBlog": true}, data)
}

//----------------------
// 博客社交, 评论

// 返回所有liked用户, bool是否还有
func (this *BlogService) ListLikedUsers(noteId string, isAll bool) ([]info.UserAndBlog, bool) {
	users, hasMore, _ := this.ListLikedUsersChecked(noteId, isAll)
	return users, hasMore
}

func (this *BlogService) ListLikedUsersChecked(noteId string, isAll bool) ([]info.UserAndBlog, bool, error) {
	if _, err := publicBlogNoteChecked(noteId); err != nil {
		return nil, false, err
	}
	if db.BlogLikes == nil {
		return nil, false, db.ErrMongoClientNotInitialized
	}
	// 默认前5
	pageSize := 5
	skipNum, sortFieldR := parsePageAndSort(1, pageSize, "CreatedTime", false)

	likes := []info.BlogLike{}
	query := bson.M{"NoteId": db.MustObjectIDFromHex(noteId)}
	q := db.BlogLikes.Find(query)

	// 总记录数
	count, err := q.Count()
	if err != nil {
		return nil, false, fmt.Errorf("count blog likes: %w", err)
	}
	if count == 0 {
		return []info.UserAndBlog{}, false, nil
	}

	if isAll {
		if err := q.Sort(sortFieldR).Skip(skipNum).Limit(pageSize).All(&likes); err != nil {
			return nil, false, fmt.Errorf("list blog likes: %w", err)
		}
	} else {
		if err := q.Sort(sortFieldR).All(&likes); err != nil {
			return nil, false, fmt.Errorf("list blog likes: %w", err)
		}
	}

	// 得到所有userIds
	userIds := make([]ObjectID, len(likes))
	for i, like := range likes {
		userIds[i] = like.UserId
	}
	// 得到用户信息
	userMap, err := userService.MapUserAndBlogByUserIdsChecked(userIds)
	if err != nil {
		return nil, false, err
	}

	users := make([]info.UserAndBlog, len(likes))
	for i, like := range likes {
		users[i] = userMap[like.UserId.Hex()]
	}

	return users, count > pageSize, nil
}

func (this *BlogService) IsILikeIt(noteId, userId string) bool {
	liked, _ := this.IsILikeItChecked(noteId, userId)
	return liked
}

func (this *BlogService) IsILikeItChecked(noteId, userId string) (bool, error) {
	if !db.IsValidObjectIDHex(userId) || db.BlogLikes == nil {
		return false, db.ErrMongoClientNotInitialized
	}
	if _, err := publicBlogNoteChecked(noteId); err != nil {
		return false, err
	}
	count, err := db.BlogLikes.Find(bson.M{"NoteId": db.MustObjectIDFromHex(noteId), "UserId": db.MustObjectIDFromHex(userId)}).Count()
	if err != nil {
		return false, fmt.Errorf("check blog like: %w", err)
	}
	return count > 0, nil
}

// 阅读次数统计+1
func (this *BlogService) IncReadNum(noteId string) bool {
	if _, ok := publicBlogNote(noteId); !ok {
		return false
	}
	return db.Update(db.Notes, bson.M{"_id": db.MustObjectIDFromHex(noteId), "IsBlog": true, "IsTrash": false, "IsDeleted": false}, bson.M{"$inc": bson.M{"ReadNum": 1}})
}

// 点赞
// retun ok , isLike
func (this *BlogService) LikeBlog(noteId, userId string) (ok bool, isLike bool) {
	ok = false
	isLike = false
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) || db.BlogLikes == nil || db.Notes == nil {
		return
	}
	// 判断是否点过赞, 如果点过那么取消点赞
	if _, public := publicBlogNote(noteId); !public {
		return
	}

	noteIdO := db.MustObjectIDFromHex(noteId)
	userIdO := db.MustObjectIDFromHex(userId)
	likeQuery := bson.M{"NoteId": noteIdO, "UserId": userIdO}
	likeCount, err := db.BlogLikes.Find(likeQuery).Count()
	if err != nil {
		return
	}
	if likeCount == 0 {
		// 添加之
		if !db.Insert(db.BlogLikes, info.BlogLike{LikeId: db.NewObjectID(), NoteId: noteIdO, UserId: userIdO, CreatedTime: time.Now()}) {
			return
		}
		isLike = true
	} else {
		// 已点过, 那么删除之
		if !db.Delete(db.BlogLikes, likeQuery) {
			return
		}
		isLike = false
	}

	count, err := db.BlogLikes.Find(bson.M{"NoteId": noteIdO}).Count()
	if err != nil {
		return false, false
	}
	ok = db.UpdateByQI(db.Notes, bson.M{"_id": noteIdO, "IsBlog": true, "IsTrash": false, "IsDeleted": false}, bson.M{"LikeNum": count})
	if !ok {
		return false, false
	}

	return
}

// 评论
// 在noteId博客下userId 给toUserId评论content
// commentId可为空(针对某条评论评论)
func validCommentSubmissionId(submissionId string) bool {
	if len(submissionId) != 32 {
		return false
	}
	for _, character := range []byte(submissionId) {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func commentContentDigest(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func commentNotificationKey(commentID, recipientID ObjectID) string {
	return "comment:" + commentID.Hex() + ":" + recipientID.Hex()
}

func commentNotificationRecipientIDs(ctx context.Context, ownerID, replyUserID, actorID ObjectID) ([]ObjectID, error) {
	recipientID := ownerID
	if !replyUserID.IsZero() {
		recipientID = replyUserID
	}
	if recipientID.IsZero() || recipientID == actorID {
		return nil, nil
	}
	if db.Users == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	var recipient info.User
	if err := db.Users.FindIdContext(ctx, recipientID).One(&recipient); err != nil {
		return nil, fmt.Errorf("load comment notification recipient: %w", err)
	}
	if strings.TrimSpace(recipient.Email) == "" {
		return nil, nil
	}
	return []ObjectID{recipientID}, nil
}

type blogCommentMutationBeforeState struct {
	Note info.Note
}

type blogCommentMutationDesiredState struct {
	Comment                  info.BlogComment
	Receipt                  info.BlogCommentSubmissionReceipt
	CountMutationID          string
	NotificationRecipientIDs []ObjectID
}

type blogCommentMutationResultState struct {
	CommentId ObjectID
}

type blogCommentDeleteDesiredState struct {
	Comment         info.BlogComment
	Receipt         info.BlogCommentSubmissionReceipt
	HasReceipt      bool
	CountMutationID string
}

type blogCommentDeleteResultState struct {
	CommentId ObjectID
}

func blogCommentReceiptMatches(got, want info.BlogCommentSubmissionReceipt) bool {
	return got.ActorId == want.ActorId &&
		got.SubmissionId == want.SubmissionId &&
		got.NoteId == want.NoteId &&
		got.ToCommentId == want.ToCommentId &&
		got.ToUserId == want.ToUserId &&
		got.ContentSHA256 == want.ContentSHA256 &&
		got.CommentId == want.CommentId
}

// A replay request is identified before its comment ID is known. Pending
// receipts already have the server-assigned comment ID, so request matching
// must compare the immutable submission intent without requiring that ID.
func blogCommentReceiptRequestMatches(got, want info.BlogCommentSubmissionReceipt) bool {
	return got.ActorId == want.ActorId &&
		got.SubmissionId == want.SubmissionId &&
		got.NoteId == want.NoteId &&
		got.ToCommentId == want.ToCommentId &&
		got.ContentSHA256 == want.ContentSHA256
}

func blogCommentsMatch(got, want info.BlogComment) bool {
	return got.CommentId == want.CommentId &&
		got.NoteId == want.NoteId &&
		got.UserId == want.UserId &&
		got.Content == want.Content &&
		got.ToCommentId == want.ToCommentId &&
		got.ToUserId == want.ToUserId
}

func blogCommentFromPendingReceipt(receipt info.BlogCommentSubmissionReceipt, content string) info.BlogComment {
	return info.BlogComment{
		CommentId:   receipt.CommentId,
		NoteId:      receipt.NoteId,
		UserId:      receipt.ActorId,
		Content:     content,
		ToCommentId: receipt.ToCommentId,
		ToUserId:    receipt.ToUserId,
		CreatedTime: receipt.CreatedTime,
	}
}

func blogCommentBeforeStateMatches(note info.Note, noteID, ownerID ObjectID) bool {
	return note.NoteId == noteID && note.UserId == ownerID
}

func loadBlogCommentReceipt(ctx context.Context, actorID ObjectID, submissionID string) (info.BlogCommentSubmissionReceipt, error) {
	var receipt info.BlogCommentSubmissionReceipt
	err := db.BlogCommentReceipts.FindContext(ctx, bson.M{"ActorId": actorID, "SubmissionId": submissionID}).One(&receipt)
	return receipt, err
}

func ensureBlogCommentReceipt(ctx context.Context, want info.BlogCommentSubmissionReceipt) error {
	existing, err := loadBlogCommentReceipt(ctx, want.ActorId, want.SubmissionId)
	if err == nil {
		if !blogCommentReceiptMatches(existing, want) || existing.Status == info.BlogCommentReceiptDeleted {
			return fmt.Errorf("comment submission receipt conflict")
		}
		return nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if err := db.BlogCommentReceipts.InsertContext(ctx, want); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		existing, lookupErr := loadBlogCommentReceipt(ctx, want.ActorId, want.SubmissionId)
		if lookupErr != nil {
			return lookupErr
		}
		if !blogCommentReceiptMatches(existing, want) || existing.Status == info.BlogCommentReceiptDeleted {
			return fmt.Errorf("comment submission receipt conflict")
		}
	}
	return nil
}

func verifyBlogCommentReceipt(ctx context.Context, want info.BlogCommentSubmissionReceipt) (bool, error) {
	existing, err := loadBlogCommentReceipt(ctx, want.ActorId, want.SubmissionId)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return blogCommentReceiptMatches(existing, want) && existing.Status != info.BlogCommentReceiptDeleted, nil
}

func ensureBlogComment(ctx context.Context, want info.BlogComment) error {
	var existing info.BlogComment
	err := db.BlogComments.FindIdContext(ctx, want.CommentId).One(&existing)
	if err == nil {
		if !blogCommentsMatch(existing, want) {
			return fmt.Errorf("comment identity conflict")
		}
		return nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if err := db.BlogComments.InsertContext(ctx, want); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		if err := db.BlogComments.FindIdContext(ctx, want.CommentId).One(&existing); err != nil {
			return err
		}
		if !blogCommentsMatch(existing, want) {
			return fmt.Errorf("comment identity conflict")
		}
	}
	return nil
}

func verifyBlogComment(ctx context.Context, want info.BlogComment) (bool, error) {
	var existing info.BlogComment
	err := db.BlogComments.FindIdContext(ctx, want.CommentId).One(&existing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return blogCommentsMatch(existing, want), nil
}

func applyCommentCountMutation(ctx context.Context, noteID ObjectID, marker string, delta int) error {
	filter := bson.M{
		"_id": noteID, "IsBlog": true, "IsTrash": false, "IsDeleted": false,
		"CommentCountMutationIds": bson.M{"$ne": marker},
	}
	if delta < 0 {
		filter["CommentNum"] = bson.M{"$gt": 0}
	}
	err := db.Notes.UpdateOneMatchedContext(ctx, filter, bson.M{
		"$inc":      bson.M{"CommentNum": delta},
		"$addToSet": bson.M{"CommentCountMutationIds": marker},
	})
	if errors.Is(err, db.ErrDocumentNotFound) {
		applied, verifyErr := verifyCommentCountMutation(ctx, noteID, marker)
		if verifyErr == nil && applied {
			return nil
		}
		if verifyErr != nil {
			return verifyErr
		}
	}
	return err
}

func verifyCommentCountMutation(ctx context.Context, noteID ObjectID, marker string) (bool, error) {
	var note info.Note
	err := db.Notes.FindIdContext(ctx, noteID).One(&note)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, mutationID := range note.CommentCountMutationIDs {
		if mutationID == marker {
			return true, nil
		}
	}
	return false, nil
}

func confirmBlogCommentReceipt(ctx context.Context, receiptID, commentID ObjectID) error {
	err := db.BlogCommentReceipts.UpdateOneMatchedContext(ctx,
		bson.M{"_id": receiptID, "CommentId": commentID, "Status": info.BlogCommentReceiptPending},
		bson.M{"$set": bson.M{"Status": info.BlogCommentReceiptConfirmed, "UpdatedTime": time.Now()}},
	)
	if errors.Is(err, db.ErrDocumentNotFound) {
		var receipt info.BlogCommentSubmissionReceipt
		lookupErr := db.BlogCommentReceipts.FindIdContext(ctx, receiptID).One(&receipt)
		if lookupErr == nil && receipt.Status == info.BlogCommentReceiptConfirmed && receipt.CommentId == commentID {
			return nil
		}
		if lookupErr != nil {
			return lookupErr
		}
	}
	return err
}

func verifyConfirmedBlogCommentReceipt(ctx context.Context, receiptID, commentID ObjectID) (bool, error) {
	var receipt info.BlogCommentSubmissionReceipt
	err := db.BlogCommentReceipts.FindIdContext(ctx, receiptID).One(&receipt)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return receipt.Status == info.BlogCommentReceiptConfirmed && receipt.CommentId == commentID, nil
}

func deleteBlogCommentReceipt(ctx context.Context, receiptID, commentID ObjectID) error {
	err := db.BlogCommentReceipts.UpdateOneMatchedContext(ctx,
		bson.M{"_id": receiptID, "CommentId": commentID, "Status": bson.M{"$ne": info.BlogCommentReceiptDeleted}},
		bson.M{"$set": bson.M{"Status": info.BlogCommentReceiptDeleted, "UpdatedTime": time.Now()}},
	)
	if errors.Is(err, db.ErrDocumentNotFound) {
		var receipt info.BlogCommentSubmissionReceipt
		lookupErr := db.BlogCommentReceipts.FindIdContext(ctx, receiptID).One(&receipt)
		if lookupErr == nil && receipt.Status == info.BlogCommentReceiptDeleted && receipt.CommentId == commentID {
			return nil
		}
		if lookupErr != nil {
			return lookupErr
		}
	}
	return err
}

func verifyDeletedBlogCommentReceipt(ctx context.Context, receiptID, commentID ObjectID) (bool, error) {
	var receipt info.BlogCommentSubmissionReceipt
	err := db.BlogCommentReceipts.FindIdContext(ctx, receiptID).One(&receipt)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return receipt.Status == info.BlogCommentReceiptDeleted && receipt.CommentId == commentID, nil
}

func (this *BlogService) Comment(noteId, toCommentId, userId, content, submissionId string) (bool, info.BlogComment) {
	var comment info.BlogComment
	if !validCommentContent(content) || !validCommentSubmissionId(submissionId) || !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) || (toCommentId != "" && !db.IsValidObjectIDHex(toCommentId)) || db.BlogComments == nil || db.BlogCommentReceipts == nil || db.Notes == nil || db.UserBlogs == nil {
		return false, comment
	}
	actorID := db.MustObjectIDFromHex(userId)
	noteID := db.MustObjectIDFromHex(noteId)
	operationID, err := applicationnotes.NewClientOperationIdentity("blog_comment", actorID, submissionId)
	if err != nil {
		return false, comment
	}
	digest := commentContentDigest(content)
	identityInput := struct {
		SubmissionID  string
		NoteID        string
		ToCommentID   string
		ContentSHA256 string
	}{submissionId, noteId, toCommentId, digest}
	_, inputDigest, _, err := applicationnotes.NewOperationIdentity("blog_comment", actorID, noteID, identityInput)
	if err != nil {
		return false, comment
	}
	receipt, receiptErr := loadBlogCommentReceipt(context.Background(), actorID, submissionId)
	if receiptErr != nil && !errors.Is(receiptErr, mongo.ErrNoDocuments) {
		return false, comment
	}
	if receiptErr == nil {
		want := info.BlogCommentSubmissionReceipt{ActorId: actorID, SubmissionId: submissionId, NoteId: noteID, ContentSHA256: digest}
		if toCommentId != "" {
			want.ToCommentId = db.MustObjectIDFromHex(toCommentId)
		}
		if !blogCommentReceiptRequestMatches(receipt, want) || receipt.Status == info.BlogCommentReceiptDeleted {
			return false, info.BlogComment{}
		}
		if receipt.Status == info.BlogCommentReceiptConfirmed {
			operation, operationErr := db.GetWorkspaceOperation(context.Background(), actorID, operationID)
			if operationErr != nil {
				return false, info.BlogComment{}
			}
			if operation.Status == applicationnotes.OperationCommitted {
				expected := info.BlogComment{
					CommentId:   receipt.CommentId,
					NoteId:      noteID,
					UserId:      actorID,
					Content:     content,
					ToCommentId: receipt.ToCommentId,
					ToUserId:    receipt.ToUserId,
				}
				if db.BlogComments.FindIdContext(context.Background(), receipt.CommentId).One(&comment) != nil || !blogCommentsMatch(comment, expected) {
					return false, info.BlogComment{}
				}
				return true, comment
			}
		}
		if receipt.Status != info.BlogCommentReceiptPending && receipt.Status != info.BlogCommentReceiptConfirmed {
			return false, info.BlogComment{}
		}
	}

	note, err := publicBlogNoteChecked(noteId)
	if err != nil {
		return false, comment
	}
	var userBlog info.UserBlog
	if err := db.UserBlogs.FindIdContext(context.Background(), note.UserId).One(&userBlog); err != nil || !userBlog.CanComment || (userBlog.CommentType != "" && userBlog.CommentType != "default") {
		return false, comment
	}
	var comment2 = info.BlogComment{}
	if toCommentId != "" {
		if db.BlogComments.FindIdContext(context.Background(), db.MustObjectIDFromHex(toCommentId)).One(&comment2) != nil || comment2.CommentId.IsZero() || comment2.NoteId != noteID {
			return false, info.BlogComment{}
		}
	}
	if receiptErr == nil {
		// Rebuild the full deterministic comment from the pending receipt and
		// current request. A receipt may survive after the comment step failed;
		// carrying only CommentId/CreatedTime would otherwise write zero fields
		// on the retry while still allowing count and notification confirmation.
		comment = blogCommentFromPendingReceipt(receipt, content)
	} else {
		comment = info.BlogComment{CommentId: db.NewObjectID(), NoteId: noteID, UserId: actorID, Content: content, CreatedTime: time.Now()}
		if toCommentId != "" {
			comment.ToCommentId = comment2.CommentId
			comment.ToUserId = comment2.UserId
		}
		receipt = info.BlogCommentSubmissionReceipt{
			ReceiptId: db.NewObjectID(), ActorId: actorID, SubmissionId: submissionId, NoteId: noteID,
			ContentSHA256: digest, CommentId: comment.CommentId, Status: info.BlogCommentReceiptPending,
			CreatedTime: comment.CreatedTime, UpdatedTime: comment.CreatedTime,
		}
		receipt.ToCommentId = comment.ToCommentId
		receipt.ToUserId = comment.ToUserId
	}
	if comment.CreatedTime.IsZero() {
		comment.CreatedTime = receipt.CreatedTime
	}
	articleOwnerID := note.UserId
	notificationRecipientIDs, err := commentNotificationRecipientIDs(context.Background(), note.UserId, comment.ToUserId, actorID)
	if err != nil {
		return false, info.BlogComment{}
	}
	desired := blogCommentMutationDesiredState{
		Comment:                  comment,
		Receipt:                  receipt,
		CountMutationID:          "add:" + operationID,
		NotificationRecipientIDs: notificationRecipientIDs,
	}
	beforePayload, err := json.Marshal(blogCommentMutationBeforeState{Note: note})
	if err != nil {
		return false, info.BlogComment{}
	}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false, info.BlogComment{}
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: actorID, ResourceID: noteID, Kind: "blog_comment", InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailurePending,
		RestoreBeforeState: func(payload []byte) error {
			var frozen blogCommentMutationBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || !blogCommentBeforeStateMatches(frozen.Note, noteID, articleOwnerID) {
				return fmt.Errorf("comment before state target changed")
			}
			note = frozen.Note
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen blogCommentMutationDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Comment.NoteId != noteID || !blogCommentReceiptMatches(frozen.Receipt, receipt) {
				return fmt.Errorf("comment desired state target changed")
			}
			desired = frozen
			comment = frozen.Comment
			return nil
		},
		CaptureResultState: func() []byte {
			payload, err := json.Marshal(blogCommentMutationResultState{CommentId: desired.Comment.CommentId})
			if err != nil {
				return nil
			}
			return payload
		},
		RestoreResultState: func(payload []byte) error {
			var frozen blogCommentMutationResultState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.CommentId.IsZero() {
				return fmt.Errorf("comment result state changed")
			}
			desired.Comment.CommentId = frozen.CommentId
			comment.CommentId = frozen.CommentId
			return nil
		},
	}
	plan.Steps = []db.WorkspaceMutationStep{
		{Name: "submission_receipt", ReplaySafe: true, Apply: func(ctx context.Context) error { return ensureBlogCommentReceipt(ctx, desired.Receipt) }, Verify: func(ctx context.Context) (bool, error) { return verifyBlogCommentReceipt(ctx, desired.Receipt) }},
		{Name: "comment", ReplaySafe: true, Apply: func(ctx context.Context) error { return ensureBlogComment(ctx, desired.Comment) }, Verify: func(ctx context.Context) (bool, error) { return verifyBlogComment(ctx, desired.Comment) }},
		{Name: "comment_count", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return applyCommentCountMutation(ctx, noteID, desired.CountMutationID, 1)
		}, Verify: func(ctx context.Context) (bool, error) {
			return verifyCommentCountMutation(ctx, noteID, desired.CountMutationID)
		}},
		{Name: "comment_notification", ReplaySafe: true, Apply: func(ctx context.Context) error {
			if len(desired.NotificationRecipientIDs) == 0 {
				return nil
			}
			if db.Users == nil {
				return db.ErrMongoClientNotInitialized
			}
			for _, recipientID := range desired.NotificationRecipientIDs {
				var recipient info.User
				if err := db.Users.FindIdContext(ctx, recipientID).One(&recipient); err != nil {
					return fmt.Errorf("load comment notification recipient: %w", err)
				}
				if strings.TrimSpace(recipient.Email) == "" {
					return fmt.Errorf("comment notification recipient email is no longer available")
				}
				if _, err := db.EnqueueOutboxEvent(ctx, db.OutboxEvent{
					IdempotencyKey: commentNotificationKey(desired.Comment.CommentId, recipientID),
					Kind:           "comment", AggregateID: desired.Comment.CommentId, Status: db.OutboxStatusUnconfirmed,
					CommentID: desired.Comment.CommentId, RecipientID: recipientID, EventVersion: 1,
					Payload: map[string]any{"email": recipient.Email, "content": desired.Comment.Content, "noteId": noteID.Hex(), "recipientId": recipientID.Hex()},
				}); err != nil {
					return err
				}
			}
			return nil
		}, Verify: func(ctx context.Context) (bool, error) {
			if len(desired.NotificationRecipientIDs) == 0 {
				return true, nil
			}
			if db.Outbox == nil {
				return false, db.ErrMongoClientNotInitialized
			}
			for _, recipientID := range desired.NotificationRecipientIDs {
				exists, err := db.OutboxEventExists(ctx, commentNotificationKey(desired.Comment.CommentId, recipientID))
				if err != nil || !exists {
					return false, err
				}
			}
			return true, nil
		}},
		{Name: "confirm_receipt", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return confirmBlogCommentReceipt(ctx, desired.Receipt.ReceiptId, desired.Comment.CommentId)
		}, Verify: func(ctx context.Context) (bool, error) {
			return verifyConfirmedBlogCommentReceipt(ctx, desired.Receipt.ReceiptId, desired.Comment.CommentId)
		}},
		{Name: "confirm_comment_notifications", ReplaySafe: true, Apply: func(ctx context.Context) error {
			for _, recipientID := range desired.NotificationRecipientIDs {
				if err := db.ConfirmCommentOutbox(ctx, db.OutboxEventIDForKey(commentNotificationKey(desired.Comment.CommentId, recipientID)), time.Now().UTC()); err != nil {
					return err
				}
			}
			return nil
		}, Verify: func(ctx context.Context) (bool, error) {
			for _, recipientID := range desired.NotificationRecipientIDs {
				event, err := db.GetOutboxEvent(ctx, db.OutboxEventIDForKey(commentNotificationKey(desired.Comment.CommentId, recipientID)))
				if err != nil {
					return false, err
				}
				if !event.IsConfirmedCommentNotification() {
					return false, nil
				}
			}
			return true, nil
		}},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !mutation.Committed {
		return false, info.BlogComment{}
	}
	var committed info.BlogComment
	if err := db.BlogComments.FindIdContext(context.Background(), desired.Comment.CommentId).One(&committed); err != nil || !blogCommentsMatch(committed, desired.Comment) {
		return false, info.BlogComment{}
	}
	return true, committed
}

// 作者(或管理员)可以删除所有评论
// 自己可以删除评论
func (this *BlogService) DeleteComment(noteId, commentId, userId string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(commentId) || !db.IsValidObjectIDHex(userId) || db.BlogComments == nil || db.BlogCommentReceipts == nil || db.Notes == nil {
		return false
	}
	note, err := publicBlogNoteChecked(noteId)
	if err != nil {
		return false
	}
	actorID := db.MustObjectIDFromHex(userId)
	noteID := db.MustObjectIDFromHex(noteId)
	commentID := db.MustObjectIDFromHex(commentId)
	input := struct{ NoteID, CommentID string }{noteId, commentId}
	operationID, inputDigest, _, err := applicationnotes.NewResourceOperationIdentity("blog_comment_delete", actorID, commentID, input)
	if err != nil {
		return false
	}

	comment := info.BlogComment{}
	receipt := info.BlogCommentSubmissionReceipt{}
	hasReceipt := false
	workspaceReceipt, workspaceErr := db.GetWorkspaceOperation(context.Background(), actorID, operationID)
	if workspaceErr != nil && !errors.Is(workspaceErr, mongo.ErrNoDocuments) {
		return false
	}
	if workspaceErr == nil && workspaceReceipt.Status != applicationnotes.OperationCommitted {
		var frozen blogCommentDeleteDesiredState
		if len(workspaceReceipt.DesiredState) == 0 || json.Unmarshal(workspaceReceipt.DesiredState, &frozen) != nil || frozen.Comment.CommentId != commentID {
			return false
		}
		comment = frozen.Comment
		receipt = frozen.Receipt
		hasReceipt = frozen.HasReceipt
	} else {
		receiptErr := db.BlogCommentReceipts.FindContext(context.Background(), bson.M{"CommentId": commentID}).One(&receipt)
		hasReceipt = receiptErr == nil
		if receiptErr != nil && !errors.Is(receiptErr, mongo.ErrNoDocuments) {
			return false
		}
		if err := db.BlogComments.FindIdContext(context.Background(), commentID).One(&comment); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) && workspaceErr == nil && workspaceReceipt.Status == applicationnotes.OperationCommitted && hasReceipt && receipt.Status == info.BlogCommentReceiptDeleted && (actorID == note.UserId || userId == configService.GetAdminUserId() || receipt.ActorId == actorID) {
				return true
			}
			return false
		}
	}
	if comment.CommentId.IsZero() || comment.NoteId != note.NoteId {
		return false
	}
	if actorID != note.UserId && userId != configService.GetAdminUserId() && comment.UserId != actorID {
		return false
	}
	if workspaceErr == nil && workspaceReceipt.Status == applicationnotes.OperationCommitted {
		return false
	}
	if !hasReceipt {
		receipt = info.BlogCommentSubmissionReceipt{}
	}
	desired := blogCommentDeleteDesiredState{Comment: comment, Receipt: receipt, HasReceipt: hasReceipt, CountMutationID: "delete:" + operationID}
	beforePayload, err := json.Marshal(blogCommentMutationBeforeState{Note: note})
	if err != nil {
		return false
	}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: actorID, ResourceID: commentID, Kind: "blog_comment_delete", InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailurePending,
		RestoreBeforeState: func(payload []byte) error {
			var frozen blogCommentMutationBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Note.NoteId != noteID {
				return fmt.Errorf("comment delete before state target changed")
			}
			note = frozen.Note
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen blogCommentDeleteDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Comment.CommentId != commentID {
				return fmt.Errorf("comment delete desired state target changed")
			}
			desired = frozen
			comment = frozen.Comment
			return nil
		},
	}
	plan.Steps = []db.WorkspaceMutationStep{
		{Name: "cancel_comment_notification", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return db.CancelOutboxForAggregate(ctx, desired.Comment.CommentId, "comment deleted")
		}, Verify: func(ctx context.Context) (bool, error) {
			return db.VerifyOutboxCancelledForAggregate(ctx, desired.Comment.CommentId)
		}},
		{Name: "comment", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return db.BlogComments.RemoveContext(ctx, bson.M{"_id": desired.Comment.CommentId, "NoteId": noteID})
		}, Verify: func(ctx context.Context) (bool, error) {
			var current info.BlogComment
			err := db.BlogComments.FindContext(ctx, bson.M{"_id": desired.Comment.CommentId, "NoteId": noteID}).One(&current)
			if errors.Is(err, mongo.ErrNoDocuments) {
				return true, nil
			}
			return false, err
		}},
		{Name: "comment_count", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return applyCommentCountMutation(ctx, noteID, desired.CountMutationID, -1)
		}, Verify: func(ctx context.Context) (bool, error) {
			return verifyCommentCountMutation(ctx, noteID, desired.CountMutationID)
		}},
	}
	if desired.HasReceipt {
		plan.Steps = append(plan.Steps, db.WorkspaceMutationStep{Name: "delete_receipt", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return deleteBlogCommentReceipt(ctx, desired.Receipt.ReceiptId, desired.Comment.CommentId)
		}, Verify: func(ctx context.Context) (bool, error) {
			return verifyDeletedBlogCommentReceipt(ctx, desired.Receipt.ReceiptId, desired.Comment.CommentId)
		}})
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	return err == nil && mutation.Committed
}

// 点赞/取消赞
func (this *BlogService) LikeComment(commentId, userId string) (ok bool, isILike bool, num int) {
	if !db.IsValidObjectIDHex(commentId) || !db.IsValidObjectIDHex(userId) || db.BlogComments == nil {
		return false, false, 0
	}
	commentObjectID := db.MustObjectIDFromHex(commentId)
	for attempt := 0; attempt < 16; attempt++ {
		var comment info.BlogComment
		if err := db.BlogComments.FindIdContext(context.Background(), commentObjectID).One(&comment); err != nil || comment.CommentId.IsZero() {
			return false, false, 0
		}
		note, publicErr := publicBlogNoteChecked(comment.NoteId.Hex())
		if publicErr != nil || note.NoteId != comment.NoteId {
			return false, false, 0
		}

		updatedUserIDs := make([]string, 0, len(comment.LikeUserIds)+1)
		wasLiked := false
		for _, likedUserID := range comment.LikeUserIds {
			if likedUserID == userId {
				wasLiked = true
				continue
			}
			updatedUserIDs = append(updatedUserIDs, likedUserID)
		}
		if !wasLiked {
			updatedUserIDs = append(updatedUserIDs, userId)
		}
		num = len(updatedUserIDs)
		err := db.BlogComments.UpdateOneMatchedContext(context.Background(),
			bson.M{"_id": commentObjectID, "NoteId": note.NoteId, "LikeUserIds": comment.LikeUserIds},
			bson.M{"$set": bson.M{"LikeUserIds": updatedUserIDs, "LikeNum": num}},
		)
		if errors.Is(err, db.ErrDocumentNotFound) {
			continue
		}
		if err != nil {
			return false, false, 0
		}
		return true, !wasLiked, num
	}
	return false, false, 0
}

// 评论列表
// userId主要是显示userId是否点过某评论的赞
// 还要获取用户信息
func (this *BlogService) ListComments(userId, noteId string, page, pageSize int) (info.Page, []info.BlogCommentPublic, map[string]info.UserAndBlog) {
	pageInfo, comments, userMap, _ := this.ListCommentsChecked(userId, noteId, page, pageSize)
	return pageInfo, comments, userMap
}

func (this *BlogService) ListCommentsChecked(userId, noteId string, page, pageSize int) (info.Page, []info.BlogCommentPublic, map[string]info.UserAndBlog, error) {
	pageInfo := info.Page{CurPage: page}
	if err := validateBlogCommentPagination(page, pageSize); err != nil {
		return pageInfo, nil, nil, err
	}
	note, err := publicBlogNoteChecked(noteId)
	if err != nil {
		return pageInfo, nil, nil, err
	}
	if db.BlogComments == nil || db.BlogCommentReceipts == nil {
		return pageInfo, nil, nil, db.ErrMongoClientNotInitialized
	}

	comments2 := []info.BlogComment{}

	skipNum, sortFieldR := parsePageAndSort(page, pageSize, "CreatedTime", false)

	query := bson.M{"NoteId": db.MustObjectIDFromHex(noteId)}
	pendingReceipts := []info.BlogCommentSubmissionReceipt{}
	if err := db.BlogCommentReceipts.Find(bson.M{"NoteId": query["NoteId"], "Status": info.BlogCommentReceiptPending}).All(&pendingReceipts); err != nil {
		return pageInfo, nil, nil, fmt.Errorf("list pending comment receipts: %w", err)
	}
	if len(pendingReceipts) > 0 {
		pendingCommentIds := make([]interface{}, len(pendingReceipts))
		for i, receipt := range pendingReceipts {
			pendingCommentIds[i] = receipt.CommentId
		}
		query["_id"] = bson.M{"$nin": pendingCommentIds}
	}
	q := db.BlogComments.Find(query)

	// 总记录数
	count, err := q.Count()
	if err != nil {
		return pageInfo, nil, nil, fmt.Errorf("count comments: %w", err)
	}
	if err := q.Sort(sortFieldR).Skip(skipNum).Limit(pageSize).All(&comments2); err != nil {
		return pageInfo, nil, nil, fmt.Errorf("list comments: %w", err)
	}

	if len(comments2) == 0 {
		return info.NewPage(page, pageSize, count, nil), []info.BlogCommentPublic{}, map[string]info.UserAndBlog{}, nil
	}

	comments := make([]info.BlogCommentPublic, len(comments2))
	// 我是否点过赞呢?
	for i, comment := range comments2 {
		comments[i].BlogComment = comment
		if comment.LikeNum > 0 && comment.LikeUserIds != nil && len(comment.LikeUserIds) > 0 && InArray(comment.LikeUserIds, userId) {
			comments[i].IsILikeIt = true
		}
	}

	// 得到用户信息
	userIdsMap := map[ObjectID]bool{note.UserId: true}
	for _, comment := range comments {
		userIdsMap[comment.UserId] = true
		if !comment.ToUserId.IsZero() { // 可能为空
			userIdsMap[comment.ToUserId] = true
		}
	}
	userIds := make([]ObjectID, len(userIdsMap))
	i := 0
	for userId, _ := range userIdsMap {
		userIds[i] = userId
		i++
	}

	// 得到用户信息
	userMap, err := userService.MapUserAndBlogByUserIdsChecked(userIds)
	if err != nil {
		return pageInfo, nil, nil, err
	}
	pageInfo = info.NewPage(page, pageSize, count, nil)

	return pageInfo, comments, userMap, nil
}

func validateBlogCommentPagination(page, pageSize int) error {
	if page < 1 || page > MaxBlogPage || pageSize < 1 || pageSize > MaxBlogPageSize {
		return fmt.Errorf("%w: comment pagination", ErrInvalidBlogQuery)
	}
	return nil
}

// 举报
func (this *BlogService) Report(noteId, commentId, reason, userId string) bool {
	if !db.IsValidObjectIDHex(noteId) || !db.IsValidObjectIDHex(userId) || db.Reports == nil || db.BlogComments == nil {
		return false
	}
	note, ok := publicBlogNote(noteId)
	if !ok {
		return false
	}

	report := info.Report{ReportId: db.NewObjectID(),
		NoteId:      db.MustObjectIDFromHex(noteId),
		UserId:      db.MustObjectIDFromHex(userId),
		Reason:      reason,
		CreatedTime: time.Now(),
	}
	if commentId != "" {
		if !db.IsValidObjectIDHex(commentId) {
			return false
		}
		report.CommentId = db.MustObjectIDFromHex(commentId)
		comment := info.BlogComment{}
		if db.BlogComments.Find(bson.M{"_id": report.CommentId}).One(&comment) != nil || comment.NoteId != note.NoteId {
			return false
		}
	}
	return db.Insert(db.Reports, report)
}

//---------------
// 分类排序

// CateIds
func (this *BlogService) UpateCateIds(userId string, cateIds []string) bool {
	return db.UpdateByQField(db.UserBlogs, bson.M{"_id": db.MustObjectIDFromHex(userId)}, "CateIds", cateIds)
}

// 修改笔记本urlTitle
func (this *BlogService) UpateCateUrlTitle(userId string, cateId, urlTitle string) (ok bool, url string) {
	url = urlTitle
	/*
		// 先清空
		ok = db.UpdateByIdAndUserIdMap(db.Notebooks, cateId, userId, bson.M{
			"UrlTitle": "",
		})
	*/
	url = GetUrTitle(userId, urlTitle, "notebook", cateId)
	ok = db.UpdateByIdAndUserIdMap(db.Notebooks, cateId, userId, bson.M{
		"UrlTitle": url,
	})
	// 返回给前端的是decode
	url = decodeValue(url)
	return
}

// 修改笔记urlTitle
func (this *BlogService) UpateBlogUrlTitle(userId string, noteId, urlTitle string) (ok bool, url string) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(noteId) || db.Notes == nil {
		return false, urlTitle
	}
	url = urlTitle
	url = GetUrTitle(userId, urlTitle, "note", noteId)
	ok = db.UpdateByIdAndUserIdMap(db.Notes, noteId, userId, bson.M{
		"UrlTitle": url,
	})
	// 返回给前端的是decode
	url = decodeValue(url)
	return
}

type blogAbstractMutationBeforeState struct {
	Note    info.Note
	Content info.NoteContent
}

type blogAbstractMutationDesiredState struct {
	ImgSrc         string
	Desc           string
	HasSelfDefined bool
	Abstract       string
}

type blogSingleMutationBeforeState struct {
	Single    info.BlogSingle
	HasSingle bool
	UserBlog  info.UserBlog
}

type blogSingleMutationDesiredState struct {
	Single          info.BlogSingle
	HasSingle       bool
	UserBlogSingles []map[string]string
}

type blogUserBlogSinglesMutationState struct {
	UserBlog info.UserBlog
	Desired  []map[string]string
}

func loadBlogSingleMutationBefore(ctx context.Context, ownerID, singleID ObjectID) (blogSingleMutationBeforeState, error) {
	if db.UserBlogs == nil || db.BlogSingles == nil {
		return blogSingleMutationBeforeState{}, db.ErrMongoClientNotInitialized
	}
	var before blogSingleMutationBeforeState
	if err := db.UserBlogs.FindIdContext(ctx, ownerID).One(&before.UserBlog); err != nil {
		return before, err
	}
	var single info.BlogSingle
	err := db.BlogSingles.FindContext(ctx, bson.M{"_id": singleID, "UserId": ownerID}).One(&single)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return before, nil
	}
	if err != nil {
		return before, err
	}
	before.Single = single
	before.HasSingle = true
	return before, nil
}

func cloneBlogSingles(source []map[string]string) []map[string]string {
	if source == nil {
		return nil
	}
	cloned := make([]map[string]string, len(source))
	for index, single := range source {
		if single == nil {
			cloned[index] = map[string]string{}
			continue
		}
		cloned[index] = make(map[string]string, len(single))
		for key, value := range single {
			cloned[index][key] = value
		}
	}
	return cloned
}

func blogSinglesEqual(left, right []map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !reflect.DeepEqual(left[index], right[index]) {
			return false
		}
	}
	return true
}

func blogSingleEqual(left, right info.BlogSingle) bool {
	return left.SingleId == right.SingleId &&
		left.UserId == right.UserId &&
		left.Title == right.Title &&
		left.UrlTitle == right.UrlTitle &&
		left.Content == right.Content &&
		left.UpdatedTime.Equal(right.UpdatedTime) &&
		left.CreatedTime.Equal(right.CreatedTime)
}

func buildBlogSinglesProjection(current []map[string]string, single info.BlogSingle, action string) ([]map[string]string, error) {
	if single.SingleId.IsZero() {
		return nil, fmt.Errorf("single projection: invalid single id")
	}
	projected := cloneBlogSingles(current)
	singleID := single.SingleId.Hex()
	index := -1
	for currentIndex, item := range projected {
		if item["SingleId"] == singleID {
			if index != -1 {
				return nil, fmt.Errorf("single projection: duplicate single id")
			}
			index = currentIndex
		}
	}
	switch action {
	case "add":
		if index != -1 {
			return nil, fmt.Errorf("single projection: single already exists")
		}
		projected = append(projected, map[string]string{
			"SingleId": singleID,
			"Title":    single.Title,
			"UrlTitle": single.UrlTitle,
		})
	case "update":
		if index == -1 {
			return nil, fmt.Errorf("single projection: single not found")
		}
		projected[index]["Title"] = single.Title
		projected[index]["UrlTitle"] = single.UrlTitle
	case "delete":
		if index == -1 {
			return nil, fmt.Errorf("single projection: single not found")
		}
		projected = append(projected[:index], projected[index+1:]...)
	default:
		return nil, fmt.Errorf("single projection: invalid action")
	}
	return projected, nil
}

func blogSinglesUpdate(value []map[string]string) bson.M {
	if value == nil {
		return bson.M{"$unset": bson.M{"Singles": ""}}
	}
	return bson.M{"$set": bson.M{"Singles": cloneBlogSingles(value)}}
}

func verifyBlogSingles(ctx context.Context, ownerID ObjectID, expected []map[string]string) (bool, error) {
	var userBlog info.UserBlog
	if err := db.UserBlogs.FindIdContext(ctx, ownerID).One(&userBlog); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return blogSinglesEqual(userBlog.Singles, expected), nil
}

func runBlogSingleMutation(ownerID, singleID ObjectID, kind string, input any, before blogSingleMutationBeforeState, desired blogSingleMutationDesiredState) bool {
	if ownerID.IsZero() || singleID.IsZero() || db.BlogSingles == nil || db.UserBlogs == nil {
		return false
	}
	operationID, inputDigest, _, err := applicationnotes.NewOperationIdentity(kind, ownerID, singleID, input)
	if err != nil {
		return false
	}
	beforePayload, err := json.Marshal(before)
	if err != nil {
		return false
	}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: singleID, Kind: kind, InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailureCompensate,
		RestoreBeforeState: func(payload []byte) error {
			var frozen blogSingleMutationBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.UserBlog.UserId != ownerID ||
				(frozen.HasSingle && (frozen.Single.SingleId != singleID || frozen.Single.UserId != ownerID)) {
				return fmt.Errorf("single before state target changed")
			}
			before = frozen
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen blogSingleMutationDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil ||
				(frozen.HasSingle && (frozen.Single.SingleId != singleID || frozen.Single.UserId != ownerID)) {
				return fmt.Errorf("single desired state target changed")
			}
			desired = frozen
			return nil
		},
		Steps: []db.WorkspaceMutationStep{
			{
				Name: "single", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					filter := bson.M{"_id": singleID, "UserId": ownerID}
					if desired.HasSingle {
						if before.HasSingle {
							return db.BlogSingles.UpdateOneMatchedContext(ctx, filter, desired.Single)
						}
						return db.BlogSingles.InsertContext(ctx, desired.Single)
					}
					return db.BlogSingles.RemoveContext(ctx, filter)
				},
				Verify: func(ctx context.Context) (bool, error) {
					var current info.BlogSingle
					err := db.BlogSingles.FindContext(ctx, bson.M{"_id": singleID, "UserId": ownerID}).One(&current)
					if !desired.HasSingle {
						return errors.Is(err, mongo.ErrNoDocuments), nil
					}
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					if err != nil {
						return false, err
					}
					return blogSingleEqual(current, desired.Single), nil
				},
				Compensate: func(ctx context.Context) error {
					filter := bson.M{"_id": singleID, "UserId": ownerID}
					if before.HasSingle {
						return db.BlogSingles.UpdateOneMatchedContext(ctx, filter, before.Single)
					}
					return db.BlogSingles.RemoveContext(ctx, filter)
				},
			},
			{
				Name: "singles_projection", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": ownerID}, blogSinglesUpdate(desired.UserBlogSingles))
				},
				Verify: func(ctx context.Context) (bool, error) {
					return verifyBlogSingles(ctx, ownerID, desired.UserBlogSingles)
				},
				Compensate: func(ctx context.Context) error {
					return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": ownerID}, blogSinglesUpdate(before.UserBlog.Singles))
				},
			},
		},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	return err == nil && mutation.Committed
}

func runBlogUserBlogSinglesMutation(ownerID ObjectID, kind string, input any, before info.UserBlog, desired []map[string]string) bool {
	if ownerID.IsZero() || db.UserBlogs == nil {
		return false
	}
	operationID, inputDigest, _, err := applicationnotes.NewOperationIdentity(kind, ownerID, ownerID, input)
	if err != nil {
		return false
	}
	state := blogUserBlogSinglesMutationState{UserBlog: before, Desired: cloneBlogSingles(desired)}
	beforePayload, err := json.Marshal(state)
	if err != nil {
		return false
	}
	desiredPayload, err := json.Marshal(state)
	if err != nil {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: ownerID, Kind: kind, InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailureCompensate,
		RestoreBeforeState: func(payload []byte) error {
			var frozen blogUserBlogSinglesMutationState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.UserBlog.UserId != ownerID {
				return fmt.Errorf("singles order before state target changed")
			}
			before = frozen.UserBlog
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen blogUserBlogSinglesMutationState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.UserBlog.UserId != ownerID {
				return fmt.Errorf("singles order desired state target changed")
			}
			desired = frozen.Desired
			return nil
		},
		Steps: []db.WorkspaceMutationStep{{
			Name: "singles_projection", ReplaySafe: true,
			Apply: func(ctx context.Context) error {
				return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": ownerID}, blogSinglesUpdate(desired))
			},
			Verify: func(ctx context.Context) (bool, error) {
				return verifyBlogSingles(ctx, ownerID, desired)
			},
			Compensate: func(ctx context.Context) error {
				return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": ownerID}, blogSinglesUpdate(before.Singles))
			},
		}},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	return err == nil && mutation.Committed
}

// 修改博客的图片, 描述, 摘要
func (this *BlogService) UpateBlogAbstract(userId string, noteId, imgSrc, desc, abstract string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(noteId) || db.Notes == nil || db.NoteContents == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	noteID := db.MustObjectIDFromHex(noteId)
	var note info.Note
	if err := db.Notes.FindContext(context.Background(), bson.M{"_id": noteID, "UserId": ownerID}).One(&note); err != nil {
		return false
	}
	var content info.NoteContent
	if err := db.NoteContents.FindContext(context.Background(), bson.M{"_id": noteID, "UserId": ownerID}).One(&content); err != nil {
		return false
	}
	input := struct {
		ImgSrc              string
		Desc                string
		Abstract            string
		BeforeNoteUpdatedAt time.Time
		BeforeAbstract      string
	}{imgSrc, desc, abstract, note.UpdatedTime, content.Abstract}
	operationID, inputDigest, _, err := applicationnotes.NewOperationIdentity("blog_abstract", ownerID, noteID, input)
	if err != nil {
		return false
	}
	before := blogAbstractMutationBeforeState{Note: note, Content: content}
	desired := blogAbstractMutationDesiredState{ImgSrc: imgSrc, Desc: desc, HasSelfDefined: true, Abstract: abstract}
	beforePayload, err := json.Marshal(before)
	if err != nil {
		return false
	}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: ownerID, ResourceID: noteID, Kind: "blog_abstract", InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailureCompensate,
		RestoreBeforeState: func(payload []byte) error {
			var frozen blogAbstractMutationBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.Note.NoteId != noteID || frozen.Note.UserId != ownerID || frozen.Content.NoteId != noteID || frozen.Content.UserId != ownerID {
				return fmt.Errorf("blog abstract before state target changed")
			}
			before = frozen
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen blogAbstractMutationDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil {
				return err
			}
			desired = frozen
			return nil
		},
		Steps: []db.WorkspaceMutationStep{
			{
				Name: "note_metadata", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return db.Notes.UpdateOneMatchedContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}, bson.M{"$set": bson.M{"ImgSrc": desired.ImgSrc, "Desc": desired.Desc, "HasSelfDefined": desired.HasSelfDefined}})
				},
				Verify: func(ctx context.Context) (bool, error) {
					var current info.Note
					err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "ImgSrc": desired.ImgSrc, "Desc": desired.Desc, "HasSelfDefined": desired.HasSelfDefined}).One(&current)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil, err
				},
				Compensate: func(ctx context.Context) error {
					return db.Notes.UpdateOneMatchedContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}, bson.M{"$set": bson.M{"ImgSrc": before.Note.ImgSrc, "Desc": before.Note.Desc, "HasSelfDefined": before.Note.HasSelfDefined}})
				},
			},
			{
				Name: "content_abstract", ReplaySafe: true,
				Apply: func(ctx context.Context) error {
					return db.NoteContents.UpdateOneMatchedContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}, bson.M{"$set": bson.M{"Abstract": desired.Abstract}})
				},
				Verify: func(ctx context.Context) (bool, error) {
					var current info.NoteContent
					err := db.NoteContents.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID, "Abstract": desired.Abstract}).One(&current)
					if errors.Is(err, mongo.ErrNoDocuments) {
						return false, nil
					}
					return err == nil, err
				},
				Compensate: func(ctx context.Context) error {
					return db.NoteContents.UpdateOneMatchedContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}, bson.M{"$set": bson.M{"Abstract": before.Content.Abstract}})
				},
			},
		},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	return err == nil && mutation.Committed
}

// 单页
func (this *BlogService) GetSingles(userId string) []map[string]string {
	userBlog := this.GetUserBlog(userId)
	singles := userBlog.Singles
	LogJ(singles)
	return singles
}
func (this *BlogService) GetSingle(singleId string) info.BlogSingle {
	page, _ := this.GetSingleChecked(singleId)
	return page
}
func (this *BlogService) GetSingleByUserIdAndUrlTitle(userId, singleIdOrUrlTitle string) info.BlogSingle {
	page, _ := this.GetSingleByUserIdAndUrlTitleChecked(userId, singleIdOrUrlTitle)
	return page
}

func (this *BlogService) GetSingleChecked(singleId string) (info.BlogSingle, error) {
	if !db.IsValidObjectIDHex(singleId) {
		return info.BlogSingle{}, ErrPublicBlogNotFound
	}
	if db.BlogSingles == nil {
		return info.BlogSingle{}, db.ErrMongoClientNotInitialized
	}
	page := info.BlogSingle{}
	err := db.BlogSingles.Find(bson.M{"_id": db.MustObjectIDFromHex(singleId)}).One(&page)
	if errors.Is(err, mongo.ErrNoDocuments) || page.SingleId.IsZero() {
		return info.BlogSingle{}, ErrPublicBlogNotFound
	}
	if err != nil {
		return info.BlogSingle{}, fmt.Errorf("load public single: %w", err)
	}
	return page, nil
}

func (this *BlogService) GetSingleByUserIdAndUrlTitleChecked(userId, singleIdOrUrlTitle string) (info.BlogSingle, error) {
	if !db.IsValidObjectIDHex(userId) || singleIdOrUrlTitle == "" {
		return info.BlogSingle{}, ErrPublicBlogNotFound
	}
	if db.BlogSingles == nil {
		return info.BlogSingle{}, db.ErrMongoClientNotInitialized
	}
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId)}
	if IsObjectId(singleIdOrUrlTitle) {
		query["_id"] = db.MustObjectIDFromHex(singleIdOrUrlTitle)
	} else {
		query["UrlTitle"] = encodeValue(singleIdOrUrlTitle)
	}
	page := info.BlogSingle{}
	err := db.BlogSingles.Find(query).One(&page)
	if errors.Is(err, mongo.ErrNoDocuments) || page.SingleId.IsZero() {
		return info.BlogSingle{}, ErrPublicBlogNotFound
	}
	if err != nil {
		return info.BlogSingle{}, fmt.Errorf("load public single: %w", err)
	}
	return page, nil
}

// 删除页面
func (this *BlogService) DeleteSingle(userId, singleId string) (ok bool) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(singleId) || db.BlogSingles == nil || db.UserBlogs == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	singleID := db.MustObjectIDFromHex(singleId)
	before, err := loadBlogSingleMutationBefore(context.Background(), ownerID, singleID)
	if err != nil || !before.HasSingle {
		return false
	}
	desiredSingles, err := buildBlogSinglesProjection(before.UserBlog.Singles, before.Single, "delete")
	if err != nil {
		return false
	}
	desired := blogSingleMutationDesiredState{HasSingle: false, UserBlogSingles: desiredSingles}
	input := struct {
		Action          string
		SingleID        string
		BeforeUpdatedAt time.Time
	}{"delete", singleId, before.Single.UpdatedTime}
	return runBlogSingleMutation(ownerID, singleID, "blog_single_delete", input, before, desired)
}

// 修改urlTitle
func (this *BlogService) UpdateSingleUrlTitle(userId, singleId, urlTitle string) (ok bool, url string) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(singleId) || db.BlogSingles == nil || db.UserBlogs == nil {
		return false, urlTitle
	}
	url = urlTitle
	url = GetUrTitle(userId, urlTitle, "single", singleId)
	ownerID := db.MustObjectIDFromHex(userId)
	singleID := db.MustObjectIDFromHex(singleId)
	before, err := loadBlogSingleMutationBefore(context.Background(), ownerID, singleID)
	if err != nil || !before.HasSingle {
		return false, decodeValue(url)
	}
	desiredSingle := before.Single
	desiredSingle.UrlTitle = url
	desiredSingles, err := buildBlogSinglesProjection(before.UserBlog.Singles, desiredSingle, "update")
	if err != nil {
		return false, decodeValue(url)
	}
	input := struct {
		Action          string
		SingleID        string
		UrlTitle        string
		BeforeUpdatedAt time.Time
	}{"update_url_title", singleId, url, before.Single.UpdatedTime}
	ok = runBlogSingleMutation(ownerID, singleID, "blog_single_url_title", input, before, blogSingleMutationDesiredState{Single: desiredSingle, HasSingle: true, UserBlogSingles: desiredSingles})
	// 返回给前端的是decode
	url = decodeValue(url)
	return
}

// 更新或添加
func (this *BlogService) AddOrUpdateSingle(userId, singleId, title, content string) (ok bool) {
	if !db.IsValidObjectIDHex(userId) || db.BlogSingles == nil || db.UserBlogs == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	if singleId != "" {
		if !db.IsValidObjectIDHex(singleId) {
			return false
		}
		singleID := db.MustObjectIDFromHex(singleId)
		before, err := loadBlogSingleMutationBefore(context.Background(), ownerID, singleID)
		if err != nil || !before.HasSingle {
			return false
		}
		desiredSingle := before.Single
		desiredSingle.Title = title
		desiredSingle.Content = content
		desiredSingle.UpdatedTime = time.Now()
		desiredSingles, err := buildBlogSinglesProjection(before.UserBlog.Singles, desiredSingle, "update")
		if err != nil {
			return false
		}
		input := struct {
			Action          string
			SingleID        string
			Title           string
			Content         string
			BeforeUpdatedAt time.Time
		}{"update", singleId, title, content, before.Single.UpdatedTime}
		return runBlogSingleMutation(ownerID, singleID, "blog_single_update", input, before, blogSingleMutationDesiredState{Single: desiredSingle, HasSingle: true, UserBlogSingles: desiredSingles})
	}
	// 添加
	page := info.BlogSingle{
		SingleId:    db.NewObjectID(),
		UserId:      ownerID,
		Title:       title,
		Content:     content,
		UrlTitle:    GetUrTitle(userId, title, "single", singleId),
		CreatedTime: time.Now(),
	}
	page.UpdatedTime = page.CreatedTime
	before, err := loadBlogSingleMutationBefore(context.Background(), ownerID, page.SingleId)
	if err != nil || before.HasSingle {
		return false
	}
	desiredSingles, err := buildBlogSinglesProjection(before.UserBlog.Singles, page, "add")
	if err != nil {
		return false
	}
	input := struct {
		Action   string
		SingleID string
		Title    string
		Content  string
	}{"add", page.SingleId.Hex(), title, content}
	return runBlogSingleMutation(ownerID, page.SingleId, "blog_single_add", input, before, blogSingleMutationDesiredState{Single: page, HasSingle: true, UserBlogSingles: desiredSingles})
}

// 重新排序
func (this *BlogService) SortSingles(userId string, singleIds []string) (ok bool) {
	if !db.IsValidObjectIDHex(userId) || db.UserBlogs == nil {
		return false
	}
	ownerID := db.MustObjectIDFromHex(userId)
	var userBlog info.UserBlog
	if err := db.UserBlogs.FindIdContext(context.Background(), ownerID).One(&userBlog); err != nil || !completeSingleOrder(userBlog.Singles, singleIds) {
		return false
	}
	singlesMap := make(map[string]map[string]string, len(userBlog.Singles))
	for _, page := range userBlog.Singles {
		singlesMap[page["SingleId"]] = page
	}
	desired := make([]map[string]string, len(singleIds))
	for index, singleID := range singleIds {
		desired[index] = cloneBlogSingles([]map[string]string{singlesMap[singleID]})[0]
	}
	input := struct {
		Requested []string
		Before    []map[string]string
	}{append([]string(nil), singleIds...), userBlog.Singles}
	return runBlogUserBlogSinglesMutation(ownerID, "blog_singles_sort", input, userBlog, desired)
}

func completeSingleOrder(singles []map[string]string, singleIds []string) bool {
	if len(singles) == 0 || len(singleIds) != len(singles) {
		return false
	}

	existing := make(map[string]struct{}, len(singles))
	for _, single := range singles {
		singleID := single["SingleId"]
		if singleID == "" {
			return false
		}
		if _, found := existing[singleID]; found {
			return false
		}
		existing[singleID] = struct{}{}
	}

	requested := make(map[string]struct{}, len(singleIds))
	for _, singleID := range singleIds {
		if _, found := existing[singleID]; !found {
			return false
		}
		if _, duplicate := requested[singleID]; duplicate {
			return false
		}
		requested[singleID] = struct{}{}
	}
	return len(requested) == len(existing)
}

// 得到用户的博客url
func (this *BlogService) GetUserBlogUrl(userBlog *info.UserBlog, username string) string {
	/*
		if userBlog != nil {
			if userBlog.Domain != "" && configService.AllowCustomDomain() {
				return configService.GetUserUrl(userBlog.Domain)
			} else if userBlog.SubDomain != "" {
				return configService.GetUserSubUrl(userBlog.SubDomain)
			}
			if username == "" {
				username = userBlog.UserId.Hex()
			}
		}
	*/
	return configService.GetBlogUrl() + "/" + username
}

// 得到所有url
func (this *BlogService) GetBlogUrls(userBlog *info.UserBlog, userInfo *info.User) info.BlogUrls {
	var indexUrl, postUrl, searchUrl, cateUrl, singleUrl, tagsUrl, archiveUrl, tagPostsUrl string

	/*
		if userBlog.Domain != "" && configService.AllowCustomDomain() { // http://demo.com
			// ok
			indexUrl = configService.GetUserUrl(userBlog.Domain)
			cateUrl = indexUrl + "/cate"     // /xxxxx
			postUrl = indexUrl + "/post"     // /xxxxx
			searchUrl = indexUrl + "/search" // /xxxxx
			singleUrl = indexUrl + "/single"
			archiveUrl = indexUrl + "/archives"
			tagsUrl = indexUrl + "/tags"
			tagPostsUrl = indexUrl + "/tag"
		} else if userBlog.SubDomain != "" { // demo.leanote.com
			indexUrl = configService.GetUserSubUrl(userBlog.SubDomain)
			cateUrl = indexUrl + "/cate"     // /xxxxx
			postUrl = indexUrl + "/post"     // /xxxxx
			searchUrl = indexUrl + "/search" // /xxxxx
			singleUrl = indexUrl + "/single"
			archiveUrl = indexUrl + "/archives"
			tagsUrl = indexUrl + "/tags"
			tagPostsUrl = indexUrl + "/tag"
		} else {
	*/
	// ok
	blogUrl := configService.GetBlogUrl() // blog.leanote.com
	userIdOrEmail := ""
	if userInfo.Username != "" {
		userIdOrEmail = userInfo.Username
	} else if userInfo.Email != "" {
		userIdOrEmail = userInfo.Email
	} else {
		userIdOrEmail = userInfo.UserId.Hex()
	}
	indexUrl = blogUrl + "/" + userIdOrEmail
	cateUrl = blogUrl + "/cate/" + userIdOrEmail        // /username/notebookId
	postUrl = blogUrl + "/post/" + userIdOrEmail        // /username/xxxxx
	searchUrl = blogUrl + "/search/" + userIdOrEmail    // blog.leanote.com/search/username
	singleUrl = blogUrl + "/single/" + userIdOrEmail    // blog.leanote.com/single/username/singleId
	archiveUrl = blogUrl + "/archives/" + userIdOrEmail // blog.leanote.com/archive/username
	tagsUrl = blogUrl + "/tags/" + userIdOrEmail
	tagPostsUrl = blogUrl + "/tag/" + userIdOrEmail // blog.leanote.com/archive/username
	// }

	return info.BlogUrls{
		IndexUrl:    indexUrl,
		CateUrl:     cateUrl,
		SearchUrl:   searchUrl,
		SingleUrl:   singleUrl,
		PostUrl:     postUrl,
		ArchiveUrl:  archiveUrl,
		TagsUrl:     tagsUrl,
		TagPostsUrl: tagPostsUrl,
	}
}

// 转成post
func (this *BlogService) FixBlogs(blogs []info.BlogItem) []info.Post {
	blogs2 := make([]info.Post, len(blogs))
	for i, blog := range blogs {
		blogs2[i] = this.FixBlog(blog)
	}
	return blogs2
}
func (this *BlogService) FixBlog(blog info.BlogItem) info.Post {
	urlTitle := blog.UrlTitle
	if urlTitle == "" {
		urlTitle = blog.NoteId.Hex()
	}
	blog2 := info.Post{
		NoteId:      blog.NoteId.Hex(),
		Title:       blog.Title,
		UrlTitle:    urlTitle,
		ImgSrc:      blog.ImgSrc,
		CreatedTime: blog.CreatedTime,
		UpdatedTime: blog.UpdatedTime,
		PublicTime:  blog.PublicTime,
		Desc:        blog.Desc,
		Abstract:    blog.Abstract,
		Content:     blog.Content,
		Tags:        blog.Tags,
		CommentNum:  blog.CommentNum,
		ReadNum:     blog.ReadNum,
		LikeNum:     blog.LikeNum,
		IsMarkdown:  blog.IsMarkdown,
	}
	if blog2.Tags != nil && len(blog2.Tags) > 0 && blog2.Tags[0] != "" {
	} else {
		blog2.Tags = nil
	}
	return blog2
}

func (this *BlogService) FixNote(note info.Note) info.Post {
	if note.NoteId.IsZero() {
		return info.Post{}
	}
	urlTitle := note.UrlTitle
	if urlTitle == "" {
		urlTitle = note.NoteId.Hex()
	}
	return info.Post{
		NoteId:      note.NoteId.Hex(),
		Title:       note.Title,
		ImgSrc:      note.ImgSrc,
		UrlTitle:    urlTitle,
		CreatedTime: note.CreatedTime,
		UpdatedTime: note.UpdatedTime,
		PublicTime:  note.PublicTime,
		Desc:        note.Desc,
		Tags:        note.Tags,
		CommentNum:  note.CommentNum,
		ReadNum:     note.ReadNum,
		LikeNum:     note.LikeNum,
		IsMarkdown:  note.IsMarkdown,
	}
}
