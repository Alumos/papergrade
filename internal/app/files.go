package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Durable private object before database reference. Random basename never uses user input.
func (a *App) save(raw []byte) (string, error) {
	f, e := os.CreateTemp(a.Config.DataDir, ".upload-")
	if e != nil {
		return "", e
	}
	tmp := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	if e = f.Chmod(0600); e != nil {
		return "", e
	}
	if _, e = f.Write(raw); e != nil {
		return "", e
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	name := ID()
	target := filepath.Join(a.Config.DataDir, name)
	if e = os.Rename(tmp, target); e != nil {
		return "", e
	}
	d, e := os.Open(a.Config.DataDir)
	if e != nil {
		return "", e
	}
	defer d.Close()
	if e = d.Sync(); e != nil {
		return "", e
	}
	ok = true
	return name, nil
}
func (a *App) load(name string, limit int64) ([]byte, error) {
	if name == "" || filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid object key")
	}
	path := filepath.Join(a.Config.DataDir, name)
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid object")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !os.SameFile(info, actual) {
		return nil, fmt.Errorf("object changed during open")
	}
	raw, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("object exceeds limit")
	}
	return raw, e
}
func (a *App) serveFile(w http.ResponseWriter, r *http.Request, name string, limit int64, contentType string) error {
	if name == "" || filepath.Base(name) != name {
		return fmt.Errorf("invalid object key")
	}
	path := filepath.Join(a.Config.DataDir, name)
	before, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !before.Mode().IsRegular() || before.Size() > limit {
		return fmt.Errorf("invalid object")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	after, e := f.Stat()
	if e != nil {
		return e
	}
	if !os.SameFile(before, after) {
		return fmt.Errorf("object changed during open")
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, "", after.ModTime(), f)
	return nil
}
func upload(w http.ResponseWriter, r *http.Request, field string, limit int64) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit+(64<<10))
	if e := r.ParseMultipartForm(1 << 20); e != nil {
		return nil, "", bad("invalid multipart or upload too large")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, h, e := r.FormFile(field)
	if e != nil {
		return nil, "", bad("multipart " + field + " file required")
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, "", e
	}
	if len(raw) == 0 || int64(len(raw)) > limit {
		return nil, "", &APIError{413, "FILE_TOO_LARGE", "file outside size limit"}
	}
	name := filepath.Base(h.Filename)
	if len(name) > 255 {
		return nil, "", bad("filename too long")
	}
	return raw, name, nil
}
