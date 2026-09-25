package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/service/contentfs"
	"github.com/yangphere/leanote/app/service/contentpdf"
	"github.com/yangphere/leanote/app/service/contentremote"
)

const (
	contentPDFPolicyID             = "leanote-pdf-v1"
	contentPDFTimeout              = 30 * time.Second
	contentPDFMaxOutputBytes       = 64 * 1024 * 1024
	contentMaintenanceTimeout      = 5 * time.Second
	contentTerminalGCLimit         = 128
	contentCreateRecoveryLimit     = 64
	contentAPIPreNoteRecoveryLimit = 64
)

type contentMaintenanceResult struct {
	DeleteTerminalGC   contentfs.TerminalGCResult
	CreateTerminalGC   contentfs.TerminalGCResult
	CreateRecovery     applicationcontent.CreateRecoveryResult
	APIPreNoteRecovery applicationcontent.CreateRecoveryResult
}

type terminalGCStore interface {
	GCTerminalsBounded(context.Context, time.Time, int) (contentfs.TerminalGCResult, error)
	GCCreateTerminalsBounded(context.Context, time.Time, int) (contentfs.TerminalGCResult, error)
}

type createRecoveryService interface {
	RecoverAbandoned(context.Context, int) (applicationcontent.CreateRecoveryResult, error)
}

type apiPreNoteRecoveryService interface {
	RecoverAPIPreNotes(context.Context, int) (applicationcontent.CreateRecoveryResult, error)
}

type PDFExporter interface {
	Export(context.Context, domain.ObjectID, domain.ObjectID) (applicationcontent.PDFArtifact, error)
}

var ContentPDF PDFExporter

type contentRuntimeStore interface {
	applicationcontent.ContentStore
	applicationcontent.TemporaryStore
	Close() error
}

var contentStore contentRuntimeStore
var contentDeleteManifests *contentfs.DeleteManifestStore
var contentLifecycle *contentfs.LifecycleStore
var contentCreateRepair *applicationcontent.CreateRepairService

// ContentRootPair identifies one durable data root and its non-public,
// same-filesystem quarantine root. The interface layer owns where these
// paths come from; content validates and opens them.
type ContentRootPair struct {
	Data       string
	Quarantine string
}

// ContentRoots is the typed startup contract consumed by the content
// runtime. Paths must already be explicit absolute deployment values.
type ContentRoots struct {
	PrivateFiles ContentRootPair
	PublicUpload ContentRootPair
	Temporary    string
	ServedRoots  []string
}

// InitContentRuntime establishes the one filesystem and PDF runtime used by
// every Web/API adapter. It is called after global administrator settings are
// loaded so renderer configuration has a single source of truth.
func InitContentRuntime(config ContentRoots) error {
	store, manifests, lifecycle, temporaryRoot, err := initializeContentStorage(config)
	if err != nil {
		return err
	}
	createRepair := &applicationcontent.CreateRepairService{
		Content: store, Lifecycle: lifecycle, Manifests: manifests, Rows: mongoContentCreateRows{},
	}
	preNoteRecovery := apiPreNoteStartupRecovery{
		scanner: manifests,
		runtime: apiPreNoteRuntime{content: store, manifests: manifests, lifecycle: lifecycle, creates: createRepair},
	}
	maintenanceContext, cancelMaintenance := context.WithTimeout(context.Background(), contentMaintenanceTimeout)
	maintenance, err := runContentStartupMaintenance(maintenanceContext, manifests, createRepair, preNoteRecovery, time.Now().UTC())
	cancelMaintenance()
	if err != nil {
		_ = closeContentRuntime(store, manifests, lifecycle)
		return fmt.Errorf("content startup maintenance: %w", err)
	}
	log.Printf("content startup maintenance: terminal manifests scanned=%d removed=%d truncated=%t; create manifests scanned=%d committed=%d discarded=%d pending=%d truncated=%t; API pre-note manifests scanned=%d committed=%d discarded=%d pending=%d truncated=%t",
		maintenance.DeleteTerminalGC.Scanned+maintenance.CreateTerminalGC.Scanned,
		len(maintenance.DeleteTerminalGC.Removed)+len(maintenance.CreateTerminalGC.Removed),
		maintenance.DeleteTerminalGC.Truncated || maintenance.CreateTerminalGC.Truncated,
		maintenance.CreateRecovery.Scanned, maintenance.CreateRecovery.Committed, maintenance.CreateRecovery.Discarded, maintenance.CreateRecovery.Pending, maintenance.CreateRecovery.Truncated,
		maintenance.APIPreNoteRecovery.Scanned, maintenance.APIPreNoteRecovery.Committed, maintenance.APIPreNoteRecovery.Discarded, maintenance.APIPreNoteRecovery.Pending, maintenance.APIPreNoteRecovery.Truncated)
	backend := contentpdf.NewConfiguredBackend(func() string {
		if ConfigS == nil {
			return ""
		}
		return ConfigS.GetGlobalStringConfig("exportPdfBinPath")
	}, temporaryRoot, contentPDFPolicyID)
	repository := contentpdf.MongoPDFRepository{}
	remoteFetcher := contentremote.New(nil, nil)
	exporter := &applicationcontent.PDFExportService{
		Notes:     contentpdf.NotePort{Repository: repository, Permission: sharePermissionAdapter{}},
		Resources: contentpdf.ResourcePort{Repository: repository, Store: store},
		RemoteResources: contentpdf.RemoteResourcePort{
			Fetcher: remoteFetcher,
			UploadImageBytes: func() (int64, error) {
				if ConfigS == nil {
					return 0, fmt.Errorf("content PDF remote limit: configuration unavailable")
				}
				return ConfigS.GetUploadLimitBytes("uploadImageSize")
			},
		},
		Renderer: applicationcontent.PDFRenderer{
			Backend: backend, Timeout: contentPDFTimeout, MaxOutputBytes: contentPDFMaxOutputBytes,
		},
	}
	previous := contentStore
	previousManifests := contentDeleteManifests
	previousLifecycle := contentLifecycle
	contentStore = store
	contentDeleteManifests = manifests
	contentLifecycle = lifecycle
	contentCreateRepair = createRepair
	ContentPDF = exporter
	if err := closeContentRuntime(previous, previousManifests, previousLifecycle); err != nil {
		return fmt.Errorf("replace content runtime: %w", err)
	}
	return nil
}

func runContentStartupMaintenance(ctx context.Context, manifests terminalGCStore, creates createRecoveryService, preNotes apiPreNoteRecoveryService, now time.Time) (contentMaintenanceResult, error) {
	if manifests == nil || creates == nil || preNotes == nil || now.IsZero() {
		return contentMaintenanceResult{}, fmt.Errorf("content startup maintenance: invalid input")
	}
	deleteGC, err := manifests.GCTerminalsBounded(ctx, now.UTC(), contentTerminalGCLimit)
	if err != nil {
		return contentMaintenanceResult{DeleteTerminalGC: deleteGC}, err
	}
	createGC, err := manifests.GCCreateTerminalsBounded(ctx, now.UTC(), contentTerminalGCLimit)
	if err != nil {
		return contentMaintenanceResult{DeleteTerminalGC: deleteGC, CreateTerminalGC: createGC}, err
	}
	recovery, err := creates.RecoverAbandoned(ctx, contentCreateRecoveryLimit)
	result := contentMaintenanceResult{DeleteTerminalGC: deleteGC, CreateTerminalGC: createGC, CreateRecovery: recovery}
	if err != nil {
		return result, err
	}
	preNoteRecovery, err := preNotes.RecoverAPIPreNotes(ctx, contentAPIPreNoteRecoveryLimit)
	result.APIPreNoteRecovery = preNoteRecovery
	if err != nil {
		return result, err
	}
	return result, nil
}

func initializeContentStore(config ContentRoots) (*contentfs.FileStore, error) {
	store, manifests, lifecycle, _, err := initializeContentStorage(config)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(manifests.Close(), lifecycle.Close()); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func initializeContentStorage(config ContentRoots) (*contentfs.FileStore, *contentfs.DeleteManifestStore, *contentfs.LifecycleStore, string, error) {
	roots, err := contentfs.ValidateContentRoots(contentfs.ContentRootsConfig{
		PrivateFiles: contentfs.DurableRootConfig{Data: config.PrivateFiles.Data, Quarantine: config.PrivateFiles.Quarantine},
		PublicUpload: contentfs.DurableRootConfig{Data: config.PublicUpload.Data, Quarantine: config.PublicUpload.Quarantine},
		Temporary:    config.Temporary,
		ServedRoots:  append([]string(nil), config.ServedRoots...),
	})
	if err != nil {
		return nil, nil, nil, "", err
	}
	temporaryRoot, err := roots.CanonicalTemporaryPath()
	if err != nil {
		return nil, nil, nil, "", err
	}
	store, err := contentfs.NewFileStore(roots)
	if err != nil {
		return nil, nil, nil, "", err
	}
	manifests, err := contentfs.NewDeleteManifestStore(roots)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, "", err
	}
	lifecycle, err := contentfs.NewLifecycleStore(roots)
	if err != nil {
		_ = manifests.Close()
		_ = store.Close()
		return nil, nil, nil, "", err
	}
	return store, manifests, lifecycle, temporaryRoot, nil
}

func closeContentRuntime(store contentRuntimeStore, manifests *contentfs.DeleteManifestStore, lifecycle *contentfs.LifecycleStore) error {
	var result error
	if lifecycle != nil {
		result = errors.Join(result, lifecycle.Close())
	}
	if manifests != nil {
		result = errors.Join(result, manifests.Close())
	}
	if store != nil {
		result = errors.Join(result, store.Close())
	}
	return result
}
