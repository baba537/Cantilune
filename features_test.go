package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

// captureLogs records all log messages until the test ends.
func captureLogs(t *testing.T) *[]string {
	t.Helper()
	var lines []string
	logFn = func(_ pdk.LogLevel, msg string) { lines = append(lines, msg) }
	t.Cleanup(func() { logFn = func(pdk.LogLevel, string) {} })
	return &lines
}

func countCalls(f *fakeServer, prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// customOnly returns a configuration with all built-in situations disabled and
// the given custom situations as JSON.
func customOnly(cat *catalog.Catalog, customJSON string) map[string]string {
	cfg := onlyEnabled(cat, nil)
	cfg[catalog.KeyCustomSituations] = customJSON
	return cfg
}

func TestPersonalPlaylistsPerUser(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Hardstyle", 120, nil)
	useFake(t, f)

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡"})
	cfg[catalog.KeyAudience] = catalog.AudiencePersonal
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	lists := f.byName("🎧 Gym Hardstyle ⚡")
	owners := map[string]bool{}
	for _, p := range lists {
		owners[p.Owner] = true
		if p.public {
			t.Errorf("personal playlist of %s must be private", p.Owner)
		}
	}
	if len(lists) != 2 || !owners["admin"] || !owners["bob"] {
		t.Fatalf("expected one private playlist for admin and bob, got %+v", lists)
	}
	for _, u := range []string{"admin", "bob"} {
		if _, ok := f.kv[historyKey("gym", u, testNow)]; !ok {
			t.Errorf("history of %s is not stored separately", u)
		}
	}

	// Back to a shared playlist: bob's personal playlist is removed.
	cfg[catalog.KeyAudience] = catalog.AudienceShared
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	lists = f.byName("🎧 Gym Hardstyle ⚡")
	if len(lists) != 1 || lists[0].Owner != "admin" || !lists[0].public {
		t.Errorf("expected one public playlist of admin, got %+v", lists)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Hardstyle", 120, nil)
	useFake(t, f)
	logs := captureLogs(t)

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡"})
	cfg[catalog.KeyDryRun] = "true"
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"createPlaylist", "updatePlaylist", "deletePlaylist"} {
		if n := countCalls(f, endpoint); n > 0 {
			t.Errorf("preview called %s %d times", endpoint, n)
		}
	}
	if len(f.kv) != 0 {
		t.Errorf("preview stored data: %v", f.kv)
	}
	if !strings.Contains(strings.Join(*logs, "\n"), "preview 🎧 Gym Hardstyle ⚡ for admin (no changes made): 50 songs") {
		t.Errorf("preview result not logged:\n%s", strings.Join(*logs, "\n"))
	}
}

func TestArchiveKeepsReplacedPlaylists(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Hardstyle", 120, nil)
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "old", Name: "🎧 Gym Hardstyle ⚡", Owner: "admin", Comment: "#cl:gym:x", Created: testNow.Add(-24 * time.Hour)}, public: true},
		{playlist: playlist{ID: "archive-recent", Name: "🎧 Gym Hardstyle ⚡ (2026-09-12)", Owner: "admin", Comment: "Archived by Cantilune · #cla:gym:2026-09-12"}},
		{playlist: playlist{ID: "archive-expired", Name: "🎧 Gym Hardstyle ⚡ (2026-09-01)", Owner: "admin", Comment: "Archived by Cantilune · #cla:gym:2026-09-01"}},
	}
	useFake(t, f)

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡"})
	cfg[catalog.KeyArchiveDays] = "7"
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	current := f.byName("🎧 Gym Hardstyle ⚡")
	if len(current) != 1 || current[0].ID == "old" || !current[0].public {
		t.Fatalf("expected one new public playlist, got %+v", current)
	}
	archived := f.byName("🎧 Gym Hardstyle ⚡ (2026-09-13)")
	if len(archived) != 1 || archived[0].ID != "old" || archived[0].public || !strings.Contains(archived[0].Comment, "#cla:gym:2026-09-13") {
		t.Errorf("replaced playlist was not archived: %+v", archived)
	}
	if len(f.byName("🎧 Gym Hardstyle ⚡ (2026-09-12)")) != 1 {
		t.Error("archive within the archive period was deleted")
	}
	if len(f.byName("🎧 Gym Hardstyle ⚡ (2026-09-01)")) != 0 {
		t.Error("expired archive was not deleted")
	}
}

func TestLearnFromEdits(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Rock", 80, func(i int, s *song) { s.Artist = fmt.Sprintf("Artist %d", i) })
	f.addSongs("Jazz", 10, nil)
	useFake(t, f)

	cfg := customOnly(cat, `[{"name":"Rock Test","genres":["Rock"],"trackCount":20}]`)
	settings := loadSettings(mapConfig(cfg), cat)
	if err := newGenerator(cat, settings, false).run(); err != nil {
		t.Fatal(err)
	}
	lists := f.byName("🎧 Rock Test")
	if len(lists) != 1 || len(lists[0].songIDs) != 20 {
		t.Fatalf("first run: %+v", lists)
	}

	// The user removes five songs and adds two jazz songs.
	removed := append([]string{}, lists[0].songIDs[:5]...)
	lists[0].songIDs = append(lists[0].songIDs[5:], "jazz-1", "jazz-2")

	nowFn = func() time.Time { return testNow.Add(24 * time.Hour) }
	defer func() { nowFn = func() time.Time { return testNow } }()
	g := newGenerator(cat, settings, false)
	if err := g.run(); err != nil {
		t.Fatal(err)
	}

	feedback := g.loadFeedback("custom-rock-test", "admin")
	for _, id := range removed {
		if feedback[id] != -1 {
			t.Errorf("removal of %s not recorded", id)
		}
	}
	if feedback["jazz-1"] != 1 || feedback["jazz-2"] != 1 {
		t.Errorf("added songs not recorded: %v", feedback)
	}
	next := f.byName("🎧 Rock Test")
	if len(next) != 1 {
		t.Fatalf("second run: %+v", next)
	}
	for _, id := range next[0].songIDs {
		for _, r := range removed {
			if id == r {
				t.Errorf("removed song %s was selected again", id)
			}
		}
	}

	// Added songs are loaded even though they do not match the genre and are
	// not penalized as repeats.
	job := buildJobs(cat, settings)[0]
	job.User = "admin"
	ctx := songContext{Recent: map[string]int{"jazz-1": 0}, Feedback: feedback}
	_, st, err := g.selectSongs(job, ctx)
	if err != nil || st.Added != 2 || st.Rejected[reasonRemoved] == 0 && len(removed) > 0 && st.Candidates == 0 {
		t.Errorf("added songs not loaded: added=%d err=%v", st.Added, err)
	}
	parts := g.scoreParts(song{ID: "jazz-1", Genres: []itemName{{"Jazz"}}}, job, ctx, testNow)
	if parts.Feedback != 2.5 || parts.Variety != 1 {
		t.Errorf("added song weight parts: %+v", parts)
	}
}

func TestWeightsScaleFactorGroups(t *testing.T) {
	cat := mustCatalog(t)
	job := Job{Recipe: catalog.Recipe{Genres: []string{"Rock"}, MinBPM: 140, MaxBPM: 180}, Mode: catalog.ModeBalanced}
	s := song{ID: "x", Genres: []itemName{{"Rock"}, {"Pop"}, {"Schlager"}}, BPM: 60}

	neutral := newGenerator(cat, Settings{Weights: defaultWeights()}, false).scoreParts(s, job, songContext{}, testNow)
	ignored := newGenerator(cat, Settings{Weights: Weights{}}, false).scoreParts(s, job, songContext{}, testNow)
	double := newGenerator(cat, Settings{Weights: Weights{Genre: 200, Tempo: 200, Preference: 200, Variety: 200}}, false).scoreParts(s, job, songContext{}, testNow)

	if neutral.Genre >= 1 || neutral.Tempo >= 1 {
		t.Fatalf("test song should be weighted down: %+v", neutral)
	}
	if ignored.Genre != 1 || ignored.Tempo != 1 || ignored.Preference != 1 || ignored.Variety != 1 {
		t.Errorf("0 %% must ignore the factor groups: %+v", ignored)
	}
	if double.Genre >= neutral.Genre || double.Tempo >= neutral.Tempo {
		t.Errorf("200 %% must strengthen the factors: neutral %+v, double %+v", neutral, double)
	}
}

func TestLogDetailsExplainsEverySong(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Rock", 40, nil)
	useFake(t, f)
	logs := captureLogs(t)

	cfg := customOnly(cat, `[{"name":"Rock Test","genres":["Rock"],"trackCount":10}]`)
	cfg[catalog.KeyLogDetails] = "true"
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	explained := 0
	for _, l := range *logs {
		if strings.Contains(l, "🎧 Rock Test for admin #") && strings.Contains(l, "weight") && strings.Contains(l, "genre") {
			explained++
		}
	}
	if explained != 10 {
		t.Errorf("expected 10 explanations, got %d:\n%s", explained, strings.Join(*logs, "\n"))
	}
}

func TestGermanPlaylistNames(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Classical", 80, nil)
	useFake(t, f)

	cfg := onlyEnabled(cat, map[string]string{"lernen": "Classical 🎻"})
	cfg[catalog.KeyLanguage] = catalog.LanguageGerman
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	lists := f.byName("🎧 Lernen Klassik 🎻")
	if len(lists) != 1 {
		t.Fatalf("German playlist missing: %+v", f.playlists)
	}
	if !strings.HasPrefix(lists[0].Comment, "Erstellt von Cantilune · Lernen · Klassik 🎻 · Ausgewogen · #cl:lernen:") {
		t.Errorf("German comment: %q", lists[0].Comment)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEmptyLibrary(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	useFake(t, f)
	err := newGenerator(cat, loadSettings(mapConfig(nil), cat), false).run()
	if err == nil {
		t.Error("expected an error when no playlist can be created")
	}
	if countCalls(f, "createPlaylist") > 0 || len(f.playlists) > 0 {
		t.Errorf("playlists created from an empty library: %+v", f.playlists)
	}
}

func TestSongsWithoutTags(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	for i := 0; i < 30; i++ {
		f.songs = append(f.songs, song{ID: fmt.Sprintf("bare-%d", i), Title: fmt.Sprintf("Track %d", i)})
	}
	useFake(t, f)
	cfg := customOnly(cat, `[{"name":"Everything","trackCount":20,"energy":"energetic","flow":"rising","minBpm":120,"maxBpm":140}]`)
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	lists := f.byName("🎧 Everything")
	if len(lists) != 1 || len(lists[0].songIDs) != 20 {
		t.Errorf("songs without tags must still be usable: %+v", lists)
	}
}

func TestSpecialCharactersInUsersAndGenres(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("R&B", 30, nil)
	f.addSongs("Children's Music", 30, nil)
	f.addSongs("Après-Ski", 30, nil)
	useFake(t, f)
	listUsers = func() ([]host.User, error) {
		return []host.User{{UserName: "Anna & Bob"}, {UserName: "admin", IsAdmin: true}}, nil
	}

	cfg := customOnly(cat, `[
		{"name":"Soul & R&B","emoji":"💜","genres":["RnB"],"trackCount":10,"targetUser":"anna & bob"},
		{"name":"Kids 100% fun","genres":["Children's Music"],"trackCount":10},
		{"name":"Après-Ski #1","genres":["Après-Ski"],"trackCount":10}
	]`)
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err != nil {
		t.Fatal(err)
	}
	for name, owner := range map[string]string{"🎧 Soul & R&B 💜": "Anna & Bob", "🎧 Kids 100% fun": "admin", "🎧 Après-Ski #1": "admin"} {
		lists := f.byName(name)
		if len(lists) != 1 || lists[0].Owner != owner || len(lists[0].songIDs) != 10 {
			t.Errorf("%s: %+v", name, lists)
		}
	}
	for _, p := range f.playlists {
		if id, _, ok := parseMarker(p.Comment); !ok || id == "" {
			t.Errorf("marker of %q not readable: %q", p.Name, p.Comment)
		}
	}
}

func TestDuplicatesAcrossGenres(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Rock", 40, func(i int, s *song) { s.Genres = []itemName{{"Rock"}, {"Pop"}} })
	f.addSongs("rock", 10, func(i int, s *song) { s.ID = fmt.Sprintf("lowercase-rock-%d", i) }) // same genre with different case
	f.addSongs("Pop", 10, nil)
	useFake(t, f)

	g := newGenerator(cat, Settings{MaxPerArtist: 0, Weights: defaultWeights()}, false)
	job := Job{Name: "t", User: "admin", Count: 55, Recipe: catalog.Recipe{Genres: []string{"Rock", "Pop"}}}
	ids, _, err := g.selectSongs(job, songContext{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate song %s", id)
		}
		seen[id] = true
	}
	if len(ids) != 55 {
		t.Errorf("expected 55 unique songs, got %d", len(ids))
	}
}

func TestGenreMismatchIsFiltered(t *testing.T) {
	cat := mustCatalog(t)
	g := newGenerator(cat, Settings{Weights: defaultWeights()}, false)
	pool := []song{
		{ID: "a", Genres: []itemName{{"Rock"}}},
		{ID: "b", Genres: []itemName{{"Rock_n_Roll"}}}, // returned by SQL LIKE for "Rock_n_Roll"-style names
		{ID: "c", Genres: []itemName{{"Pop"}}},
		{ID: "d"},
		{ID: "e", Genres: []itemName{{"Jazz"}}},
	}
	rejected := map[string]int{}
	kept := g.applyFilters(pool, catalog.Recipe{Genres: []string{"Rock"}}, songContext{Feedback: map[string]int{"e": 1}}, true, rejected)
	ids := []string{}
	for _, s := range kept {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "a,e" || rejected[reasonGenreMismatch] != 3 {
		t.Errorf("kept %v, rejected %v", ids, rejected)
	}
}

func TestChainedOrderIsMoreCoherentThanRandom(t *testing.T) {
	cat := mustCatalog(t)
	var cands []candidate
	genres := []string{"Rock", "Jazz", "Techno", "Classical"}
	for i := 0; i < 48; i++ {
		gname := genres[i%4]
		s := song{ID: fmt.Sprintf("s%d", i), Artist: fmt.Sprintf("A%d", i), Genres: []itemName{{gname}}, Year: 1970 + (i%4)*15, BPM: 70 + (i%4)*25}
		cands = append(cands, candidate{s: s, energy: songEnergy(s), genres: []string{normalizeGenre(gname)}})
	}
	g := newGenerator(cat, Settings{Weights: defaultWeights()}, false)

	chained := append([]candidate{}, cands...)
	g.order(chained, catalog.FlowShuffle)
	coherent := measureQuality(chained, catalog.Recipe{}, songContext{}).Coherence

	random := 0.0
	for k := 0; k < 20; k++ {
		shuffled := append([]candidate{}, cands...)
		g.rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		random += measureQuality(shuffled, catalog.Recipe{}, songContext{}).Coherence
	}
	random /= 20
	t.Logf("coherence: chained %.2f, random %.2f", coherent, random)
	if coherent < random+0.15 {
		t.Errorf("chained order is not clearly more coherent: %.2f vs %.2f", coherent, random)
	}
}

func TestQualityMetrics(t *testing.T) {
	ordered := []candidate{
		{s: song{ID: "1", Artist: "A", Genres: []itemName{{"Rock"}}, BPM: 120}, energy: 0.7, genres: []string{"rock"}},
		{s: song{ID: "2", Artist: "A", Genres: []itemName{{"Punk Rock"}}}, energy: -1, genres: []string{"punkrock"}},
		{s: song{ID: "3", Artist: "B", Genres: []itemName{{"Rock"}}}, energy: -1, genres: []string{"rock"}},
		{s: song{ID: "4", Artist: "C", Genres: []itemName{{"Rock"}}}, energy: -1, genres: []string{"rock"}},
	}
	q := measureQuality(ordered, catalog.Recipe{Genres: []string{"Rock"}}, songContext{Recent: map[string]int{"3": 0, "4": 2}})
	if q.ExactGenre != 0.75 || q.UniqueArtists != 0.75 || q.Repeats != 0.25 || q.Tagged != 0.25 || q.Coherence <= 0 || q.Coherence > 1 {
		t.Errorf("unexpected metrics: %+v", q)
	}
	if empty := measureQuality(nil, catalog.Recipe{}, songContext{}); empty.ExactGenre != -1 {
		t.Errorf("empty playlist metrics: %+v", empty)
	}
}
