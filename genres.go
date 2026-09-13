package main

import (
	"sort"
	"strings"
	"unicode"
)

// genreAliases unifies common spellings (after normalizeGenre).
var genreAliases = map[string]string{
	"dnb":         "drumandbass",
	"drumnbass":   "drumandbass",
	"rnb":         "randb",
	"hiphoprap":   "hiphop",
	"electronica": "electronic",
	"lofihiphop":  "lofi",
	"soundtracks": "soundtrack",
	"filmmusik":   "soundtrack",
	"klassik":     "classical",
}

// normalizeGenre makes genre names comparable: "Hip-Hop", "hip hop" and
// "HipHop" become "hiphop", "Drum & Bass" and "Drum'n'Bass" become "drumandbass".
func normalizeGenre(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("&", " and ", "'n'", " and ", " n ", " and ", "+", " and ").Replace(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	n := b.String()
	if a, ok := genreAliases[n]; ok {
		return a
	}
	return n
}

// genreMatches reports whether a library genre fits a wanted genre (both
// normalized). From 4 characters on, a partial match is enough, so "Hardstyle"
// also finds "Euphoric Hardstyle" and "House" finds "Deep House".
func genreMatches(library, wanted string) bool {
	if library == "" || wanted == "" {
		return false
	}
	if library == wanted {
		return true
	}
	return len(wanted) >= 4 && strings.Contains(library, wanted)
}

func matchesAnyGenre(names []string, wanted []string) bool {
	for _, n := range names {
		nn := normalizeGenre(n)
		for _, w := range wanted {
			if genreMatches(nn, normalizeGenre(w)) {
				return true
			}
		}
	}
	return false
}

// resolveGenres maps the wanted genres to the actual genre names in the
// library, because getRandomSongs expects exact names.
func resolveGenres(library []libraryGenre, wanted []string) (matched []libraryGenre, missing []string) {
	type hit struct {
		g     libraryGenre
		exact bool
	}
	hits := map[string]hit{}
	for _, w := range wanted {
		nw := normalizeGenre(w)
		found := false
		for _, g := range library {
			ng := normalizeGenre(g.Name)
			if !genreMatches(ng, nw) {
				continue
			}
			found = true
			h := hits[g.Name]
			h.g = g
			h.exact = h.exact || ng == nw
			hits[g.Name] = h
		}
		if !found {
			missing = append(missing, w)
		}
	}
	for _, h := range hits {
		matched = append(matched, h.g)
	}
	sort.Slice(matched, func(i, j int) bool {
		ei, ej := hits[matched[i].Name].exact, hits[matched[j].Name].exact
		if ei != ej {
			return ei
		}
		if matched[i].SongCount != matched[j].SongCount {
			return matched[i].SongCount > matched[j].SongCount
		}
		return matched[i].Name < matched[j].Name
	})
	if len(matched) > maxGenresPerPlaylist {
		matched = matched[:maxGenresPerPlaylist]
	}
	return matched, missing
}

// similarGenres suggests library genres similar to a missing genre.
func similarGenres(library []libraryGenre, wanted string, limit int) []string {
	r := []rune(normalizeGenre(wanted))
	if len(r) < 3 {
		return nil
	}
	if len(r) > 4 {
		r = r[:4]
	}
	nw := string(r)
	var out []string
	for _, g := range library {
		if strings.Contains(normalizeGenre(g.Name), nw) {
			out = append(out, g.Name)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func songGenres(s song) []string {
	names := make([]string, 0, len(s.Genres)+1)
	for _, g := range s.Genres {
		names = append(names, g.Name)
	}
	if len(names) == 0 && s.Genre != "" {
		names = append(names, s.Genre)
	}
	return names
}
