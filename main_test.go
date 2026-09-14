package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

var testNow = time.Date(2026, 9, 14, 4, 0, 0, 0, time.UTC)

func init() {
	logFn = func(pdk.LogLevel, string) {}
	nowFn = func() time.Time { return testNow }
}

func mustCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mapConfig(m map[string]string) configSource {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

// onlyEnabled returns a configuration in which only the given situations are enabled.
func onlyEnabled(cat *catalog.Catalog, presets map[string]string) map[string]string {
	cfg := map[string]string{}
	for _, s := range cat.Situations {
		preset, on := presets[s.ID]
		b, _ := json.Marshal(map[string]any{"enabled": on, "preset": preset})
		cfg[s.ID] = string(b)
	}
	return cfg
}

func TestBuildPlaylistName(t *testing.T) {
	cases := []struct{ prefix, situation, variant, emoji, want string }{
		{"🎧", "Gym", "Hardstyle", "⚡", "🎧 Gym Hardstyle ⚡"},
		{"🎧", "Driving", "", "🚗", "🎧 Driving 🚗"},
		{"[CL]", " Studying ", "", "", "[CL] Studying"},
	}
	for _, c := range cases {
		if got := buildPlaylistName(c.prefix, c.situation, c.variant, c.emoji); got != c.want {
			t.Errorf("buildPlaylistName = %q, want %q", got, c.want)
		}
	}
}

func TestCronExpression(t *testing.T) {
	cases := []struct {
		time, cron, want string
		wantErr          bool
	}{
		{"04:00", "", "0 4 * * *", false},
		{"23:45", "", "45 23 * * *", false},
		{"04:00", " 0  */6 * * * ", "0 */6 * * *", false},
		{"25:00", "", "", true},
		{"04:00", "0 4 * *", "", true},
	}
	for _, c := range cases {
		got, err := cronExpression(Settings{GenerationTime: c.time, CronExpression: c.cron})
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("cronExpression(%q,%q) = %q, %v", c.time, c.cron, got, err)
		}
	}
}

func TestResolveGenresIsTolerant(t *testing.T) {
	library := []libraryGenre{
		{"Hardstyle", 3}, {"Euphoric Hardstyle", 200}, {"Hip Hop", 50}, {"Drum'n'Bass", 20},
		{"Rock", 100}, {"Pop", 10}, {"K-Pop", 3}, {"R&B", 7},
	}
	groups, missing := resolveGenres(library, []string{"hardstyle", "Hip-Hop", "Drum and Bass", "Rawstyle", "Pop", "RnB"})
	names := map[string]bool{}
	for _, group := range groups {
		for _, m := range group {
			names[m.Name] = true
		}
	}
	for _, want := range []string{"Hardstyle", "Euphoric Hardstyle", "Hip Hop", "Drum'n'Bass", "Pop", "R&B"} {
		if !names[want] {
			t.Errorf("%q was not matched: %v", want, groups)
		}
	}
	if names["K-Pop"] || names["Rock"] {
		t.Errorf("wrong match: %v", groups)
	}
	if len(missing) != 1 || missing[0] != "Rawstyle" {
		t.Errorf("missing = %v", missing)
	}
	if len(groups[0]) != 2 || groups[0][0].Name != "Hardstyle" || groups[0][0].Quality != exactGenre {
		t.Errorf("exact matches should come first: %v", groups[0])
	}
}

func TestMatchGenreUsesWholeWords(t *testing.T) {
	cases := []struct {
		name, wanted string
		want         genreQuality
	}{
		{"Hip Hop", "Hip-Hop", exactGenre},
		{"Drum & Bass", "Drum and Bass", exactGenre},
		{"Liquid Drum'n'Bass", "Drum and Bass", subGenre},
		{"Euphoric Hardstyle", "Hardstyle", subGenre},
		{"Deep House", "House", subGenre},
		{"Nu Metal", "Metal", subGenre},
		{"Dancehall", "Dance", noMatch},
		{"Reggaeton", "Reggae", noMatch},
		{"Hardcore Punk", "Hardcore", noMatch},
		{"Neoclassical Metal", "Classical", noMatch},
		{"Neoclassical", "Classical", noMatch},
		{"Folk Metal", "Folk", noMatch},
		{"Drone Metal", "Drone", noMatch},
		{"K-Pop", "Pop", noMatch},
		{"Gamelan", "Game", noMatch},
	}
	for _, c := range cases {
		if got := matchGenre(c.name, c.wanted); got != c.want {
			t.Errorf("matchGenre(%q, %q) = %d, want %d", c.name, c.wanted, got, c.want)
		}
	}
	if !containsGenre("Christmas Pop", "Christmas") || containsGenre("Christmastime", "Christmas") || !containsGenre("Spoken Word Poetry", "Spoken Word") {
		t.Error("containsGenre should match whole words anywhere")
	}
}

func TestGenreFactorPrefersPureMatches(t *testing.T) {
	wanted := []string{"Hardstyle"}
	pure := genreFactor([]string{"Hardstyle"}, wanted)
	mixed := genreFactor([]string{"Hardstyle", "Pop", "Schlager"}, wanted)
	sub := genreFactor([]string{"Euphoric Hardstyle"}, wanted)
	none := genreFactor([]string{"Pop"}, wanted)
	if !(pure > sub && sub > mixed && mixed > none) {
		t.Errorf("unexpected order: pure=%v sub=%v mixed=%v none=%v", pure, sub, mixed, none)
	}
}

// Wrong genres that only share a word stem must not end up in the playlist,
// and every wanted genre gets a fair share of the candidates.
func TestSelectionIgnoresSimilarlyNamedGenres(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Dance", 40, nil)
	f.addSongs("Dancehall", 200, nil)
	f.addSongs("Reggaeton", 200, nil)
	f.addSongs("Deep House", 30, nil)
	f.addSongs("Tech House", 30, nil)
	f.addSongs("Progressive House", 30, nil)
	f.addSongs("Afro House", 30, nil)
	f.addSongs("EDM", 40, nil)
	useFake(t, f)

	g := newGenerator(cat, Settings{MaxPerArtist: 0, Weights: defaultWeights()}, false)
	job := Job{Name: "t", User: "admin", Count: 60, Recipe: catalog.Recipe{Genres: []string{"Dance", "House", "EDM"}}}
	ids, st, err := g.selectSongs(job, songContext{})
	if err != nil || len(ids) != 60 {
		t.Fatalf("%d songs, %v", len(ids), err)
	}
	perGenre := map[string]int{}
	for _, id := range ids {
		prefix := id[:strings.LastIndex(id, "-")]
		perGenre[prefix]++
	}
	if perGenre["dancehall"] > 0 || perGenre["reggaeton"] > 0 {
		t.Errorf("similarly named genres were selected: %v", perGenre)
	}
	house := perGenre["deephouse"] + perGenre["techhouse"] + perGenre["progressivehouse"] + perGenre["afrohouse"]
	if perGenre["dance"] < 10 || perGenre["edm"] < 10 || house > 35 {
		t.Errorf("genres not balanced: %v (house total %d)", perGenre, house)
	}
	if len(st.Missing) != 0 {
		t.Errorf("unexpected missing genres: %v", st.Missing)
	}
}

func TestBPMFactorHalfAndDoubleTime(t *testing.T) {
	cases := []struct {
		bpm, min, max int
		want          float64
	}{
		{150, 140, 180, 1.5},
		{75, 140, 180, 1.5},  // half-time tag
		{170, 85, 100, 1.5},  // double-time tag
		{130, 140, 180, 1},   // slightly below
		{112, 140, 180, 0.6}, // clearly below
		{40, 140, 180, 0.35}, // far away, even doubled
		{0, 140, 180, 1},
		{120, 0, 0, 1},
	}
	for _, c := range cases {
		if got := bpmFactor(c.bpm, c.min, c.max); got != c.want {
			t.Errorf("bpmFactor(%d,%d,%d) = %v, want %v", c.bpm, c.min, c.max, got, c.want)
		}
	}
}

func TestParseMarker(t *testing.T) {
	cases := []struct {
		comment, id, fp string
		ok              bool
	}{
		{"Created by Cantilune · Gym · Hardstyle ⚡ · Balanced · #cl:gym:1a2b3c4d", "gym", "1a2b3c4d", true},
		{"#cl:custom-my-list:ff00aa11", "custom-my-list", "ff00aa11", true},
		{"Automatisch erstellt von NaviBeat · #nb:party:cafebabe", "party", "cafebabe", true},
		{"my playlist", "", "", false},
	}
	for _, c := range cases {
		id, fp, ok := parseMarker(c.comment)
		if id != c.id || fp != c.fp || ok != c.ok {
			t.Errorf("parseMarker(%q) = %q %q %v", c.comment, id, fp, ok)
		}
	}
}

func TestInterludes(t *testing.T) {
	cases := []struct {
		title    string
		duration int
		want     bool
	}{
		{"Intro", 60, true},
		{"Skit - Mom", 40, true},
		{"Intro (Extended Mix)", 300, false},
		{"Introspection", 90, false},
	}
	for _, c := range cases {
		if got := isInterlude(song{Title: c.title, Duration: c.duration}); got != c.want {
			t.Errorf("isInterlude(%q, %d) = %v", c.title, c.duration, got)
		}
	}
}

func TestSettingsAndJobs(t *testing.T) {
	cat := mustCatalog(t)

	jobs := buildJobs(cat, loadSettings(mapConfig(nil), cat))
	if len(jobs) != 10 {
		t.Fatalf("expected 10 default playlists, got %d", len(jobs))
	}
	if jobs[0].ID != "gym" || jobs[0].Name != "🎧 Gym 💪" || jobs[0].Mode != catalog.ModeBalanced || jobs[0].Count != 50 {
		t.Errorf("unexpected gym job: %+v", jobs[0])
	}

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡", "laufen": "Drum & Bass 🥁"})
	cfg[catalog.KeyCustomSituations] = `[{"name":"My List","emoji":"🎮","genres":["Synthwave"],"energy":"energetic","flow":"rising","mode":"Discover","trackCount":20}]`
	jobs = buildJobs(cat, loadSettings(mapConfig(cfg), cat))
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d: %+v", len(jobs), jobs)
	}
	if jobs[0].Name != "🎧 Gym Hardstyle ⚡" || jobs[0].Recipe.MinBPM != 140 {
		t.Errorf("gym preset not applied: %+v", jobs[0])
	}
	if jobs[1].Name != "🎧 Running Drum & Bass 🥁" {
		t.Errorf("running name: %q", jobs[1].Name)
	}
	c := jobs[2]
	if c.ID != "custom-my-list" || c.Name != "🎧 My List 🎮" || c.Recipe.Energy != catalog.EnergyHigh ||
		c.Recipe.Flow != catalog.FlowRising || c.Mode != catalog.ModeDiscover || c.Count != 20 {
		t.Errorf("wrong custom situation: %+v", c)
	}

	cfg[catalog.KeyShowPresetInName] = "false"
	jobs = buildJobs(cat, loadSettings(mapConfig(cfg), cat))
	if jobs[0].Name != "🎧 Gym ⚡" {
		t.Errorf("preset name should be hidden: %q", jobs[0].Name)
	}
}

// Settings saved by earlier German versions keep their meaning.
func TestLegacyGermanSettings(t *testing.T) {
	cat := mustCatalog(t)
	cfg := onlyEnabled(cat, map[string]string{"lernen": "Klassik 🎻"})
	cfg["lernen"] = `{"enabled":true,"preset":"Klassik 🎻","mode":"Lieblingssongs"}`
	cfg[catalog.KeyCustomSituations] = `[{"name":"Meine Liste","genres":["Synthwave"],"energy":"energiegeladen","flow":"ansteigend","mode":"Entdecken"}]`
	jobs := buildJobs(cat, loadSettings(mapConfig(cfg), cat))
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "🎧 Studying Classical 🎻" || jobs[0].Mode != catalog.ModeFavorites {
		t.Errorf("legacy preset or mode not migrated: %+v", jobs[0])
	}
	if c := jobs[1]; c.Recipe.Energy != catalog.EnergyHigh || c.Recipe.Flow != catalog.FlowRising || c.Mode != catalog.ModeDiscover {
		t.Errorf("legacy custom situation not migrated: %+v", c)
	}
}

// ---------------------------------------------------------------------------
// Simulated Navidrome server
// ---------------------------------------------------------------------------

type fakePlaylist struct {
	playlist
	public  bool
	songIDs []string
}

type fakeServer struct {
	songs     []song
	playlists []*fakePlaylist
	calls     []string
	nextID    int
	rng       *rand.Rand
	kv        map[string][]byte

	// genre index, rebuilt when songs change
	indexed int
	byGenre map[string][]int // lower(genre) -> song indices
	genres  []libraryGenre
	byID    map[string]int
}

func newFakeServer() *fakeServer {
	return &fakeServer{rng: rand.New(rand.NewSource(1))}
}

// index mimics Navidrome: every genre tag of a song is searchable, and the
// genre filter of getRandomSongs is case-insensitive (SQL LIKE without wildcards).
func (f *fakeServer) index() {
	if f.indexed == len(f.songs) && f.byGenre != nil {
		return
	}
	f.byGenre = map[string][]int{}
	f.byID = map[string]int{}
	names := map[string]string{}
	var order []string
	for i, s := range f.songs {
		f.byID[s.ID] = i
		for _, g := range songGenres(s) {
			key := strings.ToLower(g)
			if _, ok := names[key]; !ok {
				names[key] = g
				order = append(order, key)
			}
			f.byGenre[key] = append(f.byGenre[key], i)
		}
	}
	f.genres = f.genres[:0]
	for _, key := range order {
		f.genres = append(f.genres, libraryGenre{Name: names[key], SongCount: len(f.byGenre[key])})
	}
	f.indexed = len(f.songs)
}

// randomSongs returns up to size random songs from indices (partial Fisher-Yates).
func (f *fakeServer) randomSongs(indices []int, size, from, to int) []song {
	idx := make([]int, 0, len(indices))
	for _, i := range indices {
		if s := f.songs[i]; (from > 0 && s.Year < from) || (to > 0 && s.Year > to) {
			continue
		}
		idx = append(idx, i)
	}
	n := min(size, len(idx))
	out := make([]song, n)
	for k := 0; k < n; k++ {
		j := k + f.rng.Intn(len(idx)-k)
		idx[k], idx[j] = idx[j], idx[k]
		out[k] = f.songs[idx[k]]
	}
	return out
}

func (f *fakeServer) addSongs(genre string, n int, mut func(i int, s *song)) {
	for i := 0; i < n; i++ {
		s := song{
			ID:       fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(genre, " ", "")), i),
			Title:    fmt.Sprintf("%s Track %d", genre, i),
			Artist:   fmt.Sprintf("%s Artist %d", genre, i%20),
			Genre:    genre,
			Genres:   []itemName{{genre}},
			Year:     2015,
			Duration: 200,
		}
		if mut != nil {
			mut(i, &s)
		}
		f.songs = append(f.songs, s)
	}
}

func (f *fakeServer) byName(name string) []*fakePlaylist {
	var out []*fakePlaylist
	for _, p := range f.playlists {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}

func (f *fakeServer) callIndex(prefix string) int {
	for i, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

func (f *fakeServer) call(uri string) (string, error) {
	f.calls = append(f.calls, uri)
	endpoint, rawQuery, _ := strings.Cut(uri, "?")
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	user := q.Get("u")
	if user == "" {
		return "", fmt.Errorf("missing required parameter 'u' (username)")
	}
	resp := map[string]any{"status": "ok"}
	switch endpoint {
	case "getGenres":
		f.index()
		resp["genres"] = map[string]any{"genre": f.genres}
	case "getRandomSongs":
		f.index()
		size, _ := strconv.Atoi(q.Get("size"))
		from, _ := strconv.Atoi(q.Get("fromYear"))
		to, _ := strconv.Atoi(q.Get("toYear"))
		var indices []int
		if g := q.Get("genre"); g != "" {
			indices = f.byGenre[strings.ToLower(g)]
		} else {
			indices = make([]int, len(f.songs))
			for i := range indices {
				indices[i] = i
			}
		}
		resp["randomSongs"] = map[string]any{"song": f.randomSongs(indices, size, from, to)}
	case "getPlaylists":
		var list []playlist
		for _, p := range f.playlists {
			if p.public || strings.EqualFold(p.Owner, user) {
				list = append(list, p.playlist)
			}
		}
		resp["playlists"] = map[string]any{"playlist": list}
	case "getPlaylist":
		for _, p := range f.playlists {
			if p.ID == q.Get("id") {
				pl := p.playlist
				for _, id := range p.songIDs {
					pl.Entry = append(pl.Entry, song{ID: id})
				}
				resp["playlist"] = pl
			}
		}
	case "getSong":
		f.index()
		if i, ok := f.byID[q.Get("id")]; ok {
			resp["song"] = f.songs[i]
		} else {
			resp = map[string]any{"status": "failed", "error": map[string]any{"code": 70, "message": "Song not found"}}
		}
	case "createPlaylist":
		f.nextID++
		p := &fakePlaylist{
			playlist: playlist{ID: "new" + strconv.Itoa(f.nextID), Name: q.Get("name"), Owner: user, Created: nowFn()},
			songIDs:  q["songId"],
		}
		f.playlists = append(f.playlists, p)
		resp["playlist"] = p.playlist
	case "updatePlaylist":
		for _, p := range f.playlists {
			if p.ID == q.Get("playlistId") {
				p.public = q.Get("public") == "true"
				p.Comment = q.Get("comment")
				if name := q.Get("name"); name != "" {
					p.Name = name
				}
			}
		}
	case "deletePlaylist":
		for i, p := range f.playlists {
			if p.ID == q.Get("id") {
				if !strings.EqualFold(p.Owner, user) {
					return "", fmt.Errorf("not owner")
				}
				f.playlists = append(f.playlists[:i], f.playlists[i+1:]...)
				break
			}
		}
	default:
		return "", fmt.Errorf("unexpected endpoint %s", endpoint)
	}
	b, _ := json.Marshal(map[string]any{"subsonic-response": resp})
	return string(b), nil
}

func useFake(t *testing.T, f *fakeServer) {
	subsonicCall = f.call
	listUsers = func() ([]host.User, error) {
		return []host.User{{UserName: "admin", IsAdmin: true}, {UserName: "bob"}}, nil
	}
	listAdmins = func() ([]host.User, error) { return []host.User{{UserName: "admin", IsAdmin: true}}, nil }
	if f.kv == nil {
		f.kv = map[string][]byte{}
	}
	kvSetTTL = func(key string, value []byte, ttl int64) error {
		if ttl <= 0 {
			return fmt.Errorf("ttl must be > 0")
		}
		f.kv[key] = value
		return nil
	}
	kvList = func(prefix string) ([]string, error) {
		var keys []string
		for k := range f.kv {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		return keys, nil
	}
	kvGetMany = func(keys []string) (map[string][]byte, error) {
		out := map[string][]byte{}
		for _, k := range keys {
			if v, ok := f.kv[k]; ok {
				out[k] = v
			}
		}
		return out, nil
	}
	kvDeletePrefix = func(prefix string) (int64, error) {
		n := int64(0)
		for k := range f.kv {
			if strings.HasPrefix(k, prefix) {
				delete(f.kv, k)
				n++
			}
		}
		return n, nil
	}
	t.Cleanup(func() {
		subsonicCall = host.SubsonicAPICall
		listUsers = host.UsersGetUsers
		listAdmins = host.UsersGetAdmins
		kvSetTTL = host.KVStoreSetWithTTL
		kvList = host.KVStoreList
		kvGetMany = host.KVStoreGetMany
		kvDeletePrefix = host.KVStoreDeleteByPrefix
	})
}

func TestHistoryAvoidsRepeatsAcrossDays(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Rock", 200, func(i int, s *song) { s.Artist = fmt.Sprintf("Artist %d", i) })
	useFake(t, f)
	job := Job{ID: "test", Name: "t", User: "admin", Count: 30, Recipe: catalog.Recipe{Genres: []string{"Rock"}}}
	defer func() { nowFn = func() time.Time { return testNow } }()

	// Repeats over 14 days (30 of 200 songs each), summed over several runs.
	repeats := func(historyDays int) int {
		total := 0
		for run := 0; run < 5; run++ {
			for k := range f.kv {
				delete(f.kv, k)
			}
			start := testNow.Add(time.Duration(run*100) * day)
			seen := map[string]bool{}
			for d := 0; d < 14; d++ {
				nowFn = func() time.Time { return start.Add(time.Duration(d) * day) }
				g := newGenerator(cat, Settings{MaxPerArtist: 3, HistoryDays: historyDays, Weights: defaultWeights()}, false)
				ids, _, err := g.selectSongs(job, songContext{Recent: g.loadHistory(job.ID, job.User)})
				if err != nil || len(ids) != 30 {
					t.Fatalf("day %d: %d songs, %v", d, len(ids), err)
				}
				for _, id := range ids {
					if d > 0 && seen[id] {
						total++
					}
				}
				seen = map[string]bool{}
				for _, id := range ids {
					seen[id] = true // only count repeats compared to the previous day
				}
				g.saveHistory(job.ID, job.User, ids)
			}
		}
		return total
	}
	with, without := repeats(7), repeats(0)
	t.Logf("repeats compared to the previous day (5×14 days): %d with history, %d without", with, without)
	// Without history, about 4-5 songs repeat per day; with history clearly fewer.
	if with*2 > without {
		t.Errorf("history has too little effect: %d repeats with, %d without", with, without)
	}
	if len(f.kv) != 0 {
		t.Errorf("nothing may be stored with historyDays=0: %d entries", len(f.kv))
	}

	g := newGenerator(cat, Settings{HistoryDays: 0}, false)
	g.saveHistory("off", "admin", []string{"x"})
	if _, ok := f.kv[historyKey("off", "admin", testNow)]; ok {
		t.Error("history stored although historyDays=0")
	}
}

func TestRemoveAllDeletesOnlyCantilunePlaylists(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "a", Name: "🎧 Gym Hardstyle ⚡", Owner: "admin", Comment: "#cl:gym:1"}},
		{playlist: playlist{ID: "b", Name: "🎧 Party 🎉", Owner: "bob", Comment: "#cl:party:2"}},
		{playlist: playlist{ID: "c", Name: "🎧 Cooking 🍳", Owner: "admin", Comment: "#nb:kochen:3"}},
		{playlist: playlist{ID: "d", Name: "🎧 My own", Owner: "admin"}},
	}
	useFake(t, f)
	f.kv[historyKey("gym", "admin", testNow)] = []byte("x")

	cfg := onlyEnabled(cat, map[string]string{"gym": ""})
	cfg[catalog.KeyRemoveAll] = "true"
	settings := loadSettings(mapConfig(cfg), cat)
	if !settings.RemoveAll {
		t.Fatal("RemoveAll not read")
	}
	if err := newGenerator(cat, settings, false).removeAll(); err != nil {
		t.Fatal(err)
	}
	if len(f.playlists) != 1 || f.playlists[0].ID != "d" {
		t.Errorf("expected only the user's own playlist, got %+v", f.playlists)
	}
	if len(f.kv) != 0 {
		t.Errorf("history not deleted: %v", f.kv)
	}
}

func TestGeneratorRun(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	// Hardstyle with mixed BPM tags (including half time) and a few intros.
	f.addSongs("Euphoric Hardstyle", 90, func(i int, s *song) {
		s.BPM = []int{150, 75, 0}[i%3]
		if i < 5 {
			s.Title, s.Duration = "Intro", 45
		}
	})
	f.addSongs("Hip Hop", 80, nil)
	f.addSongs("Rock", 60, func(i int, s *song) { s.Year = 1985 + i%15 })
	f.addSongs("Audiobook", 40, func(i int, s *song) { s.Year = 1990 })
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "old-gym", Name: "🎧 Gym Hardstyle ⚡", Owner: "admin", Comment: "Cantilune · #cl:gym:deadbeef", Created: testNow.Add(-24 * time.Hour)},
			songIDs: []string{"euphorichardstyle-6", "euphorichardstyle-7"}},
		{playlist: playlist{ID: "old-party", Name: "🎧 Party 🎉", Owner: "admin", Comment: "#nb:party:cafebabe"}},
		{playlist: playlist{ID: "mine", Name: "🎧 My List", Owner: "admin"}},
		{playlist: playlist{ID: "bobs", Name: "Bob's List", Owner: "bob", Comment: "private"}},
	}
	useFake(t, f)

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡", "autoFahren": "80s & 90s 📼"})
	settings := loadSettings(mapConfig(cfg), cat)
	if err := newGenerator(cat, settings, false).run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	gym := f.byName("🎧 Gym Hardstyle ⚡")
	if len(gym) != 1 || gym[0].ID == "old-gym" {
		t.Fatalf("expected exactly one new gym playlist, got %+v", gym)
	}
	if len(gym[0].songIDs) != 50 || !gym[0].public {
		t.Errorf("gym playlist: %d songs, public=%v", len(gym[0].songIDs), gym[0].public)
	}
	if id, _, ok := parseMarker(gym[0].Comment); !ok || id != "gym" {
		t.Errorf("marker missing in comment: %q", gym[0].Comment)
	}
	perArtist := map[string]int{}
	for _, id := range gym[0].songIDs {
		if !strings.HasPrefix(id, "euphorichardstyle-") {
			t.Errorf("song %s is not hardstyle", id)
		}
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "euphorichardstyle-"))
		if n < 5 {
			t.Errorf("intro %s was selected", id)
		}
		perArtist[fmt.Sprint(n%20)]++
	}
	for a, n := range perArtist {
		if n > settings.MaxPerArtist {
			t.Errorf("artist %s appears %d times", a, n)
		}
	}
	if create, del := f.callIndex("createPlaylist?"+url.Values{"name": {"🎧 Gym Hardstyle ⚡"}}.Encode()), f.callIndex("deletePlaylist?id=old-gym"); create < 0 || del < create {
		t.Errorf("old gym playlist must be deleted after the new one is created (create=%d, delete=%d)", create, del)
	}

	driving := f.byName("🎧 Driving 80s & 90s 📼")
	if len(driving) != 1 {
		t.Fatalf("driving playlist missing: %+v", f.playlists)
	}
	for _, id := range driving[0].songIDs {
		if !strings.HasPrefix(id, "rock-") {
			t.Errorf("driving playlist contains %s", id)
		}
	}

	for _, gone := range []string{"old-gym", "old-party"} {
		for _, p := range f.playlists {
			if p.ID == gone {
				t.Errorf("playlist %s should have been deleted", gone)
			}
		}
	}
	if len(f.byName("🎧 My List")) != 1 || len(f.byName("Bob's List")) != 1 {
		t.Error("other playlists were modified")
	}

	// Startup run without changes: nothing is regenerated.
	f.calls = nil
	if err := newGenerator(cat, settings, true).run(); err != nil {
		t.Fatalf("startup run: %v", err)
	}
	if f.callIndex("createPlaylist") >= 0 || f.callIndex("deletePlaylist") >= 0 {
		t.Errorf("unchanged playlists were regenerated: %v", f.calls)
	}

	// Preset changed: only gym is regenerated and the hardstyle playlist disappears.
	cfg = onlyEnabled(cat, map[string]string{"gym": "HipHop 🎤", "autoFahren": "80s & 90s 📼"})
	f.calls = nil
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), true).run(); err != nil {
		t.Fatalf("startup run after change: %v", err)
	}
	if len(f.byName("🎧 Gym HipHop 🎤")) != 1 || len(f.byName("🎧 Gym Hardstyle ⚡")) != 0 {
		t.Errorf("preset change not applied: %v", f.playlists)
	}
	if strings.Count(strings.Join(f.calls, "\n"), "createPlaylist") != 1 {
		t.Errorf("expected exactly one new playlist: %v", f.calls)
	}
}

func TestNoMatchingGenreKeepsOldPlaylist(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Hardstyle Classics", 10, nil)
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "old", Name: "🎧 Studying Classical 🎻", Owner: "admin", Comment: "#cl:lernen:x"}},
	}
	useFake(t, f)
	cfg := onlyEnabled(cat, map[string]string{"lernen": "Classical 🎻"})
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err == nil {
		t.Error("expected an error because no playlist was created")
	}
	if len(f.byName("🎧 Studying Classical 🎻")) != 1 {
		t.Error("the previous playlist must not be deleted without a replacement")
	}
}

func TestFavoritesModePrefersStarredSongs(t *testing.T) {
	cat := mustCatalog(t)
	starredIn := func(mode string) int {
		f := newFakeServer()
		f.addSongs("Rock", 300, func(i int, s *song) {
			s.Artist = fmt.Sprintf("Artist %d", i)
			if i%10 == 0 {
				ts := testNow.Add(-100 * day)
				s.Starred = &ts
			}
		})
		useFake(t, f)
		g := newGenerator(cat, Settings{MaxPerArtist: 3, SkipInterludes: true, Weights: defaultWeights()}, false)
		ids, _, err := g.selectSongs(Job{Name: "t", User: "admin", Count: 30, Mode: mode, Recipe: catalog.Recipe{Genres: []string{"Rock"}}}, songContext{})
		if err != nil || len(ids) != 30 {
			t.Fatalf("%s: %d songs, %v", mode, len(ids), err)
		}
		n := 0
		for _, id := range ids {
			if i, _ := strconv.Atoi(strings.TrimPrefix(id, "rock-")); i%10 == 0 {
				n++
			}
		}
		return n
	}
	fav, balanced := starredIn(catalog.ModeFavorites), starredIn(catalog.ModeBalanced)
	if fav < 12 || fav <= balanced {
		t.Errorf("favorites mode: %d favorites, balanced: %d", fav, balanced)
	}
}

func TestSpreadArtists(t *testing.T) {
	songs := []song{{ID: "1", Artist: "A"}, {ID: "2", Artist: "A"}, {ID: "3", Artist: "B"}, {ID: "4", Artist: "A"}, {ID: "5", Artist: "C"}}
	spreadArtists(songs)
	for i := 1; i < len(songs); i++ {
		if songs[i].Artist == songs[i-1].Artist {
			t.Errorf("same artist twice in a row: %+v", songs)
		}
	}
}
