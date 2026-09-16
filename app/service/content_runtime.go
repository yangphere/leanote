package service

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/service/contentfs"
	"github.com/yangphere/leanote/app/service/contentpdf"
)

const (
	contentPDFPolicyID       = "leanote-pdf-v1"
	contentPDFTimeout        = 30 * time.Second
	contentPDFMaxOutputBytes = 64 * 1024 * 1024
)

type PDFExporter interface {
	Export(context.Context, domain.ObjectID, domain.ObjectID) (applicationcontent.PDFArtifact, error)
}

var ContentPDF PDFExporter

var contentStore *contentfs.FileStore

// InitContentRuntime establishes the one filesystem and PDF runtime used by
// every Web/API adapter. It is called after global administrator settings are
// loaded so renderer configuration has a single source of truth.
func InitContentRuntime(basePath string) error {
	store, err := initializeContentStore(basePath)
	if err != nil {
		return err
	}
	temporaryRoot := filepath.Join(basePath, ".content-temporary")
	backend := contentpdf.NewConfiguredBackend(func() string {
		if ConfigS == nil {
			return ""
		}
		return ConfigS.GetGlobalStringConfig("exportPdfBinPath")
	}, temporaryRoot, contentPDFPolicyID)
	repository := contentpdf.MongoPDFRepository{}
	exporter := &applicationcontent.PDFExportService{
		Notes:     contentpdf.NotePort{Repository: repository},
		Resources: contentpdf.ResourcePort{Repository: repository, Store: store},
		Renderer: applicationcontent.PDFRenderer{
			Backend: backend, Timeout: contentPDFTimeout, MaxOutputBytes: contentPDFMaxOutputBytes,
		},
	}
	previous := contentStore
	contentStore = store
	ContentPDF = exporter
	if previous != nil {
		if err := previous.Close(); err != nil {
			return fmt.Errorf("replace content runtime: %w", err)
		}
	}
	return nil
}

func initializeContentStore(basePath string) (*contentfs.FileStore, error) {
	basePath, err := filepath.Abs(filepath.Clean(basePath))
	if err != nil {
		return nil, fmt.Errorf("content runtime base: %w", err)
	}
	privateData := filepath.Join(basePath, "files")
	publicData := filepath.Join(basePath, "public", "upload")
	privateQuarantine := filepath.Join(basePath, ".content-private-quarantine")
	publicQuarantine := filepath.Join(basePath, ".content-public-quarantine")
	temporary := filepath.Join(basePath, ".content-temporary")
	roots, err := contentfs.ValidateContentRoots(contentfs.ContentRootsConfig{
		PrivateFiles: contentfs.DurableRootConfig{Data: privateData, Quarantine: privateQuarantine},
		PublicUpload: contentfs.DurableRootConfig{Data: publicData, Quarantine: publicQuarantine},
		Temporary:    temporary,
		ServedRoots:  []string{filepath.Join(basePath, "public")},
	})
	if err != nil {
		return nil, err
	}
	return contentfs.NewFileStore(roots)
}
