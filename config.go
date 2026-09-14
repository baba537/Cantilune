package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

// Settings is the fully resolved plugin configuration.
type Settings struct {
	Prefix           string
	ShowPresetInName bool
	DefaultUser      string
	TrackCount       int
	PublicPlaylists  bool
	GenerationTime   string
	CronExpression   string
	RunOnStartup     bool
	MaxPerArtist     int
	AvoidRecentDays  int
	HistoryDays      int
	SkipInterludes   bool
	RemoveAll        bool
	ExcludeGenres    []string
	Situations       map[string]situationConfig
	Custom           []customSituation

	Audience       string
	Language       string
	Weights        Weights
	LearnFromEdits bool
	LogDetails     bool
	DryRun         bool
	ArchiveDays    int
}

// Weights scale the factor groups of the song weight in percent
// (100 = default, 0 = ignore the group, 200 = twice as strong).
type Weights struct {
	Genre      int
	Tempo      int
	Preference int
	Variety    int
}

// defaultWeights returns the neutral weights (100 % each).
func defaultWeights() Weights {
	return Weights{Genre: catalog.DefaultWeight, Tempo: catalog.DefaultWeight, Preference: catalog.DefaultWeight, Variety: catalog.DefaultWeight}
}

// Personal reports whether every permitted user gets a private playlist.
func (s Settings) Personal() bool {
	return s.Audience == catalog.AudiencePersonal
}

// situationConfig is the web UI row of a built-in situation.
type situationConfig struct {
	Enabled    *bool  `json:"enabled"`
	Preset     string `json:"preset"`
	Mode       string `json:"mode"`
	TrackCount int    `json:"trackCount"`
	TargetUser string `json:"targetUser"`
}

// customSituation is a user-defined situation.
type customSituation struct {
	Enabled         *bool    `json:"enabled"`
	Name            string   `json:"name"`
	Emoji           string   `json:"emoji"`
	Genres          []string `json:"genres"`
	ExcludeGenres   []string `json:"excludeGenres"`
	Moods           []string `json:"moods"`
	MinBPM          int      `json:"minBpm"`
	MaxBPM          int      `json:"maxBpm"`
	FromYear        int      `json:"fromYear"`
	ToYear          int      `json:"toYear"`
	Energy          string   `json:"energy"`
	Flow            string   `json:"flow"`
	Mode            string   `json:"mode"`
	ExcludeExplicit bool     `json:"excludeExplicit"`
	TrackCount      int      `json:"trackCount"`
	TargetUser      string   `json:"targetUser"`
}

// Job is a concrete playlist to generate.
type Job struct {
	ID          string // "gym" or "custom-<name>"
	Title       string // "Gym" (in the playlist language)
	PresetLabel string // "Hardstyle ⚡" (empty for custom situations)
	Name        string // full playlist name
	Recipe      catalog.Recipe
	Mode        string
	Count       int
	User        string // configured owner; resolved before generation
	Public      bool
	Fingerprint string
}

// configSource returns raw values from the plugin configuration.
// Navidrome stores strings as-is and all other types as JSON text.
type configSource func(key string) (string, bool)

func (c configSource) lookup(key string) (string, bool) {
	v, ok := c(key)
	return strings.TrimSpace(v), ok
}

func (c configSource) String(key, def string) string {
	if v, ok := c.lookup(key); ok {
		return v
	}
	return def
}

func (c configSource) Int(key string, def int) int {
	v, ok := c.lookup(key)
	if !ok || v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return int(f)
	}
	logf(pdk.LogWarn, "setting %q: %q is not a number, using %d", key, v, def)
	return def
}

func (c configSource) Bool(key string, def bool) bool {
	v, ok := c.lookup(key)
	if !ok || v == "" {
		return def
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	logf(pdk.LogWarn, "setting %q: %q is not a boolean, using %v", key, v, def)
	return def
}

// JSON parses a JSON value; it returns false if the value is missing or invalid.
func (c configSource) JSON(key string, target any) bool {
	v, ok := c.lookup(key)
	if !ok || v == "" {
		return false
	}
	if err := json.Unmarshal([]byte(v), target); err != nil {
		logf(pdk.LogError, "setting %q is invalid (%v), using defaults", key, err)
		return false
	}
	return true
}

// loadSettings reads and normalizes the complete configuration.
func loadSettings(src configSource, cat *catalog.Catalog) Settings {
	s := Settings{
		Prefix:           src.String(catalog.KeyPrefix, catalog.DefaultPrefix),
		ShowPresetInName: src.Bool(catalog.KeyShowPresetInName, true),
		DefaultUser:      src.String(catalog.KeyDefaultUser, ""),
		TrackCount:       clamp(src.Int(catalog.KeyTrackCount, catalog.DefaultTrackCount), 1, maxTracksPerPlaylist),
		PublicPlaylists:  src.Bool(catalog.KeyPublicPlaylists, true),
		GenerationTime:   src.String(catalog.KeyGenerationTime, catalog.DefaultGenerationTime),
		CronExpression:   src.String(catalog.KeyCronExpression, ""),
		RunOnStartup:     src.Bool(catalog.KeyRunOnStartup, true),
		MaxPerArtist:     clamp(src.Int(catalog.KeyMaxPerArtist, catalog.DefaultMaxPerArtist), 0, 100),
		AvoidRecentDays:  clamp(src.Int(catalog.KeyAvoidRecentDays, catalog.DefaultAvoidRecentDays), 0, 90),
		HistoryDays:      clamp(src.Int(catalog.KeyHistoryDays, catalog.DefaultHistoryDays), 0, 30),
		SkipInterludes:   src.Bool(catalog.KeySkipInterludes, true),
		RemoveAll:        src.Bool(catalog.KeyRemoveAll, false),
		ExcludeGenres:    catalog.DefaultExcludeGenres,
		Situations:       map[string]situationConfig{},
		Audience:         oneOf(src.String(catalog.KeyAudience, catalog.AudienceShared), catalog.Audiences),
		Language:         oneOf(src.String(catalog.KeyLanguage, catalog.LanguageEnglish), catalog.Languages),
		Weights: Weights{
			Genre:      clamp(src.Int(catalog.KeyWeightGenre, catalog.DefaultWeight), 0, 200),
			Tempo:      clamp(src.Int(catalog.KeyWeightTempo, catalog.DefaultWeight), 0, 200),
			Preference: clamp(src.Int(catalog.KeyWeightPreference, catalog.DefaultWeight), 0, 200),
			Variety:    clamp(src.Int(catalog.KeyWeightVariety, catalog.DefaultWeight), 0, 200),
		},
		LearnFromEdits: src.Bool(catalog.KeyLearnFromEdits, true),
		LogDetails:     src.Bool(catalog.KeyLogDetails, false),
		DryRun:         src.Bool(catalog.KeyDryRun, false),
		ArchiveDays:    clamp(src.Int(catalog.KeyArchiveDays, 0), 0, 30),
	}
	if s.Prefix == "" {
		s.Prefix = catalog.DefaultPrefix
	}
	var excludes []string
	if src.JSON(catalog.KeyExcludeGenres, &excludes) {
		s.ExcludeGenres = excludes
	}
	for _, sit := range cat.Situations {
		var sc situationConfig
		src.JSON(sit.ID, &sc)
		s.Situations[sit.ID] = sc
	}
	src.JSON(catalog.KeyCustomSituations, &s.Custom)
	return s
}

// buildJobs creates exactly one job for each enabled situation.
func buildJobs(cat *catalog.Catalog, s Settings) []Job {
	var jobs []Job
	for _, sit := range cat.Situations {
		sc := s.Situations[sit.ID]
		enabled := sit.Enabled
		if sc.Enabled != nil {
			enabled = *sc.Enabled
		}
		if !enabled {
			continue
		}
		preset, ok := sit.FindPreset(sc.Preset)
		if !ok && sc.Preset != "" {
			logf(pdk.LogWarn, "%s: preset %q no longer exists, using %q", sit.Name, sc.Preset, sit.PresetLabel(preset))
		}
		title := sit.DisplayName(s.Language)
		presetName := preset.DisplayName(s.Language)
		variant := ""
		if s.ShowPresetInName && !preset.Mix {
			variant = presetName
		}
		jobs = append(jobs, Job{
			ID:          sit.ID,
			Title:       title,
			PresetLabel: strings.TrimSpace(presetName + " " + sit.PresetEmoji(preset)),
			Name:        buildPlaylistName(s.Prefix, title, variant, sit.PresetEmoji(preset)),
			Recipe:      sit.Recipe(preset),
			Mode:        normalizeMode(sc.Mode),
			Count:       trackCount(sc.TrackCount, s.TrackCount),
			User:        firstNonEmpty(sc.TargetUser, s.DefaultUser),
			Public:      s.PublicPlaylists,
		})
	}

	used := map[string]int{}
	for _, c := range s.Custom {
		name := strings.TrimSpace(c.Name)
		if name == "" || (c.Enabled != nil && !*c.Enabled) {
			continue
		}
		id := "custom-" + slug(name)
		used[id]++
		if used[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, used[id])
		}
		jobs = append(jobs, Job{
			ID:    id,
			Title: name,
			Name:  buildPlaylistName(s.Prefix, name, "", c.Emoji),
			Recipe: catalog.Recipe{
				Genres:          catalog.UniqueFold(c.Genres),
				ExcludeGenres:   catalog.UniqueFold(c.ExcludeGenres),
				Moods:           catalog.UniqueFold(c.Moods),
				MinBPM:          c.MinBPM,
				MaxBPM:          c.MaxBPM,
				FromYear:        c.FromYear,
				ToYear:          c.ToYear,
				Energy:          labelValue(catalog.EnergyLabels, c.Energy),
				Flow:            labelValue(catalog.FlowLabels, c.Flow),
				ExcludeExplicit: c.ExcludeExplicit,
			},
			Mode:   normalizeMode(c.Mode),
			Count:  trackCount(c.TrackCount, s.TrackCount),
			User:   firstNonEmpty(c.TargetUser, s.DefaultUser),
			Public: s.PublicPlaylists,
		})
	}
	return jobs
}

// personalJobs expands every job into one private playlist per user.
func personalJobs(jobs []Job, users []string) []Job {
	out := make([]Job, 0, len(jobs)*len(users))
	for _, j := range jobs {
		for _, u := range users {
			pj := j
			pj.User = u
			pj.Public = false
			out = append(out, pj)
		}
	}
	return out
}

// buildPlaylistName builds names such as "🎧 Gym Hardstyle ⚡".
func buildPlaylistName(prefix, situation, variant, emoji string) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{prefix, situation, variant, emoji} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

// fingerprint detects whether the configuration of a playlist has changed.
func fingerprint(j Job, s Settings) string {
	data, _ := json.Marshal(struct {
		Name, Mode, User string
		Count            int
		Public           bool
		Recipe           catalog.Recipe
		MaxPerArtist     int
		SkipInterludes   bool
		ExcludeGenres    []string
		Weights          Weights
	}{j.Name, j.Mode, strings.ToLower(j.User), j.Count, j.Public, j.Recipe, s.MaxPerArtist, s.SkipInterludes, s.ExcludeGenres, s.Weights})
	h := fnv.New32a()
	_, _ = h.Write(data)
	return fmt.Sprintf("%08x", h.Sum32())
}

// cronExpression determines the cron expression from the cron field or the time of day.
func cronExpression(s Settings) (string, error) {
	if s.CronExpression != "" {
		fields := strings.Fields(s.CronExpression)
		if len(fields) != 5 {
			return "", fmt.Errorf("cron expression %q must have exactly 5 fields (minute hour day month weekday)", s.CronExpression)
		}
		return strings.Join(fields, " "), nil
	}
	t, err := time.Parse("15:04", s.GenerationTime)
	if err != nil {
		return "", fmt.Errorf("time %q is invalid, expected HH:MM", s.GenerationTime)
	}
	return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour()), nil
}

func normalizeMode(m string) string {
	m = catalog.Canonical(m)
	for _, mode := range catalog.Modes {
		if strings.EqualFold(strings.TrimSpace(m), mode) {
			return mode
		}
	}
	return catalog.ModeBalanced
}

// labelValue translates a dropdown label ("energetic") into the internal value ("high").
func labelValue(labels map[string]string, v string) string {
	v = strings.TrimSpace(catalog.Canonical(v))
	if internal, ok := labels[strings.ToLower(v)]; ok {
		return internal
	}
	for _, internal := range labels {
		if internal != "" && strings.EqualFold(internal, v) {
			return internal
		}
	}
	return ""
}

// oneOf returns v if it is one of the allowed values (case-insensitive), otherwise the first value.
func oneOf(v string, allowed []string) string {
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimSpace(v), a) {
			return a
		}
	}
	return allowed[0]
}

func trackCount(specific, def int) int {
	if specific > 0 {
		return clamp(specific, 1, maxTracksPerPlaylist)
	}
	return def
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
