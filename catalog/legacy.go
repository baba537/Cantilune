package catalog

import "strings"

// Settings saved by versions before 1.1.0 use German labels. Preset names are
// resolved through the nameDe fields in presets.json; the remaining dropdown
// values are mapped here so existing installations keep their configuration.
var legacyLabels = map[string]string{
	// selection modes
	"ausgewogen":      ModeBalanced,
	"lieblingssongs":  ModeFavorites,
	"entdecken":       ModeDiscover,
	"neu hinzugefügt": ModeRecentlyAdded,
	// energy
	"egal":           "any",
	"ruhig":          "calm",
	"mittel":         "medium",
	"energiegeladen": "energetic",
	// flow
	"zufällig":   "random",
	"ansteigend": "rising",
	"abklingend": "falling",
}

// germanModes translates selection modes for German playlist comments.
var germanModes = map[string]string{
	ModeBalanced:      "Ausgewogen",
	ModeFavorites:     "Lieblingssongs",
	ModeDiscover:      "Entdecken",
	ModeRecentlyAdded: "Neu hinzugefügt",
}

// Canonical maps a German label from an earlier version to its English value.
// Other values are returned unchanged.
func Canonical(label string) string {
	if en, ok := legacyLabels[strings.ToLower(strings.TrimSpace(label))]; ok {
		return en
	}
	return label
}

// ModeName returns a selection mode in the given playlist language.
func ModeName(mode, language string) string {
	if language == LanguageGerman {
		if de, ok := germanModes[mode]; ok {
			return de
		}
	}
	return mode
}
