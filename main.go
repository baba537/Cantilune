// Cantilune creates daily playlists for everyday situations (gym, driving,
// cooking, studying, …) in Navidrome.
//
// Layout:
//   - main.go       plugin entry points (OnInit, OnCallback)
//   - config.go     reading settings, building one job per situation
//   - generator.go  creating playlists, removing previous versions
//   - selection.go  song selection (filtering, weighting, ordering)
//   - genres.go     tolerant matching against library genres
//   - history.go    history of recent days in the key-value store
//   - subsonic.go   Subsonic API calls
//   - catalog/      situations and presets (presets.json), manifest generator
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

// Host calls as variables so tests can replace them.
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

// OnInit is called when the plugin is loaded and after every configuration change.
func (p *cantilunePlugin) OnInit() error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	cfg := loadSettings(pdk.GetConfig, cat)

	if cfg.RemoveAll {
		// Cleanup mode: no more daily runs, remove everything once.
		_ = host.SchedulerCancelSchedule(scheduleIDDaily)
		if _, err := host.SchedulerScheduleOneTime(startupDelaySeconds, payloadCleanup, scheduleIDStartup); err != nil {
			return fmt.Errorf("could not schedule cleanup: %w", err)
		}
		logf(pdk.LogInfo, "Cantilune paused: all Cantilune playlists will be deleted in %d seconds", startupDelaySeconds)
		return nil
	}

	cron, err := cronExpression(cfg)
	if err != nil {
		logf(pdk.LogError, "%v, using default '%s'", err, catalog.DefaultCron)
		cron = catalog.DefaultCron
	}
	if _, err := host.SchedulerScheduleRecurring(cron, payloadDaily, scheduleIDDaily); err != nil {
		// The job may still exist from a previous instance.
		_ = host.SchedulerCancelSchedule(scheduleIDDaily)
		if _, err2 := host.SchedulerScheduleRecurring(cron, payloadDaily, scheduleIDDaily); err2 != nil {
			return fmt.Errorf("could not register schedule '%s': %w", cron, errors.Join(err, err2))
		}
	}

	logf(pdk.LogInfo, "Cantilune ready: %d playlists enabled (%d situations available), schedule '%s'",
		len(buildJobs(cat, cfg)), len(cat.Situations)+len(cfg.Custom), cron)

	if cfg.RunOnStartup {
		if _, err := host.SchedulerScheduleOneTime(startupDelaySeconds, payloadStartup, scheduleIDStartup); err != nil {
			logf(pdk.LogWarn, "could not schedule immediate generation: %v", err)
		}
	}
	return nil
}

// OnCallback is triggered by the scheduler.
func (p *cantilunePlugin) OnCallback(req scheduler.SchedulerCallbackRequest) error {
	if req.Payload != payloadDaily && req.Payload != payloadStartup && req.Payload != payloadCleanup {
		logf(pdk.LogDebug, "ignoring unknown scheduler callback (id=%s, payload=%s)", req.ScheduleID, req.Payload)
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
		logf(pdk.LogError, "playlist generation failed: %v", err)
		return err
	}
	return nil
}

func main() {}

func logf(level pdk.LogLevel, format string, args ...any) {
	logFn(level, fmt.Sprintf(format, args...))
}
