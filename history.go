package main

import (
	"strings"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
)

// Der Verlauf merkt sich pro Situation und Tag, welche Songs in der Playlist
// waren (KVStore-Schlüssel "hist:<situation>:<YYYY-MM-DD>", läuft per TTL ab).
// So wiederholen sich Playlists auch über mehrere Tage hinweg kaum.
const (
	historyPrefix = "hist:"
	dateLayout    = "2006-01-02"
)

func historyKey(situationID string, t time.Time) string {
	return historyPrefix + situationID + ":" + t.UTC().Format(dateLayout)
}

// loadHistory liefert Song-ID → Alter in Tagen (0 = heute, 1 = gestern, …).
func (g *generator) loadHistory(situationID string) map[string]int {
	recent := map[string]int{}
	if g.cfg.HistoryDays <= 0 {
		return recent
	}
	keys, err := kvList(historyPrefix + situationID + ":")
	if err != nil {
		logf(pdk.LogWarn, "Verlauf für %q nicht lesbar: %v", situationID, err)
		return recent
	}
	if len(keys) == 0 {
		return recent
	}
	values, err := kvGetMany(keys)
	if err != nil {
		logf(pdk.LogWarn, "Verlauf für %q nicht lesbar: %v", situationID, err)
		return recent
	}
	today := truncateDay(nowFn())
	for key, value := range values {
		d, err := time.Parse(dateLayout, key[strings.LastIndex(key, ":")+1:])
		if err != nil {
			continue
		}
		age := int(today.Sub(d) / day)
		if age < 0 || age > g.cfg.HistoryDays {
			continue
		}
		for _, id := range strings.Split(string(value), "\n") {
			if old, ok := recent[id]; id != "" && (!ok || age < old) {
				recent[id] = age
			}
		}
	}
	return recent
}

func (g *generator) saveHistory(situationID string, songIDs []string) {
	if g.cfg.HistoryDays <= 0 {
		return
	}
	ttl := int64(g.cfg.HistoryDays+1) * int64(day/time.Second)
	if err := kvSetTTL(historyKey(situationID, nowFn()), []byte(strings.Join(songIDs, "\n")), ttl); err != nil {
		logf(pdk.LogWarn, "Verlauf für %q konnte nicht gespeichert werden: %v", situationID, err)
	}
}

// repeatFactor wertet kürzlich verwendete Songs ab – je frischer, desto stärker.
func repeatFactor(ageDays int) float64 {
	switch {
	case ageDays <= 1:
		return 0.15
	case ageDays <= 3:
		return 0.3
	default:
		return 0.5
	}
}

func truncateDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
