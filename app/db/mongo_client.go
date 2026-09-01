package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/revel/revel"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// 连接与操作超时。默认值与 conf/app.conf-default 中的 db.connectTimeoutMs、
// db.operationTimeoutMs 保持一致，配置非法或缺失时回落到默认值。
var (
	client           *mongo.Client
	database         *mongo.Database
	connectTimeout   = 10 * time.Second
	operationTimeout = 15 * time.Second
)

func loadTimeoutConfig() {
	// revel.Config is unavailable when tests call Init directly with an
	// explicit URL; fall back to the defaults in that case.
	if revel.Config == nil {
		return
	}
	connectTimeout = timeoutConfigValue("db.connectTimeoutMs", connectTimeout)
	operationTimeout = timeoutConfigValue("db.operationTimeoutMs", operationTimeout)
}

// timeoutConfigValue reads a millisecond duration from config: a missing or
// blank key keeps the default; an invalid value is a startup fatal, on par
// with a failed connection (design §4).
func timeoutConfigValue(key string, def time.Duration) time.Duration {
	raw, ok := revel.Config.String(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	ms, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || ms <= 0 {
		panic(fmt.Sprintf("invalid %s=%q: must be a positive integer (ms)", key, raw))
	}
	return time.Duration(ms) * time.Millisecond
}

// contextWithTimeout bounds a stage of database work; unbounded contexts must
// never reach the driver (MOD-001 tracks full request-context propagation).
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func operationContext() (context.Context, context.CancelFunc) {
	return contextWithTimeout(operationTimeout)
}

// dialMongo connects and pings the server; failures are fatal at startup.
func dialMongo(url string) error {
	loadTimeoutConfig()

	opts := options.Client().ApplyURI(url)
	opts.SetConnectTimeout(connectTimeout)
	opts.SetServerSelectionTimeout(connectTimeout)
	// lea.CodecRegistry stores lea.ObjectID as a plain BSON ObjectId; the
	// explicit codecs are required because the driver's kind-based array
	// decoder shadows pointer-receiver ValueUnmarshaler for defined [12]byte
	// types. DefaultDocumentM restores mgo's bson.M decode of untyped
	// documents (blog theme templates access their fields as map keys).
	opts.SetBSONOptions(&options.BSONOptions{DefaultDocumentM: true})
	opts.SetRegistry(CodecRegistry)

	c, err := mongo.Connect(opts)
	if err != nil {
		return err
	}

	ctx, cancel := contextWithTimeout(connectTimeout)
	defer cancel()
	if err := c.Ping(ctx, nil); err != nil {
		_ = c.Disconnect(context.Background())
		return err
	}

	client = c
	return nil
}

// DatabaseName returns the database selected by Init, or "" before Init.
func DatabaseName() string {
	if database == nil {
		return ""
	}
	return database.Name()
}

// CheckMongoSessionLost keeps the legacy per-request health check hook.
// Reconnection is handled by the driver's connection pool, so a failed ping
// is only reported; callers stay up and later requests retry transparently.
func CheckMongoSessionLost() {
	if client == nil {
		return
	}
	ctx, cancel := operationContext()
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		Log("mongo readiness check failed")
	}
}

// Ping reports MongoDB readiness without exposing connection details. A nil
// client is intentionally treated as not ready so callers can return a stable
// health response while the process remains available for diagnostics.
func Ping() error {
	if client == nil {
		return fmt.Errorf("mongo client is not initialized")
	}
	ctx, cancel := operationContext()
	defer cancel()
	return client.Ping(ctx, nil)
}
