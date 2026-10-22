package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type subsonicResponse struct {
	Status  string         `json:"status"`
	Error   *subsonicError `json:"error"`
	Artists *struct {
		Index []struct {
			Artist []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artist"`
		} `json:"index"`
	} `json:"artists"`
	Artist *struct {
		Album []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Year      int    `json:"year"`
			SongCount int    `json:"songCount"`
		} `json:"album"`
	} `json:"artist"`
	Album *struct {
		Song []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Track int    `json:"track"`
		} `json:"song"`
	} `json:"album"`
}

type subsonicError struct {
	Message string `json:"message"`
}

func getArtists() ([]item, error) {
	resp, err := subsonicApiRequest("getArtists", nil)
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Artists != nil {
		for _, index := range resp.Artists.Index {
			for _, artist := range index.Artist {
				items = append(items, item{ID: artist.ID, Label: artist.Name, Crumb: artist.Name})
			}
		}
	}
	return items, nil
}

func getAlbums(artistID string) ([]item, error) {
	resp, err := subsonicApiRequest("getArtist", map[string]string{"id": artistID})
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Artist != nil {
		for _, album := range resp.Artist.Album {
			year := "????"
			if album.Year > 0 {
				year = strconv.Itoa(album.Year)
			}
			label := fmt.Sprintf("%s (%s) · %d songs", album.Name, year, album.SongCount)
			crumb := fmt.Sprintf("%s (%s)", album.Name, year)
			items = append(items, item{ID: album.ID, Label: label, Crumb: crumb})
		}
	}
	return items, nil
}

func getSongs(albumID string) ([]item, error) {
	resp, err := subsonicApiRequest("getAlbum", map[string]string{"id": albumID})
	if err != nil {
		return nil, err
	}

	var items []item
	if resp.Album != nil {
		for _, song := range resp.Album.Song {
			track := "?"
			if song.Track > 0 {
				track = strconv.Itoa(song.Track)
			}
			items = append(items, item{ID: song.ID, Label: fmt.Sprintf("%s. %s", track, song.Title)})
		}
	}
	return items, nil
}

func streamUrl(songID string) string {
	salt := randomSalt()
	sum := md5.Sum([]byte(password + salt))
	query := url.Values{
		"u":  {username},
		"t":  {hex.EncodeToString(sum[:])},
		"s":  {salt},
		"v":  {apiVersion},
		"c":  {clientName},
		"id": {songID},
	}
	return serverUrl + "/rest/stream?" + query.Encode()
}

func subsonicApiRequest(endpoint string, params map[string]string) (*subsonicResponse, error) {
	salt := randomSalt()
	sum := md5.Sum([]byte(password + salt))
	query := url.Values{
		"u": {username},
		"t": {hex.EncodeToString(sum[:])},
		"s": {salt},
		"v": {apiVersion},
		"c": {clientName},
		"f": {"json"},
	}
	for key, value := range params {
		query.Set(key, value)
	}

	resp, err := http.Get(serverUrl + "/rest/" + endpoint + "?" + query.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", endpoint, resp.StatusCode)
	}

	var envelope struct {
		Response subsonicResponse `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	if envelope.Response.Status != "ok" {
		message := "unknown API error"
		if envelope.Response.Error != nil {
			message = envelope.Response.Error.Message
		}
		return nil, errors.New(message)
	}
	return &envelope.Response, nil
}

func randomSalt() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
