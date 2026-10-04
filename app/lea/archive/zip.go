package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxArchiveBytes      uint64 = 10 * 1024 * 1024
	maxArchiveFiles             = 100
	maxArchiveFileBytes  uint64 = 5 * 1024 * 1024
	maxArchiveTotalBytes uint64 = 20 * 1024 * 1024
	maxArchiveEntries           = 200
	maxArchiveDepth             = 8
)

// main functions shows how to TarGz a directory/file and
// UnTarGz a file
// Gzip and tar from source directory or file to destination file
// you need check file exist before you call this function

func Zip(srcDirPath string, destFilePath string) (ok bool) {
	defer func() { //必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			ok = false
		}
	}()

	fw, err := os.Create(destFilePath)

	if err != nil {
		panic(err)
	}
	defer fw.Close()

	// Tar writer
	tw := zip.NewWriter(fw)
	defer tw.Close()

	// Check if it's a file or a directory
	f, err := os.Open(srcDirPath)
	if err != nil {
		panic(err)
	}
	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}
	if fi.IsDir() {
		// handle source directory
		//        fmt.Println("Cerating tar.gz from directory...")
		zipDir(srcDirPath, path.Base(srcDirPath), tw)
	} else {
		// handle file directly
		//        fmt.Println("Cerating tar.gz from " + fi.Name() + "...")
		zipFile(srcDirPath, fi.Name(), tw, fi)
	}
	ok = true
	return
}

// Deal with directories
// if find files, handle them with zipFile
// Every recurrence append the base path to the recPath
// recPath is the path inside of tar.gz
func zipDir(srcDirPath string, recPath string, tw *zip.Writer) {
	// Open source diretory
	dir, err := os.Open(srcDirPath)
	if err != nil {
		panic(err)
	}
	defer dir.Close()

	// Get file info slice
	fis, err := dir.Readdir(0)
	if err != nil {
		panic(err)
	}
	for _, fi := range fis {
		// Append path
		curPath := srcDirPath + "/" + fi.Name()
		// Check it is directory or file
		if fi.IsDir() {
			// Directory
			// (Directory won't add unitl all subfiles are added)
			//            fmt.Printf("Adding path...%s\n", curPath)
			zipDir(curPath, recPath+"/"+fi.Name(), tw)
		} else {
			// File
			//            fmt.Printf("Adding file...%s\n", curPath)
		}

		zipFile(curPath, recPath+"/"+fi.Name(), tw, fi)
	}
}

// Deal with files
func zipFile(srcFile string, recPath string, tw *zip.Writer, fi os.FileInfo) {
	if fi.IsDir() {
		//    	fmt.Println("??")
		// Create tar header
		/*
		   fh, err := zip.FileInfoHeader(fi)
		   if err != nil {
		       panic(err)
		   }
		   fh.Name = recPath // + "/"
		   err = tw.WriteHeader(hdr)
		   tw.Create(recPath)
		*/
	} else {
		// File reader
		fr, err := os.Open(srcFile)
		if err != nil {
			panic(err)
		}
		defer fr.Close()

		// Write hander
		w, err2 := tw.Create(recPath)
		if err2 != nil {
			panic(err)
		}
		// Write file data
		_, err = io.Copy(w, fr)
		if err != nil {
			panic(err)
		}
	}
}

// Ungzip and untar from source file to destination directory
// you need check file exist before you call this function
func Unzip(srcFilePath string, destDirPath string) (ok bool, msg string) {
	cleanup := func() { _ = os.RemoveAll(destDirPath) }
	fail := func(err error) (bool, string) {
		cleanup()
		return false, err.Error()
	}
	stat, err := os.Stat(srcFilePath)
	if err != nil {
		return fail(err)
	}
	if stat.Size() > int64(maxArchiveBytes) {
		return fail(fmt.Errorf("zip archive exceeds 10 MiB"))
	}
	r, err := zip.OpenReader(srcFilePath)
	if err != nil {
		return fail(err)
	}
	defer r.Close()
	if len(r.File) == 0 || len(r.File) > maxArchiveEntries {
		return fail(fmt.Errorf("invalid zip entry count"))
	}

	type entry struct {
		file *zip.File
		name string
		dir  bool
	}
	entries := make([]entry, 0, len(r.File))
	top := ""
	stripTop := true
	for _, file := range r.File {
		name, dir, err := normalizeArchiveEntry(file)
		if err != nil {
			return fail(err)
		}
		parts := strings.Split(name, "/")
		if len(parts) == 1 {
			if !dir {
				stripTop = false
			}
		} else if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			stripTop = false
		}
		entries = append(entries, entry{file: file, name: name, dir: dir})
	}
	seen := make(map[string]struct{}, len(entries))
	regularFiles := 0
	filteredEntries := entries[:0]
	for i := range entries {
		if stripTop {
			parts := strings.Split(entries[i].name, "/")
			if len(parts) == 1 {
				if entries[i].dir {
					continue
				}
				return fail(fmt.Errorf("invalid empty zip entry"))
			}
			entries[i].name = strings.Join(parts[1:], "/")
		}
		if entries[i].name == "" {
			return fail(fmt.Errorf("invalid empty zip entry"))
		}
		if _, exists := seen[entries[i].name]; exists {
			return fail(fmt.Errorf("duplicate zip entry: %s", entries[i].name))
		}
		seen[entries[i].name] = struct{}{}
		if depth := len(strings.Split(entries[i].name, "/")); depth > maxArchiveDepth {
			return fail(fmt.Errorf("zip entry path is too deep"))
		}
		if !entries[i].dir {
			regularFiles++
			if regularFiles > maxArchiveFiles || entries[i].file.UncompressedSize64 > maxArchiveFileBytes {
				return fail(fmt.Errorf("zip file budget exceeded"))
			}
		}
		filteredEntries = append(filteredEntries, entries[i])
	}
	entries = filteredEntries
	if len(entries) == 0 {
		return fail(fmt.Errorf("zip archive contains no files"))
	}
	if err := os.MkdirAll(destDirPath, 0755); err != nil {
		return fail(err)
	}
	root, err := filepathAbs(destDirPath)
	if err != nil {
		return fail(err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fail(fmt.Errorf("invalid extraction root"))
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fail(fmt.Errorf("invalid extraction root"))
	}
	var total uint64
	for _, item := range entries {
		destination := filepathJoin(root, item.name)
		if destination == "" || !pathWithin(root, destination) {
			return fail(fmt.Errorf("zip entry escapes destination"))
		}
		if item.dir {
			if err := os.MkdirAll(destination, 0755); err != nil {
				return fail(err)
			}
			if err := ensureNoSymlink(root, destination); err != nil {
				return fail(fmt.Errorf("zip entry escapes destination"))
			}
			continue
		}
		if err := os.MkdirAll(filepathDir(destination), 0755); err != nil {
			return fail(err)
		}
		if err := ensureNoSymlink(root, filepathDir(destination)); err != nil {
			return fail(fmt.Errorf("zip entry escapes destination"))
		}
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fail(err)
		}
		input, err := item.file.Open()
		if err != nil {
			_ = output.Close()
			return fail(err)
		}
		limited := io.LimitReader(input, int64(maxArchiveFileBytes)+1)
		written, copyErr := io.Copy(output, limited)
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil || closeInputErr != nil || closeOutputErr != nil {
			return fail(fmt.Errorf("extract zip entry %s failed", item.name))
		}
		if written > int64(maxArchiveFileBytes) || total > maxArchiveTotalBytes-uint64(written) {
			return fail(fmt.Errorf("zip file budget exceeded"))
		}
		total += uint64(written)
	}
	return true, ""
}

func normalizeArchiveEntry(file *zip.File) (string, bool, error) {
	name := strings.ReplaceAll(file.Name, "\\", "/")
	if name == "" || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") {
		return "", false, fmt.Errorf("invalid zip entry path")
	}
	if len(name) >= 2 && name[1] == ':' {
		return "", false, fmt.Errorf("invalid zip entry path")
	}
	parts := strings.Split(strings.TrimSuffix(name, "/"), "/")
	if len(parts) == 0 {
		return "", false, fmt.Errorf("invalid zip entry path")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false, fmt.Errorf("invalid zip entry path")
		}
	}
	dir := strings.HasSuffix(name, "/")
	mode := file.Mode()
	if mode&os.ModeSymlink != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeDevice != 0 || mode&os.ModeSocket != 0 {
		return "", false, fmt.Errorf("unsupported zip entry type")
	}
	if dir != (mode.IsDir() || strings.HasSuffix(file.Name, "/")) {
		return "", false, fmt.Errorf("invalid zip entry type")
	}
	return strings.Join(parts, "/"), dir, nil
}

func filepathAbs(value string) (string, error) { return filepath.Abs(value) }
func filepathJoin(root, name string) string    { return filepath.Join(root, filepath.FromSlash(name)) }
func filepathDir(value string) string          { return filepath.Dir(value) }
func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func ensureNoSymlink(root, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return fmt.Errorf("path outside root")
	}
	current := root
	if info, err := os.Lstat(current); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink root")
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path")
		}
	}
	return nil
}
