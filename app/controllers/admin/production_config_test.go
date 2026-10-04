package admin

import (
	"testing"
	"time"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/service"
)

// productionConfigConsumerFixture models the read-only fields the admin
// application will consume. It intentionally has no config/env parsing path.
type productionConfigConsumerFixture struct {
	addr             string
	shutdownTimeout  time.Duration
	databaseName     string
	databaseIdentity service.ConfiguredDatabaseIdentity
	identityDigest   string
	credentialRef    service.CredentialProviderRef
	backupRoot       string
}

func consumeProductionConfigFixture(config *httpserver.ProductionConfig) (productionConfigConsumerFixture, error) {
	if config == nil {
		return productionConfigConsumerFixture{}, service.ErrBackupValidation
	}
	if err := config.CredentialProviderRef.Validate(); err != nil {
		return productionConfigConsumerFixture{}, err
	}
	return productionConfigConsumerFixture{
		addr:             config.Addr,
		shutdownTimeout:  config.ShutdownTimeout,
		databaseName:     config.DatabaseName,
		databaseIdentity: config.ConfiguredDatabaseIdentity,
		identityDigest:   config.DatabaseIdentityDigest,
		credentialRef:    config.CredentialProviderRef,
		backupRoot:       config.BackupRoot,
	}, nil
}

func TestProductionConfigConsumerFixtureUsesTypedInterfaceValues(t *testing.T) {
	config := &httpserver.ProductionConfig{
		Addr:                       "127.0.0.1:9000",
		ShutdownTimeout:            5 * time.Second,
		DatabaseName:               "leanote",
		ConfiguredDatabaseIdentity: service.ConfiguredDatabaseIdentity{Scheme: "mongodb", ClusterHost: "db.example", ClusterPort: "27017", DatabaseName: "leanote", TLSMode: "disabled"},
		DatabaseIdentityDigest:     "digest",
		CredentialProviderRef:      service.CredentialProviderRef{ProviderKind: "env", OpaqueHandle: "MONGODB_URL", Owner: "interface-http"},
		BackupRoot:                 "/var/lib/leanote/backup",
	}
	got, err := consumeProductionConfigFixture(config)
	if err != nil {
		t.Fatalf("consumeProductionConfigFixture: %v", err)
	}
	if got.addr != config.Addr || got.shutdownTimeout != config.ShutdownTimeout || got.databaseName != config.DatabaseName || got.databaseIdentity != config.ConfiguredDatabaseIdentity || got.identityDigest != config.DatabaseIdentityDigest || got.credentialRef != config.CredentialProviderRef || got.backupRoot != config.BackupRoot {
		t.Fatalf("consumer fixture = %+v, want typed production values", got)
	}
}
