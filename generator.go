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
	// Markierung aus der Entwicklungsphase unter dem Namen NaviBeat; solche
	// Playlists werden weiterhin erkannt, ersetzt und aufgeräumt.
	oldMarkerPrefix = "#nb:"
	// Beim Start gilt eine unveränderte Playlist als aktuell, wenn sie jünger ist.
	freshFor = 23 * time.Hour
)

type generator struct {
	cat *catalog.Catalog
	cfg Settings
	rng *rand.Rand
	// startup: nur fehlende, geänderte oder veraltete Playlists erzeugen.
	startup bool

	allowedUsers map[string]string // lower(username) -> username; nil = unbekannt
	genreCache   map[string][]libraryGenre
}

// managedPlaylist ist eine von Cantilune erstellte Playlist.
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
	keep := map[string]bool{} // Situationen, deren Playlists nicht aufgeräumt werden
	failed := 0
	for _, j := range buildJobs(g.cat, g.cfg) {
		keep[j.ID] = true
		user, err := g.resolveUser(j.User)
		if err != nil {
			logf(pdk.LogError, "%s übersprungen: %v", j.Name, err)
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
	logf(pdk.LogInfo, "Starte Generierung (%s): %d aktive Situationen, %d vorhandene Cantilune-Playlists",
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
			logf(pdk.LogError, "%s: Songs konnten nicht geladen werden: %v", j.Name, err)
			failed++
			continue
		}
		if len(ids) == 0 {
			hint := ""
			if len(st.Similar) > 0 {
				hint = " – ähnliche Genres in deiner Bibliothek: " + strings.Join(st.Similar, ", ")
			}
			logf(pdk.LogWarn, "%s: keine passenden Songs (%s)%s. Die bisherige Playlist bleibt erhalten.", j.Name, st.summary(0), hint)
			failed++
			continue
		}

		id, err := createPlaylist(j.User, j.Name, ids)
		if err != nil {
			logf(pdk.LogError, "%s konnte nicht erstellt werden: %v", j.Name, err)
			failed++
			continue
		}
		if err := updatePlaylistMeta(j.User, id, g.cfg.PublicPlaylists, playlistComment(j)); err != nil {
			logf(pdk.LogWarn, "%s: Sichtbarkeit/Kommentar konnte nicht gesetzt werden: %v", j.Name, err)
		}
		// Neue Playlist steht – die vorherige Version sofort entfernen.
		for _, old := range existing {
			if old.ID != id {
				g.deleteManaged(old.playlist)
			}
		}
		g.saveHistory(j.ID, ids)
		created++
		logf(pdk.LogInfo, "%s für %s: %s", j.Name, j.User, st.summary(len(ids)))
	}

	removed := g.deleteOldPlaylists(managed, keep)

	logf(pdk.LogInfo, "Generierung abgeschlossen: %d erstellt, %d unverändert, %d entfernt, %d fehlgeschlagen",
		created, unchanged, removed, failed)
	if created == 0 && failed > 0 {
		return fmt.Errorf("keine Playlist erstellt, %d Fehler (Details im Log)", failed)
	}
	return nil
}

func (g *generator) modeLabel() string {
	if g.startup {
		return "nur fehlende/geänderte"
	}
	return "täglich"
}

// removeAll löscht alle Cantilune-Playlists und den Verlauf (Aufräum-Modus vor dem Deinstallieren).
func (g *generator) removeAll() error {
	g.loadAllowedUsers()
	var extra []string
	if u, err := g.resolveUser(g.cfg.DefaultUser); err == nil {
		extra = append(extra, u)
	}
	removed := g.deleteOldPlaylists(g.scanPlaylists(extra), nil)
	if _, err := kvDeletePrefix(historyPrefix); err != nil {
		logf(pdk.LogWarn, "Verlauf konnte nicht gelöscht werden: %v", err)
	}
	logf(pdk.LogInfo, "Aufräumen abgeschlossen: %d Cantilune-Playlists gelöscht. Cantilune ist pausiert und kann jetzt deinstalliert werden.", removed)
	return nil
}

// scanPlaylists sammelt Cantilune-Playlists aller freigegebenen und der angegebenen Benutzer.
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
			logf(pdk.LogWarn, "Playlists von %q konnten nicht gelesen werden: %v", user, err)
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

// deleteOldPlaylists entfernt Playlists deaktivierter oder gelöschter Situationen.
func (g *generator) deleteOldPlaylists(managed map[string][]managedPlaylist, keep map[string]bool) int {
	removed := 0
	for id, lists := range managed {
		if keep[id] {
			continue
		}
		for _, pl := range lists {
			if g.deleteManaged(pl.playlist) {
				logf(pdk.LogInfo, "%q entfernt (Situation deaktiviert oder gelöscht)", pl.Name)
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
			logf(pdk.LogWarn, "%q kann nicht gelöscht werden: Besitzer %q ist nicht für das Plugin freigegeben", pl.Name, owner)
			return false
		}
		owner = actual
	}
	if err := deletePlaylist(owner, pl.ID); err != nil {
		logf(pdk.LogWarn, "%q konnte nicht gelöscht werden: %v", pl.Name, err)
		return false
	}
	return true
}

// addPreviousSongs ergänzt den Verlauf um die Songs der aktuellen Playlist
// (wichtig, falls der KVStore noch leer ist).
func (g *generator) addPreviousSongs(recent map[string]int, existing []managedPlaylist) {
	for _, pl := range existing {
		ids, err := fetchPlaylistSongIDs(pl.Owner, pl.ID)
		if err != nil {
			logf(pdk.LogDebug, "Songs von %q nicht lesbar: %v", pl.Name, err)
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
	parts := []string{"Automatisch erstellt von Cantilune", j.Title}
	if j.PresetLabel != "" {
		parts = append(parts, j.PresetLabel)
	}
	parts = append(parts, j.Mode, markerPrefix+j.ID+":"+j.Fingerprint)
	return strings.Join(parts, " · ")
}

// parseMarker liest "#cl:<situation>:<fingerprint>" aus dem Playlist-Kommentar.
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
// Benutzer
// ---------------------------------------------------------------------------

func (g *generator) loadAllowedUsers() {
	users, err := listUsers()
	if err != nil {
		logf(pdk.LogWarn, "Freigegebene Benutzer konnten nicht gelesen werden: %v", err)
		return
	}
	g.allowedUsers = map[string]string{}
	for _, u := range users {
		g.allowedUsers[strings.ToLower(u.UserName)] = u.UserName
	}
}

// resolveUser bestimmt den Zielbenutzer: konfiguriert > erster Admin.
func (g *generator) resolveUser(user string) (string, error) {
	if user != "" {
		if g.allowedUsers == nil {
			return user, nil
		}
		if actual, ok := g.allowedUsers[strings.ToLower(user)]; ok {
			return actual, nil
		}
		return "", fmt.Errorf("Benutzer %q ist nicht für das Plugin freigegeben (Plugin-Einstellungen → Benutzerzugriff)", user)
	}
	admins, err := listAdmins()
	if err != nil {
		return "", fmt.Errorf("kein Zielbenutzer konfiguriert und Admins nicht abrufbar: %w", err)
	}
	if len(admins) == 0 {
		return "", errors.New("kein Zielbenutzer konfiguriert und kein Admin für das Plugin freigegeben")
	}
	return admins[0].UserName, nil
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
