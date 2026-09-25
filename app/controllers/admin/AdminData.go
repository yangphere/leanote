package admin

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

// 数据管理, 备份和恢复
type AdminData struct {
	AdminBaseController
}

func (c AdminData) Index() revel.Result {
	backups := configService.GetGlobalArrMapConfig("backups")
	backups2 := make([]map[string]string, len(backups))
	for i := range backups {
		backups2[len(backups)-1-i] = backups[i]
	}
	c.ViewArgs["backups"] = backups2
	return c.RenderTemplate("admin/data/index.html")
}

func (c AdminData) Backup() revel.Result {
	re := info.NewRe()
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = configService.Backup("")
	return c.RenderJSON(re)
}

func (c AdminData) Restore(createdTime string) revel.Result {
	re := info.Re{}
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = configService.Restore(createdTime)
	return c.RenderJSON(re)
}

func (c AdminData) Delete(createdTime string) revel.Result {
	re := info.Re{}
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = configService.DeleteBackup(createdTime)
	return c.RenderJSON(re)
}

func (c AdminData) UpdateRemark(createdTime, remark string) revel.Result {
	re := info.Re{}
	re.Ok, re.Msg = configService.UpdateBackupRemark(createdTime, remark)
	return c.RenderJSON(re)
}

func (c AdminData) Download(createdTime string) revel.Result {
	fail := func() revel.Result {
		c.Response.Status = http.StatusInternalServerError
		return c.RenderText("backup download unavailable")
	}
	backup, ok := configService.GetBackup(createdTime)
	if !ok || revel.Config == nil || revel.BasePath == "" {
		return fail()
	}
	dbname, _ := revel.Config.String("db.dbname")
	root := filepath.Join(revel.BasePath, "mongodb_backup")
	registered, err := service.ValidateContainedPath(root, backup["path"])
	if err != nil {
		return fail()
	}
	path, err := service.ValidateContainedPath(root, filepath.Join(registered, dbname))
	if err != nil {
		return fail()
	}
	allFiles, _, err := service.StableBackupPaths(path, service.DefaultBackupLimits)
	if err != nil || len(allFiles) == 0 {
		return fail()
	}
	opaque := backup["createdTime"]
	if service.ValidateOpaqueBackupID(opaque) != nil {
		return fail()
	}
	if filepath.Base(dbname) != dbname || dbname == "" || strings.ContainsAny(dbname, "\\/\x00\r\n") {
		return fail()
	}
	filename := "backup_" + dbname + "_" + opaque + ".tar.gz"
	filesRoot := filepath.Join(revel.BasePath, "files")
	if err := os.MkdirAll(filesRoot, 0700); err != nil {
		return fail()
	}
	fw, err := os.CreateTemp(filesRoot, ".backup-*.tar.gz")
	if err != nil {
		return fail()
	}
	tempName := fw.Name()
	cleanup := func() {
		_ = fw.Close()
		_ = os.Remove(tempName)
	}
	gz := gzip.NewWriter(fw)
	tw := tar.NewWriter(gz)
	for _, relative := range allFiles {
		fullPath, joinErr := service.ValidateContainedPath(path, filepath.Join(path, filepath.FromSlash(relative)))
		if joinErr != nil {
			cleanup()
			return fail()
		}
		fr, openErr := os.Open(fullPath)
		if openErr != nil {
			cleanup()
			return fail()
		}
		fileInfo, statErr := fr.Stat()
		if statErr != nil || !fileInfo.Mode().IsRegular() {
			_ = fr.Close()
			cleanup()
			return fail()
		}
		header := &tar.Header{Name: relative, Size: fileInfo.Size(), Mode: int64(fileInfo.Mode()), ModTime: fileInfo.ModTime()}
		if err := tw.WriteHeader(header); err != nil {
			_ = fr.Close()
			cleanup()
			return fail()
		}
		if _, err := io.Copy(tw, fr); err != nil {
			_ = fr.Close()
			cleanup()
			return fail()
		}
		_ = fr.Close()
		if stat, statErr := fw.Stat(); statErr != nil || stat.Size() > service.DefaultBackupLimits.MaxArchiveBytes {
			cleanup()
			return fail()
		}
	}
	if err := tw.Close(); err != nil {
		cleanup()
		return fail()
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return fail()
	}
	if err := fw.Close(); err != nil {
		_ = os.Remove(tempName)
		return fail()
	}
	if stat, statErr := os.Stat(tempName); statErr != nil || stat.Size() > service.DefaultBackupLimits.MaxArchiveBytes {
		_ = os.Remove(tempName)
		return fail()
	}
	file, err := os.Open(tempName)
	if err != nil {
		_ = os.Remove(tempName)
		return fail()
	}
	// BinaryResult owns the reader until the response is applied. The temp
	// archive is removed by the response wrapper after the reader is closed.
	return &cleanupBinaryResult{Result: c.RenderBinary(file, filename, revel.Attachment, time.Now()), path: tempName}
}

type cleanupBinaryResult struct {
	revel.Result
	path string
}

func (r *cleanupBinaryResult) Apply(req *revel.Request, resp *revel.Response) {
	r.Result.Apply(req, resp)
	_ = os.Remove(r.path)
}
