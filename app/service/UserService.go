package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"strings"
	"time"
)

type UserService struct {
	runActionTokenMutation  func(context.Context, func(context.Context) error, func(context.Context) error) error
	updateUserFields        func(context.Context, domain.ObjectID, bson.M) error
	consumeToken            func(context.Context, string, int, time.Time, time.Duration) error
	resolveToken            func(string, int) (info.Token, error)
	emailExists             func(string) bool
	findUserIDByEmail       func(string) (string, error)
	findUserByName          func(string) (info.User, error)
	findUserByThirdIdentity func(int, string) (info.User, error)
}

func (s *UserService) actionTokenRunner() func(context.Context, func(context.Context) error, func(context.Context) error) error {
	if s.runActionTokenMutation != nil {
		return s.runActionTokenMutation
	}
	return db.RunActionTokenMutation
}

func (s *UserService) updateFields(ctx context.Context, userID domain.ObjectID, fields bson.M) error {
	if s.updateUserFields != nil {
		return s.updateUserFields(ctx, userID, fields)
	}
	if db.Users == nil {
		return db.ErrMongoClientNotInitialized
	}
	return db.Users.UpdateOneMatchedContext(ctx, bson.M{"_id": userID}, bson.M{"$set": fields})
}

func (s *UserService) consumeActionToken(ctx context.Context, token string, tokenType int, now time.Time) error {
	if s.consumeToken != nil {
		return s.consumeToken(ctx, token, tokenType, now, tokenTTL(tokenType))
	}
	_, err := db.ConsumeValidActionToken(ctx, token, tokenType, now, tokenTTL(tokenType))
	return err
}

func (s *UserService) resolveActionToken(token string, tokenType int) (info.Token, error) {
	if s.resolveToken != nil {
		return s.resolveToken(token, tokenType)
	}
	if tokenService == nil {
		return info.Token{}, errors.New("token service is not initialized")
	}
	return tokenService.Resolve(token, tokenType)
}

func (s *UserService) hasEmail(email string) bool {
	if s.emailExists != nil {
		return s.emailExists(email)
	}
	return s.IsExistsUser(email)
}

func tokenLinkError(err error) string {
	if errors.Is(err, db.ErrTokenExpired) || errors.Is(err, db.ErrTokenTypeMismatch) || errors.Is(err, ErrTokenNotFound) {
		return "该链接已过期"
	}
	return "storage"
}

// 自增Usn
// 每次notebook,note添加, 修改, 删除, 都要修改
func (this *UserService) IncrUsn(userId string) int {
	usn, err := this.AllocateUsn(context.Background(), userId)
	if err != nil {
		return 0
	}
	return usn
}

// AllocateUsn is the error-preserving workspace counter boundary. New
// mutations must use it instead of interpreting a zero return as success.
func (this *UserService) AllocateUsn(ctx context.Context, userId string) (int, error) {
	if !db.IsValidObjectIDHex(userId) {
		return 0, fmt.Errorf("allocate workspace usn: invalid user id")
	}
	return db.AllocateUserUSN(ctx, db.MustObjectIDFromHex(userId))
}

func (this *UserService) GetUsn(userId string) int {
	user := info.User{}
	query := bson.M{"_id": db.MustObjectIDFromHex(userId)}
	db.GetByQWithFields(db.Users, query, []string{"Usn"}, &user)
	return user.Usn
}

// 添加用户
func (this *UserService) AddUser(user info.User) bool {
	if user.UserId.IsZero() {
		user.UserId = db.NewObjectID()
	}
	user.CreatedTime = time.Now()

	if user.Email != "" {
		user.Email = strings.ToLower(user.Email)

		// 发送验证邮箱
		go func() {
			emailService.RegisterSendActiveEmail(user, user.Email)
			// 发送给我 life@leanote.com
			// emailService.SendEmail("life@leanote.com", "新增用户", "{header}用户名"+user.Email+"{footer}")
		}()
	}

	return db.Insert(db.Users, user)
}

// FindUserIDByEmail returns the user id for an email, preserving storage
// errors so authentication callers do not confuse database failure with a
// missing account.
func (this *UserService) FindUserIDByEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if this.findUserIDByEmail != nil {
		return this.findUserIDByEmail(email)
	}
	if db.Users == nil {
		return "", db.ErrMongoClientNotInitialized
	}
	user := info.User{}
	err := db.Users.Find(bson.M{"Email": email}).Select(bson.M{"_id": true}).One(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find user id by email: %w", err)
	}
	return user.UserId.Hex(), nil
}

// 通过email得到userId
func (this *UserService) GetUserId(email string) string {
	userID, err := this.FindUserIDByEmail(email)
	if err != nil {
		return ""
	}
	return userID
}

// 得到用户名
func (this *UserService) GetUsername(userId string) string {
	user := info.User{}
	db.GetByQWithFields(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, []string{"Username"}, &user)
	return user.Username
}

// 得到用户名
func (this *UserService) GetUsernameById(userId ObjectID) string {
	user := info.User{}
	db.GetByQWithFields(db.Users, bson.M{"_id": userId}, []string{"Username"}, &user)
	return user.Username
}

// 是否存在该用户 email
func (this *UserService) IsExistsUser(email string) bool {
	if this.GetUserId(email) == "" {
		return false
	}
	return true
}

// 是否存在该用户 username
func (this *UserService) IsExistsUserByUsername(username string) bool {
	return db.Count(db.Users, bson.M{"Username": username}) >= 1
}

// 得到用户信息, userId, username, email
func (this *UserService) GetUserInfoByAny(idEmailUsername string) info.User {
	if IsObjectId(idEmailUsername) {
		return this.GetUserInfo(idEmailUsername)
	}

	if strings.Contains(idEmailUsername, "@") {
		return this.GetUserInfoByEmail(idEmailUsername)
	}

	// username
	return this.GetUserInfoByUsername(idEmailUsername)
}

func (this *UserService) setUserLogo(user *info.User) {
	// Logo路径问题, 有些有http: 有些没有
	if user.Logo == "" {
		user.Logo = "images/blog/default_avatar.png"
	}
	if user.Logo != "" && !strings.HasPrefix(user.Logo, "http") {
		user.Logo = strings.Trim(user.Logo, "/")
		user.Logo = "/" + user.Logo
	}
}

// 仅得到用户
func (this *UserService) GetUser(userId string) info.User {
	user := info.User{}
	db.Get(db.Users, userId, &user)
	return user
}

// 得到用户信息 userId
func (this *UserService) GetUserInfo(userId string) info.User {
	user := info.User{}
	db.Get(db.Users, userId, &user)
	// Logo路径问题, 有些有http: 有些没有
	this.setUserLogo(&user)
	return user
}

// 得到用户信息 email
func (this *UserService) GetUserInfoByEmail(email string) info.User {
	user := info.User{}
	db.GetByQ(db.Users, bson.M{"Email": email}, &user)
	// Logo路径问题, 有些有http: 有些没有
	this.setUserLogo(&user)
	return user
}

// 得到用户信息 username
func (this *UserService) GetUserInfoByUsername(username string) info.User {
	user := info.User{}
	username = strings.ToLower(username)
	db.GetByQ(db.Users, bson.M{"Username": username}, &user)
	// Logo路径问题, 有些有http: 有些没有
	this.setUserLogo(&user)
	return user
}

func (this *UserService) GetUserInfoByThirdUserId(thirdUserId string) info.User {
	user, err := this.FindUserInfoByThirdIdentity(info.ThirdGithub, thirdUserId)
	if err != nil {
		return info.User{}
	}
	return user
}
func (this *UserService) ListUserInfosByUserIds(userIds []ObjectID) []info.User {
	users := []info.User{}
	db.ListByQ(db.Users, bson.M{"_id": bson.M{"$in": userIds}}, &users)
	return users
}
func (this *UserService) ListUserInfosByEmails(emails []string) []info.User {
	users := []info.User{}
	db.ListByQ(db.Users, bson.M{"Email": bson.M{"$in": emails}}, &users)
	return users
}

// 用户信息即可
func (this *UserService) MapUserInfoByUserIds(userIds []ObjectID) map[ObjectID]info.User {
	users := []info.User{}
	db.ListByQ(db.Users, bson.M{"_id": bson.M{"$in": userIds}}, &users)

	userMap := make(map[ObjectID]info.User, len(users))
	for _, user := range users {
		this.setUserLogo(&user)
		userMap[user.UserId] = user
	}
	return userMap
}

// 用户信息和博客设置信息
func (this *UserService) MapUserInfoAndBlogInfosByUserIds(userIds []ObjectID) map[ObjectID]info.User {
	return this.MapUserInfoByUserIds(userIds)
}

// 返回info.UserAndBlog
func (this *UserService) MapUserAndBlogByUserIds(userIds []ObjectID) map[string]info.UserAndBlog {
	userAndBlogMap, _ := this.MapUserAndBlogByUserIdsChecked(userIds)
	return userAndBlogMap
}

func (this *UserService) MapUserAndBlogByUserIdsChecked(userIds []ObjectID) (map[string]info.UserAndBlog, error) {
	if db.Users == nil || db.UserBlogs == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	users := []info.User{}
	if err := db.Users.Find(bson.M{"_id": bson.M{"$in": userIds}}).All(&users); err != nil {
		return nil, fmt.Errorf("list users for blog: %w", err)
	}

	userBlogs := []info.UserBlog{}
	if err := db.UserBlogs.Find(bson.M{"_id": bson.M{"$in": userIds}}).All(&userBlogs); err != nil {
		return nil, fmt.Errorf("list user blogs: %w", err)
	}

	userBlogMap := make(map[ObjectID]info.UserBlog, len(userBlogs))
	for _, user := range userBlogs {
		userBlogMap[user.UserId] = user
	}

	userAndBlogMap := make(map[string]info.UserAndBlog, len(users))

	for _, user := range users {
		this.setUserLogo(&user)

		userBlog, ok := userBlogMap[user.UserId]
		if !ok {
			continue
		}

		userAndBlogMap[user.UserId.Hex()] = info.UserAndBlog{
			UserId:    user.UserId,
			Username:  user.Username,
			Email:     user.Email,
			Logo:      user.Logo,
			BlogTitle: userBlog.Title,
			BlogLogo:  userBlog.Logo,
			BlogUrl:   blogService.GetUserBlogUrl(&userBlog, user.Username),
		}
	}
	return userAndBlogMap, nil
}

// 得到用户信息+博客主页
func (this *UserService) GetUserAndBlogUrl(userId string) info.UserAndBlogUrl {
	user := this.GetUserInfo(userId)
	userBlog := blogService.GetUserBlog(userId)

	blogUrls := blogService.GetBlogUrls(&userBlog, &user)

	return info.UserAndBlogUrl{
		User:    user,
		BlogUrl: blogUrls.IndexUrl,
		PostUrl: blogUrls.PostUrl,
	}
}

// 得到userAndBlog公开信息
func (this *UserService) GetUserAndBlog(userId string) info.UserAndBlog {
	user := this.GetUserInfo(userId)
	userBlog := blogService.GetUserBlog(userId)
	return info.UserAndBlog{
		UserId:    user.UserId,
		Username:  user.Username,
		Email:     user.Email,
		Logo:      user.Logo,
		BlogTitle: userBlog.Title,
		BlogLogo:  userBlog.Logo,
		BlogUrl:   blogService.GetUserBlogUrl(&userBlog, user.Username),
		BlogUrls:  blogService.GetBlogUrls(&userBlog, &user),
	}
}

// 通过ids得到users, 按id的顺序组织users
func (this *UserService) GetUserInfosOrderBySeq(userIds []ObjectID) []info.User {
	users := []info.User{}
	db.ListByQ(db.Users, bson.M{"_id": bson.M{"$in": userIds}}, &users)

	usersMap := map[ObjectID]info.User{}
	for _, user := range users {
		usersMap[user.UserId] = user
	}

	hasAppend := map[ObjectID]bool{} // 为了防止userIds有重复的
	users2 := []info.User{}
	for _, userId := range userIds {
		if user, ok := usersMap[userId]; ok && !hasAppend[userId] {
			hasAppend[userId] = true
			users2 = append(users2, user)
		}
	}
	return users2
}

// FindUserInfoByName returns a user by email or username while preserving
// storage errors. Missing users are returned as a zero-value user with nil
// error so callers can keep account-existence-neutral credential responses.
func (this *UserService) FindUserInfoByName(emailOrUsername string) (info.User, error) {
	emailOrUsername = strings.ToLower(strings.TrimSpace(emailOrUsername))
	if this.findUserByName != nil {
		return this.findUserByName(emailOrUsername)
	}
	if db.Users == nil {
		return info.User{}, db.ErrMongoClientNotInitialized
	}
	user := info.User{}
	var err error
	if strings.Contains(emailOrUsername, "@") {
		err = db.Users.Find(bson.M{"Email": emailOrUsername}).One(&user)
	} else {
		err = db.Users.Find(bson.M{"Username": emailOrUsername}).One(&user)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return info.User{}, nil
	}
	if err != nil {
		return info.User{}, fmt.Errorf("find user by name: %w", err)
	}
	this.setUserLogo(&user)
	return user, nil
}

// 使用email(username), 得到用户信息
func (this *UserService) GetUserInfoByName(emailOrUsername string) info.User {
	user, err := this.FindUserInfoByName(emailOrUsername)
	if err != nil {
		return info.User{}
	}
	return user
}

func (this *UserService) FindUserInfoByThirdIdentity(thirdType int, thirdUserId string) (info.User, error) {
	thirdUserId = strings.TrimSpace(thirdUserId)
	if this.findUserByThirdIdentity != nil {
		return this.findUserByThirdIdentity(thirdType, thirdUserId)
	}
	if thirdUserId == "" {
		return info.User{}, nil
	}
	if db.Users == nil {
		return info.User{}, db.ErrMongoClientNotInitialized
	}
	user := info.User{}
	err := db.Users.Find(bson.M{"ThirdType": thirdType, "ThirdUserId": thirdUserId}).One(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return info.User{}, nil
	}
	if err != nil {
		return info.User{}, fmt.Errorf("find user by third identity: %w", err)
	}
	this.setUserLogo(&user)
	return user, nil
}

// 更新username
func (this *UserService) UpdateUsername(userId, username string) (bool, string) {
	if userId == "" || !db.IsValidObjectIDHex(userId) || username == "" || strings.ToLower(username) == "admin" { // admin用户是内置的, 不能设置
		return false, "usernameIsExisted"
	}
	usernameRaw := username // 原先的, 可能是同一个, 但有大小写
	username = strings.ToLower(username)

	// 先判断是否存在
	userIdO := db.MustObjectIDFromHex(userId)
	if db.Has(db.Users, bson.M{"Username": username, "_id": bson.M{"$ne": userIdO}}) {
		return false, "usernameIsExisted"
	}

	ok := db.UpdateByQMap(db.Users, bson.M{"_id": userIdO}, bson.M{"Username": username, "UsernameRaw": usernameRaw})
	return ok, ""
}

// 修改头像
func (this *UserService) UpdateAvatar(userId, avatarPath string) bool {
	if !db.IsValidObjectIDHex(userId) {
		return false
	}
	userIdO := db.MustObjectIDFromHex(userId)
	return db.UpdateByQField(db.Users, bson.M{"_id": userIdO}, "Logo", avatarPath)
}

// ----------------------
// 已经登录了的用户修改密码
func (this *UserService) UpdatePwd(userId, oldPwd, pwd string) (bool, string) {
	if !db.IsValidObjectIDHex(userId) {
		return false, "validation"
	}
	userInfo := this.GetUserInfo(userId)
	if !ComparePwd(oldPwd, userInfo.Pwd) {
		return false, "oldPasswordError"
	}

	passwd := GenPwd(pwd)
	if passwd == "" {
		return false, "GenerateHash error"
	}

	ok := db.UpdateByQField(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, "Pwd", passwd)
	return ok, ""
}

// 管理员重置密码
func (this *UserService) ResetPwd(adminUserId, userId, pwd string) (ok bool, msg string) {
	if configService.GetAdminUserId() != adminUserId {
		return
	}
	if !db.IsValidObjectIDHex(userId) {
		return false, "validation"
	}

	passwd := GenPwd(pwd)
	if passwd == "" {
		return false, "GenerateHash error"
	}
	ok = db.UpdateByQField(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, "Pwd", passwd)
	return
}

// 修改主题
func (this *UserService) UpdateTheme(userId, theme string) bool {
	ok := db.UpdateByQField(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, "Theme", theme)
	return ok
}

// 帐户类型设置
func (this *UserService) UpdateAccount(userId, accountType string, accountStartTime, accountEndTime time.Time,
	maxImageNum, maxImageSize, maxAttachNum, maxAttachSize, maxPerAttachSize int) bool {
	return db.UpdateByQI(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, info.UserAccount{
		AccountType:      accountType,
		AccountStartTime: accountStartTime,
		AccountEndTime:   accountEndTime,
		MaxImageNum:      maxImageNum,
		MaxImageSize:     maxImageSize,
		MaxAttachNum:     maxAttachNum,
		MaxAttachSize:    maxAttachSize,
		MaxPerAttachSize: maxPerAttachSize,
	})
}

//---------------
// 修改email

// 注册后验证邮箱
func (this *UserService) ActiveEmail(token string) (ok bool, msg, email string) {
	tokenInfo, err := this.resolveActionToken(token, info.TokenActiveEmail)
	if err != nil {
		return false, tokenLinkError(err), ""
	}
	email = tokenInfo.Email
	now := time.Now()
	err = this.actionTokenRunner()(context.Background(), func(ctx context.Context) error {
		return this.updateFields(ctx, tokenInfo.UserId, bson.M{"Verified": true})
	}, func(ctx context.Context) error {
		return this.consumeActionToken(ctx, token, info.TokenActiveEmail, now)
	})
	if err != nil {
		return false, "partial_write", email
	}
	return true, "", email
}

// 修改邮箱
// 在此之前, 验证token是否过期
// 验证email是否有人注册了
func (this *UserService) UpdateEmail(token string) (ok bool, msg, email string) {
	tokenInfo, err := this.resolveActionToken(token, info.TokenUpdateEmail)
	if err != nil {
		return false, tokenLinkError(err), ""
	}
	email = strings.ToLower(tokenInfo.Email)
	if this.hasEmail(email) {
		return false, "该邮箱已注册", email
	}
	now := time.Now()
	err = this.actionTokenRunner()(context.Background(), func(ctx context.Context) error {
		return this.updateFields(ctx, tokenInfo.UserId, bson.M{"Email": email, "Verified": true})
	}, func(ctx context.Context) error {
		return this.consumeActionToken(ctx, token, info.TokenUpdateEmail, now)
	})
	if err != nil {
		return false, "partial_write", email
	}
	return true, "", email
}

//------------
// 偏好设置

// 宽度
func (this *UserService) UpdateColumnWidth(userId string, notebookWidth, noteListWidth, mdEditorWidth int) bool {
	return db.UpdateByQMap(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)},
		bson.M{"NotebookWidth": notebookWidth, "NoteListWidth": noteListWidth, "MdEditorWidth": mdEditorWidth})
}

// 左侧是否隐藏
func (this *UserService) UpdateLeftIsMin(userId string, leftIsMin bool) bool {
	return db.UpdateByQMap(db.Users, bson.M{"_id": db.MustObjectIDFromHex(userId)}, bson.M{"LeftIsMin": leftIsMin})
}

// -------------
// user admin
func (this *UserService) ListUsers(pageNumber, pageSize int, sortField string, isAsc bool, email string) (page info.Page, users []info.User) {
	users = []info.User{}
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)
	query := bson.M{}
	if email != "" {
		orQ := []bson.M{
			bson.M{"Email": bson.M{"$regex": bson.Regex{Pattern: ".*?" + email + ".*", Options: "i"}}},
			bson.M{"Username": bson.M{"$regex": bson.Regex{Pattern: ".*?" + email + ".*", Options: "i"}}},
		}
		query["$or"] = orQ
	}
	q := db.Users.Find(query)
	// 总记录数
	count, _ := q.Count()
	// 列表
	q.Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&users)
	page = info.NewPage(pageNumber, pageSize, count, nil)
	return
}

func (this *UserService) GetAllUserByFilter(userFilterEmail, userFilterWhiteList, userFilterBlackList string, verified bool) []info.User {
	query := bson.M{}

	if verified {
		query["Verified"] = true
	}

	orQ := []bson.M{}
	if userFilterEmail != "" {
		orQ = append(orQ, bson.M{"Email": bson.M{"$regex": bson.Regex{Pattern: ".*?" + userFilterEmail + ".*", Options: "i"}}},
			bson.M{"Username": bson.M{"$regex": bson.Regex{Pattern: ".*?" + userFilterEmail + ".*", Options: "i"}}},
		)
	}
	if userFilterWhiteList != "" {
		userFilterWhiteList = strings.Replace(userFilterWhiteList, "\r", "", -1)
		emails := strings.Split(userFilterWhiteList, "\n")
		orQ = append(orQ, bson.M{"Email": bson.M{"$in": emails}})
	}
	if len(orQ) > 0 {
		query["$or"] = orQ
	}

	emailQ := bson.M{}
	if userFilterBlackList != "" {
		userFilterWhiteList = strings.Replace(userFilterBlackList, "\r", "", -1)
		bEmails := strings.Split(userFilterBlackList, "\n")
		emailQ["$nin"] = bEmails
		query["Email"] = emailQ
	}

	LogJ(query)
	users := []info.User{}
	q := db.Users.Find(query)
	q.All(&users)
	// Log(len(users))

	return users
}

// 统计
func (this *UserService) CountUser() int {
	return db.Count(db.Users, bson.M{})
}
