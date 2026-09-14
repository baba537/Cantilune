package main

import (
	"sort"
	"strings"
	"unicode"
)

// genreAliases unifies common spellings (applied to the normalized form).
var genreAliases = map[string]string{
	"dnb":         "drumandbass",
	"rnb":         "randb",
	"hiphoprap":   "hiphop",
	"raphiphop":   "hiphop",
	"electronica": "electronic",
	"lofihiphop":  "lofi",
	"soundtracks": "soundtrack",
	"filmmusik":   "soundtrack",
	"klassik":     "classical",
	"worldmusic":  "world",
}

// genreQuality describes how well a genre name fits a wanted genre.
type genreQuality int

const (
	noMatch    genreQuality = iota
	subGenre                // more specific form, e.g. "Deep House" for "House"
	exactGenre              // same genre, spelling differences ignored
)

var andReplacer = strings.NewReplacer("&", " and ", "'n'", " and ", "’n’", " and ", "+", " and ")

// genreWordCache memoizes genreWords: the same few hundred genre names are
// compared many thousand times per run. The plugin runs single-threaded.
var genreWordCache = map[string][]string{}

const maxGenreWordCache = 5000

// genreWords splits a genre name into lowercase words. "&", "'n'" and a
// single "n" become "and", so "Drum & Bass", "Drum'n'Bass" and "Drum n Bass"
// are equal. The returned slice must not be modified.
func genreWords(s string) []string {
	if words, ok := genreWordCache[s]; ok {
		return words
	}
	words := strings.FieldsFunc(andReplacer.Replace(strings.ToLower(s)), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, w := range words {
		if w == "n" {
			words[i] = "and"
		}
	}
	if len(genreWordCache) >= maxGenreWordCache {
		genreWordCache = map[string][]string{}
	}
	genreWordCache[s] = words
	return words
}

func joinGenreWords(words []string) string {
	n := strings.Join(words, "")
	if a, ok := genreAliases[n]; ok {
		return a
	}
	return n
}

// normalizeGenre makes genre names comparable: "Hip-Hop", "hip hop" and
// "HipHop" become "hiphop".
func normalizeGenre(s string) string {
	return joinGenreWords(genreWords(s))
}

// matchGenre compares a genre name with a wanted genre. A sub-genre must end
// with the wanted genre as whole words, preceded by a qualifier of at least
// two characters: "Euphoric Hardstyle" fits "Hardstyle" and "Deep House" fits
// "House", while "Hardcore Punk" does not fit "Hardcore", "Dancehall" does not
// fit "Dance", "Neoclassical Metal" does not fit "Classical" and "K-Pop" does
// not fit "Pop".
func matchGenre(name, wanted string) genreQuality {
	nw := normalizeGenre(wanted)
	words := genreWords(name)
	if nw == "" || len(words) == 0 {
		return noMatch
	}
	if joinGenreWords(words) == nw {
		return exactGenre
	}
	for k := 1; k < len(words); k++ {
		if len(strings.Join(words[:k], "")) >= 2 && joinGenreWords(words[k:]) == nw {
			return subGenre
		}
	}
	return noMatch
}

// bestGenreMatch returns the best match of any name against any wanted genre.
func bestGenreMatch(names, wanted []string) genreQuality {
	best := noMatch
	for _, n := range names {
		for _, w := range wanted {
			if q := matchGenre(n, w); q > best {
				best = q
				if best == exactGenre {
					return best
				}
			}
		}
	}
	return best
}

// genreShare returns the fraction of names that match any wanted genre.
func genreShare(names, wanted []string) float64 {
	if len(names) == 0 {
		return 0
	}
	n := 0
	for _, name := range names {
		if bestGenreMatch([]string{name}, wanted) != noMatch {
			n++
		}
	}
	return float64(n) / float64(len(names))
}

// containsGenre reports whether name contains the excluded genre as whole
// words, so "Christmas" also excludes "Christmas Pop" but not "Christmastime".
func containsGenre(name, excluded string) bool {
	nx := normalizeGenre(excluded)
	words := genreWords(name)
	if nx == "" {
		return false
	}
	for i := 0; i < len(words); i++ {
		for j := i + 1; j <= len(words); j++ {
			if joinGenreWords(words[i:j]) == nx {
				return true
			}
		}
	}
	return false
}

func containsAnyGenre(names, excluded []string) bool {
	for _, n := range names {
		for _, e := range excluded {
			if containsGenre(n, e) {
				return true
			}
		}
	}
	return false
}

// genreMatch is a library genre matched to one wanted genre.
type genreMatch struct {
	libraryGenre
	Wanted  string
	Quality genreQuality
}

// resolveGenres maps each wanted genre to the genre names that exist in the
// library, because getRandomSongs only filters by existing names. The result
// is grouped by wanted genre; exact matches and larger genres come first.
func resolveGenres(library []libraryGenre, wanted []string) (groups [][]genreMatch, missing []string) {
	for _, w := range wanted {
		var group []genreMatch
		for _, g := range library {
			if q := matchGenre(g.Name, w); q != noMatch {
				group = append(group, genreMatch{libraryGenre: g, Wanted: w, Quality: q})
			}
		}
		if len(group) == 0 {
			missing = append(missing, w)
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			if group[i].Quality != group[j].Quality {
				return group[i].Quality > group[j].Quality
			}
			if group[i].SongCount != group[j].SongCount {
				return group[i].SongCount > group[j].SongCount
			}
			return group[i].Name < group[j].Name
		})
		if len(group) > maxGenresPerWanted {
			group = group[:maxGenresPerWanted]
		}
		groups = append(groups, group)
	}
	return groups, missing
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
