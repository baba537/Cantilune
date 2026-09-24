# Testing and quality checks

Every push and pull request runs the [CI workflow](../.github/workflows/ci.yml). The [release workflow](../.github/workflows/release.yml) runs the same workflow first and only publishes a release if all jobs pass.

| Layer | What it covers | Where | Command |
|---|---|---|---|
| Unit tests | Genre matching, weights, BPM and energy, markers, settings and legacy values, quality metrics | `main_test.go`, `features_test.go`, `catalog/catalog_test.go` | `make test` |
| Integration tests | Complete runs against a simulated Subsonic server: playlist creation and replacement, cleanup, personal playlists, preview, archive, learning from edits, explanations, German names | `main_test.go`, `features_test.go` | `make test` |
| Edge cases | Empty library, songs without any tags, special characters in user and genre names, duplicate songs across genres, genre names with different case, SQL `LIKE` wildcards | `features_test.go` | `make test` |
| Race detector | Data races | CI | `go test -race ./...` |
| Fuzzing | Genre normalization and matching, playlist markers, settings parsing, Subsonic responses | `fuzz_test.go` | `make fuzz` |
| Load profile | 100,000 songs, 10 users, 100 playlists; duration, API calls, memory | `bench_test.go` | `make bench` |
| End-to-end | Real Navidrome 0.63.2 and 0.64.1, fresh install and update (see below) | `scripts/e2e.sh` | `bash scripts/e2e.sh 0.64.1` |
| Static analysis | `golangci-lint` with `govet`, `staticcheck`, `errcheck`, `gosec` and more, for the native and the WebAssembly target | `.golangci.yml` | `make lint` |
| Code scanning | CodeQL | `.github/workflows/codeql.yml` | – |
| Vulnerabilities | Known vulnerabilities in dependencies and the Go standard library | CI | `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` |
| Consistency | `go.mod` is tidy, `manifest.json` matches `catalog/presets.json` | CI | `go mod tidy -diff`, `go generate ./...` |
| Reproducible build | The package is built twice from a clean cache; checksums must match | CI | `make checksums` |

## End-to-end test

[`scripts/e2e.sh`](../scripts/e2e.sh) runs on Linux with a real Navidrome binary:

1. Downloads the Navidrome release and generates 90 tagged MP3 files with `ffmpeg` (Hardstyle, Euphoric Hardstyle, Hip-Hop, Classical, Dancehall, Audiobook) plus 1,500 songs of ten unrelated genres, so that timings reflect a library of realistic size.
2. Starts Navidrome, creates the admin user and scans the library.
3. Installs `cantilune.ndp` and configures it with the Navidrome CLI (`navidrome plugin edit --all-users --config …` with the settings as JSON, then `navidrome plugin enable`).
4. Restarts Navidrome and waits until the plugin reports that generation finished, while measuring Navidrome's response time. The plugin log line includes how long the run took inside the WebAssembly runtime.
5. Checks through the Subsonic API that the three expected playlists exist exactly once, are public, carry the Cantilune marker, contain 5 to 10 songs and only songs of the expected genres (for example no Dancehall in a Hardstyle playlist).
6. Checks that the explanation log lines exist and that the configuration can be exported with `navidrome plugin info cantilune --format json`.
7. Enables the cleanup option and checks that all Cantilune playlists are removed.

**Update test:** with a third argument (`scripts/e2e.sh 0.64.1 dist/cantilune.ndp previous.ndp`), the previous version is installed and configured first. After its playlists exist, the package file is replaced by the new build and Navidrome is restarted, the way users update. The test checks that the plugin stays enabled, the settings are kept and every playlist still exists exactly once. CI runs this with the latest published release.

The matrix covers Navidrome 0.63.2 and 0.64.1 plus the update test on 0.64.1. Navidrome 0.62 and older lack the plugin CLI commands the test relies on, so they are not tested.

## Not covered

- The web UI settings form is not tested automatically. It was checked manually in a browser with Navidrome 0.64.1 (settings saved by 1.0.0 load without validation errors, saving regenerates only changed playlists).
- Very large real libraries: the load profile uses a simulated server, so Navidrome's own query time at 100,000 songs is not measured.
- No external security audit (see [SECURITY.md](../SECURITY.md)).
