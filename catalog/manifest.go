package catalog

import (
	"bytes"
	"encoding/json"
)

// Manifest-Metadaten.
const (
	PluginName        = "Cantilune"
	PluginAuthor      = "baba537"
	PluginWebsite     = "https://github.com/baba537/Cantilune"
	PluginDescription = "Erstellt täglich automatisch Playlists für 30 Alltagssituationen (Gym, Auto fahren, Kochen, Lernen, Schlafen …) – mit wählbaren Presets und einer Auswahl, die Genres, BPM, ReplayGain, Favoriten, Bewertungen und Hörverlauf berücksichtigt."
)

// Schlüssel der globalen Einstellungen.
const (
	KeyPrefix           = "playlistPrefix"
	KeyShowPresetInName = "showPresetInName"
	KeyDefaultUser      = "defaultUser"
	KeyTrackCount       = "trackCount"
	KeyPublicPlaylists  = "publicPlaylists"
	KeyGenerationTime   = "generationTime"
	KeyCronExpression   = "cronExpression"
	KeyRunOnStartup     = "runOnStartup"
	KeyMaxPerArtist     = "maxPerArtist"
	KeyAvoidRecentDays  = "avoidRecentDays"
	KeyHistoryDays      = "historyDays"
	KeySkipInterludes   = "skipInterludes"
	KeyExcludeGenres    = "excludeGenres"
	KeyCustomSituations = "customSituations"
	KeyRemoveAll        = "removeAllPlaylists"
)

// kv/obj bilden ein JSON-Objekt mit fester Schlüsselreihenfolge.
type kv struct {
	k string
	v any
}

type obj []kv

func (o obj) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := marshalNoEscape(e.k)
		if err != nil {
			return nil, err
		}
		v, err := marshalNoEscape(e.v)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func control(scope string, extra ...kv) obj {
	return append(obj{{"type", "Control"}, {"scope", scope}}, extra...)
}

func horizontal(elements ...any) obj {
	return obj{{"type", "HorizontalLayout"}, {"elements", elements}}
}

func group(label string, elements ...any) obj {
	return obj{{"type", "Group"}, {"label", label}, {"elements", elements}}
}

func stringList(title, description string) obj {
	o := obj{{"type", "array"}, {"title", title}}
	if description != "" {
		o = append(o, kv{"description", description})
	}
	return append(o, kv{"items", obj{{"type", "string"}}})
}

func intProp(title, description string, min, max, def int) obj {
	o := obj{{"type", "integer"}, {"title", title}}
	if description != "" {
		o = append(o, kv{"description", description})
	}
	return append(o, kv{"minimum", min}, kv{"maximum", max}, kv{"default", def})
}

// Kurze Titel für die kompakten Zeilen je Situation; Erklärungen stehen in der Beschreibung.
func trackCountProp() obj {
	return obj{{"type", "integer"}, {"title", "Tracks"}, {"description", "leer = Standard"}, {"minimum", 0}, {"maximum", 500}}
}

func targetUserProp() obj {
	return obj{{"type", "string"}, {"title", "Benutzer"}, {"description", "leer = Standard-Zielbenutzer"}}
}

// BuildManifest erzeugt den vollständigen Inhalt von manifest.json.
func BuildManifest() ([]byte, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}

	props := obj{
		{KeyPrefix, obj{{"type", "string"}, {"title", "Präfix"}, {"description", "Steht vor jedem Playlist-Namen, z. B. 🎧, 🌙 oder ✨."}, {"minLength", 1}, {"default", DefaultPrefix}}},
		{KeyShowPresetInName, obj{{"type", "boolean"}, {"title", "Preset im Namen anzeigen (z. B. „Gym Hardstyle ⚡“)"}, {"default", true}}},
		{KeyDefaultUser, obj{{"type", "string"}, {"title", "Zielbenutzer"}, {"description", "Besitzer der Playlists. Leer = erster freigegebener Admin."}, {"default", ""}}},
		{KeyTrackCount, intProp("Tracks pro Playlist", "", 1, 500, DefaultTrackCount)},
		{KeyPublicPlaylists, obj{{"type", "boolean"}, {"title", "Playlists öffentlich machen (für alle Benutzer sichtbar)"}, {"default", true}}},
		{KeyGenerationTime, obj{{"type", "string"}, {"title", "Uhrzeit (HH:MM)"}, {"description", "Tägliche Neugenerierung (Serverzeit)"}, {"pattern", "^([01][0-9]|2[0-3]):[0-5][0-9]$"}, {"default", DefaultGenerationTime}}},
		{KeyCronExpression, obj{{"type", "string"}, {"title", "Cron (optional)"}, {"description", "Überschreibt die Uhrzeit, z. B. „0 */6 * * *“"}, {"default", ""}}},
		{KeyRunOnStartup, obj{{"type", "boolean"}, {"title", "Nach dem Speichern fehlende oder geänderte Playlists sofort erstellen"}, {"default", true}}},
		{KeyMaxPerArtist, intProp("Max. pro Künstler", "Songs je Künstler pro Playlist, 0 = unbegrenzt", 0, 100, DefaultMaxPerArtist)},
		{KeyAvoidRecentDays, intProp("Gehörtes meiden (Tage)", "Kürzlich gehörte Songs seltener wählen, 0 = aus", 0, 90, DefaultAvoidRecentDays)},
		{KeyHistoryDays, intProp("Wiederholung meiden (Tage)", "Songs aus den Playlists der letzten Tage seltener wählen, 0 = nur Vortag", 0, 30, DefaultHistoryDays)},
		{KeySkipInterludes, obj{{"type", "boolean"}, {"title", "Kurze Intros, Skits und Interludes überspringen"}, {"default", true}}},
		{KeyExcludeGenres, append(stringList("Genres nie verwenden", "Gilt für alle Playlists, außer ein Preset wählt das Genre ausdrücklich."), kv{"default", DefaultExcludeGenres})},
		{KeyRemoveAll, obj{{"type", "boolean"}, {"title", "Alle Cantilune-Playlists löschen und Plugin pausieren"}, {"description", "Vor dem Deinstallieren einschalten und speichern: Navidrome meldet Plugins das Entfernen nicht, danach kann Cantilune nicht mehr aufräumen."}, {"default", false}}},
	}

	ui := []any{
		group("Allgemein",
			horizontal(control("#/properties/"+KeyPrefix), control("#/properties/"+KeyDefaultUser), control("#/properties/"+KeyTrackCount)),
			horizontal(control("#/properties/"+KeyPublicPlaylists), control("#/properties/"+KeyShowPresetInName)),
		),
		group("Zeitplan",
			horizontal(control("#/properties/"+KeyGenerationTime), control("#/properties/"+KeyCronExpression)),
			control("#/properties/"+KeyRunOnStartup),
		),
		group("Auswahl",
			horizontal(control("#/properties/"+KeyMaxPerArtist), control("#/properties/"+KeyAvoidRecentDays), control("#/properties/"+KeyHistoryDays)),
			control("#/properties/"+KeySkipInterludes),
			control("#/properties/"+KeyExcludeGenres),
		),
	}

	for _, cat := range c.Categories {
		var rows []any
		for _, s := range c.Situations {
			if s.Category != cat.ID {
				continue
			}
			label := s.Name + " " + s.Emoji
			labels := s.PresetLabels()
			props = append(props, kv{s.ID, obj{
				{"type", "object"},
				{"title", label},
				{"properties", obj{
					{"enabled", obj{{"type", "boolean"}, {"title", label}, {"default", s.Enabled}}},
					{"preset", obj{{"type", "string"}, {"title", "Preset"}, {"enum", labels}, {"default", labels[0]}}},
					{"mode", obj{{"type", "string"}, {"title", "Auswahl"}, {"enum", Modes}, {"default", ModeBalanced}}},
					{"trackCount", trackCountProp()},
					{"targetUser", targetUserProp()},
				}},
				{"default", obj{{"enabled", s.Enabled}, {"preset", labels[0]}, {"mode", ModeBalanced}}},
			}})
			base := "#/properties/" + s.ID + "/properties/"
			rows = append(rows, horizontal(
				control(base+"enabled", kv{"label", label}),
				control(base+"preset"),
				control(base+"mode"),
				control(base+"trackCount"),
				control(base+"targetUser"),
			))
		}
		if len(rows) > 0 {
			ui = append(ui, group(cat.Label, rows...))
		}
	}

	custom := obj{
		{"type", "array"},
		{"title", "Eigene Situationen"},
		{"description", "Beliebig viele zusätzliche Situationen mit eigenen Filtern."},
		{"items", obj{
			{"type", "object"},
			{"properties", obj{
				{"enabled", obj{{"type", "boolean"}, {"title", "Aktiviert"}, {"default", true}}},
				{"name", obj{{"type", "string"}, {"title", "Name"}, {"minLength", 1}}},
				{"emoji", obj{{"type", "string"}, {"title", "Emoji"}}},
				{"genres", stringList("Genres (leer = ganze Bibliothek)", "")},
				{"excludeGenres", stringList("Genres ausschließen", "")},
				{"moods", stringList("Stimmungen (Mood-Tags)", "")},
				{"minBpm", obj{{"type", "integer"}, {"title", "BPM min"}, {"minimum", 0}, {"maximum", 400}}},
				{"maxBpm", obj{{"type", "integer"}, {"title", "BPM max"}, {"minimum", 0}, {"maximum", 400}}},
				{"fromYear", obj{{"type", "integer"}, {"title", "Jahr ab"}, {"minimum", 0}, {"maximum", 2100}}},
				{"toYear", obj{{"type", "integer"}, {"title", "Jahr bis"}, {"minimum", 0}, {"maximum", 2100}}},
				{"energy", obj{{"type", "string"}, {"title", "Energie"}, {"enum", EnergyOrder}, {"default", EnergyOrder[0]}}},
				{"flow", obj{{"type", "string"}, {"title", "Verlauf"}, {"enum", FlowOrder}, {"default", FlowOrder[0]}}},
				{"mode", obj{{"type", "string"}, {"title", "Auswahl"}, {"enum", Modes}, {"default", ModeBalanced}}},
				{"excludeExplicit", obj{{"type", "boolean"}, {"title", "Explizite Songs ausschließen"}}},
				{"trackCount", trackCountProp()},
				{"targetUser", targetUserProp()},
			}},
			{"required", []string{"name"}},
		}},
		{"default", []any{}},
	}
	props = append(props, kv{KeyCustomSituations, custom})

	ui = append(ui, group("✏️ Eigene Situationen", control("#/properties/"+KeyCustomSituations, kv{"options", obj{
		{"elementLabelProp", "name"},
		{"detail", obj{
			{"type", "VerticalLayout"},
			{"elements", []any{
				horizontal(control("#/properties/enabled"), control("#/properties/name"), control("#/properties/emoji"), control("#/properties/mode")),
				horizontal(control("#/properties/energy"), control("#/properties/flow"), control("#/properties/trackCount"), control("#/properties/targetUser")),
				horizontal(control("#/properties/minBpm"), control("#/properties/maxBpm"), control("#/properties/fromYear"), control("#/properties/toYear")),
				control("#/properties/genres"),
				control("#/properties/excludeGenres"),
				control("#/properties/moods"),
				control("#/properties/excludeExplicit"),
			}},
		}},
	}})))

	ui = append(ui, group("🧹 Aufräumen", control("#/properties/"+KeyRemoveAll)))

	manifest := obj{
		{"name", PluginName},
		{"author", PluginAuthor},
		{"version", "0.0.0-dev"},
		{"description", PluginDescription},
		{"website", PluginWebsite},
		{"config", obj{
			{"schema", obj{{"type", "object"}, {"properties", props}}},
			{"uiSchema", obj{{"type", "VerticalLayout"}, {"elements", ui}}},
		}},
		{"permissions", obj{
			{"subsonicapi", obj{{"reason", "Songs samt Metadaten abrufen (getRandomSongs, getGenres, getPlaylist) sowie Cantilune-Playlists anlegen, veröffentlichen und ersetzen"}}},
			{"users", obj{{"reason", "Die Subsonic-API-Aufrufe im Namen der konfigurierten Zielbenutzer ausführen"}}},
			{"scheduler", obj{{"reason", "Playlists täglich zur konfigurierten Uhrzeit neu generieren"}}},
			{"kvstore", obj{{"reason", "Songs der letzten Tage merken, damit sich Playlists nicht wiederholen"}, {"maxSize", "10MB"}}},
		}},
	}

	raw, err := marshalNoEscape(manifest)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
