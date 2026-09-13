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

// onlyEnabled liefert eine Konfiguration, in der nur die genannten Situationen aktiv sind.
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
		{"🎧", "Auto fahren", "", "🚗", "🎧 Auto fahren 🚗"},
		{"[CL]", " Lernen ", "", "", "[CL] Lernen"},
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
	matched, missing := resolveGenres(library, []string{"hardstyle", "Hip-Hop", "Drum and Bass", "Rawstyle", "Pop", "RnB"})
	names := map[string]bool{}
	for _, m := range matched {
		names[m.Name] = true
	}
	for _, want := range []string{"Hardstyle", "Euphoric Hardstyle", "Hip Hop", "Drum'n'Bass", "Pop", "R&B"} {
		if !names[want] {
			t.Errorf("%q wurde nicht zugeordnet: %v", want, matched)
		}
	}
	if names["K-Pop"] || names["Rock"] {
		t.Errorf("falsche Zuordnung: %v", matched)
	}
	if len(missing) != 1 || missing[0] != "Rawstyle" {
		t.Errorf("missing = %v", missing)
	}
	if matched[0].Name != "Hardstyle" && matched[0].Name != "Hip Hop" && matched[0].Name != "Pop" {
		t.Errorf("exakte Treffer sollten zuerst kommen: %v", matched)
	}
}

func TestBPMFactorHalfAndDoubleTime(t *testing.T) {
	cases := []struct {
		bpm, min, max int
		want          float64
	}{
		{150, 140, 180, 1.6},
		{75, 140, 180, 1.6}, // Half-Time-Tag
		{170, 85, 100, 1.6}, // Double-Time-Tag
		{100, 140, 180, 0.3},
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
		{"Automatisch erstellt von Cantilune · Gym · Hardstyle ⚡ · Ausgewogen · #cl:gym:1a2b3c4d", "gym", "1a2b3c4d", true},
		{"#cl:custom-meine-liste:ff00aa11", "custom-meine-liste", "ff00aa11", true},
		{"Automatisch erstellt von NaviBeat · #nb:party:cafebabe", "party", "cafebabe", true},
		{"meine Playlist", "", "", false},
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
		t.Fatalf("erwartet 10 Standard-Playlists, got %d", len(jobs))
	}
	if jobs[0].ID != "gym" || jobs[0].Name != "🎧 Gym 💪" || jobs[0].Mode != catalog.ModeBalanced || jobs[0].Count != 50 {
		t.Errorf("unerwarteter Gym-Job: %+v", jobs[0])
	}

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡", "laufen": "Drum & Bass 🥁"})
	cfg[catalog.KeyCustomSituations] = `[{"name":"Meine Liste","emoji":"🎮","genres":["Synthwave"],"energy":"energiegeladen","flow":"ansteigend","mode":"Entdecken","trackCount":20}]`
	jobs = buildJobs(cat, loadSettings(mapConfig(cfg), cat))
	if len(jobs) != 3 {
		t.Fatalf("erwartet 3 Jobs, got %d: %+v", len(jobs), jobs)
	}
	if jobs[0].Name != "🎧 Gym Hardstyle ⚡" || jobs[0].Recipe.MinBPM != 140 {
		t.Errorf("Gym-Preset nicht übernommen: %+v", jobs[0])
	}
	if jobs[1].Name != "🎧 Laufen Drum & Bass 🥁" {
		t.Errorf("Laufen-Name: %q", jobs[1].Name)
	}
	c := jobs[2]
	if c.ID != "custom-meine-liste" || c.Name != "🎧 Meine Liste 🎮" || c.Recipe.Energy != catalog.EnergyHigh ||
		c.Recipe.Flow != catalog.FlowRising || c.Mode != catalog.ModeDiscover || c.Count != 20 {
		t.Errorf("eigene Situation falsch: %+v", c)
	}

	cfg[catalog.KeyShowPresetInName] = "false"
	jobs = buildJobs(cat, loadSettings(mapConfig(cfg), cat))
	if jobs[0].Name != "🎧 Gym ⚡" {
		t.Errorf("Preset-Name sollte ausgeblendet sein: %q", jobs[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Simulierter Navidrome-Server
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
}

func newFakeServer() *fakeServer {
	return &fakeServer{rng: rand.New(rand.NewSource(1))}
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
		counts := map[string]int{}
		for _, s := range f.songs {
			counts[s.Genre]++
		}
		var list []libraryGenre
		for g, n := range counts {
			list = append(list, libraryGenre{Name: g, SongCount: n})
		}
		resp["genres"] = map[string]any{"genre": list}
	case "getRandomSongs":
		size, _ := strconv.Atoi(q.Get("size"))
		from, _ := strconv.Atoi(q.Get("fromYear"))
		to, _ := strconv.Atoi(q.Get("toYear"))
		var pool []song
		for _, s := range f.songs {
			if g := q.Get("genre"); g != "" && s.Genre != g {
				continue
			}
			if (from > 0 && s.Year < from) || (to > 0 && s.Year > to) {
				continue
			}
			pool = append(pool, s)
		}
		f.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		if len(pool) > size {
			pool = pool[:size]
		}
		resp["randomSongs"] = map[string]any{"song": pool}
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
		return "", fmt.Errorf("unerwarteter Endpunkt %s", endpoint)
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
			return fmt.Errorf("ttl muss > 0 sein")
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

	// Wiederholungen über 14 Tage (je 30 aus 200 Songs), gemittelt über mehrere Durchläufe.
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
				g := newGenerator(cat, Settings{MaxPerArtist: 3, HistoryDays: historyDays}, false)
				ids, _, err := g.selectSongs(job, g.loadHistory(job.ID))
				if err != nil || len(ids) != 30 {
					t.Fatalf("Tag %d: %d Songs, %v", d, len(ids), err)
				}
				for _, id := range ids {
					if d > 0 && seen[id] {
						total++
					}
				}
				seen = map[string]bool{}
				for _, id := range ids {
					seen[id] = true // nur Wiederholungen zum Vortag zählen
				}
				g.saveHistory(job.ID, ids)
			}
		}
		return total
	}
	with, without := repeats(7), repeats(0)
	t.Logf("Wiederholungen zum Vortag (5×14 Tage): %d mit Verlauf, %d ohne", with, without)
	// Ohne Verlauf wiederholen sich im Schnitt ~4–5 Songs pro Tag, mit Verlauf deutlich weniger.
	if with*2 > without {
		t.Errorf("Verlauf wirkt zu schwach: %d Wiederholungen mit, %d ohne Verlauf", with, without)
	}
	if len(f.kv) != 0 {
		t.Errorf("mit historyDays=0 darf nichts gespeichert werden: %d Einträge", len(f.kv))
	}

	// Verlauf ausgeschaltet: nichts speichern.
	g := newGenerator(cat, Settings{HistoryDays: 0}, false)
	g.saveHistory("aus", []string{"x"})
	if _, ok := f.kv[historyKey("aus", testNow)]; ok {
		t.Error("Verlauf trotz historyDays=0 gespeichert")
	}
}

func TestRemoveAllDeletesOnlyCantilunePlaylists(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "a", Name: "🎧 Gym Hardstyle ⚡", Owner: "admin", Comment: "#cl:gym:1"}},
		{playlist: playlist{ID: "b", Name: "🎧 Party 🎉", Owner: "bob", Comment: "#cl:party:2"}},
		{playlist: playlist{ID: "c", Name: "🎧 Kochen 🍳", Owner: "admin", Comment: "#nb:kochen:3"}},
		{playlist: playlist{ID: "d", Name: "🎧 Meine eigene", Owner: "admin"}},
	}
	useFake(t, f)
	f.kv[historyKey("gym", testNow)] = []byte("x")

	cfg := onlyEnabled(cat, map[string]string{"gym": ""})
	cfg[catalog.KeyRemoveAll] = "true"
	settings := loadSettings(mapConfig(cfg), cat)
	if !settings.RemoveAll {
		t.Fatal("RemoveAll nicht gelesen")
	}
	if err := newGenerator(cat, settings, false).removeAll(); err != nil {
		t.Fatal(err)
	}
	if len(f.playlists) != 1 || f.playlists[0].ID != "d" {
		t.Errorf("erwartet nur die eigene Playlist, got %+v", f.playlists)
	}
	if len(f.kv) != 0 {
		t.Errorf("Verlauf nicht gelöscht: %v", f.kv)
	}
}

func TestGeneratorRun(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	// Hardstyle mit gemischten BPM-Tags (auch Half-Time) und einigen Intros.
	f.addSongs("Euphoric Hardstyle", 90, func(i int, s *song) {
		s.BPM = []int{150, 75, 0}[i%3]
		if i < 5 {
			s.Title, s.Duration = "Intro", 45
		}
	})
	f.addSongs("Hip Hop", 80, nil)
	f.addSongs("Rock", 60, func(i int, s *song) { s.Year = 1985 + i%15 })
	f.addSongs("Hörbuch", 40, func(i int, s *song) { s.Year = 1990 })
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "old-gym", Name: "🎧 Gym Hardstyle ⚡", Owner: "admin", Comment: "Cantilune · #cl:gym:deadbeef", Created: testNow.Add(-24 * time.Hour)},
			songIDs: []string{"euphorichardstyle-6", "euphorichardstyle-7"}},
		{playlist: playlist{ID: "old-party", Name: "🎧 Party 🎉", Owner: "admin", Comment: "#nb:party:cafebabe"}},
		{playlist: playlist{ID: "mine", Name: "🎧 Meine Liste", Owner: "admin"}},
		{playlist: playlist{ID: "bobs", Name: "Bobs Liste", Owner: "bob", Comment: "privat"}},
	}
	useFake(t, f)

	cfg := onlyEnabled(cat, map[string]string{"gym": "Hardstyle ⚡", "autoFahren": "80er & 90er 📼"})
	settings := loadSettings(mapConfig(cfg), cat)
	if err := newGenerator(cat, settings, false).run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	gym := f.byName("🎧 Gym Hardstyle ⚡")
	if len(gym) != 1 || gym[0].ID == "old-gym" {
		t.Fatalf("erwartet genau eine neue Gym-Playlist, got %+v", gym)
	}
	if len(gym[0].songIDs) != 50 || !gym[0].public {
		t.Errorf("Gym-Playlist: %d Songs, öffentlich=%v", len(gym[0].songIDs), gym[0].public)
	}
	if id, _, ok := parseMarker(gym[0].Comment); !ok || id != "gym" {
		t.Errorf("Marker fehlt im Kommentar: %q", gym[0].Comment)
	}
	perArtist := map[string]int{}
	for _, id := range gym[0].songIDs {
		if !strings.HasPrefix(id, "euphorichardstyle-") {
			t.Errorf("Song %s ist kein Hardstyle", id)
		}
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "euphorichardstyle-"))
		if n < 5 {
			t.Errorf("Intro %s wurde ausgewählt", id)
		}
		perArtist[fmt.Sprint(n%20)]++
	}
	for a, n := range perArtist {
		if n > settings.MaxPerArtist {
			t.Errorf("Künstler %s kommt %d-mal vor", a, n)
		}
	}
	if create, del := f.callIndex("createPlaylist?"+url.Values{"name": {"🎧 Gym Hardstyle ⚡"}}.Encode()), f.callIndex("deletePlaylist?id=old-gym"); create < 0 || del < create {
		t.Errorf("alte Gym-Playlist muss nach dem Erstellen der neuen gelöscht werden (create=%d, delete=%d)", create, del)
	}

	auto := f.byName("🎧 Auto fahren 80er & 90er 📼")
	if len(auto) != 1 {
		t.Fatalf("Auto-Playlist fehlt: %+v", f.playlists)
	}
	for _, id := range auto[0].songIDs {
		if strings.HasPrefix(id, "hörbuch") || !strings.HasPrefix(id, "rock-") {
			t.Errorf("Auto-Playlist enthält %s", id)
		}
	}

	for _, gone := range []string{"old-gym", "old-party"} {
		for _, p := range f.playlists {
			if p.ID == gone {
				t.Errorf("Playlist %s hätte gelöscht werden müssen", gone)
			}
		}
	}
	if len(f.byName("🎧 Meine Liste")) != 1 || len(f.byName("Bobs Liste")) != 1 {
		t.Error("fremde Playlists wurden verändert")
	}

	// Sofort-Lauf ohne Änderungen: nichts neu erzeugen.
	f.calls = nil
	if err := newGenerator(cat, settings, true).run(); err != nil {
		t.Fatalf("startup run: %v", err)
	}
	if f.callIndex("createPlaylist") >= 0 || f.callIndex("deletePlaylist") >= 0 {
		t.Errorf("unveränderte Playlists wurden neu erzeugt: %v", f.calls)
	}

	// Preset gewechselt: nur Gym wird neu erzeugt, die Hardstyle-Playlist verschwindet.
	cfg = onlyEnabled(cat, map[string]string{"gym": "HipHop 🎤", "autoFahren": "80er & 90er 📼"})
	f.calls = nil
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), true).run(); err != nil {
		t.Fatalf("startup run nach Änderung: %v", err)
	}
	if len(f.byName("🎧 Gym HipHop 🎤")) != 1 || len(f.byName("🎧 Gym Hardstyle ⚡")) != 0 {
		t.Errorf("Preset-Wechsel nicht umgesetzt: %v", f.playlists)
	}
	if strings.Count(strings.Join(f.calls, "\n"), "createPlaylist") != 1 {
		t.Errorf("erwartet genau eine neue Playlist: %v", f.calls)
	}
}

func TestNoMatchingGenreKeepsOldPlaylist(t *testing.T) {
	cat := mustCatalog(t)
	f := newFakeServer()
	f.addSongs("Hardstyle Classics", 10, nil)
	f.playlists = []*fakePlaylist{
		{playlist: playlist{ID: "old", Name: "🎧 Lernen Klassik 🎻", Owner: "admin", Comment: "#cl:lernen:x"}},
	}
	useFake(t, f)
	cfg := onlyEnabled(cat, map[string]string{"lernen": "Klassik 🎻"})
	if err := newGenerator(cat, loadSettings(mapConfig(cfg), cat), false).run(); err == nil {
		t.Error("erwartet Fehler, da keine Playlist erstellt wurde")
	}
	if len(f.byName("🎧 Lernen Klassik 🎻")) != 1 {
		t.Error("bisherige Playlist darf ohne Ersatz nicht gelöscht werden")
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
		g := newGenerator(cat, Settings{MaxPerArtist: 3, SkipInterludes: true}, false)
		ids, _, err := g.selectSongs(Job{Name: "t", User: "admin", Count: 30, Mode: mode, Recipe: catalog.Recipe{Genres: []string{"Rock"}}}, nil)
		if err != nil || len(ids) != 30 {
			t.Fatalf("%s: %d Songs, %v", mode, len(ids), err)
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
		t.Errorf("Lieblingssongs-Modus: %d Favoriten, Ausgewogen: %d", fav, balanced)
	}
}

func TestSpreadArtists(t *testing.T) {
	songs := []song{{ID: "1", Artist: "A"}, {ID: "2", Artist: "A"}, {ID: "3", Artist: "B"}, {ID: "4", Artist: "A"}, {ID: "5", Artist: "C"}}
	spreadArtists(songs)
	for i := 1; i < len(songs); i++ {
		if songs[i].Artist == songs[i-1].Artist {
			t.Errorf("gleicher Künstler hintereinander: %+v", songs)
		}
	}
}
