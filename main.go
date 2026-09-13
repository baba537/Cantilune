// Cantilune – erstellt täglich automatisch Playlists für Alltagssituationen
// (Gym, Auto fahren, Kochen, Lernen, …) in Navidrome.
//
// Aufbau:
//   - main.go       Plugin-Einstiegspunkte (OnInit, OnCallback)
//   - config.go     Einstellungen lesen, Jobs pro Situation bilden
//   - generator.go  Playlists erstellen, alte Versionen entfernen
//   - selection.go  Song-Auswahl (Filter, Gewichtung, Reihenfolge)
//   - genres.go     toleranter Abgleich mit den Genres der Bibliothek
//   - history.go    Verlauf der letzten Tage im KVStore
//   - subsonic.go   Subsonic-API-Aufrufe
//   - catalog/      Situationen & Presets (presets.json) und Manifest-Generator
package main

//go:generate go run ./cmd/genmanifest

import (
	"errors"
	"fmt"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/lifecycle"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scheduler"

	"cantilune/catalog"
)

const (
	scheduleIDDaily     = "cantilune-daily"
	scheduleIDStartup   = "cantilune-startup"
	payloadDaily        = "daily"
	payloadStartup      = "startup"
	payloadCleanup      = "cleanup"
	startupDelaySeconds = 15
)

// Host-Aufrufe als Variablen, damit Tests sie ersetzen können.
var (
	subsonicCall = host.SubsonicAPICall
	listUsers    = host.UsersGetUsers
	listAdmins   = host.UsersGetAdmins
	logFn        = pdk.Log
	nowFn        = time.Now

	kvSetTTL       = host.KVStoreSetWithTTL
	kvList         = host.KVStoreList
	kvGetMany      = host.KVStoreGetMany
	kvDeletePrefix = host.KVStoreDeleteByPrefix
)

type cantilunePlugin struct{}

var (
	_ lifecycle.InitProvider     = (*cantilunePlugin)(nil)
	_ scheduler.CallbackProvider = (*cantilunePlugin)(nil)
)

func init() {
	p := &cantilunePlugin{}
	lifecycle.Register(p)
	scheduler.Register(p)
}

// OnInit wird beim Laden des Plugins und nach jeder Konfigurationsänderung aufgerufen.
func (p *cantilunePlugin) OnInit() error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	cfg := loadSettings(pdk.GetConfig, cat)

	if cfg.RemoveAll {
		// Aufräum-Modus: keine täglichen Läufe mehr, einmalig alles entfernen.
		_ = host.SchedulerCancelSchedule(scheduleIDDaily)
		if _, err := host.SchedulerScheduleOneTime(startupDelaySeconds, payloadCleanup, scheduleIDStartup); err != nil {
			return fmt.Errorf("Aufräumen konnte nicht geplant werden: %w", err)
		}
		logf(pdk.LogInfo, "Cantilune pausiert: Alle Cantilune-Playlists werden in %d Sekunden gelöscht", startupDelaySeconds)
		return nil
	}

	cron, err := cronExpression(cfg)
	if err != nil {
		logf(pdk.LogError, "%v – verwende Standard '%s'", err, catalog.DefaultCron)
		cron = catalog.DefaultCron
	}
	if _, err := host.SchedulerScheduleRecurring(cron, payloadDaily, scheduleIDDaily); err != nil {
		// Möglicherweise existiert der Job noch aus einer früheren Instanz.
		_ = host.SchedulerCancelSchedule(scheduleIDDaily)
		if _, err2 := host.SchedulerScheduleRecurring(cron, payloadDaily, scheduleIDDaily); err2 != nil {
			return fmt.Errorf("Zeitplan '%s' konnte nicht registriert werden: %w", cron, errors.Join(err, err2))
		}
	}

	logf(pdk.LogInfo, "Cantilune bereit: %d Playlists aktiv (%d Situationen verfügbar), Zeitplan '%s'",
		len(buildJobs(cat, cfg)), len(cat.Situations)+len(cfg.Custom), cron)

	if cfg.RunOnStartup {
		if _, err := host.SchedulerScheduleOneTime(startupDelaySeconds, payloadStartup, scheduleIDStartup); err != nil {
			logf(pdk.LogWarn, "Sofort-Generierung konnte nicht geplant werden: %v", err)
		}
	}
	return nil
}

// OnCallback wird vom Scheduler ausgelöst.
func (p *cantilunePlugin) OnCallback(req scheduler.SchedulerCallbackRequest) error {
	if req.Payload != payloadDaily && req.Payload != payloadStartup && req.Payload != payloadCleanup {
		logf(pdk.LogDebug, "Unbekannter Scheduler-Callback ignoriert (id=%s, payload=%s)", req.ScheduleID, req.Payload)
		return nil
	}
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	cfg := loadSettings(pdk.GetConfig, cat)
	g := newGenerator(cat, cfg, req.Payload == payloadStartup)
	if cfg.RemoveAll || req.Payload == payloadCleanup {
		return g.removeAll()
	}
	if err := g.run(); err != nil {
		logf(pdk.LogError, "Playlist-Generierung fehlgeschlagen: %v", err)
		return err
	}
	return nil
}

func main() {}

func logf(level pdk.LogLevel, format string, args ...any) {
	logFn(level, fmt.Sprintf(format, args...))
}
