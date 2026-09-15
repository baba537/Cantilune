<p align="center">
  <img src="assets/cantilune-logo.png" width="160" alt="Cantilune">
</p>

<h1 align="center">Cantilune</h1>

<p align="center">
  Daily playlists for everyday situations, as a plugin for <a href="https://www.navidrome.org">Navidrome</a>
</p>

<p align="center">
  <a href="https://github.com/baba537/Cantilune/releases/latest"><img src="https://img.shields.io/github/v/release/baba537/Cantilune" alt="Release"></a>
  <a href="https://github.com/baba537/Cantilune/actions/workflows/ci.yml"><img src="https://github.com/baba537/Cantilune/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue" alt="License: GPL-3.0"></a>
</p>

Cantilune creates fresh playlists every day for situations such as working out, driving, cooking, studying or falling asleep. Each situation has a selectable preset, for example Hardstyle, HipHop or EDM for the gym. All songs come from your own library. The selection is based on metadata Navidrome already knows: tags such as genre, BPM and ReplayGain, plus favorites, ratings and listening history.

The name combines *canticle* (song) and *lune* (French for moon).

```
🎧 Gym Hardstyle ⚡
🎧 Driving 🚗
🎧 Cooking Jazz & Soul 🎷
🎧 Studying Lo-Fi ☕
🎧 Sleep Ambient 🌌
```

> [!NOTE]
> Cantilune was developed with the help of Claude Opus 5 (Anthropic). The code is covered by automated tests but may still contain bugs. Please report problems and suggestions as an [issue](https://github.com/baba537/Cantilune/issues).

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Settings](#settings)
- [Situations and presets](#situations-and-presets)
- [How songs are selected](#how-songs-are-selected)
- [Security and privacy](#security-and-privacy)
- [Stored data](#stored-data)
- [Performance](#performance)
- [Uninstalling](#uninstalling)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Project status](#project-status)
- [License](#license)

## Features

- 30 situations in six categories, each with several presets
- One playlist per situation, replaced every day; replaced playlists can be kept as archive
- One shared playlist for everybody, or a private playlist for every user based on that user's own listening data
- Four selection modes: Balanced, Favorites, Discover, Recently added
- Weighted selection based on genre, BPM, ReplayGain, mood, rating and listening history, with adjustable weights
- Transparent: the log explains why each song was chosen and reports quality metrics; a preview mode shows the result without changing playlists
- Learns from edits: songs removed from a playlist are avoided, songs added are preferred
- Coherent order: neighbouring songs are similar in genre, year and energy
- Variety across several days and at most three songs per artist (configurable)
- Genre matching on whole words: `Hip-Hop` = `Hip Hop`, `Hardstyle` also finds `Euphoric Hardstyle`, but `Dance` does not pick up `Dancehall`
- Custom situations with your own filters, configured in the web UI
- Playlist names in English or German
- No network access, no telemetry; releases with checksums, SBOM and build provenance

## Requirements

- Navidrome with plugin support (`.ndp` packages). Tested end-to-end in CI with **Navidrome 0.63.2 and 0.64.0**; older versions are not tested.
- Plugins enabled in the Navidrome configuration

## Installation

1. Download `cantilune.ndp` from the [latest release](https://github.com/baba537/Cantilune/releases/latest). To verify the download, see [Verifying a release](SECURITY.md#verifying-a-release).

2. Copy the file into Navidrome's plugin folder. When updating, replace the existing file; settings are kept.

   | Installation | Path |
   |---|---|
   | Docker | `/data/plugins/cantilune.ndp` |
   | Linux package | `/var/lib/navidrome/plugins/cantilune.ndp` |
   | General | `<DataFolder>/plugins/cantilune.ndp` |

   > [!IMPORTANT]
   > Do not rename the file. Navidrome derives the plugin ID from the file name.

3. Enable plugins if you have not done so yet, then restart Navidrome:

   ```toml
   # navidrome.toml
   [Plugins]
   Enabled = true
   ```

   With Docker you can use the environment variable `ND_PLUGINS_ENABLED=true` instead.

4. In the web UI, go to *Settings → Plugins → Cantilune*:
   1. Under *User access*, allow the users the plugin may act as, or allow all users.
   2. Choose your situations, save the settings and enable the plugin.

The playlists are created about 15 seconds after saving. After that, they are regenerated daily at the configured time.

The plugin can also be configured with the Navidrome CLI, which is useful for backups and automated setups:

```bash
navidrome plugin info cantilune --format json         # export the current settings
navidrome plugin edit cantilune --config-file settings.json
navidrome plugin enable cantilune
```

## Settings

All settings are made in the Navidrome web UI.

### General

| Setting | Default | Description |
|---|---|---|
| Prefix | `🎧` | Placed before every playlist name |
| Owner | empty | User who owns the playlists; empty means the first permitted admin |
| Tracks per playlist | `50` | 1 to 500, can be overridden per situation |
| Playlists for | Shared | **Shared:** one playlist per situation, owned by the owner and based on the owner's favorites and history. **Personal:** a private playlist for every permitted user, based on that user's own data. |
| Playlist language | English | Language of situation and preset names in playlist names (English or Deutsch) |
| Make playlists public | on | Shared playlists are visible to all users; personal playlists are always private |
| Show preset in playlist name | on | `🎧 Gym Hardstyle ⚡` instead of `🎧 Gym ⚡` |

### Schedule

| Setting | Default | Description |
|---|---|---|
| Time | `04:00` | Daily regeneration (server time) |
| Cron | empty | Overrides the time, e.g. `30 5 * * 1-5` for weekdays at 05:30 |
| Create missing or changed playlists right after saving | on | Unchanged playlists stay until the next scheduled run |

### Selection

| Setting | Default | Description |
|---|---|---|
| Max. per artist | `3` | Maximum songs per artist per playlist; `0` means unlimited |
| Avoid played | `3` days | Recently played songs are picked less often |
| Avoid repeats | `7` days | Songs from playlists of this period are picked less often |
| Skip short intros, skits and interludes | on | Skips tracks shorter than 2:30 with "Intro", "Skit", "Interlude" etc. in the title |
| Learn from playlist edits | on | Songs you remove from a Cantilune playlist are avoided for 60 days, songs you add are preferred |
| Never use these genres | Audiobook, Podcast, Comedy, Christmas and more | Applies to all playlists unless a preset explicitly includes the genre |

### Weights and transparency

| Setting | Default | Description |
|---|---|---|
| Genre fit | 100 % | How strongly exact genre matches are preferred |
| Tempo, energy, mood | 100 % | Influence of BPM, ReplayGain and mood tags |
| Favorites & ratings | 100 % | Influence of favorites, ratings, play counts and the selection mode |
| Variety | 100 % | How strongly recently played songs and recent playlists are avoided |
| Log why each song was chosen | off | Writes one log line per song with its weight and factors |
| Preview only | off | Performs the selection and logs the result, but does not change playlists |

A weight of 0 % ignores the factor group, 200 % doubles its effect. See [docs/algorithm.md](docs/algorithm.md#configurable-weights).

### Per situation

| Field | Description |
|---|---|
| Checkbox | Turns the situation on or off. Turning it off removes its playlist. |
| Preset | Music style for this situation |
| Selection | **Balanced**: favorites slightly preferred. **Favorites**: favorites, well-rated and frequently played songs. **Discover**: rarely or never played songs. **Recently added**: recently imported music. |
| Tracks | Empty means the global default |
| User | Empty means the owner from the general settings (ignored for personal playlists) |

### Custom situations

Additional playlists can be added under *Custom situations*. Available fields: name, emoji, genres, excluded genres, mood tags, BPM range, years, energy, flow, selection mode, an explicit content filter, track count and user.

### Cleanup

| Setting | Default | Description |
|---|---|---|
| Archive | 0 days | Keeps replaced playlists as private copies with the date in the name for this many days; 0 deletes them immediately |
| Delete all Cantilune playlists and pause the plugin | off | Use before uninstalling, see [Uninstalling](#uninstalling) |

## Situations and presets

Situations marked with ● are enabled after installation.

| Category | Situation | Presets |
|---|---|---|
| Sports & Fitness | ● Gym | Mix, Hardstyle, HipHop, EDM, Rock & Metal, Pop |
| | Running | Mix, Drum & Bass, Techno & House, Pop, Rock |
| | Cycling | Mix, Indie, Electronic, Rock |
| | Yoga & Meditation | Ambient, World Music, Downtempo, Classical |
| | Walking & Hiking | Mix, Folk, Indie, Acoustic |
| On the Road | ● Driving | Mix, Rock, Pop, HipHop, German, 80s & 90s |
| | Road Trip | Classics, Sing-Along, Indie, Country |
| | Commute | Mix, Lo-Fi, Indie, Electronic |
| | Night Drive | Synthwave, Deep House, Trip-Hop |
| At Home | ● Cooking | Mix, Jazz & Soul, Latin, Funk & Disco, Pop |
| | ● Mealtime | Jazz, Bossa & Lounge, Classical, Acoustic |
| | Breakfast | Acoustic, Soul, Indie Pop, Jazz |
| | Waking Up | Good Mood, Gentle, Power |
| | ● Cleaning | Mix, Pop Hits, Disco & Funk, Rock, 2000s |
| | Shower | Sing-Along, Power Ballads, German |
| | Garden & DIY | Mix, Rock, Country, Reggae |
| | Gaming | Soundtrack, Synthwave, Electronic, Metal |
| | Reading | Classical, Ambient, Jazz, Neoclassical |
| Focus | ● Studying | Mix, Lo-Fi, Classical, Ambient, Soundtrack |
| | ● Work & Focus | Mix, Electronic, Post-Rock, Lo-Fi, Classical |
| | Creative | Mix, Indie, Trip-Hop, Jazz |
| Relax & Sleep | ● Relax | Mix, Chillout, Acoustic, Reggae, Soul |
| | ● Sleep | Ambient, Piano, Classical, Nature Sounds |
| | Rainy Day | Melancholy, Jazz, Trip-Hop |
| Social & Mood | ● Party | Mix, Dance & EDM, HipHop & R&B, 2000s, 90s, Schlager & Party Hits |
| | Dinner Party | Jazz, Soul & Funk, Lounge, Latin |
| | BBQ & Summer | Mix, Reggae & Dancehall, Latin & Afro, Rock, HipHop |
| | Date Night | Soul & R&B, Jazz, Acoustic, Ballads |
| | With Kids | Kids Songs, Film & Musical, Feel-Good Pop |
| | Motivation | Epic, Rock, HipHop, Pop |

The *Mix* preset combines the genres of all other presets of a situation. The exact genres and filters are defined in [`catalog/presets.json`](catalog/presets.json).

## How songs are selected

Navidrome does not analyze audio. It does, however, read the tags of your music files and record usage data. Cantilune uses the following information:

| Information | Source | Use |
|---|---|---|
| Genre | Tag | Basis of the selection, matched against the genres in your library |
| Year | Tag | Decade presets such as 80s & 90s, order of songs |
| BPM | Tag | Songs with a suitable tempo are preferred; half and double values count as well |
| ReplayGain | Tag | Estimates how energetic a song is |
| Mood | Tag | Matching moods are preferred |
| Explicit | Tag | Excluded for situations such as Sleep and With Kids |
| Duration | File | Very short or long tracks and short intros are filtered out |
| Favorite, rating | Navidrome | Favorites and 4 to 5 stars are preferred; 1-star songs are never picked |
| Play count, last played | Navidrome | Recently played songs are picked less often |
| Date added | Navidrome | Basis of the Recently added mode |
| Your playlist edits | Cantilune | Removed songs are avoided, added songs preferred |

Steps for each playlist:

1. **Match genres.** Each genre of the preset is looked up in your library. Exact names and more specific sub-genres count (`Deep House` for `House`); genres that only share a word stem do not (`Dancehall` for `Dance`, `Reggaeton` for `Reggae`, `Hardcore Punk` for `Hardcore`).
2. **Load candidates.** Random songs are requested through the Subsonic API, about six times as many as needed. Every genre of the preset gets the same share, so a genre with many sub-genres does not crowd out the others.
3. **Filter.** Songs you removed from the playlist, excluded genres, songs whose genre tags do not fit, 1-star songs, unsuitable durations and short intros are removed. If too few songs remain, the duration and intro filters are relaxed.
4. **Weight.** Every song gets a weight from four adjustable groups: genre fit, tempo/energy/mood, favorites/ratings and variety. Songs you added to the playlist get an extra boost.
5. **Pick.** Songs are drawn at random according to their weight, with a limit per artist and an equal share for each genre of the preset.
6. **Order.** Rising energy (e.g. Gym, Party) or falling energy (e.g. Sleep, Yoga) if enough songs have BPM or ReplayGain tags; otherwise each song is followed by a similar one (genre, year, energy). Songs by the same artist never follow each other directly.

Without BPM, ReplayGain or mood tags, the selection relies on genre and listening history. These tags can be added with tools such as [beets](https://beets.io) or [MusicBrainz Picard](https://picard.musicbrainz.org). Rescan your library in Navidrome afterwards.

All factors, formulas and quality metrics are documented in [docs/algorithm.md](docs/algorithm.md).

### Why was this song chosen?

Enable *Log why each song was chosen*. Every song then gets a line in the Navidrome log:

```
🎧 Gym Hardstyle ⚡ for admin #1: Example Artist – Example Track [Hardstyle] weight 3.60 = genre 1.00 × tempo/energy/mood 2.00 × favorites/ratings 1.80 × variety 1.00 × edits 1.00
```

To try settings without changing any playlist, enable *Preview only* and save.

## Security and privacy

- **No network access:** the plugin requests no `http` or `websocket` permission, so it cannot contact external servers. There is no telemetry.
- **Minimal permissions:** `subsonicapi` (read songs, manage its own playlists), `users` (act for the users you permit), `scheduler` (daily run), `kvstore` (song IDs of earlier playlists). No file system access.
- **Only its own playlists:** Cantilune changes only playlists that carry its marker in the comment.
- **Shared vs. personal:** a shared playlist is based on the owner's favorites and listening history and is visible to everyone it is shared with. Use *Personal* if each user should only get playlists based on their own data.
- **Verifiable releases:** checksums, SBOM, signed build provenance and a reproducible build.

The permissions in detail, the threat model and how to report a vulnerability are described in [SECURITY.md](SECURITY.md).

## Stored data

All data stays in your Navidrome installation. Playlists are regular Navidrome playlists.

Cantilune uses Navidrome's key-value store (`plugins/cantilune/kvstore.db`). It stores only song IDs, separately for each situation and user:

| Data | Kept for |
|---|---|
| Songs of each generated playlist per day | *Avoid repeats* + 1 day |
| Songs of the current playlist, to detect your edits | 400 days |
| Songs you removed or added | 60 days |

The store is limited to 10 MB; with 10 situations and 10 users it holds about 1 MB.

Generated playlists are identified by a marker in the playlist comment (`#cl:<situation>:…`, archived playlists `#cla:…`). Other playlists are never modified, even if they use the same prefix.

## Performance

Measured with a simulated library of 100,000 songs and 10 users (100 playlists): Cantilune's own work takes about 0.5 seconds for the whole run and makes 1,720 Subsonic API calls. Each playlist uses at most 40 genre queries and a candidate pool of at most 1,500 songs. In the end-to-end tests, Navidrome kept answering requests within 2 ms while playlists were generated.

Measurement setup, limits and memory use: [docs/performance.md](docs/performance.md).

## Uninstalling

Navidrome does not notify plugins when they are removed. To avoid leftover playlists, clean up before removing the plugin:

1. In the plugin settings under *Cleanup*, enable *Delete all Cantilune playlists and pause the plugin* and save.
2. Wait about 15 seconds until `cleanup finished` appears in the log. Playlists, archives and stored data are removed.
3. Disable the plugin and delete `cantilune.ndp`. The folder `plugins/cantilune/` can be deleted as well.

## Troubleshooting

After each run, Cantilune writes one line per playlist to the Navidrome log:

```
🎧 Gym Hardstyle ⚡ for admin: 50 songs · Genres: Hardstyle (212), Euphoric Hardstyle (40) · not in library: Rawstyle, Gabber · candidates: 252 · filtered: duration 7, intro/skit 4 · metadata: 31 with BPM, 50 with ReplayGain, 9 favorites · quality: 86% exact genre, 94% different artists, 4% repeats, coherence 81%
```

| Log message | Cause and solution |
|---|---|
| `not in library: …` | These genres do not exist in your library; the others are still used. If none match, the log suggests similar genres. |
| `no matching songs … The previous playlist is kept.` | Choose a different preset or check your genre tags. |
| `filtered: genre mismatch …` | Navidrome returned songs whose genre tags do not fit the preset; they were skipped. |
| `user "…" is not permitted for the plugin` | Allow the user under *User access* in the plugin settings. |
| `personal playlists need at least one permitted user` | Allow users under *User access*, or switch *Playlists for* back to *Shared*. |
| `time "…" is invalid` or `cron expression … must have exactly 5 fields` | Use `HH:MM` or a five-field cron expression. Until corrected, `0 4 * * *` is used. |

To regenerate all playlists immediately, delete the Cantilune playlists and save the plugin settings again.

## Development

### Project structure

```
.
├── main.go              plugin entry points (OnInit, OnCallback)
├── config.go            reading settings, planning playlists
├── generator.go         creating, replacing, archiving and cleaning up playlists
├── selection.go         filtering, weighting, picking, ordering, quality metrics
├── genres.go            matching against library genres
├── history.go           history, edits and feedback in the key-value store
├── subsonic.go          Subsonic API calls
├── *_test.go            unit, integration, fuzz and benchmark tests
├── catalog/
│   ├── presets.json     situations and presets (English and German names)
│   ├── catalog.go       loading and resolving presets
│   ├── legacy.go        aliases for settings saved by versions before 1.1.0
│   └── manifest.go      settings schema and web UI layout, generates manifest.json
├── cmd/genmanifest/     manifest.json generator
├── scripts/e2e.sh       end-to-end test against a real Navidrome
├── docs/                algorithm, performance and testing
├── assets/              logo
└── manifest.json        generated, do not edit by hand
```

### Building and testing

Requires [Go](https://go.dev/dl/) 1.25 or later, [TinyGo](https://tinygo.org/getting-started/install/) 0.42.0 and `wasm-opt` from [Binaryen](https://github.com/WebAssembly/binaryen/releases) version 132.

```bash
make test        # unit and integration tests
make lint        # golangci-lint
make fuzz        # fuzz every target for 15 seconds
make bench       # benchmarks and load profile
make checksums   # build dist/cantilune.ndp and SHA256SUMS
```

CI runs all of these, `govulncheck`, CodeQL, a reproducible-build check and end-to-end tests against Navidrome 0.63.2 and 0.64.0. Details: [docs/testing.md](docs/testing.md).

### Adding presets

Situations and presets are maintained in [`catalog/presets.json`](catalog/presets.json). A preset is a single entry:

```json
{ "name": "Phonk", "nameDe": "Phonk", "emoji": "🚘", "genres": ["Phonk", "Drift Phonk"], "minBpm": 120, "maxBpm": 160 }
```

| Field | Meaning |
|---|---|
| `name`, `nameDe` | English and optional German name |
| `genres`, `excludeGenres` | Wanted and excluded genres |
| `moods` | Mood tags |
| `minBpm`, `maxBpm` | Tempo range |
| `fromYear`, `toYear` | Release years |
| `energy` | `low`, `medium` or `high` |
| `flow` | `shuffle`, `rising` or `falling` |
| `minDuration`, `maxDuration` | Duration in seconds |
| `excludeExplicit` | Exclude explicit songs |
| `mix` | Combine the genres of all other presets |

Values under `defaults` apply to all presets of a situation. After a change, regenerate the manifest; CI checks that both files match.

```bash
go generate ./...
```

Presets are stored in the settings by their display name. If a preset is renamed, existing installations fall back to the first preset of that situation unless the old name is kept as `nameDe` or an alias.

### Releases

Pushing a tag in the format `v*` starts the [release workflow](.github/workflows/release.yml). It runs the complete CI first, then builds the plugin, takes the version from the tag and attaches `cantilune.ndp`, `SHA256SUMS` and an SPDX SBOM to the release together with a signed build provenance attestation.

```bash
git tag -a v1.3.0 -m "Cantilune 1.3.0"
git push origin v1.3.0
```

Changes between versions are listed in the [changelog](CHANGELOG.md).

## Project status

Cantilune is maintained in free time by [@baba537](https://github.com/baba537). Bug reports, preset suggestions and pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md) and the [code of conduct](CODE_OF_CONDUCT.md). No external security audit has been performed.

## License

Cantilune is licensed under the [GPL-3.0](LICENSE). The Navidrome Plugin Development Kit, which is compiled into `plugin.wasm`, is also licensed under the GPL-3.0.

The logo was created with ChatGPT.
