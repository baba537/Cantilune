package catalog

import (
	"bytes"
	"encoding/json"
)

// Manifest metadata.
const (
	PluginName        = "Cantilune"
	PluginAuthor      = "Cantilune contributors"
	PluginWebsite     = "https://github.com/baba537/Cantilune"
	PluginDescription = "Creates daily playlists for 30 everyday situations (gym, driving, cooking, studying, sleep …) with selectable presets and a song selection based on genre, BPM, ReplayGain, favorites, ratings and listening history."
)

// Keys of the global settings.
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
	KeyAudience         = "audience"
	KeyLanguage         = "playlistLanguage"
	KeyWeightGenre      = "weightGenre"
	KeyWeightTempo      = "weightTempo"
	KeyWeightPreference = "weightPreference"
	KeyWeightVariety    = "weightVariety"
	KeyLearnFromEdits   = "learnFromEdits"
	KeyLogDetails       = "logDetails"
	KeyDryRun           = "dryRun"
	KeyArchiveDays      = "archiveDays"
)

// Fields of a built-in situation. Version 1.0.0 stored German values under
// "preset" and "mode", which the web UI marks as invalid and does not replace
// with defaults. New field names let the form start from valid defaults; the
// old fields are still read until the settings are saved again.
const (
	KeySituationPreset = "style"
	KeySituationMode   = "selection"
)

// Audience values (dropdown).
const (
	AudienceShared   = "Shared"
	AudiencePersonal = "Personal"
)

// Audiences lists the audience options in display order.
var Audiences = []string{AudienceShared, AudiencePersonal}

// DefaultWeight is the neutral weight of a factor group in percent.
const DefaultWeight = 100

// kv/obj build a JSON object with a fixed key order.
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

// Short titles for the compact rows per situation; details go into the description.
func trackCountProp() obj {
	return obj{{"type", "integer"}, {"title", "Tracks"}, {"description", "empty = default"}, {"minimum", 0}, {"maximum", 500}}
}

func targetUserProp() obj {
	return obj{{"type", "string"}, {"title", "User"}, {"description", "empty = default owner"}}
}

// DevVersion is the version in the committed manifest.json; release builds
// replace it through BuildManifestFor.
const DevVersion = "0.0.0-dev"

// BuildManifest generates the complete content of manifest.json.
func BuildManifest() ([]byte, error) {
	return BuildManifestFor(DevVersion, PluginWebsite)
}

// BuildManifestFor generates manifest.json with the given version and website.
func BuildManifestFor(version, website string) ([]byte, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}

	props := obj{
		{KeyPrefix, obj{{"type", "string"}, {"title", "Prefix"}, {"description", "Placed before every playlist name, e.g. 🎧, 🌙 or ✨."}, {"minLength", 1}, {"default", DefaultPrefix}}},
		{KeyShowPresetInName, obj{{"type", "boolean"}, {"title", "Show preset in playlist name (e.g. \"Gym Hardstyle ⚡\")"}, {"default", true}}},
		{KeyDefaultUser, obj{{"type", "string"}, {"title", "Owner"}, {"description", "User who owns the playlists. Empty = first permitted admin."}, {"default", ""}}},
		{KeyTrackCount, intProp("Tracks per playlist", "", 1, 500, DefaultTrackCount)},
		{KeyPublicPlaylists, obj{{"type", "boolean"}, {"title", "Make playlists public (visible to all users)"}, {"default", true}}},
		{KeyGenerationTime, obj{{"type", "string"}, {"title", "Time (HH:MM)"}, {"description", "Daily regeneration (server time)"}, {"pattern", "^([01][0-9]|2[0-3]):[0-5][0-9]$"}, {"default", DefaultGenerationTime}}},
		{KeyCronExpression, obj{{"type", "string"}, {"title", "Cron (optional)"}, {"description", "Overrides the time, e.g. \"0 */6 * * *\""}, {"default", ""}}},
		{KeyRunOnStartup, obj{{"type", "boolean"}, {"title", "Create missing or changed playlists right after saving"}, {"default", true}}},
		{KeyMaxPerArtist, intProp("Max. per artist", "Songs per artist per playlist, 0 = unlimited", 0, 100, DefaultMaxPerArtist)},
		{KeyAvoidRecentDays, intProp("Avoid played (days)", "Pick recently played songs less often, 0 = off", 0, 90, DefaultAvoidRecentDays)},
		{KeyHistoryDays, intProp("Avoid repeats (days)", "Pick songs from recent playlists less often, 0 = previous playlist only", 0, 30, DefaultHistoryDays)},
		{KeySkipInterludes, obj{{"type", "boolean"}, {"title", "Skip short intros, skits and interludes"}, {"default", true}}},
		{KeyExcludeGenres, append(stringList("Never use these genres", "Applies to all playlists unless a preset explicitly includes the genre."), kv{"default", DefaultExcludeGenres})},
		{KeyRemoveAll, obj{{"type", "boolean"}, {"title", "Delete all Cantilune playlists and pause the plugin"}, {"description", "Enable and save before uninstalling. Navidrome does not notify plugins when they are removed, so Cantilune cannot clean up afterwards."}, {"default", false}}},
		{KeyAudience, obj{{"type", "string"}, {"title", "Playlists for"}, {"description", "Shared: one playlist owned by the owner, based on the owner's favorites and history. Personal: a private playlist for every permitted user, based on that user's own data."}, {"enum", Audiences}, {"default", AudienceShared}}},
		{KeyLanguage, obj{{"type", "string"}, {"title", "Playlist language"}, {"description", "Language of situation and preset names in playlist names and comments"}, {"enum", Languages}, {"default", LanguageEnglish}}},
		{KeyWeightGenre, intProp("Genre fit (%)", "How strongly exact genre matches are preferred, 0 = ignore", 0, 200, DefaultWeight)},
		{KeyWeightTempo, intProp("Tempo, energy, mood (%)", "Influence of BPM, ReplayGain and mood tags, 0 = ignore", 0, 200, DefaultWeight)},
		{KeyWeightPreference, intProp("Favorites & ratings (%)", "Influence of favorites, ratings, play counts and the selection mode, 0 = ignore", 0, 200, DefaultWeight)},
		{KeyWeightVariety, intProp("Variety (%)", "How strongly recently played songs and recent playlists are avoided, 0 = ignore", 0, 200, DefaultWeight)},
		{KeyLearnFromEdits, obj{{"type", "boolean"}, {"title", "Learn from playlist edits"}, {"description", "Songs you remove from a Cantilune playlist are avoided for 60 days, songs you add are preferred"}, {"default", true}}},
		{KeyLogDetails, obj{{"type", "boolean"}, {"title", "Log why each song was chosen"}, {"default", false}}},
		{KeyDryRun, obj{{"type", "boolean"}, {"title", "Preview only (log the selection, do not change playlists)"}, {"default", false}}},
		{KeyArchiveDays, intProp("Archive (days)", "Keep replaced playlists as private archive with date, 0 = delete immediately", 0, 30, 0)},
	}

	ui := []any{
		group("General",
			horizontal(control("#/properties/"+KeyPrefix), control("#/properties/"+KeyDefaultUser), control("#/properties/"+KeyTrackCount)),
			horizontal(control("#/properties/"+KeyAudience), control("#/properties/"+KeyLanguage)),
			horizontal(control("#/properties/"+KeyPublicPlaylists), control("#/properties/"+KeyShowPresetInName)),
		),
		group("Schedule",
			horizontal(control("#/properties/"+KeyGenerationTime), control("#/properties/"+KeyCronExpression)),
			control("#/properties/"+KeyRunOnStartup),
		),
		group("Selection",
			horizontal(control("#/properties/"+KeyMaxPerArtist), control("#/properties/"+KeyAvoidRecentDays), control("#/properties/"+KeyHistoryDays)),
			control("#/properties/"+KeySkipInterludes),
			control("#/properties/"+KeyLearnFromEdits),
			control("#/properties/"+KeyExcludeGenres),
		),
		group("Weights & transparency",
			horizontal(control("#/properties/"+KeyWeightGenre), control("#/properties/"+KeyWeightTempo), control("#/properties/"+KeyWeightPreference), control("#/properties/"+KeyWeightVariety)),
			horizontal(control("#/properties/"+KeyLogDetails), control("#/properties/"+KeyDryRun)),
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
					{KeySituationPreset, obj{{"type", "string"}, {"title", "Preset"}, {"enum", labels}, {"default", labels[0]}}},
					{KeySituationMode, obj{{"type", "string"}, {"title", "Selection"}, {"enum", Modes}, {"default", ModeBalanced}}},
					{"trackCount", trackCountProp()},
					{"targetUser", targetUserProp()},
				}},
				{"default", obj{{"enabled", s.Enabled}, {KeySituationPreset, labels[0]}, {KeySituationMode, ModeBalanced}}},
			}})
			base := "#/properties/" + s.ID + "/properties/"
			rows = append(rows, horizontal(
				control(base+"enabled", kv{"label", label}),
				control(base+KeySituationPreset),
				control(base+KeySituationMode),
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
		{"title", "Custom situations"},
		{"description", "Any number of additional situations with your own filters."},
		{"items", obj{
			{"type", "object"},
			{"properties", obj{
				{"enabled", obj{{"type", "boolean"}, {"title", "Enabled"}, {"default", true}}},
				{"name", obj{{"type", "string"}, {"title", "Name"}, {"minLength", 1}}},
				{"emoji", obj{{"type", "string"}, {"title", "Emoji"}}},
				{"genres", stringList("Genres (empty = whole library)", "")},
				{"excludeGenres", stringList("Excluded genres", "")},
				{"moods", stringList("Moods (mood tags)", "")},
				{"minBpm", obj{{"type", "integer"}, {"title", "BPM min"}, {"minimum", 0}, {"maximum", 400}}},
				{"maxBpm", obj{{"type", "integer"}, {"title", "BPM max"}, {"minimum", 0}, {"maximum", 400}}},
				{"fromYear", obj{{"type", "integer"}, {"title", "From year"}, {"minimum", 0}, {"maximum", 2100}}},
				{"toYear", obj{{"type", "integer"}, {"title", "To year"}, {"minimum", 0}, {"maximum", 2100}}},
				{"energy", obj{{"type", "string"}, {"title", "Energy"}, {"enum", EnergyOrder}, {"default", EnergyOrder[0]}}},
				{"flow", obj{{"type", "string"}, {"title", "Flow"}, {"enum", FlowOrder}, {"default", FlowOrder[0]}}},
				{"mode", obj{{"type", "string"}, {"title", "Selection"}, {"enum", Modes}, {"default", ModeBalanced}}},
				{"excludeExplicit", obj{{"type", "boolean"}, {"title", "Exclude explicit songs"}}},
				{"trackCount", trackCountProp()},
				{"targetUser", targetUserProp()},
			}},
			{"required", []string{"name"}},
		}},
		{"default", []any{}},
	}
	props = append(props, kv{KeyCustomSituations, custom})

	ui = append(ui, group("✏️ Custom situations", control("#/properties/"+KeyCustomSituations, kv{"options", obj{
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

	ui = append(ui, group("🧹 Cleanup", control("#/properties/"+KeyArchiveDays), control("#/properties/"+KeyRemoveAll)))

	manifest := obj{
		{"name", PluginName},
		{"author", PluginAuthor},
		{"version", version},
		{"description", PluginDescription},
		{"website", website},
		{"config", obj{
			{"schema", obj{{"type", "object"}, {"properties", props}}},
			{"uiSchema", obj{{"type", "VerticalLayout"}, {"elements", ui}}},
		}},
		{"permissions", obj{
			{"subsonicapi", obj{{"reason", "Read songs and their metadata (getRandomSongs, getGenres, getPlaylist) and create, publish and replace Cantilune playlists"}}},
			{"users", obj{{"reason", "Perform Subsonic API calls on behalf of the configured playlist owners"}}},
			{"scheduler", obj{{"reason", "Regenerate playlists daily at the configured time"}}},
			{"kvstore", obj{{"reason", "Remember recent songs and playlist edits so playlists do not repeat and follow your changes"}, {"maxSize", "10MB"}}},
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
