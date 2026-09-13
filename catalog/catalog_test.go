package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCatalogLoads(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Situations) < 25 {
		t.Errorf("expected at least 25 situations, got %d", len(c.Situations))
	}
	for _, s := range c.Situations {
		for _, p := range s.Presets {
			r := s.Recipe(p)
			if len(r.Genres) == 0 && r.FromYear == 0 && r.ToYear == 0 {
				t.Errorf("%s/%s has neither genres nor years", s.ID, p.Name)
			}
			if r.MinBPM > 0 && r.MaxBPM > 0 && r.MinBPM > r.MaxBPM {
				t.Errorf("%s/%s: minBpm > maxBpm", s.ID, p.Name)
			}
			if r.Energy != "" && r.Energy != EnergyLow && r.Energy != EnergyMedium && r.Energy != EnergyHigh {
				t.Errorf("%s/%s: unknown energy %q", s.ID, p.Name, r.Energy)
			}
			if r.Flow != "" && r.Flow != FlowShuffle && r.Flow != FlowRising && r.Flow != FlowFalling {
				t.Errorf("%s/%s: unknown flow %q", s.ID, p.Name, r.Flow)
			}
		}
	}
}

func TestMixCombinesPresetGenres(t *testing.T) {
	c, _ := Load()
	gym, ok := c.Situation("gym")
	if !ok {
		t.Fatal("gym is missing")
	}
	mix, _ := gym.FindPreset("Mix 💪")
	if !mix.Mix {
		t.Fatalf("first gym preset should be Mix: %+v", mix)
	}
	r := gym.Recipe(mix)
	if len(r.Genres) < 10 || r.Energy != EnergyHigh {
		t.Errorf("incomplete mix recipe: %+v", r)
	}
	hs, ok := gym.FindPreset("Hardstyle ⚡")
	if !ok || gym.Recipe(hs).MinBPM != 140 {
		t.Errorf("hardstyle preset missing or wrong: %+v", hs)
	}
	if _, ok := gym.FindPreset("does not exist"); ok {
		t.Error("unknown preset must not be found")
	}
}

// Settings saved by earlier German versions must keep working.
func TestLegacyGermanLabels(t *testing.T) {
	c, _ := Load()
	cases := []struct{ situation, label, want string }{
		{"lernen", "Klassik 🎻", "Classical"},
		{"autoFahren", "80er & 90er 📼", "80s & 90s"},
		{"schlafen", "Naturklänge 🌧️", "Nature Sounds"},
		{"party", "Schlager & Partyhits 🍻", "Schlager & Party Hits"},
		{"fokus", "Elektronisch 🎧", "Electronic"},
	}
	for _, tc := range cases {
		s, ok := c.Situation(tc.situation)
		if !ok {
			t.Fatalf("situation %q is missing", tc.situation)
		}
		p, ok := s.FindPreset(tc.label)
		if !ok || p.Name != tc.want {
			t.Errorf("FindPreset(%q) = %q, %v; want %q", tc.label, p.Name, ok, tc.want)
		}
	}
	for de, en := range map[string]string{"Ausgewogen": ModeBalanced, "Entdecken": ModeDiscover, "energiegeladen": "energetic", "abklingend": "falling", "Balanced": "Balanced"} {
		if got := Canonical(de); got != en {
			t.Errorf("Canonical(%q) = %q, want %q", de, got, en)
		}
	}
}

// Fails if presets.json changed without running `go generate ./...`.
func TestManifestUpToDate(t *testing.T) {
	want, err := BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(want) {
		t.Fatal("generated manifest is not valid JSON")
	}
	got, err := os.ReadFile("../manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Fatal("manifest.json is outdated, run `go generate ./...`")
	}
}
