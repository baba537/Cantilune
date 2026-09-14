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
	maxRandomSongsPerCall = 500 // limit of getRandomSongs
	maxTracksPerPlaylist  = 500
	maxGenresPerWanted    = 12 // library genres queried per wanted genre
	maxGenreQueries       = 40 // getRandomSongs calls per playlist
	minSongsPerGenre      = 10
	maxPoolSize           = 1500
	maxAddedSongs         = 25  // songs added by the user that are loaded individually
	poolFactor            = 6   // candidates per requested track
	interludeMaxSeconds   = 150 // only short "intro"/"skit" tracks count as interludes
	maxTransition         = 2.4 // largest value of transition()
	day                   = 24 * time.Hour
)

// Reasons for filtered songs (for the log).
const (
	reasonExcludedGenre = "excluded genre"
	reasonGenreMismatch = "genre mismatch"
	reasonRemoved       = "removed by user"
	reasonDisliked      = "rated 1 star"
	reasonExplicit      = "explicit"
	reasonDuration      = "duration"
	reasonInterlude     = "intro/skit"
	reasonYear          = "year"
)

// songContext holds data about earlier playlists of the same situation and owner.
type songContext struct {
	Recent   map[string]int // song ID → days since it was in a playlist (0 = current playlist)
	Feedback map[string]int // song ID → -1 removed or +1 added by the user
}

// weightParts are the factor groups that make up the weight of a song.
// Every group is 1 when neutral; the weight is their product.
type weightParts struct {
	Genre      float64 // fit of the genre tags
	Tempo      float64 // BPM, mood and energy
	Preference float64 // favorites, ratings, play count, selection mode
	Variety    float64 // recently played, recent playlists
	Feedback   float64 // songs added to the playlist by the user
}

func (p weightParts) total() float64 {
	return math.Max(p.Genre*p.Tempo*p.Preference*p.Variety*p.Feedback, 0.001)
}

type candidate struct {
	s      song
	parts  weightParts
	weight float64
	energy float64  // 0 (calm) … 1 (energetic), -1 = unknown
	genres []string // normalized genre names
	key    float64
	group  int // index of the wanted genre the song was loaded for, -1 = none
}

// songDetail explains why a song was chosen.
type songDetail struct {
	Song   song
	Parts  weightParts
	Weight float64
}

// playlistQuality describes a generated playlist with simple, comparable metrics.
type playlistQuality struct {
	ExactGenre    float64 // share of songs tagged exactly with a wanted genre, -1 = preset without genres
	UniqueArtists float64 // distinct artists / songs
	Repeats       float64 // share of songs that were already in the previous playlist
	Tagged        float64 // share of songs with BPM or ReplayGain tags
	Coherence     float64 // 0 … 1, similarity of neighbouring songs (genre, year, energy)
}

type selectionStats struct {
	Genres     []genreMatch
	Missing    []string
	Similar    []string
	Candidates int
	Rejected   map[string]int
	Relaxed    bool
	Added      int
	WithBPM    int
	WithGain   int
	Favorites  int
	Quality    playlistQuality
	Details    []songDetail
}

// selectSongs picks the songs of a playlist:
//  1. load candidates with getRandomSongs (per matching library genre) plus
//     songs the user added to the previous playlist
//  2. hard filters (excluded genres, genre mismatch, removed by the user,
//     1 star, explicit, duration, intros)
//  3. weighting by genre fit, tempo/energy/mood, favorites/ratings and variety
//  4. weighted random selection with a limit per artist and per wanted genre
//  5. ordering by flow (rising, falling) or by similarity of neighbouring songs
func (g *generator) selectSongs(job Job, ctx songContext) ([]string, selectionStats, error) {
	st := selectionStats{Rejected: map[string]int{}}
	pool, groupOf, err := g.gatherCandidates(job, &st)
	if err != nil {
		return nil, st, err
	}
	pool = g.addUserSongs(job, ctx, pool, groupOf, &st)
	st.Candidates = len(pool)

	kept := g.applyFilters(pool, job.Recipe, ctx, true, st.Rejected)
	if len(kept) < job.Count && st.Rejected[reasonDuration]+st.Rejected[reasonInterlude] > 0 {
		st.Rejected = map[string]int{}
		kept = g.applyFilters(pool, job.Recipe, ctx, false, st.Rejected)
		st.Relaxed = true
	}
	if len(kept) == 0 {
		return nil, st, nil
	}

	now := nowFn()
	cands := make([]candidate, len(kept))
	for i, s := range kept {
		group, ok := groupOf[s.ID]
		if !ok {
			group = -1
		}
		parts := g.scoreParts(s, job, ctx, now)
		genres := songGenres(s)
		normalized := make([]string, len(genres))
		for k, name := range genres {
			normalized[k] = normalizeGenre(name)
		}
		cands[i] = candidate{s: s, parts: parts, weight: parts.total(), energy: songEnergy(s), genres: normalized, group: group}
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

	ordered := g.order(chosen, job.Recipe.Flow)
	st.Quality = measureQuality(ordered, job.Recipe, ctx)
	ids := make([]string, len(ordered))
	for i, c := range ordered {
		ids[i] = c.s.ID
		if g.cfg.LogDetails {
			st.Details = append(st.Details, songDetail{Song: c.s, Parts: c.parts, Weight: c.weight})
		}
	}
	return ids, st, nil
}

// gatherCandidates loads the candidate pool. groupOf maps each song ID to the
// index of the wanted genre it was loaded for (-1 without genres).
func (g *generator) gatherCandidates(job Job, st *selectionStats) (pool []song, groupOf map[string]int, err error) {
	r := job.Recipe
	target := clamp(job.Count*poolFactor, 150, maxPoolSize)
	groupOf = map[string]int{}
	add := func(songs []song, group int) int {
		n := 0
		for _, s := range songs {
			if _, dup := groupOf[s.ID]; s.ID != "" && !dup {
				groupOf[s.ID] = group
				pool = append(pool, s)
				n++
			}
		}
		return n
	}

	// Without genres: the whole library (optionally filtered by year).
	if len(r.Genres) == 0 {
		for round := 0; round < 3 && len(pool) < target; round++ {
			size := clamp(target-len(pool), 1, maxRandomSongsPerCall)
			songs, err := fetchRandomSongs(job.User, "", size, r.FromYear, r.ToYear)
			if err != nil {
				if round == 0 {
					return nil, nil, err
				}
				break
			}
			if add(songs, -1) == 0 || len(songs) < size {
				break
			}
		}
		return pool, groupOf, nil
	}

	library := g.libraryGenres(job.User)
	var groups [][]genreMatch
	if library == nil {
		for _, w := range r.Genres {
			groups = append(groups, []genreMatch{{libraryGenre: libraryGenre{Name: w}, Wanted: w, Quality: exactGenre}})
		}
	} else {
		groups, st.Missing = resolveGenres(library, r.Genres)
		if len(groups) == 0 {
			for _, m := range st.Missing {
				st.Similar = append(st.Similar, similarGenres(library, m, 3)...)
			}
			st.Similar = catalog.UniqueFold(st.Similar)
		}
	}
	if len(groups) == 0 {
		return nil, groupOf, nil
	}

	// Limit the number of queries: every wanted genre keeps its best matches.
	perGroupQueries := max(maxGenreQueries/len(groups), 1)
	for i := range groups {
		if len(groups[i]) > perGroupQueries {
			groups[i] = groups[i][:perGroupQueries]
		}
	}

	// Every wanted genre gets the same share of the pool, so a genre with many
	// sub-genres in the library (e.g. House) does not crowd out the others.
	// Within a wanted genre, the share is split by the size of each library genre.
	sizes := map[string]int{}
	groupOfGenre := map[string]int{}
	var order []genreMatch
	perWanted := max((target+len(groups)-1)/len(groups), minSongsPerGenre)
	for gi, group := range groups {
		total := 0
		for _, m := range group {
			total += max(m.SongCount, 1)
		}
		for _, m := range group {
			size := max(perWanted*max(m.SongCount, 1)/total, minSongsPerGenre)
			if m.SongCount > 0 {
				size = min(size, m.SongCount)
			}
			if _, ok := sizes[m.Name]; !ok {
				order = append(order, m)
				groupOfGenre[m.Name] = gi
			}
			sizes[m.Name] += size
		}
	}
	st.Genres = order

	var lastErr error
	succeeded := 0
	for _, gen := range order {
		songs, err := fetchRandomSongs(job.User, gen.Name, min(sizes[gen.Name], maxRandomSongsPerCall), r.FromYear, r.ToYear)
		if err != nil {
			logf(pdk.LogWarn, "%s: could not load songs for genre %q: %v", job.Name, gen.Name, err)
			lastErr = err
			continue
		}
		succeeded++
		add(songs, groupOfGenre[gen.Name])
	}
	if succeeded == 0 && lastErr != nil {
		return nil, nil, lastErr
	}
	return pool, groupOf, nil
}

// addUserSongs loads songs the user added to the previous playlist, so they
// can appear again even if the random query did not return them.
func (g *generator) addUserSongs(job Job, ctx songContext, pool []song, groupOf map[string]int, st *selectionStats) []song {
	ids := make([]string, 0)
	for id, v := range ctx.Feedback {
		if _, loaded := groupOf[id]; v > 0 && !loaded {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > maxAddedSongs {
		ids = ids[:maxAddedSongs]
	}
	for _, id := range ids {
		s, err := fetchSong(job.User, id)
		if err != nil || s == nil || s.ID == "" {
			continue // the song may have been removed from the library
		}
		groupOf[s.ID] = -1
		pool = append(pool, *s)
		st.Added++
	}
	return pool
}

// libraryGenres loads the genre list once per user; nil = unavailable.
func (g *generator) libraryGenres(user string) []libraryGenre {
	key := strings.ToLower(user)
	if list, ok := g.genreCache[key]; ok {
		return list
	}
	list, err := fetchGenres(user)
	if err != nil {
		logf(pdk.LogWarn, "genre list unavailable (%v), using genres unchecked", err)
		list = nil
	}
	g.genreCache[key] = list
	return list
}

func (g *generator) applyFilters(pool []song, r catalog.Recipe, ctx songContext, strict bool, rejected map[string]int) []song {
	// Global exclusions do not apply if the recipe explicitly asks for the genre.
	var excludes []string
	for _, e := range g.cfg.ExcludeGenres {
		if !containsAnyGenre(r.Genres, []string{e}) && bestGenreMatch([]string{e}, r.Genres) == noMatch {
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
		addedByUser := ctx.Feedback[s.ID] > 0
		reason := ""
		switch {
		case ctx.Feedback[s.ID] < 0:
			reason = reasonRemoved
		case len(excludes) > 0 && containsAnyGenre(songGenres(s), excludes):
			reason = reasonExcludedGenre
		// Navidrome filters genres with SQL LIKE, so "_" and "%" in a genre name
		// can match other genres. Verify the tags of every song.
		case len(r.Genres) > 0 && !addedByUser && bestGenreMatch(songGenres(s), r.Genres) == noMatch:
			reason = reasonGenreMismatch
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

// scoreParts calculates the factor groups of a song. The configured weights
// scale each group: factor^(weight/100), so 0 % ignores a group and 200 %
// doubles its effect on a logarithmic scale.
func (g *generator) scoreParts(s song, job Job, ctx songContext, now time.Time) weightParts {
	r := job.Recipe
	w := g.cfg.Weights

	variety := 1.0
	if g.cfg.AvoidRecentDays > 0 && s.Played != nil && now.Sub(*s.Played) < time.Duration(g.cfg.AvoidRecentDays)*day {
		variety *= 0.3
	}
	// Songs the user added to the playlist are wanted, so they are not treated as repeats.
	if age, ok := ctx.Recent[s.ID]; ok && ctx.Feedback[s.ID] <= 0 {
		variety *= repeatFactor(age)
	}

	tempo := bpmFactor(s.BPM, r.MinBPM, r.MaxBPM) * moodFactor(s.Moods, r.Moods)
	if r.Energy != "" {
		if e := songEnergy(s); e >= 0 {
			tempo *= energyFactor(r.Energy, e)
		}
	}

	feedback := 1.0
	if ctx.Feedback[s.ID] > 0 {
		feedback = 2.5
	}

	return weightParts{
		Genre:      weighted(genreFactor(songGenres(s), r.Genres), w.Genre),
		Tempo:      weighted(tempo, w.Tempo),
		Preference: weighted(preferenceFactor(s, job.Mode, now), w.Preference),
		Variety:    weighted(variety, w.Variety),
		Feedback:   feedback,
	}
}

func weighted(factor float64, percent int) float64 {
	if percent == catalog.DefaultWeight {
		return factor
	}
	return math.Pow(factor, float64(percent)/100)
}

// preferenceFactor weights favorites, ratings and play history according to the selection mode.
func preferenceFactor(s song, mode string, now time.Time) float64 {
	w := 1.0
	starred := s.Starred != nil
	familiarity := math.Log2(1 + float64(s.PlayCount))

	switch mode {
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
	default: // balanced
		if starred {
			w *= 1.8
		}
		w *= ratingFactor(s.UserRating, 2, 1.5, 0.5)
		w *= 1 + math.Min(0.5, 0.1*familiarity)
	}
	return w
}

// ratingFactor weights the user's star rating (0 = unrated).
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

// genreFactor prefers songs whose genre tags fit the recipe well: an exact
// genre beats a sub-genre, and a song tagged only with wanted genres beats one
// where the wanted genre is just one of many tags.
func genreFactor(names, wanted []string) float64 {
	if len(wanted) == 0 || len(names) == 0 {
		return 1
	}
	f := 0.6 + 0.4*genreShare(names, wanted)
	switch bestGenreMatch(names, wanted) {
	case exactGenre:
		return f
	case subGenre:
		return 0.8 * f
	}
	return 0.3
}

// bpmFactor prefers songs within the BPM range. Half and double BPM count as
// well, because taggers often detect half or double time (e.g. 75 instead of 150).
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
		return 1.5
	case best <= 0.1:
		return 1
	case best <= 0.25:
		return 0.6
	default:
		return 0.35
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

// songEnergy estimates energy from BPM and ReplayGain (loudly mastered songs
// have strongly negative gain values). Returns -1 if no data is available.
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

// energyFactor is deliberately moderate: the energy estimate is only a hint,
// so it must not outweigh the genre.
func energyFactor(target string, e float64) float64 {
	switch target {
	case catalog.EnergyHigh:
		return 0.6 + 0.8*e
	case catalog.EnergyLow:
		return 0.6 + 0.8*(1-e)
	case catalog.EnergyMedium:
		return 1.2 - 0.8*math.Abs(e-0.5)
	}
	return 1
}

// pick draws weighted samples without replacement (Efraimidis-Spirakis).
// The first pass limits songs per artist and gives every wanted genre an equal
// share; later passes drop the genre share and then the artist limit if there
// are not enough songs.
func (g *generator) pick(cands []candidate, count int) []candidate {
	for i := range cands {
		u := g.rng.Float64()
		for u == 0 {
			u = g.rng.Float64()
		}
		cands[i].key = -math.Log(u) / cands[i].weight
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].key < cands[j].key })

	groups := map[int]bool{}
	for _, c := range cands {
		if c.group >= 0 {
			groups[c.group] = true
		}
	}
	groupLimit := 0
	if len(groups) > 1 {
		groupLimit = (count + len(groups) - 1) / len(groups)
	}

	chosen := make([]candidate, 0, count)
	used := make([]bool, len(cands))
	perArtist := map[string]int{}
	perGroup := map[int]int{}
	fill := func(artistLimit, groupLimit int) {
		for i, c := range cands {
			if len(chosen) >= count {
				return
			}
			if used[i] {
				continue
			}
			a := artistKey(c.s)
			if artistLimit > 0 && perArtist[a] >= artistLimit {
				continue
			}
			if groupLimit > 0 && c.group >= 0 && perGroup[c.group] >= groupLimit {
				continue
			}
			used[i] = true
			perArtist[a]++
			perGroup[c.group]++
			chosen = append(chosen, c)
		}
	}
	fill(g.cfg.MaxPerArtist, groupLimit)
	fill(g.cfg.MaxPerArtist, 0)
	fill(0, 0)
	return chosen
}

// order arranges the songs. Rising and falling flows sort by estimated energy
// when enough songs have BPM or ReplayGain tags. Otherwise songs are chained so
// that neighbours are similar in genre, year and energy. Finally, songs by the
// same artist are spread apart.
func (g *generator) order(chosen []candidate, flow string) []candidate {
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
		g.chain(chosen)
	}
	spreadBy(len(chosen), func(i int) string { return artistKey(chosen[i].s) }, func(i, j int) {
		chosen[i], chosen[j] = chosen[j], chosen[i]
	})
	return chosen
}

// chain orders songs greedily so that each song is similar to the previous one.
// A small random term keeps the order different every day.
func (g *generator) chain(c []candidate) {
	if len(c) < 3 {
		g.rng.Shuffle(len(c), func(i, j int) { c[i], c[j] = c[j], c[i] })
		return
	}
	start := g.rng.Intn(len(c))
	c[0], c[start] = c[start], c[0]
	for i := 1; i < len(c)-1; i++ {
		best, bestDistance := i, math.Inf(1)
		for j := i; j < len(c); j++ {
			if d := transition(c[i-1], c[j]) + g.rng.Float64()*0.35; d < bestDistance {
				best, bestDistance = j, d
			}
		}
		c[i], c[best] = c[best], c[i]
	}
}

// transition measures how different two neighbouring songs are (0 … maxTransition).
func transition(a, b candidate) float64 {
	d := 1.0
sharedGenre:
	for _, x := range a.genres {
		for _, y := range b.genres {
			if x == y {
				d = 0
				break sharedGenre
			}
		}
	}
	if a.s.Year > 0 && b.s.Year > 0 {
		d += 0.6 * math.Min(math.Abs(float64(a.s.Year-b.s.Year))/15, 1)
	} else {
		d += 0.2
	}
	if a.energy >= 0 && b.energy >= 0 {
		d += 0.8 * math.Abs(a.energy-b.energy)
	} else {
		d += 0.25
	}
	return d
}

// measureQuality calculates the quality metrics of an ordered playlist.
func measureQuality(ordered []candidate, r catalog.Recipe, ctx songContext) playlistQuality {
	q := playlistQuality{ExactGenre: -1}
	n := len(ordered)
	if n == 0 {
		return q
	}
	artists := map[string]bool{}
	exact, repeats, tagged := 0, 0, 0
	for _, c := range ordered {
		artists[artistKey(c.s)] = true
		if len(r.Genres) > 0 && bestGenreMatch(songGenres(c.s), r.Genres) == exactGenre {
			exact++
		}
		if age, ok := ctx.Recent[c.s.ID]; ok && age == 0 {
			repeats++
		}
		if c.s.BPM > 0 || replayGain(c.s) != nil {
			tagged++
		}
	}
	if len(r.Genres) > 0 {
		q.ExactGenre = float64(exact) / float64(n)
	}
	q.UniqueArtists = float64(len(artists)) / float64(n)
	q.Repeats = float64(repeats) / float64(n)
	q.Tagged = float64(tagged) / float64(n)
	if n > 1 {
		sum := 0.0
		for i := 1; i < n; i++ {
			sum += transition(ordered[i-1], ordered[i])
		}
		q.Coherence = 1 - sum/float64(n-1)/maxTransition
	} else {
		q.Coherence = 1
	}
	return q
}

// spreadArtists avoids the same artist twice in a row.
func spreadArtists(songs []song) {
	spreadBy(len(songs), func(i int) string { return artistKey(songs[i]) }, func(i, j int) {
		songs[i], songs[j] = songs[j], songs[i]
	})
}

func spreadBy(n int, artistOf func(int) string, swap func(i, j int)) {
	const window = 8
	for i := 1; i < n; i++ {
		prev := artistOf(i - 1)
		if artistOf(i) != prev {
			continue
		}
		for j := i + 1; j < n && j <= i+window; j++ {
			if artistOf(j) != prev {
				swap(i, j)
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

func percent(v float64) int {
	return int(math.Round(v * 100))
}

// summary describes the selection for the log.
func (st selectionStats) summary(chosen int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d songs", chosen)
	if len(st.Genres) > 0 {
		names := make([]string, 0, 8)
		for i, g := range st.Genres {
			if i == 8 {
				names = append(names, fmt.Sprintf("+%d more", len(st.Genres)-8))
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
		fmt.Fprintf(&b, " · not in library: %s", strings.Join(st.Missing, ", "))
	}
	fmt.Fprintf(&b, " · candidates: %d", st.Candidates)
	if st.Added > 0 {
		fmt.Fprintf(&b, " (incl. %d added by user)", st.Added)
	}
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
		fmt.Fprintf(&b, " · filtered: %s", strings.Join(parts, ", "))
	}
	if st.Relaxed {
		b.WriteString(" (duration/intro filters relaxed)")
	}
	if chosen > 0 {
		fmt.Fprintf(&b, " · metadata: %d with BPM, %d with ReplayGain, %d favorites", st.WithBPM, st.WithGain, st.Favorites)
		q := st.Quality
		b.WriteString(" · quality: ")
		if q.ExactGenre >= 0 {
			fmt.Fprintf(&b, "%d%% exact genre, ", percent(q.ExactGenre))
		}
		fmt.Fprintf(&b, "%d%% different artists, %d%% repeats, coherence %d%%", percent(q.UniqueArtists), percent(q.Repeats), percent(q.Coherence))
	}
	return b.String()
}

// explain describes why a song was chosen.
func (d songDetail) explain() string {
	s := d.Song
	return fmt.Sprintf("%s – %s [%s] weight %.2f = genre %.2f × tempo/energy/mood %.2f × favorites/ratings %.2f × variety %.2f × edits %.2f",
		s.Artist, s.Title, strings.Join(songGenres(s), ", "), d.Weight,
		d.Parts.Genre, d.Parts.Tempo, d.Parts.Preference, d.Parts.Variety, d.Parts.Feedback)
}
