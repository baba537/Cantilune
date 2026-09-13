package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

const (
	markerPrefix = "#cl:"
	// Marker used during development under the name NaviBeat; such playlists
	// are still recognized, replaced and cleaned up.
	oldMarkerPrefix = "#nb:"
	// On startup, an unchanged playlist younger than this is considered current.
	freshFor = 23 * time.Hour
)

type generator struct {
	cat *catalog.Catalog
	cfg Settings
	rng *rand.Rand
	// startup: only create missing, changed or outdated playlists.
	startup bool

	allowedUsers map[string]string // lower(username) -> username; nil = unknown
	genreCache   map[string][]libraryGenre
}

// managedPlaylist is a playlist created by Cantilune.
type managedPlaylist struct {
	playlist
	situationID string
	fingerprint string
}

func newGenerator(cat *catalog.Catalog, cfg Settings, startup bool) *generator {
	return &generator{
		cat:        cat,
		cfg:        cfg,
		rng:        rand.New(rand.NewSource(nowFn().UnixNano())),
		startup:    startup,
		genreCache: map[string][]libraryGenre{},
	}
}

func (g *generator) run() error {
	g.loadAllowedUsers()

	var jobs []Job
	keep := map[string]bool{} // situations whose playlists are kept
	failed := 0
	for _, j := range buildJobs(g.cat, g.cfg) {
		keep[j.ID] = true
		user, err := g.resolveUser(j.User)
		if err != nil {
			logf(pdk.LogError, "%s skipped: %v", j.Name, err)
			failed++
			continue
		}
		j.User = user
		j.Fingerprint = fingerprint(j, g.cfg)
		jobs = append(jobs, j)
	}

	users := make([]string, 0, len(jobs))
	for _, j := range jobs {
		users = append(users, j.User)
	}
	managed := g.scanPlaylists(users)
	logf(pdk.LogInfo, "starting generation (%s): %d active situations, %d existing Cantilune playlists",
		g.modeLabel(), len(jobs), countManaged(managed))

	created, unchanged := 0, 0
	for _, j := range jobs {
		existing := managed[j.ID]
		if g.startup && isUpToDate(existing, j) {
			unchanged++
			continue
		}

		recent := g.loadHistory(j.ID)
		g.addPreviousSongs(recent, existing)
		ids, st, err := g.selectSongs(j, recent)
		if err != nil {
			logf(pdk.LogError, "%s: could not load songs: %v", j.Name, err)
			failed++
			continue
		}
		if len(ids) == 0 {
			hint := ""
			if len(st.Similar) > 0 {
				hint = "; similar genres in your library: " + strings.Join(st.Similar, ", ")
			}
			logf(pdk.LogWarn, "%s: no matching songs (%s)%s. The previous playlist is kept.", j.Name, st.summary(0), hint)
			failed++
			continue
		}

		id, err := createPlaylist(j.User, j.Name, ids)
		if err != nil {
			logf(pdk.LogError, "could not create %s: %v", j.Name, err)
			failed++
			continue
		}
		if err := updatePlaylistMeta(j.User, id, g.cfg.PublicPlaylists, playlistComment(j)); err != nil {
			logf(pdk.LogWarn, "%s: could not set visibility or comment: %v", j.Name, err)
		}
		// The new playlist exists, so remove the previous version right away.
		for _, old := range existing {
			if old.ID != id {
				g.deleteManaged(old.playlist)
			}
		}
		g.saveHistory(j.ID, ids)
		created++
		logf(pdk.LogInfo, "%s for %s: %s", j.Name, j.User, st.summary(len(ids)))
	}

	removed := g.deleteOldPlaylists(managed, keep)

	logf(pdk.LogInfo, "generation finished: %d created, %d unchanged, %d removed, %d failed",
		created, unchanged, removed, failed)
	if created == 0 && failed > 0 {
		return fmt.Errorf("no playlist created, %d errors (see log)", failed)
	}
	return nil
}

func (g *generator) modeLabel() string {
	if g.startup {
		return "missing or changed only"
	}
	return "daily"
}

// removeAll deletes all Cantilune playlists and the history (cleanup before uninstalling).
func (g *generator) removeAll() error {
	g.loadAllowedUsers()
	var extra []string
	if u, err := g.resolveUser(g.cfg.DefaultUser); err == nil {
		extra = append(extra, u)
	}
	removed := g.deleteOldPlaylists(g.scanPlaylists(extra), nil)
	if _, err := kvDeletePrefix(historyPrefix); err != nil {
		logf(pdk.LogWarn, "could not delete history: %v", err)
	}
	logf(pdk.LogInfo, "cleanup finished: %d Cantilune playlists deleted. Cantilune is paused and can now be uninstalled.", removed)
	return nil
}

// scanPlaylists collects the Cantilune playlists of all permitted and the given users.
func (g *generator) scanPlaylists(extraUsers []string) map[string][]managedPlaylist {
	users := map[string]string{}
	for _, u := range g.allowedUsers {
		users[strings.ToLower(u)] = u
	}
	for _, u := range extraUsers {
		if u != "" {
			users[strings.ToLower(u)] = u
		}
	}

	managed := map[string][]managedPlaylist{}
	seen := map[string]bool{}
	for _, user := range users {
		lists, err := fetchOwnPlaylists(user)
		if err != nil {
			logf(pdk.LogWarn, "could not read playlists of %q: %v", user, err)
			continue
		}
		for _, pl := range lists {
			if seen[pl.ID] {
				continue
			}
			seen[pl.ID] = true
			if id, fp, ok := parseMarker(pl.Comment); ok {
				managed[id] = append(managed[id], managedPlaylist{playlist: pl, situationID: id, fingerprint: fp})
			}
		}
	}
	return managed
}

// deleteOldPlaylists removes playlists of disabled or deleted situations.
func (g *generator) deleteOldPlaylists(managed map[string][]managedPlaylist, keep map[string]bool) int {
	removed := 0
	for id, lists := range managed {
		if keep[id] {
			continue
		}
		for _, pl := range lists {
			if g.deleteManaged(pl.playlist) {
				logf(pdk.LogInfo, "removed %q (situation disabled or deleted)", pl.Name)
				removed++
			}
		}
	}
	return removed
}

func (g *generator) deleteManaged(pl playlist) bool {
	owner := pl.Owner
	if g.allowedUsers != nil {
		actual, ok := g.allowedUsers[strings.ToLower(owner)]
		if !ok {
			logf(pdk.LogWarn, "cannot delete %q: owner %q is not permitted for the plugin", pl.Name, owner)
			return false
		}
		owner = actual
	}
	if err := deletePlaylist(owner, pl.ID); err != nil {
		logf(pdk.LogWarn, "could not delete %q: %v", pl.Name, err)
		return false
	}
	return true
}

// addPreviousSongs adds the songs of the current playlist to the history
// (relevant while the key-value store is still empty).
func (g *generator) addPreviousSongs(recent map[string]int, existing []managedPlaylist) {
	for _, pl := range existing {
		ids, err := fetchPlaylistSongIDs(pl.Owner, pl.ID)
		if err != nil {
			logf(pdk.LogDebug, "could not read songs of %q: %v", pl.Name, err)
			continue
		}
		for _, id := range ids {
			recent[id] = 0
		}
	}
}

func isUpToDate(existing []managedPlaylist, j Job) bool {
	if len(existing) != 1 {
		return false
	}
	pl := existing[0]
	return pl.fingerprint == j.Fingerprint &&
		pl.Name == j.Name &&
		equalFold(pl.Owner, j.User) &&
		nowFn().Sub(pl.Created) < freshFor
}

func playlistComment(j Job) string {
	parts := []string{"Created by Cantilune", j.Title}
	if j.PresetLabel != "" {
		parts = append(parts, j.PresetLabel)
	}
	parts = append(parts, j.Mode, markerPrefix+j.ID+":"+j.Fingerprint)
	return strings.Join(parts, " · ")
}

// parseMarker reads "#cl:<situation>:<fingerprint>" from the playlist comment.
func parseMarker(comment string) (id, fp string, ok bool) {
	prefix := markerPrefix
	i := strings.LastIndex(comment, prefix)
	if i < 0 {
		prefix = oldMarkerPrefix
		if i = strings.LastIndex(comment, prefix); i < 0 {
			return "", "", false
		}
	}
	rest := comment[i+len(prefix):]
	if end := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }); end >= 0 {
		rest = rest[:end]
	}
	if j := strings.LastIndex(rest, ":"); j >= 0 {
		id, fp = rest[:j], rest[j+1:]
	} else {
		id = rest
	}
	return id, fp, id != ""
}

func countManaged(m map[string][]managedPlaylist) int {
	n := 0
	for _, l := range m {
		n += len(l)
	}
	return n
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (g *generator) loadAllowedUsers() {
	users, err := listUsers()
	if err != nil {
		logf(pdk.LogWarn, "could not read permitted users: %v", err)
		return
	}
	g.allowedUsers = map[string]string{}
	for _, u := range users {
		g.allowedUsers[strings.ToLower(u.UserName)] = u.UserName
	}
}

// resolveUser determines the playlist owner: the configured user, otherwise the first admin.
func (g *generator) resolveUser(user string) (string, error) {
	if user != "" {
		if g.allowedUsers == nil {
			return user, nil
		}
		if actual, ok := g.allowedUsers[strings.ToLower(user)]; ok {
			return actual, nil
		}
		return "", fmt.Errorf("user %q is not permitted for the plugin (plugin settings → user access)", user)
	}
	admins, err := listAdmins()
	if err != nil {
		return "", fmt.Errorf("no owner configured and admins could not be read: %w", err)
	}
	if len(admins) == 0 {
		return "", errors.New("no owner configured and no admin permitted for the plugin")
	}
	return admins[0].UserName, nil
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
