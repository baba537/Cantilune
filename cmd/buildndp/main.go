// buildndp builds dist/cantilune.ndp from dist/plugin.wasm and the manifest.
//
// The archive is reproducible: entries are written in a fixed order with the
// commit time as modification date, so the same commit gives the same file on
// every platform. The file name is the plugin ID in Navidrome and must stay
// cantilune.ndp.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"cantilune/catalog"
)

func main() {
	dist := flag.String("dist", "dist", "directory with plugin.wasm, receives the package")
	version := flag.String("version", catalog.DevVersion, "plugin version written to the manifest")
	website := flag.String("website", catalog.PluginWebsite, "website written to the manifest")
	epoch := flag.String("epoch", os.Getenv("SOURCE_DATE_EPOCH"), "modification time of the entries (Unix seconds)")
	flag.Parse()

	if err := run(*dist, *version, *website, *epoch); err != nil {
		fmt.Fprintln(os.Stderr, "buildndp:", err)
		os.Exit(1)
	}
}

func run(dist, version, website, epoch string) error {
	if version == "" {
		version = catalog.DevVersion
	}
	if website == "" {
		website = catalog.PluginWebsite
	}
	modified := time.Unix(0, 0).UTC()
	if epoch != "" {
		secs, err := strconv.ParseInt(epoch, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid epoch %q: %w", epoch, err)
		}
		modified = time.Unix(secs, 0).UTC()
	}

	manifest, err := catalog.BuildManifestFor(version, website)
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(filepath.Join(dist, "plugin.wasm")) //nolint:gosec // build directory chosen by the developer
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dist, "manifest.json"), manifest, 0o644); err != nil { //nolint:gosec // build output, not a secret
		return err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct {
		name string
		data []byte
	}{{"manifest.json", manifest}, {"plugin.wasm", wasm}} {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: modified}
		h.SetMode(0o644)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := w.Write(f.data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	out := filepath.Join(dist, "cantilune.ndp")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil { //nolint:gosec // release artifact, meant to be public
		return err
	}
	fmt.Printf("Package created: %s (version %s)\n", out, version)
	return nil
}
