// Package catalog enthält die eingebauten Alltagssituationen mit ihren Presets
// (presets.json) sowie die Standardwerte der Plugin-Einstellungen.
//
// presets.json ist die einzige Quelle für Situationen und Presets:
// manifest.json wird daraus mit `go generate` erzeugt, und das Plugin bettet
// die Datei zur Laufzeit ein.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed presets.json
var presetsJSON []byte

// Energie-Stufen und Verläufe (Werte in presets.json).
const (
	EnergyLow    = "low"
	EnergyMedium = "medium"
	EnergyHigh   = "high"

	FlowShuffle = "shuffle"
	FlowRising  = "rising"
	FlowFalling = "falling"
)

// Auswahl-Modi (Dropdown pro Situation). Die Texte sind gleichzeitig die
// gespeicherten Werte – nicht umbenennen, sonst fallen gespeicherte
// Einstellungen auf den Standard zurück.
const (
	ModeBalanced      = "Ausgewogen"
	ModeFavorites     = "Lieblingssongs"
	ModeDiscover      = "Entdecken"
	ModeRecentlyAdded = "Neu hinzugefügt"
)

// Modes listet alle Auswahl-Modi in Anzeigereihenfolge.
var Modes = []string{ModeBalanced, ModeFavorites, ModeDiscover, ModeRecentlyAdded}

// Energie- und Verlaufs-Bezeichnungen für eigene Situationen im Web-UI.
var (
	EnergyLabels = map[string]string{"egal": "", "ruhig": EnergyLow, "mittel": EnergyMedium, "energiegeladen": EnergyHigh}
	FlowLabels   = map[string]string{"zufällig": FlowShuffle, "ansteigend": FlowRising, "abklingend": FlowFalling}
	EnergyOrder  = []string{"egal", "ruhig", "mittel", "energiegeladen"}
	FlowOrder    = []string{"zufällig", "ansteigend", "abklingend"}
)

// Standardwerte der globalen Einstellungen.
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

// DefaultExcludeGenres werden nie ausgewählt, außer ein Preset verlangt sie ausdrücklich.
var DefaultExcludeGenres = []string{
	"Audiobook", "Hörbuch", "Hörspiel", "Podcast", "Spoken Word", "Comedy",
	"Christmas", "Weihnachten", "Kinderlieder", "Children's Music",
}

// Recipe beschreibt, welche Songs in eine Playlist passen.
// Leere bzw. 0-Werte bedeuten "keine Einschränkung".
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

// Preset ist eine wählbare Variante einer Situation.
type Preset struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji,omitempty"`
	// Mix kombiniert die Genres aller anderen Presets der Situation.
	Mix bool `json:"mix,omitempty"`
	Recipe
}

// Situation ist eine eingebaute Alltagssituation.
type Situation struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Emoji    string   `json:"emoji"`
	Category string   `json:"category"`
	Enabled  bool     `json:"enabled"`
	Defaults Recipe   `json:"defaults"`
	Presets  []Preset `json:"presets"`
}

// Category gruppiert Situationen im Web-UI.
type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Catalog ist der geladene Inhalt von presets.json.
type Catalog struct {
	Categories []Category  `json:"categories"`
	Situations []Situation `json:"situations"`
}

var loaded *Catalog

// Load liefert den eingebetteten Katalog (einmalig geparst).
func Load() (*Catalog, error) {
	if loaded != nil {
		return loaded, nil
	}
	var c Catalog
	if err := json.Unmarshal(presetsJSON, &c); err != nil {
		return nil, fmt.Errorf("presets.json ist ungültig: %w", err)
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
			return fmt.Errorf("presets.json: Situation ohne id/name")
		}
		if ids[s.ID] {
			return fmt.Errorf("presets.json: doppelte Situation %q", s.ID)
		}
		ids[s.ID] = true
		if !cats[s.Category] {
			return fmt.Errorf("presets.json: Situation %q hat unbekannte Kategorie %q", s.ID, s.Category)
		}
		if len(s.Presets) == 0 {
			return fmt.Errorf("presets.json: Situation %q hat keine Presets", s.ID)
		}
		labels := map[string]bool{}
		for _, p := range s.Presets {
			l := s.PresetLabel(p)
			if labels[l] {
				return fmt.Errorf("presets.json: Situation %q hat doppeltes Preset %q", s.ID, l)
			}
			labels[l] = true
		}
	}
	return nil
}

// Situation sucht eine Situation per ID.
func (c *Catalog) Situation(id string) (Situation, bool) {
	for _, s := range c.Situations {
		if s.ID == id {
			return s, true
		}
	}
	return Situation{}, false
}

// PresetLabel ist der Text im Dropdown, z. B. "Hardstyle ⚡" oder "Mix 💪".
func (s Situation) PresetLabel(p Preset) string {
	return strings.TrimSpace(p.Name + " " + s.PresetEmoji(p))
}

// PresetEmoji liefert das Emoji eines Presets (Fallback: Emoji der Situation).
func (s Situation) PresetEmoji(p Preset) string {
	if p.Emoji != "" {
		return p.Emoji
	}
	return s.Emoji
}

// PresetLabels listet alle Dropdown-Texte in Reihenfolge.
func (s Situation) PresetLabels() []string {
	out := make([]string, len(s.Presets))
	for i, p := range s.Presets {
		out[i] = s.PresetLabel(p)
	}
	return out
}

// FindPreset sucht ein Preset per Dropdown-Text; unbekannte Werte liefern das erste Preset.
func (s Situation) FindPreset(label string) (Preset, bool) {
	for _, p := range s.Presets {
		if s.PresetLabel(p) == label || strings.EqualFold(p.Name, strings.TrimSpace(label)) {
			return p, true
		}
	}
	return s.Presets[0], false
}

// Recipe kombiniert die Standardwerte der Situation mit dem Preset.
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

// Merge überschreibt die Werte von r mit allen gesetzten Werten aus o.
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

// UniqueFold entfernt leere Einträge und Duplikate (Groß-/Kleinschreibung egal).
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
