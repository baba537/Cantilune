package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"cantilune/catalog"
)

// Fuzz tests for code that handles data from the library, the settings and
// the Subsonic API. Run a single target with, for example:
//
//	go test -run '^$' -fuzz FuzzMatchGenre -fuzztime 30s

func FuzzMatchGenre(f *testing.F) {
	for _, seed := range []string{"Hip-Hop", "Drum'n'Bass", "R&B", "Children's Music", "Après-Ski", "", " ", "K-Pop", "100% Rock", "Rock_n_Roll", "日本のロック", "\x00\xff"} {
		f.Add(seed, "Rock")
	}
	f.Fuzz(func(t *testing.T, name, wanted string) {
		q := matchGenre(name, wanted)
		if q < noMatch || q > exactGenre {
			t.Fatalf("invalid quality %d", q)
		}
		if normalizeGenre(name) != "" && matchGenre(name, name) != exactGenre {
			t.Fatalf("%q does not match itself", name)
		}
		if q == exactGenre && normalizeGenre(name) != normalizeGenre(wanted) {
			t.Fatalf("exact match of %q and %q with different normal forms", name, wanted)
		}
		_ = containsGenre(name, wanted)
		_ = similarGenres([]libraryGenre{{Name: name, SongCount: 1}}, wanted, 3)
	})
}

func FuzzParseMarkers(f *testing.F) {
	for _, seed := range []string{
		"Created by Cantilune · Gym · Hardstyle ⚡ · Balanced · #cl:gym:1a2b3c4d",
		"Archived by Cantilune · #cla:gym:2026-09-13",
		"#nb:party:cafebabe", "#cl:", "#cla::", "#cl:a:b:c", "#cla:x:9999-99-99", "",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, comment string) {
		if id, _, ok := parseMarker(comment); ok && id == "" {
			t.Fatalf("empty situation ID accepted in %q", comment)
		}
		if id, date, ok := parseArchiveMarker(comment); ok && (id == "" || date.IsZero()) {
			t.Fatalf("invalid archive marker accepted in %q", comment)
		}
	})
}

func FuzzLoadSettings(f *testing.F) {
	f.Add("true", "50", `[{"name":"x","genres":["Rock"]}]`, `{"enabled":true,"preset":"Hardstyle ⚡"}`, "Personal")
	f.Add("", "-1", "null", "[]", "")
	f.Add("maybe", "1e309", `[{"name":`, `{"enabled":"yes"}`, "Deutsch")
	cat, err := catalog.Load()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, boolValue, intValue, customJSON, situationJSON, choice string) {
		cfg := map[string]string{
			catalog.KeyPublicPlaylists:  boolValue,
			catalog.KeyTrackCount:       intValue,
			catalog.KeyWeightGenre:      intValue,
			catalog.KeyArchiveDays:      intValue,
			catalog.KeyCustomSituations: customJSON,
			catalog.KeyExcludeGenres:    customJSON,
			catalog.KeyAudience:         choice,
			catalog.KeyLanguage:         choice,
			catalog.KeyGenerationTime:   choice,
			"gym":                       situationJSON,
		}
		s := loadSettings(mapConfig(cfg), cat)
		if s.TrackCount < 1 || s.TrackCount > maxTracksPerPlaylist {
			t.Fatalf("track count out of range: %d", s.TrackCount)
		}
		if s.Weights.Genre < 0 || s.Weights.Genre > 200 || s.ArchiveDays < 0 || s.ArchiveDays > 30 {
			t.Fatalf("value out of range: %+v", s)
		}
		for _, j := range buildJobs(cat, s) {
			if j.ID == "" || j.Count < 1 || j.Count > maxTracksPerPlaylist || (utf8.ValidString(customJSON) && !utf8.ValidString(j.Name)) {
				t.Fatalf("invalid job %+v", j)
			}
		}
		_, _ = cronExpression(s)
	})
}

func FuzzSubsonicResponse(f *testing.F) {
	f.Add(`{"subsonic-response":{"status":"ok","randomSongs":{"song":[{"id":"1","genres":[{"name":"Rock"}],"bpm":120,"replayGain":{"trackGain":-8}}]}}}`)
	f.Add(`{"subsonic-response":{"status":"failed","error":{"code":70,"message":"not found"}}}`)
	f.Add(`{"subsonic-response":{"status":"ok","playlists":{"playlist":[{"id":"p","name":"🎧","owner":"admin","created":"not a date"}]}}}`)
	f.Add(`not json`)
	f.Fuzz(func(t *testing.T, body string) {
		old := subsonicCall
		subsonicCall = func(string) (string, error) { return body, nil }
		defer func() { subsonicCall = old }()

		songs, err := fetchRandomSongs("admin", "Rock", 10, 0, 0)
		if err == nil {
			for _, s := range songs {
				_ = songEnergy(s)
				_ = songGenres(s)
				_ = isInterlude(s)
			}
		}
		_, _ = fetchOwnPlaylists("admin")
		_, _ = fetchGenres("admin")
		if _, err := fetchSong("admin", "1"); err != nil && !strings.Contains(err.Error(), "getSong") {
			t.Fatalf("error without endpoint context: %v", err)
		}
	})
}
