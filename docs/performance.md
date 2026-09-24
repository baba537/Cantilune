# Performance

This document lists measured values and the limits built into Cantilune. All numbers can be reproduced with `make bench`; CI records the load profile on every push.

## What was measured

Two parts contribute to the time a daily run takes:

1. **Cantilune's own work:** parsing responses, filtering, weighting, picking and ordering. This is measured with a simulated Subsonic server and a generated library.
2. **Navidrome's work:** answering the Subsonic API calls, mainly the database queries behind `getRandomSongs`. This depends on the server's hardware and database and is **not** included in the load profile. The end-to-end tests show the combined time on a small real library.

The benchmarks run natively with Go. In Navidrome, the plugin runs as WebAssembly (compiled with TinyGo) in the wazero runtime, which is slower than native code; this factor has not been measured separately.

## Load profile: 100,000 songs, 10 users

Scenario: the 10 default situations in *Personal* mode for 10 users, which creates 100 playlists with 50 songs each. The library has 300 genres; about a third of the songs have BPM tags and half have ReplayGain tags.

| Measurement | GitHub Actions runner (Linux) | AMD Ryzen 7 5800X3D (Windows) |
|---|---|---|
| Cantilune's work for the whole run | 459 ms | 506 ms |
| Per playlist | 4.6 ms | 5.1 ms |
| Subsonic API calls | 1,720 | 1,720 |
| Memory allocated during the run (simulated server included) | 183 MB | 184 MB |
| Heap in use after the run (library of the simulated server included) | 53.5 MB | 51.1 MB |

API calls in this scenario: 1,500 × `getRandomSongs`, 100 × `createPlaylist`, 100 × `updatePlaylist`, 10 × `getPlaylists`, 10 × `getGenres`.

Single playlist (Gym Mix, 28 genres, 100,000 songs), Ryzen 7 5800X3D: 7.2 ms, 2.0 MB allocated, 54,000 allocations (`BenchmarkSelectSongs`).

Allocations are short-lived: the candidate pool of one playlist is released before the next playlist is generated.

## End-to-end with a real Navidrome

These numbers are measured inside Navidrome, with the plugin compiled to WebAssembly. The plugin logs the duration of every run (`generation finished: … in 114 ms`).

CI installs Navidrome 0.63.2 and 0.64.1, generates a library of 1,590 songs (90 tagged test songs and 1,500 songs of ten other genres) and lets the plugin create three playlists.

| Measurement | Navidrome 0.63.2 | Navidrome 0.64.1 |
|---|---|---|
| Generation run, 3 playlists (WebAssembly) | 126–127 ms | 83–119 ms |
| Slowest Navidrome `ping` response during generation | 3 ms | 1–3 ms |

A manual run with Navidrome 0.64.1 on Windows (Ryzen 7 5800X3D, 1,524 songs) created 4 playlists and checked 7 unchanged ones in 310 ms.

The first run starts about 15 seconds after Navidrome or the plugin starts; this delay is intentional. The plugin runs in Navidrome's scheduler callback, and Navidrome kept answering requests without noticeable delay during generation.

## Limits

| Limit | Value | Where |
|---|---|---|
| `getRandomSongs` calls per playlist | at most 40 | `maxGenreQueries` |
| Library genres per wanted genre | at most 12 | `maxGenresPerWanted` |
| Songs per `getRandomSongs` call | at most 500 (Subsonic limit) | `maxRandomSongsPerCall` |
| Candidate pool per playlist | 6 × playlist size, at least 150, at most 1,500 songs | `poolFactor`, `maxPoolSize` |
| Songs loaded individually (added by the user) | at most 25 per playlist | `maxAddedSongs` |
| Songs per playlist | at most 500 | `maxTracksPerPlaylist` |
| Key-value store | 10 MB (manifest), all entries expire | `manifest.json`, `history.go` |

API calls per playlist: up to 40 × `getRandomSongs`, 1 × `createPlaylist`, 1 × `updatePlaylist`, 1 × `getPlaylist` per existing playlist and up to 25 × `getSong`. Per user per run: 1 × `getGenres` and 1 × `getPlaylists`.

### Stored data size

The key-value store holds one entry per situation, user and day with the song IDs of the playlist (`hist:`), one entry with the current playlist (`gen:`) and one small entry per edited song (`fb:`). Navidrome song IDs are 22 characters. A 50-song playlist takes about 1.2 KB per day; with *Avoid repeats* at 7 days, 10 situations and 10 users this is about 1 MB.

## Reproducing

```bash
make bench
```

This runs `BenchmarkSelectSongs`, `BenchmarkDailyRunPersonal` and the load profile (`CANTILUNE_LOAD=1 go test -run TestLoadProfile -v .`).
