// genmanifest generates manifest.json from catalog/presets.json.
//
// Run from the repository root: go generate ./...
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
	if err := os.WriteFile("manifest.json", data, 0o644); err != nil { //nolint:gosec // manifest.json is a public source file of the repository

		fmt.Fprintln(os.Stderr, "genmanifest:", err)
		os.Exit(1)
	}
	fmt.Println("manifest.json updated")
}
