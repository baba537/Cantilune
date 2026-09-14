package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"

	"cantilune/catalog"
)

// Benchmarks and a load test with a large simulated library. They measure the
// plugin's own work (candidate handling, weighting, ordering) and count the
// Subsonic API calls. Query time inside Navidrome is not included because the
// server is simulated; see docs/performance.md.

// largeLibrary builds a library of n songs spread over the genres used by the
// presets plus additional genres, with realistic gaps in the tags.
func largeLibrary(n int) *fakeServer {
	cat, _ := catalog.Load()
	seen := map[string]bool{}
	var genres []string
	for _, s := range cat.Situations {
		for _, p := range s.Presets {
			for _, g := range p.Genres {
				if !seen[strings.ToLower(g)] {
					seen[strings.ToLower(g)] = true
					genres = append(genres, g)
				}
			}
		}
	}
	for i := 0; len(genres) < 300; i++ {
		genres = append(genres, fmt.Sprintf("Extra Genre %d", i))
	}

	rng := rand.New(rand.NewSource(42))
	f := newFakeServer()
	f.songs = make([]song, 0, n)
	for i := 0; i < n; i++ {
		s := song{
			ID:       fmt.Sprintf("song-%d", i),
			Title:    fmt.Sprintf("Track %d", i),
			Artist:   fmt.Sprintf("Artist %d", rng.Intn(n/20+1)),
			Duration: 90 + rng.Intn(400),
			Year:     1960 + rng.Intn(65),
		}
		s.ArtistID = s.Artist
		s.Genres = []itemName{{genres[rng.Intn(len(genres))]}}
		if rng.Intn(3) == 0 {
			s.Genres = append(s.Genres, itemName{genres[rng.Intn(len(genres))]})
		}
		if rng.Intn(3) == 0 {
			s.BPM = 60 + rng.Intn(120)
		}
		if rng.Intn(2) == 0 {
			gain := -12 + rng.Float64()*14
			s.ReplayGain.TrackGain = &gain
		}
		if rng.Intn(20) == 0 {
			t := testNow.Add(-time.Duration(rng.Intn(400)) * day)
			s.Starred = &t
		}
		s.PlayCount = int64(rng.Intn(30))
		f.songs = append(f.songs, s)
	}
	return f
}

func useLargeLibrary(tb testing.TB, f *fakeServer, users int) {
	tb.Helper()
	t, ok := tb.(*testing.T)
	if ok {
		useFake(t, f)
	} else {
		useFakeForBenchmark(tb, f)
	}
	list := make([]host.User, users)
	for i := range list {
		list[i] = host.User{UserName: fmt.Sprintf("user%d", i), IsAdmin: i == 0}
	}
	listUsers = func() ([]host.User, error) { return list, nil }
	listAdmins = func() ([]host.User, error) { return list[:1], nil }
}

// useFakeForBenchmark installs the simulated server for benchmarks.
func useFakeForBenchmark(tb testing.TB, f *fakeServer) {
	f.kv = map[string][]byte{}
	subsonicCall = func(uri string) (string, error) {
		out, err := f.call(uri)
		f.calls = f.calls[:0] // do not keep call logs in benchmarks
		return out, err
	}
	kvSetTTL = func(key string, value []byte, ttl int64) error { f.kv[key] = value; return nil }
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
	kvDeletePrefix = func(string) (int64, error) { return 0, nil }
	tb.Cleanup(func() {
		subsonicCall = host.SubsonicAPICall
		listUsers = host.UsersGetUsers
		listAdmins = host.UsersGetAdmins
		kvSetTTL = host.KVStoreSetWithTTL
		kvList = host.KVStoreList
		kvGetMany = host.KVStoreGetMany
		kvDeletePrefix = host.KVStoreDeleteByPrefix
	})
}

// BenchmarkSelectSongs measures one 50-song playlist (Gym Mix, 28 genres) from 100,000 songs.
func BenchmarkSelectSongs(b *testing.B) {
	cat := mustCatalogB(b)
	f := largeLibrary(100_000)
	useLargeLibrary(b, f, 1)
	settings := loadSettings(mapConfig(onlyEnabled(cat, map[string]string{"gym": "Mix 💪"})), cat)
	job := buildJobs(cat, settings)[0]
	job.User = "user0"
	g := newGenerator(cat, settings, false)
	g.libraryGenres(job.User) // genre list is cached per run

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ids, _, err := g.selectSongs(job, songContext{}); err != nil || len(ids) != 50 {
			b.Fatalf("%d songs, %v", len(ids), err)
		}
	}
}

// BenchmarkDailyRunPersonal measures a complete daily run: 10 default
// situations for 10 users (100 playlists) with 100,000 songs.
func BenchmarkDailyRunPersonal(b *testing.B) {
	cat := mustCatalogB(b)
	f := largeLibrary(100_000)
	useLargeLibrary(b, f, 10)
	cfg := map[string]string{catalog.KeyAudience: catalog.AudiencePersonal}
	settings := loadSettings(mapConfig(cfg), cat)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.playlists = nil
		if err := newGenerator(cat, settings, false).run(); err != nil {
			b.Fatal(err)
		}
	}
}

func mustCatalogB(b *testing.B) *catalog.Catalog {
	b.Helper()
	c, err := catalog.Load()
	if err != nil {
		b.Fatal(err)
	}
	return c
}

// TestLoadProfile runs the daily run with 100,000 songs and 10 users and
// prints duration, API calls and memory. It is skipped unless CANTILUNE_LOAD=1
// is set, because it takes several seconds.
func TestLoadProfile(t *testing.T) {
	if os.Getenv("CANTILUNE_LOAD") == "" {
		t.Skip("set CANTILUNE_LOAD=1 to run the load profile")
	}
	cat := mustCatalog(t)
	f := largeLibrary(100_000)
	useLargeLibrary(t, f, 10)
	settings := loadSettings(mapConfig(map[string]string{catalog.KeyAudience: catalog.AudiencePersonal}), cat)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	if err := newGenerator(cat, settings, false).run(); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	calls := map[string]int{}
	for _, c := range f.calls {
		endpoint, _, _ := strings.Cut(c, "?")
		calls[endpoint]++
	}
	playlists := len(f.playlists)
	t.Logf("library: %d songs, 10 users, %d playlists created", len(f.songs), playlists)
	t.Logf("duration: %v total, %v per playlist", elapsed.Round(time.Millisecond), (elapsed / time.Duration(max(playlists, 1))).Round(time.Microsecond))
	t.Logf("API calls: %d total (%v)", len(f.calls), calls)
	t.Logf("memory: %.1f MB allocated during the run, %.1f MB heap in use afterwards (simulated server included)",
		float64(after.TotalAlloc-before.TotalAlloc)/1e6, float64(after.HeapInuse)/1e6)
	if playlists != 100 {
		t.Errorf("expected 100 playlists, got %d", playlists)
	}
}
