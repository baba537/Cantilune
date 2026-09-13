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
- [Stored data](#stored-data)
- [Uninstalling](#uninstalling)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [License](#license)

## Features

- 30 situations in six categories, each with several presets
- One playlist per situation, replaced every day
- Four selection modes: Balanced, Favorites, Discover, Recently added
- Weighted selection based on genre, BPM, ReplayGain, mood, rating and listening history
- Variety across several days and at most three songs per artist (configurable)
- Tolerant genre matching, e.g. `Hip-Hop` = `Hip Hop`, and `Hardstyle` also finds `Euphoric Hardstyle`
- Custom situations with your own filters, configured in the web UI
- Schedule by time of day or cron expression
- Public playlists owned by a user of your choice

## Requirements

- Navidrome with plugin support (`.ndp` packages). Developed and tested with Navidrome 0.64.
- Plugins enabled in the Navidrome configuration

## Installation

1. Download `cantilune.ndp` from the [latest release](https://github.com/baba537/Cantilune/releases/latest).

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

## Settings

### General

| Setting | Default | Description |
|---|---|---|
| Prefix | `🎧` | Placed before every playlist name |
| Owner | empty | User who owns the playlists; empty means the first permitted admin |
| Tracks per playlist | `50` | 1 to 500, can be overridden per situation |
| Make playlists public | on | Playlists are visible to all users |
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
| Never use these genres | Audiobook, Podcast, Comedy, Christmas and more | Applies to all playlists unless a preset explicitly includes the genre |

### Per situation

| Field | Description |
|---|---|
| Checkbox | Turns the situation on or off. Turning it off removes its playlist. |
| Preset | Music style for this situation |
| Selection | **Balanced**: favorites slightly preferred. **Favorites**: favorites, well-rated and frequently played songs. **Discover**: rarely or never played songs. **Recently added**: recently imported music. |
| Tracks | Empty means the global default |
| User | Empty means the owner from the general settings |

### Custom situations

Additional playlists can be added under *Custom situations*. Available fields: name, emoji, genres, excluded genres, mood tags, BPM range, years, energy, flow, selection mode, an explicit content filter, track count and user.

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
| Year | Tag | Decade presets such as 80s & 90s |
| BPM | Tag | Songs with a suitable tempo are preferred; half and double values count as well |
| ReplayGain | Tag | Estimates how energetic a song is |
| Mood | Tag | Matching moods are preferred |
| Explicit | Tag | Excluded for situations such as Sleep and With Kids |
| Duration | File | Very short or long tracks and short intros are filtered out |
| Favorite, rating | Navidrome | Favorites and 4 to 5 stars are preferred; 1-star songs are never picked |
| Play count, last played | Navidrome | Recently played songs are picked less often |
| Date added | Navidrome | Basis of the Recently added mode |

Steps for each playlist:

1. **Load candidates.** For every matching genre, random songs are requested through the Subsonic API, about six times as many as needed in total.
2. **Filter.** Excluded genres, 1-star songs, unsuitable durations and short intros are removed. If too few songs remain, the duration and intro filters are relaxed.
3. **Weight.** Each song gets a weight from BPM, energy, mood, rating, listening history and selection mode. Songs from recent playlists are weighted down.
4. **Pick.** Songs are drawn at random according to their weight, with a limit per artist.
5. **Order.** Depending on the situation, the order is random, rising (e.g. Gym, Party) or falling (e.g. Sleep, Yoga). Songs by the same artist never follow each other directly.

Without BPM, ReplayGain or mood tags, the selection relies on genre and listening history. These tags can be added with tools such as [beets](https://beets.io) or [MusicBrainz Picard](https://picard.musicbrainz.org). Rescan your library in Navidrome afterwards.

## Stored data

Cantilune uses Navidrome's key-value store (`plugins/cantilune/kvstore.db`). For each situation and day it stores only the song IDs of the generated playlist. Entries expire after the period set under *Avoid repeats*. The store is limited to 10 MB; actual usage is usually below 1 MB.

Generated playlists are identified by a marker in the playlist comment (`#cl:<situation>:…`). Other playlists are never modified, even if they use the same prefix.

## Uninstalling

Navidrome does not notify plugins when they are removed. To avoid leftover playlists, clean up before removing the plugin:

1. In the plugin settings under *Cleanup*, enable *Delete all Cantilune playlists and pause the plugin* and save.
2. Wait about 15 seconds until `cleanup finished` appears in the log.
3. Disable the plugin and delete `cantilune.ndp`. The folder `plugins/cantilune/` can be deleted as well.

## Troubleshooting

After each run, Cantilune writes one line per playlist to the Navidrome log:

```
🎧 Gym Hardstyle ⚡ for admin: 50 songs · Genres: Hardstyle (212), Euphoric Hardstyle (40) · not in library: Rawstyle, Gabber · candidates: 252 · filtered: duration 7, intro/skit 4 · metadata: 31 with BPM, 50 with ReplayGain, 9 favorites
```

| Log message | Cause and solution |
|---|---|
| `not in library: …` | These genres do not exist in your library; the others are still used. If none match, the log suggests similar genres. |
| `no matching songs … The previous playlist is kept.` | Choose a different preset or check your genre tags. |
| `user "…" is not permitted for the plugin` | Allow the user under *User access* in the plugin settings. |
| `time "…" is invalid` or `cron expression … must have exactly 5 fields` | Use `HH:MM` or a five-field cron expression. Until corrected, `0 4 * * *` is used. |

To regenerate all playlists immediately, delete the Cantilune playlists and save the plugin settings again.

## Development

### Project structure

```
.
├── main.go              plugin entry points (OnInit, OnCallback)
├── config.go            reading settings, planning playlists
├── generator.go         creating, replacing and cleaning up playlists
├── selection.go         filtering, weighting, picking and ordering
├── genres.go            matching against library genres
├── history.go           history in the key-value store
├── subsonic.go          Subsonic API calls
├── catalog/
│   ├── presets.json     situations and presets
│   ├── catalog.go       loading and resolving presets
│   ├── legacy.go        aliases for settings saved by versions before 1.1.0
│   └── manifest.go      generating manifest.json
├── cmd/genmanifest/     manifest.json generator
├── assets/              logo
└── manifest.json        generated, do not edit by hand
```

### Building

Requires [Go](https://go.dev/dl/) 1.25 or later, [TinyGo](https://tinygo.org/getting-started/install/) 0.39 or later (tested with 0.42) and `wasm-opt` from [Binaryen](https://github.com/WebAssembly/binaryen/releases).

```bash
go test ./...
make package        # creates dist/cantilune.ndp
```

Without `make`:

```bash
tinygo build -no-debug -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip -j cantilune.ndp manifest.json plugin.wasm
```

### Adding presets

Situations and presets are maintained in [`catalog/presets.json`](catalog/presets.json). A preset is a single entry:

```json
{ "name": "Phonk", "emoji": "🚘", "genres": ["Phonk", "Drift Phonk"], "minBpm": 120, "maxBpm": 160 }
```

| Field | Meaning |
|---|---|
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

Presets are stored in the settings by their display name. If a preset is renamed, existing installations fall back to the first preset of that situation unless an alias is added in `catalog/legacy.go`.

### Releases

Pushing a tag in the format `v*` starts the [release workflow](.github/workflows/release.yml). It runs the tests, builds the plugin, takes the version from the tag and attaches `cantilune.ndp` to the release.

```bash
git tag -a v1.2.0 -m "Cantilune 1.2.0"
git push origin v1.2.0
```

Changes between versions are listed in the [changelog](CHANGELOG.md).

### Contributing

Bug reports, suggestions for new situations and presets, and pull requests are welcome. Please run `go generate ./...` and `go test ./...` before opening a pull request.

## License

Cantilune is licensed under the [GPL-3.0](LICENSE). The Navidrome Plugin Development Kit, which is compiled into `plugin.wasm`, is also licensed under the GPL-3.0.

The logo was created with ChatGPT.
