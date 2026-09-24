package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackageIsReproducible(t *testing.T) {
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "plugin.wasm"), []byte("\x00asm test"), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func() []byte {
		if err := run(dist, "9.9.9", "https://example.org", "1758240000"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dist, "cantilune.ndp"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := build()
	if second := build(); !bytes.Equal(first, second) {
		t.Fatal("two builds of the same input differ")
	}

	zr, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 || zr.File[0].Name != "manifest.json" || zr.File[1].Name != "plugin.wasm" {
		t.Fatalf("unexpected entries: %v", zr.File)
	}
	for _, f := range zr.File {
		if !f.Modified.Equal(time.Unix(1758240000, 0)) {
			t.Errorf("%s: modification time %v", f.Name, f.Modified)
		}
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Version, Website string }
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != "9.9.9" || m.Website != "https://example.org" {
		t.Errorf("manifest not updated: %+v", m)
	}
}
