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
		t.Errorf("erwartet mindestens 25 Situationen, got %d", len(c.Situations))
	}
	for _, s := range c.Situations {
		for _, p := range s.Presets {
			r := s.Recipe(p)
			if len(r.Genres) == 0 && r.FromYear == 0 && r.ToYear == 0 {
				t.Errorf("%s/%s hat weder Genres noch Jahre", s.ID, p.Name)
			}
			if r.MinBPM > 0 && r.MaxBPM > 0 && r.MinBPM > r.MaxBPM {
				t.Errorf("%s/%s: minBpm > maxBpm", s.ID, p.Name)
			}
			if r.Energy != "" && r.Energy != EnergyLow && r.Energy != EnergyMedium && r.Energy != EnergyHigh {
				t.Errorf("%s/%s: unbekannte Energie %q", s.ID, p.Name, r.Energy)
			}
			if r.Flow != "" && r.Flow != FlowShuffle && r.Flow != FlowRising && r.Flow != FlowFalling {
				t.Errorf("%s/%s: unbekannter Verlauf %q", s.ID, p.Name, r.Flow)
			}
		}
	}
}

func TestMixCombinesPresetGenres(t *testing.T) {
	c, _ := Load()
	gym, ok := c.Situation("gym")
	if !ok {
		t.Fatal("gym fehlt")
	}
	mix, _ := gym.FindPreset("Mix 💪")
	if !mix.Mix {
		t.Fatalf("erstes Gym-Preset sollte Mix sein: %+v", mix)
	}
	r := gym.Recipe(mix)
	if len(r.Genres) < 10 || r.Energy != EnergyHigh {
		t.Errorf("Mix-Rezept unvollständig: %+v", r)
	}
	hs, ok := gym.FindPreset("Hardstyle ⚡")
	if !ok || gym.Recipe(hs).MinBPM != 140 {
		t.Errorf("Hardstyle-Preset nicht gefunden oder falsch: %+v", hs)
	}
	if _, ok := gym.FindPreset("gibt es nicht"); ok {
		t.Error("unbekanntes Preset darf nicht gefunden werden")
	}
}

// Schlägt fehl, wenn presets.json geändert, aber `go generate ./...` nicht ausgeführt wurde.
func TestManifestUpToDate(t *testing.T) {
	want, err := BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(want) {
		t.Fatal("erzeugtes Manifest ist kein gültiges JSON")
	}
	got, err := os.ReadFile("../manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Fatal("manifest.json ist veraltet – bitte `go generate ./...` ausführen")
	}
}
