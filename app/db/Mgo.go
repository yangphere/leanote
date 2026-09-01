package db

import (
	"context"
	"errors"
	"strings"

	"github.com/revel/revel"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Init mgo and the common DAO

// 各个表的Collection对象
var Notebooks *Collection
var Notes *Collection
var NoteContents *Collection
var NoteContentHistories *Collection

var ShareNotes *Collection
var ShareNotebooks *Collection
var HasShareNotes *Collection

var Blogs *Collection
var Users *Collection
var Groups *Collection
var GroupUsers *Collection

var Tags *Collection
var NoteTags *Collection
var TagCounts *Collection

var UserBlogs *Collection

var Tokens *Collection

var Suggestions *Collection

// Album & file(image)
var Albums *Collection
var Files *Collection
var Attachs *Collection

var NoteImages *Collection
var Configs *Collection
var EmailLogs *Collection

// blog
var BlogLikes *Collection
var BlogComments *Collection
var Reports *Collection
var BlogSingles *Collection
var Themes *Collection

// session
var Sessions *Collection

// 初始化时连接数据库
func Init(url, dbname string) {
	if err := InitWithError(url, dbname); err != nil {
		panic(err)
	}
}

// InitFromRevelConfigForDevelopment keeps the legacy Revel test harness
// usable without making those aliases part of the production entry point.
// Production callers must use the explicit db.urlEnv contract instead.
func InitFromRevelConfigForDevelopment() {
	if revel.Config == nil {
		panic("development database configuration is unavailable")
	}
	dbURL, _ := revel.Config.String("db.url")
	if dbURL == "" {
		dbURL, _ = revel.Config.String("db.urlEnv")
	}
	dbName, _ := revel.Config.String("db.dbname")
	if dbURL == "" {
		host, _ := revel.Config.String("db.host")
		port, _ := revel.Config.String("db.port")
		user, _ := revel.Config.String("db.username")
		pass, _ := revel.Config.String("db.password")
		credentials := ""
		if user != "" && pass != "" {
			credentials = user + ":" + pass + "@"
		}
		dbURL = "mongodb://" + credentials + host + ":" + port + "/" + dbName
	}
	if strings.TrimSpace(dbURL) == "" || strings.TrimSpace(dbName) == "" {
		panic("development database configuration is incomplete")
	}
	Init(strings.TrimSpace(dbURL), strings.TrimSpace(dbName))
}

// InitWithError initializes MongoDB and collections, returning connection
// failures to callers that need to expose readiness through /healthz.
func InitWithError(url, dbname string) error {
	if url == "" {
		return errors.New("mongo URL is required")
	}
	if dbname == "" {
		return errors.New("mongo database name is required")
	}
	if err := dialMongo(url); err != nil {
		client = nil
		database = nil
		return err
	}

	database = client.Database(dbname)

	// notebook
	Notebooks = wrapCollection(database.Collection("notebooks"))

	// notes
	Notes = wrapCollection(database.Collection("notes"))

	// noteContents
	NoteContents = wrapCollection(database.Collection("note_contents"))
	NoteContentHistories = wrapCollection(database.Collection("note_content_histories"))

	// share
	ShareNotes = wrapCollection(database.Collection("share_notes"))
	ShareNotebooks = wrapCollection(database.Collection("share_notebooks"))
	HasShareNotes = wrapCollection(database.Collection("has_share_notes"))

	// user
	Users = wrapCollection(database.Collection("users"))
	// group
	Groups = wrapCollection(database.Collection("groups"))
	GroupUsers = wrapCollection(database.Collection("group_users"))

	// blog
	Blogs = wrapCollection(database.Collection("blogs"))

	// tag
	Tags = wrapCollection(database.Collection("tags"))
	NoteTags = wrapCollection(database.Collection("note_tags"))
	TagCounts = wrapCollection(database.Collection("tag_count"))

	// blog
	UserBlogs = wrapCollection(database.Collection("user_blogs"))
	BlogSingles = wrapCollection(database.Collection("blog_singles"))
	Themes = wrapCollection(database.Collection("themes"))

	// find password
	Tokens = wrapCollection(database.Collection("tokens"))

	// Suggestion
	Suggestions = wrapCollection(database.Collection("suggestions"))

	// Album & file
	Albums = wrapCollection(database.Collection("albums"))
	Files = wrapCollection(database.Collection("files"))
	Attachs = wrapCollection(database.Collection("attachs"))

	NoteImages = wrapCollection(database.Collection("note_images"))

	Configs = wrapCollection(database.Collection("configs"))
	EmailLogs = wrapCollection(database.Collection("email_logs"))

	// 社交
	BlogLikes = wrapCollection(database.Collection("blog_likes"))
	BlogComments = wrapCollection(database.Collection("blog_comments"))

	// 举报
	Reports = wrapCollection(database.Collection("reports"))

	// session
	Sessions = wrapCollection(database.Collection("sessions"))
	return nil
}

func close() {
	if client != nil {
		_ = client.Disconnect(context.Background())
	}
}

// FindInCollection runs a find against an arbitrary database. It exists for
// test-support paths only (e2e run markers) and must not be used for
// business data access.
func FindInCollection(database, collection string, filter, result interface{}) error {
	if client == nil {
		return errors.New("mongo client is not initialized")
	}
	ctx, cancel := operationContext()
	defer cancel()
	cursor, err := client.Database(database).Collection(collection).Find(ctx, filter)
	if err != nil {
		Logf("mongo find failed on %s.%s category=%s", database, collection, classifyError(err))
		return err
	}
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			Logf("mongo cursor close failed on %s.%s category=%s", database, collection, classifyError(cerr))
		}
	}()
	return cursor.All(ctx, result)
}

// common DAO
// 公用方法

//----------------------

func Insert(collection *Collection, i interface{}) bool {
	return Err(collection.Insert(i))
}

//----------------------

// 适合一条记录全部更新
func Update(collection *Collection, query interface{}, i interface{}) bool {
	return Err(collection.Update(query, i))
}
func Upsert(collection *Collection, query interface{}, i interface{}) bool {
	_, err := collection.Upsert(query, i)
	return Err(err)
}
func UpdateAll(collection *Collection, query interface{}, i interface{}) bool {
	_, err := collection.UpdateAll(query, i)
	return Err(err)
}
func UpdateByIdAndUserId(collection *Collection, id, userId string, i interface{}) bool {
	return Err(collection.Update(GetIdAndUserIdQ(id, userId), i))
}

func UpdateByIdAndUserId2(collection *Collection, id, userId ObjectID, i interface{}) bool {
	return Err(collection.Update(GetIdAndUserIdBsonQ(id, userId), i))
}
func UpdateByIdAndUserIdField(collection *Collection, id, userId, field string, value interface{}) bool {
	return UpdateByIdAndUserId(collection, id, userId, bson.M{"$set": bson.M{field: value}})
}
func UpdateByIdAndUserIdMap(collection *Collection, id, userId string, v bson.M) bool {
	return UpdateByIdAndUserId(collection, id, userId, bson.M{"$set": v})
}

func UpdateByIdAndUserIdField2(collection *Collection, id, userId ObjectID, field string, value interface{}) bool {
	return UpdateByIdAndUserId2(collection, id, userId, bson.M{"$set": bson.M{field: value}})
}
func UpdateByIdAndUserIdMap2(collection *Collection, id, userId ObjectID, v bson.M) bool {
	return UpdateByIdAndUserId2(collection, id, userId, bson.M{"$set": v})
}

func UpdateByQField(collection *Collection, q interface{}, field string, value interface{}) bool {
	_, err := collection.UpdateAll(q, bson.M{"$set": bson.M{field: value}})
	return Err(err)
}
func UpdateByQI(collection *Collection, q interface{}, v interface{}) bool {
	_, err := collection.UpdateAll(q, bson.M{"$set": v})
	return Err(err)
}

// 查询条件和值
func UpdateByQMap(collection *Collection, q interface{}, v interface{}) bool {
	_, err := collection.UpdateAll(q, bson.M{"$set": v})
	return Err(err)
}

//------------------------

// 删除一条
func Delete(collection *Collection, q interface{}) bool {
	return Err(collection.Remove(q))
}
func DeleteByIdAndUserId(collection *Collection, id, userId string) bool {
	return Err(collection.Remove(GetIdAndUserIdQ(id, userId)))
}
func DeleteByIdAndUserId2(collection *Collection, id, userId ObjectID) bool {
	return Err(collection.Remove(GetIdAndUserIdBsonQ(id, userId)))
}

// 删除所有
func DeleteAllByIdAndUserId(collection *Collection, id, userId string) bool {
	_, err := collection.RemoveAll(GetIdAndUserIdQ(id, userId))
	return Err(err)
}
func DeleteAllByIdAndUserId2(collection *Collection, id, userId ObjectID) bool {
	_, err := collection.RemoveAll(GetIdAndUserIdBsonQ(id, userId))
	return Err(err)
}

func DeleteAll(collection *Collection, q interface{}) bool {
	_, err := collection.RemoveAll(q)
	return Err(err)
}

//-------------------------

func Get(collection *Collection, id string, i interface{}) {
	collection.FindId(MustObjectIDFromHex(id)).One(i)
}
func Get2(collection *Collection, id ObjectID, i interface{}) {
	collection.FindId(id).One(i)
}

func GetByQ(collection *Collection, q interface{}, i interface{}) {
	collection.Find(q).One(i)
}
func ListByQ(collection *Collection, q interface{}, i interface{}) {
	collection.Find(q).All(i)
}

func ListByQLimit(collection *Collection, q interface{}, i interface{}, limit int) {
	collection.Find(q).Limit(limit).All(i)
}

// 查询某些字段, q是查询条件, fields是字段名列表
func GetByQWithFields(collection *Collection, q bson.M, fields []string, i interface{}) {
	selector := make(bson.M, len(fields))
	for _, field := range fields {
		selector[field] = true
	}
	collection.Find(q).Select(selector).One(i)
}

// 查询某些字段, q是查询条件, fields是字段名列表
func ListByQWithFields(collection *Collection, q bson.M, fields []string, i interface{}) {
	selector := make(bson.M, len(fields))
	for _, field := range fields {
		selector[field] = true
	}
	collection.Find(q).Select(selector).All(i)
}
func GetByIdAndUserId(collection *Collection, id, userId string, i interface{}) {
	collection.Find(GetIdAndUserIdQ(id, userId)).One(i)
}
func GetByIdAndUserId2(collection *Collection, id, userId ObjectID, i interface{}) {
	collection.Find(GetIdAndUserIdBsonQ(id, userId)).One(i)
}

// 按field去重
func Distinct(collection *Collection, q bson.M, field string, i interface{}) {
	collection.Find(q).Distinct(field, i)
}

//----------------------

func Count(collection *Collection, q interface{}) int {
	cnt, err := collection.Find(q).Count()
	if err != nil {
		Err(err)
	}
	return cnt
}

func Has(collection *Collection, q interface{}) bool {
	if Count(collection, q) > 0 {
		return true
	}
	return false
}

//-----------------

// 得到主键和userId的复合查询条件
func GetIdAndUserIdQ(id, userId string) bson.M {
	return bson.M{"_id": MustObjectIDFromHex(id), "UserId": MustObjectIDFromHex(userId)}
}
func GetIdAndUserIdBsonQ(id, userId ObjectID) bson.M {
	return bson.M{"_id": id, "UserId": userId}
}

// DB处理错误
// Err maps a driver error onto the legacy bool contract: nil and no-documents
// ("not found") are successes; every other error fails. Failures are logged
// with collection and operation by the Collection methods themselves.
func Err(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true
	}
	return false
}
