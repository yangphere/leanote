package service

import (
	"context"
	"errors"
	"testing"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
)

func TestContentAssetReceiptGenerationUsesFrozenAssignedUSN(t *testing.T) {
	receipt := applicationnotes.OperationReceipt{AssignedUSN: 17}
	if got, err := contentAssetReceiptGeneration(receipt, 0, false); err != nil || got != 17 {
		t.Fatalf("derived generation=(%d, %v), want (17, nil)", got, err)
	}
	if got, err := contentAssetReceiptGeneration(receipt, 17, true); err != nil || got != 17 {
		t.Fatalf("supplied generation=(%d, %v), want (17, nil)", got, err)
	}
	for _, test := range []struct {
		name     string
		receipt  applicationnotes.OperationReceipt
		supplied int
		required bool
		category applicationcontent.ErrorCategory
	}{
		{name: "missing receipt generation", receipt: applicationnotes.OperationReceipt{}, supplied: 17, required: true, category: applicationcontent.ErrorConflict},
		{name: "missing command generation", receipt: receipt, required: true, category: applicationcontent.ErrorValidation},
		{name: "changed command generation", receipt: receipt, supplied: 18, required: true, category: applicationcontent.ErrorConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			var contentErr *applicationcontent.Error
			if _, err := contentAssetReceiptGeneration(test.receipt, test.supplied, test.required); err == nil || !errors.As(err, &contentErr) || contentErr.Category != test.category {
				t.Fatalf("generation error=%v, want category %q", err, test.category)
			}
		})
	}
}

func TestVerifyFrozenCopyRowsRejectsNonCanonicalDestinationIdentity(t *testing.T) {
	destinationOwnerID, err := domain.ParseObjectID("507f1f77bcf86cd799439021")
	if err != nil {
		t.Fatal(err)
	}
	command := applicationnotes.CopyNoteAssetsCommand{
		AssetOperationID:   "copy-assets",
		DestinationOwnerID: destinationOwnerID,
	}
	for _, test := range []struct {
		name  string
		asset applicationnotes.OperationAsset
	}{
		{name: "image", asset: applicationnotes.OperationAsset{AssetID: "507f1f77bcf86cd799439022", LocalFileID: "507f1f77bcf86cd799439023"}},
		{name: "attachment", asset: applicationnotes.OperationAsset{AssetID: "507f1f77bcf86cd799439024", LocalFileID: "507f1f77bcf86cd799439025", IsAttach: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var contentErr *applicationcontent.Error
			ok, err := verifyFrozenCopyRows(context.Background(), command, []applicationnotes.OperationAsset{test.asset})
			if ok || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorConflict {
				t.Fatalf("verifyFrozenCopyRows() = (%v, %v), want conflict", ok, err)
			}
		})
	}
}

func TestRequireContentAssetReceiptKindRejectsUnrelatedReceipt(t *testing.T) {
	if err := requireContentAssetReceiptKind(applicationnotes.OperationReceipt{Kind: "note_save"}, "note_save"); err != nil {
		t.Fatalf("expected note receipt rejection: %v", err)
	}
	var contentErr *applicationcontent.Error
	err := requireContentAssetReceiptKind(applicationnotes.OperationReceipt{Kind: "notebook_delete"}, "note_save", "note_create")
	if !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorConflict {
		t.Fatalf("unrelated receipt error=%v", err)
	}
}

func TestValidateContentAssetManifestPreservesFrozenOrder(t *testing.T) {
	valid := []applicationnotes.OperationAsset{
		{AssetID: "507f1f77bcf86cd799439011", Index: 0},
		{AssetID: "507f1f77bcf86cd799439012", Index: 2, IsAttach: true},
	}
	if err := validateContentAssetManifest(valid); err != nil {
		t.Fatalf("valid manifest error=%v", err)
	}
	for _, test := range []struct {
		name   string
		assets []applicationnotes.OperationAsset
	}{
		{name: "reordered", assets: []applicationnotes.OperationAsset{{AssetID: valid[0].AssetID, Index: 2}, {AssetID: valid[1].AssetID, Index: 1}}},
		{name: "duplicate", assets: []applicationnotes.OperationAsset{{AssetID: valid[0].AssetID, Index: 0}, {AssetID: valid[0].AssetID, Index: 1}}},
		{name: "duplicate index", assets: []applicationnotes.OperationAsset{{AssetID: valid[0].AssetID, Index: 0}, {AssetID: valid[1].AssetID, Index: 0}}},
		{name: "invalid identity", assets: []applicationnotes.OperationAsset{{AssetID: "invalid", Index: 0}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var contentErr *applicationcontent.Error
			if err := validateContentAssetManifest(test.assets); err == nil || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorConflict {
				t.Fatalf("validateContentAssetManifest() error=%v", err)
			}
		})
	}
}

func TestValidateFrozenCopyManifestRequiresContentAndRecordDigests(t *testing.T) {
	validDigest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	valid := applicationnotes.OperationAsset{
		AssetID: "507f1f77bcf86cd799439011", LocalFileID: "507f1f77bcf86cd799439012",
		ContentSHA256: validDigest, RecordSHA256: validDigest,
	}
	if err := validateFrozenCopyManifest([]applicationnotes.OperationAsset{valid}); err != nil {
		t.Fatalf("valid frozen manifest error=%v", err)
	}
	for _, test := range []struct {
		name  string
		asset applicationnotes.OperationAsset
	}{
		{name: "missing source identity", asset: func() applicationnotes.OperationAsset { value := valid; value.LocalFileID = ""; return value }()},
		{name: "missing content digest", asset: func() applicationnotes.OperationAsset { value := valid; value.ContentSHA256 = ""; return value }()},
		{name: "invalid content digest", asset: func() applicationnotes.OperationAsset { value := valid; value.ContentSHA256 = "invalid"; return value }()},
		{name: "missing row digest", asset: func() applicationnotes.OperationAsset { value := valid; value.RecordSHA256 = ""; return value }()},
		{name: "invalid row digest", asset: func() applicationnotes.OperationAsset { value := valid; value.RecordSHA256 = "invalid"; return value }()},
	} {
		t.Run(test.name, func(t *testing.T) {
			var contentErr *applicationcontent.Error
			if err := validateFrozenCopyManifest([]applicationnotes.OperationAsset{test.asset}); err == nil || !errors.As(err, &contentErr) || contentErr.Category != applicationcontent.ErrorConflict {
				t.Fatalf("validateFrozenCopyManifest() error=%v", err)
			}
		})
	}
}
