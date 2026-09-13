// genmanifest erzeugt manifest.json aus catalog/presets.json.
//
// Aufruf im Repo-Wurzelverzeichnis: go generate ./...
package main

import (
	"fmt"
	"os"

	"cantilune/catalog"
)

func main() {
	data, err := catalog.BuildManifest()
	if err != nil {
		fmt.Fprintln(os.Stderr, "genmanifest:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("manifest.json", data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "genmanifest:", err)
		os.Exit(1)
	}
	fmt.Println("manifest.json aktualisiert")
}
