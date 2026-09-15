# How Cantilune selects songs

This document describes the selection in detail. The implementation is in [`selection.go`](../selection.go), [`genres.go`](../genres.go) and [`history.go`](../history.go).

## Inputs

Cantilune uses only data that Navidrome provides through the Subsonic API.

| Data | Source | Used for |
|---|---|---|
| Genres | File tags | Candidate queries, genre fit, exclusions, ordering |
| Year | File tags | Decade presets, ordering |
| BPM | File tags | Tempo fit, energy estimate |
| ReplayGain (track or album gain) | File tags | Energy estimate |
| Mood | File tags | Mood fit |
| Explicit flag | File tags | Optional exclusion |
| Duration, title | File | Duration filter, intro and skit detection |
| Favorite, rating, play count, last played, date added | Navidrome, per user | Selection mode, variety |
| Songs of earlier Cantilune playlists | Key-value store | Variety, learning from edits |

Navidrome does not analyze audio and does not record skipped songs, so neither is available. Favorites, ratings and play data belong to the playlist owner. In *Shared* mode that is one user for everybody; in *Personal* mode every user's playlist uses that user's own data.

## 1. Genre matching

Each genre of a preset is compared with the genres that exist in the library (`getGenres`). Names are normalized first: lower case, `&`, `'n'` and a single `n` become `and`, punctuation and spaces are removed, a few aliases are applied (`RnB` = `R&B`, `DnB` = `Drum and Bass`, `Electronica` = `Electronic`).

| Result | Rule | Example for wanted `House` |
|---|---|---|
| Exact | Same normalized name | `house`, `HOUSE` |
| Sub-genre | Ends with the wanted genre as whole words after a qualifier of at least two characters | `Deep House`, `Afro House` |
| No match | Anything else | `Housemusic`, `Glasshouse`, `K-House` |

So `Dance` does not match `Dancehall`, `Reggae` does not match `Reggaeton`, and `Classical` does not match `Neoclassical Metal`.

Per wanted genre, at most 12 library genres are queried (exact matches and larger genres first), and at most 40 per playlist.

## 2. Candidates

Cantilune requests random songs with `getRandomSongs`, about six times as many as the playlist needs (at least 150, at most 1,500). Every wanted genre gets the same share; within a wanted genre the share is split by the number of songs of each library genre. Presets without genres (for example *80s & 90s*) query the whole library, filtered by year.

Songs the user added to the previous playlist are loaded individually (`getSong`, at most 25), so they can be chosen again.

## 3. Filters

A song is removed if any of these apply:

| Filter | Reason in the log |
|---|---|
| The user removed it from a Cantilune playlist of this situation in the last 60 days | `removed by user` |
| One of its genres contains an excluded genre as whole words (global list or preset) | `excluded genre` |
| None of its genres matches the preset (guards against SQL `LIKE` wildcards in genre names) | `genre mismatch` |
| Rated 1 star | `rated 1 star` |
| Explicit, if the preset excludes explicit songs | `explicit` |
| Year outside the preset range | `year` |
| Duration outside the preset range (default 60 to 900 seconds) | `duration` |
| Shorter than 2:30 with *intro*, *outro*, *skit*, *interlude*, *intermission*, *prelude* or *reprise* as a word in the title | `intro/skit` |

If fewer songs remain than the playlist needs, the duration and intro filters are dropped and the log notes `(duration/intro filters relaxed)`. Global exclusions do not apply to genres a preset explicitly asks for (for example *Kids Songs*).

## 4. Weights

Every remaining song gets a weight. It is the product of five factor groups; 1 is neutral.

| Group | Factors |
|---|---|
| **Genre** | `0.6 + 0.4 × share of the song's genres that match` for an exact match, 80 % of that for a sub-genre |
| **Tempo, energy, mood** | BPM: 1.5 inside the range, 1.0 up to 10 % outside, 0.6 up to 25 % outside, 0.35 beyond (half and double BPM also count). Mood: 2.0 if a mood tag matches, 0.6 if the song has other moods. Energy: see below. Missing tags count as 1. |
| **Favorites and ratings** | Depends on the selection mode (next table) |
| **Variety** | 0.3 if played within *Avoid played* days. Songs from Cantilune playlists of the last days: 0.15 for today and yesterday, 0.3 up to three days, 0.5 older. |
| **Edits** | 2.5 for songs the user added to a Cantilune playlist; they are also exempt from the variety penalty of the current playlist |

| Selection mode | Favorites and ratings factor |
|---|---|
| Balanced | favorite × 1.8; rating 5 × 2, 4 × 1.5, 2 × 0.5; play count up to × 1.5 |
| Favorites | favorite × 4; rating 5 × 4, 4 × 2.5, 2 × 0.3; play count up to × 3; never played, unrated and not a favorite × 0.3 |
| Discover | never played × 3, played 1–2 times × 1.6, 10 or more × 0.4; not played for 180 days × 1.5; rating 5 × 1.2, 4 × 1.1, 2 × 0.3 |
| Recently added | added within 30 days × 5, 90 days × 2.5, one year × 1.2, older × 0.5; rating 5 × 1.5, 4 × 1.2, 2 × 0.4 |

**Energy** is estimated from BPM (70 BPM → 0, 140 BPM → 1) and ReplayGain (+2 dB → 0, −10 dB → 1, louder masters have lower gain). If both exist, the average is used. Presets with *high* energy weight songs with `0.6 + 0.8 × energy`, *low* with `0.6 + 0.8 × (1 − energy)`, *medium* with `1.2 − 0.8 × |energy − 0.5|`. The estimate is only a hint, so its range is deliberately small.

### Configurable weights

The settings *Genre fit*, *Tempo, energy, mood*, *Favorites & ratings* and *Variety* scale their group as `factor ^ (weight / 100)`. At 100 % the factor is unchanged, at 0 % the group is ignored, at 200 % its effect doubles on a logarithmic scale (a factor of 0.5 becomes 0.25, 2 becomes 4).

## 5. Picking

Songs are drawn at random according to their weight without replacement (Efraimidis–Spirakis: each song gets the key `−ln(u) / weight` with a random `u`, the smallest keys win). Higher weights make a song more likely, but every day's playlist is different.

The first pass allows at most *Max. per artist* songs per artist and gives each wanted genre an equal share of the playlist. If that does not fill the playlist, the genre share and then the artist limit are dropped.

## 6. Order

- **Rising** (e.g. Gym, Party) and **falling** (e.g. Sleep, Yoga) sort by estimated energy with a small random offset, if at least half of the songs have BPM or ReplayGain tags.
- Otherwise songs are **chained**: starting from a random song, the next song is the most similar remaining one, with a small random term. The distance between two songs is `genre (0 if they share a genre, else 1) + 0.6 × min(year difference / 15, 1) + 0.8 × energy difference`; missing years count 0.2, missing energy 0.25.
- Finally, songs by the same artist are moved apart so they never follow each other directly.

## Quality metrics

After each playlist the log reports:

| Metric | Meaning |
|---|---|
| exact genre | Share of songs tagged exactly with a wanted genre |
| different artists | Distinct artists divided by songs |
| repeats | Share of songs that were in the previous playlist |
| coherence | `1 − average distance between neighbouring songs / 2.4` |

Tests check these metrics on generated libraries: with history enabled, repeats compared to the previous day drop from 310 to 77 songs over 14 days (5 runs, 30 of 200 songs a day), and chained order reaches a coherence of 0.95 compared to 0.35 for random order.

Cantilune has no A/B testing: it cannot observe how users react to a playlist beyond edits, favorites, ratings and play counts. The metrics make changes to presets and weights comparable over time.

## Explanations

With *Log why each song was chosen* enabled, every song gets a log line with its weight and the factor groups:

```
🎧 Gym Hardstyle ⚡ for admin #1: Example Artist – Example Track [Hardstyle] weight 3.60 = genre 1.00 × tempo/energy/mood 2.00 × favorites/ratings 1.80 × variety 1.00 × edits 1.00
```

With *Preview only* enabled, Cantilune performs the whole selection and writes the log lines, but does not create, change or delete playlists and does not store anything.

## Learning from edits

After creating a playlist, Cantilune stores its song IDs. At the next run it compares them with the playlist's current content:

- Songs that are no longer in the playlist are treated as removed by the user and excluded for this situation and user for 60 days.
- Songs that were added by the user are preferred (weight × 2.5) for 60 days, even if they do not match the preset's genres.

The feature can be switched off with *Learn from playlist edits*. Changes are recorded only for Cantilune playlists and only for the playlist's owner.
