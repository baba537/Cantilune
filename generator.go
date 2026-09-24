package main

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"

	"cantilune/catalog"
)

const (
	markerPrefix  = "#cl:"
	archivePrefix = "#cla:"
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

// archivedPlaylist is a replaced playlist kept as private archive.
type archivedPlaylist struct {
	playlist
	situationID string
	date        time.Time
}

// playlistIndex holds the Cantilune playlists found on the server.
type playlistIndex struct {
	managed  map[string][]managedPlaylist // by situation ID
	archived []archivedPlaylist
}

func (idx playlistIndex) forOwner(situationID, owner string) []managedPlaylist {
	var out []managedPlaylist
	for _, pl := range idx.managed[situationID] {
		if equalFold(pl.Owner, owner) {
			out = append(out, pl)
		}
	}
	return out
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
	start := time.Now()
	g.loadAllowedUsers()

	base := buildJobs(g.cat, g.cfg)
	keepSituation := map[string]bool{} // situations whose playlists must not be removed at all
	jobs := base
	if g.cfg.Personal() {
		users := g.permittedUsers()
		if len(users) == 0 {
			return errors.New("personal playlists need at least one permitted user (plugin settings → user access)")
		}
		jobs = personalJobs(base, users)
	}

	var ready []Job
	keepOwner := map[string]bool{} // situation ID + owner of playlists that are still wanted
	failed := 0
	for _, j := range jobs {
		user, err := g.resolveUser(j.User)
		if err != nil {
			logf(pdk.LogError, "%s skipped: %v", j.Name, err)
			keepSituation[j.ID] = true
			failed++
			continue
		}
		j.User = user
		j.Fingerprint = fingerprint(j, g.cfg)
		keepOwner[ownerKey(j.ID, user)] = true
		ready = append(ready, j)
	}

	owners := make([]string, 0, len(ready))
	for _, j := range ready {
		owners = append(owners, j.User)
	}
	idx := g.scanPlaylists(owners)
	logf(pdk.LogInfo, "starting generation (%s): %d playlists, %d existing Cantilune playlists",
		g.modeLabel(), len(ready), countManaged(idx.managed))

	created, unchanged, previewed := 0, 0, 0
	for _, j := range ready {
		existing := idx.forOwner(j.ID, j.User)
		if g.startup && !g.cfg.DryRun && isUpToDate(existing, j) {
			unchanged++
			continue
		}

		current := g.playlistSongs(existing)
		if g.cfg.LearnFromEdits && !g.cfg.DryRun && len(existing) > 0 {
			if removed, added := g.learnFromEdits(j.ID, j.User, current); removed+added > 0 {
				logf(pdk.LogInfo, "%s for %s: learned from playlist edits (%d removed, %d added)", j.Name, j.User, removed, added)
			}
		}
		ctx := songContext{Recent: g.loadHistory(j.ID, j.User), Feedback: g.loadFeedback(j.ID, j.User)}
		for id := range current {
			ctx.Recent[id] = 0
		}

		ids, st, err := g.selectSongs(j, ctx)
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

		if g.cfg.DryRun {
			logf(pdk.LogInfo, "preview %s for %s (no changes made): %s", j.Name, j.User, st.summary(len(ids)))
			g.logDetails(j, st)
			previewed++
			continue
		}

		id, err := createPlaylist(j.User, j.Name, ids)
		if err != nil {
			logf(pdk.LogError, "could not create %s: %v", j.Name, err)
			failed++
			continue
		}
		if err := updatePlaylistMeta(j.User, id, j.Public, playlistComment(j, g.cfg.Language)); err != nil {
			logf(pdk.LogWarn, "%s: could not set visibility or comment: %v", j.Name, err)
		}
		// The new playlist exists, so retire the previous version right away.
		for _, old := range existing {
			if old.ID != id {
				g.retire(old)
			}
		}
		g.saveHistory(j.ID, j.User, ids)
		g.saveGenerated(j.ID, j.User, ids)
		created++
		logf(pdk.LogInfo, "%s for %s: %s", j.Name, j.User, st.summary(len(ids)))
		g.logDetails(j, st)
	}

	if g.cfg.DryRun {
		logf(pdk.LogInfo, "preview finished: %d playlists previewed, %d failed in %s. Disable \"Preview only\" to create them.", previewed, failed, elapsed(start))
		return nil
	}

	removed := g.deleteOldPlaylists(idx, keepSituation, keepOwner)
	expired := g.expireArchives(idx.archived)

	logf(pdk.LogInfo, "generation finished: %d created, %d unchanged, %d removed, %d archives expired, %d failed in %s",
		created, unchanged, removed, expired, failed, elapsed(start))
	if created == 0 && failed > 0 {
		return fmt.Errorf("no playlist created, %d errors (see log)", failed)
	}
	return nil
}

func (g *generator) modeLabel() string {
	switch {
	case g.cfg.DryRun:
		return "preview"
	case g.startup:
		return "missing or changed only"
	}
	return "daily"
}

func (g *generator) logDetails(j Job, st selectionStats) {
	if !g.cfg.LogDetails {
		return
	}
	for i, d := range st.Details {
		logf(pdk.LogInfo, "%s for %s #%d: %s", j.Name, j.User, i+1, d.explain())
	}
}

// removeAll deletes all Cantilune playlists, archives and stored data (cleanup before uninstalling).
func (g *generator) removeAll() error {
	g.loadAllowedUsers()
	var extra []string
	if u, err := g.resolveUser(g.cfg.DefaultUser); err == nil {
		extra = append(extra, u)
	}
	idx := g.scanPlaylists(extra)
	removed := g.deleteOldPlaylists(idx, nil, nil)
	for _, a := range idx.archived {
		if g.deleteManaged(a.playlist) {
			removed++
		}
	}
	for _, prefix := range storePrefixes {
		if _, err := kvDeletePrefix(prefix); err != nil {
			logf(pdk.LogWarn, "could not delete stored data %q: %v", prefix, err)
		}
	}
	logf(pdk.LogInfo, "cleanup finished: %d Cantilune playlists deleted. Cantilune is paused and can now be uninstalled.", removed)
	return nil
}

// permittedUsers returns the users the plugin may act for, sorted by name.
func (g *generator) permittedUsers() []string {
	users := make([]string, 0, len(g.allowedUsers))
	for _, u := range g.allowedUsers {
		users = append(users, u)
	}
	sort.Strings(users)
	return users
}

// scanPlaylists collects the Cantilune playlists of all permitted and the given users.
func (g *generator) scanPlaylists(extraUsers []string) playlistIndex {
	users := map[string]string{}
	for _, u := range g.allowedUsers {
		users[strings.ToLower(u)] = u
	}
	for _, u := range extraUsers {
		if u != "" {
			users[strings.ToLower(u)] = u
		}
	}

	idx := playlistIndex{managed: map[string][]managedPlaylist{}}
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
			if id, date, ok := parseArchiveMarker(pl.Comment); ok {
				idx.archived = append(idx.archived, archivedPlaylist{playlist: pl, situationID: id, date: date})
			} else if id, fp, ok := parseMarker(pl.Comment); ok {
				idx.managed[id] = append(idx.managed[id], managedPlaylist{playlist: pl, situationID: id, fingerprint: fp})
			}
		}
	}
	return idx
}

// deleteOldPlaylists removes playlists of disabled or deleted situations and of
// owners that no longer get the playlist (e.g. after switching the audience).
func (g *generator) deleteOldPlaylists(idx playlistIndex, keepSituation, keepOwner map[string]bool) int {
	removed := 0
	for id, lists := range idx.managed {
		if keepSituation[id] {
			continue
		}
		for _, pl := range lists {
			if keepOwner[ownerKey(id, pl.Owner)] {
				continue
			}
			if g.deleteManaged(pl.playlist) {
				logf(pdk.LogInfo, "removed %q of %s (situation disabled, deleted or no longer assigned)", pl.Name, pl.Owner)
				removed++
			}
		}
	}
	return removed
}

// retire archives or deletes a replaced playlist.
func (g *generator) retire(old managedPlaylist) {
	if g.cfg.ArchiveDays <= 0 {
		g.deleteManaged(old.playlist)
		return
	}
	date := old.Created.UTC()
	if date.IsZero() {
		date = nowFn().UTC()
	}
	name := fmt.Sprintf("%s (%s)", old.Name, date.Format(dateLayout))
	comment := "Archived by Cantilune · " + archivePrefix + old.situationID + ":" + date.Format(dateLayout)
	if err := archivePlaylist(g.ownerFor(old.Owner), old.ID, name, comment); err != nil {
		logf(pdk.LogWarn, "could not archive %q: %v", old.Name, err)
	}
}

// expireArchives deletes archived playlists older than the archive period.
func (g *generator) expireArchives(archived []archivedPlaylist) int {
	cutoff := truncateDay(nowFn()).AddDate(0, 0, -g.cfg.ArchiveDays)
	expired := 0
	for _, a := range archived {
		if g.cfg.ArchiveDays > 0 && !a.date.Before(cutoff) {
			continue
		}
		if g.deleteManaged(a.playlist) {
			expired++
		}
	}
	return expired
}

func (g *generator) ownerFor(owner string) string {
	if actual, ok := g.allowedUsers[strings.ToLower(owner)]; ok {
		return actual
	}
	return owner
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

// playlistSongs returns the songs currently in the given playlists.
func (g *generator) playlistSongs(existing []managedPlaylist) map[string]bool {
	songs := map[string]bool{}
	for _, pl := range existing {
		ids, err := fetchPlaylistSongIDs(g.ownerFor(pl.Owner), pl.ID)
		if err != nil {
			logf(pdk.LogDebug, "could not read songs of %q: %v", pl.Name, err)
			continue
		}
		for _, id := range ids {
			songs[id] = true
		}
	}
	return songs
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

func playlistComment(j Job, language string) string {
	intro := "Created by Cantilune"
	if language == catalog.LanguageGerman {
		intro = "Erstellt von Cantilune"
	}
	parts := []string{intro, j.Title}
	if j.PresetLabel != "" {
		parts = append(parts, j.PresetLabel)
	}
	parts = append(parts, catalog.ModeName(j.Mode, language), markerPrefix+j.ID+":"+j.Fingerprint)
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
	rest := markerValue(comment[i+len(prefix):])
	if j := strings.LastIndex(rest, ":"); j >= 0 {
		id, fp = rest[:j], rest[j+1:]
	} else {
		id = rest
	}
	return id, fp, id != ""
}

// parseArchiveMarker reads "#cla:<situation>:<YYYY-MM-DD>" from the playlist comment.
// elapsed formats the time since start for log lines, in milliseconds.
func elapsed(start time.Time) string {
	return fmt.Sprintf("%d ms", time.Since(start).Milliseconds())
}

func parseArchiveMarker(comment string) (id string, date time.Time, ok bool) {
	i := strings.LastIndex(comment, archivePrefix)
	if i < 0 {
		return "", time.Time{}, false
	}
	rest := markerValue(comment[i+len(archivePrefix):])
	j := strings.LastIndex(rest, ":")
	if j <= 0 {
		return "", time.Time{}, false
	}
	date, err := time.Parse(dateLayout, rest[j+1:])
	if err != nil {
		return "", time.Time{}, false
	}
	return rest[:j], date, true
}

func markerValue(rest string) string {
	if end := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }); end >= 0 {
		return rest[:end]
	}
	return rest
}

func ownerKey(situationID, owner string) string {
	return situationID + "|" + strings.ToLower(strings.TrimSpace(owner))
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
