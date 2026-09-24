# Changelog

All notable changes to Cantilune. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows [Semantic Versioning](https://semver.org/).

## [1.3.2] – 2026-09-24

### Added
- The log line at the end of a run shows how long the run took.
- End-to-end tests now update from the previous release to the new build and use a library of 1,590 songs; Navidrome 0.64.1 is tested.

### Changed
- The package is built by a small Go program (`cmd/buildndp`) instead of `zip` and `jq`, so it can be built and reproduced on any platform with Go and TinyGo.
- The manifest names "Cantilune contributors" as author.
- Updated GitHub Actions and the Navidrome plugin PDK.

### Documentation
- Updating: Navidrome disables a plugin whose file has changed. Settings are kept; enable Cantilune again after replacing the file.
- Why Windows builds are not byte-identical to Linux builds (different LLVM versions in the TinyGo release archives).

## [1.3.1] – 2026-09-19

### Fixed
- Settings saved by version 1.0.0 contained German preset and selection values. The web UI marked them as invalid, did not fill in defaults and could not be saved until every field was changed by hand. Preset and selection are now stored under new field names, so the form starts from valid defaults. Old values are still used until the settings are saved again.

### Changed
- After updating, the preset and selection of every situation show their defaults in the web UI. Choices that differ from the defaults need to be selected again once; enabled situations, track counts and users are kept.

## [1.3.0] – 2026-09-15

### Added
- *Playlists for*: one shared playlist per situation, or a private playlist for every permitted user based on that user's own favorites and listening history. Stored data is kept separately per user.
- *Learn from playlist edits*: songs removed from a Cantilune playlist are avoided for 60 days, songs added are preferred.
- *Log why each song was chosen*: one log line per song with its weight and factor groups.
- *Preview only*: runs the selection and logs the result without changing playlists.
- Adjustable weights for genre fit, tempo/energy/mood, favorites/ratings and variety.
- *Archive*: replaced playlists can be kept as private, dated copies for up to 30 days.
- *Playlist language*: situation and preset names in English or German.
- Quality metrics in the log: exact genre share, artist variety, repeats and coherence.
- Security policy with threat model and permission overview, contribution guidelines, code of conduct.
- Documentation of the algorithm, performance measurements and test strategy in `docs/`.
- Releases include `SHA256SUMS`, an SPDX SBOM and a signed build provenance attestation; the build is reproducible.
- CI: golangci-lint, CodeQL, govulncheck, fuzzing, a load profile with 100,000 songs and end-to-end tests against Navidrome 0.63.2 and 0.64.0. Dependabot keeps dependencies and actions up to date.

### Changed
- Songs follow each other by similarity in genre, year and energy instead of pure random order when a playlist has no rising or falling flow.
- Every candidate's genre tags are verified, because Navidrome filters genres with SQL `LIKE`.
- At most 40 genre queries per playlist.
- Releases are built with the latest Go 1.26 patch release instead of Go 1.25.0.

### Fixed
- Genre normalization created a string replacer on every comparison; generating a playlist from 100,000 songs is now about nine times faster and allocates about 40 times less memory.

## [1.2.0] – 2026-09-14

### Fixed
- Genre matching compared parts of words, so presets picked up unrelated genres: `Dance` matched `Dancehall`, `Reggae` matched `Reggaeton`, `Hardcore` matched `Hardcore Punk`, `Classical` matched `Neoclassical Metal`, `Folk` and `Drone` matched their metal variants. Genres are now compared as whole words; only real sub-genres such as `Deep House` or `Euphoric Hardstyle` count.
- A genre with many sub-genres in the library (e.g. House) could dominate a preset. Every genre of a preset now gets an equal share of the candidates and of the final playlist.
- Excluded genres are also matched as whole words (`Christmas` excludes `Christmas Pop`).

### Changed
- Genre fit is part of the song weight: songs tagged exactly with a wanted genre, and songs without many unrelated genre tags, are preferred.
- BPM and energy estimates have less influence, so missing or inaccurate tags no longer push songs out of a playlist.

## [1.1.0] – 2026-09-14

### Changed
- The plugin is now fully in English: settings page, situation and preset names, playlist names, playlist comments and log messages
- Code comments and tests are in English

### Compatibility
- Settings saved with 1.0.0 keep working: German preset names, selection modes, energy and flow values are mapped to the new English values
- Existing playlists are renamed automatically the next time the settings are saved or at the next scheduled run

## [1.0.0] – 2026-09-14

First public release.

### Features
- 30 situations in six categories, each with several presets, defined in `catalog/presets.json`
- One playlist per situation, replaced every day
- Selection modes: balanced, favorites, discover, recently added
- Weighted selection based on genre, year, BPM (including half and double values), ReplayGain, mood, rating, play count and date added
- Tolerant genre matching with suggestions for missing genres in the log
- Per-artist limit and filters for duration, intros and explicit content
- Energy flow per situation: random, rising or falling
- History of recent days in the key-value store to reduce repeats
- Custom situations with user-defined filters
- Schedule by time of day or cron expression, with immediate update after saving
- Option to delete all generated playlists before uninstalling

[1.3.2]: https://github.com/baba537/Cantilune/releases/tag/v1.3.2
[1.3.1]: https://github.com/baba537/Cantilune/releases/tag/v1.3.1
[1.3.0]: https://github.com/baba537/Cantilune/releases/tag/v1.3.0
[1.2.0]: https://github.com/baba537/Cantilune/releases/tag/v1.2.0
[1.1.0]: https://github.com/baba537/Cantilune/releases/tag/v1.1.0
[1.0.0]: https://github.com/baba537/Cantilune/releases/tag/v1.0.0
