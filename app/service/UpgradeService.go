package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"time"
)

type UpgradeService struct {
}

func upgradeInputDigest(kind, target string) string {
	digest := sha256.Sum256([]byte(kind + "\x00" + target))
	return hex.EncodeToString(digest[:])
}

type dbUpgradeCheckpoint struct{ value info.UpgradeCheckpoint }

func (this *UpgradeService) acquireStep(kind, step, target string) (dbUpgradeCheckpoint, bool, error) {
	checkpoint, done, err := db.AcquireUpgradeCheckpoint(context.Background(), db.UpgradeOperationID(kind, target), step, upgradeInputDigest(kind, target), target, time.Now().UTC())
	if err != nil {
		return dbUpgradeCheckpoint{}, false, err
	}
	return dbUpgradeCheckpoint{value: checkpoint}, done, nil
}

// 添加了PublicTime, RecommendTime
func (this *UpgradeService) UpgradeBlog() (bool, string) {
	cp, done, err := this.acquireStep("upgrade-blog", "public-times", "all")
	if err != nil {
		return false, fmt.Sprintf("upgrade checkpoint: %v", err)
	}
	if done {
		return true, "already applied"
	}
	notes := []info.Note{}
	if err := db.Notes.FindContext(context.Background(), bson.M{"IsBlog": true}).All(&notes); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("list blog notes: %v", err)
	}

	// PublicTime, RecommendTime = UpdatedTime
	for _, note := range notes {
		if note.IsBlog && note.PublicTime.Year() < 100 {
			if err := db.Notes.UpdateOneMatchedContext(context.Background(), bson.M{"_id": note.NoteId, "UserId": note.UserId}, bson.M{"$set": bson.M{"PublicTime": note.UpdatedTime, "RecommendTime": note.UpdatedTime}}); err != nil {
				_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
				return false, fmt.Sprintf("update blog note %s: %v", note.NoteId.Hex(), err)
			}
			Log(note.NoteId.Hex())
		}
	}
	if _, err := db.CompleteUpgradeCheckpoint(context.Background(), cp.value, upgradeInputDigest("upgrade-blog-result", fmt.Sprintf("%d", len(notes))), time.Now().UTC()); err != nil {
		return false, fmt.Sprintf("complete upgrade checkpoint: %v", err)
	}
	return true, "success"
}

// 11-5自定义博客升级, 将aboutMe移至pages
/*
<li>Migrate "About me" to single(a single post)</li>
<li>Add some default themes to administrator</li>
<li>Generate "UrlTitle" for all notes. "UrlTitle" is a friendly url for post</li>
<li>Generate "UrlTitle" for all notebooks</li>
<li>Generate "UrlTitle" for all singles</li>
*/
func (this *UpgradeService) UpgradeBetaToBeta2(userId string) (ok bool, msg string) {
	cp, done, checkpointErr := this.acquireStep("upgrade-beta2", "migration", "all")
	if checkpointErr != nil {
		return false, fmt.Sprintf("upgrade checkpoint: %v", checkpointErr)
	}
	if done {
		return true, "already applied"
	}
	if configService.GetGlobalStringConfig("UpgradeBetaToBeta2") != "" {
		if _, err := db.CompleteUpgradeCheckpoint(context.Background(), cp.value, upgradeInputDigest("upgrade-beta2-result", "already-configured"), time.Now().UTC()); err != nil {
			return false, fmt.Sprintf("complete upgrade checkpoint: %v", err)
		}
		return true, "already applied"
	}

	// 1. aboutMe -> page
	userBlogs := []info.UserBlog{}
	if err := db.UserBlogs.FindContext(context.Background(), bson.M{}).All(&userBlogs); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("list user blogs: %v", err)
	}

	for _, userBlog := range userBlogs {
		if !blogService.AddOrUpdateSingle(userBlog.UserId.Hex(), "", "About Me", userBlog.AboutMe) {
			_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
			return false, fmt.Sprintf("migrate about me for user %s", userBlog.UserId.Hex())
		}
	}

	// 2. 默认主题, 给admin用户
	if !themeService.UpgradeThemeBeta2() {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, "migrate default themes failed"
	}

	// 3. UrlTitles

	// 3.1 note
	notes := []info.Note{}
	if err := db.Notes.FindContext(context.Background(), bson.M{}).All(&notes); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("list notes: %v", err)
	}
	for _, note := range notes {
		data := bson.M{}
		noteId := note.NoteId.Hex()
		// PublicTime, RecommendTime = UpdatedTime
		if note.IsBlog && note.PublicTime.Year() < 100 {
			data["PublicTime"] = note.UpdatedTime
			data["RecommendTime"] = note.UpdatedTime
			Log("Time " + noteId)
		}
		data["UrlTitle"] = GetUrTitle(note.UserId.Hex(), note.Title, "note", noteId)
		if err := db.Notes.UpdateOneMatchedContext(context.Background(), bson.M{"_id": note.NoteId, "UserId": note.UserId}, bson.M{"$set": data}); err != nil {
			_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
			return false, fmt.Sprintf("update note %s: %v", noteId, err)
		}
		Log(noteId)
	}

	// 3.2
	Log("notebook")
	notebooks := []info.Notebook{}
	if err := db.Notebooks.FindContext(context.Background(), bson.M{}).All(&notebooks); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("list notebooks: %v", err)
	}
	for _, notebook := range notebooks {
		notebookId := notebook.NotebookId.Hex()
		data := bson.M{}
		data["UrlTitle"] = GetUrTitle(notebook.UserId.Hex(), notebook.Title, "notebook", notebookId)
		if err := db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{"_id": notebook.NotebookId, "UserId": notebook.UserId}, bson.M{"$set": data}); err != nil {
			_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
			return false, fmt.Sprintf("update notebook %s: %v", notebookId, err)
		}
		Log(notebookId)
	}

	// 3.3 single
	/*
		singles := []info.BlogSingle{}
		db.ListByQ(db.BlogSingles, bson.M{}, &singles)
		for _, single := range singles {
			singleId := single.SingleId.Hex()
			blogService.UpdateSingleUrlTitle(single.UserId.Hex(), singleId, single.Title)
			Log(singleId)
		}
	*/

	// 删除索引
	if err := db.ShareNotes.DropIndex("UserId", "ToUserId", "NoteId"); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("drop obsolete share index: %v", err)
	}
	ok = true
	msg = "success"
	if !configService.UpdateGlobalStringConfig(userId, "UpgradeBetaToBeta2", "1") {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, "persist upgrade marker failed"
	}
	if _, err := db.CompleteUpgradeCheckpoint(context.Background(), cp.value, upgradeInputDigest("upgrade-beta2-result", "completed"), time.Now().UTC()); err != nil {
		return false, fmt.Sprintf("complete upgrade checkpoint: %v", err)
	}

	return
}

// Usn设置
// 客户端 api

func (this *UpgradeService) moveTag() error {
	usnI, err := nextCollectionUSN(db.NoteTags)
	if err != nil {
		return err
	}
	tags := []info.Tag{}
	if err := db.Tags.FindContext(context.Background(), bson.M{}).All(&tags); err != nil {
		return err
	}
	for _, eachTag := range tags {
		tagTitles := eachTag.Tags
		now := time.Now()
		if tagTitles != nil && len(tagTitles) > 0 {
			for _, tagTitle := range tagTitles {
				noteTag := info.NoteTag{}
				noteTag.TagId = db.OutboxEventIDForKey("upgrade-beta4-tag:" + eachTag.UserId.Hex() + ":" + tagTitle)
				noteTag.Count = 1
				noteTag.Tag = tagTitle
				noteTag.UserId = eachTag.UserId
				noteTag.CreatedTime = now
				noteTag.UpdatedTime = now
				noteTag.Usn = usnI
				noteTag.IsDeleted = false
				var existing info.NoteTag
				if err := db.NoteTags.FindIdContext(context.Background(), noteTag.TagId).One(&existing); err == nil {
					if existing.UserId != noteTag.UserId || existing.Tag != noteTag.Tag {
						return errors.New("upgrade tag identity conflict")
					}
					// A prior attempt already materialized this deterministic tag;
					// preserve its USN on replay.
					continue
				} else if errors.Is(err, mongo.ErrNoDocuments) {
					if err := db.NoteTags.InsertContext(context.Background(), noteTag); err != nil {
						return err
					}
				} else {
					return err
				}
				usnI++
			}
		}
	}
	return nil
}

func (this *UpgradeService) setNotebookUsn() error {
	usnI, err := nextCollectionUSN(db.Notebooks)
	if err != nil {
		return err
	}
	notebooks := []info.Notebook{}
	if err := db.Notebooks.FindContext(context.Background(), bson.M{}).All(&notebooks); err != nil {
		return err
	}

	for _, notebook := range notebooks {
		if notebook.Usn > 0 {
			continue
		}
		if err := db.Notebooks.UpdateOneMatchedContext(context.Background(), bson.M{"_id": notebook.NotebookId}, bson.M{"$set": bson.M{"Usn": usnI}}); err != nil {
			return err
		}
		usnI++
	}
	return nil
}

func (this *UpgradeService) setNoteUsn() error {
	usnI, err := nextCollectionUSN(db.Notes)
	if err != nil {
		return err
	}
	notes := []info.Note{}
	if err := db.Notes.FindContext(context.Background(), bson.M{}).All(&notes); err != nil {
		return err
	}

	for _, note := range notes {
		if note.Usn > 0 {
			continue
		}
		if err := db.Notes.UpdateOneMatchedContext(context.Background(), bson.M{"_id": note.NoteId}, bson.M{"$set": bson.M{"Usn": usnI}}); err != nil {
			return err
		}
		usnI++
	}
	return nil
}

// nextCollectionUSN preserves the monotonic USN contract when an upgrade is
// resumed after a partial write. It deliberately queries the same collection
// being rewritten, so a retry cannot reuse values already assigned by an
// earlier attempt or by another migration step.
func nextCollectionUSN(collection *db.Collection) (int, error) {
	if collection == nil {
		return 0, db.ErrMongoClientNotInitialized
	}
	var max struct {
		Usn int `bson:"Usn"`
	}
	if err := collection.FindContext(context.Background(), bson.M{}).Sort("-Usn").Limit(1).One(&max); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 1, nil
		}
		return 0, err
	}
	if max.Usn < 0 {
		return 1, nil
	}
	return max.Usn + 1, nil
}

// 升级为Api, beta.4
func (this *UpgradeService) Api(userId string) (ok bool, msg string) {
	cp, done, checkpointErr := this.acquireStep("upgrade-beta4", "migration", "all")
	if checkpointErr != nil {
		return false, fmt.Sprintf("upgrade checkpoint: %v", checkpointErr)
	}
	if done {
		return true, "already applied"
	}
	if configService.GetGlobalStringConfig("UpgradeBetaToBeta4") != "" {
		if _, err := db.CompleteUpgradeCheckpoint(context.Background(), cp.value, upgradeInputDigest("upgrade-beta4-result", "already-configured"), time.Now().UTC()); err != nil {
			return false, fmt.Sprintf("complete upgrade checkpoint: %v", err)
		}
		return true, "already applied"
	}

	// user
	if _, err := db.Users.UpdateAllContext(context.Background(), bson.M{}, bson.M{"$max": bson.M{"Usn": 200000}}); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("update users: %v", err)
	}
	if count, err := db.Users.FindContext(context.Background(), bson.M{"Usn": bson.M{"$lt": 200000}}).Count(); err != nil || count != 0 {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		if err != nil {
			return false, fmt.Sprintf("verify user usn: %v", err)
		}
		return false, "verify user usn: baseline was not applied"
	}

	// notebook
	if _, err := db.Notebooks.UpdateAllContext(context.Background(), bson.M{}, bson.M{"$set": bson.M{"IsDeleted": false}}); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("update notebooks: %v", err)
	}
	if err := this.setNotebookUsn(); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("set notebook usn: %v", err)
	}

	// note
	// 1-N
	if _, err := db.Notes.UpdateAllContext(context.Background(), bson.M{}, bson.M{"$set": bson.M{"IsDeleted": false}}); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("update notes: %v", err)
	}
	if err := this.setNoteUsn(); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("set note usn: %v", err)
	}

	// tag
	// 1-N
	/// tag, 要重新插入, 将之前的Tag表迁移到NoteTag中
	if err := this.moveTag(); err != nil {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, fmt.Sprintf("migrate tags: %v", err)
	}

	if !configService.UpdateGlobalStringConfig(userId, "UpgradeBetaToBeta4", "1") {
		_ = db.FailUpgradeCheckpoint(context.Background(), cp.value, "storage", time.Now().UTC())
		return false, "persist upgrade marker failed"
	}
	if _, err := db.CompleteUpgradeCheckpoint(context.Background(), cp.value, upgradeInputDigest("upgrade-beta4-result", "completed"), time.Now().UTC()); err != nil {
		return false, fmt.Sprintf("complete upgrade checkpoint: %v", err)
	}

	return true, ""
}
