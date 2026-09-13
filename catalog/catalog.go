// Package catalog contains the built-in situations with their presets
// (presets.json) and the default values of the plugin settings.
//
// presets.json is the single source for situations and presets: manifest.json
// is generated from it with `go generate`, and the plugin embeds it at runtime.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed presets.json
var presetsJSON []byte

// Energy levels and flows (values used in presets.json).
const (
	EnergyLow    = "low"
	EnergyMedium = "medium"
	EnergyHigh   = "high"

	FlowShuffle = "shuffle"
	FlowRising  = "rising"
	FlowFalling = "falling"
)

// Selection modes (dropdown per situation). The labels are also the stored
// values; renaming them resets existing settings to the default unless an
// alias is added in legacy.go.
const (
	ModeBalanced      = "Balanced"
	ModeFavorites     = "Favorites"
	ModeDiscover      = "Discover"
	ModeRecentlyAdded = "Recently added"
)

// Modes lists all selection modes in display order.
var Modes = []string{ModeBalanced, ModeFavorites, ModeDiscover, ModeRecentlyAdded}

// Energy and flow labels for custom situations in the web UI.
var (
	EnergyLabels = map[string]string{"any": "", "calm": EnergyLow, "medium": EnergyMedium, "energetic": EnergyHigh}
	FlowLabels   = map[string]string{"random": FlowShuffle, "rising": FlowRising, "falling": FlowFalling}
	EnergyOrder  = []string{"any", "calm", "medium", "energetic"}
	FlowOrder    = []string{"random", "rising", "falling"}
)

// Default values of the global settings.
const (
	DefaultPrefix          = "🎧"
	DefaultHistoryDays     = 7
	DefaultTrackCount      = 50
	DefaultGenerationTime  = "04:00"
	DefaultCron            = "0 4 * * *"
	DefaultMaxPerArtist    = 3
	DefaultAvoidRecentDays = 3
	DefaultMinDuration     = 60
	DefaultMaxDuration     = 900
)

// DefaultExcludeGenres are never selected unless a preset explicitly asks for them.
// The list contains genre tags as they appear in libraries, including German ones.
var DefaultExcludeGenres = []string{
	"Audiobook", "Hörbuch", "Hörspiel", "Podcast", "Spoken Word", "Comedy",
	"Christmas", "Weihnachten", "Kinderlieder", "Children's Music",
}

// Recipe describes which songs fit a playlist.
// Empty or zero values mean "no restriction".
type Recipe struct {
	Genres          []string `json:"genres,omitempty"`
	ExcludeGenres   []string `json:"excludeGenres,omitempty"`
	Moods           []string `json:"moods,omitempty"`
	MinBPM          int      `json:"minBpm,omitempty"`
	MaxBPM          int      `json:"maxBpm,omitempty"`
	FromYear        int      `json:"fromYear,omitempty"`
	ToYear          int      `json:"toYear,omitempty"`
	Energy          string   `json:"energy,omitempty"`
	Flow            string   `json:"flow,omitempty"`
	MinDuration     int      `json:"minDuration,omitempty"`
	MaxDuration     int      `json:"maxDuration,omitempty"`
	ExcludeExplicit bool     `json:"excludeExplicit,omitempty"`
}

// Preset is a selectable variant of a situation.
type Preset struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji,omitempty"`
	// Mix combines the genres of all other presets of the situation.
	Mix bool `json:"mix,omitempty"`
	Recipe
}

// Situation is a built-in everyday situation.
type Situation struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Emoji    string   `json:"emoji"`
	Category string   `json:"category"`
	Enabled  bool     `json:"enabled"`
	Defaults Recipe   `json:"defaults"`
	Presets  []Preset `json:"presets"`
}

// Category groups situations in the web UI.
type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Catalog is the loaded content of presets.json.
type Catalog struct {
	Categories []Category  `json:"categories"`
	Situations []Situation `json:"situations"`
}

var loaded *Catalog

// Load returns the embedded catalog (parsed once).
func Load() (*Catalog, error) {
	if loaded != nil {
		return loaded, nil
	}
	var c Catalog
	if err := json.Unmarshal(presetsJSON, &c); err != nil {
		return nil, fmt.Errorf("invalid presets.json: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	loaded = &c
	return loaded, nil
}

func (c *Catalog) validate() error {
	cats := map[string]bool{}
	for _, cat := range c.Categories {
		cats[cat.ID] = true
	}
	ids := map[string]bool{}
	for _, s := range c.Situations {
		if s.ID == "" || s.Name == "" {
			return fmt.Errorf("presets.json: situation without id or name")
		}
		if ids[s.ID] {
			return fmt.Errorf("presets.json: duplicate situation %q", s.ID)
		}
		ids[s.ID] = true
		if !cats[s.Category] {
			return fmt.Errorf("presets.json: situation %q has unknown category %q", s.ID, s.Category)
		}
		if len(s.Presets) == 0 {
			return fmt.Errorf("presets.json: situation %q has no presets", s.ID)
		}
		labels := map[string]bool{}
		for _, p := range s.Presets {
			l := s.PresetLabel(p)
			if labels[l] {
				return fmt.Errorf("presets.json: situation %q has duplicate preset %q", s.ID, l)
			}
			labels[l] = true
		}
	}
	return nil
}

// Situation looks up a situation by ID.
func (c *Catalog) Situation(id string) (Situation, bool) {
	for _, s := range c.Situations {
		if s.ID == id {
			return s, true
		}
	}
	return Situation{}, false
}

// PresetLabel is the dropdown text, e.g. "Hardstyle ⚡" or "Mix 💪".
func (s Situation) PresetLabel(p Preset) string {
	return strings.TrimSpace(p.Name + " " + s.PresetEmoji(p))
}

// PresetEmoji returns the preset's emoji, falling back to the situation's emoji.
func (s Situation) PresetEmoji(p Preset) string {
	if p.Emoji != "" {
		return p.Emoji
	}
	return s.Emoji
}

// PresetLabels lists all dropdown texts in order.
func (s Situation) PresetLabels() []string {
	out := make([]string, len(s.Presets))
	for i, p := range s.Presets {
		out[i] = s.PresetLabel(p)
	}
	return out
}

// FindPreset looks up a preset by its dropdown text. Labels from earlier
// German versions are recognized. Unknown values return the first preset.
func (s Situation) FindPreset(label string) (Preset, bool) {
	label = strings.TrimSpace(label)
	name := strings.TrimSpace(strings.TrimSuffix(label, s.emojiOf(label)))
	if en, ok := legacyPresetNames[name]; ok {
		name = en
	}
	for _, p := range s.Presets {
		if s.PresetLabel(p) == label || strings.EqualFold(p.Name, label) || strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return s.Presets[0], false
}

// emojiOf returns the preset emoji that label ends with, if any.
func (s Situation) emojiOf(label string) string {
	for _, p := range s.Presets {
		if e := s.PresetEmoji(p); e != "" && strings.HasSuffix(label, e) {
			return e
		}
	}
	return ""
}

// Recipe combines the situation defaults with the preset.
func (s Situation) Recipe(p Preset) Recipe {
	r := s.Defaults.Merge(p.Recipe)
	if p.Mix && len(p.Genres) == 0 {
		var genres []string
		for _, other := range s.Presets {
			if !other.Mix {
				genres = append(genres, other.Genres...)
			}
		}
		r.Genres = UniqueFold(genres)
	}
	return r
}

// Merge overrides the values of r with all values set in o.
func (r Recipe) Merge(o Recipe) Recipe {
	if len(o.Genres) > 0 {
		r.Genres = o.Genres
	}
	if len(o.ExcludeGenres) > 0 {
		r.ExcludeGenres = o.ExcludeGenres
	}
	if len(o.Moods) > 0 {
		r.Moods = o.Moods
	}
	if o.MinBPM > 0 || o.MaxBPM > 0 {
		r.MinBPM, r.MaxBPM = o.MinBPM, o.MaxBPM
	}
	if o.FromYear > 0 {
		r.FromYear = o.FromYear
	}
	if o.ToYear > 0 {
		r.ToYear = o.ToYear
	}
	if o.Energy != "" {
		r.Energy = o.Energy
	}
	if o.Flow != "" {
		r.Flow = o.Flow
	}
	if o.MinDuration > 0 {
		r.MinDuration = o.MinDuration
	}
	if o.MaxDuration > 0 {
		r.MaxDuration = o.MaxDuration
	}
	if o.ExcludeExplicit {
		r.ExcludeExplicit = true
	}
	return r
}

// UniqueFold removes empty entries and case-insensitive duplicates.
func UniqueFold(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}
