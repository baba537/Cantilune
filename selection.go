package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

const (
	maxRandomSongsPerCall = 500 // Obergrenze von getRandomSongs
	maxTracksPerPlaylist  = 500
	maxGenresPerPlaylist  = 40
	maxPoolSize           = 1500
	poolFactor            = 6   // Kandidaten pro gewünschtem Track
	interludeMaxSeconds   = 150 // nur kurze "Intro"/"Skit"-Tracks gelten als Zwischenspiel
	day                   = 24 * time.Hour
)

// Gründe für aussortierte Songs (für das Log).
const (
	reasonExcludedGenre = "Genre ausgeschlossen"
	reasonDisliked      = "mit 1 Stern bewertet"
	reasonExplicit      = "explizit"
	reasonDuration      = "Länge"
	reasonInterlude     = "Intro/Skit"
	reasonYear          = "Jahr"
)

type candidate struct {
	s      song
	weight float64
	energy float64 // 0 (ruhig) … 1 (energiegeladen), -1 = unbekannt
	key    float64
}

type selectionStats struct {
	Genres     []libraryGenre
	Missing    []string
	Similar    []string
	Candidates int
	Rejected   map[string]int
	Relaxed    bool
	WithBPM    int
	WithGain   int
	Favorites  int
}

// selectSongs wählt die Songs einer Playlist aus:
//  1. Kandidaten per getRandomSongs laden (je passendem Bibliotheks-Genre)
//  2. harte Filter (ausgeschlossene Genres, 1-Stern, explizit, Länge, Intros)
//  3. Gewichtung nach BPM, Energie (BPM + ReplayGain), Stimmung, Favoriten,
//     Bewertung, Wiedergaben, zuletzt gehört und Vortags-Playlist
//  4. gewichtete Zufallsauswahl mit Obergrenze pro Künstler
//  5. Reihenfolge nach Verlauf (zufällig, ansteigend, abklingend)
func (g *generator) selectSongs(job Job, recent map[string]int) ([]string, selectionStats, error) {
	st := selectionStats{Rejected: map[string]int{}}
	pool, err := g.gatherCandidates(job, &st)
	if err != nil {
		return nil, st, err
	}
	st.Candidates = len(pool)

	kept := g.applyFilters(pool, job.Recipe, true, st.Rejected)
	if len(kept) < job.Count && st.Rejected[reasonDuration]+st.Rejected[reasonInterlude] > 0 {
		st.Rejected = map[string]int{}
		kept = g.applyFilters(pool, job.Recipe, false, st.Rejected)
		st.Relaxed = true
	}
	if len(kept) == 0 {
		return nil, st, nil
	}

	now := nowFn()
	cands := make([]candidate, len(kept))
	for i, s := range kept {
		cands[i] = candidate{s: s, energy: songEnergy(s), weight: g.score(s, job, recent, now)}
	}
	chosen := g.pick(cands, job.Count)
	for _, c := range chosen {
		if c.s.BPM > 0 {
			st.WithBPM++
		}
		if replayGain(c.s) != nil {
			st.WithGain++
		}
		if c.s.Starred != nil || c.s.UserRating >= 4 {
			st.Favorites++
		}
	}

	songs := g.order(chosen, job.Recipe.Flow)
	ids := make([]string, len(songs))
	for i, s := range songs {
		ids[i] = s.ID
	}
	return ids, st, nil
}

func (g *generator) gatherCandidates(job Job, st *selectionStats) ([]song, error) {
	r := job.Recipe
	target := clamp(job.Count*poolFactor, 150, maxPoolSize)
	seen := map[string]bool{}
	var pool []song
	add := func(songs []song) int {
		n := 0
		for _, s := range songs {
			if s.ID != "" && !seen[s.ID] {
				seen[s.ID] = true
				pool = append(pool, s)
				n++
			}
		}
		return n
	}

	// Ohne Genres: gesamte Bibliothek (ggf. nach Jahren gefiltert).
	if len(r.Genres) == 0 {
		for round := 0; round < 3 && len(pool) < target; round++ {
			size := clamp(target-len(pool), 1, maxRandomSongsPerCall)
			songs, err := fetchRandomSongs(job.User, "", size, r.FromYear, r.ToYear)
			if err != nil {
				if round == 0 {
					return nil, err
				}
				break
			}
			if add(songs) == 0 || len(songs) < size {
				break
			}
		}
		return pool, nil
	}

	library := g.libraryGenres(job.User)
	var matched []libraryGenre
	if library == nil {
		for _, w := range r.Genres {
			matched = append(matched, libraryGenre{Name: w})
		}
	} else {
		matched, st.Missing = resolveGenres(library, r.Genres)
		if len(matched) == 0 {
			for _, m := range st.Missing {
				st.Similar = append(st.Similar, similarGenres(library, m, 3)...)
			}
			st.Similar = catalog.UniqueFold(st.Similar)
		}
	}
	st.Genres = matched
	if len(matched) == 0 {
		return nil, nil
	}

	per := max((target+len(matched)-1)/len(matched), 20)
	var lastErr error
	succeeded := 0
	for _, gen := range matched {
		size := per
		if gen.SongCount > 0 && gen.SongCount < size {
			size = gen.SongCount
		}
		songs, err := fetchRandomSongs(job.User, gen.Name, size, r.FromYear, r.ToYear)
		if err != nil {
			logf(pdk.LogWarn, "%s: Songs für Genre %q konnten nicht geladen werden: %v", job.Name, gen.Name, err)
			lastErr = err
			continue
		}
		succeeded++
		add(songs)
	}
	if succeeded == 0 && lastErr != nil {
		return nil, lastErr
	}
	return pool, nil
}

// libraryGenres lädt die Genre-Liste einmal pro Benutzer; nil = nicht verfügbar.
func (g *generator) libraryGenres(user string) []libraryGenre {
	key := strings.ToLower(user)
	if list, ok := g.genreCache[key]; ok {
		return list
	}
	list, err := fetchGenres(user)
	if err != nil {
		logf(pdk.LogWarn, "Genre-Liste nicht verfügbar (%v) – Genres werden ungeprüft verwendet", err)
		list = nil
	}
	g.genreCache[key] = list
	return list
}

func (g *generator) applyFilters(pool []song, r catalog.Recipe, strict bool, rejected map[string]int) []song {
	// Globale Ausschlüsse gelten nicht, wenn das Rezept das Genre ausdrücklich verlangt.
	var excludes []string
	for _, e := range g.cfg.ExcludeGenres {
		if !matchesAnyGenre([]string{e}, r.Genres) && !matchesAnyGenre(r.Genres, []string{e}) {
			excludes = append(excludes, e)
		}
	}
	excludes = append(excludes, r.ExcludeGenres...)

	minDur := r.MinDuration
	if minDur == 0 {
		minDur = catalog.DefaultMinDuration
	}
	maxDur := r.MaxDuration
	if maxDur == 0 {
		maxDur = catalog.DefaultMaxDuration
	}

	kept := make([]song, 0, len(pool))
	for _, s := range pool {
		reason := ""
		switch {
		case len(excludes) > 0 && matchesAnyGenre(songGenres(s), excludes):
			reason = reasonExcludedGenre
		case s.UserRating == 1:
			reason = reasonDisliked
		case r.ExcludeExplicit && strings.EqualFold(s.ExplicitStatus, "explicit"):
			reason = reasonExplicit
		case s.Year > 0 && ((r.FromYear > 0 && s.Year < r.FromYear) || (r.ToYear > 0 && s.Year > r.ToYear)):
			reason = reasonYear
		case strict && s.Duration > 0 && (s.Duration < minDur || s.Duration > maxDur):
			reason = reasonDuration
		case strict && g.cfg.SkipInterludes && isInterlude(s):
			reason = reasonInterlude
		}
		if reason != "" {
			rejected[reason]++
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// score berechnet das Auswahlgewicht eines Songs (1 = neutral).
func (g *generator) score(s song, job Job, recent map[string]int, now time.Time) float64 {
	r := job.Recipe
	w := 1.0
	starred := s.Starred != nil
	familiarity := math.Log2(1 + float64(s.PlayCount))

	switch job.Mode {
	case catalog.ModeFavorites:
		if starred {
			w *= 4
		}
		w *= ratingFactor(s.UserRating, 4, 2.5, 0.3)
		w *= 1 + math.Min(2, 0.35*familiarity)
		if !starred && s.UserRating == 0 && s.PlayCount == 0 {
			w *= 0.3
		}
	case catalog.ModeDiscover:
		switch {
		case s.PlayCount == 0:
			w *= 3
		case s.PlayCount <= 2:
			w *= 1.6
		case s.PlayCount >= 10:
			w *= 0.4
		}
		if s.Played != nil && now.Sub(*s.Played) > 180*day {
			w *= 1.5
		}
		w *= ratingFactor(s.UserRating, 1.2, 1.1, 0.3)
	case catalog.ModeRecentlyAdded:
		if s.Created != nil {
			switch age := now.Sub(*s.Created); {
			case age <= 30*day:
				w *= 5
			case age <= 90*day:
				w *= 2.5
			case age <= 365*day:
				w *= 1.2
			default:
				w *= 0.5
			}
		}
		w *= ratingFactor(s.UserRating, 1.5, 1.2, 0.4)
	default: // Ausgewogen
		if starred {
			w *= 1.8
		}
		w *= ratingFactor(s.UserRating, 2, 1.5, 0.5)
		w *= 1 + math.Min(0.5, 0.1*familiarity)
	}

	if g.cfg.AvoidRecentDays > 0 && s.Played != nil && now.Sub(*s.Played) < time.Duration(g.cfg.AvoidRecentDays)*day {
		w *= 0.3
	}
	if age, ok := recent[s.ID]; ok {
		w *= repeatFactor(age) // Abwechslung zu den Playlists der letzten Tage
	}
	w *= bpmFactor(s.BPM, r.MinBPM, r.MaxBPM)
	w *= moodFactor(s.Moods, r.Moods)
	if r.Energy != "" {
		if e := songEnergy(s); e >= 0 {
			w *= energyFactor(r.Energy, e)
		}
	}
	return math.Max(w, 0.001)
}

// ratingFactor gewichtet die Sterne-Bewertung des Benutzers (0 = unbewertet).
func ratingFactor(rating int, five, four, two float64) float64 {
	switch rating {
	case 5:
		return five
	case 4:
		return four
	case 2:
		return two
	}
	return 1
}

// bpmFactor bevorzugt Songs im BPM-Bereich. Halbe/doppelte BPM zählen mit,
// da Taggern oft Half-/Double-Time erkennen (z. B. Hardstyle 75 statt 150).
func bpmFactor(bpm, minBPM, maxBPM int) float64 {
	if bpm <= 0 || (minBPM <= 0 && maxBPM <= 0) {
		return 1
	}
	lo, hi := float64(minBPM), float64(maxBPM)
	if maxBPM <= 0 {
		hi = math.Inf(1)
	}
	best := math.Inf(1)
	for _, b := range []float64{float64(bpm), float64(bpm) * 2, float64(bpm) / 2} {
		d := 0.0
		if b < lo {
			d = (lo - b) / lo
		} else if b > hi {
			d = (b - hi) / hi
		}
		best = math.Min(best, d)
	}
	switch {
	case best == 0:
		return 1.6
	case best <= 0.08:
		return 1
	default:
		return 0.3
	}
}

func moodFactor(songMoods, wanted []string) float64 {
	if len(wanted) == 0 || len(songMoods) == 0 {
		return 1
	}
	for _, m := range songMoods {
		for _, w := range wanted {
			if strings.EqualFold(strings.TrimSpace(m), strings.TrimSpace(w)) {
				return 2
			}
		}
	}
	return 0.6
}

// songEnergy schätzt die Energie aus BPM und ReplayGain (laut gemasterte Songs
// haben stark negative Gain-Werte). -1, wenn keine Daten vorliegen.
func songEnergy(s song) float64 {
	sum, n := 0.0, 0
	if s.BPM > 0 {
		b := float64(s.BPM)
		for b > 190 {
			b /= 2
		}
		for b < 60 {
			b *= 2
		}
		sum += clamp01((b - 70) / 70) // 70 BPM → 0, 140 BPM → 1
		n++
	}
	if gain := replayGain(s); gain != nil {
		sum += clamp01((2 - *gain) / 12) // +2 dB → 0, −10 dB → 1
		n++
	}
	if n == 0 {
		return -1
	}
	return sum / float64(n)
}

func replayGain(s song) *float64 {
	if s.ReplayGain.TrackGain != nil {
		return s.ReplayGain.TrackGain
	}
	return s.ReplayGain.AlbumGain
}

func energyFactor(target string, e float64) float64 {
	switch target {
	case catalog.EnergyHigh:
		return 0.4 + 1.4*e
	case catalog.EnergyLow:
		return 0.4 + 1.4*(1-e)
	case catalog.EnergyMedium:
		return 1.4 - 1.6*math.Abs(e-0.5)
	}
	return 1
}

// pick zieht gewichtet ohne Zurücklegen (Efraimidis-Spirakis) und begrenzt
// die Songs pro Künstler; reicht das nicht, wird ohne Grenze aufgefüllt.
func (g *generator) pick(cands []candidate, count int) []candidate {
	for i := range cands {
		u := g.rng.Float64()
		for u == 0 {
			u = g.rng.Float64()
		}
		cands[i].key = -math.Log(u) / cands[i].weight
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].key < cands[j].key })

	chosen := make([]candidate, 0, count)
	used := make([]bool, len(cands))
	if limit := g.cfg.MaxPerArtist; limit > 0 {
		perArtist := map[string]int{}
		for i, c := range cands {
			if len(chosen) >= count {
				break
			}
			a := artistKey(c.s)
			if perArtist[a] >= limit {
				continue
			}
			perArtist[a]++
			used[i] = true
			chosen = append(chosen, c)
		}
	}
	for i, c := range cands {
		if len(chosen) >= count {
			break
		}
		if !used[i] {
			chosen = append(chosen, c)
		}
	}
	return chosen
}

// order sortiert nach Verlauf und verteilt gleiche Künstler.
func (g *generator) order(chosen []candidate, flow string) []song {
	sorted := false
	if flow == catalog.FlowRising || flow == catalog.FlowFalling {
		known := 0
		for _, c := range chosen {
			if c.energy >= 0 {
				known++
			}
		}
		if known*2 >= len(chosen) && known > 0 {
			for i := range chosen {
				e := chosen[i].energy
				if e < 0 {
					e = 0.5
				}
				chosen[i].key = e + (g.rng.Float64()-0.5)*0.25
			}
			sort.SliceStable(chosen, func(i, j int) bool {
				if flow == catalog.FlowFalling {
					return chosen[i].key > chosen[j].key
				}
				return chosen[i].key < chosen[j].key
			})
			sorted = true
		}
	}
	if !sorted {
		g.rng.Shuffle(len(chosen), func(i, j int) { chosen[i], chosen[j] = chosen[j], chosen[i] })
	}
	songs := make([]song, len(chosen))
	for i, c := range chosen {
		songs[i] = c.s
	}
	spreadArtists(songs)
	return songs
}

// spreadArtists vermeidet denselben Künstler direkt hintereinander.
func spreadArtists(songs []song) {
	const window = 8
	for i := 1; i < len(songs); i++ {
		prev := artistKey(songs[i-1])
		if artistKey(songs[i]) != prev {
			continue
		}
		for j := i + 1; j < len(songs) && j <= i+window; j++ {
			if artistKey(songs[j]) != prev {
				songs[i], songs[j] = songs[j], songs[i]
				break
			}
		}
	}
}

func artistKey(s song) string {
	if s.ArtistID != "" {
		return s.ArtistID
	}
	return strings.ToLower(strings.TrimSpace(s.Artist))
}

func isInterlude(s song) bool {
	if s.Duration <= 0 || s.Duration > interludeMaxSeconds {
		return false
	}
	words := strings.FieldsFunc(strings.ToLower(s.Title), func(r rune) bool { return !unicode.IsLetter(r) })
	for _, w := range words {
		switch w {
		case "intro", "outro", "skit", "interlude", "intermission", "prelude", "reprise":
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

// summary fasst die Auswahl für das Log zusammen.
func (st selectionStats) summary(chosen int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d Songs", chosen)
	if len(st.Genres) > 0 {
		names := make([]string, 0, 8)
		for i, g := range st.Genres {
			if i == 8 {
				names = append(names, fmt.Sprintf("+%d weitere", len(st.Genres)-8))
				break
			}
			if g.SongCount > 0 {
				names = append(names, fmt.Sprintf("%s (%d)", g.Name, g.SongCount))
			} else {
				names = append(names, g.Name)
			}
		}
		fmt.Fprintf(&b, " · Genres: %s", strings.Join(names, ", "))
	}
	if len(st.Missing) > 0 {
		fmt.Fprintf(&b, " · nicht in der Bibliothek: %s", strings.Join(st.Missing, ", "))
	}
	fmt.Fprintf(&b, " · Kandidaten: %d", st.Candidates)
	if len(st.Rejected) > 0 {
		keys := make([]string, 0, len(st.Rejected))
		for k := range st.Rejected {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = fmt.Sprintf("%s %d", k, st.Rejected[k])
		}
		fmt.Fprintf(&b, " · aussortiert: %s", strings.Join(parts, ", "))
	}
	if st.Relaxed {
		b.WriteString(" (Längen-/Intro-Filter gelockert)")
	}
	if chosen > 0 {
		fmt.Fprintf(&b, " · Metadaten: %d mit BPM, %d mit ReplayGain, %d Favoriten", st.WithBPM, st.WithGain, st.Favorites)
	}
	return b.String()
}
