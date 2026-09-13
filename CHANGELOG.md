# Changelog

All notable changes to Cantilune. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows [Semantic Versioning](https://semver.org/).

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

[1.1.0]: https://github.com/baba537/Cantilune/releases/tag/v1.1.0
[1.0.0]: https://github.com/baba537/Cantilune/releases/tag/v1.0.0
