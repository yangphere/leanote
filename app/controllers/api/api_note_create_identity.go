package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

type apiNoteCreateIdentityInput struct {
	NoteID      string
	NotebookID  string
	Title       string
	Desc        string
	Tags        []string
	Abstract    string
	Content     string
	IsMarkdown  bool
	IsBlog      bool
	IsTrash     bool
	IsDeleted   bool
	CreatedTime string
	UpdatedTime string
	PublicTime  string
	Files       []apiNoteCreateFileIdentity
}

type apiNoteCreateFileIdentity struct {
	FileID        string
	LocalFileID   string
	Type          string
	Title         string
	HasBody       bool
	IsAttach      bool
	ContentDigest string
}

// newAPINoteCreateOperation derives the identity before upload. Server file
// IDs are intentionally absent from the digest; the returned asset identities
// are deterministic from the client NoteId, request and file slot.
func newAPINoteCreateOperation(ownerID, noteID domain.ObjectID, request info.ApiNote) (string, string, []applicationnotes.OperationAsset, error) {
	return newAPINoteCreateOperationWithContent(ownerID, noteID, request, nil)
}

// newAPINoteCreateOperationWithContent keeps the client NoteId as the
// operation scope while binding the request digest to the bytes that will be
// uploaded. A local file id is only a client-side name; it is not proof that
// two retries contain the same asset.
func newAPINoteCreateOperationWithContent(ownerID, noteID domain.ObjectID, request info.ApiNote, content map[int][]byte) (string, string, []applicationnotes.OperationAsset, error) {
	digests := make(map[int]string, len(content))
	for index, data := range content {
		digests[index] = apiAssetContentDigest(data)
	}
	return newAPINoteCreateOperationWithDigests(ownerID, noteID, request, digests)
}

func newAPINoteCreateOperationWithDigests(ownerID, noteID domain.ObjectID, request info.ApiNote, digests map[int]string) (string, string, []applicationnotes.OperationAsset, error) {
	if ownerID.IsZero() || noteID.IsZero() {
		return "", "", nil, fmt.Errorf("api note create identity: invalid owner or note")
	}
	input := apiNoteCreateIdentityInputFromRequest(noteID, request)
	for index := range input.Files {
		if !input.Files[index].HasBody {
			continue
		}
		digest, ok := digests[index]
		if !ok {
			input.Files[index].ContentDigest = "missing"
			continue
		}
		input.Files[index].ContentDigest = digest
	}
	assets := make([]applicationnotes.OperationAsset, 0, len(request.Files))
	operationID, digest, _, err := applicationnotes.NewResourceOperationIdentity("note_create", ownerID, noteID, input)
	if err != nil {
		return "", "", nil, err
	}
	for index, file := range input.Files {
		assetID := file.FileID
		if file.HasBody {
			if file.LocalFileID == "" {
				continue
			}
			assetID = stableAPIAssetID(operationID, file.LocalFileID, index, file.IsAttach)
		} else if assetID != "" {
			if _, err := domain.ParseObjectID(assetID); err != nil {
				return "", "", nil, fmt.Errorf("api note create identity: invalid existing asset: %w", err)
			}
		}
		if assetID == "" {
			continue
		}
		assets = append(assets, applicationnotes.OperationAsset{
			AssetID:     assetID,
			LocalFileID: file.LocalFileID, ContentSHA256: file.ContentDigest, Index: index, IsAttach: file.IsAttach,
		})
	}
	return operationID, digest, assets, nil
}

func apiNoteCreateIdentityInputFromRequest(noteID domain.ObjectID, request info.ApiNote) apiNoteCreateIdentityInput {
	input := apiNoteCreateIdentityInput{
		NoteID: noteID.Hex(), NotebookID: request.NotebookId, Title: request.Title, Desc: request.Desc,
		Tags: append([]string(nil), request.Tags...), Abstract: request.Abstract, Content: request.Content,
		IsMarkdown: request.IsMarkdown, IsBlog: request.IsBlog, IsTrash: request.IsTrash, IsDeleted: request.IsDeleted,
		CreatedTime: request.CreatedTime.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		UpdatedTime: request.UpdatedTime.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		PublicTime:  request.PublicTime.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		Files:       make([]apiNoteCreateFileIdentity, len(request.Files)),
	}
	for index, file := range request.Files {
		input.Files[index] = apiNoteCreateFileIdentity{
			FileID: file.FileId, LocalFileID: file.LocalFileId, Type: file.Type, Title: file.Title,
			HasBody: file.HasBody, IsAttach: file.IsAttach,
		}
	}
	return input
}

func stableAPIUpdateAssetSeed(ownerID, noteID domain.ObjectID, expectedUSN int, request info.ApiNote) string {
	return stableAPIUpdateAssetSeedWithContent(ownerID, noteID, expectedUSN, request, nil)
}

func stableAPIUpdateAssetSeedWithContent(ownerID, noteID domain.ObjectID, expectedUSN int, request info.ApiNote, content map[int][]byte) string {
	digests := make(map[int]string, len(content))
	for index, data := range content {
		digests[index] = apiAssetContentDigest(data)
	}
	return stableAPIUpdateAssetSeedWithDigests(ownerID, noteID, expectedUSN, request, digests)
}

func stableAPIUpdateAssetSeedWithDigests(ownerID, noteID domain.ObjectID, expectedUSN int, request info.ApiNote, digests map[int]string) string {
	identityRequest := apiNoteCreateIdentityInputFromRequest(noteID, request)
	for index := range identityRequest.Files {
		if !identityRequest.Files[index].HasBody {
			continue
		}
		digest, ok := digests[index]
		if !ok {
			identityRequest.Files[index].ContentDigest = "missing"
			continue
		}
		identityRequest.Files[index].ContentDigest = digest
	}
	input := struct {
		ExpectedUSN int
		Request     apiNoteCreateIdentityInput
	}{ExpectedUSN: expectedUSN, Request: identityRequest}
	operationID, _, _, err := applicationnotes.NewOperationIdentity("note_update_assets", ownerID, noteID, input)
	if err != nil {
		return ""
	}
	return operationID
}

func apiAssetContentDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func stableAPIAssetID(operationID, localFileID string, index int, isAttach bool) string {
	kind := "image"
	if isAttach {
		kind = "attach"
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("api-note-asset\x00%s\x00%s\x00%d\x00%s", operationID, localFileID, index, kind)))
	var id [12]byte
	copy(id[:], sum[:12])
	if id == ([12]byte{}) {
		id[11] = 1
	}
	return hex.EncodeToString(id[:])
}

func stableAPIUploadPath(ownerID, assetID string, isAttach bool) string {
	kind := "images"
	if isAttach {
		kind = "attachs"
	}
	return strings.TrimRight("files/"+ownerID+"/"+assetID+"/"+kind, "/")
}
