# Security

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub: open the [Security tab](https://github.com/baba537/Cantilune/security) of this repository and choose *Report a vulnerability*. Do not open a public issue for security problems.

Cantilune is maintained in free time. Reports are handled as soon as possible; there is no guaranteed response time. Vulnerabilities in Navidrome itself or in its plugin sandbox should be reported to the [Navidrome project](https://github.com/navidrome/navidrome/security).

Only the latest release receives security fixes.

## What the plugin can and cannot do

Navidrome runs plugins as WebAssembly in a sandbox. A plugin can only use the host services it declares in its manifest, and an administrator has to approve the users it may act for.

### Requested permissions

| Permission | Why | What Cantilune does with it |
|---|---|---|
| `subsonicapi` | Read songs and manage playlists | Calls only `getGenres`, `getRandomSongs`, `getSong`, `getPlaylists`, `getPlaylist`, `createPlaylist`, `updatePlaylist` and `deletePlaylist` (see [`subsonic.go`](subsonic.go)). |
| `users` | Act on behalf of playlist owners | Reads the list of users the administrator permitted. Navidrome does not expose passwords or e-mail addresses to plugins. |
| `scheduler` | Daily regeneration | Registers one recurring job and one-time jobs after saving the settings. |
| `kvstore` | Avoid repeats, learn from edits | Stores song IDs per situation and user. Limited to 10 MB, all entries expire (see [Stored data](README.md#stored-data)). |

Navidrome grants Subsonic API access per user, not per endpoint. The list above is what the code calls; it is not enforced by Navidrome.

### Not requested

| Permission | Consequence |
|---|---|
| `http`, `websocket` | The plugin has **no network access**. It cannot contact external servers, send telemetry or load code at runtime. |
| `library` (including file system access) | The plugin cannot read music files or folders. It only sees metadata returned by the Subsonic API. |
| `cache`, `storage`, `artwork`, `matcher`, `taskqueue`, `scrobbleRetriever` | Not used. |

Cantilune contains no telemetry, analytics or update checks.

### Playlists it touches

Cantilune only changes or deletes playlists that carry its marker in the comment (`#cl:` for current playlists, `#cla:` for archived ones, `#nb:` for playlists of the predecessor NaviBeat) and that belong to a user the administrator permitted. Playlists created by people are never modified, even if they use the same name prefix.

## Threat model

### Assets

- Listening data of users: favorites, ratings, play counts, play dates
- Playlists of users
- Plugin settings and the data in the key-value store (song IDs only)
- The released `cantilune.ndp` package

### Trust boundaries

1. **Navidrome ↔ plugin:** Navidrome enforces permissions and the sandbox. Cantilune trusts the Subsonic API responses to be well-formed, but still validates them.
2. **Music library ↔ plugin:** Tags come from arbitrary files and can contain unexpected content.
3. **Administrator settings ↔ plugin:** Settings are entered in the web UI or with the Navidrome CLI.
4. **Build and release pipeline ↔ users:** Users install a binary built by GitHub Actions.

### Threats and mitigations

| Threat | Mitigation |
|---|---|
| Crafted tags (very long values, invalid UTF-8, SQL wildcards such as `%` or `_` in genre names) produce wrong selections or crashes | Tags are treated as data only. Every song's genres are verified after the query, because Navidrome filters genres with SQL `LIKE`. Genre matching, marker parsing and response parsing are fuzz tested. |
| Malformed settings (invalid JSON, out-of-range numbers, unknown values) | Invalid values fall back to defaults, numbers are clamped to their allowed ranges. Settings parsing is fuzz tested. |
| The plugin deletes or changes playlists it does not own | Only playlists with a Cantilune marker and a permitted owner are changed. Covered by tests. |
| Listening data of one user becomes visible to others | In *Shared* mode the public playlist reflects the owner's favorites and history; this is documented next to the setting. *Personal* mode creates private playlists from each user's own data and keeps stored data separate per user. |
| Data leaves the server | No network permissions (see above). |
| Very large libraries exhaust memory or block Navidrome | At most 40 genre queries and 500 songs per query per playlist, candidate pool limited to 1,500 songs, key-value store limited to 10 MB. See [docs/performance.md](docs/performance.md). |
| A dependency or the toolchain contains a known vulnerability | Dependabot updates Go modules and GitHub Actions weekly. `govulncheck` runs on every push. Builds use the latest Go 1.26 patch release. |
| Unsafe code patterns | `golangci-lint` (including `gosec`) and CodeQL run on every push. |
| A tampered release package | Releases include SHA-256 checksums, an SBOM and a signed build provenance attestation. Builds are reproducible (see below). |

### Out of scope

- Vulnerabilities in Navidrome, its plugin runtime or the Subsonic API implementation
- Administrators who grant the plugin access to users on purpose
- Physical or operating system access to the server

## Verifying a release

Every release contains `cantilune.ndp`, `SHA256SUMS` and `cantilune.spdx.json` (software bill of materials).

**Checksum:**

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

**Build provenance** (requires the [GitHub CLI](https://cli.github.com)): proves that the file was built by the release workflow of this repository from the tagged commit.

```bash
gh attestation verify cantilune.ndp -R baba537/Cantilune
```

**Rebuild from source:** the build is reproducible with the pinned toolchain on Linux x86-64. CI builds every commit twice from a clean cache and compares the checksums.

```bash
git checkout v1.3.2
make checksums VERSION=1.3.2 WEBSITE=https://github.com/baba537/Cantilune
```

Use the Go version named in the attestation (the latest Go 1.26 patch release at release time), TinyGo 0.42.0 and Binaryen version 132. `dist/plugin.wasm` and `dist/cantilune.ndp` then match the checksums in `SHA256SUMS`. The package is written by `cmd/buildndp` with fixed entry order and dates, so it does not depend on the installed `zip`.

Builds on Windows or macOS work, but are not byte-identical to the Linux build. Clang/LLVM and TinyGo report the same versions on both platforms; the difference is introduced in the Binaryen (`wasm-opt`) step. The Windows file declares two additional WebAssembly features (`bulk-memory-opt`, `call-indirect-overlong`) in its `target_features` section, and fewer functions are merged. The CI log names the exact `wasm-opt` it used.

## Security reviews

No external security audit has been performed. The measures above are automated checks and tests. Reviews and reports from the community are welcome, publicly through issues or privately as described at the top.
