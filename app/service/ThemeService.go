package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/revel/revel"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/lea/archive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	_ "golang.org/x/image/bmp"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// 主题
type ThemeService struct {
}

var (
	ErrThemeValidation = errors.New("theme validation error")
	ErrThemeTemplate   = errors.New("theme template error")
	ErrThemePath       = errors.New("theme path error")
)

var defaultStyle = "blog_default"
var elegantStyle = "blog_daqi"
var fixedStyle = "blog_left_fixed"

// admin用户的主题基路径
func (this *ThemeService) getDefaultThemeBasePath() string {
	return revel.BasePath + "/public/blog/themes"
}

// 默认主题路径
func (this *ThemeService) getDefaultThemePath(style string) string {
	if style == elegantStyle {
		return this.getDefaultThemeBasePath() + "/elegant"
	} else if style == fixedStyle {
		return this.getDefaultThemeBasePath() + "/nav_fixed"
	} else {
		return this.getDefaultThemeBasePath() + "/default"
	}
}

// blogService用
func (this *ThemeService) GetDefaultThemePath(style string) string {
	if style == elegantStyle {
		return "public/blog/themes/elegant"
	} else if style == fixedStyle {
		return "public/blog/themes/nav_fixed"
	} else {
		return "public/blog/themes/default"
	}
}

// 得到默认主题
// style是之前的值, 有3个值 blog_default, blog_daqi, blog_left_fixed
func (this *ThemeService) getDefaultTheme(style string) info.Theme {
	if style == elegantStyle {
		return info.Theme{
			IsDefault: true,
			Path:      "public/blog/themes/elegant",
			Name:      "leanote elegant",
			Author:    "leanote",
			AuthorUrl: "http://leanote.com",
			Version:   "1.0",
		}
	} else if style == fixedStyle {
		return info.Theme{
			IsDefault: true,
			Path:      "public/blog/themes/nav_fixed",
			Name:      "leanote nav fixed",
			Author:    "leanote",
			AuthorUrl: "http://leanote.com",
			Version:   "1.0",
		}
	} else { // blog default
		return info.Theme{
			IsDefault: true,
			Path:      "public/blog/themes/default",
			Name:      "leanote default",
			Author:    "leanote",
			AuthorUrl: "http://leanote.com",
			Version:   "1.0",
		}
	}
}

// 用户的主题路径设置
func (this *ThemeService) getUserThemeBasePath(userId string) string {
	if !db.IsValidObjectIDHex(userId) {
		return ""
	}
	return revel.BasePath + "/public/upload/" + Digest3(userId) + "/" + userId + "/themes"
}
func (this *ThemeService) getUserThemePath(userId, themeId string) string {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(themeId) {
		return ""
	}
	return filepath.Join(this.getUserThemeBasePath(userId), themeId)
}
func (this *ThemeService) getUserThemePath2(userId, themeId string) string {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(themeId) {
		return ""
	}
	return "public/upload/" + Digest3(userId) + "/" + userId + "/themes/" + themeId
}

func (this *ThemeService) getUserThemeExportPath(userId string) string {
	if !db.IsValidObjectIDHex(userId) {
		return ""
	}
	return filepath.Join(revel.BasePath, "public", "upload", Digest3(userId), userId, "tmp")
}

func (this *ThemeService) GetThemeUploadTempPath(userId string) string {
	return this.getUserThemeExportPath(userId)
}

func removeThemeTree(path string) bool {
	if path == "" {
		return false
	}
	return os.RemoveAll(path) == nil
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(rel) || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// 新建主题
// 复制默认主题到文件夹下
func (this *ThemeService) CopyDefaultTheme(userBlog info.UserBlog) (ok bool, themeId string) {
	if db.Themes == nil || !db.IsValidObjectIDHex(userBlog.UserId.Hex()) {
		return false, ""
	}
	newThemeId := db.NewObjectID()
	themeId = newThemeId.Hex()
	userId := userBlog.UserId.Hex()
	themePath := this.getUserThemePath(userId, themeId)
	err := os.MkdirAll(themePath, 0755)
	if err != nil {
		return
	}
	// 复制默认主题
	defaultThemePath := this.getDefaultThemePath(userBlog.Style)
	err = CopyDir(defaultThemePath, themePath)
	if err != nil {
		removeThemeTree(themePath)
		return
	}

	// 保存到数据库中
	theme, err := this.getThemeConfig(themePath)
	if err != nil {
		removeThemeTree(themePath)
		return false, ""
	}
	theme.ThemeId = newThemeId
	theme.Path = this.getUserThemePath2(userId, themeId)
	theme.CreatedTime = time.Now()
	theme.UpdatedTime = theme.CreatedTime
	theme.UserId = db.MustObjectIDFromHex(userId)

	ok = db.Insert(db.Themes, theme)
	if !ok {
		removeThemeTree(themePath)
	}
	return ok, themeId
}

// 第一次新建主题
// 设为active true
func (this *ThemeService) NewThemeForFirst(userBlog info.UserBlog) (ok bool, themeId string) {
	ok, themeId = this.CopyDefaultTheme(userBlog)
	if !ok || themeId == "" {
		return false, ""
	}
	if !this.ActiveTheme(userBlog.UserId.Hex(), themeId) {
		removeThemeTree(this.getUserThemePath(userBlog.UserId.Hex(), themeId))
		if db.Themes != nil && db.IsValidObjectIDHex(themeId) {
			db.Delete(db.Themes, bson.M{
				"_id":    db.MustObjectIDFromHex(themeId),
				"UserId": userBlog.UserId,
			})
		}
		return false, ""
	}
	// db.UpdateByQField(db.Themes, bson.M{"_id": db.MustObjectIDFromHex(themeId)}, "IsActive", true)
	return true, themeId
}

// 新建主题, 判断是否有主题了
func (this *ThemeService) NewTheme(userId string) (ok bool, themeId string) {
	userBlog := blogService.GetUserBlog(userId)
	// 如果还没有主题, 那先复制旧的主题
	if userBlog.ThemeId.IsZero() {
		if ok, _ := this.NewThemeForFirst(userBlog); !ok {
			return false, ""
		}
	}
	// 再copy一个默认主题
	userBlog.Style = "defaultStyle"
	ok, themeId = this.CopyDefaultTheme(userBlog)
	return
}

// 将字符串转成Theme配置
func (this *ThemeService) parseConfig(configStr string) (theme info.Theme, err error) {
	theme = info.Theme{}
	// 除去/**/注释
	reg, _ := regexp.Compile("/\\*[\\s\\S]*?\\*/")
	configStr = reg.ReplaceAllString(configStr, "")
	// 转成map
	config := map[string]interface{}{}
	err = json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		return
	}
	// 没有错, 则将Name, Version, Author, AuthorUrl
	metadata := map[string]*string{
		"Name":      &theme.Name,
		"Version":   &theme.Version,
		"Author":    &theme.Author,
		"AuthorUrl": &theme.AuthorUrl,
	}
	for field, target := range metadata {
		value, present := config[field]
		if !present || value == nil {
			continue
		}
		stringValue, ok := value.(string)
		if !ok {
			return info.Theme{}, fmt.Errorf("%w: %s must be a string", ErrThemeValidation, field)
		}
		*target = stringValue
	}
	theme.Info = config

	return
}

// 读取theme.json得到值
func (this *ThemeService) getThemeConfig(themePath string) (theme info.Theme, err error) {
	theme = info.Theme{}
	configBytes, readErr := os.ReadFile(filepath.Join(themePath, "theme.json"))
	if readErr != nil {
		return theme, fmt.Errorf("%w: read theme.json: %v", ErrThemeValidation, readErr)
	}
	configStr := string(configBytes)
	theme, err = this.parseConfig(configStr)
	return
}

func (this *ThemeService) GetTheme(userId, themeId string) info.Theme {
	theme := info.Theme{}
	userObjectID, userErr := parseThemeObjectID(userId)
	themeObjectID, themeErr := parseThemeObjectID(themeId)
	if userErr == nil && themeErr == nil && db.Themes != nil {
		db.GetByQ(db.Themes, bson.M{"_id": themeObjectID, "UserId": userObjectID}, &theme)
	}
	return theme
}
func (this *ThemeService) GetThemeById(themeId string) info.Theme {
	theme := info.Theme{}
	themeObjectID, themeErr := parseThemeObjectID(themeId)
	if themeErr == nil && db.Themes != nil {
		db.GetByQ(db.Themes, bson.M{"_id": themeObjectID}, &theme)
	}
	return theme
}

// 得到主题信息, 为了给博客用
func (this *ThemeService) GetThemeInfo(themeId, style string) map[string]interface{} {
	q := bson.M{}
	if themeId == "" {
		if style == "" {
			style = defaultStyle
		}
		q["Style"] = style
		q["IsDefault"] = true
	} else {
		themeObjectID, err := parseThemeObjectID(themeId)
		if err != nil || db.Themes == nil {
			return nil
		}
		q["_id"] = themeObjectID
	}
	if db.Themes == nil {
		return nil
	}
	theme := info.Theme{}
	if err := db.Themes.Find(q).One(&theme); err != nil || theme.ThemeId.IsZero() {
		return nil
	}
	if _, err := this.safeThemeAbsolutePath(theme.UserId.Hex(), theme.ThemeId.Hex(), theme.Path); err != nil {
		return nil
	}
	return theme.Info
}

// 得到用户的主题
// 若用户没有主题, 则至少有一个默认主题
// 第一个返回值是当前active
func (this *ThemeService) GetUserThemes(userId string) (theme info.Theme, themes []info.Theme) {
	theme = info.Theme{}
	themes = []info.Theme{}
	if !db.IsValidObjectIDHex(userId) || db.Themes == nil {
		return theme, themes
	}

	//	db.ListByQ(db.Themes, bson.M{"UserId": db.MustObjectIDFromHex(userId)}, &themes)

	// 创建时间逆序
	query := bson.M{"UserId": db.MustObjectIDFromHex(userId)}
	q := db.Themes.Find(query)
	q.Sort("-CreatedTime").All(&themes)
	if len(themes) == 0 {
		userBlog := blogService.GetUserBlog(userId)
		theme = this.getDefaultTheme(userBlog.Style)
	} else {
		var has = false
		// 第一个是active的主题
		themes2 := make([]info.Theme, len(themes))
		i := 0
		for _, t := range themes {
			if t.IsActive {
				theme = t
			} else {
				has = true
				themes2[i] = t
				i++
			}
		}
		if has {
			themes = themes2
		} else {
			themes = nil
		}
	}
	return
}

// 得到默认主题供选择
func (this *ThemeService) GetDefaultThemes() []info.Theme {
	themes, _ := this.GetDefaultThemesChecked()
	return themes
}

func (this *ThemeService) GetDefaultThemesChecked() ([]info.Theme, error) {
	adminID, err := configuredThemeAdminID()
	if err != nil {
		return nil, err
	}
	if db.Themes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	var themes []info.Theme
	if err := db.Themes.Find(bson.M{"UserId": adminID, "IsDefault": true}).All(&themes); err != nil {
		return nil, err
	}
	valid := make([]info.Theme, 0, len(themes))
	for _, theme := range themes {
		if _, err := this.validatePublicThemeSource(theme, adminID); err == nil {
			valid = append(valid, theme)
		}
	}
	return valid, nil
}

func configuredThemeAdminID() (ObjectID, error) {
	if configService == nil {
		return ObjectID{}, ErrThemeValidation
	}
	return parseThemeObjectID(configService.GetAdminUserId())
}

func (this *ThemeService) publicThemePath(theme info.Theme, adminID ObjectID) bool {
	if theme.ThemeId.IsZero() || theme.UserId != adminID {
		return false
	}
	return theme.Path == "public/blog/themes/default" ||
		theme.Path == "public/blog/themes/elegant" ||
		theme.Path == "public/blog/themes/nav_fixed" ||
		theme.Path == this.getUserThemePath2(adminID.Hex(), theme.ThemeId.Hex())
}

func (this *ThemeService) validateThemeSource(theme info.Theme, ownerID ObjectID) (string, error) {
	if theme.ThemeId.IsZero() || theme.UserId != ownerID {
		return "", ErrThemeValidation
	}
	root, err := this.safeThemeAbsolutePath(ownerID.Hex(), theme.ThemeId.Hex(), theme.Path)
	if err != nil {
		return "", err
	}
	metadata, err := this.getThemeConfig(root)
	if err != nil {
		return "", err
	}
	if metadata.Name == "" || metadata.Name != theme.Name || metadata.Version != theme.Version || metadata.Author != theme.Author || metadata.AuthorUrl != theme.AuthorUrl {
		return "", ErrThemeValidation
	}
	if ok, _ := this.ValidateTheme(root, "", ""); !ok {
		return "", ErrThemeTemplate
	}
	return root, nil
}

func (this *ThemeService) validatePublicThemeSource(theme info.Theme, adminID ObjectID) (string, error) {
	if !theme.IsDefault || !this.publicThemePath(theme, adminID) {
		return "", ErrThemeValidation
	}
	return this.validateThemeSource(theme, adminID)
}

func validateFilename(filename string) bool {
	if filename == "" || strings.ContainsRune(filename, 0) || strings.ContainsRune(filename, '\\') {
		return false
	}
	path := filepath.FromSlash(filename)
	if filepath.IsAbs(path) {
		return false
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || filepath.ToSlash(cleanPath) != filename {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(cleanPath), "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, ':') {
			return false
		}
	}
	return true
}

func validateThemeImageFilename(filename string) bool {
	return validateFilename(filename) && !strings.ContainsAny(filename, "/\\")
}

func validateThemePath(userId, themeId, themePath string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(themeId) || themePath == "" {
		return false
	}
	cleanPath := filepath.Clean(filepath.FromSlash(themePath))
	if filepath.IsAbs(cleanPath) || cleanPath == "." || strings.ContainsRune(cleanPath, 0) {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(cleanPath), "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `/\\:`) {
			return false
		}
	}

	customRoot := filepath.Join("public", "upload", Digest3(userId), userId, "themes", themeId)
	defaultRoot := filepath.Join("public", "blog", "themes")
	cleanPath = filepath.Clean(cleanPath)
	return cleanPath == customRoot || (pathWithin(defaultRoot, cleanPath) && cleanPath != defaultRoot)
}

func (this *ThemeService) safeThemeAbsolutePath(userId, themeId, themePath string) (string, error) {
	if !validateThemePath(userId, themeId, themePath) {
		return "", ErrThemePath
	}
	basePath, err := filepath.Abs(revel.BasePath)
	if err != nil {
		return "", fmt.Errorf("%w: resolve base path: %v", ErrThemePath, err)
	}
	absPath := filepath.Clean(filepath.Join(basePath, filepath.FromSlash(themePath)))
	if !pathWithin(basePath, absPath) {
		return "", ErrThemePath
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("%w: template root unavailable", ErrThemePath)
	}
	allowedRoot := filepath.Join(basePath, "public", "upload", Digest3(userId), userId, "themes", themeId)
	if pathWithin(filepath.Join(basePath, "public", "blog", "themes"), absPath) {
		allowedRoot = filepath.Join(basePath, "public", "blog", "themes")
	}
	if !pathWithin(allowedRoot, resolvedPath) {
		return "", ErrThemePath
	}
	if stat, err := os.Stat(resolvedPath); err != nil || !stat.IsDir() {
		return "", fmt.Errorf("%w: template root unavailable", ErrThemePath)
	}
	return resolvedPath, nil
}

func (this *ThemeService) ResolvePreviewTheme(userId, themeId string) (info.Theme, error) {
	userObjectID, err := parseThemeObjectID(userId)
	if err != nil {
		return info.Theme{}, err
	}
	themeObjectID, err := parseThemeObjectID(themeId)
	if err != nil {
		return info.Theme{}, err
	}
	if db.Themes == nil {
		return info.Theme{}, fmt.Errorf("%w: themes collection unavailable", ErrThemePath)
	}
	theme := info.Theme{}
	if err := db.Themes.Find(bson.M{"_id": themeObjectID, "UserId": userObjectID}).One(&theme); err != nil {
		return info.Theme{}, fmt.Errorf("theme lookup: %w", err)
	}
	absPath, err := this.safeThemeAbsolutePath(userId, themeId, theme.Path)
	if err != nil {
		return info.Theme{}, err
	}
	if _, err := this.getThemeConfig(absPath); err != nil {
		return info.Theme{}, err
	}
	return theme, nil
}

func parseThemeObjectID(value string) (ObjectID, error) {
	if !db.IsValidObjectIDHex(value) {
		return ObjectID{}, fmt.Errorf("%w: invalid theme identifier", ErrThemeValidation)
	}
	return db.MustObjectIDFromHex(value), nil
}

func (this *ThemeService) ownedThemeRoot(userId, themeId string) (info.Theme, string, error) {
	userObjectID, err := parseThemeObjectID(userId)
	if err != nil {
		return info.Theme{}, "", err
	}
	themeObjectID, err := parseThemeObjectID(themeId)
	if err != nil {
		return info.Theme{}, "", err
	}
	if db.Themes == nil {
		return info.Theme{}, "", fmt.Errorf("%w: themes collection unavailable", ErrThemePath)
	}
	theme := info.Theme{}
	if err := db.Themes.Find(bson.M{"_id": themeObjectID, "UserId": userObjectID}).One(&theme); err != nil {
		return info.Theme{}, "", fmt.Errorf("theme lookup: %w", err)
	}
	root, err := this.safeThemeAbsolutePath(userId, themeId, theme.Path)
	if err != nil {
		return info.Theme{}, "", err
	}
	return theme, root, nil
}

func (this *ThemeService) safeThemeFilePath(userId, themeId, filename string) (string, error) {
	if !validateFilename(filename) {
		return "", ErrThemePath
	}
	_, root, err := this.ownedThemeRoot(userId, themeId)
	if err != nil {
		return "", err
	}
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(filename)))
	if !pathWithin(root, path) {
		return "", ErrThemePath
	}
	if fileInfo, lstatErr := os.Lstat(path); lstatErr == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			resolved, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr != nil || !pathWithin(root, resolved) {
				return "", ErrThemePath
			}
		}
	} else if !os.IsNotExist(lstatErr) {
		return "", fmt.Errorf("%w: inspect template path: %v", ErrThemePath, lstatErr)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || !pathWithin(root, parent) {
		return "", ErrThemePath
	}
	return path, nil
}

func safeThemeArchiveName(name, themeId string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
	name = strings.Map(func(r rune) rune {
		if r == 0 || unicode.IsControl(r) || r == '/' || r == '\\' || r == ':' {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" || name == "." || name == ".." {
		name = "theme-" + themeId
	}
	return name + ".zip"
}

// 得到模板内容
func (this *ThemeService) GetTplContent(userId, themeId, filename string) string {
	content, _ := this.ReadTplContent(userId, themeId, filename)
	return content
}

func (this *ThemeService) ReadTplContent(userId, themeId, filename string) (string, bool) {
	path, err := this.safeThemeFilePath(userId, themeId, filename)
	if err != nil {
		return "", false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(content), true
}

// 得到主题的绝对路径
func (this *ThemeService) GetThemeAbsolutePath(userId, themeId string) string {
	_, path, err := this.ownedThemeRoot(userId, themeId)
	if err == nil {
		return path
	}
	return ""
}
func (this *ThemeService) GetThemePath(userId, themeId string) string {
	theme, _, err := this.ownedThemeRoot(userId, themeId)
	if err != nil || theme.Path == "" {
		return ""
	}
	return filepath.ToSlash(theme.Path)
}

// 更新模板内容
func (this *ThemeService) UpdateTplContent(userId, themeId, filename, content string) (ok bool, msg string) {
	if !validateFilename(filename) {
		return
	}

	path, pathErr := this.safeThemeFilePath(userId, themeId, filename)
	if pathErr != nil {
		return false, pathErr.Error()
	}
	_, basePath, pathErr := this.ownedThemeRoot(userId, themeId)
	if pathErr != nil {
		return false, pathErr.Error()
	}
	if strings.HasSuffix(strings.ToLower(filename), ".html") {
		// Log(">>")
		if ok, msg = this.ValidateTheme(basePath, filename, content); ok {
			// 模板
			if ok, msg = this.mustTpl(filename, content); ok {
				ok = PutFileStrContent(path, content)
			}
		}
		return
	} else if filename == "theme.json" {
		// 主题配置, 判断是否是正确的json
		theme, err := this.parseConfig(content)
		if err != nil {
			return false, fmt.Sprintf("%v", err)
		}
		oldContent, oldErr := os.ReadFile(path)
		if oldErr != nil && !os.IsNotExist(oldErr) {
			return false, fmt.Sprintf("%v", oldErr)
		}
		// 正确, 更新theme信息
		if !PutFileStrContent(path, content) {
			return false, "write theme.json failed"
		}
		userObjectID, userIDErr := parseThemeObjectID(userId)
		themeObjectID, themeIDErr := parseThemeObjectID(themeId)
		if userIDErr != nil || themeIDErr != nil || db.Themes == nil {
			if oldErr == nil {
				_ = os.WriteFile(path, oldContent, 0666)
			} else {
				_ = os.Remove(path)
			}
			return false, "theme metadata update failed"
		}
		ok = db.UpdateByQMap(db.Themes, bson.M{"_id": themeObjectID, "UserId": userObjectID},
			bson.M{
				"Name":      theme.Name,
				"Version":   theme.Version,
				"Author":    theme.Author,
				"AuthorUrl": theme.AuthorUrl,
				"Info":      theme.Info,
			})
		if !ok {
			if oldErr == nil {
				_ = os.WriteFile(path, oldContent, 0666)
			} else {
				_ = os.Remove(path)
			}
		}
		return
	}
	ok = PutFileStrContent(path, content)
	return
}

func (this *ThemeService) DeleteTpl(userId, themeId, filename string) (ok bool) {
	path, err := this.safeThemeFilePath(userId, themeId, filename)
	if err != nil {
		return false
	}
	ok = DeleteFile(path)
	return
}

func (this *ThemeService) themeImagesPath(userId, themeId string) (string, error) {
	path, err := this.safeThemeFilePath(userId, themeId, "images")
	if err != nil {
		return "", err
	}
	if info, lstatErr := os.Lstat(path); lstatErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", ErrThemePath
		}
	} else if os.IsNotExist(lstatErr) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("%w: create images directory: %v", ErrThemePath, err)
		}
	} else {
		return "", fmt.Errorf("%w: inspect images directory: %v", ErrThemePath, lstatErr)
	}
	return path, nil
}

func (this *ThemeService) ListThemeImages(userId, themeId string) ([]string, bool) {
	path, err := this.themeImagesPath(userId, themeId)
	if err != nil {
		return nil, false
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, false
	}
	images := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !validateThemeImageFilename(entry.Name()) {
			continue
		}
		images = append(images, entry.Name())
	}
	return images, true
}

func (this *ThemeService) DeleteThemeImage(userId, themeId, filename string) bool {
	if !validateThemeImageFilename(filename) {
		return false
	}
	path, err := this.safeThemeFilePath(userId, themeId, filepath.ToSlash(filepath.Join("images", filename)))
	if err != nil {
		return false
	}
	return os.Remove(path) == nil
}

func (this *ThemeService) SaveThemeImage(userId, themeId, filename string, data []byte) (bool, string) {
	if len(data) == 0 || !validateThemeImageFilename(filename) {
		return false, "图片文件名无效"
	}
	_, ext := SplitFilename(filename)
	format, formatErr := imageFormat(data)
	if formatErr != nil {
		return false, "不是图片"
	}
	if !isSupportedThemeImage(format, ext) {
		return false, "不是图片"
	}
	if len(data) > 5*1024*1024 {
		return false, "图片大于5M"
	}
	if _, err := this.themeImagesPath(userId, themeId); err != nil {
		return false, "主题图片目录不可用"
	}
	path, err := this.safeThemeFilePath(userId, themeId, filepath.ToSlash(filepath.Join("images", filename)))
	if err != nil {
		return false, "主题图片路径无效"
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return false, "图片写入失败"
	}
	converted, convertedPath := TransToGif(path, 0, true)
	if !converted {
		_ = os.Remove(path)
		if convertedPath != "" && convertedPath != path {
			_ = os.Remove(convertedPath)
		}
		return false, "图片转换失败"
	}
	return true, filename
}

func imageFormat(data []byte) (string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return "", fmt.Errorf("invalid image")
	}
	return format, nil
}

func isSupportedThemeImage(format, extension string) bool {
	switch strings.ToLower(extension) {
	case ".gif":
		return format == "gif"
	case ".jpg", ".jpeg":
		return format == "jpeg"
	case ".png":
		return format == "png"
	case ".bmp":
		return format == "bmp"
	default:
		return false
	}
}

// 判断是否有语法错误
func (this *ThemeService) mustTpl(filename, content string) (ok bool, msg string) {
	ok = true
	defer func() {
		if err := recover(); err != nil {
			ok = false
			msg = fmt.Sprintf("%v: %v", ErrThemeTemplate, err)
		}
	}()
	template.Must(template.New(filename).Funcs(revel.TemplateFuncs).Parse(content))
	return
}

/////////

// 使用主题
type themeActivationBeforeState struct {
	ActiveThemes []info.Theme  `json:"activeThemes"`
	UserBlog     info.UserBlog `json:"userBlog"`
}

type themeActivationDesiredState struct {
	ThemeID ObjectID `json:"themeId"`
}

type themeActivationOperationInput struct {
	ActiveThemeIDs []string `json:"activeThemeIds"`
	BlogThemeID    string   `json:"blogThemeId"`
	Nonce          string   `json:"nonce,omitempty"`
}

func verifyThemeActivation(ctx context.Context, ownerID, themeID ObjectID) (bool, error) {
	var themes []info.Theme
	if err := db.Themes.FindContext(ctx, bson.M{"UserId": ownerID}).All(&themes); err != nil {
		return false, err
	}
	var userBlog info.UserBlog
	if err := db.UserBlogs.FindIdContext(ctx, ownerID).One(&userBlog); err != nil {
		return false, err
	}
	activeCount := 0
	for _, theme := range themes {
		if !theme.IsActive {
			continue
		}
		activeCount++
		if theme.ThemeId != themeID {
			return false, nil
		}
	}
	return activeCount == 1 && userBlog.ThemeId == themeID, nil
}

func restoreThemeActivation(ctx context.Context, ownerID ObjectID, before themeActivationBeforeState) error {
	if _, err := db.Themes.UpdateAllContext(ctx, bson.M{"UserId": ownerID}, bson.M{"$set": bson.M{"IsActive": false}}); err != nil {
		return err
	}
	for _, theme := range before.ActiveThemes {
		if err := db.Themes.UpdateOneMatchedContext(ctx, bson.M{"_id": theme.ThemeId, "UserId": ownerID}, bson.M{"$set": bson.M{"IsActive": true}}); err != nil {
			return err
		}
	}
	return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": ownerID}, bson.M{"$set": bson.M{"ThemeId": before.UserBlog.ThemeId}})
}

// 使用主题
func (this *ThemeService) ActiveTheme(userId, themeId string) (ok bool) {
	userObjectID, userErr := parseThemeObjectID(userId)
	themeObjectID, themeErr := parseThemeObjectID(themeId)
	if userErr != nil || themeErr != nil || db.Themes == nil || db.UserBlogs == nil {
		return false
	}
	targetQuery := bson.M{"_id": themeObjectID, "UserId": userObjectID}
	var target info.Theme
	if err := db.Themes.Find(targetQuery).One(&target); err != nil {
		return false
	}
	if _, err := this.safeThemeAbsolutePath(userId, themeId, target.Path); err != nil {
		return false
	}
	var before themeActivationBeforeState
	if err := db.Themes.Find(bson.M{"UserId": userObjectID, "IsActive": true}).All(&before.ActiveThemes); err != nil {
		return false
	}
	if err := db.UserBlogs.FindId(userObjectID).One(&before.UserBlog); err != nil {
		return false
	}
	pending, hasPending, err := db.UnfinishedWorkspaceOperation(context.Background(), userObjectID, ObjectID{}, "theme_activate")
	if err != nil || (hasPending && pending.ResourceID != themeObjectID) {
		return false
	}
	if !hasPending {
		verified, err := verifyThemeActivation(context.Background(), userObjectID, themeObjectID)
		if err != nil {
			return false
		}
		if verified {
			return true
		}
	}
	targetWasActive := target.IsActive
	activeThemeIDs := make([]string, 0, len(before.ActiveThemes))
	for _, activeTheme := range before.ActiveThemes {
		activeThemeIDs = append(activeThemeIDs, activeTheme.ThemeId.Hex())
	}
	sort.Strings(activeThemeIDs)
	input := themeActivationOperationInput{ActiveThemeIDs: activeThemeIDs, BlogThemeID: before.UserBlog.ThemeId.Hex()}
	operationID, inputDigest, _, err := applicationnotes.NewOperationIdentity("theme_activate", userObjectID, themeObjectID, input)
	if err != nil {
		return false
	}
	if hasPending {
		operationID, inputDigest = pending.OperationID, pending.InputDigest
	} else {
		existing, lookupErr := db.GetWorkspaceOperation(context.Background(), userObjectID, operationID)
		switch {
		case lookupErr == nil && !applicationnotes.IsTerminalOperation(existing.Status):
			operationID, inputDigest = existing.OperationID, existing.InputDigest
		case lookupErr == nil:
			input.Nonce, err = newWorkspaceIntentNonce()
			if err != nil {
				return false
			}
			operationID, inputDigest, _, err = applicationnotes.NewOperationIdentity("theme_activate", userObjectID, themeObjectID, input)
			if err != nil {
				return false
			}
		case !errors.Is(lookupErr, mongo.ErrNoDocuments):
			return false
		}
	}
	beforePayload, err := json.Marshal(before)
	if err != nil {
		return false
	}
	desired := themeActivationDesiredState{ThemeID: themeObjectID}
	desiredPayload, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	plan := db.WorkspaceMutationPlan{
		OperationID: operationID, OwnerID: userObjectID, ResourceID: themeObjectID, Kind: "theme_activate", InputDigest: inputDigest,
		BeforeState: beforePayload, DesiredState: desiredPayload, FailurePolicy: applicationnotes.FailureCompensate,
		RestoreBeforeState: func(payload []byte) error {
			var frozen themeActivationBeforeState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.UserBlog.UserId != userObjectID {
				return fmt.Errorf("theme activation before state target changed")
			}
			targetWasActive = false
			for _, theme := range frozen.ActiveThemes {
				if theme.UserId != userObjectID || theme.ThemeId.IsZero() || !theme.IsActive {
					return fmt.Errorf("theme activation before state invalid")
				}
				if theme.ThemeId == themeObjectID {
					targetWasActive = true
				}
			}
			before = frozen
			return nil
		},
		RestoreDesiredState: func(payload []byte) error {
			var frozen themeActivationDesiredState
			if err := json.Unmarshal(payload, &frozen); err != nil || frozen.ThemeID != themeObjectID {
				return fmt.Errorf("theme activation desired state target changed")
			}
			desired = frozen
			return nil
		},
	}
	plan.Steps = []db.WorkspaceMutationStep{
		{Name: "activate_theme", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return db.Themes.UpdateOneMatchedContext(ctx, targetQuery, bson.M{"$set": bson.M{"IsActive": true}})
		}, Verify: func(ctx context.Context) (bool, error) {
			var current info.Theme
			err := db.Themes.FindContext(ctx, targetQuery).One(&current)
			return err == nil && current.IsActive, err
		}, Compensate: func(ctx context.Context) error {
			return db.Themes.UpdateOneMatchedContext(ctx, targetQuery, bson.M{"$set": bson.M{"IsActive": targetWasActive}})
		}},
		{Name: "deactivate_other_themes", ReplaySafe: true, Apply: func(ctx context.Context) error {
			_, err := db.Themes.UpdateAllContext(ctx, bson.M{"UserId": userObjectID, "_id": bson.M{"$ne": themeObjectID}}, bson.M{"$set": bson.M{"IsActive": false}})
			return err
		}, Verify: func(ctx context.Context) (bool, error) {
			var active []info.Theme
			if err := db.Themes.FindContext(ctx, bson.M{"UserId": userObjectID, "IsActive": true, "_id": bson.M{"$ne": themeObjectID}}).All(&active); err != nil {
				return false, err
			}
			return len(active) == 0, nil
		}, Compensate: func(ctx context.Context) error {
			return restoreThemeActivation(ctx, userObjectID, before)
		}},
		{Name: "set_blog_theme", ReplaySafe: true, Apply: func(ctx context.Context) error {
			return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": userObjectID}, bson.M{"$set": bson.M{"ThemeId": desired.ThemeID}})
		}, Verify: func(ctx context.Context) (bool, error) {
			var current info.UserBlog
			if err := db.UserBlogs.FindIdContext(ctx, userObjectID).One(&current); err != nil {
				return false, err
			}
			return current.ThemeId == desired.ThemeID, nil
		}, Compensate: func(ctx context.Context) error {
			return db.UserBlogs.UpdateOneMatchedContext(ctx, bson.M{"_id": userObjectID}, bson.M{"$set": bson.M{"ThemeId": before.UserBlog.ThemeId}})
		}},
		{Name: "verify_theme_activation", ReplaySafe: true, Apply: func(ctx context.Context) error {
			verified, err := verifyThemeActivation(ctx, userObjectID, desired.ThemeID)
			if err != nil {
				return err
			}
			if !verified {
				return fmt.Errorf("theme activation read-back mismatch")
			}
			return nil
		}, Verify: func(ctx context.Context) (bool, error) {
			return verifyThemeActivation(ctx, userObjectID, desired.ThemeID)
		}},
	}
	mutation, err := db.RunWorkspaceMutation(context.Background(), plan)
	if err != nil || !mutation.Committed {
		if err != nil {
			Logf("activate theme failed: %v", err)
		}
		return false
	}
	verified, err := verifyThemeActivation(context.Background(), userObjectID, themeObjectID)
	return err == nil && verified
}

func deleteThemeStaged(themePath, backupPath string, removeMetadata, restoreMetadata func() error, removeTree func(string) error) error {
	if err := os.Rename(themePath, backupPath); err != nil {
		return err
	}
	if err := removeMetadata(); err != nil {
		return errors.Join(err, os.Rename(backupPath, themePath))
	}
	if err := removeTree(backupPath); err != nil {
		restoreErr := restoreMetadata()
		renameErr := os.Rename(backupPath, themePath)
		return errors.Join(err, restoreErr, renameErr)
	}
	return nil
}

// 删除主题
func (this *ThemeService) DeleteTheme(userId, themeId string) (ok bool) {
	userObjectID, userErr := parseThemeObjectID(userId)
	themeObjectID, themeErr := parseThemeObjectID(themeId)
	if userErr != nil || themeErr != nil || db.Themes == nil {
		return false
	}
	query := bson.M{"_id": themeObjectID, "UserId": userObjectID}
	theme := info.Theme{}
	if err := db.Themes.Find(query).One(&theme); err != nil || theme.IsDefault || theme.IsActive {
		return false
	}
	themePath, pathErr := this.safeThemeAbsolutePath(userId, themeId, theme.Path)
	if pathErr != nil {
		return false
	}

	if theme.Path != this.getUserThemePath2(userId, themeId) {
		return false
	}
	backupPath := filepath.Join(filepath.Dir(themePath), "."+themeId+".deleting-"+db.NewObjectID().Hex())
	removeMetadata := func() error {
		if err := db.Themes.RemoveOneMatchedContext(context.Background(), bson.M{"_id": themeObjectID, "UserId": userObjectID, "IsActive": false, "IsDefault": false}); err == nil {
			return nil
		} else {
			_, restoreErr := db.Themes.UpsertContext(context.Background(), query, theme)
			return errors.Join(err, restoreErr)
		}
	}
	restoreMetadata := func() error {
		_, err := db.Themes.UpsertContext(context.Background(), query, theme)
		return err
	}
	if err := deleteThemeStaged(themePath, backupPath, removeMetadata, restoreMetadata, os.RemoveAll); err != nil {
		Logf("delete theme failed: %v", err)
		return false
	}
	return true
}

// 公开主题, 只有管理员才有权限, 之前没公开的变成公开
func (this *ThemeService) PublicTheme(userId, themeId string) (ok bool) {
	userObjectID, userErr := parseThemeObjectID(userId)
	themeObjectID, themeErr := parseThemeObjectID(themeId)
	adminID, adminErr := configuredThemeAdminID()
	if userErr != nil || themeErr != nil || adminErr != nil || userObjectID != adminID || db.Themes == nil {
		return false
	}
	theme := info.Theme{}
	if err := db.Themes.Find(bson.M{"_id": themeObjectID, "UserId": adminID}).One(&theme); err != nil {
		return false
	}
	desired := !theme.IsDefault
	if desired {
		if _, err := this.validateThemeSource(theme, adminID); err != nil {
			return false
		}
	}
	if err := db.Themes.UpdateOneMatchedContext(context.Background(), bson.M{"_id": themeObjectID, "UserId": adminID, "IsDefault": theme.IsDefault}, bson.M{"$set": bson.M{"IsDefault": desired}}); err != nil {
		return false
	}
	var current info.Theme
	if err := db.Themes.Find(bson.M{"_id": themeObjectID, "UserId": adminID}).One(&current); err != nil {
		return false
	}
	return current.IsDefault == desired
}

// 导出主题
func (this *ThemeService) ExportTheme(userId, themeId string) (ok bool, path string) {
	theme, sourcePath, err := this.ownedThemeRoot(userId, themeId)
	if err != nil || theme.Path == "" {
		return
	}

	targetPath := this.getUserThemeExportPath(userId)
	if targetPath == "" {
		return
	}
	err = os.MkdirAll(targetPath, 0755)
	if err != nil {
		// Log(err)
		return
	}
	targetName := filepath.Join(targetPath, safeThemeArchiveName(theme.Name, themeId))
	if !pathWithin(targetPath, targetName) {
		return false, ""
	}
	// Log(sourcePath)
	// Log(targetName)
	ok = archive.Zip(sourcePath, targetName)
	if !ok {
		_ = os.Remove(targetName)
		return
	}

	return true, targetName
}

// 导入主题
// path == /llllllll/..../public/upload/.../aa.zip, 绝对路径
func (this *ThemeService) ImportTheme(userId, path string) (ok bool, msg string) {
	if !db.IsValidObjectIDHex(userId) || db.Themes == nil || path == "" {
		return false, "error"
	}
	if !isThemeUploadTempPath(userId, path) {
		return false, "error"
	}
	defer os.Remove(path)
	themeIdO := db.NewObjectID()
	themeId := themeIdO.Hex()
	targetPath := this.getUserThemePath(userId, themeId) // revel.BasePath + "/public/upload/" + userId + "/themes/" + themeId

	err := os.MkdirAll(targetPath, 0755)
	if err != nil {
		msg = "error"
		return
	}
	if ok, msg = archive.Unzip(path, targetPath); !ok {
		removeThemeTree(targetPath)
		// Log("oh no")
		return
	}

	// 主题验证
	if ok, msg = this.ValidateTheme(targetPath, "", ""); !ok {
		removeThemeTree(targetPath)
		return
	}
	// 解压成功, 那么新建之
	// 保存到数据库中
	theme, err := this.getThemeConfig(targetPath)
	if err != nil {
		removeThemeTree(targetPath)
		msg = fmt.Sprintf("%v", err)
		return false, msg
	}
	if theme.Name == "" {
		ok = false
		removeThemeTree(targetPath)
		msg = "解析错误"
		return
	}
	theme.ThemeId = themeIdO
	theme.Path = this.getUserThemePath2(userId, themeId)
	theme.CreatedTime = time.Now()
	theme.UpdatedTime = theme.CreatedTime
	theme.UserId = db.MustObjectIDFromHex(userId)

	ok = db.Insert(db.Themes, theme)
	if !ok {
		removeThemeTree(targetPath)
	}
	DeleteFile(path)
	return
}

func isThemeUploadTempPath(userId, candidate string) bool {
	root := thisGetThemeUploadTempPath(userId)
	if root == "" {
		return false
	}
	root, rootErr := filepath.Abs(root)
	path, pathErr := filepath.Abs(candidate)
	if rootErr != nil || pathErr != nil || !pathWithin(root, path) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && !info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func thisGetThemeUploadTempPath(userId string) string {
	if !db.IsValidObjectIDHex(userId) {
		return ""
	}
	return filepath.Join(revel.BasePath, "public", "upload", Digest3(userId), userId, "tmp")
}

// 升级用
// public/

func (this *ThemeService) UpgradeThemeBeta2() (ok bool) {
	adminUserId := configService.GetAdminUserId()
	this.upgradeThemeBeta2(adminUserId, defaultStyle, true)
	this.upgradeThemeBeta2(adminUserId, elegantStyle, false)
	this.upgradeThemeBeta2(adminUserId, fixedStyle, false)
	return true
}
func (this *ThemeService) upgradeThemeBeta2(userId, style string, isActive bool) (ok bool) {
	// 解压成功, 那么新建之
	// 保存到数据库中
	targetPath := this.GetDefaultThemePath(style)
	theme, err := this.getThemeConfig(revel.BasePath + "/" + targetPath)
	if err != nil || theme.Name == "" {
		ok = false
		return
	}
	themeIdO := db.NewObjectID()
	theme.ThemeId = themeIdO
	theme.Path = targetPath // public
	theme.CreatedTime = time.Now()
	theme.UpdatedTime = theme.CreatedTime
	theme.UserId = db.MustObjectIDFromHex(userId)
	theme.IsActive = isActive
	theme.IsDefault = true
	theme.Style = style
	ok = db.Insert(db.Themes, theme)
	return ok
}

// 安装主题
// 得到该主题路径
func (this *ThemeService) InstallTheme(userId, themeId string) (ok bool) {
	installerID, installerErr := parseThemeObjectID(userId)
	sourceThemeID, sourceErr := parseThemeObjectID(themeId)
	adminID, adminErr := configuredThemeAdminID()
	if installerErr != nil || sourceErr != nil || adminErr != nil || db.Themes == nil {
		return false
	}
	var theme info.Theme
	if err := db.Themes.Find(bson.M{"_id": sourceThemeID, "UserId": adminID, "IsDefault": true}).One(&theme); err != nil {
		return false
	}
	sourceThemePath, err := this.validatePublicThemeSource(theme, adminID)
	if err != nil {
		return false
	}

	// 生成新主题
	newThemeId := db.NewObjectID()
	themeId = newThemeId.Hex()
	themePath := this.getUserThemePath(userId, themeId)
	err = os.MkdirAll(themePath, 0755)
	if err != nil {
		return
	}
	// Recheck the public source immediately before copying so an unpublish race
	// cannot turn a withdrawn theme into a new installation.
	var currentSource info.Theme
	if err := db.Themes.Find(bson.M{"_id": sourceThemeID, "UserId": adminID, "IsDefault": true}).One(&currentSource); err != nil {
		removeThemeTree(themePath)
		return false
	}
	if sourceThemePath, err = this.validatePublicThemeSource(currentSource, adminID); err != nil {
		removeThemeTree(themePath)
		return false
	}
	err = CopyDir(sourceThemePath, themePath)
	if err != nil {
		removeThemeTree(themePath)
		return
	}

	// 保存到数据库中
	theme, err = this.getThemeConfig(themePath)
	if err != nil {
		removeThemeTree(themePath)
		return false
	}
	theme.ThemeId = newThemeId
	theme.Path = this.getUserThemePath2(userId, themeId)
	theme.CreatedTime = time.Now()
	theme.UpdatedTime = theme.CreatedTime
	theme.UserId = installerID

	ok = db.Insert(db.Themes, theme)
	if !ok {
		removeThemeTree(themePath)
		return false
	}

	// 激活之; a failed activation must not leave an installed orphan.
	if !this.ActiveTheme(userId, themeId) {
		removeThemeTree(themePath)
		db.Delete(db.Themes, bson.M{"_id": newThemeId, "UserId": installerID})
		return false
	}

	return ok
}

// 验证主题是否全法, 存在循环引用?
// filename, newContent 表示在修改模板时要判断模板修改时是否有错误
func (this *ThemeService) ValidateTheme(path string, filename, newContent string) (ok bool, msg string) {
	// Log("theme Path")
	// Log(path)
	// 建立一个有向图
	// 将该path下的所有文件提出, 得到文件的引用情况
	files := ListDir(path)
	LogJ(files)
	size := len(files)
	if size > 100 {
		ok = false
		msg = "tooManyFiles"
		return
	}
	/*
		111111111
		111000000
	*/
	vector := make([][]int, size)
	for i := 0; i < size; i++ {
		vector[i] = make([]int, size)
	}
	fileIndexMap := map[string]int{}   // fileName => index
	fileContent := map[string]string{} // fileName => content
	index := 0
	// 得到文件内容, 和建立索引, 每个文件都有一个index, 对应数组位置
	for _, t := range files {
		if !strings.Contains(t, ".html") {
			continue
		}
		if t != filename {
			fileBytes, err := ioutil.ReadFile(path + "/" + t)
			if err != nil {
				continue
			}

			fileIndexMap[t] = index
			// html内容
			fileStr := string(fileBytes)
			fileContent[t] = fileStr
		} else {
			fileIndexMap[t] = index
			fileContent[t] = newContent
		}
		index++
	}
	// 分析文件内容, 建立有向图
	reg, _ := regexp.Compile("{{ *template \"(.+?\\.html)\".*}}")
	for filename, content := range fileContent {
		thisIndex := fileIndexMap[filename]
		finds := reg.FindAllStringSubmatch(content, -1) // 子匹配
		LogJ(finds)
		//		Log(content)
		if finds != nil && len(finds) > 0 {
			for _, includes := range finds {
				include := includes[1]
				includeIndex, has := fileIndexMap[include]
				// Log(includeIndex)
				// Log("??")
				// Log(has)
				if has {
					vector[thisIndex][includeIndex] = 1
				}
			}
		}
	}

	LogJ(vector)
	LogJ(fileIndexMap)
	// 建立图后, 判断是否有环
	if this.hasRound(vector, index) {
		ok = false
		msg = "themeValidHasRoundInclude"
	} else {
		ok = true
	}
	return
}

// 检测有向图是否有环, DFS
func (this *ThemeService) hasRound(vector [][]int, size int) (ok bool) {
	for i := 0; i < size; i++ {
		visited := make([]int, size)
		if this.hasRoundEach(vector, i, size, visited) {
			return true
		}
	}
	return false
}

// 从每个节点出发, 判断是否有环
func (this *ThemeService) hasRoundEach(vector [][]int, index int, size int, visited []int) (ok bool) {
	if visited[index] > 0 {
		return true
	}
	visited[index] = 1
	// 遍历它的孩子
	for i := 0; i < size; i++ {
		if vector[index][i] > 0 {
			return this.hasRoundEach(vector, i, size, visited)
		}
	}
	visited[index] = 0
	return false
}
