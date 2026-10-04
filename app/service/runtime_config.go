package service

import "errors"

// DatabaseConnection contains only the fields required by the maintenance
// command seam. It is never persisted or included in ProductionConfig; the
// interface layer provides it through an opaque resolver at invocation time.
type DatabaseConnection struct {
	Scheme       string
	Host         string
	Port         string
	DatabaseName string
	Username     string
	Password     string
	AuthSource   string
	TLSMode      string
}

type DatabaseConnectionProvider interface {
	ResolveDatabaseConnection() (DatabaseConnection, error)
}

type DatabaseConnectionProviderFunc func() (DatabaseConnection, error)

func (f DatabaseConnectionProviderFunc) ResolveDatabaseConnection() (DatabaseConnection, error) {
	if f == nil {
		return DatabaseConnection{}, errors.New("database connection provider is unavailable")
	}
	return f()
}

// RuntimeConfig is the typed handoff consumed by backup/restore services.
// Paths and database identity are validated by the interface startup layer;
// the credential-bearing resolver remains opaque and is evaluated only when
// an authorized maintenance action actually runs.
type RuntimeConfig struct {
	BackupRoot       string
	DatabaseName     string
	DatabaseIdentity ConfiguredDatabaseIdentity
	DatabaseProvider DatabaseConnectionProvider
}

var runtimeConfig RuntimeConfig

func SetRuntimeConfig(config RuntimeConfig) { runtimeConfig = config }

func currentRuntimeConfig() RuntimeConfig { return runtimeConfig }
