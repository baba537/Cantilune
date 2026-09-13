# Changelog

Alle nennenswerten Änderungen an Cantilune. Das Format orientiert sich an [Keep a Changelog](https://keepachangelog.com/de/1.1.0/), die Versionierung folgt [Semantic Versioning](https://semver.org/lang/de/).

## [1.0.0] – 2026-09-14

Erste öffentliche Version.

### Funktionen
- 30 Situationen in sechs Kategorien mit jeweils mehreren Presets, definiert in `catalog/presets.json`
- Eine Playlist pro Situation, die täglich ersetzt wird
- Auswahl-Modi: Ausgewogen, Lieblingssongs, Entdecken, Neu hinzugefügt
- Gewichtete Auswahl anhand von Genre, Jahr, BPM (inklusive halber und doppelter Werte), ReplayGain, Mood, Bewertung, Wiedergaben und Hinzufügedatum
- Tolerante Genre-Erkennung mit Vorschlägen für fehlende Genres im Log
- Obergrenze pro Künstler sowie Filter für Länge, Intros und explizite Inhalte
- Energie-Verlauf je Situation: zufällig, ansteigend oder abklingend
- Verlauf der letzten Tage im KVStore, damit sich Playlists selten wiederholen
- Eigene Situationen mit frei wählbaren Filtern
- Zeitplan per Uhrzeit oder Cron-Ausdruck, sofortige Aktualisierung nach dem Speichern
- Option zum Löschen aller erzeugten Playlists vor der Deinstallation

[1.0.0]: https://github.com/baba537/Cantilune/releases/tag/v1.0.0
