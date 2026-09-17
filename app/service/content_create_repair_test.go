package service

import (
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestContentCreateImageRecordDigestCoversStableMetadata(t *testing.T) {
	row := info.File{
		FileId: domain.ObjectID{1}, UserId: domain.ObjectID{2}, AlbumId: domain.ObjectID{3}, Name: "stored.png", Title: "title",
		Size: 5, Type: "", Path: "files/owner/stored.png", IsDefaultAlbum: true, CreatedTime: time.Unix(1_700_000_000, 123).UTC(), FromFileId: domain.ObjectID{4},
	}
	want := contentCreateImageRecordDigest(row)
	bsonRoundTrip := row
	bsonRoundTrip.CreatedTime = bsonRoundTrip.CreatedTime.Truncate(time.Millisecond)
	if got := contentCreateImageRecordDigest(bsonRoundTrip); got != want {
		t.Fatal("BSON millisecond time normalization changed image record digest")
	}
	tests := []struct {
		name   string
		mutate func(*info.File)
	}{
		{"name", func(value *info.File) { value.Name = "other.png" }},
		{"title", func(value *info.File) { value.Title = "other" }},
		{"album", func(value *info.File) { value.AlbumId = domain.ObjectID{9} }},
		{"type", func(value *info.File) { value.Type = "png" }},
		{"default", func(value *info.File) { value.IsDefaultAlbum = false }},
		{"created", func(value *info.File) { value.CreatedTime = value.CreatedTime.Add(time.Second) }},
		{"lineage", func(value *info.File) { value.FromFileId = domain.ObjectID{8} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := row
			test.mutate(&changed)
			if got := contentCreateImageRecordDigest(changed); got == want {
				t.Fatal("metadata change did not alter record digest")
			}
		})
	}
}

func TestContentCreateAttachmentRecordDigestCoversStableMetadata(t *testing.T) {
	row := info.Attach{
		AttachId: domain.ObjectID{1}, NoteId: domain.ObjectID{2}, UploadUserId: domain.ObjectID{3}, Name: "stored.pdf", Title: "title",
		Size: 5, Type: "pdf", Path: "files/owner/stored.pdf", CreatedTime: time.Unix(1_700_000_000, 123).UTC(),
	}
	want := contentCreateAttachmentRecordDigest(row)
	bsonRoundTrip := row
	bsonRoundTrip.CreatedTime = bsonRoundTrip.CreatedTime.Truncate(time.Millisecond)
	if got := contentCreateAttachmentRecordDigest(bsonRoundTrip); got != want {
		t.Fatal("BSON millisecond time normalization changed attachment record digest")
	}
	tests := []struct {
		name   string
		mutate func(*info.Attach)
	}{
		{"name", func(value *info.Attach) { value.Name = "other.pdf" }},
		{"title", func(value *info.Attach) { value.Title = "other" }},
		{"type", func(value *info.Attach) { value.Type = "bin" }},
		{"created", func(value *info.Attach) { value.CreatedTime = value.CreatedTime.Add(time.Second) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := row
			test.mutate(&changed)
			if got := contentCreateAttachmentRecordDigest(changed); got == want {
				t.Fatal("metadata change did not alter record digest")
			}
		})
	}
}
