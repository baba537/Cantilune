package catalog

import "strings"

// Settings saved by versions before 1.1.0 use German labels. These aliases
// map them to the current English values so existing installations keep
// their configuration.

var legacyPresetNames = map[string]string{
	"Elektro":              "Electronic",
	"Elektronisch":         "Electronic",
	"Weltmusik":            "World Music",
	"Klassik":              "Classical",
	"Akustik":              "Acoustic",
	"Deutsch":              "German",
	"80er & 90er":          "80s & 90s",
	"Klassiker":            "Classics",
	"Mitsingen":            "Sing-Along",
	"Gute Laune":           "Good Mood",
	"Sanft":                "Gentle",
	"Pop-Hits":             "Pop Hits",
	"2000er":               "2000s",
	"Power-Balladen":       "Power Ballads",
	"Neoklassik":           "Neoclassical",
	"Klavier":              "Piano",
	"Naturklänge":          "Nature Sounds",
	"Melancholie":          "Melancholy",
	"90er":                 "90s",
	"Schlager & Partyhits": "Schlager & Party Hits",
	"Balladen":             "Ballads",
	"Kinderlieder":         "Kids Songs",
	"Gute-Laune-Pop":       "Feel-Good Pop",
	"Episch":               "Epic",
}

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

// Canonical maps a German label from an earlier version to its English value.
// Other values are returned unchanged.
func Canonical(label string) string {
	if en, ok := legacyLabels[strings.ToLower(strings.TrimSpace(label))]; ok {
		return en
	}
	return label
}
