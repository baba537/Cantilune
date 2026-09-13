<p align="center">
  <img src="assets/logo.png" width="160" alt="Cantilune">
</p>

<h1 align="center">Cantilune</h1>

<p align="center">
  Tägliche Playlists für Alltagssituationen als Plugin für <a href="https://www.navidrome.org">Navidrome</a>
</p>

<p align="center">
  <a href="https://github.com/baba537/Cantilune/releases/latest"><img src="https://img.shields.io/github/v/release/baba537/Cantilune" alt="Release"></a>
  <a href="https://github.com/baba537/Cantilune/actions/workflows/ci.yml"><img src="https://github.com/baba537/Cantilune/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue" alt="Lizenz: GPL-3.0"></a>
</p>

Cantilune erstellt jeden Tag neue Playlists für Situationen wie Training, Autofahren, Kochen, Lernen oder Schlafen. Für jede Situation lässt sich ein Preset auswählen, zum Beispiel Hardstyle, HipHop oder EDM beim Training. Die Songs stammen aus der eigenen Bibliothek. Grundlage der Auswahl sind die Metadaten, die Navidrome ohnehin kennt: Tags wie Genre, BPM und ReplayGain sowie Favoriten, Bewertungen und der Hörverlauf.

Der Name verbindet *Canticle* (Gesang) und *Lune* (französisch für Mond).

```
🎧 Gym Hardstyle ⚡
🎧 Auto fahren 🚗
🎧 Kochen Jazz & Soul 🎷
🎧 Lernen Lo-Fi ☕
🎧 Schlafen Ambient 🌌
```

> [!NOTE]
> Cantilune wurde mit Unterstützung von Claude Opus 5 (Anthropic) entwickelt. Der Code ist durch automatisierte Tests abgedeckt, kann aber trotzdem Fehler enthalten. Probleme und Vorschläge bitte als [Issue](https://github.com/baba537/Cantilune/issues) melden.

## Inhalt

- [Funktionen](#funktionen)
- [Voraussetzungen](#voraussetzungen)
- [Installation](#installation)
- [Einstellungen](#einstellungen)
- [Situationen und Presets](#situationen-und-presets)
- [Wie die Songs ausgewählt werden](#wie-die-songs-ausgewählt-werden)
- [Gespeicherte Daten](#gespeicherte-daten)
- [Deinstallation](#deinstallation)
- [Fehlersuche](#fehlersuche)
- [Entwicklung](#entwicklung)
- [Lizenz](#lizenz)

## Funktionen

- 30 Situationen in sechs Kategorien, jeweils mit mehreren Presets
- Eine Playlist pro Situation, die täglich ersetzt wird
- Vier Auswahl-Modi: Ausgewogen, Lieblingssongs, Entdecken, Neu hinzugefügt
- Gewichtete Auswahl anhand von Genre, BPM, ReplayGain, Mood, Bewertung und Hörverlauf
- Abwechslung über mehrere Tage und höchstens drei Songs pro Künstler (einstellbar)
- Tolerante Genre-Erkennung, z. B. `Hip-Hop` = `Hip Hop`, `Hardstyle` findet auch `Euphoric Hardstyle`
- Eigene Situationen mit frei wählbaren Filtern über die Weboberfläche
- Zeitplan per Uhrzeit oder Cron-Ausdruck
- Öffentliche Playlists im Namen eines wählbaren Benutzers

## Voraussetzungen

- Navidrome mit Plugin-Unterstützung (`.ndp`-Pakete). Entwickelt und getestet mit Navidrome 0.64.
- Aktivierte Plugins in der Navidrome-Konfiguration

## Installation

1. `cantilune.ndp` aus dem [neuesten Release](https://github.com/baba537/Cantilune/releases/latest) herunterladen.

2. Die Datei in den Plugin-Ordner von Navidrome kopieren. Bei einem Update wird die vorhandene Datei ersetzt, die Einstellungen bleiben erhalten.

   | Installation | Pfad |
   |---|---|
   | Docker | `/data/plugins/cantilune.ndp` |
   | Linux-Paket | `/var/lib/navidrome/plugins/cantilune.ndp` |
   | Allgemein | `<DataFolder>/plugins/cantilune.ndp` |

   > [!IMPORTANT]
   > Die Datei nicht umbenennen. Navidrome leitet die Plugin-ID aus dem Dateinamen ab.

3. Plugins aktivieren, falls noch nicht geschehen, und Navidrome neu starten:

   ```toml
   # navidrome.toml
   [Plugins]
   Enabled = true
   ```

   Bei Docker alternativ über die Umgebungsvariable `ND_PLUGINS_ENABLED=true`.

4. In der Weboberfläche unter *Einstellungen → Plugins → Cantilune*:
   1. Unter *Benutzerzugriff* die gewünschten Benutzer freigeben oder alle Benutzer erlauben.
   2. Situationen auswählen, Einstellungen speichern und das Plugin aktivieren.

Etwa 15 Sekunden nach dem Speichern werden die Playlists erstellt. Danach läuft die Generierung täglich zur eingestellten Uhrzeit.

## Einstellungen

### Allgemein

| Einstellung | Standard | Beschreibung |
|---|---|---|
| Präfix | `🎧` | Steht vor jedem Playlist-Namen |
| Zielbenutzer | leer | Besitzer der Playlists; leer bedeutet erster freigegebener Admin |
| Tracks pro Playlist | `50` | 1 bis 500, pro Situation änderbar |
| Playlists öffentlich | an | Playlists sind für alle Benutzer sichtbar |
| Preset im Namen anzeigen | an | `🎧 Gym Hardstyle ⚡` statt `🎧 Gym ⚡` |

### Zeitplan

| Einstellung | Standard | Beschreibung |
|---|---|---|
| Uhrzeit | `04:00` | Tägliche Generierung in Serverzeit |
| Cron | leer | Ersetzt die Uhrzeit, z. B. `30 5 * * 1-5` für werktags 05:30 |
| Nach dem Speichern erstellen | an | Erstellt fehlende und geänderte Playlists sofort; unveränderte bleiben bis zum nächsten Lauf bestehen |

### Auswahl

| Einstellung | Standard | Beschreibung |
|---|---|---|
| Max. pro Künstler | `3` | Obergrenze pro Playlist; `0` bedeutet unbegrenzt |
| Gehörtes meiden | `3` Tage | Kürzlich gehörte Songs werden seltener gewählt |
| Wiederholung meiden | `7` Tage | Songs aus den Playlists dieses Zeitraums werden seltener gewählt |
| Intros und Skits überspringen | an | Tracks unter 2:30 mit „Intro“, „Skit“, „Interlude“ o. Ä. im Titel |
| Genres nie verwenden | Hörbuch, Podcast, Comedy, Weihnachten u. a. | Gilt für alle Playlists, sofern ein Preset das Genre nicht ausdrücklich enthält |

### Pro Situation

| Feld | Beschreibung |
|---|---|
| Aktiv | Schaltet die Situation ein oder aus. Beim Ausschalten wird die Playlist entfernt. |
| Preset | Auswahl der Musikrichtung |
| Auswahl | **Ausgewogen**: Favoriten leicht bevorzugt. **Lieblingssongs**: Favoriten, gut bewertete und oft gehörte Songs. **Entdecken**: selten oder nie gehörte Songs. **Neu hinzugefügt**: kürzlich importierte Musik. |
| Tracks | Leer bedeutet globaler Standard |
| Benutzer | Leer bedeutet Zielbenutzer aus den allgemeinen Einstellungen |

### Eigene Situationen

Zusätzliche Playlists lassen sich unter *Eigene Situationen* anlegen. Verfügbar sind Name, Emoji, Genres, ausgeschlossene Genres, Mood-Tags, BPM-Bereich, Jahre, Energie, Verlauf, Auswahl-Modus, ein Filter für explizite Inhalte, Trackanzahl und Benutzer.

## Situationen und Presets

Die mit ● markierten Situationen sind nach der Installation aktiv.

| Kategorie | Situation | Presets |
|---|---|---|
| Sport & Bewegung | ● Gym | Mix, Hardstyle, HipHop, EDM, Rock & Metal, Pop |
| | Laufen | Mix, Drum & Bass, Techno & House, Pop, Rock |
| | Radfahren | Mix, Indie, Elektro, Rock |
| | Yoga & Meditation | Ambient, Weltmusik, Downtempo, Klassik |
| | Spazieren & Wandern | Mix, Folk, Indie, Akustik |
| Unterwegs | ● Auto fahren | Mix, Rock, Pop, HipHop, Deutsch, 80er & 90er |
| | Roadtrip | Klassiker, Mitsingen, Indie, Country |
| | Bus & Bahn | Mix, Lo-Fi, Indie, Elektro |
| | Nachtfahrt | Synthwave, Deep House, Trip-Hop |
| Zuhause | ● Kochen | Mix, Jazz & Soul, Latin, Funk & Disco, Pop |
| | ● Essen | Jazz, Bossa & Lounge, Klassik, Akustik |
| | Frühstück | Akustik, Soul, Indie Pop, Jazz |
| | Aufwachen | Gute Laune, Sanft, Power |
| | ● Putzen | Mix, Pop-Hits, Disco & Funk, Rock, 2000er |
| | Duschen | Mitsingen, Power-Balladen, Deutsch |
| | Garten & Heimwerken | Mix, Rock, Country, Reggae |
| | Gaming | Soundtrack, Synthwave, Elektro, Metal |
| | Lesen | Klassik, Ambient, Jazz, Neoklassik |
| Konzentration | ● Lernen | Mix, Lo-Fi, Klassik, Ambient, Soundtrack |
| | ● Arbeiten & Fokus | Mix, Elektronisch, Post-Rock, Lo-Fi, Klassik |
| | Kreativ | Mix, Indie, Trip-Hop, Jazz |
| Entspannung & Schlaf | ● Entspannen | Mix, Chillout, Akustik, Reggae, Soul |
| | ● Schlafen | Ambient, Klavier, Klassik, Naturklänge |
| | Regentag | Melancholie, Jazz, Trip-Hop |
| Gesellschaft & Stimmung | ● Party | Mix, Dance & EDM, HipHop & R&B, 2000er, 90er, Schlager & Partyhits |
| | Dinner mit Gästen | Jazz, Soul & Funk, Lounge, Latin |
| | Grillen & Sommer | Mix, Reggae & Dancehall, Latin & Afro, Rock, HipHop |
| | Date & Romantik | Soul & R&B, Jazz, Akustik, Balladen |
| | Mit Kindern | Kinderlieder, Film & Musical, Gute-Laune-Pop |
| | Motivation | Episch, Rock, HipHop, Pop |

Das Preset *Mix* kombiniert die Genres aller anderen Presets einer Situation. Die genauen Genres und Filter stehen in [`catalog/presets.json`](catalog/presets.json).

## Wie die Songs ausgewählt werden

Navidrome analysiert keine Audiodaten. Es liest jedoch die Tags der Musikdateien und speichert Nutzungsdaten. Cantilune verwendet folgende Informationen:

| Information | Quelle | Verwendung |
|---|---|---|
| Genre | Tag | Grundlage der Auswahl, abgeglichen mit den Genres der Bibliothek |
| Jahr | Tag | Presets für Jahrzehnte, z. B. 80er & 90er |
| BPM | Tag | Songs im passenden Tempo werden bevorzugt; halbe und doppelte Werte zählen mit |
| ReplayGain | Tag | Schätzung der Energie eines Songs |
| Mood | Tag | Passende Stimmungen werden bevorzugt |
| Explicit | Tag | Wird z. B. bei „Schlafen“ und „Mit Kindern“ ausgeschlossen |
| Länge | Datei | Sehr kurze oder lange Tracks sowie kurze Intros werden aussortiert |
| Favorit, Bewertung | Navidrome | Favoriten und 4 bis 5 Sterne werden bevorzugt, 1 Stern wird nie gewählt |
| Wiedergaben, zuletzt gehört | Navidrome | Kürzlich Gehörtes wird seltener gewählt |
| Hinzugefügt am | Navidrome | Grundlage des Modus „Neu hinzugefügt“ |

Ablauf pro Playlist:

1. **Kandidaten laden.** Für jedes passende Genre werden über die Subsonic-API zufällige Songs abgefragt, insgesamt etwa sechsmal so viele wie benötigt.
2. **Filtern.** Ausgeschlossene Genres, mit einem Stern bewertete Songs, unpassende Längen und kurze Intros werden entfernt. Bleiben zu wenige Songs übrig, werden Längen- und Intro-Filter gelockert.
3. **Gewichten.** Jeder Song erhält ein Gewicht aus BPM, Energie, Stimmung, Bewertung, Hörverlauf und Auswahl-Modus. Songs aus den Playlists der letzten Tage werden abgewertet.
4. **Auswählen.** Die Songs werden gewichtet zufällig gezogen, mit einer Obergrenze pro Künstler.
5. **Sortieren.** Je nach Situation zufällig, ansteigend (z. B. Gym, Party) oder abklingend (z. B. Schlafen, Yoga). Songs desselben Künstlers folgen nicht direkt aufeinander.

Fehlen BPM-, ReplayGain- oder Mood-Tags, stützt sich die Auswahl auf Genre und Hörverlauf. Solche Tags lassen sich z. B. mit [beets](https://beets.io) oder [MusicBrainz Picard](https://picard.musicbrainz.org) ergänzen. Anschließend muss die Bibliothek in Navidrome neu gescannt werden.

## Gespeicherte Daten

Cantilune verwendet den Schlüssel-Wert-Speicher von Navidrome (`plugins/cantilune/kvstore.db`). Gespeichert werden pro Situation und Tag ausschließlich die Song-IDs der erzeugten Playlist. Die Einträge verfallen nach Ablauf des unter „Wiederholung meiden“ eingestellten Zeitraums. Der Speicher ist auf 10 MB begrenzt, der tatsächliche Bedarf liegt meist unter 1 MB.

Erzeugte Playlists erkennt Cantilune an einer Markierung im Kommentar (`#cl:<situation>:…`). Andere Playlists werden nie verändert, auch wenn sie dasselbe Präfix verwenden.

## Deinstallation

Navidrome benachrichtigt Plugins nicht, wenn sie entfernt werden. Damit keine Playlists zurückbleiben, vor dem Entfernen aufräumen:

1. In den Plugin-Einstellungen unter *Aufräumen* die Option „Alle Cantilune-Playlists löschen und Plugin pausieren“ aktivieren und speichern.
2. Etwa 15 Sekunden warten, bis im Log `Aufräumen abgeschlossen` erscheint.
3. Das Plugin deaktivieren und `cantilune.ndp` löschen. Der Ordner `plugins/cantilune/` kann ebenfalls gelöscht werden.

## Fehlersuche

Nach jedem Lauf schreibt Cantilune pro Playlist eine Zeile ins Navidrome-Log:

```
🎧 Gym Hardstyle ⚡ für admin: 50 Songs · Genres: Hardstyle (212), Euphoric Hardstyle (40) · nicht in der Bibliothek: Rawstyle, Gabber · Kandidaten: 252 · aussortiert: Intro/Skit 4, Länge 7 · Metadaten: 31 mit BPM, 50 mit ReplayGain, 9 Favoriten
```

| Meldung | Ursache und Lösung |
|---|---|
| `nicht in der Bibliothek: …` | Diese Genres fehlen in der Bibliothek; die übrigen werden verwendet. Fehlen alle, schlägt das Log ähnliche Genres vor. |
| `keine passenden Songs … Die bisherige Playlist bleibt erhalten` | Ein anderes Preset wählen oder die Genre-Tags prüfen. |
| `Benutzer "…" ist nicht für das Plugin freigegeben` | Den Benutzer unter *Benutzerzugriff* freigeben. |
| `Uhrzeit "…" ist ungültig` oder `Cron-Ausdruck …` | Format `HH:MM` bzw. fünfteiliger Cron-Ausdruck. Bis zur Korrektur gilt `0 4 * * *`. |

Um alle Playlists sofort neu zu erzeugen, die Cantilune-Playlists löschen und die Plugin-Einstellungen erneut speichern.

## Entwicklung

### Projektstruktur

```
.
├── main.go              Einstiegspunkte des Plugins (OnInit, OnCallback)
├── config.go            Einstellungen lesen, Playlists planen
├── generator.go         Playlists erstellen, ersetzen und aufräumen
├── selection.go         Filter, Gewichtung, Auswahl und Reihenfolge
├── genres.go            Abgleich mit den Genres der Bibliothek
├── history.go           Verlauf im KVStore
├── subsonic.go          Aufrufe der Subsonic-API
├── catalog/
│   ├── presets.json     Situationen und Presets
│   ├── catalog.go       Laden und Auflösen der Presets
│   └── manifest.go      Erzeugung von manifest.json
├── cmd/genmanifest/     Generator für manifest.json
├── assets/              Logo
└── manifest.json        generiert, nicht manuell bearbeiten
```

### Bauen

Benötigt werden [Go](https://go.dev/dl/) ab 1.25, [TinyGo](https://tinygo.org/getting-started/install/) ab 0.39 (getestet mit 0.42) und `wasm-opt` aus [Binaryen](https://github.com/WebAssembly/binaryen/releases).

```bash
go test ./...
make package        # erzeugt dist/cantilune.ndp
```

Ohne `make`:

```bash
tinygo build -no-debug -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip -j cantilune.ndp manifest.json plugin.wasm
```

### Presets ergänzen

Situationen und Presets werden in [`catalog/presets.json`](catalog/presets.json) gepflegt. Ein Preset ist ein einzelner Eintrag:

```json
{ "name": "Phonk", "emoji": "🚘", "genres": ["Phonk", "Drift Phonk"], "minBpm": 120, "maxBpm": 160 }
```

| Feld | Bedeutung |
|---|---|
| `genres`, `excludeGenres` | Gewünschte bzw. ausgeschlossene Genres |
| `moods` | Mood-Tags |
| `minBpm`, `maxBpm` | Tempo-Bereich |
| `fromYear`, `toYear` | Erscheinungsjahre |
| `energy` | `low`, `medium` oder `high` |
| `flow` | `shuffle`, `rising` oder `falling` |
| `minDuration`, `maxDuration` | Länge in Sekunden |
| `excludeExplicit` | Explizite Songs ausschließen |
| `mix` | Genres aller anderen Presets kombinieren |

Werte unter `defaults` gelten für alle Presets einer Situation. Nach einer Änderung muss das Manifest neu erzeugt werden; die CI prüft, ob beide Dateien übereinstimmen.

```bash
go generate ./...
```

Presets werden in den Einstellungen über ihren Anzeigenamen gespeichert. Wird ein Preset umbenannt, fällt die Situation bei bestehenden Installationen auf das erste Preset zurück.

### Releases

Ein Tag im Format `v*` startet den [Release-Workflow](.github/workflows/release.yml). Er führt die Tests aus, baut das Plugin, übernimmt die Version aus dem Tag und hängt `cantilune.ndp` an das Release.

```bash
git tag -a v1.1.0 -m "Cantilune 1.1.0"
git push origin v1.1.0
```

Änderungen zwischen den Versionen sind im [Changelog](CHANGELOG.md) aufgeführt.

### Mitwirken

Fehlerberichte, Vorschläge für neue Situationen und Presets sowie Pull Requests sind willkommen. Vor einem Pull Request bitte `go generate ./...` und `go test ./...` ausführen.

## Lizenz

Cantilune steht unter der [GPL-3.0](LICENSE). Das Navidrome Plugin Development Kit, das in `plugin.wasm` einkompiliert wird, ist ebenfalls unter GPL-3.0 lizenziert.

Das Logo wurde mit ChatGPT erstellt.
