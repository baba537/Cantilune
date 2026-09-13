package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// song holds the metadata Navidrome provides through the (Open)Subsonic API:
// file tags (genre, year, BPM, mood, ReplayGain, explicit) and usage data
// recorded by Navidrome itself (play count, favorites, rating).
type song struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Artist         string     `json:"artist"`
	ArtistID       string     `json:"artistId"`
	AlbumID        string     `json:"albumId"`
	Genre          string     `json:"genre"`
	Genres         []itemName `json:"genres"`
	Year           int        `json:"year"`
	Duration       int        `json:"duration"`
	BPM            int        `json:"bpm"`
	Moods          []string   `json:"moods"`
	ExplicitStatus string     `json:"explicitStatus"`
	PlayCount      int64      `json:"playCount"`
	Played         *time.Time `json:"played"`
	Created        *time.Time `json:"created"`
	Starred        *time.Time `json:"starred"`
	UserRating     int        `json:"userRating"`
	ReplayGain     struct {
		TrackGain *float64 `json:"trackGain"`
		AlbumGain *float64 `json:"albumGain"`
	} `json:"replayGain"`
}

type itemName struct {
	Name string `json:"name"`
}

type playlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Comment   string    `json:"comment"`
	Owner     string    `json:"owner"`
	SongCount int       `json:"songCount"`
	Created   time.Time `json:"created"`
	Entry     []song    `json:"entry"`
}

type libraryGenre struct {
	Name      string `json:"value"`
	SongCount int    `json:"songCount"`
}

type subsonicResponse struct {
	Status string `json:"status"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RandomSongs *struct {
		Song []song `json:"song"`
	} `json:"randomSongs"`
	Playlist  *playlist `json:"playlist"`
	Playlists *struct {
		Playlist []playlist `json:"playlist"`
	} `json:"playlists"`
	Genres *struct {
		Genre []libraryGenre `json:"genre"`
	} `json:"genres"`
}

// callAPI performs a Subsonic call on behalf of user and checks the status.
func callAPI(user, endpoint string, q url.Values) (*subsonicResponse, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("u", user)
	raw, err := subsonicCall(endpoint + "?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	var env struct {
		Response subsonicResponse `json:"subsonic-response"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, fmt.Errorf("%s: invalid response: %w", endpoint, err)
	}
	if env.Response.Status != "ok" {
		msg := "unknown error"
		if e := env.Response.Error; e != nil {
			msg = fmt.Sprintf("%s (code %d)", e.Message, e.Code)
		}
		return nil, fmt.Errorf("%s: %s", endpoint, msg)
	}
	return &env.Response, nil
}

func fetchGenres(user string) ([]libraryGenre, error) {
	resp, err := callAPI(user, "getGenres", nil)
	if err != nil {
		return nil, err
	}
	if resp.Genres == nil {
		return nil, nil
	}
	return resp.Genres.Genre, nil
}

// fetchRandomSongs requests up to size random songs, optionally filtered by genre and years.
func fetchRandomSongs(user, genre string, size, fromYear, toYear int) ([]song, error) {
	q := url.Values{}
	q.Set("size", strconv.Itoa(clamp(size, 1, maxRandomSongsPerCall)))
	if genre != "" {
		q.Set("genre", genre)
	}
	if fromYear > 0 {
		q.Set("fromYear", strconv.Itoa(fromYear))
	}
	if toYear > 0 {
		q.Set("toYear", strconv.Itoa(toYear))
	}
	resp, err := callAPI(user, "getRandomSongs", q)
	if err != nil {
		return nil, err
	}
	if resp.RandomSongs == nil {
		return nil, nil
	}
	return resp.RandomSongs.Song, nil
}

func fetchOwnPlaylists(user string) ([]playlist, error) {
	resp, err := callAPI(user, "getPlaylists", nil)
	if err != nil {
		return nil, err
	}
	var own []playlist
	if resp.Playlists != nil {
		for _, pl := range resp.Playlists.Playlist {
			if equalFold(pl.Owner, user) {
				own = append(own, pl)
			}
		}
	}
	return own, nil
}

func fetchPlaylistSongIDs(user, id string) ([]string, error) {
	q := url.Values{}
	q.Set("id", id)
	resp, err := callAPI(user, "getPlaylist", q)
	if err != nil {
		return nil, err
	}
	if resp.Playlist == nil {
		return nil, nil
	}
	ids := make([]string, 0, len(resp.Playlist.Entry))
	for _, e := range resp.Playlist.Entry {
		ids = append(ids, e.ID)
	}
	return ids, nil
}

// createPlaylist creates a playlist and returns its ID.
func createPlaylist(user, name string, songIDs []string) (string, error) {
	q := url.Values{}
	q.Set("name", name)
	for _, id := range songIDs {
		q.Add("songId", id)
	}
	resp, err := callAPI(user, "createPlaylist", q)
	if err != nil {
		return "", err
	}
	if resp.Playlist != nil && resp.Playlist.ID != "" {
		return resp.Playlist.ID, nil
	}
	// Older servers do not return the playlist, so look up the newest one with this name.
	lists, err := fetchOwnPlaylists(user)
	if err != nil {
		return "", fmt.Errorf("playlist created, but its ID could not be determined: %w", err)
	}
	var newest *playlist
	for i := range lists {
		if lists[i].Name == name && (newest == nil || lists[i].Created.After(newest.Created)) {
			newest = &lists[i]
		}
	}
	if newest == nil {
		return "", errors.New("playlist created, but not found in getPlaylists")
	}
	return newest.ID, nil
}

func updatePlaylistMeta(user, id string, public bool, comment string) error {
	q := url.Values{}
	q.Set("playlistId", id)
	q.Set("public", strconv.FormatBool(public))
	q.Set("comment", comment)
	_, err := callAPI(user, "updatePlaylist", q)
	return err
}

func deletePlaylist(user, id string) error {
	q := url.Values{}
	q.Set("id", id)
	_, err := callAPI(user, "deletePlaylist", q)
	return err
}
