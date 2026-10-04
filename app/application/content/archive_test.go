package content

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type archiveTestSource struct {
	data      []byte
	openErr   error
	readErr   error
	closeErr  error
	nilReader bool
}

func (source archiveTestSource) Open(context.Context) (io.ReadCloser, int64, error) {
	if source.openErr != nil {
		return nil, 0, source.openErr
	}
	if source.nilReader {
		return nil, 0, nil
	}
	return &archiveTestReader{Reader: bytes.NewReader(source.data), readErr: source.readErr, closeErr: source.closeErr}, int64(len(source.data)), nil
}

type archiveTestReader struct {
	*bytes.Reader
	readErr  error
	closeErr error
}

func (reader *archiveTestReader) Read(buffer []byte) (int, error) {
	if reader.readErr != nil {
		return 0, reader.readErr
	}
	return reader.Reader.Read(buffer)
}

func (reader *archiveTestReader) Close() error { return reader.closeErr }

func TestWriteDeterministicTarGzSanitizesAndDeduplicatesEntries(t *testing.T) {
	entries := []ArchiveEntry{
		{DisplayName: `../report.txt`, Source: archiveTestSource{data: []byte("one")}},
		{DisplayName: `folder\\report.txt`, Source: archiveTestSource{data: []byte("two")}},
		{DisplayName: "", Source: archiveTestSource{data: []byte("three")}},
	}
	var first bytes.Buffer
	if err := WriteDeterministicTarGz(context.Background(), &first, entries); err != nil {
		t.Fatalf("WriteDeterministicTarGz() error = %v", err)
	}
	var second bytes.Buffer
	if err := WriteDeterministicTarGz(context.Background(), &second, entries); err != nil {
		t.Fatalf("second WriteDeterministicTarGz() error = %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("archive bytes are not deterministic")
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	tarReader := tar.NewReader(gzipReader)
	wantNames := []string{"report.txt", "report (2).txt", "attachment"}
	for index, wantName := range wantNames {
		header, err := tarReader.Next()
		if err != nil {
			t.Fatalf("entry %d: %v", index, err)
		}
		if header.Name != wantName || header.Typeflag != tar.TypeReg || header.Mode != 0o600 ||
			header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" || header.Linkname != "" ||
			!header.ModTime.Equal(time.Unix(0, 0).UTC()) {
			t.Fatalf("entry %d header = %#v", index, header)
		}
	}
	if _, err := tarReader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("archive has trailing entry: %v", err)
	}
}

func TestWriteDeterministicTarGzPropagatesSourceCloseFailure(t *testing.T) {
	var output bytes.Buffer
	err := WriteDeterministicTarGz(context.Background(), &output, []ArchiveEntry{{
		DisplayName: "a.txt", Source: archiveTestSource{data: []byte("a"), closeErr: errors.New("close failed")},
	}})
	var contentErr *Error
	if !errors.As(err, &contentErr) || contentErr.Category != ErrorStorageUnavailable || contentErr.Code != "archive_source_close" {
		t.Fatalf("WriteDeterministicTarGz() error = %#v", err)
	}
}

func TestWriteDeterministicTarGzPropagatesSourceReadFailure(t *testing.T) {
	readFailure := errors.New("read failed")
	closeFailure := errors.New("close failed")
	var output bytes.Buffer
	err := WriteDeterministicTarGz(context.Background(), &output, []ArchiveEntry{{
		DisplayName: "a.txt", Source: archiveTestSource{data: []byte("a"), readErr: readFailure, closeErr: closeFailure},
	}})
	var contentErr *Error
	if !errors.As(err, &contentErr) || contentErr.Category != ErrorStorageUnavailable || contentErr.Code != "archive_source_read" ||
		!errors.Is(err, readFailure) || !errors.Is(err, closeFailure) {
		t.Fatalf("WriteDeterministicTarGz() error = %#v", err)
	}
}

func TestWriteDeterministicTarGzPropagatesOpenFailure(t *testing.T) {
	var output bytes.Buffer
	err := WriteDeterministicTarGz(context.Background(), &output, []ArchiveEntry{{
		DisplayName: "a.txt", Source: archiveTestSource{openErr: errors.New("open failed")},
	}})
	var contentErr *Error
	if !errors.As(err, &contentErr) || contentErr.Code != "archive_source_open" {
		t.Fatalf("WriteDeterministicTarGz() error = %#v", err)
	}
}

func TestWriteDeterministicTarGzRejectsNilSourceReader(t *testing.T) {
	var output bytes.Buffer
	err := WriteDeterministicTarGz(context.Background(), &output, []ArchiveEntry{{
		DisplayName: "a.txt", Source: archiveTestSource{nilReader: true},
	}})
	var contentErr *Error
	if !errors.As(err, &contentErr) || contentErr.Code != "archive_source_open" {
		t.Fatalf("WriteDeterministicTarGz() error = %#v", err)
	}
}

func TestWriteDeterministicTarGzSupportsConfirmedVisibleTextLimit(t *testing.T) {
	name := strings.Repeat("a", MaxVisibleTextBytes-4) + ".txt"
	var output bytes.Buffer
	if err := WriteDeterministicTarGz(context.Background(), &output, []ArchiveEntry{{
		DisplayName: name, Source: archiveTestSource{data: []byte("body")},
	}}); err != nil {
		t.Fatalf("WriteDeterministicTarGz(long name) error = %v", err)
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	header, err := tar.NewReader(gzipReader).Next()
	if err != nil || header.Name != name {
		t.Fatalf("long archive entry = %q, %v", header.Name, err)
	}
}

func TestArchiveDownloadFilenameSanitizesHistoricalNoteTitle(t *testing.T) {
	if got, want := ArchiveDownloadFilename(" ../quarter\"\r\nreport\\final "), ".._quarter_report_final.tar.gz"; got != want {
		t.Fatalf("ArchiveDownloadFilename() = %q, want %q", got, want)
	}
	if got := ArchiveDownloadFilename("\x00\n"); got != "all.tar.gz" {
		t.Fatalf("ArchiveDownloadFilename(empty) = %q", got)
	}
	if got := ArchiveDownloadFilename(strings.Repeat("界", 100)); len(got) > MaxVisibleTextBytes || !strings.HasSuffix(got, ".tar.gz") || !utf8.ValidString(got) {
		t.Fatalf("ArchiveDownloadFilename(long) = %q (%d bytes)", got, len(got))
	}
}

func TestAttachmentDownloadFilenameSanitizesHistoricalDisplayName(t *testing.T) {
	if got, want := AttachmentDownloadFilename(" ../report\"\r\nfinal.txt "), ".._report_final.txt"; got != want {
		t.Fatalf("AttachmentDownloadFilename() = %q, want %q", got, want)
	}
	if got := AttachmentDownloadFilename("\x00\n"); got != "attachment" {
		t.Fatalf("AttachmentDownloadFilename(empty) = %q", got)
	}
	if got := AttachmentDownloadFilename(strings.Repeat("界", 100) + ".txt"); len(got) > MaxVisibleTextBytes || !utf8.ValidString(got) {
		t.Fatalf("AttachmentDownloadFilename(long) = %q (%d bytes)", got, len(got))
	}
}
