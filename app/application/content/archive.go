package content

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ArchiveSource interface {
	Open(context.Context) (io.ReadCloser, int64, error)
}

type TemporaryArtifact interface {
	io.Writer
	Seal(context.Context) (io.ReadCloser, int64, error)
	Abort() error
}

type TemporaryStore interface {
	CreateTemporary(context.Context, string) (TemporaryArtifact, error)
}

type ArchiveEntry struct {
	DisplayName string
	Source      ArchiveSource
}

const archiveExtension = ".tar.gz"

// ArchiveDownloadFilename preserves access to historical note titles while
// ensuring the Content-Disposition filename cannot inject a header or path.
func ArchiveDownloadFilename(title string) string {
	return sanitizedDownloadFilename(title, "all", archiveExtension)
}

func AttachmentDownloadFilename(displayName string) string {
	return sanitizedDownloadFilename(displayName, "attachment", "")
}

func sanitizedDownloadFilename(value, fallback, suffix string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		switch r {
		case '/', '\\', '"':
			return '_'
		default:
			return r
		}
	}, strings.TrimSpace(value))
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback + suffix
	}
	maxValueBytes := MaxVisibleTextBytes - len(suffix)
	for len(value) > maxValueBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	if value == "" {
		return fallback + suffix
	}
	return value + suffix
}

func WriteDeterministicTarGz(ctx context.Context, destination io.Writer, entries []ArchiveEntry) error {
	if destination == nil {
		return validationError("archive_destination_required", nil)
	}
	gzipWriter := gzip.NewWriter(destination)
	gzipWriter.Name = ""
	gzipWriter.Comment = ""
	gzipWriter.ModTime = time.Time{}
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	names := make(map[string]int, len(entries))

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return errors.Join(contentError(ErrorTimeout, "archive_cancelled", err), closeArchiveWriters(tarWriter, gzipWriter))
		}
		if entry.Source == nil {
			return errors.Join(validationError("archive_source_required", nil), closeArchiveWriters(tarWriter, gzipWriter))
		}
		reader, size, err := entry.Source.Open(ctx)
		if err != nil {
			return errors.Join(storageError("archive_source_open", err), closeArchiveWriters(tarWriter, gzipWriter))
		}
		if reader == nil {
			return errors.Join(
				storageError("archive_source_open", fmt.Errorf("nil archive source reader")),
				closeArchiveWriters(tarWriter, gzipWriter),
			)
		}
		if size < 0 {
			return errors.Join(validationError("archive_source_size", nil), reader.Close(), closeArchiveWriters(tarWriter, gzipWriter))
		}
		name := uniqueArchiveName(safeArchiveName(entry.DisplayName), names)
		header := &tar.Header{
			Name:       name,
			Mode:       0o600,
			Size:       size,
			ModTime:    time.Unix(0, 0).UTC(),
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Typeflag:   tar.TypeReg,
			Format:     tar.FormatPAX,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return errors.Join(storageError("archive_header_write", err), reader.Close(), closeArchiveWriters(tarWriter, gzipWriter))
		}
		if _, err := io.Copy(tarWriter, contextArchiveReader{ctx: ctx, reader: reader}); err != nil {
			closeErr := errors.Join(reader.Close(), closeArchiveWriters(tarWriter, gzipWriter))
			if ctxErr := ctx.Err(); ctxErr != nil {
				return errors.Join(contentError(ErrorTimeout, "archive_cancelled", ctxErr), closeErr)
			}
			return errors.Join(storageError("archive_source_read", err), closeErr)
		}
		if err := reader.Close(); err != nil {
			return errors.Join(storageError("archive_source_close", err), closeArchiveWriters(tarWriter, gzipWriter))
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return storageError("archive_tar_close", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return storageError("archive_gzip_close", err)
	}
	return nil
}

func closeArchiveWriters(tarWriter *tar.Writer, gzipWriter *gzip.Writer) error {
	return errors.Join(tarWriter.Close(), gzipWriter.Close())
}

type contextArchiveReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextArchiveReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func safeArchiveName(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	cleaned, err := CleanVisibleText(value, true)
	if err != nil || cleaned == "" || cleaned == "." || cleaned == ".." {
		return "attachment"
	}
	return cleaned
}

func uniqueArchiveName(name string, seen map[string]int) string {
	seen[name]++
	if seen[name] == 1 {
		return name
	}
	extension := path.Ext(name)
	base := strings.TrimSuffix(name, extension)
	for index := seen[name]; ; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, index, extension)
		if seen[candidate] == 0 {
			seen[name] = index
			seen[candidate] = 1
			return candidate
		}
	}
}
