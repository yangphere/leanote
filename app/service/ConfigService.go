package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 配置服务
// 只是全局的, 用户的配置没有
type ConfigService struct {
	adminUserId   string
	siteUrl       string
	adminUsername string
	// 全局的
	GlobalAllConfigs    map[string]interface{}
	GlobalStringConfigs map[string]string
	GlobalArrayConfigs  map[string][]string
	GlobalMapConfigs    map[string]map[string]string
	GlobalArrMapConfigs map[string][]map[string]string
	findDemoUser        func(string) (info.User, error)
}

var ErrDemoConfiguration = errors.New("demo configuration")

type DemoAccount struct {
	UserID domain.ObjectID
	Login  string
}

// appStart时 将全局的配置从数据库中得到作为全局
func (this *ConfigService) InitGlobalConfigs() bool {
	return this.InitGlobalConfigsWithError() == nil
}

// InitGlobalConfigsWithError loads a complete snapshot before publishing it.
// A Mongo read or identity failure leaves the previous in-memory snapshot
// untouched instead of silently falling back to empty/default settings.
func (this *ConfigService) InitGlobalConfigsWithError() error {
	if this == nil || db.Configs == nil || userService == nil || revel.Config == nil {
		return errors.New("configuration dependencies are not initialized")
	}
	adminUsername, _ := revel.Config.String("adminUsername")
	if adminUsername == "" {
		adminUsername = "admin"
	}
	siteURL, _ := revel.Config.String("site.url")
	userInfo := userService.GetUserInfoByAny(adminUsername)
	if userInfo.UserId.IsZero() {
		return errors.New("configured admin user does not exist")
	}
	configs := []info.Config{}
	if err := db.Configs.FindContext(context.Background(), bson.M{}).All(&configs); err != nil {
		return fmt.Errorf("load global configurations: %w", err)
	}
	all := map[string]interface{}{}
	stringsConfig := map[string]string{}
	arraysConfig := map[string][]string{}
	mapsConfig := map[string]map[string]string{}
	arrMapsConfig := map[string][]map[string]string{}
	for _, config := range configs {
		if strings.TrimSpace(config.Key) == "" {
			return errors.New("global configuration contains a blank key")
		}
		if config.IsArr {
			arraysConfig[config.Key] = append([]string(nil), config.ValueArr...)
			all[config.Key] = arraysConfig[config.Key]
		} else if config.IsMap {
			mapsConfig[config.Key] = cloneStringMap(config.ValueMap)
			all[config.Key] = mapsConfig[config.Key]
		} else if config.IsArrMap {
			arrMapsConfig[config.Key] = cloneArrMap(config.ValueArrMap)
			all[config.Key] = arrMapsConfig[config.Key]
		} else {
			stringsConfig[config.Key] = config.ValueStr
			all[config.Key] = config.ValueStr
		}
	}
	if err := validateConfiguredSecurityArrays(arraysConfig); err != nil {
		return err
	}
	if current, ok := stringsConfig["siteUrl"]; !ok || current != "" {
		stringsConfig["siteUrl"] = siteURL
		all["siteUrl"] = siteURL
	}
	this.adminUsername = adminUsername
	this.siteUrl = siteURL
	this.adminUserId = userInfo.UserId.Hex()
	this.GlobalAllConfigs = all
	this.GlobalStringConfigs = stringsConfig
	this.GlobalArrayConfigs = arraysConfig
	this.GlobalMapConfigs = mapsConfig
	this.GlobalArrMapConfigs = arrMapsConfig
	return nil
}

func validateConfiguredSecurityArrays(arrays map[string][]string) error {
	for _, key := range []string{"mongoExecutableAllowlist", "pdfExecutableAllowlist"} {
		if values, configured := arrays[key]; configured {
			if _, err := NormalizeExecutableAllowlist(values); err != nil {
				return fmt.Errorf("validate %s: %w", key, err)
			}
		}
	}
	if values, configured := arrays["feedbackRecipients"]; configured {
		if _, err := ValidateFeedbackRecipients(values); err != nil {
			return fmt.Errorf("validate feedbackRecipients: %w", err)
		}
	}
	return nil
}

func (this *ConfigService) GetSiteUrl() string {
	s := this.GetGlobalStringConfig("siteUrl")
	if s != "" {
		return s
	}

	return this.siteUrl
}
func (this *ConfigService) GetAdminUsername() string {
	return this.adminUsername
}
func (this *ConfigService) GetAdminUserId() string {
	return this.adminUserId
}

// 通用方法
func (this *ConfigService) updateGlobalConfig(userId, key string, value interface{}, isArr, isMap, isArrMap bool) bool {
	return this.updateGlobalConfigWithError(userId, key, value, isArr, isMap, isArrMap) == nil
}

func (this *ConfigService) updateGlobalConfigWithError(userId, key string, value interface{}, isArr, isMap, isArrMap bool) error {
	if strings.TrimSpace(key) == "" || (isArr && (isMap || isArrMap)) || (isMap && isArrMap) {
		return errors.New("invalid global configuration key or value kind")
	}
	if db.Configs == nil {
		return db.ErrMongoClientNotInitialized
	}
	userID := domain.ObjectID{}
	if strings.TrimSpace(userId) != "" {
		parsed, err := domain.ParseObjectID(strings.TrimSpace(userId))
		if err != nil {
			return fmt.Errorf("invalid config actor: %w", err)
		}
		userID = parsed
	}
	config, err := buildConfigValue(db.NewObjectID(), userID, key, value, isArr, isMap, isArrMap)
	if err != nil {
		return err
	}
	var existing info.Config
	findErr := db.Configs.FindContext(context.Background(), bson.M{"Key": key}).One(&existing)
	if findErr != nil && !errors.Is(findErr, mongo.ErrNoDocuments) {
		return fmt.Errorf("read config %s: %w", key, findErr)
	}
	if errors.Is(findErr, mongo.ErrNoDocuments) {
		if err := db.Configs.InsertContext(context.Background(), config); err != nil {
			return fmt.Errorf("insert config %s: %w", key, err)
		}
		existing = config
	} else {
		update := bson.M{"$set": bson.M{
			"UserId": userID, "Key": key, "IsArr": isArr, "IsMap": isMap, "IsArrMap": isArrMap,
			"UpdatedTime": config.UpdatedTime,
		}}
		setConfigValue(update["$set"].(bson.M), config)
		unset := bson.M{}
		for _, field := range []string{"ValueStr", "ValueArr", "ValueMap", "ValueArrMap"} {
			if !isConfigValueField(field, config) {
				unset[field] = ""
			}
		}
		if len(unset) > 0 {
			update["$unset"] = unset
		}
		if err := db.Configs.UpdateOneMatchedContext(context.Background(), bson.M{"_id": existing.ConfigId, "Key": key}, update); err != nil {
			return fmt.Errorf("update config %s: %w", key, err)
		}
	}
	var readBack info.Config
	if err := db.Configs.FindContext(context.Background(), bson.M{"Key": key}).One(&readBack); err != nil {
		return fmt.Errorf("read back config %s: %w", key, err)
	}
	if !configValuesEqual(readBack, config) {
		return fmt.Errorf("read back config %s did not match requested value", key)
	}
	this.applyConfigCache(config)
	return nil
}

func buildConfigValue(id, userID domain.ObjectID, key string, value interface{}, isArr, isMap, isArrMap bool) (info.Config, error) {
	config := info.Config{ConfigId: id, UserId: userID, Key: key, IsArr: isArr, IsMap: isMap, IsArrMap: isArrMap, UpdatedTime: time.Now().UTC()}
	if isArr {
		v, ok := value.([]string)
		if !ok {
			return info.Config{}, errors.New("global array configuration requires []string")
		}
		config.ValueArr = append([]string(nil), v...)
	} else if isMap {
		v, ok := value.(map[string]string)
		if !ok {
			return info.Config{}, errors.New("global map configuration requires map[string]string")
		}
		config.ValueMap = cloneStringMap(v)
	} else if isArrMap {
		v, ok := value.([]map[string]string)
		if !ok {
			return info.Config{}, errors.New("global array-map configuration requires []map[string]string")
		}
		config.ValueArrMap = cloneArrMap(v)
	} else {
		v, ok := value.(string)
		if !ok {
			return info.Config{}, errors.New("global string configuration requires string")
		}
		config.ValueStr = v
	}
	return config, nil
}

func setConfigValue(target bson.M, config info.Config) {
	target["ValueStr"] = config.ValueStr
	target["ValueArr"] = config.ValueArr
	target["ValueMap"] = config.ValueMap
	target["ValueArrMap"] = config.ValueArrMap
}

func isConfigValueField(field string, config info.Config) bool {
	switch field {
	case "ValueStr":
		return !config.IsArr && !config.IsMap && !config.IsArrMap
	case "ValueArr":
		return config.IsArr
	case "ValueMap":
		return config.IsMap
	case "ValueArrMap":
		return config.IsArrMap
	default:
		return false
	}
}

func configValuesEqual(a, b info.Config) bool {
	if a.Key != b.Key || a.IsArr != b.IsArr || a.IsMap != b.IsMap || a.IsArrMap != b.IsArrMap || a.ValueStr != b.ValueStr {
		return false
	}
	if len(a.ValueArr) != len(b.ValueArr) || len(a.ValueArrMap) != len(b.ValueArrMap) || len(a.ValueMap) != len(b.ValueMap) {
		return false
	}
	for i := range a.ValueArr {
		if a.ValueArr[i] != b.ValueArr[i] {
			return false
		}
	}
	for key, value := range a.ValueMap {
		if b.ValueMap[key] != value {
			return false
		}
	}
	for i := range a.ValueArrMap {
		for key, value := range a.ValueArrMap[i] {
			if b.ValueArrMap[i][key] != value {
				return false
			}
		}
	}
	return true
}

func (this *ConfigService) applyConfigCache(config info.Config) {
	if this.GlobalAllConfigs == nil {
		this.GlobalAllConfigs = map[string]interface{}{}
	}
	if this.GlobalStringConfigs == nil {
		this.GlobalStringConfigs = map[string]string{}
	}
	if this.GlobalArrayConfigs == nil {
		this.GlobalArrayConfigs = map[string][]string{}
	}
	if this.GlobalMapConfigs == nil {
		this.GlobalMapConfigs = map[string]map[string]string{}
	}
	if this.GlobalArrMapConfigs == nil {
		this.GlobalArrMapConfigs = map[string][]map[string]string{}
	}
	if config.IsArr {
		delete(this.GlobalStringConfigs, config.Key)
		delete(this.GlobalMapConfigs, config.Key)
		delete(this.GlobalArrMapConfigs, config.Key)
		this.GlobalArrayConfigs[config.Key] = append([]string(nil), config.ValueArr...)
		this.GlobalAllConfigs[config.Key] = this.GlobalArrayConfigs[config.Key]
	} else if config.IsMap {
		delete(this.GlobalStringConfigs, config.Key)
		delete(this.GlobalArrayConfigs, config.Key)
		delete(this.GlobalArrMapConfigs, config.Key)
		this.GlobalMapConfigs[config.Key] = cloneStringMap(config.ValueMap)
		this.GlobalAllConfigs[config.Key] = this.GlobalMapConfigs[config.Key]
	} else if config.IsArrMap {
		delete(this.GlobalStringConfigs, config.Key)
		delete(this.GlobalArrayConfigs, config.Key)
		delete(this.GlobalMapConfigs, config.Key)
		this.GlobalArrMapConfigs[config.Key] = cloneArrMap(config.ValueArrMap)
		this.GlobalAllConfigs[config.Key] = this.GlobalArrMapConfigs[config.Key]
	} else {
		delete(this.GlobalArrayConfigs, config.Key)
		delete(this.GlobalMapConfigs, config.Key)
		delete(this.GlobalArrMapConfigs, config.Key)
		this.GlobalStringConfigs[config.Key] = config.ValueStr
		this.GlobalAllConfigs[config.Key] = config.ValueStr
	}
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func cloneArrMap(value []map[string]string) []map[string]string {
	result := make([]map[string]string, len(value))
	for i, item := range value {
		result[i] = cloneStringMap(item)
	}
	return result
}

// 更新用户配置
func (this *ConfigService) UpdateGlobalStringConfig(userId, key string, value string) bool {
	return this.updateGlobalConfig(userId, key, value, false, false, false)
}
func (this *ConfigService) UpdateGlobalArrayConfig(userId, key string, value []string) bool {
	return this.updateGlobalConfig(userId, key, value, true, false, false)
}
func (this *ConfigService) UpdateGlobalMapConfig(userId, key string, value map[string]string) bool {
	return this.updateGlobalConfig(userId, key, value, false, true, false)
}
func (this *ConfigService) UpdateGlobalArrMapConfig(userId, key string, value []map[string]string) bool {
	return this.updateGlobalConfig(userId, key, value, false, false, true)
}

// 获取全局配置, 博客平台使用
func (this *ConfigService) GetGlobalStringConfig(key string) string {
	return this.GlobalStringConfigs[key]
}

// DemoAccount validates the complete demo identity configuration. demoUserId
// is authoritative, while demoUsername must resolve to that same identity;
// incomplete or inconsistent configuration never degrades into a username
// guess or an unprotected non-demo result.
func (this *ConfigService) DemoAccount() (DemoAccount, error) {
	idValue := strings.TrimSpace(this.GetGlobalStringConfig("demoUserId"))
	login := strings.ToLower(strings.TrimSpace(this.GetGlobalStringConfig("demoUsername")))
	if idValue == "" || login == "" {
		return DemoAccount{}, fmt.Errorf("%w: missing identity", ErrDemoConfiguration)
	}
	id, err := domain.ParseObjectID(idValue)
	if err != nil || id.IsZero() {
		return DemoAccount{}, fmt.Errorf("%w: invalid user id", ErrDemoConfiguration)
	}
	finder := this.findDemoUser
	if finder == nil {
		if userService == nil {
			return DemoAccount{}, errors.New("demo user service is not initialized")
		}
		finder = userService.FindUserInfoByName
	}
	user, err := finder(login)
	if err != nil {
		return DemoAccount{}, fmt.Errorf("resolve demo username: %w", err)
	}
	if user.UserId.IsZero() || user.UserId != id {
		return DemoAccount{}, fmt.Errorf("%w: username identity mismatch", ErrDemoConfiguration)
	}
	return DemoAccount{UserID: id, Login: login}, nil
}

// IsDemoUser compares an authenticated principal only after the shared demo
// configuration has passed DemoAccount validation.
func (this *ConfigService) IsDemoUser(userID string) (bool, error) {
	account, err := this.DemoAccount()
	if err != nil {
		return false, err
	}
	id, err := domain.ParseObjectID(strings.TrimSpace(userID))
	if err != nil || id.IsZero() {
		return false, fmt.Errorf("validate demo principal: invalid user id")
	}
	return id == account.UserID, nil
}
func (this *ConfigService) GetGlobalArrayConfig(key string) []string {
	arr := this.GlobalArrayConfigs[key]
	if arr == nil {
		return []string{}
	}
	return append([]string(nil), arr...)
}
func (this *ConfigService) GetGlobalMapConfig(key string) map[string]string {
	m := this.GlobalMapConfigs[key]
	if m == nil {
		return map[string]string{}
	}
	return cloneStringMap(m)
}
func (this *ConfigService) GetGlobalArrMapConfig(key string) []map[string]string {
	m := this.GlobalArrMapConfigs[key]
	if m == nil {
		return []map[string]string{}
	}
	return cloneArrMap(m)
}

func (this *ConfigService) IsOpenRegister() bool {
	return this.GetGlobalStringConfig("openRegister") != ""
}

// -------
// 修改共享笔记的配置
func (this *ConfigService) UpdateShareNoteConfig(registerSharedUserId string,
	registerSharedNotebookPerms, registerSharedNotePerms []int,
	registerSharedNotebookIds, registerSharedNoteIds, registerCopyNoteIds []string) (ok bool, msg string) {

	defer func() {
		if err := recover(); err != nil {
			ok = false
			msg = fmt.Sprint(err)
		}
	}()

	// 用户是否存在?
	if registerSharedUserId == "" {
		ok = true
		msg = "share userId is blank, So it share nothing to register"
		this.UpdateGlobalStringConfig(this.adminUserId, "registerSharedUserId", "")
		return
	} else {
		user := userService.GetUserInfo(registerSharedUserId)
		if user.UserId.IsZero() {
			ok = false
			msg = "no such user: " + registerSharedUserId
			return
		} else {
			this.UpdateGlobalStringConfig(this.adminUserId, "registerSharedUserId", registerSharedUserId)
		}
	}

	notebooks := []map[string]string{}
	// 共享笔记本
	if len(registerSharedNotebookIds) > 0 {
		for i := 0; i < len(registerSharedNotebookIds); i++ {
			// 判断笔记本是否存在
			notebookId := registerSharedNotebookIds[i]
			if notebookId == "" {
				continue
			}
			notebook := notebookService.GetNotebook(notebookId, registerSharedUserId)
			if notebook.NotebookId.IsZero() {
				ok = false
				msg = "The user has no such notebook: " + notebookId
				return
			} else {
				perm := "0"
				if registerSharedNotebookPerms[i] == 1 {
					perm = "1"
				}
				notebooks = append(notebooks, map[string]string{"notebookId": notebookId, "perm": perm})
			}
		}
	}
	this.UpdateGlobalArrMapConfig(this.adminUserId, "registerSharedNotebooks", notebooks)

	notes := []map[string]string{}
	// 共享笔记
	if len(registerSharedNoteIds) > 0 {
		for i := 0; i < len(registerSharedNoteIds); i++ {
			// 判断笔记本是否存在
			noteId := registerSharedNoteIds[i]
			if noteId == "" {
				continue
			}
			note := noteService.GetNote(noteId, registerSharedUserId)
			if note.NoteId.IsZero() {
				ok = false
				msg = "The user has no such note: " + noteId
				return
			} else {
				perm := "0"
				if registerSharedNotePerms[i] == 1 {
					perm = "1"
				}
				notes = append(notes, map[string]string{"noteId": noteId, "perm": perm})
			}
		}
	}
	this.UpdateGlobalArrMapConfig(this.adminUserId, "registerSharedNotes", notes)

	// 复制
	noteIds := []string{}
	if len(registerCopyNoteIds) > 0 {
		for i := 0; i < len(registerCopyNoteIds); i++ {
			// 判断笔记本是否存在
			noteId := registerCopyNoteIds[i]
			if noteId == "" {
				continue
			}
			note := noteService.GetNote(noteId, registerSharedUserId)
			if note.NoteId.IsZero() {
				ok = false
				msg = "The user has no such note: " + noteId
				return
			} else {
				noteIds = append(noteIds, noteId)
			}
		}
	}
	this.UpdateGlobalArrayConfig(this.adminUserId, "registerCopyNoteIds", noteIds)

	ok = true
	return
}

// 添加备份
func (this *ConfigService) AddBackup(path, remark string) bool {
	return this.addBackup(path, remark, "confirmed")
}

func (this *ConfigService) addBackup(path, remark, state string) (ok bool) {
	return this.addBackupExcluding(path, remark, state, nil, nil, "")
}

func (this *ConfigService) addBackupExcluding(path, remark, state string, protectedPaths map[string]struct{}, identity *ConfiguredDatabaseIdentity, identityDigest string) (ok bool) {
	if state != "confirmed" && state != "protected" {
		return false
	}
	root := filepath.Join(revel.BasePath, "mongodb_backup")
	canonical, err := ValidateContainedPath(root, path)
	if err != nil {
		return false
	}
	// A failed registration or retention pass must not leave an unregistered
	// dump behind for a later cleanup job to mistake as a usable backup.
	_, existingPathErr := os.Lstat(canonical)
	cleanup := existingPathErr != nil
	defer func() {
		if !ok && cleanup {
			_ = os.RemoveAll(canonical)
		}
	}()
	backups := this.GetGlobalArrMapConfig("backups") // [{}, {}]
	backups = cloneArrMap(backups)
	n := time.Now().Unix()
	nstr := fmt.Sprintf("%v", n)
	entry := map[string]string{"createdTime": nstr, "path": canonical, "remark": remark, "state": state}
	if identity != nil {
		entry["databaseScheme"] = identity.Scheme
		entry["databaseClusterHost"] = identity.ClusterHost
		entry["databaseClusterPort"] = identity.ClusterPort
		entry["databaseName"] = identity.DatabaseName
		entry["databaseAuthSource"] = identity.AuthSource
		entry["databaseTLSMode"] = identity.TLSMode
	}
	if identityDigest != "" {
		entry["databaseIdentityDigest"] = identityDigest
	}
	backups = append(backups, entry)
	newSize, err := backupDirectorySize(canonical)
	if err != nil {
		return false
	}
	retainedBytes := int64(0)
	for _, candidate := range backups {
		if candidate["path"] == canonical {
			retainedBytes += newSize
			continue
		}
		if candidate["state"] == "orphan" {
			continue
		}
		candidateSize, sizeErr := backupDirectorySize(candidate["path"])
		if sizeErr != nil {
			candidate["state"] = "orphan"
			continue
		}
		retainedBytes += candidateSize
	}
	// Retention is evaluated from confirmed, non-protected metadata. An
	// over-budget or orphaned entry remains visible for operator repair.
	for len(backups) > DefaultBackupLimits.MaxCopies || retainedBytes > DefaultBackupLimits.MaxBytes {
		removed := false
		for index, candidate := range backups {
			if candidate["state"] == "protected" || candidate["state"] == "running" || candidate["path"] == canonical {
				continue
			}
			if _, protected := protectedPaths[candidate["path"]]; protected {
				continue
			}
			if _, err := ValidateContainedPath(root, candidate["path"]); err != nil {
				candidate["state"] = "orphan"
				continue
			}
			candidateSize, sizeErr := backupDirectorySize(candidate["path"])
			if sizeErr != nil {
				candidate["state"] = "orphan"
				continue
			}
			if removeErr := os.RemoveAll(candidate["path"]); removeErr != nil {
				return false
			}
			backups = append(backups[:index], backups[index+1:]...)
			retainedBytes -= candidateSize
			removed = true
			break
		}
		if !removed {
			return false
		}
	}
	if !this.UpdateGlobalArrMapConfig(this.adminUserId, "backups", backups) {
		return false
	}
	cleanup = false
	return true
}

func (this *ConfigService) getBackupDirname() string {
	n := time.Now()
	y, m, d := n.Date()
	return strconv.Itoa(y) + "_" + m.String() + "_" + strconv.Itoa(d) + "_" + fmt.Sprintf("%v", n.Unix())
}

func (this *ConfigService) validMongoExecutable(key string) (string, error) {
	path := strings.TrimSpace(this.GetGlobalStringConfig(key))
	allowlist := this.GetGlobalArrayConfig("mongoExecutableAllowlist")
	if len(allowlist) == 0 {
		allowlist = this.GetGlobalArrayConfig("backupExecutableAllowlist")
	}
	return ValidateExecutable(path, allowlist)
}

func configuredDatabaseIdentity() (ConfiguredDatabaseIdentity, string, error) {
	if revel.Config == nil {
		return ConfiguredDatabaseIdentity{}, "", errors.New("database configuration is unavailable")
	}
	host, _ := revel.Config.String("db.host")
	port, _ := revel.Config.String("db.port")
	databaseName, _ := revel.Config.String("db.dbname")
	authSource, _ := revel.Config.String("db.authSource")
	tlsMode, _ := revel.Config.String("db.tls")
	identity := ConfiguredDatabaseIdentity{
		Scheme:       "mongodb",
		ClusterHost:  host,
		ClusterPort:  port,
		DatabaseName: databaseName,
		AuthSource:   authSource,
		TLSMode:      tlsMode,
	}
	digest, err := identity.Digest()
	if err != nil {
		return ConfiguredDatabaseIdentity{}, "", err
	}
	return identity, digest, nil
}

func backupDirectorySize(root string) (int64, error) {
	rootInfo, err := os.Stat(root)
	if err != nil || !rootInfo.IsDir() {
		return 0, fmt.Errorf("backup directory is unavailable")
	}
	var total int64
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("backup tree contains a non-regular file")
		}
		if info.Size() > math.MaxInt64-total {
			return fmt.Errorf("backup size overflows int64")
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (this *ConfigService) Backup(remark string) (ok bool, msg string) {
	return this.backupWithState(remark, "confirmed")
}

// 还原
func (this *ConfigService) Restore(createdTime string) (ok bool, msg string) {
	backups := this.GetGlobalArrMapConfig("backups") // [{}, {}]
	var i int
	var backup map[string]string
	for i, backup = range backups {
		if backup["createdTime"] == createdTime {
			break
		}
	}
	if i == len(backups) {
		return false, "Backup Not Found"
	}

	if revel.BasePath == "" || revel.Config == nil {
		return false, "restore configuration is unavailable"
	}
	dbname, _ := revel.Config.String("db.dbname")
	host, _ := revel.Config.String("db.host")
	port, _ := revel.Config.String("db.port")
	username, _ := revel.Config.String("db.username")
	password, _ := revel.Config.String("db.password")
	_, identityDigest, identityErr := configuredDatabaseIdentity()
	if identityErr != nil {
		return false, "database identity unavailable"
	}
	root := filepath.Join(revel.BasePath, "mongodb_backup")
	registered, err := ValidateContainedPath(root, backup["path"])
	if err != nil {
		return false, "backup path validation failed"
	}
	path, err := ValidateContainedPath(root, filepath.Join(registered, dbname))
	if err != nil {
		return false, "backup database path validation failed"
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		return false, "registered backup data is unavailable"
	}
	storedIdentity := ConfiguredDatabaseIdentity{
		Scheme:       backup["databaseScheme"],
		ClusterHost:  backup["databaseClusterHost"],
		ClusterPort:  backup["databaseClusterPort"],
		DatabaseName: backup["databaseName"],
		AuthSource:   backup["databaseAuthSource"],
		TLSMode:      backup["databaseTLSMode"],
	}
	storedDigest, digestErr := storedIdentity.Digest()
	if digestErr != nil || backup["databaseIdentityDigest"] == "" || storedDigest != backup["databaseIdentityDigest"] || backup["databaseIdentityDigest"] != identityDigest {
		return false, "backup database identity does not match the configured database"
	}
	if _, _, scanErr := StableBackupPaths(path, DefaultBackupLimits); scanErr != nil {
		return false, "backup manifest validation failed"
	}
	if protected, protectMsg := this.backupWithStateExcluding("Auto backup when restore from "+backup["createdTime"], "protected", map[string]struct{}{registered: {}}); !protected {
		return false, "protect current database before restore: " + protectMsg
	}
	executable, err := this.validMongoExecutable("mongorestorePath")
	if err != nil {
		return false, "mongorestore executable validation failed"
	}
	args := BuildMongoRestoreArgs(executable, host, port, dbname, path, username)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if password != "" {
		cmd.Stdin = strings.NewReader(password + "\n")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = output
		if ctx.Err() != nil {
			return false, "mongorestore timed out"
		}
		return false, "mongorestore failed"
	}
	return true, ""
}

func (this *ConfigService) backupWithState(remark, state string) (bool, string) {
	return this.backupWithStateExcluding(remark, state, nil)
}

func (this *ConfigService) backupWithStateExcluding(remark, state string, protectedPaths map[string]struct{}) (bool, string) {
	if revel.BasePath == "" || revel.Config == nil {
		return false, "backup configuration is unavailable"
	}
	executable, err := this.validMongoExecutable("mongodumpPath")
	if err != nil {
		return false, "mongodump executable validation failed"
	}
	dbname, _ := revel.Config.String("db.dbname")
	host, _ := revel.Config.String("db.host")
	port, _ := revel.Config.String("db.port")
	username, _ := revel.Config.String("db.username")
	password, _ := revel.Config.String("db.password")
	identity, identityDigest, identityErr := configuredDatabaseIdentity()
	if identityErr != nil {
		return false, "database identity unavailable"
	}
	if strings.TrimSpace(dbname) == "" || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return false, "database identity is incomplete"
	}
	root := filepath.Join(revel.BasePath, "mongodb_backup")
	if err := os.MkdirAll(root, 0700); err != nil {
		return false, "create backup root failed"
	}
	dir := filepath.Join(root, this.getBackupDirname())
	if _, err := ValidateContainedPath(root, dir); err != nil {
		return false, "backup path validation failed"
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return false, "create backup directory failed"
	}
	registered := false
	defer func() {
		if !registered {
			_ = os.RemoveAll(dir)
		}
	}()
	args := BuildMongoDumpArgs(executable, host, port, dbname, dir, username)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if password != "" {
		cmd.Stdin = strings.NewReader(password + "\n")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = output
		_ = os.RemoveAll(dir)
		if ctx.Err() != nil {
			return false, "mongodump timed out"
		}
		return false, "mongodump failed"
	}
	paths, total, err := StableBackupPaths(dir, DefaultBackupLimits)
	if err != nil || len(paths) == 0 {
		_ = os.RemoveAll(dir)
		if err == nil {
			err = errors.New("backup produced no regular files")
		}
		return false, "backup manifest validation failed"
	}
	if total > DefaultBackupLimits.MaxBytes {
		_ = os.RemoveAll(dir)
		return false, "backup retention byte budget exceeded"
	}
	if !this.addBackupExcluding(dir, remark, state, protectedPaths, &identity, identityDigest) {
		return false, "backup metadata write failed"
	}
	registered = true
	return true, ""
}
func (this *ConfigService) DeleteBackup(createdTime string) (bool, string) {
	backups := this.GetGlobalArrMapConfig("backups") // [{}, {}]
	var i int
	var backup map[string]string
	for i, backup = range backups {
		if backup["createdTime"] == createdTime {
			break
		}
	}
	if i == len(backups) {
		return false, "Backup Not Found"
	}

	root := filepath.Join(revel.BasePath, "mongodb_backup")
	path, err := ValidateContainedPath(root, backups[i]["path"])
	if err != nil {
		return false, "backup path validation failed"
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, "backup directory is missing"
		}
		return false, "backup directory cannot be inspected"
	}
	if !info.IsDir() {
		return false, "registered backup path is not a directory"
	}
	// Delete only the registered directory after containment and symlink checks.
	err = os.RemoveAll(path)
	if err != nil {
		return false, fmt.Sprintf("%v", err)
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return false, "backup directory still exists after deletion"
	} else if !os.IsNotExist(statErr) {
		return false, "backup directory deletion could not be verified"
	}

	// 删除之
	backups = append(backups[0:i], backups[i+1:]...)

	ok := this.UpdateGlobalArrMapConfig(this.adminUserId, "backups", backups)
	if !ok {
		return false, "backup metadata delete failed after filesystem deletion"
	}
	return true, ""
}

func (this *ConfigService) UpdateBackupRemark(createdTime, remark string) (bool, string) {
	backups := this.GetGlobalArrMapConfig("backups") // [{}, {}]
	var i int
	var backup map[string]string
	for i, backup = range backups {
		if backup["createdTime"] == createdTime {
			break
		}
	}
	if i == len(backups) {
		return false, "Backup Not Found"
	}
	backup["remark"] = remark

	ok := this.UpdateGlobalArrMapConfig(this.adminUserId, "backups", backups)
	return ok, ""
}

// 得到备份
func (this *ConfigService) GetBackup(createdTime string) (map[string]string, bool) {
	backups := this.GetGlobalArrMapConfig("backups") // [{}, {}]
	var i int
	var backup map[string]string
	for i, backup = range backups {
		if backup["createdTime"] == createdTime {
			break
		}
	}
	if i == len(backups) {
		return map[string]string{}, false
	}
	return backup, true
}

// --------------
// sub domain
var defaultDomain string
var schema = "http://"
var port string

func init() {
	revel.OnAppStart(func() {
		/*
			不用配置的, 因为最终通过命令可以改, 而且有的使用nginx代理
			port  = strconv.Itoa(revel.HttpPort)
			if port != "80" {
				port = ":" + port
			} else {
				port = "";
			}
		*/

		siteUrl, _ := revel.Config.String("site.url") // 已包含:9000, http, 去掉成 leanote.com
		if strings.HasPrefix(siteUrl, "http://") {
			defaultDomain = siteUrl[len("http://"):]
		} else if strings.HasPrefix(siteUrl, "https://") {
			defaultDomain = siteUrl[len("https://"):]
			schema = "https://"
		}

		// port localhost:9000
		ports := strings.Split(defaultDomain, ":")
		if len(ports) == 2 {
			port = ports[1]
		}
		if port == "80" {
			port = ""
		} else {
			port = ":" + port
		}
	})
}

func (this *ConfigService) GetSchema() string {
	return schema
}

// 默认
func (this *ConfigService) GetDefaultDomain() string {
	return defaultDomain
}

// note
func (this *ConfigService) GetNoteDomain() string {
	return "/note"
}
func (this *ConfigService) GetNoteUrl() string {
	return this.GetNoteDomain()
}

// blog
func (this *ConfigService) GetBlogDomain() string {
	return "/blog"
}
func (this *ConfigService) GetBlogUrl() string {
	return this.GetBlogDomain()
}

// lea
func (this *ConfigService) GetLeaDomain() string {
	return "/lea"
}
func (this *ConfigService) GetLeaUrl() string {
	return schema + this.GetLeaDomain()
}

func (this *ConfigService) GetUserUrl(domain string) string {
	return schema + domain + port
}
func (this *ConfigService) GetUserSubUrl(subDomain string) string {
	return schema + subDomain + "." + this.GetDefaultDomain()
}

// 是否允许自定义域名
func (this *ConfigService) AllowCustomDomain() bool {
	return configService.GetGlobalStringConfig("allowCustomDomain") != ""
}

// 是否是好的自定义域名
func (this *ConfigService) IsGoodCustomDomain(domain string) bool {
	blacks := this.GetGlobalArrayConfig("blackCustomDomains")
	for _, black := range blacks {
		if strings.Contains(domain, black) {
			return false
		}
	}
	return true
}
func (this *ConfigService) IsGoodSubDomain(domain string) bool {
	blacks := this.GetGlobalArrayConfig("blackSubDomains")
	LogJ(blacks)
	for _, black := range blacks {
		if domain == black {
			return false
		}
	}
	return true
}

// 上传大小
func (this *ConfigService) GetUploadSize(key string) float64 {
	f, _ := strconv.ParseFloat(this.GetGlobalStringConfig(key), 64)
	return f
}

// GetUploadLimitBytes is the fail-closed upload configuration boundary used
// by content commands.  The legacy float-only getter remains for display
// compatibility, but upload paths must not turn malformed values into an
// implicit 1000 MB allowance.
func (this *ConfigService) GetUploadLimitBytes(key string) (int64, error) {
	value := strings.TrimSpace(this.GetGlobalStringConfig(key))
	megabytes, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(megabytes) || math.IsInf(megabytes, 0) || megabytes <= 0 {
		return 0, fmt.Errorf("invalid upload size configuration for %s", key)
	}
	const bytesPerMegabyte = 1024 * 1024
	if megabytes > float64(math.MaxInt64)/bytesPerMegabyte {
		return 0, fmt.Errorf("upload size configuration overflows for %s", key)
	}
	limit := int64(megabytes * bytesPerMegabyte)
	if limit <= 0 {
		return 0, fmt.Errorf("upload size configuration is below one byte for %s", key)
	}
	return limit, nil
}
func (this *ConfigService) GetInt64(key string) int64 {
	f, _ := strconv.ParseInt(this.GetGlobalStringConfig(key), 10, 64)
	return f
}
func (this *ConfigService) GetInt32(key string) int32 {
	f, _ := strconv.ParseInt(this.GetGlobalStringConfig(key), 10, 32)
	return int32(f)
}
func (this *ConfigService) GetUploadSizeLimit() map[string]float64 {
	return map[string]float64{
		"uploadImageSize":    this.GetUploadSize("uploadImageSize"),
		"uploadBlogLogoSize": this.GetUploadSize("uploadBlogLogoSize"),
		"uploadAttachSize":   this.GetUploadSize("uploadAttachSize"),
		"uploadAvatarSize":   this.GetUploadSize("uploadAvatarSize"),
	}
}

// 为用户得到全局的配置
// NoteController调用
func (this *ConfigService) GetGlobalConfigForUser() map[string]interface{} {
	uploadSizeConfigs := this.GetUploadSizeLimit()
	config := map[string]interface{}{}
	for k, v := range uploadSizeConfigs {
		config[k] = v
	}
	return config
}

// 主页是否是管理员的博客页
func (this *ConfigService) HomePageIsAdminsBlog() bool {
	return this.GetGlobalStringConfig("homePage") == ""
}

func (this *ConfigService) GetVersion() string {
	return BuildVersion
}
