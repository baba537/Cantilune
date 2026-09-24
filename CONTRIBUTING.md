# Contributing

Bug reports, suggestions and pull requests are welcome. By participating you agree to follow the [code of conduct](CODE_OF_CONDUCT.md).

## Reporting bugs and suggesting presets

- **Bugs:** use the [bug report form](https://github.com/baba537/Cantilune/issues/new?template=bug_report.yml). The log lines starting with `🎧` from the Navidrome log help most. Enable *Log why each song was chosen* for details.
- **Presets and situations:** use the [suggestion form](https://github.com/baba537/Cantilune/issues/new?template=feature_request.yml) and list genres as they appear in real libraries.
- **Security problems:** do not open an issue; see [SECURITY.md](SECURITY.md).

## Development setup

Requirements:

- [Go](https://go.dev/dl/) 1.25 or later (CI uses the latest 1.26 release)
- [TinyGo](https://tinygo.org/getting-started/install/) 0.42.0 and `wasm-opt` from [Binaryen](https://github.com/WebAssembly/binaryen/releases) version 132, only for building the plugin
- [golangci-lint](https://golangci-lint.run/) v2.13 for linting
- `make` (optional; every target is a single command you can also run directly)

| Command | Purpose |
|---|---|
| `go generate ./...` | Regenerate `manifest.json` from `catalog/presets.json` |
| `make test` | Unit and integration tests |
| `make lint` | Static analysis |
| `make fuzz` | Run every fuzz target for 15 seconds |
| `make bench` | Benchmarks and the load profile (100,000 songs, 10 users) |
| `make checksums` | Build `dist/cantilune.ndp` and write checksums |
| `bash scripts/e2e.sh 0.64.0` | End-to-end test against a real Navidrome (Linux, needs `ffmpeg`, `curl`, `jq`, `python3`) |

See [docs/testing.md](docs/testing.md) for what each test layer covers.

## Pull requests

1. Keep changes focused. Open an issue first for larger changes.
2. Add or update tests for changed behaviour.
3. Run `go generate ./...`, `make test` and `make lint`.
4. Update `README.md`, `docs/` and `CHANGELOG.md` (section *Unreleased*) when behaviour or settings change.
5. CI must pass: lint, tests, fuzzing, vulnerability check, reproducible build and the end-to-end tests against Navidrome.

Code, comments, documentation and commit messages are written in English. Format code with `gofmt`.

### Presets

Situations and presets live only in [`catalog/presets.json`](catalog/presets.json). After a change run `go generate ./...`; CI fails if `manifest.json` is out of date. Preset names are stored in user settings, so renaming a preset needs an alias in `catalog/catalog.go` or `catalog/legacy.go`.

### Settings

New settings are defined in `catalog/manifest.go`. They must have a safe default, work for existing installations without saving the settings again, and be covered by the settings fuzz test.

## License

Contributions are accepted under the [GPL-3.0](LICENSE), the license of the project.

## Maintenance

Cantilune is maintained by [@baba537](https://github.com/baba537) in free time. There is no funding and no guaranteed response time.

The project does not lock users in: playlists are regular Navidrome playlists, all data stays in the user's Navidrome installation, and the [uninstall option](README.md#uninstalling) removes everything the plugin created. Because the code is licensed under the GPL-3.0, anyone can fork and continue the project. If Cantilune is no longer maintained, the repository will be archived with a note in the README.
