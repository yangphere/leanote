package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// 登录与权限 Login & Register

var ErrInvalidCredentials = errors.New("invalid_credentials")

type AuthService struct {
	now                     func() time.Time
	userExists              func(string) bool
	usernameExists          func(string) bool
	findUserByName          func(string) (info.User, error)
	findUserByThirdIdentity func(int, string) (info.User, error)
	runUserInitialization   func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error)
	insertUser              func(context.Context, info.User) error
	removeUser              func(context.Context, ObjectID) error
	insertNotebook          func(context.Context, info.Notebook) error
	addRegistrationNotebook func(context.Context, info.Notebook) (info.Notebook, error)
	removeNotebooks         func(context.Context, ObjectID) error
	upsertUserBlog          func(context.Context, info.UserBlog) error
	removeUserBlog          func(context.Context, ObjectID) error
	insertBlogSingle        func(context.Context, info.BlogSingle) error
	removeBlogSingles       func(context.Context, ObjectID) error
	issueActionToken        func(context.Context, ObjectID, string, string, int, time.Time) (db.ActionToken, error)
	sharedUserID            func() string
	sharedNotebooks         func() []map[string]string
	sharedNotes             func() []map[string]string
	copyNoteIDs             func() []string
	insertHasShareNote      func(context.Context, ObjectID, ObjectID) error
	insertShareNotebook     func(context.Context, registrationShareEntry, ObjectID, ObjectID, time.Time) error
	insertShareNote         func(context.Context, registrationShareEntry, ObjectID, ObjectID, time.Time) error
	removeShares            func(context.Context, ObjectID, ObjectID) error
	copyRegistrationNote    func(context.Context, registrationNoteCopy, time.Time) error
	removeCopiedNotes       func(context.Context, []registrationNoteCopy) error
}

type registrationShareEntry struct {
	ID   ObjectID
	Perm int
}

type registrationNoteCopy struct {
	SourceNoteID     ObjectID
	TargetNoteID     ObjectID
	TargetNotebookID ObjectID
	FromUserID       ObjectID
	ToUserID         ObjectID
}

type registrationShareConfig struct {
	SharedUserID ObjectID
	Notebooks    []registrationShareEntry
	Notes        []registrationShareEntry
	CopyNotes    []registrationNoteCopy
}

func (c registrationShareConfig) hasSharedResources() bool {
	return !c.SharedUserID.IsZero() && (len(c.Notebooks) > 0 || len(c.Notes) > 0)
}

func (c registrationShareConfig) hasCopiedNotes() bool {
	return !c.SharedUserID.IsZero() && len(c.CopyNotes) > 0
}

func (s *AuthService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *AuthService) hasUser(email string) bool {
	if s.userExists != nil {
		return s.userExists(email)
	}
	return userService.IsExistsUser(email)
}

func (s *AuthService) hasUsername(username string) bool {
	if s.usernameExists != nil {
		return s.usernameExists(username)
	}
	return userService.IsExistsUserByUsername(username)
}

func (s *AuthService) lookupUserByName(name string) (info.User, error) {
	if s.findUserByName != nil {
		return s.findUserByName(name)
	}
	if userService == nil {
		return info.User{}, errors.New("user service is not initialized")
	}
	return userService.FindUserInfoByName(name)
}

func (s *AuthService) lookupThirdIdentity(thirdType int, thirdUserID string) (info.User, error) {
	if s.findUserByThirdIdentity != nil {
		return s.findUserByThirdIdentity(thirdType, thirdUserID)
	}
	if userService == nil {
		return info.User{}, errors.New("user service is not initialized")
	}
	return userService.FindUserInfoByThirdIdentity(thirdType, thirdUserID)
}

func (s *AuthService) runInitialization(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
	if s.runUserInitialization != nil {
		return s.runUserInitialization(ctx, plan)
	}
	return db.RunUserInitialization(ctx, plan)
}

func (s *AuthService) addUser(ctx context.Context, user info.User) error {
	if s.insertUser != nil {
		return s.insertUser(ctx, user)
	}
	if db.Users == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.InsertContext(ctx, user)
}

func (s *AuthService) deleteUser(ctx context.Context, userID ObjectID) error {
	if s.removeUser != nil {
		return s.removeUser(ctx, userID)
	}
	if db.Users == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.RemoveContext(ctx, bson.M{"_id": userID})
}

func (s *AuthService) addNotebook(ctx context.Context, notebook info.Notebook) (info.Notebook, error) {
	if s.addRegistrationNotebook != nil {
		return s.addRegistrationNotebook(ctx, notebook)
	}
	if s.insertNotebook != nil {
		return notebook, s.insertNotebook(ctx, notebook)
	}
	if db.Notebooks == nil || db.Users == nil {
		return info.Notebook{}, db.ErrMongoClientNotInitialized
	}
	if notebook.NotebookId.IsZero() {
		notebook.NotebookId = db.NewObjectID()
	}
	notebook.UrlTitle = GetUrTitle(notebook.UserId.Hex(), notebook.Title, "notebook", notebook.NotebookId.Hex())
	usn, err := s.nextRegistrationUSN(ctx, notebook.UserId)
	if err != nil {
		return info.Notebook{}, err
	}
	notebook.Usn = usn
	if notebook.CreatedTime.IsZero() {
		notebook.CreatedTime = s.clock()
	}
	if notebook.UpdatedTime.IsZero() {
		notebook.UpdatedTime = notebook.CreatedTime
	}
	if err := db.Notebooks.InsertContext(ctx, notebook); err != nil {
		return info.Notebook{}, err
	}
	return notebook, nil
}

func (s *AuthService) nextRegistrationUSN(ctx context.Context, userID ObjectID) (int, error) {
	var user info.User
	if err := db.Users.FindIdContext(ctx, userID).One(&user); err != nil {
		return 0, fmt.Errorf("read registration user usn: %w", err)
	}
	next := user.Usn + 1
	if err := db.Users.UpdateOneMatchedContext(ctx, bson.M{"_id": userID}, bson.M{"$set": bson.M{"Usn": next}}); err != nil {
		return 0, fmt.Errorf("increment registration user usn: %w", err)
	}
	return next, nil
}

func (s *AuthService) deleteNotebooks(ctx context.Context, userID ObjectID) error {
	if s.removeNotebooks != nil {
		return s.removeNotebooks(ctx, userID)
	}
	if db.Notebooks == nil {
		return db.ErrMongoClientNotInitialized
	}
	_, err := db.Notebooks.RemoveAllContext(ctx, bson.M{"UserId": userID})
	return err
}

func (s *AuthService) saveUserBlog(ctx context.Context, userBlog info.UserBlog) error {
	if s.upsertUserBlog != nil {
		return s.upsertUserBlog(ctx, userBlog)
	}
	if db.UserBlogs == nil {
		return db.ErrMongoClientNotInitialized
	}
	_, err := db.UserBlogs.UpsertContext(ctx, bson.M{"_id": userBlog.UserId}, userBlog)
	return err
}

func (s *AuthService) deleteUserBlog(ctx context.Context, userID ObjectID) error {
	if s.removeUserBlog != nil {
		return s.removeUserBlog(ctx, userID)
	}
	if db.UserBlogs == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.UserBlogs.RemoveContext(ctx, bson.M{"_id": userID})
}

func (s *AuthService) addBlogSingle(ctx context.Context, single info.BlogSingle) error {
	if s.insertBlogSingle != nil {
		return s.insertBlogSingle(ctx, single)
	}
	if db.BlogSingles == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.BlogSingles.InsertContext(ctx, single)
}

func (s *AuthService) deleteBlogSingles(ctx context.Context, userID ObjectID) error {
	if s.removeBlogSingles != nil {
		return s.removeBlogSingles(ctx, userID)
	}
	if db.BlogSingles == nil {
		return db.ErrMongoClientNotInitialized
	}
	_, err := db.BlogSingles.RemoveAllContext(ctx, bson.M{"UserId": userID})
	return err
}

func (s *AuthService) issueActivationToken(ctx context.Context, userID ObjectID, email, value string, now time.Time) (db.ActionToken, error) {
	if s.issueActionToken != nil {
		return s.issueActionToken(ctx, userID, email, value, info.TokenActiveEmail, now)
	}
	return db.IssueActionToken(ctx, userID, email, value, info.TokenActiveEmail, now)
}

func (s *AuthService) registerSharedUserID() string {
	if s.sharedUserID != nil {
		return s.sharedUserID()
	}
	if configService == nil {
		return ""
	}
	return configService.GetGlobalStringConfig("registerSharedUserId")
}

func (s *AuthService) registerSharedNotebooks() []map[string]string {
	if s.sharedNotebooks != nil {
		return s.sharedNotebooks()
	}
	if configService == nil {
		return nil
	}
	return configService.GetGlobalArrMapConfig("registerSharedNotebooks")
}

func (s *AuthService) registerSharedNotes() []map[string]string {
	if s.sharedNotes != nil {
		return s.sharedNotes()
	}
	if configService == nil {
		return nil
	}
	return configService.GetGlobalArrMapConfig("registerSharedNotes")
}

func (s *AuthService) registerCopyNoteIDs() []string {
	if s.copyNoteIDs != nil {
		return s.copyNoteIDs()
	}
	if configService == nil {
		return nil
	}
	return configService.GetGlobalArrayConfig("registerCopyNoteIds")
}

func (s *AuthService) saveHasShareNote(ctx context.Context, sharedUserID, targetUserID ObjectID) error {
	if s.insertHasShareNote != nil {
		return s.insertHasShareNote(ctx, sharedUserID, targetUserID)
	}
	if db.HasShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	query := bson.M{"UserId": sharedUserID, "ToUserId": targetUserID}
	if _, err := db.HasShareNotes.RemoveAllContext(ctx, query); err != nil {
		return err
	}
	return db.HasShareNotes.InsertContext(ctx, info.HasShareNote{
		HasShareNotebookId: db.NewObjectID(),
		UserId:             sharedUserID,
		ToUserId:           targetUserID,
	})
}

func (s *AuthService) saveShareNotebook(ctx context.Context, share registrationShareEntry, sharedUserID, targetUserID ObjectID, now time.Time) error {
	if s.insertShareNotebook != nil {
		return s.insertShareNotebook(ctx, share, sharedUserID, targetUserID, now)
	}
	if db.ShareNotebooks == nil {
		return db.ErrMongoClientNotInitialized
	}
	query := bson.M{"NotebookId": share.ID, "UserId": sharedUserID, "ToUserId": targetUserID}
	if _, err := db.ShareNotebooks.RemoveAllContext(ctx, query); err != nil {
		return err
	}
	return db.ShareNotebooks.InsertContext(ctx, info.ShareNotebook{
		ShareNotebookId: db.NewObjectID(),
		NotebookId:      share.ID,
		UserId:          sharedUserID,
		ToUserId:        targetUserID,
		Perm:            share.Perm,
		CreatedTime:     now,
	})
}

func (s *AuthService) saveShareNote(ctx context.Context, share registrationShareEntry, sharedUserID, targetUserID ObjectID, now time.Time) error {
	if s.insertShareNote != nil {
		return s.insertShareNote(ctx, share, sharedUserID, targetUserID, now)
	}
	if db.ShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	query := bson.M{"NoteId": share.ID, "UserId": sharedUserID, "ToUserId": targetUserID}
	if _, err := db.ShareNotes.RemoveAllContext(ctx, query); err != nil {
		return err
	}
	return db.ShareNotes.InsertContext(ctx, info.ShareNote{
		ShareNoteId: db.NewObjectID(),
		NoteId:      share.ID,
		UserId:      sharedUserID,
		ToUserId:    targetUserID,
		Perm:        share.Perm,
		CreatedTime: now,
	})
}

func (s *AuthService) deleteRegistrationShares(ctx context.Context, sharedUserID, targetUserID ObjectID) error {
	if s.removeShares != nil {
		return s.removeShares(ctx, sharedUserID, targetUserID)
	}
	if db.HasShareNotes == nil || db.ShareNotebooks == nil || db.ShareNotes == nil {
		return db.ErrMongoClientNotInitialized
	}
	query := bson.M{"UserId": sharedUserID, "ToUserId": targetUserID}
	if _, err := db.ShareNotebooks.RemoveAllContext(ctx, query); err != nil {
		return err
	}
	if _, err := db.ShareNotes.RemoveAllContext(ctx, query); err != nil {
		return err
	}
	_, err := db.HasShareNotes.RemoveAllContext(ctx, query)
	return err
}

func (s *AuthService) saveRegistrationNoteCopy(ctx context.Context, copy registrationNoteCopy, now time.Time) (registrationNoteCopy, error) {
	if s.copyRegistrationNote != nil {
		return copy, s.copyRegistrationNote(ctx, copy, now)
	}
	if db.Notes == nil || db.NoteContents == nil || db.Notebooks == nil || db.Users == nil {
		return registrationNoteCopy{}, db.ErrMongoClientNotInitialized
	}
	if copy.SourceNoteID.IsZero() || copy.TargetNoteID.IsZero() || copy.TargetNotebookID.IsZero() || copy.FromUserID.IsZero() || copy.ToUserID.IsZero() {
		return registrationNoteCopy{}, fmt.Errorf("copy registration note: invalid identity")
	}
	var targetNotebook info.Notebook
	if err := db.Notebooks.FindContext(ctx, bson.M{
		"_id":    copy.TargetNotebookID,
		"UserId": copy.ToUserID,
	}).One(&targetNotebook); err != nil {
		return registrationNoteCopy{}, fmt.Errorf("read registration target notebook: %w", err)
	}
	var sourceNote info.Note
	if err := db.Notes.FindContext(ctx, bson.M{
		"_id":       copy.SourceNoteID,
		"UserId":    copy.FromUserID,
		"IsDeleted": false,
	}).One(&sourceNote); err != nil {
		return registrationNoteCopy{}, fmt.Errorf("read registration source note: %w", err)
	}
	var sourceContent info.NoteContent
	if err := db.NoteContents.FindContext(ctx, bson.M{
		"_id":    copy.SourceNoteID,
		"UserId": copy.FromUserID,
	}).One(&sourceContent); err != nil {
		return registrationNoteCopy{}, fmt.Errorf("read registration source note content: %w", err)
	}
	usn, err := s.nextRegistrationUSN(ctx, copy.ToUserID)
	if err != nil {
		return registrationNoteCopy{}, err
	}
	note, content := registrationNoteDocuments(sourceNote, sourceContent, copy, now, usn)
	if err := db.Notes.InsertContext(ctx, note); err != nil {
		writeErr := fmt.Errorf("insert registration note copy: %w", err)
		if cleanupErr := s.deleteRegistrationNoteCopies(ctx, []registrationNoteCopy{copy}); cleanupErr != nil {
			return registrationNoteCopy{}, errors.Join(writeErr, fmt.Errorf("clean uncertain registration note copy: %w", cleanupErr))
		}
		return registrationNoteCopy{}, writeErr
	}
	if err := db.NoteContents.InsertContext(ctx, content); err != nil {
		writeErr := fmt.Errorf("insert registration note content copy: %w", err)
		if cleanupErr := s.deleteRegistrationNoteCopies(ctx, []registrationNoteCopy{copy}); cleanupErr != nil {
			return registrationNoteCopy{}, errors.Join(writeErr, fmt.Errorf("clean partial registration note copy: %w", cleanupErr))
		}
		return registrationNoteCopy{}, writeErr
	}
	return copy, nil
}

func registrationNoteDocuments(sourceNote info.Note, sourceContent info.NoteContent, copy registrationNoteCopy, now time.Time, usn int) (info.Note, info.NoteContent) {
	note := sourceNote
	note.NoteId = copy.TargetNoteID
	note.UserId = copy.ToUserID
	note.CreatedUserId = copy.ToUserID
	note.NotebookId = copy.TargetNotebookID
	note.ImgSrc = ""
	note.AttachNum = 0
	note.IsTrash = false
	note.IsBlog = false
	note.IsRecommend = false
	note.IsTop = false
	note.UrlTitle = ""
	note.RecommendTime = time.Time{}
	note.PublicTime = time.Time{}
	note.CreatedTime = now
	note.UpdatedTime = now
	note.UpdatedUserId = copy.ToUserID
	note.Usn = usn
	note.IsDeleted = false

	content := sourceContent
	content.NoteId = copy.TargetNoteID
	content.UserId = copy.ToUserID
	content.IsBlog = false
	content.CreatedTime = now
	content.UpdatedTime = now
	content.UpdatedUserId = copy.ToUserID
	return note, content
}

func (s *AuthService) deleteRegistrationNoteCopies(ctx context.Context, copies []registrationNoteCopy) error {
	if len(copies) == 0 {
		return nil
	}
	if s.removeCopiedNotes != nil {
		return s.removeCopiedNotes(ctx, copies)
	}
	if db.Notes == nil || db.NoteContents == nil {
		return db.ErrMongoClientNotInitialized
	}
	ids := make([]ObjectID, 0, len(copies))
	targetUserID := copies[0].ToUserID
	for _, copy := range copies {
		ids = append(ids, copy.TargetNoteID)
	}
	query := bson.M{"_id": bson.M{"$in": ids}, "UserId": targetUserID}
	_, contentErr := db.NoteContents.RemoveAllContext(ctx, query)
	_, noteErr := db.Notes.RemoveAllContext(ctx, query)
	return errors.Join(contentErr, noteErr)
}

// 使用bcrypt认证或者Md5认证
// Use bcrypt (Md5 depreciated)
func (this *AuthService) Login(emailOrUsername, pwd string) (info.User, error) {
	emailOrUsername = strings.ToLower(strings.TrimSpace(emailOrUsername))
	//	pwd = strings.Trim(pwd, " ")
	userInfo, err := this.lookupUserByName(emailOrUsername)
	if err != nil {
		return info.User{}, fmt.Errorf("login user lookup: %w", err)
	}
	if userInfo.UserId.IsZero() || !ComparePwd(pwd, userInfo.Pwd) {
		return info.User{}, ErrInvalidCredentials
	}
	return userInfo, nil
}

// 注册
/*
注册 leanote@leanote.com userId = "5368c1aa99c37b029d000001"
添加 在博客上添加一篇欢迎note, note1 5368c1b919807a6f95000000

将nk1(只读), nk2(可写) 分享给该用户
将note1 复制到用户的生活nk上
*/
// 1. 添加用户
// 2. 将leanote共享给我
// [ok]
func (this *AuthService) Register(email, pwd, fromUserId string) (bool, string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !IsEmail(email) {
		return false, "errorEmail"
	}
	if len(pwd) < 6 {
		return false, "errorPassword"
	}
	// 用户是否已存在
	if this.hasUser(email) {
		return false, "userHasBeenRegistered-" + email
	}
	passwd := GenPwd(pwd)
	if passwd == "" {
		return false, "GenerateHash error"
	}
	userID := db.NewObjectID()
	username := this.registrationUsername(email, userID)
	user := info.User{UserId: userID, Email: email, Username: username, UsernameRaw: username, Pwd: passwd}
	if fromUserId != "" && IsObjectId(fromUserId) {
		user.FromUserId = db.MustObjectIDFromHex(fromUserId)
	}
	return this.register(user)
}

func (this *AuthService) registrationUsername(email string, userID ObjectID) string {
	base := email
	if strings.Contains(email, "@") {
		base = strings.Split(email, "@")[0]
	}
	return this.uniqueUsername(base, userID)
}

func (this *AuthService) uniqueUsername(base string, userID ObjectID) string {
	base = strings.ToLower(base)
	var builder strings.Builder
	for _, r := range base {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	username := strings.Trim(builder.String(), "-_")
	if len(username) < 4 {
		suffix := userID.Hex()
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		username = "user-" + suffix
	}
	candidate := username
	for i := 1; this.hasUsername(candidate); i++ {
		candidate = fmt.Sprintf("%s-%d", username, i)
	}
	return candidate
}

func (this *AuthService) register(user info.User) (bool, string) {
	if user.UserId.IsZero() {
		user.UserId = db.NewObjectID()
	}
	now := this.clock()
	user.CreatedTime = now
	activationToken := NewGuidWith(user.Email)
	if user.Email != "" && activationToken == "" {
		return false, "storage"
	}
	plan, err := this.registrationPlan(user, activationToken, now)
	if err != nil {
		return false, "configuration"
	}
	result, err := this.runInitialization(context.Background(), plan)
	if err != nil {
		if errors.Is(err, db.ErrSideEffect) {
			return false, "side_effect"
		}
		if result.PartialWrite || errors.Is(err, db.ErrPartialWrite) {
			return false, "partial_write"
		}
		return false, "storage"
	}
	return true, ""
}

func (this *AuthService) registrationPlan(user info.User, activationToken string, now time.Time) (db.UserInitializationPlan, error) {
	lifeID := db.NewObjectID()
	studyID := db.NewObjectID()
	workID := db.NewObjectID()
	aboutID := db.NewObjectID()
	shareConfig, err := this.registrationShareConfiguration(lifeID, user.UserId)
	if err != nil {
		return db.UserInitializationPlan{}, err
	}
	steps := []db.InitializationStep{
		{
			Name: "user",
			Apply: func(ctx context.Context) error {
				return this.addUser(ctx, user)
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteUser(ctx, user.UserId)
			},
		},
		{
			Name: "default_notebooks",
			Apply: func(ctx context.Context) (err error) {
				applied := false
				defer func() {
					if err != nil && applied {
						if cleanupErr := this.deleteNotebooks(ctx, user.UserId); cleanupErr != nil {
							err = errors.Join(err, fmt.Errorf("clean registration notebooks: %w", cleanupErr))
						}
					}
				}()
				notebooks := []info.Notebook{
					{NotebookId: lifeID, UserId: user.UserId, Seq: -1, Title: "life", CreatedTime: now, UpdatedTime: now},
					{NotebookId: studyID, UserId: user.UserId, Seq: -1, Title: "study", CreatedTime: now, UpdatedTime: now},
					{NotebookId: workID, UserId: user.UserId, Seq: -1, Title: "work", CreatedTime: now, UpdatedTime: now},
				}
				for _, notebook := range notebooks {
					if _, err := this.addNotebook(ctx, notebook); err != nil {
						return err
					}
					applied = true
				}
				return nil
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteNotebooks(ctx, user.UserId)
			},
		},
		{
			Name: "user_blog",
			Apply: func(ctx context.Context) error {
				return this.saveUserBlog(ctx, info.UserBlog{
					UserId:     user.UserId,
					Title:      user.Username + " 's Blog",
					SubTitle:   "Love Leanote!",
					AboutMe:    "Hello, I am (^_^)",
					CanComment: true,
					Singles: []map[string]string{{
						"SingleId": aboutID.Hex(),
						"Title":    "About Me",
						"UrlTitle": "about-me",
					}},
				})
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteUserBlog(ctx, user.UserId)
			},
		},
		{
			Name: "about_single",
			Apply: func(ctx context.Context) error {
				return this.addBlogSingle(ctx, info.BlogSingle{
					SingleId:    aboutID,
					UserId:      user.UserId,
					Title:       "About Me",
					UrlTitle:    "about-me",
					Content:     "Hello, I am (^_^)",
					CreatedTime: now,
					UpdatedTime: now,
				})
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteBlogSingles(ctx, user.UserId)
			},
		},
	}
	if shareConfig.hasSharedResources() {
		steps = append(steps, db.InitializationStep{
			Name: "shared_resources",
			Apply: func(ctx context.Context) (err error) {
				applied := false
				defer func() {
					if err != nil && applied {
						if cleanupErr := this.deleteRegistrationShares(ctx, shareConfig.SharedUserID, user.UserId); cleanupErr != nil {
							err = errors.Join(err, fmt.Errorf("clean registration shares: %w", cleanupErr))
						}
					}
				}()
				if err := this.saveHasShareNote(ctx, shareConfig.SharedUserID, user.UserId); err != nil {
					return err
				}
				applied = true
				for _, notebook := range shareConfig.Notebooks {
					if err := this.saveShareNotebook(ctx, notebook, shareConfig.SharedUserID, user.UserId, now); err != nil {
						return err
					}
				}
				for _, note := range shareConfig.Notes {
					if err := this.saveShareNote(ctx, note, shareConfig.SharedUserID, user.UserId, now); err != nil {
						return err
					}
				}
				return nil
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteRegistrationShares(ctx, shareConfig.SharedUserID, user.UserId)
			},
		})
	}
	if shareConfig.hasCopiedNotes() {
		appliedCopyNotes := make([]registrationNoteCopy, 0, len(shareConfig.CopyNotes))
		steps = append(steps, db.InitializationStep{
			Name: "copy_notes",
			Apply: func(ctx context.Context) error {
				appliedCopyNotes = appliedCopyNotes[:0]
				applied := make([]registrationNoteCopy, 0, len(shareConfig.CopyNotes))
				for _, copy := range shareConfig.CopyNotes {
					appliedCopy, err := this.saveRegistrationNoteCopy(ctx, copy, now)
					if err != nil {
						if cleanupErr := this.deleteRegistrationNoteCopies(ctx, applied); cleanupErr != nil {
							return errors.Join(err, fmt.Errorf("clean copied registration notes: %w", cleanupErr))
						}
						return err
					}
					applied = append(applied, appliedCopy)
				}
				appliedCopyNotes = append(appliedCopyNotes, applied...)
				return nil
			},
			Compensate: func(ctx context.Context) error {
				return this.deleteRegistrationNoteCopies(ctx, appliedCopyNotes)
			},
		})
	}
	var outbox *db.OutboxEvent
	if user.Email != "" {
		steps = append(steps, db.InitializationStep{
			Name: "activation_token",
			Apply: func(ctx context.Context) error {
				_, err := this.issueActivationToken(ctx, user.UserId, user.Email, activationToken, now)
				return err
			},
			Compensate: func(ctx context.Context) error {
				if db.Tokens == nil {
					return db.ErrMongoClientNotInitialized
				}
				_, err := db.Tokens.RemoveAllContext(ctx, bson.M{"UserId": user.UserId, "Type": info.TokenActiveEmail})
				return err
			},
		})
		outbox = &db.OutboxEvent{
			IdempotencyKey: "register:" + user.UserId.Hex(),
			Kind:           "activate-email",
			AggregateID:    user.UserId,
			Payload: map[string]any{
				"userId":    user.UserId.Hex(),
				"email":     user.Email,
				"username":  user.Username,
				"token":     activationToken,
				"tokenType": info.TokenActiveEmail,
			},
		}
	}
	return db.UserInitializationPlan{UserID: user.UserId, Steps: steps, Outbox: outbox}, nil
}

func (this *AuthService) registrationShareConfiguration(defaultNotebookID, targetUserID ObjectID) (registrationShareConfig, error) {
	sharedUserIDText := strings.TrimSpace(this.registerSharedUserID())
	if sharedUserIDText == "" {
		return registrationShareConfig{}, nil
	}
	if !db.IsValidObjectIDHex(sharedUserIDText) {
		return registrationShareConfig{}, fmt.Errorf("registration shared user id: invalid ObjectID")
	}
	sharedUserID := db.MustObjectIDFromHex(sharedUserIDText)
	notebooks, err := parseRegistrationShareEntries(this.registerSharedNotebooks(), "notebookId")
	if err != nil {
		return registrationShareConfig{}, err
	}
	notes, err := parseRegistrationShareEntries(this.registerSharedNotes(), "noteId")
	if err != nil {
		return registrationShareConfig{}, err
	}
	copySourceIDs, err := parseRegistrationObjectIDs(this.registerCopyNoteIDs(), "copy note")
	if err != nil {
		return registrationShareConfig{}, err
	}
	copies := make([]registrationNoteCopy, 0, len(copySourceIDs))
	for _, sourceID := range copySourceIDs {
		copies = append(copies, registrationNoteCopy{
			SourceNoteID:     sourceID,
			TargetNoteID:     db.NewObjectID(),
			TargetNotebookID: defaultNotebookID,
			FromUserID:       sharedUserID,
			ToUserID:         targetUserID,
		})
	}
	return registrationShareConfig{
		SharedUserID: sharedUserID,
		Notebooks:    notebooks,
		Notes:        notes,
		CopyNotes:    copies,
	}, nil
}

func parseRegistrationShareEntries(entries []map[string]string, idKey string) ([]registrationShareEntry, error) {
	shares := make([]registrationShareEntry, 0, len(entries))
	for _, entry := range entries {
		idText := strings.TrimSpace(entry[idKey])
		if idText == "" {
			return nil, fmt.Errorf("registration share %s: missing ObjectID", idKey)
		}
		if !db.IsValidObjectIDHex(idText) {
			return nil, fmt.Errorf("registration share %s: invalid ObjectID", idKey)
		}
		perm := 0
		if permText := strings.TrimSpace(entry["perm"]); permText != "" {
			parsed, err := strconv.Atoi(permText)
			if err != nil || parsed < 0 || parsed > 1 {
				return nil, fmt.Errorf("registration share %s: invalid permission", idKey)
			}
			perm = parsed
		}
		shares = append(shares, registrationShareEntry{
			ID:   db.MustObjectIDFromHex(idText),
			Perm: perm,
		})
	}
	return shares, nil
}

func parseRegistrationObjectIDs(values []string, label string) ([]ObjectID, error) {
	ids := make([]ObjectID, 0, len(values))
	for _, value := range values {
		idText := strings.TrimSpace(value)
		if idText == "" {
			continue
		}
		if !db.IsValidObjectIDHex(idText) {
			return nil, fmt.Errorf("registration %s: invalid ObjectID", label)
		}
		ids = append(ids, db.MustObjectIDFromHex(idText))
	}
	return ids, nil
}

//--------------
// 第三方注册

// 第三方得到用户名, 可能需要多次判断
func (this *AuthService) getUsername(thirdType, thirdUsername string) (username string) {
	return this.uniqueUsername(thirdType+"-"+thirdUsername, db.NewObjectID())
}

func (this *AuthService) ThirdRegister(thirdType, thirdUserId, thirdUsername string) (exists bool, userInfo info.User) {
	thirdTypeValue, err := parseThirdType(thirdType)
	if err != nil {
		return false, info.User{}
	}
	thirdUserId = strings.TrimSpace(thirdUserId)
	if thirdUserId == "" {
		return false, info.User{}
	}
	userInfo, err = this.lookupThirdIdentity(thirdTypeValue, thirdUserId)
	if err != nil {
		return false, info.User{}
	}
	if !userInfo.UserId.IsZero() {
		exists = true
		return
	}

	username := this.getUsername(thirdType, thirdUsername)
	userInfo = info.User{UserId: db.NewObjectID(),
		Username:      username,
		ThirdUserId:   thirdUserId,
		ThirdUsername: thirdUsername,
		ThirdType:     thirdTypeValue,
	}
	ok, _ := this.register(userInfo)
	if !ok {
		return false, info.User{}
	}
	return false, userInfo
}

func parseThirdType(thirdType string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(thirdType)) {
	case "github":
		return info.ThirdGithub, nil
	case "qq":
		return info.ThirdQQ, nil
	default:
		return strconv.Atoi(strings.TrimSpace(thirdType))
	}
}
