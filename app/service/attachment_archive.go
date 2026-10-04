package service

import (
	"context"
	"errors"
	"io"
	"sort"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/info"
)

type attachmentArchiveSource struct {
	store applicationcontent.ContentStore
	path  applicationcontent.LogicalPath
}

type attachmentArchiveStore interface {
	applicationcontent.ContentStore
	applicationcontent.TemporaryStore
}

func (source attachmentArchiveSource) Open(ctx context.Context) (io.ReadCloser, int64, error) {
	opened, err := source.store.Open(ctx, source.path)
	if err != nil {
		return nil, 0, err
	}
	return opened.Reader, opened.Size, nil
}

func OpenAttachmentArchive(ctx context.Context, attachments []info.Attach) (io.ReadCloser, error) {
	if contentStore == nil {
		return nil, applicationcontent.NewError(applicationcontent.ErrorDependency, "content_store_unavailable", nil)
	}
	return openAttachmentArchive(ctx, contentStore, attachments)
}

func openAttachmentArchive(ctx context.Context, store attachmentArchiveStore, attachments []info.Attach) (io.ReadCloser, error) {
	if store == nil {
		return nil, applicationcontent.NewError(applicationcontent.ErrorDependency, "content_store_unavailable", nil)
	}
	ordered := append([]info.Attach(nil), attachments...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftKey := ordered[left].AttachId.Hex() + "\x00" + ordered[left].Path + "\x00" + ordered[left].Title
		rightKey := ordered[right].AttachId.Hex() + "\x00" + ordered[right].Path + "\x00" + ordered[right].Title
		return leftKey < rightKey
	})
	entries := make([]applicationcontent.ArchiveEntry, 0, len(ordered))
	for _, attachment := range ordered {
		logical, err := applicationcontent.ParseStoredPath(attachment.Path)
		if err != nil {
			return nil, err
		}
		entries = append(entries, applicationcontent.ArchiveEntry{
			DisplayName: attachment.Title,
			Source:      attachmentArchiveSource{store: store, path: logical},
		})
	}
	temporary, err := store.CreateTemporary(ctx, ".leanote-attachment-archive-")
	if err != nil {
		return nil, err
	}
	if err := applicationcontent.WriteDeterministicTarGz(ctx, temporary, entries); err != nil {
		return nil, errors.Join(err, temporary.Abort())
	}
	reader, _, err := temporary.Seal(ctx)
	if err != nil {
		return nil, errors.Join(
			err,
			temporary.Abort(),
		)
	}
	return reader, nil
}
