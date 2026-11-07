package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateObjectSafety(t *testing.T) {
	h := setup(t)
	key, e := h.a.save([]byte("private evidence"))
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(h.a.Config.DataDir, key)
	info, e := os.Stat(p)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe permissions", info, e)
	}
	raw, e := h.a.load(key, 20)
	if e != nil || string(raw) != "private evidence" {
		t.Fatal(string(raw), e)
	}
	if _, e = h.a.load(key, 2); e == nil {
		t.Fatal("size limit bypass")
	}
	if _, e = h.a.load("../outside", 20); e == nil {
		t.Fatal("path traversal")
	}
	link := filepath.Join(h.a.Config.DataDir, "link")
	if e = os.Symlink(p, link); e != nil {
		t.Fatal(e)
	}
	if _, e = h.a.load("link", 20); e == nil {
		t.Fatal("symlink accepted")
	}
}
