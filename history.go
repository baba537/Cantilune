package main

import (
	"net/url"
	"strings"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
)

// Data kept in Navidrome's key-value store. Every key is scoped by situation
// and playlist owner, so personal playlists never mix data of different users.
//
//	hist:<situation>:<user>:<YYYY-MM-DD>  song IDs of the playlist generated that day
//	gen:<situation>:<user>                song IDs of the current generated playlist
//	fb:<situation>:<user>:<songID>        "-" removed or "+" added by the user
//
// All entries expire via TTL.
const (
	historyPrefix   = "hist:"
	generatedPrefix = "gen:"
	feedbackPrefix  = "fb:"
	dateLayout      = "2006-01-02"
	feedbackDays    = 60
	generatedDays   = 400
)

// storePrefixes lists all key prefixes Cantilune writes.
var storePrefixes = []string{historyPrefix, generatedPrefix, feedbackPrefix}

func userKey(user string) string {
	return url.PathEscape(strings.ToLower(strings.TrimSpace(user)))
}

func historyKey(situationID, user string, t time.Time) string {
	return historyPrefix + situationID + ":" + userKey(user) + ":" + t.UTC().Format(dateLayout)
}

func ttlDays(days int) int64 {
	return int64(days) * int64(day/time.Second)
}

// loadHistory returns song ID → age in days (0 = today, 1 = yesterday, …).
func (g *generator) loadHistory(situationID, user string) map[string]int {
	recent := map[string]int{}
	if g.cfg.HistoryDays <= 0 {
		return recent
	}
	values := g.loadPrefix(historyPrefix + situationID + ":" + userKey(user) + ":")
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
		for _, id := range splitIDs(value) {
			if old, ok := recent[id]; !ok || age < old {
				recent[id] = age
			}
		}
	}
	return recent
}

func (g *generator) saveHistory(situationID, user string, songIDs []string) {
	if g.cfg.HistoryDays <= 0 {
		return
	}
	if err := kvSetTTL(historyKey(situationID, user, nowFn()), joinIDs(songIDs), ttlDays(g.cfg.HistoryDays+1)); err != nil {
		logf(pdk.LogWarn, "could not save history for %q: %v", situationID, err)
	}
}

// saveGenerated remembers the songs of the playlist Cantilune just created,
// so edits by the user can be detected at the next run.
func (g *generator) saveGenerated(situationID, user string, songIDs []string) {
	if !g.cfg.LearnFromEdits {
		return
	}
	key := generatedPrefix + situationID + ":" + userKey(user)
	if err := kvSetTTL(key, joinIDs(songIDs), ttlDays(generatedDays)); err != nil {
		logf(pdk.LogWarn, "could not save generated songs for %q: %v", situationID, err)
	}
}

// learnFromEdits compares the songs Cantilune generated with the songs the
// playlist contains now. Removed songs are avoided and added songs preferred
// for feedbackDays.
func (g *generator) learnFromEdits(situationID, user string, current map[string]bool) (removed, added int) {
	key := generatedPrefix + situationID + ":" + userKey(user)
	values, err := kvGetMany([]string{key})
	if err != nil || len(values[key]) == 0 {
		return 0, 0
	}
	generated := map[string]bool{}
	for _, id := range splitIDs(values[key]) {
		generated[id] = true
	}
	prefix := feedbackPrefix + situationID + ":" + userKey(user) + ":"
	for id := range generated {
		if !current[id] {
			if err := kvSetTTL(prefix+id, []byte("-"), ttlDays(feedbackDays)); err == nil {
				removed++
			}
		}
	}
	for id := range current {
		if !generated[id] {
			if err := kvSetTTL(prefix+id, []byte("+"), ttlDays(feedbackDays)); err == nil {
				added++
			}
		}
	}
	return removed, added
}

// loadFeedback returns song ID → -1 (removed by the user) or +1 (added by the user).
func (g *generator) loadFeedback(situationID, user string) map[string]int {
	feedback := map[string]int{}
	if !g.cfg.LearnFromEdits {
		return feedback
	}
	prefix := feedbackPrefix + situationID + ":" + userKey(user) + ":"
	for key, value := range g.loadPrefix(prefix) {
		id := strings.TrimPrefix(key, prefix)
		switch string(value) {
		case "-":
			feedback[id] = -1
		case "+":
			feedback[id] = 1
		}
	}
	return feedback
}

func (g *generator) loadPrefix(prefix string) map[string][]byte {
	keys, err := kvList(prefix)
	if err != nil {
		logf(pdk.LogWarn, "could not read stored data %q: %v", prefix, err)
		return nil
	}
	if len(keys) == 0 {
		return nil
	}
	values, err := kvGetMany(keys)
	if err != nil {
		logf(pdk.LogWarn, "could not read stored data %q: %v", prefix, err)
		return nil
	}
	return values
}

// repeatFactor weights down recently used songs; the more recent, the stronger.
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

func joinIDs(ids []string) []byte {
	return []byte(strings.Join(ids, "\n"))
}

func splitIDs(value []byte) []string {
	var ids []string
	for _, id := range strings.Split(string(value), "\n") {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func truncateDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
